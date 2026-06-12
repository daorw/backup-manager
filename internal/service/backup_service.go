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
	FilesRemoved  int    `json:"files_removed"`
	CommitHash    string `json:"commit_hash,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// Trigger performs a full backup cycle: incremental detection → sync → git add → commit → push.
func (s *BackupService) Trigger(repoID string) (result *BackupResult, err error) {
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
	changed, err := s.syncChangedFiles(repo)
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
	commitMsg := fmt.Sprintf("Backup: %s", time.Now().Format("2006-01-02 15:04:05"))

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
func (s *BackupService) Push(repoID string) error {
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
	if err := s.gitEngine.Push(repo.Path, "origin", branch, envVars); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}
	return nil
}

// syncChangedFiles checks each symlink's source and syncs changed files to data/.
// Returns the number of files synced.
func (s *BackupService) syncChangedFiles(repo *model.Repo) (int, error) {
	symlinks, err := s.store.ListSymlinks(repo.ID)
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, sym := range symlinks {
		changed, err := s.syncOneFile(repo, sym)
		if err != nil {
			return synced, fmt.Errorf("failed to sync %q: %w", sym.RelativePath, err)
		}
		if changed {
			synced++
		}
	}

	return synced, nil
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
	if sym.Type == model.SymlinkTypeDirectory {
		// Remove old data and re-copy
		os.RemoveAll(dataPath)
		if err := util.CopyDir(sym.TargetPath, dataPath); err != nil {
			return false, fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := util.CopyFile(sym.TargetPath, dataPath); err != nil {
			return false, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	// Update stored metadata
	sym.FileSize = currentSize
	sym.ModifiedAt = &currentModTime
	if err := s.store.UpdateSymlink(sym); err != nil {
		return false, fmt.Errorf("failed to update symlink metadata: %w", err)
	}

	return true, nil
}

// getRepoMutex returns the per-repo mutex from the shared manager.
func (s *BackupService) getRepoMutex(repoID string) *sync.Mutex {
	return s.repoMu.Get(repoID)
}
