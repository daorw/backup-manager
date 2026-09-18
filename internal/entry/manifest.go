// Package entry 实现「条目 + 链接」模型：条目持有内容，链接是它的 in/out 视图。
//
// 核心不变量（R-1..R-5），加载与写入时强制校验：
//
//	R-1 每个条目至多一个 in 链接（0 个仅允许出现在新设备初始化阶段）
//	R-2 链接只绑定完整条目，绝不绑定条目的子路径
//	R-3 条目之间永不重叠（repo_path 无祖先/后代关系）
//	R-4 链接的 local_path 不得位于某个目录条目的 local_path 之内
//	R-5 同一条目的所有链接指向同一目标、kind 一致（由构造保证）
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

// Load 读取并校验仓库清单。文件不存在时返回一份空清单（不算错误）。
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

	if err := validateManifest(m); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	s.mu.Lock()
	s.cache[repoRoot] = &cachedManifest{manifest: m, mtime: info.ModTime(), size: info.Size()}
	s.mu.Unlock()
	return m, nil
}

// Save 校验并原子写入清单（tmp → fsync → rename），随后刷新缓存。
func (s *manifestStore) Save(repoRoot string, m *model.Manifest) error {
	if err := validateManifest(m); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

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

// validateManifest 校验清单的 R-1/R-3/R-4 及引用完整性。
func validateManifest(m *model.Manifest) error {
	entryIDs := make(map[string]bool, len(m.Entries))
	linkIDs := make(map[string]bool)

	for _, e := range m.Entries {
		if e.ID == "" || e.RepoPath == "" {
			return fmt.Errorf("entry is missing id or repo_path")
		}
		if entryIDs[e.ID] {
			return fmt.Errorf("duplicate entry id %q", e.ID)
		}
		entryIDs[e.ID] = true

		// R-2：链接只能挂在条目上，repo_path 必须是干净的非空相对路径
		if e.RepoPath != filepath.ToSlash(filepath.Clean(e.RepoPath)) ||
			strings.HasPrefix(e.RepoPath, "../") || e.RepoPath == "." {
			return fmt.Errorf("entry %q has an invalid repo_path", e.RepoPath)
		}

		// R-1：至多一个 in 链接。0 个是合法状态（新设备初始化时未绑定）。
		inCount := 0
		for _, l := range e.Links {
			if linkIDs[l.ID] {
				return fmt.Errorf("duplicate link id %q", l.ID)
			}
			linkIDs[l.ID] = true

			if l.Type == model.LinkTypeIn {
				inCount++
			} else if l.Type != model.LinkTypeOut {
				return fmt.Errorf("link %q has an unknown type %q", l.ID, l.Type)
			}
			if l.LocalPath == "" || l.Device == "" {
				return fmt.Errorf("link %q is missing local_path or device", l.ID)
			}
			// 引用完整性：链接必须指向已登记的设备
			if m.FindDevice(l.Device) == nil {
				return fmt.Errorf("link %q references an unknown device %q", l.ID, l.Device)
			}
		}
		if inCount > 1 {
			return fmt.Errorf("entry %q has %d in links (R-1 allows at most one)", e.RepoPath, inCount)
		}
	}

	// R-3：条目之间不得互为祖先/后代
	for i := 0; i < len(m.Entries); i++ {
		for j := i + 1; j < len(m.Entries); j++ {
			if repoPathsOverlap(m.Entries[i].RepoPath, m.Entries[j].RepoPath) {
				return fmt.Errorf("entries %q and %q overlap (R-3 forbids nested entries)",
					m.Entries[i].RepoPath, m.Entries[j].RepoPath)
			}
		}
	}

	// R-4：链接的 local_path 不得位于某个目录条目的 local_path 之内
	for _, dir := range m.Entries {
		if dir.Kind != model.EntryKindDir {
			continue
		}
		for _, owner := range dir.Links {
			for _, e := range m.Entries {
				for _, l := range e.Links {
					if l == owner {
						continue
					}
					if localPathsOverlap(l.LocalPath, owner.LocalPath) {
						return fmt.Errorf("link %q (%s) is inside directory entry %q (R-4)",
							l.LocalPath, e.RepoPath, dir.RepoPath)
					}
				}
			}
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
