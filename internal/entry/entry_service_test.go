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

// TestAdoptMovesContentAndCreatesInLink 验证 adopt 把内容移入仓库、原位置变成软链接。
func TestAdoptMovesContentAndCreatesInLink(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")

	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if view.Unbound {
		t.Fatal("entry should be bound after adopt")
	}
	if len(view.Links) != 1 || view.Links[0].Type != string(model.LinkTypeIn) {
		t.Fatalf("expected exactly one in link, got %+v", view.Links)
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

// TestAdoptRejectsOverlappingRepoPath 验证 R-3：条目之间不得重叠。
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

// TestAddOutLinkAndSwitch 验证 out 链接与「指定新的 in」。
func TestAddOutLinkAndSwitch(t *testing.T) {
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

	// 指定新的 in：纯元数据变更，文件系统不动
	var outID, oldInID string
	for _, l := range view.Links {
		if l.Type == string(model.LinkTypeOut) {
			outID = l.ID
		} else {
			oldInID = l.ID
		}
	}
	inBefore, _ := os.Lstat(local)

	view, err = svc.Switch(repo.ID, view.ID, outID)
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	for _, l := range view.Links {
		if l.ID == outID && l.Type != string(model.LinkTypeIn) {
			t.Fatal("designated link should now be the in link")
		}
		if l.ID == oldInID && l.Type != string(model.LinkTypeOut) {
			t.Fatal("previous in link should have been demoted to out")
		}
	}
	inAfter, _ := os.Lstat(local)
	if !inBefore.ModTime().Equal(inAfter.ModTime()) {
		t.Fatal("switch should not touch the filesystem")
	}
}

// TestRemoveInLinkLeavesEntryUnbound 验证移除 in 链接后条目变为未绑定，内容不受影响。
func TestRemoveInLinkLeavesEntryUnbound(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}

	view, err = svc.RemoveLink(repo.ID, view.ID, view.Links[0].ID)
	if err != nil {
		t.Fatalf("remove in link: %v", err)
	}
	if !view.Unbound {
		t.Fatal("entry should be unbound after removing its in link")
	}
	if _, err := os.Stat(filepath.Join(repo.Path, "data", "notes.txt")); err != nil {
		t.Fatalf("content must survive: %v", err)
	}
}

// linkManifest 构造一份带一个设备与一条链接的清单，用于校验测试。
func linkManifest(linkType model.LinkType, repoPath, localPath string) *model.Manifest {
	return &model.Manifest{
		Version: manifestVersion,
		Devices: []*model.Device{{Fingerprint: "fp1", Name: "dev1"}},
		Entries: []*model.Entry{{
			ID:        "e1",
			RepoPath:  repoPath,
			Kind:      model.EntryKindFile,
			CreatedAt: time.Now(),
			Links: []*model.Link{{
				ID: "l1", Type: linkType, Device: "fp1", LocalPath: localPath, Enabled: true,
			}},
		}},
	}
}

// TestApplyRecreatesMissingLink 验证 apply 会重建缺失的本机链接，且不触碰内容。
func TestApplyRecreatesMissingLink(t *testing.T) {
	svc, repo := newTestService(t)

	local := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, local, "hello")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: "notes.txt"})
	if err != nil {
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
	if view.Unbound {
		t.Fatal("entry should stay bound")
	}
}

// TestValidateManifestEnforcesCardinality 验证 R-1：至多一个 in 链接。
func TestValidateManifestEnforcesCardinality(t *testing.T) {
	// 一个 in 链接：合法
	if err := validateManifest(linkManifest(model.LinkTypeIn, "a.txt", "/tmp/a")); err != nil {
		t.Fatalf("single in link should be valid: %v", err)
	}

	// 零个 in 链接（未绑定）：合法
	if err := validateManifest(linkManifest(model.LinkTypeOut, "a.txt", "/tmp/a")); err != nil {
		t.Fatalf("zero in links should be valid (unbound): %v", err)
	}

	// 两个 in 链接：非法
	m := linkManifest(model.LinkTypeIn, "a.txt", "/tmp/a")
	m.Entries[0].Links = append(m.Entries[0].Links, &model.Link{
		ID: "l2", Type: model.LinkTypeIn, Device: "fp1", LocalPath: "/tmp/b", Enabled: true,
	})
	if err := validateManifest(m); err == nil {
		t.Fatal("two in links should be rejected")
	}
}

// TestValidateManifestEnforcesNoOverlap 验证 R-3。
func TestValidateManifestEnforcesNoOverlap(t *testing.T) {
	m := linkManifest(model.LinkTypeIn, "docs", "/tmp/docs")
	m.Entries[0].Kind = model.EntryKindDir
	m.Entries = append(m.Entries, &model.Entry{
		ID: "e2", RepoPath: "docs/vendor", Kind: model.EntryKindDir, CreatedAt: time.Now(),
		Links: []*model.Link{{ID: "l2", Type: model.LinkTypeIn, Device: "fp1", LocalPath: "/tmp/vendor", Enabled: true}},
	})
	if err := validateManifest(m); err == nil {
		t.Fatal("nested entries should be rejected")
	}
}

// TestValidateManifestEnforcesNoLinkInsideDirectoryEntry 验证 R-4。
func TestValidateManifestEnforcesNoLinkInsideDirectoryEntry(t *testing.T) {
	m := linkManifest(model.LinkTypeIn, "docs", "/tmp/docs")
	m.Entries[0].Kind = model.EntryKindDir
	m.Entries = append(m.Entries, &model.Entry{
		ID: "e2", RepoPath: "other", Kind: model.EntryKindFile, CreatedAt: time.Now(),
		Links: []*model.Link{{ID: "l2", Type: model.LinkTypeOut, Device: "fp1",
			LocalPath: "/tmp/docs/inner", Enabled: true}},
	})
	if err := validateManifest(m); err == nil {
		t.Fatal("a link inside a directory entry should be rejected")
	}
}

// TestManifestRoundTripPreservesUnbound 验证未绑定条目经保存/加载后保持不变。
func TestManifestRoundTripPreservesUnbound(t *testing.T) {
	repoRoot := t.TempDir()
	ms := newManifestStore()

	m := linkManifest(model.LinkTypeOut, "a.txt", "/tmp/a")
	if err := ms.Save(repoRoot, m); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := newManifestStore().Load(repoRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Entries) != 1 || len(loaded.Entries[0].Links) != 1 {
		t.Fatalf("round trip lost data: %+v", loaded)
	}
	if loaded.Entries[0].InLink() != nil {
		t.Fatal("entry should still be unbound after round trip")
	}
}
