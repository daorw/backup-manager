package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"

	"github.com/google/uuid"
)

func setupBackupServiceTest(t *testing.T) (*BackupService, *store.Store, *model.Repo, *SymlinkService, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	s := store.NewStore(db)
	symSvc := NewSymlinkService(s)
	gitEngine := git.NewGitEngine()
	repoMu := NewRepoMutexManager()
	// Create a temporary key manager for auth service
	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := util.NewKeyManager(keyPath)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	authSvc := NewAuthService(s, km)
	svc := NewBackupService(s, gitEngine, symSvc, authSvc, repoMu)

	repoPath := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	repo := &model.Repo{
		ID:   uuid.New().String(),
		Name: "test-repo",
		Path: repoPath,
	}
	if err := s.CreateRepo(repo); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create .links and data directories
	if err := os.MkdirAll(filepath.Join(repoPath, ".links"), 0755); err != nil {
		t.Fatalf("failed to create .links: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, "data"), 0755); err != nil {
		t.Fatalf("failed to create data: %v", err)
	}

	// Init git repo
	if err := gitEngine.Init(repoPath); err != nil {
		t.Fatalf("failed to init git: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return svc, s, repo, symSvc, cleanup
}

func TestSyncDirectoryFiles(t *testing.T) {
	svc, _, repo, symSvc, cleanup := setupBackupServiceTest(t)
	defer cleanup()

	// Create a source directory with files
	sourceDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	os.WriteFile(filepath.Join(sourceDir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "b.txt"), []byte("bbb"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "sub"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "sub", "c.txt"), []byte("ccc"), 0644)

	// Create a directory symlink
	sym, err := symSvc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "mydir",
	})
	if err != nil {
		t.Fatalf("failed to create directory symlink: %v", err)
	}

	// Initial copy should have happened during Create
	dataDir := filepath.Join(repo.Path, "data", "mydir")
	if _, err := os.Stat(filepath.Join(dataDir, "a.txt")); os.IsNotExist(err) {
		t.Fatal("expected a.txt to exist in data/ after creation")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "sub", "c.txt")); os.IsNotExist(err) {
		t.Fatal("expected sub/c.txt to exist in data/ after creation")
	}

	// Modify a source file after creation
	os.WriteFile(filepath.Join(sourceDir, "a.txt"), []byte("modified"), 0644)

	// Add a new file to the source
	os.WriteFile(filepath.Join(sourceDir, "new.txt"), []byte("new"), 0644)

	// Test syncDirectoryFiles
	stats, err := svc.syncDirectoryFiles(repo, sym)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.FilesAdded != 1 {
		t.Fatalf("expected 1 file added, got %d", stats.FilesAdded)
	}
	if stats.FilesChanged != 1 {
		t.Fatalf("expected 1 file changed, got %d", stats.FilesChanged)
	}

	// Verify the modified file was synced
	content, err := os.ReadFile(filepath.Join(dataDir, "a.txt"))
	if err != nil {
		t.Fatalf("failed to read a.txt from data: %v", err)
	}
	if string(content) != "modified" {
		t.Fatalf("expected 'modified', got '%s'", string(content))
	}

	// Verify the new file was synced
	if _, err := os.Stat(filepath.Join(dataDir, "new.txt")); os.IsNotExist(err) {
		t.Fatal("expected new.txt to exist in data/ after sync")
	}

	// Verify the unchanged file is still there
	if _, err := os.Stat(filepath.Join(dataDir, "sub", "c.txt")); os.IsNotExist(err) {
		t.Fatal("expected sub/c.txt to still exist in data/")
	}

	// Remove a file from source and sync again
	os.Remove(filepath.Join(sourceDir, "b.txt"))

	stats, err = svc.syncDirectoryFiles(repo, sym)
	if err != nil {
		t.Fatalf("unexpected error on second sync: %v", err)
	}
	if stats.FilesRemoved != 1 {
		t.Fatalf("expected 1 file removed, got %d", stats.FilesRemoved)
	}

	// Verify the removed file is gone from data
	if _, err := os.Stat(filepath.Join(dataDir, "b.txt")); !os.IsNotExist(err) {
		t.Fatal("expected b.txt to be removed from data/")
	}
}

