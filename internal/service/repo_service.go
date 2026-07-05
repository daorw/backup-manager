package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"backup-manager/internal/git"
	"backup-manager/internal/model"
	"backup-manager/internal/store"

	"github.com/google/uuid"
)

// Scheduler defines the interface for scheduling auto-backup jobs.
type Scheduler interface {
	Register(repoID, cronExpr string) error
	Unregister(repoID string)
	IsRegistered(repoID string) bool
}

// RepoService handles repository business logic.
type RepoService struct {
	store     *store.Store
	gitEngine *git.GitEngine
	scheduler Scheduler
}

// NewRepoService creates a new RepoService.
func NewRepoService(s *store.Store, g *git.GitEngine, sch Scheduler) *RepoService {
	return &RepoService{store: s, gitEngine: g, scheduler: sch}
}

// CreateRepoRequest is the input for creating a new repository.
type CreateRepoRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// UpdateConfigRequest is the input for partially updating repo config.
type UpdateConfigRequest struct {
	RemoteURL          *string `json:"remote_url,omitempty"`
	Branch             *string `json:"branch,omitempty"`
	AutoBackup         *bool   `json:"auto_backup,omitempty"`
	AutoBackupInterval *string `json:"auto_backup_interval,omitempty"`
	GitUserName        *string `json:"git_user_name,omitempty"`
	GitUserEmail       *string `json:"git_user_email,omitempty"`
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// hasAllRepoDirs checks if the directory contains .links, data, and .git subdirectories.
func hasAllRepoDirs(path string) bool {
	for _, dir := range []string{".links", "data", ".git"} {
		info, err := os.Stat(filepath.Join(path, dir))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

// Create creates a new backup repository.
func (s *RepoService) Create(req *CreateRepoRequest) (*model.Repo, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to check path: %w", err)
		}
		// Path does not exist — create it (original behavior)
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create repo directory: %w", err)
		}
	} else {
		// Path exists
		if !info.IsDir() {
			return nil, fmt.Errorf("path is not a directory: %s", absPath)
		}

		if hasAllRepoDirs(absPath) {
			// Has .links, data, .git — existing backup repo, reuse it
		} else {
			// Check if directory is empty
			empty, err := isDirEmpty(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory: %w", err)
			}
			if !empty {
				return nil, fmt.Errorf("directory already occupied, please choose another path: %s", absPath)
			}
			// Empty directory — reuse and initialize
		}
	}

	// Check if path is already used by another repo
	existingRepo, err := s.store.GetRepoByPath(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing repo: %w", err)
	}
	if existingRepo != nil {
		return nil, fmt.Errorf("path already used by repository \"%s\": %s", existingRepo.Name, absPath)
	}

	// Ensure .links and data directories exist
	for _, dir := range []string{".links", "data"} {
		if err := os.MkdirAll(filepath.Join(absPath, dir), 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}

	// Initialize git repo if not already initialized
	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		if err := s.gitEngine.Init(absPath); err != nil {
			return nil, fmt.Errorf("failed to init git: %w", err)
		}
	}

	// Ensure .gitignore exists
	gitignorePath := filepath.Join(absPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := "# Backup Manager managed files\n.links/\n"
		if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	now := time.Now()
	repo := &model.Repo{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Path:      absPath,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    model.RepoStatusActive,
	}

	if err := s.store.CreateRepo(repo); err != nil {
		return nil, fmt.Errorf("failed to save repo: %w", err)
	}

	config := &model.RepoConfig{
		RepoID: repo.ID,
		Branch: "main",
	}
	if err := s.store.CreateRepoConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return repo, nil
}

// Get returns a repository by ID with its config.
func (s *RepoService) Get(id string) (*model.Repo, *model.RepoConfig, error) {
	repo, err := s.store.GetRepo(id)
	if err != nil {
		return nil, nil, err
	}

	config, err := s.store.GetRepoConfig(id)
	if err != nil {
		// Config may not exist for legacy repos; return empty config
		config = &model.RepoConfig{RepoID: id, Branch: "main"}
	}

	return repo, config, nil
}

// List returns all repositories with their configs.
func (s *RepoService) List() ([]*model.Repo, []*model.RepoConfig, error) {
	repos, err := s.store.ListRepos()
	if err != nil {
		return nil, nil, err
	}

	var configs []*model.RepoConfig
	for _, r := range repos {
		c, err := s.store.GetRepoConfig(r.ID)
		if err != nil {
			c = &model.RepoConfig{RepoID: r.ID, Branch: "main"}
		}
		configs = append(configs, c)
	}

	return repos, configs, nil
}

// GitInit initializes a Git repository in the repo directory.
func (s *RepoService) GitInit(id string) error {
	repo, err := s.store.GetRepo(id)
	if err != nil {
		return err
	}
	if err := s.gitEngine.Init(repo.Path); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}
	// Ensure .gitignore exists after init
	gitignorePath := filepath.Join(repo.Path, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := "# Backup Manager managed files\n.links/\n"
		if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}
	return nil
}

