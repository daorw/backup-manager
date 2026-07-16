//go:build darwin

package shortcut

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// createPlatformShortcut creates a Finder alias on the Desktop pointing to the binary.
func createPlatformShortcut(binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	desktopDir := filepath.Join(homeDir, "Desktop")
	aliasPath := filepath.Join(desktopDir, Name)

	// Check if shortcut already exists
	if _, err := os.Stat(aliasPath); err == nil {
		log.Printf("[shortcut] desktop shortcut already exists at %s", aliasPath)
		return nil
	}

	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	log.Printf("[shortcut] creating Finder alias: %s -> %s", aliasPath, absBinary)

	script := fmt.Sprintf(
		`tell application "Finder" to make alias file to POSIX file %q at POSIX file %q`,
		absBinary,
		desktopDir,
	)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// osascript creates "<binary name> alias" — rename to display name
	binaryName := filepath.Base(absBinary)
	generatedName := binaryName + " alias"
	generatedPath := filepath.Join(desktopDir, generatedName)

	if _, err := os.Stat(generatedPath); err == nil {
		if generatedPath != aliasPath {
			if renameErr := os.Rename(generatedPath, aliasPath); renameErr != nil {
				log.Printf("[shortcut] warning: could not rename alias: %v", renameErr)
			}
		}
	}

	log.Printf("[shortcut] desktop shortcut: %s", aliasPath)
	return nil
}
