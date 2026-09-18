package service

import (
	"os"
	"path/filepath"
	"testing"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// newContentService 创建使用临时仓库的内容服务。
func newContentService(t *testing.T) (*ContentService, *model.Repo) {
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
	if err := os.MkdirAll(filepath.Join(repoRoot, "data", "docs"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "data", "docs", "notes.md"), []byte("# hi"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ds := store.NewStore(db)
	repo := &model.Repo{ID: "r1", Name: "test", Path: repoRoot, Status: model.RepoStatusActive}
	if err := ds.CreateRepo(repo); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	return NewContentService(ds, git.NewGitEngine(), util.NewRepoMutexManager()), repo
}

// TestTreePreviewSaveRoundTrip 验证 Tree / Preview / Save 三者的路径口径一致
// （都相对于 data/），否则前端拿 tree 返回的路径去预览会解析失败。
func TestTreePreviewSaveRoundTrip(t *testing.T) {
	svc, repo := newContentService(t)

	roots, err := svc.Tree(repo.ID, "")
	if err != nil {
		t.Fatalf("tree root: %v", err)
	}
	if len(roots) != 1 || roots[0].Path != "docs" || roots[0].Type != "directory" {
		t.Fatalf("unexpected root entries: %+v", roots)
	}

	children, err := svc.Tree(repo.ID, roots[0].Path)
	if err != nil {
		t.Fatalf("tree docs: %v", err)
	}
	if len(children) != 1 || children[0].Path != "docs/notes.md" {
		t.Fatalf("unexpected child entries: %+v", children)
	}

	// 用 tree 返回的路径直接预览
	preview, err := svc.Preview(repo.ID, children[0].Path)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Content != "# hi" || !preview.Text {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	// 保存到同一路径，并保留权限
	if _, err := svc.Save(repo.ID, children[0].Path, "# updated"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repo.Path, "data", "docs", "notes.md"))
	if err != nil || string(got) != "# updated" {
		t.Fatalf("file not updated in data/: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(repo.Path, "data", "docs", "notes.md")); err != nil ||
		fi.Mode().Perm() != 0644 {
		t.Fatalf("permissions should be preserved: %v", err)
	}
}

// TestContentRejectsPathTraversal 验证路径穿越被拒绝。
func TestContentRejectsPathTraversal(t *testing.T) {
	svc, repo := newContentService(t)

	if _, err := svc.Preview(repo.ID, "../../etc/passwd"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}
