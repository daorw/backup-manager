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
