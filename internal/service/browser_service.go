package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// BrowseEntry represents an entry in a directory listing.
type BrowseEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "file" or "directory"
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// BrowserService handles filesystem browsing with security boundaries.
type BrowserService struct {
	store        *store.Store
	allowedRoots []string
}

// NewBrowserService creates a new BrowserService.
// allowedRoots are additional directories that can be browsed (e.g., $HOME).
func NewBrowserService(s *store.Store, additionalRoots ...string) *BrowserService {
	return &BrowserService{
		store:        s,
		allowedRoots: additionalRoots,
	}
}

// Browse lists the contents of a directory, with security checks.
// Only directories within allowed roots can be browsed.
func (s *BrowserService) Browse(browsePath string) ([]BrowseEntry, error) {
	if browsePath == "" {
		browsePath = "."
	}

	// Build the list of allowed roots: repos' root paths + additional roots
	roots := s.buildAllowedRoots()

	// Resolve the browse path safely
	resolved, err := s.resolveBrowsePath(browsePath, roots)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var result []BrowseEntry
	for _, entry := range entries {
		e := BrowseEntry{
			Name: entry.Name(),
			Path: filepath.Join(resolved, entry.Name()),
		}

		if entry.IsDir() {
			e.Type = "directory"
		} else {
			e.Type = "file"
			fi, err := entry.Info()
			if err == nil {
				e.Size = fi.Size()
				e.ModifiedAt = fi.ModTime().Format("2006-01-02T15:04:05Z07:00")
			}
		}

		// Skip hidden files/directories
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		result = append(result, e)
	}

	// Sort: directories first, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			if result[i].Type == "directory" {
				return true
			}
			return false
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// buildAllowedRoots collects all directories that can be browsed.
func (s *BrowserService) buildAllowedRoots() []string {
	rootSet := make(map[string]bool)

	// Include additional roots (e.g., $HOME)
	for _, r := range s.allowedRoots {
		if r != "" {
			abs, err := filepath.Abs(r)
			if err == nil {
				rootSet[abs] = true
			}
		}
	}

	// Include repo root directories
	repos, err := s.store.ListRepos()
	if err == nil {
		for _, repo := range repos {
			rootSet[repo.Path] = true
		}
	}

	var roots []string
	for r := range rootSet {
		roots = append(roots, r)
	}
	return roots
}

// AllowedRoots returns the list of directories that can be browsed.
func (s *BrowserService) AllowedRoots() []string {
	return s.buildAllowedRoots()
}

// resolveBrowsePath resolves a user-provided path against all allowed roots.
func (s *BrowserService) resolveBrowsePath(userPath string, roots []string) (string, error) {
	// First try to resolve as-is (absolute or relative to CWD) against each root
	if filepath.IsAbs(userPath) {
		cleaned := filepath.Clean(userPath)
		for _, root := range roots {
			resolved, err := util.SafeResolve(root, cleaned)
			if err == nil {
				return resolved, nil
			}
		}
		return "", fmt.Errorf("path %q is outside allowed browsing roots", userPath)
	}

	// For relative paths, try each root
	for _, root := range roots {
		resolved, err := util.SafeResolve(root, userPath)
		if err == nil {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed browsing roots", userPath)
}
