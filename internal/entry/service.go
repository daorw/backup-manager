package entry

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// Service 实现条目、链接与设备的全部业务逻辑。
// 所有会改动文件系统的操作都持有仓库级互斥锁，避免与备份、回滚并发。
type Service struct {
	dataStore *store.Store
	gitEngine *git.GitEngine
	repoMu    *util.RepoMutexManager
	manifests *manifestStore
}

// NewService 创建条目服务。
func NewService(s *store.Store, g *git.GitEngine, repoMu *util.RepoMutexManager) *Service {
	return &Service{
		dataStore: s,
		gitEngine: g,
		repoMu:    repoMu,
		manifests: newManifestStore(),
	}
}

// ---------- 对外视图 ----------

// LinkView 链接视图，含按需计算的派生字段。
type LinkView struct {
	ID         string    `json:"id"`
	EntryID    string    `json:"entry_id"`
	Device     string    `json:"device"`
	DeviceName string    `json:"device_name,omitempty"`
	LocalPath  string    `json:"local_path"`
	Enabled    bool      `json:"enabled"`
	IsCurrent  bool      `json:"is_current"` // 派生：属于当前设备
	State      string    `json:"state"`
	StateNote  string    `json:"state_note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// EntryView 条目视图，含其全部链接。
type EntryView struct {
	ID        string      `json:"id"`
	RepoPath  string      `json:"repo_path"`
	Kind      string      `json:"kind"`
	CreatedAt time.Time   `json:"created_at"`
	Links     []*LinkView `json:"links"`
}

// DeviceView 设备视图。
type DeviceView struct {
	Fingerprint string     `json:"fingerprint"`
	Name        string     `json:"name"`
	Hostname    string     `json:"hostname,omitempty"`
	OS          string     `json:"os,omitempty"`
	IsCurrent   bool       `json:"is_current"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	LinkCount   int        `json:"link_count"`
}

// CurrentDeviceInfo 当前机器的标识信息。
type CurrentDeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Name        string `json:"name"`
}

// ApplyAction 单条链接的收敛动作结果。
type ApplyAction struct {
	EntryID   string `json:"entry_id"`
	LinkID    string `json:"link_id"`
	RepoPath  string `json:"repo_path"`
	LocalPath string `json:"local_path"`
	Action    string `json:"action"` // create / repair / skip / conflict / orphan
	Reason    string `json:"reason,omitempty"`
}

// ApplyResult 设备收敛结果。
type ApplyResult struct {
	Device      string        `json:"device"`
	Created     []ApplyAction `json:"created"`
	Repaired    []ApplyAction `json:"repaired"`
	Skipped     []ApplyAction `json:"skipped"`
	Conflicts   []ApplyAction `json:"conflicts"`
	Orphans     []ApplyAction `json:"orphans"`
	DryRun      bool          `json:"dry_run"`
	CompletedAt time.Time     `json:"completed_at"`
}

// ---------- 内部辅助 ----------

// getRepo 读取仓库记录。
func (s *Service) getRepo(repoID string) (*model.Repo, error) {
	return s.dataStore.GetRepo(repoID)
}

// lock 获取仓库级互斥锁。
func (s *Service) lock(repoID string) func() {
	mu := s.repoMu.Get(repoID)
	mu.Lock()
	return mu.Unlock
}

// load 读取仓库记录与清单。
func (s *Service) load(repoID string) (*model.Repo, *model.Manifest, error) {
	repo, err := s.getRepo(repoID)
	if err != nil {
		return nil, nil, err
	}
	m, err := s.manifests.Load(repo.Path)
	if err != nil {
		return nil, nil, err
	}
	return repo, m, nil
}

// save 写入清单并提交，使定义随仓库一起传输。
func (s *Service) save(repo *model.Repo, m *model.Manifest, msg string) error {
	if err := s.manifests.Save(repo.Path, m); err != nil {
		return err
	}
	s.commitManifest(repo.Path, msg)
	return nil
}

// saveConverging 与 save 相同，但跳过不变量校验。
//
// 只供「只会减少违规」的收敛路径使用（巡检修复、移除链接/条目）。若这些操作也被校验拦住，
// 清单一旦被手工编辑成不合规状态，用户就再也无法通过应用把它修回来。
func (s *Service) saveConverging(repo *model.Repo, m *model.Manifest, msg string) error {
	if err := s.manifests.SaveUnchecked(repo.Path, m); err != nil {
		return err
	}
	s.commitManifest(repo.Path, msg)
	return nil
}

// commitManifest 把清单变更提交进 Git，使定义随仓库一起传输。
// 仓库尚未初始化 Git 时直接跳过；提交失败只记日志 —— 清单文件已经落盘，
// 下次备份的 git add -A 会兜住它。
func (s *Service) commitManifest(repoRoot, msg string) {
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return
	}
	rel := filepath.ToSlash(filepath.Join(ManifestDirName, ManifestFileName))
	if err := s.gitEngine.Add(repoRoot, rel); err != nil {
		log.Printf("[entry] git add manifest failed: %v", err)
		return
	}
	if err := s.gitEngine.Commit(repoRoot, msg); err != nil {
		log.Printf("[entry] git commit manifest skipped: %v", err)
	}
}

