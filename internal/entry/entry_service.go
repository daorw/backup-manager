package entry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/util"
)

// 条目级移除模式。
const (
	RemoveModeUnlink   = "unlink"    // 只删除本机链接，条目保留（可能变为未绑定）
	RemoveModeMoveBack = "move_back" // 把内容移回指定链接的本机路径，再移除条目
	RemoveModePurge    = "purge"     // 删除内容并移除条目
)

// AdoptRequest 创建条目的入参。
type AdoptRequest struct {
	LocalPath      string `json:"local_path"`
	RepoPath       string `json:"repo_path,omitempty"`
	FollowSymlinks bool   `json:"follow_symlinks,omitempty"`
}

// List 返回仓库的全部条目（含逐链接状态）。
func (s *Service) List(repoID string) ([]*EntryView, error) {
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	return s.buildEntryViews(repo.Path, m, util.MachineFingerprint()), nil
}

// Get 返回单个条目。
func (s *Service) Get(repoID, entryID string) (*EntryView, error) {
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	if m.FindEntry(entryID) == nil {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	for _, v := range s.buildEntryViews(repo.Path, m, util.MachineFingerprint()) {
		if v.ID == entryID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("entry not found: %s", entryID)
}

// Adopt 创建条目：把本机内容「移动」进 data/<repo_path>，并在原位置创建软链接。
// 这是条目的第一条链接。任一环节失败都会回滚，不会丢内容。
func (s *Service) Adopt(repoID string, req *AdoptRequest) (*EntryView, error) {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return nil, err
	}
	if req.LocalPath == "" {
		return nil, fmt.Errorf("local_path is required")
	}

	local := filepath.Clean(req.LocalPath)
	if !filepath.IsAbs(local) {
		abs, err := filepath.Abs(local)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve local_path: %w", err)
		}
		local = abs
	}

	// 拒绝把仓库自身或其内部路径纳入
	if insidePath(local, repo.Path) || insidePath(repo.Path, local) {
		return nil, fmt.Errorf("invalid local_path: it is the repository itself or inside it")
	}

	info, err := os.Lstat(local)
	if err != nil {
		return nil, fmt.Errorf("cannot access local_path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("local_path is already a symlink")
	}

	// 目录树内的软链接会把仓库之外的内容拖进 data/，默认拒绝
	if info.IsDir() && !req.FollowSymlinks {
		found, err := findSymlinks(local)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory: %w", err)
		}
		if len(found) > 0 {
			return nil, fmt.Errorf("directory contains symlinks (set follow_symlinks to dereference them): %s",
				strings.Join(found, ", "))
		}
	}

	repoPath := req.RepoPath
	if repoPath == "" {
		repoPath = filepath.Base(local)
	}
	repoPath, err = resolveRepoPath(repoPath)
	if err != nil {
		return nil, err
	}

	// R-3：条目之间不得重叠
	for _, e := range m.Entries {
		if repoPathsOverlap(repoPath, e.RepoPath) {
			return nil, fmt.Errorf("repo_path %q overlaps existing entry %q", repoPath, e.RepoPath)
		}
	}

	content := DataPath(repo.Path, repoPath)
	if _, err := os.Lstat(content); err == nil {
		return nil, fmt.Errorf("repo_path already exists in data/: %s", repoPath)
	}

	// 1) 内容移入仓库
	if err := movePath(local, content); err != nil {
		return nil, fmt.Errorf("failed to move content into repository: %w", err)
	}

	// 2) 原位置替换为软链接（in 链接）
	if err := os.Symlink(content, local); err != nil {
		_ = movePath(content, local)
		return nil, fmt.Errorf("failed to create in link: %w", err)
	}

	kind := model.EntryKindFile
	if info.IsDir() {
		kind = model.EntryKindDir
	}

	fingerprint := util.MachineFingerprint()
	ensureDevice(m, fingerprint)
	now := time.Now().UTC()

	entry := &model.Entry{
		ID:        newID(),
		RepoPath:  repoPath,
		Kind:      kind,
		CreatedAt: now,
		Links: []*model.Link{{
			ID:        newID(),
			Device:    fingerprint,
			LocalPath: local,
			Enabled:   true,
			CreatedAt: now,
		}},
	}
	m.Entries = append(m.Entries, entry)

	// 3) 落盘清单（校验失败则回滚文件系统）
	if err := s.save(repo, m, "entry: adopt "+repoPath); err != nil {
		os.Remove(local)
		_ = movePath(content, local)
		return nil, err
	}

	return s.Get(repoID, entry.ID)
}

// Remove 按模式移除条目。
func (s *Service) Remove(repoID, entryID, mode, linkID string) error {
	defer s.lock(repoID)()
	repo, m, err := s.load(repoID)
	if err != nil {
		return err
	}
	e := m.FindEntry(entryID)
	if e == nil {
		return fmt.Errorf("entry not found: %s", entryID)
	}

	fingerprint := util.MachineFingerprint()
	content := DataPath(repo.Path, e.RepoPath)

	switch mode {
	case RemoveModeUnlink:
		// 只删除本机链接；条目与内容都保留
		kept := e.Links[:0]
		for _, l := range e.Links {
			if l.Device == fingerprint {
				os.Remove(l.LocalPath)
				continue
			}
			kept = append(kept, l)
		}
		e.Links = kept
		if len(e.Links) > 0 {
			return s.saveConverging(repo, m, "entry: unlink "+e.RepoPath)
		}

	case RemoveModeMoveBack:
		// 把内容移回指定链接的本机路径，再移除条目。
		// 未显式指定时回退到本机上的一条链接，再回退到第一条 —— 都是确定的。
		target := e.FindLink(linkID)
		if target == nil {
			target = e.FirstLinkOn(fingerprint)
		}
		if target == nil && len(e.Links) > 0 {
			target = e.Links[0]
		}
		if target == nil {
			return fmt.Errorf("move_back requires a link to move the content to")
		}
		os.Remove(target.LocalPath)
		if _, err := os.Lstat(content); err == nil {
			if err := movePath(content, target.LocalPath); err != nil {
				return fmt.Errorf("failed to move content back: %w", err)
			}
		}

	case RemoveModePurge:
		// 删除内容；其他设备的链接会失效，由各自下次 apply 报告
		for _, l := range e.Links {
			if l.Device == fingerprint {
				os.Remove(l.LocalPath)
			}
		}
		if err := os.RemoveAll(content); err != nil {
			return fmt.Errorf("failed to remove content: %w", err)
		}

	default:
		return fmt.Errorf("invalid remove mode: %s", mode)
	}

	m.Entries = removeEntry(m.Entries, entryID)
	return s.saveConverging(repo, m, "entry: remove "+e.RepoPath)
}

// removeEntry 从列表中移除指定条目。
func removeEntry(list []*model.Entry, id string) []*model.Entry {
	kept := list[:0]
	for _, e := range list {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	return kept
}

// insidePath 判断 child 是否位于 parent 之内（含相等）。
func insidePath(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}
