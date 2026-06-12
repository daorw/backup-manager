package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeResolve resolves userPath relative to allowedRoot and ensures the
// resolved path does not escape allowedRoot via symlinks or ".." traversal.
//
// Steps:
//  1. filepath.Clean() to normalize the path
//  2. If relative, join with allowedRoot
//  3. Convert to absolute path
//  4. Evaluate symlinks (if ErrNotExist, resolve the existing prefix)
//  5. Verify the resolved path is within allowedRoot
func SafeResolve(allowedRoot, userPath string) (string, error) {
	if allowedRoot == "" {
		return "", fmt.Errorf("allowedRoot must not be empty")
	}

	cleaned := filepath.Clean(userPath)

	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(allowedRoot, cleaned)
	}

	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Evaluate symlinks
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Some component(s) of the path do not exist.
			// Resolve symlinks on the longest existing prefix to prevent
			// symlink-escape attacks through non-existent paths.
			realPath, err = resolvePartialPath(absPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve partial path: %w", err)
			}
		} else {
			// Permission denied or other errors — do NOT degrade.
			return "", fmt.Errorf("failed to evaluate symlinks: %w", err)
		}
	}

	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute root: %w", err)
	}

	// Also resolve symlinks in the root to handle macOS /tmp → /private/tmp
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		realRoot = absRoot
	}

	// Check that realPath is within the allowed root
	if realPath != realRoot && !strings.HasPrefix(realPath, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside allowed root %q", userPath, allowedRoot)
	}

	return realPath, nil
}

// resolvePartialPath resolves symlinks on the longest existing prefix of the
// given absPath, then appends the non-existent suffix.
func resolvePartialPath(absPath string) (string, error) {
	// Walk up from absPath until we find an existing component
	dir := absPath
	for {
		_, err := os.Stat(dir)
		if err == nil {
			// Found an existing component
			realDir, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return "", fmt.Errorf("failed to eval symlinks on %q: %w", dir, err)
			}

			// Append the non-existent suffix
			suffix, _ := strings.CutPrefix(absPath, dir)
			// If the existing dir is a symlink itself, we resolved it to realDir.
			// If the existing dir is the path itself (exact match), suffix is empty.
			if suffix == "" {
				return realDir, nil
			}
			return realDir + suffix, nil
		}

		if !os.IsNotExist(err) {
			// Permission error or similar — propagate
			return "", fmt.Errorf("failed to stat %q: %w", dir, err)
		}

		// This component doesn't exist, walk up
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root and it still doesn't exist
			return "", fmt.Errorf("path %q does not exist and no existing ancestor found", absPath)
		}
		dir = parent
	}
}

// SafeResolveFile is like SafeResolve but additionally ensures the resolved
// path points to a regular file (not a directory).
func SafeResolveFile(allowedRoot, userPath string) (string, error) {
	resolved, err := SafeResolve(allowedRoot, userPath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to stat resolved path: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("resolved path %q is a directory, expected a file", resolved)
	}

	return resolved, nil
}
