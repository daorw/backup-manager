package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeResolve(t *testing.T) {
	// Create temp dir structure
	root := t.TempDir()
	subDir := filepath.Join(root, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("world"), 0644)

	t.Run("absolute path within root", func(t *testing.T) {
		path := filepath.Join(root, "a.txt")
		resolved, err := SafeResolve(root, path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := filepath.EvalSymlinks(filepath.Join(root, "a.txt"))
		if resolved != expected && resolved != filepath.Join(root, "a.txt") {
			t.Fatalf("expected %s or %s, got %s", filepath.Join(root, "a.txt"), expected, resolved)
		}
	})

	t.Run("relative path within root", func(t *testing.T) {
		resolved, err := SafeResolve(root, "a.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := filepath.EvalSymlinks(filepath.Join(root, "a.txt"))
		if resolved != expected && resolved != filepath.Join(root, "a.txt") {
			t.Fatalf("expected %s or %s, got %s", filepath.Join(root, "a.txt"), expected, resolved)
		}
	})

	t.Run("path traversal attack", func(t *testing.T) {
		_, err := SafeResolve(root, "../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})

	t.Run("non-existent file within root is allowed", func(t *testing.T) {
		// Per P1-5: EvalSymlinks ErrNotExist degrades gracefully; non-existent
		// paths within the allowed root are still valid.
		resolved, err := SafeResolve(root, "nonexistent.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The resolved path includes symlink resolution (e.g., /tmp → /private/tmp)
		expected := filepath.Join(root, "nonexistent.txt")
		realRoot, _ := filepath.EvalSymlinks(root)
		expectedReal := filepath.Join(realRoot, "nonexistent.txt")
		if resolved != expected && resolved != expectedReal {
			t.Fatalf("expected %s or %s, got %s", expected, expectedReal, resolved)
		}
	})

	t.Run("absolute path outside root", func(t *testing.T) {
		_, err := SafeResolve(root, "/etc/passwd")
		if err == nil {
			t.Fatal("expected error for path outside root")
		}
	})

	t.Run("root itself is valid", func(t *testing.T) {
		resolved, err := SafeResolve(root, root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The resolved path may differ due to symlink resolution (e.g., /tmp → /private/tmp)
		absRoot, _ := filepath.EvalSymlinks(root)
		if resolved != absRoot && resolved != root {
			t.Fatalf("expected %s or %s, got %s", root, absRoot, resolved)
		}
	})

	t.Run("subdirectory file", func(t *testing.T) {
		resolved, err := SafeResolve(root, "sub/b.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := filepath.EvalSymlinks(filepath.Join(root, "sub", "b.txt"))
		if resolved != expected && resolved != filepath.Join(root, "sub", "b.txt") {
			t.Fatalf("expected %s or %s, got %s", filepath.Join(root, "sub", "b.txt"), expected, resolved)
		}
	})

	t.Run("empty root", func(t *testing.T) {
		_, err := SafeResolve("", "a.txt")
		if err == nil {
			t.Fatal("expected error for empty root")
		}
	})

	t.Run("symlink escape through non-existent path is blocked", func(t *testing.T) {
		// Create a symlink inside the root that points outside
		outsideDir := t.TempDir()
		outsideFile := filepath.Join(outsideDir, "secret.txt")
		os.WriteFile(outsideFile, []byte("secret"), 0644)

		linkDir := filepath.Join(root, "escape_link")
		os.Symlink(outsideDir, linkDir)

		// Try to access a non-existent file through the escaped symlink
		_, err := SafeResolve(root, "escape_link/nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for path escaping root via symlink")
		}

		// Also try to access an existing file outside through the symlink
		_, err = SafeResolve(root, "escape_link/secret.txt")
		if err == nil {
			t.Fatal("expected error for existing file escaping root via symlink")
		}
	})
}

func TestSafeResolveFile(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0644)
	subDir := filepath.Join(root, "sub")
	os.MkdirAll(subDir, 0755)

	t.Run("existing file", func(t *testing.T) {
		resolved, err := SafeResolveFile(root, "a.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := filepath.EvalSymlinks(filepath.Join(root, "a.txt"))
		if resolved != expected && resolved != filepath.Join(root, "a.txt") {
			t.Fatalf("expected %s or %s, got %s", filepath.Join(root, "a.txt"), expected, resolved)
		}
	})

	t.Run("directory returns error", func(t *testing.T) {
		_, err := SafeResolveFile(root, "sub")
		if err == nil {
			t.Fatal("expected error for directory")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := SafeResolveFile(root, "nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})
}
