package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/resolver"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// CommitFileChange represents a file changed in a commit, presented
// to the frontend for display and selection.
type CommitFileChange struct {
	ChangeType   string `json:"change_type"`   // "A" | "M" | "D"
	RelativePath string `json:"relative_path"` // Path relative to data/ (e.g., "notes/file.md")
	SymlinkID    string `json:"symlink_id,omitempty"`
	SymlinkType  string `json:"symlink_type,omitempty"` // "file" | "directory"
}

// RollbackRequest is the request payload for a rollback operation.
type RollbackRequest struct {
	CommitHash string   `json:"commit_hash" binding:"required"`
	SymlinkIDs []string `json:"symlink_ids"` // Empty means rollback all changed files
}

// RollbackResult contains the summary of a rollback operation.
type RollbackResult struct {
	RepoID      string            `json:"repo_id"`
	CommitHash  string            `json:"commit_hash"`
	Total       int               `json:"total"`
	Success     int               `json:"success"`
	Skipped     int               `json:"skipped"`
	Failed      int               `json:"failed"`
	Failures    []RollbackFailure `json:"failures,omitempty"`
	CompletedAt string            `json:"completed_at"`
}

// RollbackFailure records a single file's rollback failure.
type RollbackFailure struct {
	RelativePath string `json:"relative_path"`
	Error        string `json:"error"`
}

// CommitFileResult is the result of fetching a file from a commit.
type CommitFileResult struct {
	Content   string `json:"content,omitempty"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Text      bool   `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// RestoreFileRequest is the request payload for restoring a single file.
type RestoreFileRequest struct {
	Path string `json:"path" binding:"required"`
}

// RestoreFileResult contains the result of restoring a single file.
type RestoreFileResult struct {
	RelativePath string `json:"relative_path"`
	Success      bool   `json:"success"`
	RestoredAt   string `json:"restored_at"`
}

const maxCommitFileSize = 10 * 1024 * 1024 // 10MB

// RollbackService handles rollback operations: restoring source files
// to the state they had in a historical Git commit.
type RollbackService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	repoMu    *RepoMutexManager
}

// NewRollbackService creates a new RollbackService.
func NewRollbackService(s *store.Store, g *git.GitEngine, repoMu *RepoMutexManager) *RollbackService {
	return &RollbackService{
		store:     s,
		gitEngine: g,
		repoMu:    repoMu,
	}
}

// ListCommitFiles returns the list of files changed under data/ in the given commit,
// matched against current symlinks. Used by the frontend to display what can be rolled back.
func (s *RollbackService) ListCommitFiles(repoID, commitHash string) ([]CommitFileChange, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	// Get changed files in commit
	changedFiles, err := s.gitEngine.GetChangedFilesInCommit(repo.Path, commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed files: %w", err)
	}

	// Load current symlinks
	allSymlinks, err := s.store.ListSymlinks(repoID)
	if err != nil {
		return nil, err
	}

	// Build a lookup map: relative_path → symlink
	symlinkMap := make(map[string]*model.Symlink)
	for _, sym := range allSymlinks {
		symlinkMap[sym.RelativePath] = sym
	}

	// Build the file change list
	res := make([]CommitFileChange, 0, len(changedFiles))
	for _, gitPath := range changedFiles {
		relPath, ok := strings.CutPrefix(gitPath, resolver.DataDirPrefix)
		if !ok {
			continue
		}

		change := CommitFileChange{
			RelativePath: relPath,
		}

		// Determine change type from git diff-tree output
		// We don't have the change type directly from GetChangedFilesInCommit,
		// but we can look at the symlink to provide extra info
		if sym, found := symlinkMap[relPath]; found {
			change.SymlinkID = sym.ID
			change.SymlinkType = string(sym.Type)
		} else {
			// Try prefix match for directories
			for _, sym := range allSymlinks {
				if sym.Type == model.SymlinkTypeDirectory &&
					strings.HasPrefix(relPath, sym.RelativePath+"/") {
					change.SymlinkID = sym.ID
					change.SymlinkType = string(sym.Type)
					break
				}
			}
		}

		res = append(res, change)
	}

	return res, nil
}

