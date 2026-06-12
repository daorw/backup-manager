package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-manager/internal/model"

	"github.com/google/uuid"
)

func setupTestDB(t *testing.T) (*Store, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	s := NewStore(db)

	cleanup := func() {
		db.Close()
	}

	return s, cleanup
}

func TestRepoStore(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("create and get repo", func(t *testing.T) {
		repo := &model.Repo{
			ID:   uuid.New().String(),
			Name: "test-repo",
			Path: t.TempDir(),
		}

		err := s.CreateRepo(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := s.GetRepo(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != repo.Name {
			t.Fatalf("expected name %s, got %s", repo.Name, got.Name)
		}
		if got.Path != repo.Path {
			t.Fatalf("expected path %s, got %s", repo.Path, got.Path)
		}
		if got.Status != model.RepoStatusActive {
			t.Fatalf("expected status %s, got %s", model.RepoStatusActive, got.Status)
		}
	})

	t.Run("get non-existent repo", func(t *testing.T) {
		_, err := s.GetRepo("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent repo")
		}
	})

	t.Run("list repos", func(t *testing.T) {
		repo1 := &model.Repo{
			ID:   uuid.New().String(),
			Name: "repo1",
			Path: t.TempDir(),
		}
		repo2 := &model.Repo{
			ID:   uuid.New().String(),
			Name: "repo2",
			Path: filepath.Join(t.TempDir(), "sub"),
		}
		os.MkdirAll(repo2.Path, 0755)

		s.CreateRepo(repo1)
		s.CreateRepo(repo2)

		repos, err := s.ListRepos()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) < 2 {
			t.Fatalf("expected at least 2 repos, got %d", len(repos))
		}
	})

	t.Run("update repo", func(t *testing.T) {
		repo := &model.Repo{
			ID:   uuid.New().String(),
			Name: "update-test",
			Path: t.TempDir(),
		}
		s.CreateRepo(repo)

		now := time.Now()
		repo.Name = "updated-name"
		repo.Status = model.RepoStatusError
		repo.LastBackupAt = &now

		err := s.UpdateRepo(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := s.GetRepo(repo.ID)
		if got.Name != "updated-name" {
			t.Fatalf("expected name 'updated-name', got %s", got.Name)
		}
		if got.Status != model.RepoStatusError {
			t.Fatalf("expected status %s, got %s", model.RepoStatusError, got.Status)
		}
		if got.LastBackupAt == nil {
			t.Fatal("expected last_backup_at to be set")
		}
	})

	t.Run("delete repo", func(t *testing.T) {
		repo := &model.Repo{
			ID:   uuid.New().String(),
			Name: "delete-test",
			Path: t.TempDir(),
		}
		s.CreateRepo(repo)

		err := s.DeleteRepo(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = s.GetRepo(repo.ID)
		if err == nil {
			t.Fatal("expected error after deletion")
		}
	})
}

func TestRepoConfigStore(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	repo := &model.Repo{
		ID:   uuid.New().String(),
		Name: "config-test",
		Path: t.TempDir(),
	}
	s.CreateRepo(repo)

	t.Run("create and get config", func(t *testing.T) {
		config := &model.RepoConfig{
			RepoID:             repo.ID,
			RemoteURL:          "https://github.com/user/repo.git",
			Branch:             "main",
			AutoBackup:         true,
			AutoBackupInterval: "0 */6 * * *",
			GitUserName:        "testuser",
			GitUserEmail:       "test@example.com",
		}

		err := s.CreateRepoConfig(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := s.GetRepoConfig(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.RemoteURL != config.RemoteURL {
			t.Fatalf("expected %s, got %s", config.RemoteURL, got.RemoteURL)
		}
		if got.AutoBackup != config.AutoBackup {
			t.Fatalf("expected auto_backup %v, got %v", config.AutoBackup, got.AutoBackup)
		}
	})

	t.Run("get non-existent config", func(t *testing.T) {
		_, err := s.GetRepoConfig("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent config")
		}
	})

	t.Run("update config", func(t *testing.T) {
		config, _ := s.GetRepoConfig(repo.ID)
		config.RemoteURL = "https://github.com/user/new-repo.git"
		config.AutoBackup = false

		err := s.UpdateRepoConfig(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := s.GetRepoConfig(repo.ID)
		if got.RemoteURL != "https://github.com/user/new-repo.git" {
			t.Fatalf("expected updated url, got %s", got.RemoteURL)
		}
		if got.AutoBackup != false {
			t.Fatal("expected auto_backup to be false")
		}
	})

	t.Run("update non-existent config", func(t *testing.T) {
		err := s.UpdateRepoConfig(&model.RepoConfig{RepoID: "nonexistent"})
		if err == nil {
			t.Fatal("expected error for non-existent config")
		}
	})

	t.Run("delete config", func(t *testing.T) {
		err := s.DeleteRepoConfig(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = s.GetRepoConfig(repo.ID)
		if err == nil {
			t.Fatal("expected error after deletion")
		}
	})
}

func TestSymlinkStore(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	repo := &model.Repo{
		ID:   uuid.New().String(),
		Name: "symlink-test",
		Path: t.TempDir(),
	}
	s.CreateRepo(repo)

	t.Run("create and get symlink", func(t *testing.T) {
		link := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "documents/report.docx",
			TargetPath:   "/Users/test/Documents/report.docx",
			Type:         model.SymlinkTypeFile,
			FileSize:     1024,
		}

		err := s.CreateSymlink(link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := s.GetSymlink(link.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.RelativePath != link.RelativePath {
			t.Fatalf("expected %s, got %s", link.RelativePath, got.RelativePath)
		}
		if got.TargetPath != link.TargetPath {
			t.Fatalf("expected %s, got %s", link.TargetPath, got.TargetPath)
		}
	})

	t.Run("list symlinks by repo id", func(t *testing.T) {
		link1 := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "file1.txt",
			TargetPath:   "/tmp/file1.txt",
			Type:         model.SymlinkTypeFile,
		}
		link2 := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "file2.txt",
			TargetPath:   "/tmp/file2.txt",
			Type:         model.SymlinkTypeFile,
		}
		s.CreateSymlink(link1)
		s.CreateSymlink(link2)

		links, err := s.ListSymlinks(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) < 2 {
			t.Fatalf("expected at least 2 symlinks, got %d", len(links))
		}
	})

	t.Run("duplicate relative path returns error", func(t *testing.T) {
		link := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "duplicate.txt",
			TargetPath:   "/tmp/duplicate.txt",
			Type:         model.SymlinkTypeFile,
		}
		err := s.CreateSymlink(link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dup := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "duplicate.txt",
			TargetPath:   "/tmp/other.txt",
			Type:         model.SymlinkTypeFile,
		}
		err = s.CreateSymlink(dup)
		if err == nil {
			t.Fatal("expected error for duplicate relative_path")
		}
	})

	t.Run("update symlink", func(t *testing.T) {
		link := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "update-test.txt",
			TargetPath:   "/tmp/old.txt",
			Type:         model.SymlinkTypeFile,
		}
		s.CreateSymlink(link)

		link.TargetPath = "/tmp/new.txt"
		link.FileSize = 2048

		err := s.UpdateSymlink(link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := s.GetSymlink(link.ID)
		if got.TargetPath != "/tmp/new.txt" {
			t.Fatalf("expected /tmp/new.txt, got %s", got.TargetPath)
		}
		if got.FileSize != 2048 {
			t.Fatalf("expected file_size 2048, got %d", got.FileSize)
		}
	})

	t.Run("delete symlink", func(t *testing.T) {
		link := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo.ID,
			RelativePath: "delete-test.txt",
			TargetPath:   "/tmp/delete.txt",
			Type:         model.SymlinkTypeFile,
		}
		s.CreateSymlink(link)

		err := s.DeleteSymlink(link.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = s.GetSymlink(link.ID)
		if err == nil {
			t.Fatal("expected error after deletion")
		}
	})

	t.Run("list symlinks for non-existent repo", func(t *testing.T) {
		links, err := s.ListSymlinks("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 0 {
			t.Fatalf("expected 0 symlinks, got %d", len(links))
		}
	})

	t.Run("symlink is deleted on repo cascade", func(t *testing.T) {
		repo2 := &model.Repo{
			ID:   uuid.New().String(),
			Name: "cascade-test",
			Path: t.TempDir(),
		}
		s.CreateRepo(repo2)

		link := &model.Symlink{
			ID:           uuid.New().String(),
			RepoID:       repo2.ID,
			RelativePath: "cascade.txt",
			TargetPath:   "/tmp/cascade.txt",
			Type:         model.SymlinkTypeFile,
		}
		s.CreateSymlink(link)

		s.DeleteRepo(repo2.ID)

		links, _ := s.ListSymlinks(repo2.ID)
		if len(links) != 0 {
			t.Fatalf("expected 0 symlinks after repo deletion, got %d", len(links))
		}
	})
}

func TestRepoAuthStore(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	repo := &model.Repo{
		ID:   uuid.New().String(),
		Name: "auth-test",
		Path: t.TempDir(),
	}
	s.CreateRepo(repo)

	t.Run("create and get auth", func(t *testing.T) {
		auth := &model.GitAuth{
			RepoID:            repo.ID,
			AuthType:          model.GitAuthSSHKey,
			SSHPrivateKey:     "encrypted-key-data",
			SSHPrivateKeyPath: "/home/user/.ssh/id_rsa",
			Username:          "gituser",
		}

		err := s.CreateRepoAuth(auth)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := s.GetRepoAuth(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AuthType != model.GitAuthSSHKey {
			t.Fatalf("expected auth_type %s, got %s", model.GitAuthSSHKey, got.AuthType)
		}
		if got.SSHPrivateKeyPath != "/home/user/.ssh/id_rsa" {
			t.Fatalf("expected key path, got %s", got.SSHPrivateKeyPath)
		}
	})

	t.Run("get non-existent auth", func(t *testing.T) {
		_, err := s.GetRepoAuth("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent auth")
		}
	})

	t.Run("update auth", func(t *testing.T) {
		auth, _ := s.GetRepoAuth(repo.ID)
		auth.AuthType = model.GitAuthPassword
		auth.Username = "newuser"
		auth.PasswordEncrypted = []byte("encrypted-pass")

		err := s.UpdateRepoAuth(auth)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := s.GetRepoAuth(repo.ID)
		if got.AuthType != model.GitAuthPassword {
			t.Fatalf("expected auth_type %s, got %s", model.GitAuthPassword, got.AuthType)
		}
		if got.Username != "newuser" {
			t.Fatalf("expected username 'newuser', got %s", got.Username)
		}
	})

	t.Run("delete auth", func(t *testing.T) {
		err := s.DeleteRepoAuth(repo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = s.GetRepoAuth(repo.ID)
		if err == nil {
			t.Fatal("expected error after auth deletion")
		}
	})

	t.Run("auth is deleted on repo cascade", func(t *testing.T) {
		repo2 := &model.Repo{
			ID:   uuid.New().String(),
			Name: "auth-cascade",
			Path: t.TempDir(),
		}
		s.CreateRepo(repo2)
		s.CreateRepoAuth(&model.GitAuth{
			RepoID:   repo2.ID,
			AuthType: model.GitAuthNone,
		})

		s.DeleteRepo(repo2.ID)

		_, err := s.GetRepoAuth(repo2.ID)
		if err == nil {
			t.Fatal("expected error after repo deletion (cascade)")
		}
	})
}
