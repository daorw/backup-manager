package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitEngine_Init(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewGitEngine()

	t.Run("init git repo", func(t *testing.T) {
		err := engine.Init(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify .git directory exists
		if _, err := os.Stat(filepath.Join(tmpDir, ".git")); os.IsNotExist(err) {
			t.Fatal(".git directory was not created")
		}
	})

	t.Run("init in non-existent directory", func(t *testing.T) {
		err := engine.Init("/nonexistent/path/repo")
		if err == nil {
			t.Fatal("expected error for non-existent directory")
		}
	})
}

func TestGitEngine_AddCommitLog(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewGitEngine()

	// Init git repo
	if err := engine.Init(tmpDir); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Set up user config
	cmd := exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "test")
	cmd.Dir = tmpDir
	cmd.Run()

	t.Run("add and commit file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("hello"), 0644)

		err := engine.Add(tmpDir, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = engine.Commit(tmpDir, "initial commit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("add all files", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "another.txt")
		os.WriteFile(testFile, []byte("world"), 0644)

		err := engine.AddAll(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = engine.Commit(tmpDir, "another file")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("get log", func(t *testing.T) {
		log, err := engine.Log(tmpDir, 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(log) == 0 {
			t.Fatal("expected at least one commit in log")
		}
	})

	t.Run("commit with author", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "author_test.txt")
		os.WriteFile(testFile, []byte("author test"), 0644)

		engine.AddAll(tmpDir)
		err := engine.CommitWithAuthor(tmpDir, "author commit", "Custom Author", "custom@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		log, _ := engine.Log(tmpDir, 1, 0)
		if len(log) > 0 && log[0].Author != "Custom Author" {
			t.Fatalf("expected author 'Custom Author', got %s", log[0].Author)
		}
	})
}

func TestGitEngine_Status(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewGitEngine()

	engine.Init(tmpDir)
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "config", "user.name", "test").Run()

	t.Run("status returns no error", func(t *testing.T) {
		status, err := engine.Status(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Status should be present as a string
		if status == "" {
			t.Log("status is empty (clean repo)")
		}
	})
}

func TestNewGitEngine(t *testing.T) {
	engine := NewGitEngine()
	if engine == nil {
		t.Fatal("expected non-nil GitEngine")
	}
}

func TestGitEngine_AddErrors(t *testing.T) {
	engine := NewGitEngine()

	t.Run("add in non-git directory", func(t *testing.T) {
		err := engine.Add(t.TempDir(), "test.txt")
		if err == nil {
			t.Fatal("expected error in non-git directory")
		}
	})

	t.Run("add all in non-git directory", func(t *testing.T) {
		err := engine.AddAll(t.TempDir())
		if err == nil {
			t.Fatal("expected error in non-git directory")
		}
	})

	t.Run("commit in non-git directory", func(t *testing.T) {
		err := engine.Commit(t.TempDir(), "msg")
		if err == nil {
			t.Fatal("expected error in non-git directory")
		}
	})

	t.Run("log in non-git directory", func(t *testing.T) {
		_, err := engine.Log(t.TempDir(), 10, 0)
		if err == nil {
			t.Fatal("expected error in non-git directory")
		}
	})

	t.Run("status in non-git directory", func(t *testing.T) {
		_, err := engine.Status(t.TempDir())
		if err == nil {
			t.Fatal("expected error in non-git directory")
		}
	})
}

func TestGitCommandOutput(t *testing.T) {
	// Test that runGitCommand properly captures stderr on failure
	engine := NewGitEngine()
	tmpDir := t.TempDir()

	t.Run("non-existent command captures stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := engine.runGitCommand(tmpDir, []string{"--invalid-flag"}, &stdout, &stderr)
		if err == nil {
			t.Fatal("expected error for invalid git command")
		}
		if stderr.Len() == 0 {
			t.Fatal("expected stderr to contain error message")
		}
	})
}
