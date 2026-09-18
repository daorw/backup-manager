package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"backup-manager/internal/store"
)

// BrowseEntry represents an entry in a directory listing.
type BrowseEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "file" or "directory"
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// BrowserService handles filesystem browsing.
//
// There is no AllowedRoots whitelist anymore: any path the server process can
// see is browsable. The only normalisation applied is `~` expansion, Clean and
// (best-effort) symlink evaluation, so callers always get back an absolute,
// canonical path.
type BrowserService struct {
	store   *store.Store
	homeDir string
}

// NewBrowserService creates a new BrowserService.
// homeDir is the default / fallback directory (also used to expand `~`).
func NewBrowserService(s *store.Store, homeDir string) *BrowserService {
	return &BrowserService{
		store:   s,
		homeDir: homeDir,
	}
}

// Home returns the default starting directory for browsing.
func (s *BrowserService) Home() string {
	if s.homeDir != "" {
		return s.homeDir
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return string(filepath.Separator)
}

// Browse lists the contents of a directory.
// includeHidden controls whether dot-files/dot-directories are returned.
func (s *BrowserService) Browse(browsePath string, includeHidden bool) ([]BrowseEntry, error) {
	if strings.TrimSpace(browsePath) == "" {
		browsePath = s.Home()
	}

	resolved := s.resolvePath(browsePath)

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot access path %q: %w", browsePath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", resolved)
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", resolved, err)
	}

	var result []BrowseEntry
	for _, entry := range entries {
		// Skip hidden files/directories unless explicitly requested
		if !includeHidden && isHidden(entry.Name()) {
			continue
		}

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

// resolvePath normalises a user-provided path into an absolute path:
// expands a leading `~`, makes it absolute (relative to the home directory),
// cleans it, and evaluates symlinks when the path exists.
func (s *BrowserService) resolvePath(userPath string) string {
	p := strings.TrimSpace(userPath)

	if p == "~" || strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		p = filepath.Join(s.Home(), strings.TrimPrefix(p, "~"))
	}

	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Home(), p)
	}

	cleaned := filepath.Clean(p)

	if real, err := filepath.EvalSymlinks(cleaned); err == nil {
		return real
	}
	return cleaned
}

// isHidden reports whether a file name should be treated as hidden.
func isHidden(name string) bool {
	return name != "" && name[0] == '.'
}
