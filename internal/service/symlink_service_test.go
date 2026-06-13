package service

import (
	"fmt"
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

	t.Run("is_new is true for modified source file", func(t *testing.T) {
		// Modify the source file a.txt after data copy was already created
		// (the data copy exists from Create, now we modify the source)
		sourceFile := filepath.Join(sourceDir, "a.txt")
		// a.txt was created with "aaa", modify it
		if err := os.WriteFile(sourceFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("failed to modify source file: %v", err)
		}

		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Find a.txt entry and verify is_new is true
		for _, e := range entries {
			if e.Name == "a.txt" {
				if !e.IsNew {
					t.Fatal("expected a.txt to be marked as new (is_new=true) after modification")
				}
				return
			}
		}
		t.Fatal("a.txt not found in entries")
	})

	t.Run("is_new is false for unchanged source file", func(t *testing.T) {
		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// b.txt was never modified after Create, so it should match data copy
		for _, e := range entries {
			if e.Name == "b.txt" {
				if e.IsNew {
					t.Fatal("expected b.txt is_new=false since it was not modified")
				}
				return
			}
		}
		t.Fatal("b.txt not found in entries")
	})

	t.Run("is_new is true for file with no data copy", func(t *testing.T) {
		// Create a new file in source that was never synced to data/
		newFile := filepath.Join(sourceDir, "newfile.txt")
		if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
			t.Fatalf("failed to create new source file: %v", err)
		}

		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, e := range entries {
			if e.Name == "newfile.txt" {
				if !e.IsNew {
					t.Fatal("expected newfile.txt to be marked as new (no data copy exists)")
				}
				return
			}
		}
		t.Fatal("newfile.txt not found in entries")
	})

	t.Run("directory entries always have is_new=false", func(t *testing.T) {
		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, e := range entries {
			if e.Type == "directory" && e.IsNew {
				t.Fatalf("directory entry %s should have is_new=false", e.Name)
			}
		}
	})
}

func TestComputeSymlinkIsNew(t *testing.T) {
	svc, _, repo, cleanup := setupSymlinkServiceTest(t)
	defer cleanup()

	t.Run("file symlink returns false when source is unchanged", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "test.txt")
		if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
			TargetPath: srcFile,
			RelPath:    "test.txt",
		})
		if err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		isNew, err := svc.ComputeSymlinkIsNew(sym)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isNew {
			t.Fatal("expected is_new=false for unchanged file")
		}
	})

	t.Run("file symlink returns true when source is modified", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "modtest.txt")
		if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
			TargetPath: srcFile,
			RelPath:    "modtest.txt",
		})
		if err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Modify the source file after creation
		if err := os.WriteFile(srcFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("failed to modify source file: %v", err)
		}

		isNew, err := svc.ComputeSymlinkIsNew(sym)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isNew {
			t.Fatal("expected is_new=true for modified file")
		}
	})

	t.Run("directory symlink always returns false", func(t *testing.T) {
		srcDir := filepath.Join(t.TempDir(), "dirtest")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatalf("failed to create source dir: %v", err)
		}

		sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
			TargetPath: srcDir,
			RelPath:    "dirtest",
		})
		if err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		isNew, err := svc.ComputeSymlinkIsNew(sym)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isNew {
			t.Fatal("expected is_new=false for directory symlink")
		}
	})
}

