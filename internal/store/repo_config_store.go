package store

import (
	"database/sql"
	"fmt"

	"backup-manager/internal/model"
)

// CreateRepoConfig inserts a new repo config row.
func (s *Store) CreateRepoConfig(c *model.RepoConfig) error {
	query := `INSERT INTO repo_configs (repo_id, remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`
	autoBackup := 0
	if c.AutoBackup {
		autoBackup = 1
	}
	_, err := s.db.Exec(query, c.RepoID, c.RemoteURL, c.Branch, autoBackup, c.AutoBackupInterval, c.GitUserName, c.GitUserEmail)
	if err != nil {
		return fmt.Errorf("failed to create repo config: %w", err)
	}
	return nil
}

// GetRepoConfig retrieves the config for a given repo.
func (s *Store) GetRepoConfig(repoID string) (*model.RepoConfig, error) {
	query := `SELECT repo_id, remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
	           FROM repo_configs WHERE repo_id = ?`
	row := s.db.QueryRow(query, repoID)

	c := &model.RepoConfig{}
	var autoBackup int
	err := row.Scan(&c.RepoID, &c.RemoteURL, &c.Branch, &autoBackup, &c.AutoBackupInterval, &c.GitUserName, &c.GitUserEmail)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("repo config not found: %s", repoID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repo config: %w", err)
	}
	c.AutoBackup = autoBackup == 1
	return c, nil
}

// UpdateRepoConfig updates an existing repo config.
func (s *Store) UpdateRepoConfig(c *model.RepoConfig) error {
	query := `UPDATE repo_configs SET remote_url = ?, branch = ?, auto_backup = ?, auto_backup_interval = ?,
	           git_user_name = ?, git_user_email = ? WHERE repo_id = ?`
	autoBackup := 0
	if c.AutoBackup {
		autoBackup = 1
	}
	result, err := s.db.Exec(query, c.RemoteURL, c.Branch, autoBackup, c.AutoBackupInterval, c.GitUserName, c.GitUserEmail, c.RepoID)
	if err != nil {
		return fmt.Errorf("failed to update repo config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo config not found: %s", c.RepoID)
	}
	return nil
}

// DeleteRepoConfig deletes the config for a given repo.
func (s *Store) DeleteRepoConfig(repoID string) error {
	query := `DELETE FROM repo_configs WHERE repo_id = ?`
	result, err := s.db.Exec(query, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repo config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo config not found: %s", repoID)
	}
	return nil
}

// ListAutoBackupRepos returns all repo configs with auto_backup enabled.
func (s *Store) ListAutoBackupRepos() ([]*model.RepoConfig, error) {
	query := `SELECT repo_id, remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
	           FROM repo_configs WHERE auto_backup = 1`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list auto-backup repos: %w", err)
	}
	defer rows.Close()

	var configs []*model.RepoConfig
	for rows.Next() {
		c := &model.RepoConfig{}
		var autoBackup int
		if err := rows.Scan(&c.RepoID, &c.RemoteURL, &c.Branch, &autoBackup, &c.AutoBackupInterval, &c.GitUserName, &c.GitUserEmail); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}
		c.AutoBackup = autoBackup == 1
		configs = append(configs, c)
	}
	return configs, rows.Err()
}
