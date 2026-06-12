package store

import (
	"database/sql"
	"fmt"
	"time"

	"backup-manager/internal/model"
)

// CreateRepoAuth inserts a new auth row for a repo.
func (s *Store) CreateRepoAuth(a *model.GitAuth) error {
	query := `INSERT INTO repo_auths (repo_id, auth_type, ssh_private_key, ssh_private_key_path, username, password_encrypted, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, a.RepoID, a.AuthType, []byte(a.SSHPrivateKey), a.SSHPrivateKeyPath, a.Username, a.PasswordEncrypted, time.Now())
	if err != nil {
		return fmt.Errorf("failed to create repo auth: %w", err)
	}
	return nil
}

// GetRepoAuth retrieves the auth config for a repo.
func (s *Store) GetRepoAuth(repoID string) (*model.GitAuth, error) {
	query := `SELECT repo_id, auth_type, ssh_private_key, ssh_private_key_path, username, password_encrypted, updated_at
	           FROM repo_auths WHERE repo_id = ?`
	row := s.db.QueryRow(query, repoID)

	a := &model.GitAuth{}
	var sshKey []byte
	err := row.Scan(&a.RepoID, &a.AuthType, &sshKey, &a.SSHPrivateKeyPath, &a.Username, &a.PasswordEncrypted, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("repo auth not found: %s", repoID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repo auth: %w", err)
	}
	a.SSHPrivateKey = string(sshKey)
	return a, nil
}

// UpdateRepoAuth updates the auth config for a repo.
func (s *Store) UpdateRepoAuth(a *model.GitAuth) error {
	query := `UPDATE repo_auths SET auth_type = ?, ssh_private_key = ?, ssh_private_key_path = ?,
	           username = ?, password_encrypted = ?, updated_at = ? WHERE repo_id = ?`
	result, err := s.db.Exec(query, a.AuthType, []byte(a.SSHPrivateKey), a.SSHPrivateKeyPath, a.Username, a.PasswordEncrypted, time.Now(), a.RepoID)
	if err != nil {
		return fmt.Errorf("failed to update repo auth: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo auth not found: %s", a.RepoID)
	}
	return nil
}

// DeleteRepoAuth deletes the auth config for a repo.
func (s *Store) DeleteRepoAuth(repoID string) error {
	query := `DELETE FROM repo_auths WHERE repo_id = ?`
	result, err := s.db.Exec(query, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repo auth: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo auth not found: %s", repoID)
	}
	return nil
}
