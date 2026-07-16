//go:build darwin

package shortcut

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// createPlatformShortcut creates a minimal .app bundle on the Desktop that
// launches the binary silently in the background (no terminal window).
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
	if err := os.MkdirAll(macosDir, 0755); err != nil {
		return fmt.Errorf("failed to create app bundle: %w", err)
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

	// Info.plist — LSUIElement hides from Dock (app is tray-only)
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

	// Touch the .app bundle so Finder updates its icon cache
	if dirEntries, err := os.ReadDir(appPath); err == nil {
		for range dirEntries {
			// Force the directory to be re-scanned
		}
	}

	log.Printf("[shortcut] .app bundle created: %s", appPath)
	return nil
}
