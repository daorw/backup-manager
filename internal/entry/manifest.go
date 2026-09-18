// Package entry 实现「条目 + 链接」模型：条目持有内容，链接是指向该内容的软链接视图。
//
// 核心不变量（R-1..R-3），加载与写入时强制校验：
//
//	R-1 链接只绑定完整条目，绝不绑定条目的子路径
//	R-2 条目之间永不重叠（repo_path 无祖先/后代关系）
//	R-3 链接的 local_path 不得位于某个目录条目的 local_path 之内
//
// 同一条目的所有链接指向同一目标，这是构造特性而非需要校验的不变量。
package entry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backup-manager/internal/model"

	"github.com/google/uuid"
)

const (
	// ManifestDirName 清单目录名（位于仓库根目录下）
	ManifestDirName = ".backup-manager"
	// ManifestFileName 清单文件名
	ManifestFileName = "manifest.json"
	manifestVersion  = 1
)

// ManifestPath 返回仓库清单文件的绝对路径。
func ManifestPath(repoRoot string) string {
	return filepath.Join(repoRoot, ManifestDirName, ManifestFileName)
}

// DataPath 返回条目在仓库内的绝对路径。
func DataPath(repoRoot, repoPath string) string {
	return filepath.Join(repoRoot, "data", filepath.FromSlash(repoPath))
}

// newID 生成 16 位十六进制短 id（条目与链接共用）。
func newID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
}

// manifestStore 负责清单的加载、进程内缓存与原子写。
// 缓存以文件 mtime + size 判断是否被外部改动（git pull、人工编辑等）。
type manifestStore struct {
	mu    sync.Mutex
	cache map[string]*cachedManifest
}

type cachedManifest struct {
	manifest *model.Manifest
	mtime    time.Time
	size     int64
}

// newManifestStore 创建清单存储。
func newManifestStore() *manifestStore {
	return &manifestStore{cache: make(map[string]*cachedManifest)}
}

// Load 读取仓库清单。文件不存在时返回一份空清单（不算错误）。
//
// 这里**不做不变量校验**：手工编辑出错时仓库仍应可打开、可在巡检里看到问题，
// 而不是每个接口都失败。校验只拦写入（Save），并且巡检会把问题列出来。
func (s *manifestStore) Load(repoRoot string) (*model.Manifest, error) {
	path := ManifestPath(repoRoot)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyManifest(), nil
		}
		return nil, fmt.Errorf("failed to stat manifest: %w", err)
	}

	s.mu.Lock()
	if c, ok := s.cache[repoRoot]; ok && c.mtime.Equal(info.ModTime()) && c.size == info.Size() {
		m := c.manifest
		s.mu.Unlock()
		return m, nil
	}
	s.mu.Unlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	m := emptyManifest()
	if err := json.Unmarshal(raw, m); err != nil {
		return nil, fmt.Errorf("manifest is not valid JSON (resolve the conflict or restore the file): %w", err)
	}
	assignMissingIDs(m)

	s.mu.Lock()
	s.cache[repoRoot] = &cachedManifest{manifest: m, mtime: info.ModTime(), size: info.Size()}
	s.mu.Unlock()
	return m, nil
}

// Save 校验不变量后原子写入清单。
func (s *manifestStore) Save(repoRoot string, m *model.Manifest) error {
	if err := validateManifest(m); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	return s.SaveUnchecked(repoRoot, m)
}

// SaveUnchecked 跳过不变量校验直接原子写入（tmp → fsync → rename），随后刷新缓存。
//
// 仅供「只会减少违规」的收敛路径使用（巡检修复、移除链接/条目）：
// 这些操作必须能在清单已经不合规时依然落盘，否则用户会陷入无法修复的死结。
func (s *manifestStore) SaveUnchecked(repoRoot string, m *model.Manifest) error {
	m.Version = manifestVersion
	m.UpdatedAt = time.Now().UTC()

	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Join(repoRoot, ManifestDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create manifest dir: %w", err)
	}

	path := ManifestPath(repoRoot)
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("failed to create temp manifest: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("failed to write temp manifest: %w", err)
	}
	// fsync 后再 rename，避免断电留下半截文件
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("failed to sync temp manifest: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to close temp manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to replace manifest: %w", err)
	}

	if info, err := os.Stat(path); err == nil {
		s.mu.Lock()
		s.cache[repoRoot] = &cachedManifest{manifest: m, mtime: info.ModTime(), size: info.Size()}
		s.mu.Unlock()
	}
	return nil
}

// emptyManifest 返回一份空的 v1 清单。
func emptyManifest() *model.Manifest {
	return &model.Manifest{
		Version: manifestVersion,
		Devices: []*model.Device{},
		Entries: []*model.Entry{},
	}
}

