package model

import "time"

// GitAuthType represents the type of Git authentication.
type GitAuthType string

const (
	GitAuthNone     GitAuthType = "none"
	GitAuthSSHKey   GitAuthType = "ssh_key"
	GitAuthPassword GitAuthType = "password"
)

// GitAuth represents Git authentication configuration for a repository.
type GitAuth struct {
	RepoID            string      `json:"repo_id"`
	AuthType          GitAuthType `json:"auth_type"`
	SSHPrivateKey     string      `json:"ssh_private_key,omitempty"`
	SSHPrivateKeyPath string      `json:"ssh_private_key_path,omitempty"`
	Username          string      `json:"username,omitempty"`
	PasswordEncrypted []byte      `json:"password_encrypted,omitempty"`
	UpdatedAt         time.Time   `json:"updated_at"`
}