func TestSyncDirectoryFiles_NoChanges(t *testing.T) {
	svc, _, repo, symSvc, cleanup := setupBackupServiceTest(t)
	defer cleanup()

	sourceDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	os.WriteFile(filepath.Join(sourceDir, "a.txt"), []byte("aaa"), 0644)

	sym, err := symSvc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "nodir",
	})
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Sync again with no changes
	stats, err := svc.syncDirectoryFiles(repo, sym)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.FilesAdded != 0 || stats.FilesChanged != 0 || stats.FilesRemoved != 0 {
		t.Fatalf("expected no changes for unmodified directory, got added=%d changed=%d removed=%d",
			stats.FilesAdded, stats.FilesChanged, stats.FilesRemoved)
	}
}

func TestSyncStats(t *testing.T) {
	stats := &SyncStats{}
	if stats.FilesAdded != 0 || stats.FilesChanged != 0 || stats.FilesRemoved != 0 {
		t.Fatal("new SyncStats should have zero values")
	}
}

func TestRepoMutexManager(t *testing.T) {
	m := NewRepoMutexManager()
	mu1 := m.Get("repo1")
	mu2 := m.Get("repo1")
	mu3 := m.Get("repo2")

	if mu1 != mu2 {
		t.Fatal("expected same mutex for same repo ID")
	}
	if mu1 == mu3 {
		t.Fatal("expected different mutex for different repo ID")
	}

	// Ensure it's a *sync.Mutex
	var _ *sync.Mutex = mu1
	_ = mu1
}

func TestSyncChangedFiles_DispatchByType(t *testing.T) {
	svc, _, repo, symSvc, cleanup := setupBackupServiceTest(t)
	defer cleanup()

	// Create a file-type symlink
	srcFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	_, err := symSvc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: srcFile,
		RelPath:    "test.txt",
	})
	if err != nil {
		t.Fatalf("failed to create file symlink: %v", err)
	}

	// Create a directory-type symlink
	sourceDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	os.WriteFile(filepath.Join(sourceDir, "a.txt"), []byte("aaa"), 0644)

	_, err = symSvc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "mydir",
	})
	if err != nil {
		t.Fatalf("failed to create directory symlink: %v", err)
	}

	// Modify the source file
	os.WriteFile(srcFile, []byte("modified"), 0644)

	// Add a file to directory
	os.WriteFile(filepath.Join(sourceDir, "new.txt"), []byte("new"), 0644)

	// Run syncChangedFiles - should handle both types
	total, added, err := svc.syncChangedFiles(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least 2 total changes (1 file + 1+ directory files), got %d", total)
	}
	if added != 1 {
		t.Fatalf("expected 1 file added from directory, got %d", added)
	}
}

func TestWalkSourceDir(t *testing.T) {
	t.Run("walks regular directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("aaa"), 0644)
		os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "sub", "b.txt"), []byte("bbb"), 0644)

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("follows symlinks to files", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(t.TempDir(), "target.txt")
		os.WriteFile(target, []byte("content"), 0644)
		os.Symlink(target, filepath.Join(tmpDir, "link.txt"))

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if files[0].fi.Name() != "target.txt" {
			t.Fatalf("expected target.txt, got %s", files[0].fi.Name())
		}
	})

	t.Run("follows symlinks to directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetDir := filepath.Join(t.TempDir(), "target_dir")
		os.MkdirAll(targetDir, 0755)
		os.WriteFile(filepath.Join(targetDir, "inner.txt"), []byte("inner"), 0644)
		os.Symlink(targetDir, filepath.Join(tmpDir, "link_dir"))

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if files[0].relPath != "link_dir/inner.txt" {
			t.Fatalf("expected link_dir/inner.txt, got %s", files[0].relPath)
		}
	})

	t.Run("handles nested symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(t.TempDir(), "target.txt")
		os.WriteFile(target, []byte("content"), 0644)

		link1 := filepath.Join(t.TempDir(), "link1")
		link2 := filepath.Join(tmpDir, "link2")
		os.Symlink(target, link1)
		os.Symlink(link1, link2)

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("skips hidden files", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("visible"), 0644)
		os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file (hidden excluded), got %d", len(files))
		}
	})

	t.Run("skips inaccessible files", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("aaa"), 0644)

		// Create a symlink to a non-existent target
		os.Symlink("/nonexistent/path", filepath.Join(tmpDir, "broken"))

		var files []walkResult
		err := walkSourceDir(tmpDir, tmpDir, func(relPath string, fi os.FileInfo) error {
			files = append(files, walkResult{relPath: relPath, fi: fi})
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file (broken symlink skipped), got %d", len(files))
		}
	})
}

