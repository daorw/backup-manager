package service

import (
	"os"
	"path/filepath"
	"testing"

	"backup-manager/internal/model"
	"backup-manager/internal/store"

	"github.com/google/uuid"
)

func setupSymlinkServiceTest(t *testing.T) (*SymlinkService, *store.Store, *model.Repo, func()) {
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
	svc := NewSymlinkService(s)

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

	cleanup := func() {
		db.Close()
	}

	return svc, s, repo, cleanup
}

func TestListDirEntries(t *testing.T) {
	svc, _, repo, cleanup := setupSymlinkServiceTest(t)
	defer cleanup()

	// Create a source directory with files
	sourceDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Create some files and subdirectories
	os.WriteFile(filepath.Join(sourceDir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "b.txt"), []byte("bbb"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "sub1"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "sub1", "c.txt"), []byte("ccc"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "sub2"), 0755)

	// Create a directory symlink in the repo
	sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "mydir",
	})
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	t.Run("list root entries of directory symlink", func(t *testing.T) {
		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have 4 entries: a.txt, b.txt, sub1, sub2
		if len(entries) != 4 {
			t.Fatalf("expected 4 entries, got %d", len(entries))
		}

		// First entries should be directories
		if entries[0].Type != "directory" {
			t.Fatalf("expected first entry to be a directory, got %s", entries[0].Type)
		}
		if entries[1].Type != "directory" {
			t.Fatalf("expected second entry to be a directory, got %s", entries[1].Type)
		}

		// Last entries should be files
		if entries[2].Type != "file" {
			t.Fatalf("expected third entry to be a file, got %s", entries[2].Type)
		}
		if entries[3].Type != "file" {
			t.Fatalf("expected fourth entry to be a file, got %s", entries[3].Type)
		}
	})

	t.Run("list subdirectory entries", func(t *testing.T) {
		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "sub1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Name != "c.txt" {
			t.Fatalf("expected c.txt, got %s", entries[0].Name)
		}
		if entries[0].Type != "file" {
			t.Fatalf("expected type 'file', got %s", entries[0].Type)
		}
	})

	t.Run("non-existent subpath returns error", func(t *testing.T) {
		_, err := svc.ListDirEntries(repo.ID, sym.ID, "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent subpath")
		}
	})

	t.Run("file-type symlink returns error", func(t *testing.T) {
		// Create a file symlink
		srcFile := filepath.Join(t.TempDir(), "test.txt")
		os.WriteFile(srcFile, []byte("test"), 0644)

		fileSym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
			TargetPath: srcFile,
			RelPath:    "test.txt",
		})
		if err != nil {
			t.Fatalf("failed to create file symlink: %v", err)
		}

		_, err = svc.ListDirEntries(repo.ID, fileSym.ID, "")
		if err == nil {
			t.Fatal("expected error for file-type symlink")
		}
	})

	t.Run("path traversal in subPath is blocked", func(t *testing.T) {
		_, err := svc.ListDirEntries(repo.ID, sym.ID, "../../etc")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})

	t.Run("hidden files are excluded", func(t *testing.T) {
		os.WriteFile(filepath.Join(sourceDir, ".hidden"), []byte("hidden"), 0644)
		defer os.Remove(filepath.Join(sourceDir, ".hidden"))

		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, e := range entries {
			if e.Name == ".hidden" {
				t.Fatal("hidden file should not be returned")
			}
		}
	})
}
