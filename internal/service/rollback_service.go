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

// CommitFileChange 提交中变更的一个文件（路径相对 data/）。
type CommitFileChange struct {
	ChangeType   string `json:"change_type"`   // "A" | "M" | "D"
	RelativePath string `json:"relative_path"` // 例如 "notes/file.md"
}

// RollbackRequest 回滚入参。Paths 为空表示回滚该提交的全部变更。
type RollbackRequest struct {
	CommitHash string   `json:"commit_hash" binding:"required"`
	Paths      []string `json:"paths,omitempty"`
}

// RollbackResult 回滚结果汇总。
type RollbackResult struct {
	RepoID      string            `json:"repo_id"`
	CommitHash  string            `json:"commit_hash"`
	Total       int               `json:"total"`
	Success     int               `json:"success"`
	Failed      int               `json:"failed"`
	Failures    []RollbackFailure `json:"failures,omitempty"`
	CompletedAt string            `json:"completed_at"`
}

// RollbackFailure 单个文件回滚失败记录。
type RollbackFailure struct {
	RelativePath string `json:"relative_path"`
	Error        string `json:"error"`
}

// CommitFileResult 提交中某个文件的内容。
type CommitFileResult struct {
	Content   string `json:"content,omitempty"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Text      bool   `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// RestoreFileResult 单文件恢复结果。
type RestoreFileResult struct {
	RelativePath string `json:"relative_path"`
	Success      bool   `json:"success"`
	RestoredAt   string `json:"restored_at"`
}

const maxCommitFileSize = 10 * 1024 * 1024 // 10MB

// RollbackService 负责把 data/ 恢复到历史提交的状态。
//
// 新模型下它非常简单：内容只存在于 data/，所有本机链接都指向 data/，
// 因此回滚只需就地写回文件，本机全部路径自动反映结果。
type RollbackService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	repoMu    *util.RepoMutexManager
}

// NewRollbackService 创建回滚服务。
func NewRollbackService(s *store.Store, g *git.GitEngine, repoMu *util.RepoMutexManager) *RollbackService {
	return &RollbackService{store: s, gitEngine: g, repoMu: repoMu}
}

// ListCommitFiles 返回提交中变更的文件列表，用于前端展示与选择。
func (s *RollbackService) ListCommitFiles(repoID, commitHash string) ([]CommitFileChange, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}
	changed, err := s.gitEngine.GetChangedFilesInCommit(repo.Path, commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed files: %w", err)
	}

	res := make([]CommitFileChange, 0, len(changed))
	for _, c := range changed {
		rel, ok := strings.CutPrefix(c.Path, git.DataDirName+"/")
		if !ok {
			continue
		}
		res = append(res, CommitFileChange{ChangeType: c.ChangeType, RelativePath: rel})
	}
	return res, nil
}

// Rollback 把 data/ 下的文件恢复到指定提交的版本。
func (s *RollbackService) Rollback(repoID string, req *RollbackRequest) (*RollbackResult, error) {
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}
	if repo.Status == model.RepoStatusBackingUp {
		return nil, fmt.Errorf("cannot rollback while backup is in progress")
	}

	paths := req.Paths
	if len(paths) == 0 {
		changed, err := s.gitEngine.GetChangedFilesInCommit(repo.Path, req.CommitHash)
		if err != nil {
			return nil, fmt.Errorf("failed to list changed files: %w", err)
		}
		for _, c := range changed {
			if rel, ok := strings.CutPrefix(c.Path, git.DataDirName+"/"); ok {
				paths = append(paths, rel)
			}
		}
	}

	result := &RollbackResult{RepoID: repoID, CommitHash: req.CommitHash}
	for _, p := range paths {
		result.Total++
		if err := s.restoreFromCommit(repo, req.CommitHash, p); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, RollbackFailure{RelativePath: p, Error: err.Error()})
			continue
		}
		result.Success++
	}

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

// GetCommitFile 读取提交中某个文件的内容，用于回滚前预览。
func (s *RollbackService) GetCommitFile(repoID, commitHash, relPath string) (*CommitFileResult, error) {
	if relPath == "" {
		return nil, fmt.Errorf("path is required")
	}
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return nil, err
	}
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}

	gitFilePath := git.DataDirName + "/" + clean
	size, err := s.gitEngine.GetCommitFileSize(repo.Path, commitHash, gitFilePath)
	if err != nil {
		return nil, fmt.Errorf("file not found in commit: %w", err)
	}
	if size > maxCommitFileSize {
		return &CommitFileResult{Size: size, MimeType: "application/octet-stream", Truncated: true}, nil
	}

	content, mimeType, isText, err := s.gitEngine.ReadFileContent(repo.Path, commitHash, gitFilePath, maxCommitFileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	return &CommitFileResult{Content: content, MimeType: mimeType, Size: size, Text: isText}, nil
}

// RestoreFile 从提交中恢复单个文件到 data/。
func (s *RollbackService) RestoreFile(repoID, commitHash, relPath string) (*RestoreFileResult, error) {
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}
	if repo.Status == model.RepoStatusBackingUp {
		return nil, fmt.Errorf("cannot restore while backup is in progress")
	}
	if err := s.restoreFromCommit(repo, commitHash, relPath); err != nil {
		return nil, err
	}
	return &RestoreFileResult{
		RelativePath: relPath,
		Success:      true,
		RestoredAt:   time.Now().Format(time.RFC3339),
	}, nil
}

// restoreFromCommit 把 data/<relPath> 写成提交中的版本，保留原始权限。
func (s *RollbackService) restoreFromCommit(repo *model.Repo, commitHash, relPath string) error {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return err
	}
	gitFilePath := git.DataDirName + "/" + clean
	dest, err := util.SafeJoin(filepath.Join(repo.Path, "data"), clean)
	if err != nil {
		return fmt.Errorf("path safety check failed: %w", err)
	}

	perm, err := s.gitEngine.GetCommitFileMode(repo.Path, commitHash, gitFilePath)
	if err != nil {
		perm = 0644
	}
	if err := s.gitEngine.WriteFileContentTo(repo.Path, commitHash, gitFilePath, dest, perm); err != nil {
		return err
	}
	return os.Chmod(dest, perm)
}

// cleanRelPath 校验并规范相对路径，拒绝绝对路径与向上穿越。
func cleanRelPath(p string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(p))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid path: %s", p)
	}
	return clean, nil
}