// Rollback restores source files to the state they had in the given commit.
// If SymlinkIDs is non-empty, only those symlinks are rolled back.
// Otherwise, all symlinks with changes in the commit are rolled back.
func (s *RollbackService) Rollback(repoID string, req *RollbackRequest) (*RollbackResult, error) {
	// Acquire per-repo mutex — mutual exclusion with backup operations
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	// Reject if repo is currently backing up
	if repo.Status == model.RepoStatusBackingUp {
		return nil, fmt.Errorf("cannot rollback while backup is in progress")
	}

	// Get changed files in the target commit (deduplicated)
	changedFiles, err := s.gitEngine.GetChangedFilesInCommit(repo.Path, req.CommitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed files: %w", err)
	}
	changedFiles = deduplicateStrings(changedFiles)

	// Load current symlinks
	allSymlinks, err := s.store.ListSymlinks(repoID)
	if err != nil {
		return nil, err
	}

	// Build resolver and resolve paths
	resolver := resolver.NewSymlinkResolver(allSymlinks)
	grouped := resolver.ResolveCommitFiles(changedFiles)

	// Build rollback list (apply symlink_id filter if specified)
	type rollbackItem struct {
		symlink    *model.Symlink
		targetPath string
		gitRelPath string
	}

	var items []rollbackItem

	if len(req.SymlinkIDs) > 0 {
		// Filter by specified symlink IDs with ownership check
		for _, sid := range req.SymlinkIDs {
			sym, err := s.store.GetSymlink(sid)
			if err != nil {
				log.Printf("[rollback] symlink %s not found, skipping: %v", sid, err)
				continue
			}
			if sym.RepoID != repoID {
				log.Printf("[rollback] symlink %s belongs to repo %s, not %s, skipping",
					sid, sym.RepoID, repoID)
				continue
			}
			results, ok := grouped[sid]
			if !ok {
				continue
			}
			for _, r := range results {
				items = append(items, rollbackItem{
					symlink:    r.Symlink,
					targetPath: r.TargetPath,
					gitRelPath: r.GitRelPath,
				})
			}
		}
	} else {
		// Rollback all
		for _, results := range grouped {
			for _, r := range results {
				items = append(items, rollbackItem{
					symlink:    r.Symlink,
					targetPath: r.TargetPath,
					gitRelPath: r.GitRelPath,
				})
			}
		}
	}

	result := &RollbackResult{
		RepoID:     repoID,
		CommitHash: req.CommitHash,
	}

	// Track which symlinks have been updated for metadata refresh
	updatedSymlinks := make(map[string]*model.Symlink)

	for _, item := range items {
		result.Total++

		// Path safety check: ensure target is within symlink's allowed base
		var base string
		if item.symlink.Type == model.SymlinkTypeFile {
			base = filepath.Dir(item.symlink.TargetPath)
		} else {
			base = item.symlink.TargetPath
		}
		validatedPath, err := safeRollbackTarget(base, item.targetPath)
		if err != nil {
			result.Failed++
			result.Failures = append(result.Failures, RollbackFailure{
				RelativePath: item.gitRelPath,
				Error:        fmt.Sprintf("path safety check failed: %v", err),
			})
			continue
		}

		// Get file mode from git for permission preservation
		gitFilePath := git.DataDirName + "/" + item.gitRelPath
		perm, err := s.gitEngine.GetCommitFileMode(repo.Path, req.CommitHash, gitFilePath)
		if err != nil {
			log.Printf("[rollback] failed to get file mode for %s: %v, using 0644", gitFilePath, err)
			perm = 0644 // default fallback
		}

		// Stream write file content from git to source path
		if err := s.gitEngine.WriteFileContentTo(
			repo.Path, req.CommitHash, gitFilePath, validatedPath, perm,
		); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, RollbackFailure{
				RelativePath: item.gitRelPath,
				Error:        err.Error(),
			})
			continue
		}

		result.Success++

		// Track symlinks that need metadata refresh
		if _, ok := updatedSymlinks[item.symlink.ID]; !ok {
			updatedSymlinks[item.symlink.ID] = item.symlink
		}
	}

	// Update symlink metadata (file_size, modified_at) after successful rollback
	for _, sym := range updatedSymlinks {
		if sym.Type == model.SymlinkTypeFile {
			info, err := os.Stat(sym.TargetPath)
			if err == nil {
				sym.FileSize = info.Size()
				t := info.ModTime()
				sym.ModifiedAt = &t
				if updateErr := s.store.UpdateSymlink(sym); updateErr != nil {
					log.Printf("[rollback] failed to update symlink %s metadata: %v", sym.ID, updateErr)
				}
			}
		}
	}

	// Update repo's last backup time to indicate a rollback occurred
	now := time.Now()
	repo.UpdatedAt = now
	if repo.Status == model.RepoStatusError {
		repo.Status = model.RepoStatusActive
	}
	if err := s.store.UpdateRepo(repo); err != nil {
		log.Printf("[rollback] failed to update repo: %v", err)
	}

	result.CompletedAt = now.Format(time.RFC3339)

	return result, nil
}

