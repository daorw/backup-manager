package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
// standardizing it and preventing path traversal via ".." components.
func resolveTargetPath(targetPath string) (string, error) {
	cleaned := filepath.Clean(targetPath)
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid target path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	// Verify no ".." component remains after resolution
	for _, part := range strings.Split(absPath, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("path traversal detected in target path %q", targetPath)
		}
	}

	return absPath, nil
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


