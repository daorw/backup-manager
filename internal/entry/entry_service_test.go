package entry

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// newTestService 创建使用临时仓库的条目服务。
func newTestService(t *testing.T) (*Service, *model.Repo) {
	t.Helper()

	db, err := store.OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	ds := store.NewStore(db)
	repo := &model.Repo{ID: "r1", Name: "test", Path: repoRoot, Status: model.RepoStatusActive}
	if err := ds.CreateRepo(repo); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	return NewService(ds, git.NewGitEngine(), util.NewRepoMutexManager()), repo
}

// writeFile 写入一个测试文件。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// TestAdoptMovesContentAndCreatesLink 验证 adopt 把内容移入仓库、原位置变成软链接。
func TestAdoptMovesContentAndCreatesLink(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")

	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if len(view.Links) != 1 {
		t.Fatalf("adopt should create exactly one link, got %+v", view.Links)
	}
	if view.Links[0].LocalPath != local || !view.Links[0].IsCurrent {
		t.Fatalf("unexpected link: %+v", view.Links[0])
	}

	// 内容已移入 data/
	content, err := os.ReadFile(filepath.Join(repo.Path, "data", "notes.txt"))
	if err != nil || string(content) != "hello" {
		t.Fatalf("content not moved into data/: %v", err)
	}

	// 原位置变成指向 data/ 的软链接
	if fi, err := os.Lstat(local); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("local path should be a symlink, got %v", err)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "hello" {
		t.Fatalf("reading through the symlink failed: %v", err)
	}

	// 清单已落盘
	if _, err := os.Stat(ManifestPath(repo.Path)); err != nil {
		t.Fatalf("manifest should exist: %v", err)
	}
}

// TestAdoptRejectsRepoPathOutsideData 验证新建条目时内容不得落到 data/ 之外。
func TestAdoptRejectsRepoPathOutsideData(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")

	for _, p := range []string{
		"../../escape.txt",
		"/etc/passwd",
		"a/../../escape.txt",
		"..\\escape.txt",
	} {
		if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: p}); err == nil {
			t.Fatalf("expected repo_path %q to be rejected", p)
		}
	}
}

// TestAdoptRejectsRepoPathEscapingViaSymlink 验证 data/ 内的软链接也不能把内容带出仓库。
func TestAdoptRejectsRepoPathEscapingViaSymlink(t *testing.T) {
	svc, repo := newTestService(t)

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repo.Path, "data", "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")

	if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "escape/notes.txt"}); err == nil {
		t.Fatal("expected repo_path escaping data/ through a symlink to be rejected")
	}
}

// TestAdoptRejectsOverlappingRepoPath 验证 R-2：条目之间不得重叠。
func TestAdoptRejectsOverlappingRepoPath(t *testing.T) {
	svc, repo := newTestService(t)

	dir := filepath.Join(t.TempDir(), "docs")
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: dir, RepoPath: "docs"}); err != nil {
		t.Fatalf("adopt dir: %v", err)
	}

	other := filepath.Join(t.TempDir(), "b.txt")
	writeFile(t, other, "b")
	if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: other, RepoPath: "docs/b.txt"}); err == nil {
		t.Fatal("expected overlapping repo_path to be rejected")
	}
}

// TestAddLink 验证为条目添加链接（所有链接完全等价）。
func TestAddLink(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "copy.txt")
	view, err = svc.AddLink(repo.ID, view.ID, &AddLinkRequest{LocalPath: outPath})
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	if len(view.Links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(view.Links))
	}

	// 两条链接等价：都指向同一份内容，通过任一条读到的东西相同
	for _, l := range view.Links {
		if got, err := os.ReadFile(l.LocalPath); err != nil || string(got) != "hello" {
			t.Fatalf("link %s should resolve to the content: %v", l.LocalPath, err)
		}
	}
	// 内容只有一份，且就在 data/ 下
	if _, err := os.Stat(filepath.Join(repo.Path, "data", "notes.txt")); err != nil {
		t.Fatalf("content must live in data/: %v", err)
	}
}

