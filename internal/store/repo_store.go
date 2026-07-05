package store

import (
	"database/sql"
	"fmt"
	"time"

	"backup-manager/internal/model"
)

// CreateRepo inserts a new repository row.
func (s *Store) CreateRepo(r *model.Repo) error {
	query := `INSERT INTO repos (id, name, path, created_at, updated_at, status)
	           VALUES (?, ?, ?, ?, ?, ?)`
	now := time.Now()
	_, err := s.db.Exec(query, r.ID, r.Name, r.Path, now, now, model.RepoStatusActive)
	if err != nil {
		return fmt.Errorf("failed to create repo: %w", err)
	}
	return nil
}

// GetRepo retrieves a repository by ID.
func (s *Store) GetRepo(id string) (*model.Repo, error) {
	query := `SELECT id, name, path, created_at, updated_at, last_backup_at, status
	           FROM repos WHERE id = ?`
	row := s.db.QueryRow(query, id)

	r := &model.Repo{}
	var lastBackup sql.NullTime
	err := row.Scan(&r.ID, &r.Name, &r.Path, &r.CreatedAt, &r.UpdatedAt, &lastBackup, &r.Status)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("repo not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	if lastBackup.Valid {
		r.LastBackupAt = &lastBackup.Time
	}
	return r, nil
}

// GetRepoByPath retrieves a repository by its path.
// Returns nil, nil if not found.
func (s *Store) GetRepoByPath(path string) (*model.Repo, error) {
	query := `SELECT id, name, path, created_at, updated_at, last_backup_at, status
	           FROM repos WHERE path = ?`
	row := s.db.QueryRow(query, path)

	r := &model.Repo{}
	var lastBackup sql.NullTime
	err := row.Scan(&r.ID, &r.Name, &r.Path, &r.CreatedAt, &r.UpdatedAt, &lastBackup, &r.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get repo by path: %w", err)
	}
	if lastBackup.Valid {
		r.LastBackupAt = &lastBackup.Time
	}
	return r, nil
}

// ListRepos returns all repositories ordered by creation time.
func (s *Store) ListRepos() ([]*model.Repo, error) {
	query := `SELECT id, name, path, created_at, updated_at, last_backup_at, status
	           FROM repos ORDER BY created_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}
	defer rows.Close()

	var repos []*model.Repo
	for rows.Next() {
		r := &model.Repo{}
		var lastBackup sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &r.CreatedAt, &r.UpdatedAt, &lastBackup, &r.Status); err != nil {
			return nil, fmt.Errorf("failed to scan repo row: %w", err)
		}
		if lastBackup.Valid {
			r.LastBackupAt = &lastBackup.Time
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// UpdateRepo updates an existing repository.
func (s *Store) UpdateRepo(r *model.Repo) error {
	query := `UPDATE repos SET name = ?, path = ?, updated_at = ?, last_backup_at = ?, status = ?
	           WHERE id = ?`
	result, err := s.db.Exec(query, r.Name, r.Path, time.Now(), r.LastBackupAt, r.Status, r.ID)
	if err != nil {
		return fmt.Errorf("failed to update repo: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo not found: %s", r.ID)
	}
	return nil
}

// DeleteRepo deletes a repository by ID (cascades to config, auth, symlinks).
func (s *Store) DeleteRepo(id string) error {
	query := `DELETE FROM repos WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete repo: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("repo not found: %s", id)
	}
	return nil
}