// IsGitInitialized checks whether the repo has a .git directory.
func (s *RepoService) IsGitInitialized(id string) (bool, error) {
	repo, err := s.store.GetRepo(id)
	if err != nil {
		return false, err
	}
	gitDir := filepath.Join(repo.Path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// HasGitRemote checks whether the repo has a remote configured in git.
func (s *RepoService) HasGitRemote(id string) (bool, error) {
	repo, err := s.store.GetRepo(id)
	if err != nil {
		return false, err
	}
	url, err := s.gitEngine.GetRemoteURL(repo.Path, "origin")
	if err != nil {
		return false, nil
	}
	return url != "", nil
}

// Delete removes a repository from the database.
// The filesystem contents are left intact for safety.
func (s *RepoService) Delete(id string) error {
	s.scheduler.Unregister(id)

	if err := s.store.DeleteRepo(id); err != nil {
		return err
	}

	return nil
}

// UpdateConfig updates a repo's configuration,
// applies git config changes, and updates the scheduler.
// Peripheral operations (file writes, git commands, scheduler) are performed
// before updating the database to avoid state inconsistency on failure.
func (s *RepoService) UpdateConfig(id string, req *UpdateConfigRequest) error {
	repo, err := s.store.GetRepo(id)
	if err != nil {
		return err
	}

	config, err := s.store.GetRepoConfig(id)
	if err != nil {
		config = &model.RepoConfig{RepoID: id, Branch: "main"}
	}

	changed := false

	if req.RemoteURL != nil {
		config.RemoteURL = *req.RemoteURL
		changed = true
	}
	if req.Branch != nil {
		config.Branch = *req.Branch
		changed = true
	}
	if req.AutoBackup != nil {
		config.AutoBackup = *req.AutoBackup
		changed = true
	}
	if req.AutoBackupInterval != nil {
		config.AutoBackupInterval = *req.AutoBackupInterval
		changed = true
	}
	if req.GitUserName != nil {
		config.GitUserName = *req.GitUserName
		changed = true
	}
	if req.GitUserEmail != nil {
		config.GitUserEmail = *req.GitUserEmail
		changed = true
	}

	if !changed {
		return nil
	}

	if req.GitUserName != nil && *req.GitUserName != "" {
		if err := s.gitEngine.ConfigSet(repo.Path, "user.name", *req.GitUserName); err != nil {
			return fmt.Errorf("failed to set git user.name: %w", err)
		}
	}
	if req.GitUserEmail != nil && *req.GitUserEmail != "" {
		if err := s.gitEngine.ConfigSet(repo.Path, "user.email", *req.GitUserEmail); err != nil {
			return fmt.Errorf("failed to set git user.email: %w", err)
		}
	}

	if req.RemoteURL != nil && *req.RemoteURL != "" {
		if err := s.gitEngine.RemoteSetURL(repo.Path, *req.RemoteURL); err != nil {
			return fmt.Errorf("failed to set remote URL: %w", err)
		}
	}

	// Update scheduler for auto-backup
	if req.AutoBackup != nil || req.AutoBackupInterval != nil {
		s.scheduler.Unregister(id)
		if config.AutoBackup && config.AutoBackupInterval != "" {
			if err := s.scheduler.Register(id, config.AutoBackupInterval); err != nil {
				return fmt.Errorf("failed to register backup schedule: %w", err)
			}
		}
	}

	// Update database last — all peripheral operations succeeded
	if err := s.store.UpdateRepoConfig(config); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	return nil
}
