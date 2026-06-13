package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// BackupService handles backup execution and history.
type BackupService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	repoMu    *RepoMutexManager
	symSvc    *SymlinkService
	authSvc   *AuthService
}

// NewBackupService creates a new BackupService.
func NewBackupService(s *store.Store, g *git.GitEngine, symSvc *SymlinkService, authSvc *AuthService, repoMu *RepoMutexManager) *BackupService {
	return &BackupService{
		store:     s,
		gitEngine: g,
		symSvc:    symSvc,
		authSvc:   authSvc,
		repoMu:    repoMu,
	}
}

// BackupResult contains the result of a backup operation.
type BackupResult struct {
	RepoID        string `json:"repo_id"`
	CompletedAt   string `json:"completed_at"`
	FilesChanged  int    `json:"files_changed"`
	FilesAdded    int    `json:"files_added"`
	FilesRemoved  int    `json:"files_removed"`
	CommitHash    string `json:"commit_hash,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// SyncStats tracks per-directory sync statistics.
type SyncStats struct {
	FilesAdded   int
	FilesChanged int
	FilesRemoved int
}

// Trigger performs a full backup cycle: incremental detection → sync → git add → commit → push.
// If commitMessage is empty, a default message "Backup: YYYY-MM-DD HH:mm:ss" is used.
func (s *BackupService) Trigger(repoID string, commitMessage string) (result *BackupResult, err error) {
	// Per-repo mutual exclusion
	mu := s.getRepoMutex(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	// Set status to backing_up
	repo.Status = model.RepoStatusBackingUp
	if err := s.store.UpdateRepo(repo); err != nil {
		return nil, fmt.Errorf("failed to update repo status: %w", err)
	}

	// Restore status on exit — set to Error if backup failed, Active otherwise
	defer func() {
		if err != nil {
			repo.Status = model.RepoStatusError
		} else {
			repo.Status = model.RepoStatusActive
		}
		repo.UpdatedAt = time.Now()
		if updateErr := s.store.UpdateRepo(repo); updateErr != nil {
			log.Printf("failed to restore repo status: %v", updateErr)
		}
	}()

	// Verify the repository is a git repository
	if _, statErr := os.Stat(filepath.Join(repo.Path, ".git")); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("repository not initialized, please run Git Init first")
	}

	config, err := s.store.GetRepoConfig(repoID)
	if err != nil {
		config = &model.RepoConfig{RepoID: repoID, Branch: "main"}
	}

	// Step 1: Sync deleted source files
	removed, err := s.symSvc.SyncDeletedSource(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to sync deleted sources: %w", err)
	}

	// Step 2: Incremental detection - check mtime and size changes
	changed, filesAdded, err := s.syncChangedFiles(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to sync changed files: %w", err)
	}

	totalChanges := removed + changed
	if totalChanges == 0 {
		status, statusErr := s.gitEngine.Status(repo.Path)
		if statusErr != nil || status == "" {
			now := time.Now()
			return &BackupResult{
				RepoID:      repoID,
				CompletedAt: now.Format(time.RFC3339),
			}, nil
		}
		// There are uncommitted files (e.g., initial data/ copy at symlink
		// creation time was never committed) — count them and proceed.
		changed = len(strings.Split(strings.TrimSpace(status), "\n"))
		totalChanges = changed
	}

	// Step 3: git add -A
	if err := s.gitEngine.AddAll(repo.Path); err != nil {
		return nil, fmt.Errorf("git add failed: %w", err)
	}

	// Step 4: git commit
	commitMsg := commitMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Backup: %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	var commitHash string
	if config.GitUserName != "" && config.GitUserEmail != "" {
		if err := s.gitEngine.CommitWithAuthor(repo.Path, commitMsg, config.GitUserName, config.GitUserEmail); err != nil {
			return nil, fmt.Errorf("git commit failed: %w", err)
		}
	} else {
		if err := s.gitEngine.Commit(repo.Path, commitMsg); err != nil {
			return nil, fmt.Errorf("git commit failed: %w", err)
		}
	}

	// Get the commit hash from git log
	entries, logErr := s.gitEngine.Log(repo.Path, 1, 0)
	if logErr == nil && len(entries) > 0 {
		commitHash = entries[0].Hash
	}

	// Update last_backup_at (defer will handle status)
	now := time.Now()
	repo.LastBackupAt = &now
	repo.UpdatedAt = now

	return &BackupResult{
		RepoID:        repoID,
		CompletedAt:   now.Format(time.RFC3339),
		FilesChanged:  changed,
		FilesAdded:    filesAdded,
		FilesRemoved:  removed,
		CommitHash:    commitHash,
		CommitMessage: commitMsg,
	}, nil
}

// History returns the git commit history for a repo.
func (s *BackupService) History(repoID string, limit, offset int) ([]git.CommitEntry, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	// Verify that the repo path still exists on disk
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository directory %q no longer exists on disk", repo.Path)
	}

	return s.gitEngine.Log(repo.Path, limit, offset)
}

// Push pushes committed changes to the remote repository.
// It first checks the config's RemoteURL, then falls back to the git repo's
// existing remote origin. Auth env vars are only injected if configured;
// otherwise, the system's git credentials (SSH agent, credential helper, etc.)
// are used automatically.
func (s *BackupService) Push(repoID string, opts ...git.PushOption) error {
	mu := s.getRepoMutex(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return err
	}

	config, err := s.store.GetRepoConfig(repoID)
	if err != nil {
		config = &model.RepoConfig{RepoID: repoID, Branch: "main"}
	}

	// Prefer config RemoteURL; fall back to git repo's existing remote origin
	remoteURL := config.RemoteURL
	if remoteURL == "" {
		remoteURL, err = s.gitEngine.GetRemoteURL(repo.Path, "origin")
		if err != nil {
			return fmt.Errorf("no remote URL configured. Set it in Config tab or via 'git remote add origin <url>'")
		}
	}

	branch := config.Branch
	if branch == "" {
		branch = "main"
	}

	envVars := s.authSvc.BuildEnvVars(repoID)

	if err := s.gitEngine.Push(repo.Path, "origin", branch, envVars, opts...); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}
	return nil
}

// syncChangedFiles checks each symlink's source and syncs changed files to data/.
// Returns:
//   - totalChanges: the total number of file-level changes (added + changed + removed)
//   - filesAdded: the number of new files added (from directory symlinks only)
//   - err: any fatal error that occurred
func (s *BackupService) syncChangedFiles(repo *model.Repo) (totalChanges int, filesAdded int, err error) {
	symlinks, err := s.store.ListSymlinks(repo.ID)
	if err != nil {
		return 0, 0, err
	}

	for _, sym := range symlinks {
		if sym.Type == model.SymlinkTypeDirectory {
			stats, iterErr := s.syncDirectoryFiles(repo, sym)
			if iterErr != nil {
				return totalChanges, filesAdded, fmt.Errorf("failed to sync directory %q: %w", sym.RelativePath, iterErr)
			}
			totalChanges += stats.FilesAdded + stats.FilesChanged + stats.FilesRemoved
			filesAdded += stats.FilesAdded
		} else {
			changed, iterErr := s.syncOneFile(repo, sym)
			if iterErr != nil {
				return totalChanges, filesAdded, fmt.Errorf("failed to sync %q: %w", sym.RelativePath, iterErr)
			}
			if changed {
				totalChanges++
			}
		}
	}

	return totalChanges, filesAdded, nil
}

// syncOneFile checks if a file has changed and syncs it to data/.
func (s *BackupService) syncOneFile(repo *model.Repo, sym *model.Symlink) (bool, error) {
	info, err := os.Stat(sym.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Handled by SyncDeletedSource
			return false, nil
		}
		return false, fmt.Errorf("failed to stat target: %w", err)
	}

	currentSize := int64(0)
	if !info.IsDir() {
		currentSize = info.Size()
	}
	currentModTime := info.ModTime()

	// Compare mtime and size
	needsSync := false
	if sym.ModifiedAt == nil || !sym.ModifiedAt.Equal(currentModTime) {
		needsSync = true
	}
	if sym.FileSize != currentSize {
		needsSync = true
	}

	if !needsSync {
		return false, nil
	}

	dataPath := filepath.Join(repo.Path, "data", sym.RelativePath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		return false, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Copy updated source to data/
	if err := util.CopyFile(sym.TargetPath, dataPath); err != nil {
		return false, fmt.Errorf("failed to copy file: %w", err)
	}

	// Update stored metadata
	sym.FileSize = currentSize
	sym.ModifiedAt = &currentModTime
	if err := s.store.UpdateSymlink(sym); err != nil {
		return false, fmt.Errorf("failed to update symlink metadata: %w", err)
	}

	return true, nil
}

// syncDirectoryFiles performs incremental per-file sync for a directory-type
// symlink. It walks the source directory tree, follows nested symlinks, compares
// each file against the corresponding file in data/ by mtime and size, and
// copies only changed or new files. Files that exist in data/ but not in source
// are removed. It updates the symlink's aggregated metadata (FileSize = total
// size of all files, ModifiedAt = latest modification time across all files).
//
// Errors for individual files are logged but do not abort the entire sync.
func (s *BackupService) syncDirectoryFiles(repo *model.Repo, sym *model.Symlink) (*SyncStats, error) {
	stats := &SyncStats{}
	dataDir := filepath.Join(repo.Path, "data", sym.RelativePath)

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return stats, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Walk the source directory and sync files
	var latestModTime time.Time
	var totalSize int64

	// Collect all source files (relative paths) using walkSourceDir
	type sourceFile struct {
		relPath string
		fi      os.FileInfo
	}
	var sourceFiles []sourceFile

	if err := walkSourceDir(sym.TargetPath, sym.TargetPath, func(relPath string, fi os.FileInfo) error {
		sourceFiles = append(sourceFiles, sourceFile{relPath: relPath, fi: fi})
		totalSize += fi.Size()
		if fi.ModTime().After(latestModTime) {
			latestModTime = fi.ModTime()
		}
		return nil
	}); err != nil {
		return stats, fmt.Errorf("failed to walk source directory: %w", err)
	}

	// Build set of source files for removal detection
	sourceFileSet := make(map[string]bool, len(sourceFiles))
	for _, sf := range sourceFiles {
		sourceFileSet[sf.relPath] = true
	}

	// Sync each source file: copy if mtime or size differs
	for _, sf := range sourceFiles {
		srcPath := filepath.Join(sym.TargetPath, sf.relPath)
		dstPath := filepath.Join(dataDir, sf.relPath)

		// Ensure parent directory in data/ exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			log.Printf("[backup] warning: failed to create parent dir for %q: %v", sf.relPath, err)
			continue
		}

		// Check if destination exists and is up-to-date
		dstFi, dstErr := os.Stat(dstPath)
		if dstErr == nil {
			if dstFi.Size() == sf.fi.Size() && dstFi.ModTime().Equal(sf.fi.ModTime()) {
				// File is up-to-date, skip
				continue
			}
			stats.FilesChanged++
		} else if os.IsNotExist(dstErr) {
			stats.FilesAdded++
		} else {
			log.Printf("[backup] warning: cannot stat destination %q: %v", sf.relPath, dstErr)
			continue
		}

		// Copy the file
		if err := util.CopyFile(srcPath, dstPath); err != nil {
			log.Printf("[backup] warning: failed to copy %q: %v", sf.relPath, err)
			continue
		}
	}

	// Remove files in data/ that no longer exist in source
	if err := filepath.Walk(dataDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return nil
		}
		if !sourceFileSet[rel] {
			if err := os.Remove(path); err != nil {
				log.Printf("[backup] warning: failed to remove stale data file %q: %v", rel, err)
			} else {
				stats.FilesRemoved++
				log.Printf("[backup] removed stale data file: %s", rel)
			}
		}
		return nil
	}); err != nil {
		log.Printf("[backup] warning: failed to walk data directory for cleanup: %v", err)
	}

	// Clean up empty directories in data/
	s.cleanEmptyDataDirs(dataDir)

	// Update aggregated metadata
	sym.FileSize = totalSize
	if !latestModTime.IsZero() {
		sym.ModifiedAt = &latestModTime
	}
	if err := s.store.UpdateSymlink(sym); err != nil {
		return stats, fmt.Errorf("failed to update symlink metadata: %w", err)
	}

	return stats, nil
}

// cleanEmptyDataDirs removes empty directories recursively from the given path.
func (s *BackupService) cleanEmptyDataDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(dir, entry.Name())
			s.cleanEmptyDataDirs(subPath)
		}
	}
	// Try to remove if empty (after cleaning children)
	entries, err = os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}

// walkSourceDir walks a source directory tree, following symlinks and detecting
// cycles. It calls the callback for each regular file found, skipping hidden
// files and handling errors gracefully.
func walkSourceDir(baseDir, currentDir string, callback func(relPath string, fi os.FileInfo) error) error {
	return walkSourceDirRecursive(baseDir, currentDir, callback, make(map[string]bool), 0)
}

// walkSourceDirRecursive is the recursive implementation of walkSourceDir with
// cycle detection and depth limiting.
func walkSourceDirRecursive(baseDir, currentDir string, callback func(relPath string, fi os.FileInfo) error, visited map[string]bool, depth int) error {
	if depth > maxDirDepth {
		log.Printf("[backup] warning: skipping directory %q (depth limit exceeded)", currentDir)
		return nil
	}

	resolvedDir, err := filepath.EvalSymlinks(currentDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[backup] warning: skipping non-existent directory %q", currentDir)
			return nil
		}
		return fmt.Errorf("failed to evaluate symlinks for %q: %w", currentDir, err)
	}

	if visited[resolvedDir] {
		log.Printf("[backup] warning: skipping directory %q (cycle detected)", currentDir)
		return nil
	}
	visited[resolvedDir] = true

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		log.Printf("[backup] warning: cannot read directory %q: %v", currentDir, err)
		return nil
	}

	for _, entry := range entries {
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		fullPath := filepath.Join(currentDir, entry.Name())
		info, err := os.Lstat(fullPath)
		if err != nil {
			log.Printf("[backup] warning: cannot lstat %q: %v", fullPath, err)
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolved, chain, err := util.ResolveNestedSymlink(fullPath)
			if err != nil {
				log.Printf("[backup] warning: skipping symlink %q: %v", fullPath, err)
				continue
			}

			if detectCycle(chain) {
				log.Printf("[backup] warning: skipping symlink %q (cycle detected)", fullPath)
				continue
			}

			resolvedInfo, err := os.Stat(resolved)
			if err != nil {
				log.Printf("[backup] warning: cannot stat resolved symlink %q: %v", resolved, err)
				continue
			}

			if resolvedInfo.IsDir() {
				if err := walkSourceDirRecursive(baseDir, fullPath, callback, visited, depth+1); err != nil {
					return err
				}
			} else {
				relPath, err := filepath.Rel(baseDir, fullPath)
				if err != nil {
					log.Printf("[backup] warning: cannot compute relative path for %q: %v", fullPath, err)
					continue
				}
				if err := callback(relPath, resolvedInfo); err != nil {
					return err
				}
			}
		} else if info.IsDir() {
			if err := walkSourceDirRecursive(baseDir, fullPath, callback, visited, depth+1); err != nil {
				return err
			}
		} else {
			relPath, err := filepath.Rel(baseDir, fullPath)
			if err != nil {
				log.Printf("[backup] warning: cannot compute relative path for %q: %v", fullPath, err)
				continue
			}
			if err := callback(relPath, info); err != nil {
				return err
			}
		}
	}

	return nil
}

// detectCycleInBackup checks if following symlinks from currentPath would create
// a cycle within the context of baseDir. This is a simple check - more complex
// cycle detection is handled by walkSourceDirRecursive.
func detectCycleInBackup(currentPath, baseDir string) bool {
	resolved, err := filepath.EvalSymlinks(currentPath)
	if err != nil {
		return false
	}

	baseResolved, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return false
	}

	// Check if resolved path is the same as base
	return resolved == baseResolved
}

// getRepoMutex returns the per-repo mutex from the shared manager.
func (s *BackupService) getRepoMutex(repoID string) *sync.Mutex {
	return s.repoMu.Get(repoID)
}