func TestDetectNestedSymlink(t *testing.T) {
	t.Run("detects regular file as not nested symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "file.txt")
		os.WriteFile(target, []byte("content"), 0644)

		info, err := os.Lstat(target)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(target, info)
		if entry.IsNestedSymlink {
			t.Fatal("expected IsNestedSymlink=false for regular file")
		}
		if entry.Type != "file" {
			t.Fatalf("expected type 'file', got %s", entry.Type)
		}
	})

	t.Run("detects regular directory as not nested symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		dir := filepath.Join(tmpDir, "subdir")
		os.MkdirAll(dir, 0755)

		info, err := os.Lstat(dir)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(dir, info)
		if entry.IsNestedSymlink {
			t.Fatal("expected IsNestedSymlink=false for regular directory")
		}
		if entry.Type != "directory" {
			t.Fatalf("expected type 'directory', got %s", entry.Type)
		}
	})

	t.Run("detects symlink to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("content"), 0644)

		link := filepath.Join(tmpDir, "link.txt")
		os.Symlink(target, link)

		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(link, info)
		if !entry.IsNestedSymlink {
			t.Fatal("expected IsNestedSymlink=true for symlink")
		}
		if entry.Type != "symlink_file" {
			t.Fatalf("expected type 'symlink_file', got %s", entry.Type)
		}
		if entry.NestedTarget == "" {
			t.Fatal("expected NestedTarget to be set")
		}
		if entry.NestedDepth != 1 {
			t.Fatalf("expected NestedDepth=1, got %d", entry.NestedDepth)
		}
	})

	t.Run("detects symlink to directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target_dir")
		os.MkdirAll(target, 0755)

		link := filepath.Join(tmpDir, "link_dir")
		os.Symlink(target, link)

		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(link, info)
		if !entry.IsNestedSymlink {
			t.Fatal("expected IsNestedSymlink=true for symlink")
		}
		if entry.Type != "symlink_directory" {
			t.Fatalf("expected type 'symlink_directory', got %s", entry.Type)
		}
	})

	t.Run("detects nested symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("content"), 0644)

		link1 := filepath.Join(tmpDir, "link1")
		link2 := filepath.Join(tmpDir, "link2")
		os.Symlink(target, link1)
		os.Symlink(link1, link2)

		info, err := os.Lstat(link2)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(link2, info)
		if !entry.IsNestedSymlink {
			t.Fatal("expected IsNestedSymlink=true for nested symlink")
		}
		if entry.NestedDepth != 2 {
			t.Fatalf("expected NestedDepth=2, got %d", entry.NestedDepth)
		}
	})

	t.Run("detects cycle in symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		link1 := filepath.Join(tmpDir, "link1")
		link2 := filepath.Join(tmpDir, "link2")
		os.Symlink(link2, link1)
		os.Symlink(link1, link2)

		info, err := os.Lstat(link1)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(link1, info)
		if !entry.HasCycle {
			t.Fatal("expected HasCycle=true for cyclic symlink")
		}
		if entry.Type != "symlink_error" {
			t.Fatalf("expected type 'symlink_error', got %s", entry.Type)
		}
	})

	t.Run("detects depth limit exceeded", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("content"), 0644)

		// Create a chain of 11 symlinks to exceed the limit of 10
		prev := target
		for i := 0; i < 11; i++ {
			link := filepath.Join(tmpDir, fmt.Sprintf("link%d", i))
			os.Symlink(prev, link)
			prev = link
		}

		info, err := os.Lstat(prev)
		if err != nil {
			t.Fatalf("failed to lstat: %v", err)
		}

		entry := detectNestedSymlink(prev, info)
		if entry.Type != "symlink_error" {
			t.Fatalf("expected type 'symlink_error', got %s", entry.Type)
		}
	})
}

func TestDetectCycle(t *testing.T) {
	t.Run("no cycle in linear chain", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("content"), 0644)

		link1 := filepath.Join(tmpDir, "link1")
		link2 := filepath.Join(tmpDir, "link2")
		os.Symlink(target, link1)
		os.Symlink(link1, link2)

		chain := []string{link2, link1}
		if detectCycle(chain) {
			t.Fatal("expected no cycle in linear chain")
		}
	})

	t.Run("detects cycle in chain", func(t *testing.T) {
		tmpDir := t.TempDir()
		link1 := filepath.Join(tmpDir, "link1")
		link2 := filepath.Join(tmpDir, "link2")

		chain := []string{link1, link2, link1}
		if !detectCycle(chain) {
			t.Fatal("expected cycle to be detected")
		}
	})

	t.Run("empty chain has no cycle", func(t *testing.T) {
		if detectCycle(nil) {
			t.Fatal("expected no cycle in empty chain")
		}
	})
}

func TestListDirEntries_NestedSymlinks(t *testing.T) {
	svc, _, repo, cleanup := setupSymlinkServiceTest(t)
	defer cleanup()

	// Create source directory with nested symlinks
	sourceDir := filepath.Join(t.TempDir(), "src")
	os.MkdirAll(sourceDir, 0755)

	// Create a regular file
	os.WriteFile(filepath.Join(sourceDir, "regular.txt"), []byte("regular"), 0644)

	// Create a symlink to a file
	targetFile := filepath.Join(t.TempDir(), "target.txt")
	os.WriteFile(targetFile, []byte("target content"), 0644)
	os.Symlink(targetFile, filepath.Join(sourceDir, "link_file.txt"))

	// Create a symlink to a directory
	targetDir := filepath.Join(t.TempDir(), "target_dir")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "inner.txt"), []byte("inner"), 0644)
	os.Symlink(targetDir, filepath.Join(sourceDir, "link_dir"))

	// Create a directory symlink in the repo
	sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "nested_test",
	})
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	t.Run("lists entries with nested symlink info", func(t *testing.T) {
		entries, err := svc.ListDirEntries(repo.ID, sym.ID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		// Find each entry and verify its type
		for _, e := range entries {
			switch e.Name {
			case "regular.txt":
				if e.Type != "file" {
					t.Fatalf("expected regular.txt type 'file', got %s", e.Type)
				}
				if e.IsNestedSymlink {
					t.Fatal("expected regular.txt IsNestedSymlink=false")
				}
			case "link_file.txt":
				if e.Type != "symlink_file" {
					t.Fatalf("expected link_file.txt type 'symlink_file', got %s", e.Type)
				}
				if !e.IsNestedSymlink {
					t.Fatal("expected link_file.txt IsNestedSymlink=true")
				}
				if e.NestedTarget == "" {
					t.Fatal("expected link_file.txt NestedTarget to be set")
				}
			case "link_dir":
				if e.Type != "symlink_directory" {
					t.Fatalf("expected link_dir type 'symlink_directory', got %s", e.Type)
				}
				if !e.IsNestedSymlink {
					t.Fatal("expected link_dir IsNestedSymlink=true")
				}
			default:
				t.Fatalf("unexpected entry: %s", e.Name)
			}
		}
	})
}