// TestRemoveLastLinkKeepsEntryAndContent 验证移除最后一条链接后条目与内容都保留。
func TestRemoveLastLinkKeepsEntryAndContent(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}

	view, err = svc.RemoveLink(repo.ID, view.ID, view.Links[0].ID)
	if err != nil {
		t.Fatalf("remove link: %v", err)
	}
	if len(view.Links) != 0 {
		t.Fatalf("expected no links left, got %+v", view.Links)
	}
	// 没有链接的条目依然是被备份对象，内容也还在
	if _, err := os.Stat(filepath.Join(repo.Path, "data", "notes.txt")); err != nil {
		t.Fatalf("content must survive: %v", err)
	}
	entries, err := svc.List(repo.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the entry must stay: %v %+v", err, entries)
	}
}

// linkManifest 构造一份带一个设备与一条链接的清单，用于校验测试。
func linkManifest(repoPath, localPath string) *model.Manifest {
	return &model.Manifest{
		Version: manifestVersion,
		Devices: []*model.Device{{Fingerprint: "fp1", Name: "dev1"}},
		Entries: []*model.Entry{{
			ID:        "e1",
			RepoPath:  repoPath,
			Kind:      model.EntryKindFile,
			CreatedAt: time.Now(),
			Links: []*model.Link{{
				ID: "l1", Device: "fp1", LocalPath: localPath, Enabled: true,
			}},
		}},
	}
}

