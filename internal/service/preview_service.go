package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// PreviewService handles file preview and save logic.
type PreviewService struct {
	store *store.Store
}

// NewPreviewService creates a new PreviewService.
func NewPreviewService(s *store.Store) *PreviewService {
	return &PreviewService{store: s}
}

// ResolvedSource represents the resolved source file information.
type ResolvedSource struct {
	SourcePath       string  // Absolute path to the source file
	RepoRelativePath string  // Relative path within the repo (for data/ sync)
	SymlinkID        *string // Symlink ID (nil for files inside directory symlinks)
}

// ResolveSource resolves a relative path to the source file path.
// Strategy:
//  1. Exact match: sym.RelativePath == relPath -> sym.TargetPath (file symlink)
//  2. Prefix match: iterate directory symlinks, strings.HasPrefix(relPath, dirPath+"/")
//     -> filepath.Join(dirSym.TargetPath, suffix) via util.SafeResolveFile
//  3. No match: error
func (s *PreviewService) ResolveSource(repoID, relPath string) (*ResolvedSource, error) {
	// Case 1: Exact match
	if sym, err := s.store.GetSymlinkByRelativePath(repoID, relPath); err == nil {
		if sym.Type == model.SymlinkTypeDirectory {
			return nil, fmt.Errorf("cannot preview a directory symlink")
		}
		return &ResolvedSource{
			SourcePath:       sym.TargetPath,
			RepoRelativePath: sym.RelativePath,
			SymlinkID:        &sym.ID,
		}, nil
	}

	// Case 2: Prefix match - directory symlink inner file
	// Use longest prefix match to correctly handle overlapping directory
	// symlinks (e.g., "docs/" and "docs/work/").
	all, err := s.store.ListSymlinks(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list symlinks: %w", err)
	}

	var bestMatch *model.Symlink
	var bestPrefixLen int
	for _, sym := range all {
		if sym.Type != model.SymlinkTypeDirectory {
			continue
		}
		prefix := sym.RelativePath + "/"
		if strings.HasPrefix(relPath, prefix) && len(prefix) > bestPrefixLen {
			bestMatch = sym
			bestPrefixLen = len(prefix)
		}
	}

	if bestMatch != nil {
		prefix := bestMatch.RelativePath + "/"
		suffix := strings.TrimPrefix(relPath, prefix)
		sourcePath, err := util.SafeJoin(bestMatch.TargetPath, suffix)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve file inside directory symlink: %w", err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("source file not found: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("cannot preview a directory")
		}
		return &ResolvedSource{
			SourcePath:       sourcePath,
			RepoRelativePath: relPath,
			SymlinkID:        nil,
		}, nil
	}

	return nil, fmt.Errorf("no symlink found for path: %s", relPath)
}

// PreviewResult represents the result of a file preview.
type PreviewResult struct {
	Content   string `json:"content,omitempty"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Text      bool   `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// SaveResult represents the result of saving a file.
type SaveResult struct {
	FileSize   int64  `json:"file_size"`
	ModifiedAt string `json:"modified_at"`
}

// Preview reads a source file and returns its content for preview.
func (s *PreviewService) Preview(repoID, relPath string) (*PreviewResult, error) {
	resolved, err := s.ResolveSource(repoID, relPath)
	if err != nil {
		return nil, err
	}

	sourcePath := resolved.SourcePath
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source file not found: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("cannot preview a directory")
	}

	const maxPreviewSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxPreviewSize {
		return nil, fmt.Errorf("file too large for preview (max %d bytes)", maxPreviewSize)
	}

	// Detect MIME type
	mimeType, err := util.DetectMIME(sourcePath)
	if err != nil {
		mimeType = "application/octet-stream"
	}

	isText, _ := util.IsTextFile(sourcePath)

	result := &PreviewResult{
		MimeType: mimeType,
		Size:     info.Size(),
		Text:     isText,
	}

	if isText {
		content, truncated, err := readTextFile(sourcePath, maxPreviewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		result.Content = content
		result.Truncated = truncated
	}

	return result, nil
}

// SaveRequest is the API request for saving a file.
type SaveRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// SaveFile saves edited content to the source file and syncs to data/.
// It preserves the original file permissions.
func (s *PreviewService) SaveFile(repoID, relPath, content string) (*SaveResult, error) {
	// Resolve source
	resolved, err := s.ResolveSource(repoID, relPath)
	if err != nil {
		return nil, err
	}

	sourcePath := resolved.SourcePath

	// Check that source file exists
	info, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source file no longer exists: %s", sourcePath)
		}
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("cannot save to a directory")
	}

	// Preserve original file permissions
	origMode := info.Mode().Perm()

	// Write to source file
	if err := os.WriteFile(sourcePath, []byte(content), origMode); err != nil {
		return nil, fmt.Errorf("failed to write source file: %w", err)
	}
	// Ensure permissions are not affected by umask
	if err := os.Chmod(sourcePath, origMode); err != nil {
		return nil, fmt.Errorf("failed to preserve source file permissions: %w", err)
	}

	// Sync to data/ directory
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	dataPath := filepath.Join(repo.Path, "data", resolved.RepoRelativePath)
	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.WriteFile(dataPath, []byte(content), origMode); err != nil {
		return nil, fmt.Errorf("failed to sync to data/: %w", err)
	}
	if err := os.Chmod(dataPath, origMode); err != nil {
		return nil, fmt.Errorf("failed to preserve data file permissions: %w", err)
	}

	// Update symlink metadata (only for file symlinks)
	if resolved.SymlinkID != nil {
		now := time.Now()
		sym, err := s.store.GetSymlink(*resolved.SymlinkID)
		if err != nil {
			return nil, fmt.Errorf("failed to get symlink for metadata update: %w", err)
		}
		newSize := int64(len(content))
		sym.FileSize = newSize
		sym.ModifiedAt = &now
		if err := s.store.UpdateSymlink(sym); err != nil {
			return nil, fmt.Errorf("failed to update symlink metadata: %w", err)
		}
	}

	now := time.Now()
	return &SaveResult{
		FileSize:   int64(len(content)),
		ModifiedAt: now.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// readTextFile reads a text file with size limit.
func readTextFile(filePath string, maxSize int) (string, bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	buf := make([]byte, maxSize+1)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", false, err
	}

	truncated := n > maxSize
	if n > maxSize {
		n = maxSize
	}

	return string(buf[:n]), truncated, nil
}