// assignMissingIDs 为手工编辑的条目/链接补全缺失的 id。
func assignMissingIDs(m *model.Manifest) {
	for _, e := range m.Entries {
		if e.ID == "" {
			e.ID = newID()
		}
		if e.Links == nil {
			e.Links = []*model.Link{}
		}
		for _, l := range e.Links {
			if l.ID == "" {
				l.ID = newID()
			}
		}
	}
	if m.Devices == nil {
		m.Devices = []*model.Device{}
	}
	if m.Entries == nil {
		m.Entries = []*model.Entry{}
	}
}

// checkManifest 校验清单的结构、引用完整性与 R-1..R-3 不变量，
// 返回**全部**问题（不短路），供写入校验与巡检共用。
func checkManifest(m *model.Manifest) []Finding {
	var findings []Finding
	add := func(code, repoPath, linkID, localPath, format string, args ...any) {
		findings = append(findings, Finding{
			Code:      code,
			Severity:  SeverityError,
			RepoPath:  repoPath,
			LinkID:    linkID,
			LocalPath: localPath,
			Message:   fmt.Sprintf(format, args...),
		})
	}

	entryIDs := make(map[string]bool, len(m.Entries))
	linkIDs := make(map[string]bool)

	for _, e := range m.Entries {
		if e.ID == "" || e.RepoPath == "" {
			add(CodeInvalidEntry, e.RepoPath, "", "", "entry is missing id or repo_path")
			continue
		}
		if entryIDs[e.ID] {
			add(CodeInvalidEntry, e.RepoPath, "", "", "duplicate entry id: %q", e.ID)
		}
		entryIDs[e.ID] = true

		// R-1：链接只能绑定完整条目，repo_path 必须是干净的非空相对路径
		if e.RepoPath != filepath.ToSlash(filepath.Clean(e.RepoPath)) ||
			strings.HasPrefix(e.RepoPath, "../") || e.RepoPath == "." {
			add(CodeInvalidEntry, e.RepoPath, "", "", "invalid repo_path")
		}

		for _, l := range e.Links {
			if linkIDs[l.ID] {
				add(CodeInvalidLink, e.RepoPath, l.ID, l.LocalPath, "duplicate link id: %q", l.ID)
			}
			linkIDs[l.ID] = true

			if l.LocalPath == "" || l.Device == "" {
				add(CodeInvalidLink, e.RepoPath, l.ID, l.LocalPath, "link is missing local_path or device")
			}
			// 引用完整性：链接必须指向已登记的设备
			if m.FindDevice(l.Device) == nil {
				add(CodeUnknownDevice, e.RepoPath, l.ID, l.LocalPath, "link references an unregistered device: %q", l.Device)
			}
		}
	}

	// R-2：条目之间不得互为祖先/后代
	for i := 0; i < len(m.Entries); i++ {
		for j := i + 1; j < len(m.Entries); j++ {
			if repoPathsOverlap(m.Entries[i].RepoPath, m.Entries[j].RepoPath) {
				add(CodeOverlappingEntries, m.Entries[i].RepoPath, "", "",
					"overlaps entry %q (R-2: entries must not nest)", m.Entries[j].RepoPath)
			}
		}
	}

	// R-3：链接的 local_path 不得位于某个目录条目的 local_path 之内。
	// 已禁用的链接不参与判定 —— 禁用正是巡检修复 R-3 违规的收敛手段。
	for _, dir := range m.Entries {
		if dir.Kind != model.EntryKindDir {
			continue
		}
		for _, owner := range dir.Links {
			if !owner.Enabled {
				continue
			}
			for _, e := range m.Entries {
				for _, l := range e.Links {
					if l == owner || !l.Enabled {
						continue
					}
					if localPathsOverlap(l.LocalPath, owner.LocalPath) {
						add(CodeNestedLink, e.RepoPath, l.ID, l.LocalPath,
							"is inside the local path of directory entry %q (R-3)", dir.RepoPath)
					}
				}
			}
		}
	}
	return findings
}

// validateManifest 供写入路径使用：存在 error 级问题时返回第一个错误。
func validateManifest(m *model.Manifest) error {
	for _, f := range checkManifest(m) {
		if f.Severity == SeverityError {
			if f.RepoPath != "" {
				return fmt.Errorf("%s: %s", f.RepoPath, f.Message)
			}
			return fmt.Errorf("%s", f.Message)
		}
	}
	return nil
}

// repoPathsOverlap 判断两个仓库相对路径是否存在祖先/后代关系。
func repoPathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// localPathsOverlap 判断 a 是否等于 b 或位于 b 之内。
func localPathsOverlap(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator))
}

// uniqueDeviceName 在设备列表中生成不重名的设备名。
func uniqueDeviceName(m *model.Manifest, base string) string {
	used := make(map[string]bool, len(m.Devices))
	for _, d := range m.Devices {
		used[d.Name] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !used[name] {
			return name
		}
	}
}
