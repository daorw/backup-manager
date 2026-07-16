package shortcut

import (
	"fmt"
	"log"
	"os"
)

// Name is the display name used for the desktop shortcut.
const Name = "Backup Manager"

// CreateOnDesktop creates a desktop shortcut to the currently running binary.
// If a shortcut already exists, it skips creation and returns nil.
// On macOS, it creates a Finder alias.
// On Windows, it creates a .lnk shortcut via PowerShell.
func CreateOnDesktop() error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	if err := createPlatformShortcut(binaryPath); err != nil {
		return err
	}

	log.Printf("[shortcut] desktop shortcut created successfully")
	return nil
}
