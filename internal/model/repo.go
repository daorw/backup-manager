package model

import "time"

// RepoStatus represents the current status of a backup repository.
type RepoStatus string

const (
	RepoStatusActive   RepoStatus = "active"
	RepoStatusError    RepoStatus = "error"
	RepoStatusBackingUp RepoStatus = "backing_up"
)

// Repo represents a backup repository.
type Repo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastBackupAt *time.Time `json:"last_backup_at,omitempty"`
	Status       RepoStatus `json:"status"`
}

// RepoConfig represents the configuration of a backup repository.
type RepoConfig struct {
	RepoID             string `json:"repo_id"`
	RemoteURL          string `json:"remote_url,omitempty"`
	Branch             string `json:"branch,omitempty"`
	AutoBackup         bool   `json:"auto_backup"`
	AutoBackupInterval string `json:"auto_backup_interval,omitempty"`
	GitUserName        string `json:"git_user_name,omitempty"`
	GitUserEmail       string `json:"git_user_email,omitempty"`
}
