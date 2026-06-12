package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DataDirName is the name of the data directory within a repo root.
const DataDirName = "data"

// FileEntry represents a file entry from git ls-tree.
type FileEntry struct {
	Mode string // e.g., "100644", "100755", "040000"
	Path string // relative to repo root
}

// CommitFileEntry represents a file changed in a commit diff.
// ChangeType: 'A' (Added), 'M' (Modified), 'D' (Deleted).
type CommitFileEntry struct {
	ChangeType string
	Path       string
}

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
// Returns an empty list if the repository has no commits yet.
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
		// An empty repository (no commits yet) causes git log to exit non-zero.
		// Return an empty list instead of an error.
		if strings.Contains(stderr.String(), "does not have any commits yet") {
			return []CommitEntry{}, nil
		}
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

// PushOption configures git push behavior.
type PushOption func(*pushConfig)

type pushConfig struct {
	force bool
}

// WithForce enables --force flag on git push.
func WithForce() PushOption {
	return func(cfg *pushConfig) {
		cfg.force = true
	}
}

// Push pushes commits to the remote repository with optional authentication
// via environment variables (GIT_SSH_COMMAND for SSH keys).
func (e *GitEngine) Push(repoPath, remote, branch string, envVars []string, opts ...PushOption) error {
	cfg := &pushConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	args := []string{"push", remote, branch}
	if cfg.force {
		args = []string{"push", "--force", remote, branch}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), envVars...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to git push: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// GetRemoteURL returns the URL of a remote.
func (e *GitEngine) GetRemoteURL(repoPath, remote string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := e.runGitCommand(repoPath, []string{"remote", "get-url", remote}, &stdout, &stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
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

// WriteFileContentTo streams the content of a file from a specific commit
// directly to a destination file, avoiding loading the entire content into memory.
// filePath is relative to the repo root (e.g., "data/notes/file.md").
func (e *GitEngine) WriteFileContentTo(repoPath, commitHash, filePath, destPath string, perm os.FileMode) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent dir: %w", err)
	}

	args := []string{"show", fmt.Sprintf("%s:%s", commitHash, filePath)}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Pipe stdout directly to file — zero memory copy for content
	cmd.Stdout = dstFile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git show failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// LsTree lists the contents of a tree object at the given commit.
// treePath is the directory path within the commit (e.g., "data/notes").
func (e *GitEngine) LsTree(repoPath, commitHash, treePath string) ([]FileEntry, error) {
	var stdout, stderr bytes.Buffer
	args := []string{"ls-tree", "-r", commitHash, treePath}
	err := e.runGitCommand(repoPath, args, &stdout, &stderr)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %w", err)
	}

	return parseLsTreeOutput(stdout.String()), nil
}

// GetCommitFileMode returns the file mode for a specific path in a commit.
func (e *GitEngine) GetCommitFileMode(repoPath, commitHash, filePath string) (os.FileMode, error) {
	entries, err := e.LsTree(repoPath, commitHash, filePath)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, fmt.Errorf("path %q not found in commit %s", filePath, commitHash)
	}
	return gitModeToFileMode(entries[0].Mode), nil
}

// IsRootCommit checks whether the given commit is a root commit (has no parent).
// Uses: git rev-parse --verify <commit>^ which exits non-zero if no parent exists.
func (e *GitEngine) IsRootCommit(repoPath, commitHash string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", commitHash+"^")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check root commit: %w", err)
	}
	return false, nil
}

// GetChangedFilesInCommit returns the list of files changed under data/
// in a specific commit, compared to its parent.
// For root commits, returns all files under data/.
func (e *GitEngine) GetChangedFilesInCommit(repoPath, commitHash string) ([]string, error) {
	isRoot, err := e.IsRootCommit(repoPath, commitHash)
	if err != nil {
		return nil, err
	}

	if isRoot {
		// Root commit: list all files under data/
		entries, err := e.LsTree(repoPath, commitHash, DataDirName+"/")
		if err != nil {
			// data/ might not exist in this commit
			return nil, nil
		}
		paths := make([]string, 0, len(entries))
		for _, entry := range entries {
			paths = append(paths, entry.Path)
		}
		return paths, nil
	}

	// Non-root commit: use diff-tree to get changed files
	var stdout, stderr bytes.Buffer
	args := []string{"diff-tree", "--no-commit-id", "-r", "--name-only",
		"--diff-filter=ACDMRT", "-m", commitHash}
	err = e.runGitCommand(repoPath, args, &stdout, &stderr)
	if err != nil {
		return nil, fmt.Errorf("git diff-tree failed: %w", err)
	}

	var dataFiles []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, DataDirName+"/") {
			dataFiles = append(dataFiles, line)
		}
	}
	return dataFiles, nil
}

// gitModeToFileMode converts a git mode string (e.g. "100755") to os.FileMode.
func gitModeToFileMode(gitMode string) os.FileMode {
	mode, err := strconv.ParseUint(gitMode, 8, 32)
	if err != nil {
		return 0644
	}
	return os.FileMode(mode) & os.ModePerm
}

// parseLsTreeOutput parses git ls-tree output into FileEntry slices.
// Input format: <mode> <type> <hash>\t<path>
func parseLsTreeOutput(output string) []FileEntry {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	entries := make([]FileEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		modeFields := strings.Fields(parts[0])
		if len(modeFields) > 0 {
			entries = append(entries, FileEntry{
				Mode: modeFields[0],
				Path: parts[1],
			})
		}
	}
	return entries
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