// TestApplyRecreatesMissingLink 验证 apply 会重建缺失的本机链接，且不触碰内容。
func TestApplyRecreatesMissingLink(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"}); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	// 模拟用户删掉本机链接
	if err := os.Remove(local); err != nil {
		t.Fatalf("remove link: %v", err)
	}

	views, err := svc.List(repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := views[0].Links[0].State; got != string(model.LinkStateMissing) {
		t.Fatalf("expected missing state, got %s", got)
	}

	// dry run 只出计划，不落地
	plan, err := svc.Apply(repo.ID, "", true)
	if err != nil {
		t.Fatalf("apply dry run: %v", err)
	}
	if len(plan.Created) != 1 {
		t.Fatalf("expected 1 planned create, got %d", len(plan.Created))
	}
	if _, err := os.Lstat(local); err == nil {
		t.Fatal("dry run must not create the link")
	}

	result, err := svc.Apply(repo.ID, "", false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("expected 1 created link, got %d", len(result.Created))
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "hello" {
		t.Fatalf("link should be recreated and readable: %v", err)
	}
}

// TestReadoptMovesReplacedContentIntoRepository 验证 replaced 状态下
// 把本机真实文件重新纳入仓库并恢复软链接。
func TestReadoptMovesReplacedContentIntoRepository(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "old")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	linkID := view.Links[0].ID

	// 模拟应用的原子写：软链接被替换为真实文件
	if err := os.Remove(local); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	writeFile(t, local, "new")

	views, err := svc.List(repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := views[0].Links[0].State; got != string(model.LinkStateReplaced) {
		t.Fatalf("expected replaced state, got %s", got)
	}

	if _, err := svc.Readopt(repo.ID, view.ID, linkID); err != nil {
		t.Fatalf("readopt: %v", err)
	}

	// 新内容已进入仓库
	content, err := os.ReadFile(filepath.Join(repo.Path, "data", "notes.txt"))
	if err != nil || string(content) != "new" {
		t.Fatalf("repository content should hold the new content: %v", err)
	}
	// 本机路径恢复为软链接
	if fi, err := os.Lstat(local); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("local path should be a symlink again: %v", err)
	}

	after, err := svc.List(repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := after[0].Links[0].State; got != string(model.LinkStateOK) {
		t.Fatalf("expected ok state after readopt, got %s", got)
	}
}

// TestDetachUnlinkThenApplyRestores 验证卸载本机后可以一键重新挂载。
func TestDetachUnlinkThenApplyRestores(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	out := filepath.Join(t.TempDir(), "copy.txt")
	if _, err := svc.AddLink(repo.ID, view.ID, &AddLinkRequest{LocalPath: out}); err != nil {
		t.Fatalf("add link: %v", err)
	}

	result, err := svc.Detach(repo.ID, "", DetachModeUnlink)
	if err != nil {
		t.Fatalf("detach: %v", err)
	}
	if len(result.Removed) != 2 {
		t.Fatalf("expected 2 removed links, got %+v", result.Removed)
	}
	for _, p := range []string{local, out} {
		if _, err := os.Lstat(p); err == nil {
			t.Fatalf("local symlink should be gone: %s", p)
		}
	}
	// 内容与定义都保留
	if _, err := os.Stat(filepath.Join(repo.Path, "data", "notes.txt")); err != nil {
		t.Fatalf("repository content must survive a detach: %v", err)
	}

	// 定义仍然启用，因此一次 Apply 即可重新挂载
	if _, err := svc.Apply(repo.ID, "", false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "hello" {
		t.Fatalf("apply should recreate the in link: %v", err)
	}
}

// TestDetachKeepLeavesFilesystemAlone 验证 keep 模式不动文件系统。
func TestDetachKeepLeavesFilesystemAlone(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	if _, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"}); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	result, err := svc.Detach(repo.ID, "", DetachModeKeep)
	if err != nil {
		t.Fatalf("detach: %v", err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("keep mode must not remove anything: %+v", result.Removed)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "hello" {
		t.Fatalf("local path must be untouched: %v", err)
	}
}

// TestValidateManifestEnforcesNoOverlap 验证 R-2。
func TestValidateManifestEnforcesNoOverlap(t *testing.T) {
	m := linkManifest("docs", "/tmp/docs")
	m.Entries[0].Kind = model.EntryKindDir
	m.Entries = append(m.Entries, &model.Entry{
		ID: "e2", RepoPath: "docs/vendor", Kind: model.EntryKindDir, CreatedAt: time.Now(),
		Links: []*model.Link{{ID: "l2", Device: "fp1", LocalPath: "/tmp/vendor", Enabled: true}},
	})
	if err := validateManifest(m); err == nil {
		t.Fatal("nested entries should be rejected")
	}
}

// TestValidateManifestEnforcesNoLinkInsideDirectoryEntry 验证 R-3。
func TestValidateManifestEnforcesNoLinkInsideDirectoryEntry(t *testing.T) {
	m := linkManifest("docs", "/tmp/docs")
	m.Entries[0].Kind = model.EntryKindDir
	m.Entries = append(m.Entries, &model.Entry{
		ID: "e2", RepoPath: "other", Kind: model.EntryKindFile, CreatedAt: time.Now(),
		Links: []*model.Link{{ID: "l2", Device: "fp1",
			LocalPath: "/tmp/docs/inner", Enabled: true}},
	})
	if err := validateManifest(m); err == nil {
		t.Fatal("a link inside a directory entry should be rejected")
	}
}

// TestValidateManifestAllowsLinklessEntry 验证没有链接的条目是合法的。
func TestValidateManifestAllowsLinklessEntry(t *testing.T) {
	m := linkManifest("a.txt", "/tmp/a")
	m.Entries[0].Links = []*model.Link{}
	if err := validateManifest(m); err != nil {
		t.Fatalf("an entry with no links should be valid: %v", err)
	}
}

// TestManifestRoundTripKeepsLinklessEntry 验证无链接条目的清单经保存/加载后保持不变。
func TestManifestRoundTripKeepsLinklessEntry(t *testing.T) {
	repoRoot := t.TempDir()
	ms := newManifestStore()

	m := linkManifest("a.txt", "/tmp/a")
	m.Entries[0].Links = []*model.Link{} // 无链接条目也合法
	if err := ms.Save(repoRoot, m); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := newManifestStore().Load(repoRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Entries) != 1 || len(loaded.Entries[0].Links) != 0 {
		t.Fatalf("round trip lost data: %+v", loaded)
	}
}
