package store

import (
	"database/sql"
	"fmt"
	"time"

	"backup-manager/internal/model"
)

// CreateSymlink inserts a new symlink row.
func (s *Store) CreateSymlink(l *model.Symlink) error {
	query := `INSERT INTO symlinks (id, repo_id, relative_path, target_path, type, file_size, modified_at, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	_, err := s.db.Exec(query, l.ID, l.RepoID, l.RelativePath, l.TargetPath, l.Type, l.FileSize, l.ModifiedAt, now)
	if err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}
	return nil
}

// GetSymlink retrieves a symlink by ID.
func (s *Store) GetSymlink(id string) (*model.Symlink, error) {
	query := `SELECT id, repo_id, relative_path, target_path, type, file_size, modified_at, created_at
	           FROM symlinks WHERE id = ?`
	row := s.db.QueryRow(query, id)

	l := &model.Symlink{}
	var modifiedAt sql.NullTime
	err := row.Scan(&l.ID, &l.RepoID, &l.RelativePath, &l.TargetPath, &l.Type, &l.FileSize, &modifiedAt, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("symlink not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get symlink: %w", err)
	}
	if modifiedAt.Valid {
		l.ModifiedAt = &modifiedAt.Time
	}
	return l, nil
}

// ListSymlinks returns all symlinks for a given repo.
func (s *Store) ListSymlinks(repoID string) ([]*model.Symlink, error) {
	query := `SELECT id, repo_id, relative_path, target_path, type, file_size, modified_at, created_at
	           FROM symlinks WHERE repo_id = ? ORDER BY relative_path`
	rows, err := s.db.Query(query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list symlinks: %w", err)
	}
	defer rows.Close()

	var links []*model.Symlink
	for rows.Next() {
		l := &model.Symlink{}
		var modifiedAt sql.NullTime
		if err := rows.Scan(&l.ID, &l.RepoID, &l.RelativePath, &l.TargetPath, &l.Type, &l.FileSize, &modifiedAt, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan symlink row: %w", err)
		}
		if modifiedAt.Valid {
			l.ModifiedAt = &modifiedAt.Time
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetSymlinkByRelativePath retrieves a symlink by repo ID and relative path.
func (s *Store) GetSymlinkByRelativePath(repoID, relativePath string) (*model.Symlink, error) {
	query := `SELECT id, repo_id, relative_path, target_path, type, file_size, modified_at, created_at
	           FROM symlinks WHERE repo_id = ? AND relative_path = ?`
	row := s.db.QueryRow(query, repoID, relativePath)

	l := &model.Symlink{}
	var modifiedAt sql.NullTime
	err := row.Scan(&l.ID, &l.RepoID, &l.RelativePath, &l.TargetPath, &l.Type, &l.FileSize, &modifiedAt, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("symlink not found for repo %s and path %s", repoID, relativePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get symlink by relative path: %w", err)
	}
	if modifiedAt.Valid {
		l.ModifiedAt = &modifiedAt.Time
	}
	return l, nil
}

// UpdateSymlink updates an existing symlink.
func (s *Store) UpdateSymlink(l *model.Symlink) error {
	query := `UPDATE symlinks SET relative_path = ?, target_path = ?, type = ?, file_size = ?, modified_at = ?
	           WHERE id = ?`
	result, err := s.db.Exec(query, l.RelativePath, l.TargetPath, l.Type, l.FileSize, l.ModifiedAt, l.ID)
	if err != nil {
		return fmt.Errorf("failed to update symlink: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("symlink not found: %s", l.ID)
	}
	return nil
}

// DeleteSymlink deletes a symlink by ID.
func (s *Store) DeleteSymlink(id string) error {
	query := `DELETE FROM symlinks WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete symlink: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("symlink not found: %s", id)
	}
	return nil
}
