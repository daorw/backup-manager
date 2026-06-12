package git

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary Git repository with a data/ directory
// and a single commit containing test files.
func setupTestRepo(t *testing.T) (repoPath string, commitHash string, cleanup func()) {
	t.Helper()

	repoPath = t.TempDir()
	engine := NewGitEngine()

	// Init git repo
	if err := engine.Init(repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// git config user
	engine.ConfigSet(repoPath, "user.name", "Test")
	engine.ConfigSet(repoPath, "user.email", "test@test.com")

	// Create data/ directory
	dataDir := filepath.Join(repoPath, DataDirName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create a file in data/
	file1 := filepath.Join(dataDir, "test.txt")
	if err := os.WriteFile(file1, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(dataDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	file2 := filepath.Join(subDir, "nested.txt")
	if err := os.WriteFile(file2, []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}

	// Create an executable file
	scriptPath := filepath.Join(dataDir, "script.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// git add and commit
	if err := engine.AddAll(repoPath); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := engine.Commit(repoPath, "Initial commit"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Get the commit hash
	entries, err := engine.Log(repoPath, 1, 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("failed to get commit hash: %v", err)
	}
	commitHash = entries[0].Hash

	cleanup = func() {
		os.RemoveAll(repoPath)
	}

	return
}

func TestIsRootCommit_RootCommit(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	isRoot, err := engine.IsRootCommit(repoPath, commitHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isRoot {
		t.Error("expected root commit, got false")
	}
}

func TestIsRootCommit_NonRootCommit(t *testing.T) {
	repoPath, _, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()

	// Create a second commit
	filePath := filepath.Join(repoPath, DataDirName, "new.txt")
	if err := os.WriteFile(filePath, []byte("new"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := engine.AddAll(repoPath); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := engine.Commit(repoPath, "Second commit"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	entries, err := engine.Log(repoPath, 1, 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("failed to get latest commit: %v", err)
	}

	isRoot, err := engine.IsRootCommit(repoPath, entries[0].Hash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isRoot {
		t.Error("expected non-root commit, got true")
	}
}

func TestGetChangedFilesInCommit_RootCommit(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	files, err := engine.GetChangedFilesInCommit(repoPath, commitHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := map[string]bool{
		"data/test.txt":    false,
		"data/sub/nested.txt": false,
		"data/script.sh":   false,
	}

	for _, f := range files {
		if _, ok := expectedFiles[f]; ok {
			expectedFiles[f] = true
		}
	}

	for path, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %q in root commit output", path)
		}
	}
}

func TestGetChangedFilesInCommit_NonRootCommit(t *testing.T) {
	repoPath, _, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()

	// Create a second commit with changes
	filePath := filepath.Join(repoPath, DataDirName, "test.txt")
	if err := os.WriteFile(filePath, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	if err := engine.AddAll(repoPath); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := engine.Commit(repoPath, "Modified test.txt"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	entries, err := engine.Log(repoPath, 1, 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("failed to get latest commit: %v", err)
	}

	files, err := engine.GetChangedFilesInCommit(repoPath, entries[0].Hash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, f := range files {
		if f == "data/test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected data/test.txt in changed files")
	}
}

func TestGetCommitFileMode_RegularFile(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	mode, err := engine.GetCommitFileMode(repoPath, commitHash, "data/test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != 0644 {
		t.Errorf("expected mode 0644, got %o", mode)
	}
}

func TestGetCommitFileMode_ExecutableFile(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	mode, err := engine.GetCommitFileMode(repoPath, commitHash, "data/script.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != 0755 {
		t.Errorf("expected mode 0755, got %o", mode)
	}
}

func TestGetCommitFileMode_NotFound(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	_, err := engine.GetCommitFileMode(repoPath, commitHash, "data/nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLsTree(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()
	entries, err := engine.LsTree(repoPath, commitHash, DataDirName+"/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify script.sh has executable mode
	for _, entry := range entries {
		if entry.Path == "data/script.sh" {
			if entry.Mode != "100755" {
				t.Errorf("expected mode 100755 for script.sh, got %s", entry.Mode)
			}
		}
	}
}

func TestGitModeToFileMode(t *testing.T) {
	tests := []struct {
		gitMode  string
		expected os.FileMode
	}{
		{"100644", 0644},
		{"100755", 0755},
		{"120000", 0}, // symlink mode masked by ModePerm = 0
		{"040000", 0}, // directory mode masked by ModePerm = 0
	}

	for _, tt := range tests {
		result := gitModeToFileMode(tt.gitMode)
		if result != tt.expected {
			t.Errorf("gitModeToFileMode(%q) = %o, want %o", tt.gitMode, result, tt.expected)
		}
	}
}

func TestWriteFileContentTo(t *testing.T) {
	repoPath, commitHash, cleanup := setupTestRepo(t)
	defer cleanup()

	engine := NewGitEngine()

	// Write to a temporary location
	destPath := filepath.Join(t.TempDir(), "restored.txt")
	err := engine.WriteFileContentTo(repoPath, commitHash, "data/test.txt", destPath, 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(content))
	}
}