func TestAddNestedSymlink(t *testing.T) {
	svc, s, repo, cleanup := setupSymlinkServiceTest(t)
	defer cleanup()

	// Create a source directory with a subdirectory
	sourceDir := filepath.Join(t.TempDir(), "src")
	os.MkdirAll(filepath.Join(sourceDir, "sub"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "sub", "file.txt"), []byte("content"), 0644)

	// Create a directory symlink in the repo
	sym, err := svc.Create(repo.ID, &CreateSymlinkRequest{
		TargetPath: sourceDir,
		RelPath:    "parent",
	})
	if err != nil {
		t.Fatalf("failed to create parent symlink: %v", err)
	}

	t.Run("nested symlink is persisted to database", func(t *testing.T) {
		// Create a target for the nested symlink
		nestedTarget := filepath.Join(t.TempDir(), "nested_target")
		os.MkdirAll(nestedTarget, 0755)
		os.WriteFile(filepath.Join(nestedTarget, "data.txt"), []byte("nested data"), 0644)

		nestedSym, err := svc.AddNestedSymlink(repo.ID, sym.ID, &AddNestedSymlinkRequest{
			TargetPath: nestedTarget,
			SubPath:    "sub/nested_link",
		})
		if err != nil {
			t.Fatalf("failed to add nested symlink: %v", err)
		}

		// Verify the symlink exists in the database
		dbSym, err := s.GetSymlink(nestedSym.ID)
		if err != nil {
			t.Fatalf("nested symlink not found in database: %v", err)
		}

		if dbSym.ID != nestedSym.ID {
			t.Fatalf("expected symlink ID %s, got %s", nestedSym.ID, dbSym.ID)
		}
		if dbSym.RepoID != repo.ID {
			t.Fatalf("expected repo ID %s, got %s", repo.ID, dbSym.RepoID)
		}
		expectedRelPath := filepath.Join("parent", "sub", "nested_link")
		if dbSym.RelativePath != expectedRelPath {
			t.Fatalf("expected relative path %s, got %s", expectedRelPath, dbSym.RelativePath)
		}
		// Use EvalSymlinks to resolve the path for macOS (/var -> /private/var)
		resolvedTarget, _ := filepath.EvalSymlinks(nestedTarget)
		if dbSym.TargetPath != resolvedTarget {
			t.Fatalf("expected target path %s, got %s", resolvedTarget, dbSym.TargetPath)
		}
		if dbSym.Type != model.SymlinkTypeDirectory {
			t.Fatalf("expected type directory, got %s", dbSym.Type)
		}
	})

	t.Run("nested symlink appears in repo symlinks list", func(t *testing.T) {
		symlinks, err := s.ListSymlinks(repo.ID)
		if err != nil {
			t.Fatalf("failed to list symlinks: %v", err)
		}

		found := false
		for _, s := range symlinks {
			if s.RelativePath == filepath.Join("parent", "sub", "nested_link") {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("nested symlink not found in repo symlinks list")
		}
	})

	t.Run("nested symlink can be deleted from database", func(t *testing.T) {
		// Create another nested symlink to delete
		nestedTarget := filepath.Join(t.TempDir(), "to_delete")
		os.MkdirAll(nestedTarget, 0755)

		nestedSym, err := svc.AddNestedSymlink(repo.ID, sym.ID, &AddNestedSymlinkRequest{
			TargetPath: nestedTarget,
			SubPath:    "sub/to_delete",
		})
		if err != nil {
			t.Fatalf("failed to add nested symlink: %v", err)
		}

		// Delete it
		if err := svc.Delete(nestedSym.ID); err != nil {
			t.Fatalf("failed to delete nested symlink: %v", err)
		}

		// Verify it's gone from database
		_, err = s.GetSymlink(nestedSym.ID)
		if err == nil {
			t.Fatal("expected error when getting deleted symlink")
		}
	})
}
