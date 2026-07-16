//go:build !darwin && !windows

package shortcut

import "log"

// createPlatformShortcut is a no-op on unsupported platforms.
func createPlatformShortcut(binaryPath string) error {
	log.Printf("[shortcut] desktop shortcut creation not supported on this platform")
	return nil
}
