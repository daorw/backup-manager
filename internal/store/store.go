package store

import "database/sql"

// Store wraps the SQLite database and provides typed accessors
// for each entity.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store backed by the given *sql.DB.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}
