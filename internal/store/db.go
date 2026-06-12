package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database at the given path with WAL mode enabled.
func OpenDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return db, nil
}

// Migrate creates all required tables if they don't exist.
func Migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS repos (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		path          TEXT NOT NULL UNIQUE,
		created_at    DATETIME DEFAULT (datetime('now')),
		updated_at    DATETIME DEFAULT (datetime('now')),
		last_backup_at DATETIME,
		status        TEXT DEFAULT 'active'
	);

	CREATE TABLE IF NOT EXISTS repo_configs (
		repo_id             TEXT PRIMARY KEY,
		remote_url          TEXT,
		branch              TEXT DEFAULT 'main',
		auto_backup         INTEGER DEFAULT 0,
		auto_backup_interval TEXT,
		git_user_name       TEXT,
		git_user_email      TEXT,
		FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS repo_auths (
		repo_id             TEXT PRIMARY KEY,
		auth_type           TEXT NOT NULL DEFAULT 'none',
		ssh_private_key     BLOB,
		ssh_private_key_path TEXT,
		username            TEXT,
		password_encrypted  BLOB,
		updated_at          DATETIME DEFAULT (datetime('now')),
		FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS symlinks (
		id              TEXT PRIMARY KEY,
		repo_id         TEXT NOT NULL,
		relative_path   TEXT NOT NULL,
		target_path     TEXT NOT NULL,
		type            TEXT NOT NULL,
		file_size       INTEGER,
		modified_at     DATETIME,
		created_at      DATETIME DEFAULT (datetime('now')),
		FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE,
		UNIQUE(repo_id, relative_path)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
