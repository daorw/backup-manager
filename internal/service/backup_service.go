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
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// BackupResult 备份结果。
type BackupResult struct {
	RepoID        string `json:"repo_id"`
	CompletedAt   string `json:"completed_at"`
	FilesChanged  int    `json:"files_changed"`
	FilesAdded    int    `json:"files_added"`
	FilesRemoved  int    `json:"files_removed"`
	CommitHash    string `json:"commit_hash,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// BackupService 执行备份。
//
// 新模型下内容本来就存在于 data/，本机路径只是指向它的软链接，
// 因此备份只剩三件事：git add -A → git commit →（可选）git push。
type BackupService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	authSvc   *AuthService
	repoMu    *util.RepoMutexManager
}

// NewBackupService 创建备份服务。
func NewBackupService(s *store.Store, g *git.GitEngine, authSvc *AuthService, repoMu *util.RepoMutexManager) *BackupService {
	return &BackupService{store: s, gitEngine: g, authSvc: authSvc, repoMu: repoMu}
}

// Trigger 执行一次备份：git add -A → git commit → 记录状态。
func (s *BackupService) Trigger(repoID, commitMessage string) (result *BackupResult, err error) {
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	repo.Status = model.RepoStatusBackingUp
	if err := s.store.UpdateRepo(repo); err != nil {
		return nil, fmt.Errorf("failed to update repo status: %w", err)
	}
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

	if _, statErr := os.Stat(filepath.Join(repo.Path, ".git")); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("repository not initialized, please run Git Init first")
	}

	// 统计未提交变更，同时用于判断是否有内容需要提交
	status, err := s.gitEngine.Status(repo.Path)
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}
	added, changed, removed := countChanges(status)

	now := time.Now()
	if added+changed+removed == 0 {
		return &BackupResult{RepoID: repoID, CompletedAt: now.Format(time.RFC3339)}, nil
	}

	if err := s.gitEngine.AddAll(repo.Path); err != nil {
		return nil, fmt.Errorf("git add failed: %w", err)
	}

	commitMsg := commitMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Backup: %s", now.Format("2006-01-02 15:04:05"))
	}

	config, cfgErr := s.store.GetRepoConfig(repoID)
	if cfgErr == nil && config.GitUserName != "" && config.GitUserEmail != "" {
		err = s.gitEngine.CommitWithAuthor(repo.Path, commitMsg, config.GitUserName, config.GitUserEmail)
	} else {
		err = s.gitEngine.Commit(repo.Path, commitMsg)
	}
	if err != nil {
		return nil, fmt.Errorf("git commit failed: %w", err)
	}

	commitHash := ""
	if entries, logErr := s.gitEngine.Log(repo.Path, 1, 0); logErr == nil && len(entries) > 0 {
		commitHash = entries[0].Hash
	}

	repo.LastBackupAt = &now
	repo.UpdatedAt = now

	return &BackupResult{
		RepoID:        repoID,
		CompletedAt:   now.Format(time.RFC3339),
		FilesChanged:  changed,
		FilesAdded:    added,
		FilesRemoved:  removed,
		CommitHash:    commitHash,
		CommitMessage: commitMsg,
	}, nil
}

// History 返回提交历史。
func (s *BackupService) History(repoID string, limit, offset int) ([]git.CommitEntry, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository directory %q no longer exists on disk", repo.Path)
	}
	return s.gitEngine.Log(repo.Path, limit, offset)
}

// Push 推送提交到远程仓库。
func (s *BackupService) Push(repoID string, opts ...git.PushOption) error {
	mu := s.repoMu.Get(repoID)
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

	remoteURL := config.RemoteURL
	if remoteURL == "" {
		if remoteURL, err = s.gitEngine.GetRemoteURL(repo.Path, "origin"); err != nil {
			return fmt.Errorf("no remote URL configured. Set it in Config tab or via 'git remote add origin <url>'")
		}
	}

	branch := config.Branch
	if branch == "" {
		branch = "main"
	}

	if err := s.gitEngine.Push(repo.Path, "origin", branch, s.authSvc.BuildEnvVars(repoID), opts...); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}
	return nil
}

// countChanges 解析 git status --porcelain，统计新增/修改/删除数量。
func countChanges(status string) (added, changed, removed int) {
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 2 {
			continue
		}
		code := line[:2]
		switch {
		case strings.HasPrefix(code, "??"), code[0] == 'A':
			added++
		case code[0] == 'D' || code[1] == 'D':
			removed++
		default:
			changed++
		}
	}
	return added, changed, removed
}