func TestDetectCycleInBackup(t *testing.T) {
	t.Run("no cycle in regular path", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target")
		os.MkdirAll(target, 0755)
		if detectCycleInBackup(target, tmpDir) {
			t.Fatal("expected no cycle in regular path")
		}
	})

	t.Run("detects cycle when symlink points to base", func(t *testing.T) {
		tmpDir := t.TempDir()
		link := filepath.Join(tmpDir, "link")
		os.Symlink(tmpDir, link)

		if !detectCycleInBackup(link, tmpDir) {
			t.Fatal("expected cycle to be detected")
		}
	})

	t.Run("no false positive for valid symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target")
		os.MkdirAll(target, 0755)
		os.WriteFile(filepath.Join(target, "file.txt"), []byte("content"), 0644)

		link := filepath.Join(tmpDir, "link")
		os.Symlink(target, link)

		if detectCycleInBackup(link, tmpDir) {
			t.Fatal("expected no cycle for valid symlink")
		}
	})
}

func TestSyncDirectoryFiles_NestedSymlinks(t *testing.T) {
	svc, _, repo, symSvc, cleanup := setupBackupServiceTest(t)
	defer cleanup()

	// Create source directory with nested symlinks
	sourceDir := filepath.Join(t.TempDir(), "src")
	os.MkdirAll(sourceDir, 0755)

	// Regular file
	os.WriteFile(filepath.Join(sourceDir, "regular.txt"), []byte("regular"), 0644)

	// Symlink to file
	targetFile := filepath.Join(t.TempDir(), "target.txt")
	os.WriteFile(targetFile, []byte("target content"), 0644)
	os.Symlink(targetFile, filepath.Join(sourceDir, "link_file.txt"))

	// Symlink to directory
	targetDir := filepath.Join(t.TempDir(), "target_dir")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "inner.txt"), []byte("inner"), 0644)
	os.Symlink(targetDir, filepath.Join(sourceDir, "link_dir"))

	// Create directory symlink
	sym, err := symSvc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "nested_backup",
	})
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Initial sync happens during Create, so we should have all files
	// Modify a file and add a new one to test incremental sync
	os.WriteFile(filepath.Join(sourceDir, "regular.txt"), []byte("modified"), 0644)
	os.WriteFile(filepath.Join(targetFile), []byte("modified target"), 0644)

	// Sync should handle nested symlinks
	stats, err := svc.syncDirectoryFiles(repo, sym)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 2 changed files (regular.txt and link_file.txt)
	if stats.FilesChanged < 2 {
		t.Fatalf("expected at least 2 files changed, got %d", stats.FilesChanged)
	}

	// Verify files exist in data directory
	dataDir := filepath.Join(repo.Path, "data", "nested_backup")
	if _, err := os.Stat(filepath.Join(dataDir, "regular.txt")); os.IsNotExist(err) {
		t.Fatal("expected regular.txt in data/")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "link_file.txt")); os.IsNotExist(err) {
		t.Fatal("expected link_file.txt in data/")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "link_dir", "inner.txt")); os.IsNotExist(err) {
		t.Fatal("expected link_dir/inner.txt in data/")
	}
}

type walkResult struct {
	relPath string
	fi      os.FileInfo
}
