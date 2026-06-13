package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoin securely joins a sub-path to an allowed root directory.
// It performs the following checks:
//   1. Cleans the subPath using filepath.Clean
//   2. Rejects path traversal attempts (.. or ../)
//   3. If subPath is absolute, ensures it is within allowedRoot
//   4. Joins subPath to allowedRoot and ensures result is within allowedRoot
//
// Unlike SafeResolve, SafeJoin does NOT call filepath.EvalSymlinks.
// This is intentional for directory symlink traversal: when the user has
// already authorized a directory symlink, nested symlinks inside it may
// legitimately point outside the allowedRoot, and we should not reject them.
func SafeJoin(allowedRoot, subPath string) (string, error) {
	cleaned := filepath.Clean(subPath)

	if !filepath.IsAbs(cleaned) && !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("path traversal not allowed: %s", subPath)
	}

	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve allowed root: %w", err)
	}

	var result string
	if filepath.IsAbs(cleaned) {
		result = cleaned
	} else {
		result = filepath.Join(absRoot, cleaned)
	}

	// Ensure result is within allowedRoot
	rootPrefix := absRoot
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	if result != absRoot && !strings.HasPrefix(result, rootPrefix) {
		return "", fmt.Errorf("path outside allowed root: %s", subPath)
	}

	return result, nil
}

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

	// Check that realPath is within the allowed root.
	// When realRoot is "/", appending a separator would produce "//"
	// which would break the prefix check. Handle that edge case.
	rootPrefix := realRoot
	if realRoot != "/" {
		rootPrefix += string(filepath.Separator)
	}
	if realPath != realRoot && !strings.HasPrefix(realPath, rootPrefix) {
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

const maxSymlinkDepth = 10

// ResolveNestedSymlink resolves a symlink chain, following each symlink until
// a non-symlink target is found. It returns the final resolved path (with
// symlinks evaluated) and the chain of symlink paths visited. It detects
// cycles and enforces a depth limit.
func ResolveNestedSymlink(linkPath string) (string, []string, error) {
	visited := make(map[string]bool)
	var chain []string

	current := linkPath
	for depth := 0; depth < maxSymlinkDepth; depth++ {
		if visited[current] {
			return "", nil, fmt.Errorf("symlink cycle detected at %q", current)
		}
		visited[current] = true

		fi, err := os.Lstat(current)
		if err != nil {
			return "", nil, fmt.Errorf("failed to lstat %q: %w", current, err)
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", nil, fmt.Errorf("failed to evaluate symlinks on %q: %w", current, err)
			}
			return resolved, chain, nil
		}

		chain = append(chain, current)

		target, err := os.Readlink(current)
		if err != nil {
			return "", nil, fmt.Errorf("failed to readlink %q: %w", current, err)
		}

		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		target = filepath.Clean(target)
		current = target
	}

	return "", nil, fmt.Errorf("symlink depth exceeds maximum (%d)", maxSymlinkDepth)
}

// SafeResolveNestedSymlink resolves a nested symlink chain while ensuring the
// final resolved path stays within allowedRoot. It detects cycles and enforces
// the depth limit.
func SafeResolveNestedSymlink(allowedRoot, linkPath string) (string, []string, error) {
	resolved, chain, err := ResolveNestedSymlink(linkPath)
	if err != nil {
		return "", nil, err
	}

	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve absolute root: %w", err)
	}

	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		realRoot = absRoot
	}

	rootPrefix := realRoot
	if realRoot != "/" {
		rootPrefix += string(filepath.Separator)
	}

	if resolved != realRoot && !strings.HasPrefix(resolved, rootPrefix) {
		return "", nil, fmt.Errorf("symlink target %q is outside allowed root %q", resolved, allowedRoot)
	}

	return resolved, chain, nil
}

// SafeResolveWithSymlinkChain resolves a path that may contain symlinks,
// returning both the final resolved path and any symlink chain encountered.
// It combines SafeResolve with nested symlink resolution, ensuring the final
// path stays within allowedRoot.
func SafeResolveWithSymlinkChain(allowedRoot, userPath string) (string, []string, error) {
	absPath, err := filepath.Abs(userPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	fi, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			resolved, err := SafeResolve(allowedRoot, userPath)
			if err != nil {
				return "", nil, err
			}
			return resolved, nil, nil
		}
		return "", nil, fmt.Errorf("failed to lstat path: %w", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		resolved, err := SafeResolve(allowedRoot, userPath)
		if err != nil {
			return "", nil, err
		}
		return resolved, nil, nil
	}

	return SafeResolveNestedSymlink(allowedRoot, absPath)
}
