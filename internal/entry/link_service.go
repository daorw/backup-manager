package entry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/util"
)

// AddLinkRequest 添加 out 链接的入参。
type AddLinkRequest struct {
	LocalPath string `json:"local_path"`
	Device    string `json:"device,omitempty"` // 默认当前设备
}

// BulkLinkRequest 批量添加 out 链接的入参。
type BulkLinkRequest struct {
	LocalRoot string   `json:"local_root"`
	EntryIDs  []string `json:"entry_ids,omitempty"` // 为空表示全部条目
}

// AddLink 为条目添加一条链接：在 local_path 创建指向 data/<repo_path> 的软链接，不复制内容。
// 所有链接完全等价，不存在 in/out 之分。
func (s *Service) AddLink(repoID, entryID string, req *AddLinkRequest) (*EntryView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	e := m.FindEntry(entryID)
	if e == nil {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	if req.LocalPath == "" {
		return nil, fmt.Errorf("local_path is required")
	}

	local, err := absPath(req.LocalPath)
	if err != nil {
		return nil, err
	}
	device := req.Device
	if device == "" {
		device = util.MachineFingerprint()
	}

	if err := checkLinkPlacement(repo.Path, m, e, local); err != nil {
		return nil, err
	}

	if err := createSymlink(local, repo.Path, e.RepoPath); err != nil {
		return nil, err
	}

	ensureDevice(m, device)
	e.Links = append(e.Links, &model.Link{
		ID:        newID(),
		Device:    device,
		LocalPath: local,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	})

	if err := s.save(repo, m, "link: add "+e.RepoPath); err != nil {
		os.Remove(local)
		return nil, err
	}
	return s.Get(repoID, entryID)
}

// BulkLink 把一个本地根目录下的多个条目批量链接到本机。
// 用于「刚 clone 完仓库，一键把文件放回本机」。
func (s *Service) BulkLink(repoID string, req *BulkLinkRequest) ([]*EntryView, error) {
	if req.LocalRoot == "" {
		return nil, fmt.Errorf("local_root is required")
	}
	root, err := absPath(req.LocalRoot)
	if err != nil {
		return nil, err
	}

	// 先算出待处理条目，再逐条走单条逻辑，避免重复实现
	views, err := s.List(repoID)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(req.EntryIDs))
	for _, id := range req.EntryIDs {
		wanted[id] = true
	}

	var results []*EntryView
	for _, e := range views {
		if len(wanted) > 0 && !wanted[e.ID] {
			continue
		}
		v, err := s.AddLink(repoID, e.ID, &AddLinkRequest{
			LocalPath: filepath.Join(root, filepath.FromSlash(e.RepoPath)),
		})
		if err != nil {
			return results, fmt.Errorf("failed to link %q: %w", e.RepoPath, err)
		}
		results = append(results, v)
	}
	return results, nil
}

// RepairLink 重建一条链接的本机软链接。
// 只在状态为 missing / wrong_target / dangling 时可用，占用路径不会被覆盖。
func (s *Service) RepairLink(repoID, entryID, linkID string) (*EntryView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	e := m.FindEntry(entryID)
	if e == nil {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	l := e.FindLink(linkID)
	if l == nil {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	if info, err := os.Lstat(l.LocalPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return nil, fmt.Errorf("local path is a real file or directory, refusing to overwrite: %s", l.LocalPath)
		}
		os.Remove(l.LocalPath)
	}
	if err := createSymlink(l.LocalPath, repo.Path, e.RepoPath); err != nil {
		return nil, err
	}
	return s.Get(repoID, entryID)
}

// Readopt 处理 replaced 状态：本机路径被真实文件/目录占用
// （应用用「临时文件 + rename」原子写配置时会把软链接替换掉）。
//
// 处理办法是把这份新内容移入 data/<repo_path>，然后恢复软链接。
// 被覆盖掉的旧版本仍可从 Git 历史恢复（§9.12：Git 就是回收站）。
func (s *Service) Readopt(repoID, entryID, linkID string) (*EntryView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	e := m.FindEntry(entryID)
	if e == nil {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	l := e.FindLink(linkID)
	if l == nil {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	info, err := os.Lstat(l.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("local path is not occupied, use repair instead: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local path is still a symlink, use repair instead")
	}

	content := DataPath(repo.Path, e.RepoPath)
	if err := os.RemoveAll(content); err != nil {
		return nil, fmt.Errorf("failed to clear the old repository content: %w", err)
	}
	if err := movePath(l.LocalPath, content); err != nil {
		return nil, fmt.Errorf("failed to move the new content into the repository: %w", err)
	}
	if err := os.Symlink(content, l.LocalPath); err != nil {
		// 回滚：内容移回本机路径
		_ = movePath(content, l.LocalPath)
		return nil, fmt.Errorf("failed to recreate the link: %w", err)
	}
	return s.Get(repoID, entryID)
}

// RemoveLink 移除一条链接（含 in 链接 —— 移除后条目变为未绑定，内容不受影响）。
func (s *Service) RemoveLink(repoID, entryID, linkID string) (*EntryView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	e := m.FindEntry(entryID)
	if e == nil {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	l := e.FindLink(linkID)
	if l == nil {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	os.Remove(l.LocalPath)
	e.RemoveLink(linkID)

	// 移除只会减少违规，因此跳过校验，避免清单已不合规时无法清理
	if err := s.saveConverging(repo, m, "link: remove "+l.LocalPath); err != nil {
		return nil, err
	}
	return s.Get(repoID, entryID)
}

// absPath 把用户给出的路径规范为绝对路径。
func absPath(p string) (string, error) {
	cleaned := filepath.Clean(p)
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return abs, nil
}

// checkLinkPlacement 校验链接落点：不得指向仓库内部，不得占用已有内容，
// 且不得位于某个目录条目的本机路径之内（R-4）。
func checkLinkPlacement(repoRoot string, m *model.Manifest, e *model.Entry, local string) error {
	if insidePath(local, repoRoot) {
		return fmt.Errorf("invalid local_path: it is inside the repository")
	}

	// R-4：不能在已跟踪的目录内部再挂东西
	for _, dir := range m.Entries {
		if dir.Kind != model.EntryKindDir {
			continue
		}
		for _, owner := range dir.Links {
			if localPathsOverlap(local, owner.LocalPath) {
				return fmt.Errorf("local_path is inside directory entry %q (%s)", dir.RepoPath, owner.LocalPath)
			}
		}
	}

	// 落点必须空闲：不存在，或为空目录
	info, err := os.Lstat(local)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot access local_path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local_path already exists: %s", local)
	}
	entries, err := os.ReadDir(local)
	if err != nil {
		return fmt.Errorf("cannot read local_path: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("local_path is a non-empty directory: %s", local)
	}
	return nil
}
