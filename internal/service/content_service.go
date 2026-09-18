package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

const (
	maxPreviewSize = 10 * 1024 * 1024 // 预览上限 10MB
	dataDirName    = "data"
)

// ContentEntry 浏览仓库内容时返回的一个节点。
type ContentEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"` // 相对 data/ 的斜杠路径
	Type       string `json:"type"` // file | directory
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// PreviewResult 文件预览结果。
type PreviewResult struct {
	Content   string `json:"content,omitempty"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Text      bool   `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// SaveResult 保存结果。
type SaveResult struct {
	FileSize   int64  `json:"file_size"`
	ModifiedAt string `json:"modified_at"`
}

// SaveRequest 保存文件的入参。
type SaveRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ContentService 负责浏览、预览与编辑仓库内容。
//
// 新模型下它只读写 data/ —— 内容就在那里，本机路径只是指向它的软链接，
// 所以一次写入即可让该条目的所有链接同步反映，不存在双写与同步步骤。
type ContentService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	repoMu    *util.RepoMutexManager
}

// NewContentService 创建内容服务。
func NewContentService(s *store.Store, g *git.GitEngine, repoMu *util.RepoMutexManager) *ContentService {
	return &ContentService{store: s, gitEngine: g, repoMu: repoMu}
}

// resolve 把 data/ 下的相对路径解析为安全的绝对路径。
func (s *ContentService) resolve(repoID, relPath string) (string, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(relPath)
	if relPath == "" || clean == "." {
		return filepath.Join(repo.Path, dataDirName), nil
	}
	resolved, err := util.SafeJoin(filepath.Join(repo.Path, dataDirName), clean)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return resolved, nil
}

// Tree 列出 data/<relPath> 下的条目，目录优先、组内按名称排序，隐藏项跳过。
// relPath 与返回的 path 都相对于 data/，与 Preview/Save 保持一致。
func (s *ContentService) Tree(repoID, relPath string) ([]ContentEntry, error) {
	dir, err := s.resolve(repoID, relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	raw, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	base := filepath.ToSlash(filepath.Clean(relPath))
	if base == "." {
		base = ""
	}

	result := make([]ContentEntry, 0, len(raw))
	for _, r := range raw {
		if strings.HasPrefix(r.Name(), ".") {
			continue
		}
		e := ContentEntry{Name: r.Name(), Path: filepath.ToSlash(filepath.Join(base, r.Name()))}
		if r.IsDir() {
			e.Type = "directory"
		} else {
			e.Type = "file"
			if fi, err := r.Info(); err == nil {
				e.Size = fi.Size()
				e.ModifiedAt = fi.ModTime().Format(time.RFC3339)
			}
		}
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		if (result[i].Type == "directory") != (result[j].Type == "directory") {
			return result[i].Type == "directory"
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Preview 读取 data/ 下的文件内容。
func (s *ContentService) Preview(repoID, relPath string) (*PreviewResult, error) {
	path, err := s.resolve(repoID, relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot preview a directory")
	}
	if info.Size() > maxPreviewSize {
		return nil, fmt.Errorf("file too large for preview (max %d bytes)", maxPreviewSize)
	}

	mimeType, err := util.DetectMIME(path)
	if err != nil {
		mimeType = "application/octet-stream"
	}
	isText, _ := util.IsTextFile(path)

	res := &PreviewResult{MimeType: mimeType, Size: info.Size(), Text: isText}
	if isText {
		content, truncated, err := readTextFile(path, maxPreviewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		res.Content = content
		res.Truncated = truncated
	}
	return res, nil
}

// Save 就地写入 data/ 下的文件，并保留原始权限。
// 因为本机链接都是指向该文件的软链接，写入后所有链接立即反映变更。
func (s *ContentService) Save(repoID, relPath, content string) (*SaveResult, error) {
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	defer mu.Unlock()

	path, err := s.resolve(repoID, relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot save to a directory")
	}

	mode := info.Mode().Perm()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	// 重新 Chmod，避免受 umask 影响
	if err := os.Chmod(path, mode); err != nil {
		return nil, fmt.Errorf("failed to preserve permissions: %w", err)
	}

	return &SaveResult{
		FileSize:   int64(len(content)),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// Changes 返回 data/ 下未提交的变更（git status --porcelain）。
// 它取代了旧模型的 is_new 比对，作为「有新变更」的信号。
func (s *ContentService) Changes(repoID string) ([]string, error) {
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		return nil, err
	}
	status, err := s.gitEngine.Status(repo.Path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, dataDirName+"/") || strings.HasSuffix(line, dataDirName) {
			out = append(out, line)
		}
	}
	return out, nil
}

// readTextFile 带大小上限地读取文本文件。
func readTextFile(path string, maxSize int) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	buf := make([]byte, maxSize+1)
	n, _ := f.Read(buf)
	truncated := n > maxSize
	if truncated {
		n = maxSize
	}
	return string(buf[:n]), truncated, nil
}
