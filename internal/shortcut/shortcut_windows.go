//go:build windows

package shortcut

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// createPlatformShortcut creates a .lnk shortcut on the Desktop using PowerShell.
func createPlatformShortcut(binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	desktopDir := filepath.Join(homeDir, "Desktop")
	shortcutPath := filepath.Join(desktopDir, Name+".lnk")

	// Check if shortcut already exists
	if _, err := os.Stat(shortcutPath); err == nil {
		log.Printf("[shortcut] desktop shortcut already exists at %s", shortcutPath)
		return nil
	}

	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	log.Printf("[shortcut] creating .lnk: %s -> %s", shortcutPath, absBinary)

	// Build VBS launcher to hide console window.
	// VBS `Run` with mode 0 = hidden window, False = don't wait.
	vbsPath := shortcutPath + ".launcher.vbs"
	vbsScript := fmt.Sprintf(
		`CreateObject("WScript.Shell").Run """%s""", 0, False`+"\n",
		absBinary,
	)
	if err := os.WriteFile(vbsPath, []byte(vbsScript), 0644); err != nil {
		return fmt.Errorf("failed to write launcher script: %w", err)
	}

	// Shortcut runs wscript.exe with the VBS launcher, no console flash.
	psScript := fmt.Sprintf(
		`$WScriptShell = New-Object -ComObject WScript.Shell
$Shortcut = $WScriptShell.CreateShortcut(%q)
$Shortcut.TargetPath = "wscript.exe"
$Shortcut.Arguments = "//B " + %q
$Shortcut.Description = "Backup Manager - File backup management tool"
$Shortcut.WorkingDirectory = %q
$Shortcut.WindowStyle = 7
$Shortcut.Save()`,
		shortcutPath,
		vbsPath,
		filepath.Dir(absBinary),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell shortcut creation failed: %w (output: %s)", err, string(output))
	}

	log.Printf("[shortcut] desktop shortcut: %s", shortcutPath)
	return nil
}