// ensureDevice 确保当前设备已登记在清单中（仅改内存，由后续保存落盘）。
func ensureDevice(m *model.Manifest, fingerprint string) *model.Device {
	if d := m.FindDevice(fingerprint); d != nil {
		return d
	}
	d := &model.Device{
		Fingerprint: fingerprint,
		Name:        uniqueDeviceName(m, util.Hostname()),
		Hostname:    util.Hostname(),
		OS:          runtime.GOOS,
	}
	m.Devices = append(m.Devices, d)
	return d
}

// createSymlink 创建指向 data/<repoPath> 的软链接。
// in 与 out 的物理形态完全相同 —— 这正是「in 是 out 的特例」的落地。
func createSymlink(localPath, repoRoot, repoPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	if err := os.Symlink(DataPath(repoRoot, repoPath), localPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}
	return nil
}

// movePath 把 src 移动到 dst。跨文件系统时降级为「复制 + 校验大小 + 删除」。
// 只有在复制内容校验通过后才会删除源，确保不会丢数据。
func movePath(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if info.IsDir() {
		if err := util.CopyDir(src, dst); err != nil {
			return fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := util.CopyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}

	if err := verifySameSize(src, dst, info.IsDir()); err != nil {
		os.RemoveAll(dst)
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		os.RemoveAll(dst)
		return fmt.Errorf("failed to remove source after copy: %w", err)
	}
	return nil
}

// verifySameSize 校验 dst 与 src 内容大小一致（目录则递归比较）。
func verifySameSize(src, dst string, isDir bool) error {
	if !isDir {
		s, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("failed to stat source: %w", err)
		}
		d, err := os.Stat(dst)
		if err != nil {
			return fmt.Errorf("failed to stat copy: %w", err)
		}
		if s.Size() != d.Size() {
			return fmt.Errorf("copy verification failed: size mismatch (%d != %d)", s.Size(), d.Size())
		}
		return nil
	}

	// 目录：逐文件比对大小
	srcFiles := map[string]int64{}
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		srcFiles[rel] = fi.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk source: %w", err)
	}
	dstFiles := map[string]int64{}
	err = filepath.Walk(dst, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dst, p)
		dstFiles[rel] = fi.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk copy: %w", err)
	}
	if len(srcFiles) != len(dstFiles) {
		return fmt.Errorf("copy verification failed: file count mismatch (%d != %d)", len(srcFiles), len(dstFiles))
	}
	for rel, size := range srcFiles {
		if dstFiles[rel] != size {
			return fmt.Errorf("copy verification failed for %q", rel)
		}
	}
	return nil
}

// findSymlinks 扫描目录树中的软链接，用于拒绝把外部内容拖进 data/。
func findSymlinks(root string) ([]string, error) {
	var found []string
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			found = append(found, p)
		}
		return nil
	})
	return found, err
}

// resolveRepoPath 校验用户给出的仓库相对路径，返回斜杠分隔的干净形式。
//
// 起点永远是仓库的 data/ 目录：绝对路径与任何 ".." 段都被拒绝，
// 因此条目内容不可能落到 data/ 之外。
func resolveRepoPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("repo_path is required")
	}
	// 先统一分隔符，避免 "a\..\b" 这类写法绕过 filepath.Clean
	normalized := strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("invalid repo_path %q: must be relative to data/", p)
	}
	for _, seg := range strings.Split(normalized, "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid repo_path %q: must stay inside data/", p)
		}
	}
	cleaned := filepath.ToSlash(filepath.Clean(normalized))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid repo_path %q: must stay inside data/", p)
	}
	return cleaned, nil
}

// resolveRepoPathIn 在 resolveRepoPath 之上再解析一次真实路径：
// 结果必须仍位于 <repoRoot>/data 之内，防止借 data/ 下的软链接跳出仓库。
//
// 新建条目（即新建备份文件/目录）是唯一会往 data/ 写入新内容的入口，
// 所以这里必须做最严的包含性校验。
func resolveRepoPathIn(repoRoot, p string) (string, error) {
	cleaned, err := resolveRepoPath(p)
	if err != nil {
		return "", err
	}
	dataRoot := filepath.Join(repoRoot, "data")
	if _, err := util.SafeResolve(dataRoot, cleaned); err != nil {
		// 带上 "invalid"，让 respondError 归为 400（这是用户输入问题，不是服务端故障）
		return "", fmt.Errorf("invalid repo_path %q: must stay inside the repository data/ directory", p)
	}
	return cleaned, nil
}
