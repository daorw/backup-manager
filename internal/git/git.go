package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CommitEntry represents a single Git commit in the log.
type CommitEntry struct {
	Hash    string
	Author  string
	Email   string
	Date    string
	Message string
}

// GitEngine wraps os/exec calls to the system git binary.
type GitEngine struct{}

// NewGitEngine creates a new GitEngine.
func NewGitEngine() *GitEngine {
	return &GitEngine{}
}

// runGitCommand executes a git command in the given directory.
func (e *GitEngine) runGitCommand(dir string, args []string, stdout, stderr *bytes.Buffer) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git command failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Init initializes a new Git repository at the given path.
func (e *GitEngine) Init(path string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(path, []string{"init"}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to init git repo: %w", err)
	}
	return nil
}

// Add stages a specific file or path.
func (e *GitEngine) Add(repoPath, filePath string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"add", filePath}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to git add: %w", err)
	}
	return nil
}

// AddAll stages all changes in the repository.
func (e *GitEngine) AddAll(repoPath string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"add", "-A"}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to git add -A: %w", err)
	}
	return nil
}

// Commit creates a new commit with the given message.
func (e *GitEngine) Commit(repoPath, message string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"commit", "-m", message}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to git commit: %w", err)
	}
	return nil
}

// CommitWithAuthor creates a new commit with custom author information.
func (e *GitEngine) CommitWithAuthor(repoPath, message, authorName, authorEmail string) error {
	var stderr bytes.Buffer
	args := []string{"commit", "-m", message,
		fmt.Sprintf("--author=%s <%s>", authorName, authorEmail)}
	err := e.runGitCommand(repoPath, args, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to git commit with author: %w", err)
	}
	return nil
}

// Log returns the commit history for the repository.
func (e *GitEngine) Log(repoPath string, limit, offset int) ([]CommitEntry, error) {
	format := "--format=%H%x00%an%x00%ae%x00%ai%x00%s"
	args := []string{"log", format}
	if limit > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", limit))
	}
	if offset > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", offset))
	}

	var stdout, stderr bytes.Buffer
	err := e.runGitCommand(repoPath, args, &stdout, &stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	return parseLogOutput(stdout.String())
}

// Status returns the working tree status.
func (e *GitEngine) Status(repoPath string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"status", "--porcelain"}, &stdout, &stderr)
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ConfigSet sets a git config value in the repository.
func (e *GitEngine) ConfigSet(repoPath, key, value string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"config", key, value}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to set git config: %w", err)
	}
	return nil
}

// Push pushes commits to the remote repository with optional authentication
// via environment variables (GIT_SSH_COMMAND for SSH keys).
func (e *GitEngine) Push(repoPath, remote, branch string, envVars []string) error {
	args := []string{"push", remote, branch}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	cmd.Env = append(cmd.Env, envVars...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to git push: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// RemoteSetURL sets the remote origin URL.
func (e *GitEngine) RemoteSetURL(repoPath, url string) error {
	var stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"remote", "remove", "origin"}, &bytes.Buffer{}, &stderr)
	if err != nil {
		// Ignore error if origin doesn't exist
	}
	err = e.runGitCommand(repoPath, []string{"remote", "add", "origin", url}, &bytes.Buffer{}, &stderr)
	if err != nil {
		return fmt.Errorf("failed to set remote origin: %w", err)
	}
	return nil
}

// parseLogOutput parses the git log output into CommitEntry slices.
func parseLogOutput(output string) ([]CommitEntry, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return []CommitEntry{}, nil
	}

	var entries []CommitEntry
	for _, line := range lines {
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) < 5 {
			continue
		}
		entries = append(entries, CommitEntry{
			Hash:    parts[0],
			Author:  parts[1],
			Email:   parts[2],
			Date:    parts[3],
			Message: parts[4],
		})
	}
	return entries, nil
}
