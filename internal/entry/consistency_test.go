package entry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-manager/internal/model"
	"backup-manager/internal/util"
)

// writeRawManifest 直接写入清单文件，用于模拟手工编辑后的状态。
// 末尾把 mtime 往后推一秒，确保清单缓存失效、重新从磁盘读取。
func writeRawManifest(t *testing.T, repoRoot, content string) {
	t.Helper()
	dir := filepath.Join(repoRoot, ManifestDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	path := filepath.Join(dir, ManifestFileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// hasCode 判断巡检结论中是否包含指定结论码。
func hasCode(res *AuditResult, code string) bool {
	for _, f := range res.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// adoptOne 创建单个文件的条目并返回视图。
func adoptOne(t *testing.T, svc *Service, repo *model.Repo, repoPath string) *EntryView {
	t.Helper()
	local := filepath.Join(t.TempDir(), filepath.Base(repoPath))
	writeFile(t, local, "content")
	view, err := svc.Adopt(repo.ID, &AdoptRequest{LocalPath: local, RepoPath: repoPath})
	if err != nil {
		t.Fatalf("adopt %s: %v", repoPath, err)
	}
	return view
}

// TestAuditClean 验证正常仓库巡检结论为空。
func TestAuditClean(t *testing.T) {
	svc, repo := newTestService(t)
	view := adoptOne(t, svc, repo, "notes.txt")

	out := filepath.Join(t.TempDir(), "copy.txt")
	if _, err := svc.AddLink(repo.ID, view.ID, &AddLinkRequest{LocalPath: out}); err != nil {
		t.Fatalf("add link: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !res.Clean {
		t.Fatalf("expected a clean audit, got %+v", res.Findings)
	}
	if res.EntryCnt != 1 || res.LinkCnt != 2 {
		t.Fatalf("unexpected counts: entries=%d links=%d", res.EntryCnt, res.LinkCnt)
	}
}

// TestAuditDetectsMissingLink 验证本机软链接被删除后能被检出并可修复。
func TestAuditDetectsMissingLink(t *testing.T) {
	svc, repo := newTestService(t)
	view := adoptOne(t, svc, repo, "notes.txt")
	local := view.Links[0].LocalPath

	if err := os.Remove(local); err != nil {
		t.Fatalf("remove link: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !hasCode(res, CodeLinkMissing) {
		t.Fatalf("expected %s, got %+v", CodeLinkMissing, res.Findings)
	}
	if res.Errors != 0 {
		t.Fatalf("a missing link is a warning, not an error: %+v", res.Findings)
	}

	repair, err := svc.RepairAll(repo.ID)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if repair.RepairedCount != 1 {
		t.Fatalf("expected 1 repaired link, got %+v", repair.Repaired)
	}
	if _, err := os.ReadFile(local); err != nil {
		t.Fatalf("link should be readable again: %v", err)
	}

	after, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit after repair: %v", err)
	}
	if !after.Clean {
		t.Fatalf("expected a clean audit after repair, got %+v", after.Findings)
	}
}

// TestAuditDetectsContentMissing 验证仓库内容缺失被报为 error。
func TestAuditDetectsContentMissing(t *testing.T) {
	svc, repo := newTestService(t)
	adoptOne(t, svc, repo, "notes.txt")

	if err := os.Remove(filepath.Join(repo.Path, "data", "notes.txt")); err != nil {
		t.Fatalf("remove content: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !hasCode(res, CodeContentMissing) || res.Errors == 0 {
		t.Fatalf("expected an error-level %s, got %+v", CodeContentMissing, res.Findings)
	}
}

// TestAuditDetectsSymlinkInData 验证 data/ 内出现软链接会被报为 error。
func TestAuditDetectsSymlinkInData(t *testing.T) {
	svc, repo := newTestService(t)
	adoptOne(t, svc, repo, "notes.txt")

	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeFile(t, outside, "x")
	if err := os.Symlink(outside, filepath.Join(repo.Path, "data", "sneaky.txt")); err != nil {
		t.Fatalf("symlink into data/: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !hasCode(res, CodeSymlinkInData) {
		t.Fatalf("expected %s, got %+v", CodeSymlinkInData, res.Findings)
	}
}

// TestAuditLinklessEntryIsClean 验证没有链接的条目完全合法，巡检不报任何问题。
func TestAuditLinklessEntryIsClean(t *testing.T) {
	svc, repo := newTestService(t)
	view := adoptOne(t, svc, repo, "notes.txt")

	if _, err := svc.RemoveLink(repo.ID, view.ID, view.Links[0].ID); err != nil {
		t.Fatalf("remove link: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !res.Clean {
		t.Fatalf("an entry with no links is legal: %+v", res.Findings)
	}
}

// TestAuditDetectsUnmanagedLink 验证扫描到指向 data/ 的未托管软链接。
func TestAuditDetectsUnmanagedLink(t *testing.T) {
	svc, repo := newTestService(t)
	view := adoptOne(t, svc, repo, "notes.txt")
	parent := filepath.Dir(view.Links[0].LocalPath)

	// 绕过应用，手工建一个指向同一份内容的软链接
	sneaky := filepath.Join(parent, "sneaky.txt")
	if err := os.Symlink(filepath.Join(repo.Path, "data", "notes.txt"), sneaky); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !hasCode(res, CodeUnmanagedLink) {
		t.Fatalf("expected %s, got %+v", CodeUnmanagedLink, res.Findings)
	}
}

// TestOverlappingEntriesReportedAndWritesBlocked 验证 R-2 违规可被检出、
// 无法自动修复，且新的写入会被拒绝。
func TestOverlappingEntriesAreReportedAndBlockWrites(t *testing.T) {
	svc, repo := newTestService(t)
	fp := util.MachineFingerprint()
	now := time.Now().UTC().Format(time.RFC3339)

	dir := t.TempDir()
	docsPath := filepath.Join(dir, "docs")
	vendorPath := filepath.Join(dir, "vendor")

	writeRawManifest(t, repo.Path, fmt.Sprintf(`{
  "version": 1,
  "updated_at": %q,
  "devices": [{"fingerprint": %q, "name": "dev"}],
  "entries": [
    {"id": "e1", "repo_path": "docs", "kind": "dir", "created_at": %q,
     "links": [{"id": "l1", "device": %q, "local_path": %q, "enabled": true, "created_at": %q}]},
    {"id": "e2", "repo_path": "docs/vendor", "kind": "dir", "created_at": %q,
     "links": [{"id": "l2", "device": %q, "local_path": %q, "enabled": true, "created_at": %q}]}
  ]
}`, now, fp, now, fp, docsPath, now, now, fp, vendorPath, now))

	res, err := svc.Audit(repo.ID)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if !hasCode(res, CodeOverlappingEntries) {
		t.Fatalf("expected %s, got %+v", CodeOverlappingEntries, res.Findings)
	}

	// 自动修复无法安全处理条目重叠，必须原样返回为未修复项
	repair, err := svc.RepairAll(repo.ID)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if repair.RemainingErrors == 0 {
		t.Fatalf("overlapping entries cannot be auto-repaired: %+v", repair)
	}

	// 新的写入必须被拒绝，避免把不合规状态继续固化
	if _, err := svc.AddLink(repo.ID, "e1", &AddLinkRequest{LocalPath: filepath.Join(t.TempDir(), "x")}); err == nil {
		t.Fatal("expected writes to be refused while the manifest is invalid")
	}
}
