package service

import (
	"fmt"
	"os"
	"path/filepath"

	"backup-manager/internal/model"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

// AuthService handles Git authentication business logic.
type AuthService struct {
	store      *store.Store
	keyManager *util.KeyManager
}

// NewAuthService creates a new AuthService.
func NewAuthService(s *store.Store, km *util.KeyManager) *AuthService {
	return &AuthService{store: s, keyManager: km}
}

// SetAuthRequest is the input for setting Git authentication.
type SetAuthRequest struct {
	AuthType          model.GitAuthType `json:"auth_type"`
	SSHPrivateKey     string            `json:"ssh_private_key,omitempty"`
	SSHPrivateKeyPath string            `json:"ssh_private_key_path,omitempty"`
	Username          string            `json:"username,omitempty"`
	Password          string            `json:"password,omitempty"`
}

// AuthResponse is the safe response (without sensitive data).
type AuthResponse struct {
	RepoID            string            `json:"repo_id"`
	AuthType          model.GitAuthType `json:"auth_type"`
	SSHPrivateKeyPath string            `json:"ssh_private_key_path,omitempty"`
	Username          string            `json:"username,omitempty"`
	HasPassword       bool              `json:"has_password"`
	HasSSHKey         bool              `json:"has_ssh_key"`
}

// Get returns the auth config for a repo (with sensitive fields masked).
func (s *AuthService) Get(repoID string) (*AuthResponse, error) {
	auth, err := s.store.GetRepoAuth(repoID)
	if err != nil {
		return nil, err
	}

	resp := &AuthResponse{
		RepoID:            repoID,
		AuthType:          auth.AuthType,
		SSHPrivateKeyPath: auth.SSHPrivateKeyPath,
		Username:          auth.Username,
		HasPassword:       len(auth.PasswordEncrypted) > 0,
		HasSSHKey:         auth.SSHPrivateKey != "",
	}

	// Validate auth_type
	if resp.AuthType == "" {
		resp.AuthType = model.GitAuthNone
	}

	return resp, nil
}

// Set configures Git authentication for a repo.
// Sensitive fields (SSH key, password) are encrypted before storage.
func (s *AuthService) Set(repoID string, req *SetAuthRequest) error {
	if req.AuthType == "" {
		req.AuthType = model.GitAuthNone
	}

	auth := &model.GitAuth{
		RepoID:            repoID,
		AuthType:          req.AuthType,
		SSHPrivateKeyPath: req.SSHPrivateKeyPath,
		Username:          req.Username,
	}

	switch req.AuthType {
	case model.GitAuthSSHKey:
		// Encrypt SSH private key content
		if req.SSHPrivateKey != "" {
			encrypted, err := s.keyManager.Encrypt([]byte(req.SSHPrivateKey))
			if err != nil {
				return fmt.Errorf("failed to encrypt SSH key: %w", err)
			}
			auth.SSHPrivateKey = string(encrypted)

			// If no key path specified, write to a temp file in the repo
			if auth.SSHPrivateKeyPath == "" {
				keyPath, err := s.writeSSHKeyFile(repoID, req.SSHPrivateKey)
				if err != nil {
					return fmt.Errorf("failed to write SSH key file: %w", err)
				}
				auth.SSHPrivateKeyPath = keyPath
			}
		}

	case model.GitAuthPassword:
		if req.Password != "" {
			encrypted, err := s.keyManager.Encrypt([]byte(req.Password))
			if err != nil {
				return fmt.Errorf("failed to encrypt password: %w", err)
			}
			auth.PasswordEncrypted = encrypted
		}

	case model.GitAuthNone:
		// Clear all auth data

	default:
		return fmt.Errorf("invalid auth type: %s", req.AuthType)
	}

	// Check if auth exists
	_, err := s.store.GetRepoAuth(repoID)
	if err != nil {
		// Create new auth
		return s.store.CreateRepoAuth(auth)
	}

	return s.store.UpdateRepoAuth(auth)
}

// Clear removes all authentication config for a repo.
func (s *AuthService) Clear(repoID string) error {
	// Clean up SSH key file if it exists
	if auth, err := s.store.GetRepoAuth(repoID); err == nil {
		if auth.SSHPrivateKeyPath != "" {
			os.Remove(auth.SSHPrivateKeyPath)
		}
	}

	return s.store.DeleteRepoAuth(repoID)
}

// BuildEnvVars builds environment variables for git operations based on auth config.
func (s *AuthService) BuildEnvVars(repoID string) []string {
	auth, err := s.store.GetRepoAuth(repoID)
	if err != nil {
		return nil
	}

	var envVars []string

	switch auth.AuthType {
	case model.GitAuthSSHKey:
		if auth.SSHPrivateKeyPath != "" {
			// Ensure the key file has correct permissions
			if info, err := os.Stat(auth.SSHPrivateKeyPath); err == nil {
				if info.Mode() != 0600 {
					os.Chmod(auth.SSHPrivateKeyPath, 0600)
				}
			}
			sshCmd := fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", auth.SSHPrivateKeyPath)
			envVars = append(envVars, sshCmd)
		}

	case model.GitAuthPassword:
		if len(auth.PasswordEncrypted) > 0 && auth.Username != "" {
			// Decrypt password
			password, err := s.keyManager.Decrypt(auth.PasswordEncrypted)
			if err == nil {
				envVars = append(envVars, fmt.Sprintf("GIT_USERNAME=%s", auth.Username))
				envVars = append(envVars, fmt.Sprintf("GIT_PASSWORD=%s", string(password)))
			}
		}
	}

	return envVars
}

// writeSSHKeyFile writes the SSH private key to a temporary file.
func (s *AuthService) writeSSHKeyFile(repoID, keyContent string) (string, error) {
	keyDir := filepath.Join(os.TempDir(), "backup-manager-ssh")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create SSH key directory: %w", err)
	}

	keyPath := filepath.Join(keyDir, fmt.Sprintf("%s_id_rsa", repoID))
	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write SSH key file: %w", err)
	}

	return keyPath, nil
}
