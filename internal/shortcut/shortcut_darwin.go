//go:build darwin

package shortcut

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed app-icon-darwin.png
var appIconPNG []byte

// createPlatformShortcut creates a minimal .app bundle on the Desktop that
// launches the binary silently in the background (no terminal window).
// The bundle includes a proper .icns icon so Finder displays the custom
// app icon (not the default generic document icon).
func createPlatformShortcut(binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	desktopDir := filepath.Join(homeDir, "Desktop")
	appPath := filepath.Join(desktopDir, Name+".app")

	// Check if shortcut already exists
	if _, err := os.Stat(appPath); err == nil {
		log.Printf("[shortcut] desktop shortcut already exists at %s", appPath)
		return nil
	}

	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	log.Printf("[shortcut] creating .app bundle: %s -> %s", appPath, absBinary)

	// Create minimal .app bundle directory structure
	macosDir := filepath.Join(appPath, "Contents", "MacOS")
	resourcesDir := filepath.Join(appPath, "Contents", "Resources")
	if err := os.MkdirAll(macosDir, 0755); err != nil {
		return fmt.Errorf("failed to create app bundle: %w", err)
	}
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create resources dir: %w", err)
	}

	// Launcher shell script — runs binary in background, no terminal
	launcherPath := filepath.Join(macosDir, "backup-manager-launcher")
	launcher := fmt.Sprintf(
		`#!/bin/bash
nohup %q > /dev/null 2>&1 &
`, absBinary)
	if err := os.WriteFile(launcherPath, []byte(launcher), 0755); err != nil {
		return fmt.Errorf("failed to write launcher: %w", err)
	}

	// Generate the .icns icon from the embedded PNG using sips + iconutil
	if err := generateAppIcon(appPath, resourcesDir); err != nil {
		// Icon generation is best-effort — log warning but don't fail shortcut creation
		log.Printf("[shortcut] warning: failed to generate .icns icon: %v (using default icon)", err)
	}

	// Info.plist — LSUIElement hides from Dock (app is tray-only)
	// CFBundleIconFile points to the .icns file in Contents/Resources/
	infoPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>backup-manager-launcher</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>com.backup-manager</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleIconFile</key>
	<string>app-icon</string>
	<key>LSUIElement</key>
	<true/>
	<key>LSBackgroundOnly</key>
	<false/>
</dict>
</plist>`, Name)

	infoPlistPath := filepath.Join(appPath, "Contents", "Info.plist")
	if err := os.WriteFile(infoPlistPath, []byte(infoPlist), 0644); err != nil {
		return fmt.Errorf("failed to write Info.plist: %w", err)
	}

	// Touch the .app bundle so Finder refreshes its icon cache
	if err := exec.Command("touch", appPath).Run(); err != nil {
		log.Printf("[shortcut] warning: failed to touch .app bundle: %v", err)
	}

	log.Printf("[shortcut] .app bundle created: %s", appPath)
	return nil
}

// generateAppIcon builds Contents/Resources/app-icon.icns from the embedded
// appIconPNG. macOS requires CFBundleIconFile to point to an .icns file (not
// a PNG) for the custom icon to appear in Finder.
//
// Workflow:
//  1. Write the embedded PNG to a temp file
//  2. Use sips to create the canonical 10 sizes inside a .iconset directory
//  3. Use iconutil -c icns to package the iconset into a single .icns file
//  4. Clean up the temp files
func generateAppIcon(appPath, resourcesDir string) error {
	// 1. Write the embedded PNG to a temp file
	tmpDir, err := os.MkdirTemp("", "backup-manager-icon-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPNG := filepath.Join(tmpDir, "source.png")
	if err := os.WriteFile(srcPNG, appIconPNG, 0644); err != nil {
		return fmt.Errorf("write source png: %w", err)
	}

	// 2. Build the .iconset directory with all required sizes
	iconsetDir := filepath.Join(tmpDir, "app-icon.iconset")
	if err := os.MkdirAll(iconsetDir, 0755); err != nil {
		return fmt.Errorf("create iconset dir: %w", err)
	}

	// (filename, pixel-size) pairs required by iconutil for a complete .icns
	sizes := []struct {
		name string
		size int
	}{
		{"icon_16x16.png", 16},
		{"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32},
		{"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128},
		{"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256},
		{"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512},
		{"icon_512x512@2x.png", 1024},
	}
	for _, s := range sizes {
		out := filepath.Join(iconsetDir, s.name)
		cmd := exec.Command("sips", "-z", fmt.Sprintf("%d", s.size), fmt.Sprintf("%d", s.size),
			srcPNG, "--out", out)
		if outBuf, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sips %s: %v (%s)", s.name, err, string(outBuf))
		}
	}

	// 3. Convert iconset → .icns
	icnsPath := filepath.Join(resourcesDir, "app-icon.icns")
	cmd := exec.Command("iconutil", "-c", "icns", iconsetDir, "-o", icnsPath)
	if outBuf, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iconutil: %v (%s)", err, string(outBuf))
	}

	// Verify the file was created and is non-empty
	info, err := os.Stat(icnsPath)
	if err != nil {
		return fmt.Errorf("stat icns: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("icns file is empty")
	}

	log.Printf("[shortcut] generated app icon: %s (%.1f KB)",
		icnsPath, float64(info.Size())/1024)
	return nil
}
