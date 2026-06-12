package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"

	"github.com/google/uuid"
)

// SymlinkService handles symlink business logic.
type SymlinkService struct {
	store *store.Store
}

// NewSymlinkService creates a new SymlinkService.
func NewSymlinkService(s *store.Store) *SymlinkService {
	return &SymlinkService{store: s}
}

// CreateSymlinkRequest is the input for creating a new symlink.
type CreateSymlinkRequest struct {
	TargetPath string `json:"target_path"`
	RelPath    string `json:"relative_path,omitempty"`
}

// Create creates a new symlink in .links/ and copies the source to data/.
func (s *SymlinkService) Create(repoID string, req *CreateSymlinkRequest) (*model.Symlink, error) {
	if req.TargetPath == "" {
		return nil, fmt.Errorf("target_path is required")
	}

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	targetAbs, err := resolveTargetPath(req.TargetPath)
	if err != nil {
		return nil, err
	}

	sourceInfo, err := os.Stat(targetAbs)
	if err != nil {
		return nil, fmt.Errorf("cannot access target path: %w", err)
	}

	symType := model.SymlinkTypeFile
	if sourceInfo.IsDir() {
		symType = model.SymlinkTypeDirectory
	}

	relPath := req.RelPath
	if relPath == "" {
		relPath = filepath.Base(targetAbs)
	}
	relPath = filepath.Clean(relPath)

	// Ensure the relative path does not escape the .links directory
	if relPath == "." || relPath == ".." || strings.HasPrefix(relPath, "../") {
		return nil, fmt.Errorf("invalid relative path: %s", relPath)
	}

	linksDir := filepath.Join(repo.Path, ".links")
	dataDir := filepath.Join(repo.Path, "data")

	linkPath := filepath.Join(linksDir, relPath)
	dataPath := filepath.Join(dataDir, relPath)

	// Ensure parent directories exist in both .links/ and data/
	linkParent := filepath.Dir(linkPath)
	dataParent := filepath.Dir(dataPath)
	if err := os.MkdirAll(linkParent, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directories in .links: %w", err)
	}
	if err := os.MkdirAll(dataParent, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directories in data: %w", err)
	}

	// Create symlink in .links/
	if err := os.Symlink(targetAbs, linkPath); err != nil {
		return nil, fmt.Errorf("failed to create symlink: %w", err)
	}

	// Copy source to data/
	if symType == model.SymlinkTypeDirectory {
		if err := util.CopyDir(targetAbs, dataPath); err != nil {
			os.Remove(linkPath)
			return nil, fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := util.CopyFile(targetAbs, dataPath); err != nil {
			os.Remove(linkPath)
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	fileSize := int64(0)
	modifiedAt := time.Now()
	if !sourceInfo.IsDir() {
		fileSize = sourceInfo.Size()
	}
	modifiedAt = sourceInfo.ModTime()

	symlink := &model.Symlink{
		ID:           uuid.New().String(),
		RepoID:       repoID,
		RelativePath: relPath,
		TargetPath:   targetAbs,
		Type:         symType,
		FileSize:     fileSize,
		ModifiedAt:   &modifiedAt,
	}

	if err := s.store.CreateSymlink(symlink); err != nil {
		os.Remove(linkPath)
		os.RemoveAll(dataPath)
		return nil, fmt.Errorf("failed to save symlink: %w", err)
	}

	return symlink, nil
}

// List returns all symlinks for a repo.
func (s *SymlinkService) List(repoID string) ([]*model.Symlink, error) {
	return s.store.ListSymlinks(repoID)
}

// Get returns a single symlink by ID.
func (s *SymlinkService) Get(id string) (*model.Symlink, error) {
	return s.store.GetSymlink(id)
}

// Delete removes a symlink, its data copy, and cleans up empty directories.
func (s *SymlinkService) Delete(id string) error {
	sym, err := s.store.GetSymlink(id)
	if err != nil {
		return err
	}

	repo, err := s.store.GetRepo(sym.RepoID)
	if err != nil {
		return err
	}

	linkPath := filepath.Join(repo.Path, ".links", sym.RelativePath)
	dataPath := filepath.Join(repo.Path, "data", sym.RelativePath)

	// Remove symlink
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// Remove data copy
	if err := os.RemoveAll(dataPath); err != nil {
		return fmt.Errorf("failed to remove data: %w", err)
	}

	// Clean up empty parent directories in .links/ and data/
	s.cleanEmptyParentDirs(filepath.Join(repo.Path, ".links"), sym.RelativePath)
	s.cleanEmptyParentDirs(filepath.Join(repo.Path, "data"), sym.RelativePath)

	return s.store.DeleteSymlink(id)
}

// UpdateTarget changes the target of an existing symlink.
func (s *SymlinkService) UpdateTarget(id, newTarget string) (*model.Symlink, error) {
	sym, err := s.store.GetSymlink(id)
	if err != nil {
		return nil, err
	}

	repo, err := s.store.GetRepo(sym.RepoID)
	if err != nil {
		return nil, err
	}

	targetAbs, err := resolveTargetPath(newTarget)
	if err != nil {
		return nil, err
	}

	sourceInfo, err := os.Stat(targetAbs)
	if err != nil {
		return nil, fmt.Errorf("cannot access new target: %w", err)
	}

	symType := model.SymlinkTypeFile
	if sourceInfo.IsDir() {
		symType = model.SymlinkTypeDirectory
	}

	linkPath := filepath.Join(repo.Path, ".links", sym.RelativePath)
	dataPath := filepath.Join(repo.Path, "data", sym.RelativePath)

	// Remove old data and symlink
	os.Remove(linkPath)
	os.RemoveAll(dataPath)

	// Create new symlink
	if err := os.Symlink(targetAbs, linkPath); err != nil {
		return nil, fmt.Errorf("failed to create new symlink: %w", err)
	}

	// Copy new source to data/
	if symType == model.SymlinkTypeDirectory {
		if err := util.CopyDir(targetAbs, dataPath); err != nil {
			os.Remove(linkPath)
			return nil, fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := util.CopyFile(targetAbs, dataPath); err != nil {
			os.Remove(linkPath)
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	fileSize := int64(0)
	if !sourceInfo.IsDir() {
		fileSize = sourceInfo.Size()
	}
	modifiedAt := sourceInfo.ModTime()

	sym.TargetPath = targetAbs
	sym.Type = symType
	sym.FileSize = fileSize
	sym.ModifiedAt = &modifiedAt

	if err := s.store.UpdateSymlink(sym); err != nil {
		return nil, err
	}

	return sym, nil
}

// BatchImport creates multiple symlinks at once with rollback on failure.
func (s *SymlinkService) BatchImport(repoID string, targets []string) ([]*model.Symlink, error) {
	var results []*model.Symlink
	var createdIDs []string
	for _, target := range targets {
		sym, err := s.Create(repoID, &CreateSymlinkRequest{TargetPath: target})
		if err != nil {
			// Rollback: delete all previously created symlinks
			for _, id := range createdIDs {
				if rerr := s.Delete(id); rerr != nil {
					log.Printf("[batch-import] failed to rollback symlink %s: %v", id, rerr)
				}
			}
			return nil, fmt.Errorf("failed to import %q: %w", target, err)
		}
		results = append(results, sym)
		createdIDs = append(createdIDs, sym.ID)
	}
	return results, nil
}

// SyncDeletedSource checks all symlinks for a repo and removes entries
// whose source files no longer exist on disk.
func (s *SymlinkService) SyncDeletedSource(repoID string) (int, error) {
	symlinks, err := s.store.ListSymlinks(repoID)
	if err != nil {
		return 0, err
	}

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, sym := range symlinks {
		if _, err := os.Stat(sym.TargetPath); os.IsNotExist(err) {
			// Source file is gone — remove data and symlink
			dataPath := filepath.Join(repo.Path, "data", sym.RelativePath)
			linkPath := filepath.Join(repo.Path, ".links", sym.RelativePath)

			os.Remove(linkPath)
			os.RemoveAll(dataPath)

			s.cleanEmptyParentDirs(filepath.Join(repo.Path, ".links"), sym.RelativePath)
			s.cleanEmptyParentDirs(filepath.Join(repo.Path, "data"), sym.RelativePath)

			if err := s.store.DeleteSymlink(sym.ID); err != nil {
				return removed, err
			}
			removed++
		}
	}

	return removed, nil
}

// resolveTargetPath safely resolves a user-provided target path by
// standardizing it and preventing path traversal via SafeResolve.
func resolveTargetPath(targetPath string) (string, error) {
	return util.SafeResolve("/", targetPath)
}

// cleanEmptyParentDirs removes empty parent directories starting from the
// directory containing the given relativePath, up to the base path.
func (s *SymlinkService) cleanEmptyParentDirs(base, relativePath string) {
	dir := filepath.Dir(filepath.Join(base, relativePath))
	for {
		if dir == base {
			break
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			break
		}
		if len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

// SymlinkDirEntry represents an entry in a directory-type symlink's source directory.
// It extends BrowseEntry with an IsNew field that indicates whether the source file
// differs from its data/ copy.
type SymlinkDirEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "file" or "directory"
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	IsNew      bool   `json:"is_new"`
}

// compareFileWithData compares a source file against its data/ copy.
// Returns true if the source file is different from the data copy (or the data
// copy does not exist), false if they are identical.
// If the source file cannot be stat'd for reasons other than not-exist, the
// error is returned so callers can decide how to handle it.
func compareFileWithData(sourcePath, dataPath string) (bool, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Source file deleted — consider it "changed" relative to data/
			return true, nil
		}
		return false, fmt.Errorf("stat source %s: %w", sourcePath, err)
	}
	dataInfo, err := os.Stat(dataPath)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return true, nil
	}
	return sourceInfo.Size() != dataInfo.Size() || !sourceInfo.ModTime().Equal(dataInfo.ModTime()), nil
}

const maxDirDepth = 50

// ListDirEntries lists the contents of a directory within a directory-type
// symlink's source directory. It returns sorted entries (directories first, then
// files, alphabetically within each group), with hidden files (dot-prefixed)
// excluded. For file entries, it computes whether the source has changed
// compared to the data/ copy (is_new).
//
// subPath is an optional relative path within the symlink's source directory.
// An empty string lists the root of the symlink's source directory.
func (s *SymlinkService) ListDirEntries(repoID, linkID, subPath string) ([]SymlinkDirEntry, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	sym, err := s.store.GetSymlink(linkID)
	if err != nil {
		return nil, err
	}

	if sym.RepoID != repoID {
		return nil, fmt.Errorf("symlink does not belong to repo")
	}

	if sym.Type != model.SymlinkTypeDirectory {
		return nil, fmt.Errorf("symlink is not a directory")
	}

	// Build the base source directory (actual source) and data directory
	symSourceDir := sym.TargetPath
	symDataDir := filepath.Join(repo.Path, "data", sym.RelativePath)

	// Resolve the target directory: symSourceDir + subPath
	targetDir := symSourceDir
	dataDir := symDataDir
	if subPath != "" {
		resolved, err := util.SafeResolve(symSourceDir, subPath)
		if err != nil {
			return nil, fmt.Errorf("invalid subpath: %w", err)
		}
		targetDir = resolved
		// Compute corresponding data directory (may not exist, handled gracefully)
		dataDir = filepath.Join(symDataDir, subPath)
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	// Depth check relative to source root
	rel, err := filepath.Rel(symSourceDir, targetDir)
	if err != nil {
		return nil, fmt.Errorf("path resolution error: %w", err)
	}
	if rel != "." {
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if depth > maxDirDepth {
			return nil, fmt.Errorf("directory depth exceeds maximum (%d)", maxDirDepth)
		}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var result []SymlinkDirEntry
	for _, entry := range entries {
		// Skip hidden files/directories
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		e := SymlinkDirEntry{
			Name: entry.Name(),
			Path: filepath.Join(targetDir, entry.Name()),
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
			// Compare source file with data copy
			dataPath := filepath.Join(dataDir, entry.Name())
			if changed, err := compareFileWithData(filepath.Join(targetDir, entry.Name()), dataPath); err == nil {
				e.IsNew = changed
			} else {
				// On stat failure, conservatively mark as new so the user knows
				// something may be off with this file.
				e.IsNew = true
				log.Printf("[symlink] warning: compareFileWithData(%q, %q): %v",
					filepath.Join(targetDir, entry.Name()), dataPath, err)
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

// ComputeSymlinkIsNew checks if a file-type symlink's source file differs from
// its data/ copy. For directory-type symlinks, it returns false.
func (s *SymlinkService) ComputeSymlinkIsNew(sym *model.Symlink) (bool, error) {
	if sym.Type != model.SymlinkTypeFile {
		return false, nil
	}
	repo, err := s.store.GetRepo(sym.RepoID)
	if err != nil {
		return false, err
	}
	dataPath := filepath.Join(repo.Path, "data", sym.RelativePath)
	return compareFileWithData(sym.TargetPath, dataPath)
}