// GetCommitFile retrieves a file's content from a specific commit for preview.
// If the file exceeds maxCommitFileSize, only metadata is returned.
func (s *RollbackService) GetCommitFile(repoID, commitHash, relPath string) (*CommitFileResult, error) {
	if relPath == "" {
		return nil, fmt.Errorf("path is required")
	}

	cleanPath := filepath.Clean(relPath)
	if strings.HasPrefix(cleanPath, ".") || filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("invalid path: %s", relPath)
	}

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	gitFilePath := git.DataDirName + "/" + cleanPath

	// Get file size first
	size, err := s.gitEngine.GetCommitFileSize(repo.Path, commitHash, gitFilePath)
	if err != nil {
		return nil, fmt.Errorf("file not found in commit: %w", err)
	}

	// If file exceeds max size, return metadata only (with truncated flag)
	if size > maxCommitFileSize {
		return &CommitFileResult{
			Size:      size,
			MimeType:  "application/octet-stream",
			Text:      false,
			Truncated: true,
		}, nil
	}

	// Read file content from git
	content, mimeType, isText, err := s.gitEngine.ReadFileContent(repo.Path, commitHash, gitFilePath, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return &CommitFileResult{
		Content:  content,
		MimeType: mimeType,
		Size:     size,
		Text:     isText,
	}, nil
}

// RestoreFile restores a single file from a commit back to its source location,
// syncs to data/, and updates symlink metadata.
func (s *RollbackService) RestoreFile(repoID, commitHash string, req *RestoreFileRequest) (*RestoreFileResult, error) {
	// Acquire per-repo mutex — mutual exclusion with backup operations
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	// Reject if repo is currently backing up
	if repo.Status == model.RepoStatusBackingUp {
		return nil, fmt.Errorf("cannot restore while backup is in progress")
	}

	// Validate path
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	cleanPath := filepath.Clean(req.Path)
	if strings.HasPrefix(cleanPath, ".") || filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("invalid path: %s", req.Path)
	}

	gitFilePath := git.DataDirName + "/" + cleanPath

	// Find matching symlink via resolver
	allSymlinks, err := s.store.ListSymlinks(repoID)
	if err != nil {
		return nil, err
	}

	resolver := resolver.NewSymlinkResolver(allSymlinks)
	resolved, err := resolver.Resolve(gitFilePath)
	if err != nil {
		return nil, fmt.Errorf("no matching symlink for %q: %w", req.Path, err)
	}

	// Path safety check: ensure target is within symlink's allowed base
	var base string
	if resolved.Symlink.Type == model.SymlinkTypeFile {
		base = filepath.Dir(resolved.Symlink.TargetPath)
	} else {
		base = resolved.Symlink.TargetPath
	}
	validatedPath, err := safeRollbackTarget(base, resolved.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("path safety check failed: %w", err)
	}

	// Get file mode from git for permission preservation
	perm, err := s.gitEngine.GetCommitFileMode(repo.Path, commitHash, gitFilePath)
	if err != nil {
		log.Printf("[restore-file] failed to get file mode for %s: %v, using 0644", gitFilePath, err)
		perm = 0644
	}

	// Stream write file content from git to source path
	if err := s.gitEngine.WriteFileContentTo(repo.Path, commitHash, gitFilePath, validatedPath, perm); err != nil {
		return nil, fmt.Errorf("failed to write file content: %w", err)
	}

	// Sync to data/ — CopyFile from source to data/
	dataPath := filepath.Join(repo.Path, "data", resolved.GitRelPath)
	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		// Source file is already restored; data sync failure is logged but
		// returned as an error so the user knows to expect a re-sync on next backup.
		return nil, fmt.Errorf("file restored but failed to sync to data/: %w", err)
	}
	if err := util.CopyFile(validatedPath, dataPath); err != nil {
		return nil, fmt.Errorf("file restored but failed to sync to data/: %w", err)
	}

	// Update symlink metadata
	info, err := os.Stat(validatedPath)
	if err == nil {
		resolved.Symlink.FileSize = info.Size()
		t := info.ModTime()
		resolved.Symlink.ModifiedAt = &t
		if updateErr := s.store.UpdateSymlink(resolved.Symlink); updateErr != nil {
			log.Printf("[restore-file] failed to update symlink metadata: %v", updateErr)
		}
	}

	return &RestoreFileResult{
		RelativePath: resolved.GitRelPath,
		Success:      true,
		RestoredAt:   time.Now().Format(time.RFC3339),
	}, nil
}

// deduplicateStrings removes duplicate entries from a string slice while preserving order.
func deduplicateStrings(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, s := range input {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// safeRollbackTarget verifies that the target path stays within the allowed base.
// For file-type symlinks, base = filepath.Dir(sym.TargetPath).
// For directory-type symlinks, base = sym.TargetPath.
// It resolves symlinks in both base and target to prevent symlink-based escapes.
func safeRollbackTarget(base, target string) (string, error) {
	absBaseRaw, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absTargetRaw, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	// Resolve symlinks to prevent escape via symlink manipulation
	absBase, err := filepath.EvalSymlinks(absBaseRaw)
	if err != nil {
		// If base doesn't exist, use the unresolved path
		absBase = absBaseRaw
	}
	absTarget, err := filepath.EvalSymlinks(absTargetRaw)
	if err != nil {
		// If target doesn't exist, use the unresolved path
		absTarget = absTargetRaw
	}

	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) &&
		absTarget != absBase {
		return "", fmt.Errorf("rollback target %q escapes base %q", target, base)
	}
	return absTarget, nil
}
