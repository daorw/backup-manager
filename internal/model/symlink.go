package model

import "time"

// SymlinkType represents whether a symlink points to a file or a directory.
type SymlinkType string

const (
	SymlinkTypeFile      SymlinkType = "file"
	SymlinkTypeDirectory SymlinkType = "directory"
)

// Symlink represents a symlink entry in a backup repository.
type Symlink struct {
	ID           string      `json:"id"`
	RepoID       string      `json:"repo_id"`
	RelativePath string      `json:"relative_path"`
	TargetPath   string      `json:"target_path"`
	Type         SymlinkType `json:"type"`
	FileSize     int64       `json:"file_size,omitempty"`
	ModifiedAt   *time.Time  `json:"modified_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}
