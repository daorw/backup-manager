package util

import (
	"fmt"
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

func TestSafeResolve_RootIsSlash(t *testing.T) {
	t.Run("root slash itself", func(t *testing.T) {
		resolved, err := SafeResolve("/", "/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		absRoot, _ := filepath.EvalSymlinks("/")
		if resolved != "/" && resolved != absRoot {
			t.Fatalf("expected '/' or '%s', got '%s'", absRoot, resolved)
		}
	})

	t.Run("absolute path under root slash", func(t *testing.T) {
		tmp := t.TempDir()
		resolved, err := SafeResolve("/", tmp)
		if err != nil {
			t.Fatalf("unexpected error for path %q under root '/': %v", tmp, err)
		}
		expected, _ := filepath.EvalSymlinks(tmp)
		if resolved != expected && resolved != tmp {
			t.Fatalf("expected %s or %s, got %s", tmp, expected, resolved)
		}
	})

	t.Run("system tmp under root slash", func(t *testing.T) {
		resolved, err := SafeResolve("/", "/tmp")
		if err != nil {
			t.Fatalf("unexpected error for /tmp under root '/': %v", err)
		}
		absTmp, _ := filepath.EvalSymlinks("/tmp")
		if resolved != "/tmp" && resolved != absTmp {
			t.Fatalf("expected '/tmp' or '%s', got '%s'", absTmp, resolved)
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

func TestResolveNestedSymlink(t *testing.T) {
	t.Run("resolves single symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		link := filepath.Join(tmpDir, "link.txt")
		os.Symlink(target, link)

		resolved, chain, err := ResolveNestedSymlink(link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 1 {
			t.Fatalf("expected chain length 1, got %d", len(chain))
		}
		if chain[0] != link {
			t.Fatalf("expected chain[0]=%s, got %s", link, chain[0])
		}
	})

	t.Run("resolves nested symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		link1 := filepath.Join(tmpDir, "link1.txt")
		link2 := filepath.Join(tmpDir, "link2.txt")
		link3 := filepath.Join(tmpDir, "link3.txt")

		os.Symlink(target, link1)
		os.Symlink(link1, link2)
		os.Symlink(link2, link3)

		resolved, chain, err := ResolveNestedSymlink(link3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 3 {
			t.Fatalf("expected chain length 3, got %d", len(chain))
		}
	})

	t.Run("detects cycle", func(t *testing.T) {
		tmpDir := t.TempDir()
		link1 := filepath.Join(tmpDir, "link1")
		link2 := filepath.Join(tmpDir, "link2")

		os.Symlink(link2, link1)
		os.Symlink(link1, link2)

		_, _, err := ResolveNestedSymlink(link1)
		if err == nil {
			t.Fatal("expected error for cycle")
		}
	})

	t.Run("returns non-symlink path as-is", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "regular.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		resolved, chain, err := ResolveNestedSymlink(target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 0 {
			t.Fatalf("expected empty chain, got %d", len(chain))
		}
	})

	t.Run("respects depth limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a chain of 11 symlinks to exceed the limit of 10
		target := filepath.Join(tmpDir, "target.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		prev := target
		for i := 0; i < 11; i++ {
			link := filepath.Join(tmpDir, fmt.Sprintf("link%d", i))
			os.Symlink(prev, link)
			prev = link
		}

		_, _, err := ResolveNestedSymlink(prev)
		if err == nil {
			t.Fatal("expected error for exceeding depth limit")
		}
	})
}

func TestSafeResolveNestedSymlink(t *testing.T) {
	t.Run("resolves symlink within allowed root", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		link := filepath.Join(root, "link.txt")
		os.Symlink(target, link)

		resolved, chain, err := SafeResolveNestedSymlink(root, link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 1 {
			t.Fatalf("expected chain length 1, got %d", len(chain))
		}
	})

	t.Run("blocks symlink escaping root", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		target := filepath.Join(outside, "secret.txt")
		os.WriteFile(target, []byte("secret"), 0644)

		link := filepath.Join(root, "escape.txt")
		os.Symlink(target, link)

		_, _, err := SafeResolveNestedSymlink(root, link)
		if err == nil {
			t.Fatal("expected error for symlink escaping root")
		}
	})

	t.Run("blocks nested symlink escaping root", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		target := filepath.Join(outside, "secret.txt")
		os.WriteFile(target, []byte("secret"), 0644)

		link1 := filepath.Join(root, "link1")
		link2 := filepath.Join(root, "link2")
		os.Symlink(target, link1)
		os.Symlink(link1, link2)

		_, _, err := SafeResolveNestedSymlink(root, link2)
		if err == nil {
			t.Fatal("expected error for nested symlink escaping root")
		}
	})

	t.Run("detects cycle within allowed root", func(t *testing.T) {
		root := t.TempDir()
		link1 := filepath.Join(root, "link1")
		link2 := filepath.Join(root, "link2")

		os.Symlink(link2, link1)
		os.Symlink(link1, link2)

		_, _, err := SafeResolveNestedSymlink(root, link1)
		if err == nil {
			t.Fatal("expected error for cycle")
		}
	})
}

func TestSafeResolveWithSymlinkChain(t *testing.T) {
	t.Run("returns chain for nested symlinks", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		link1 := filepath.Join(root, "link1.txt")
		link2 := filepath.Join(root, "link2.txt")
		os.Symlink(target, link1)
		os.Symlink(link1, link2)

		resolved, chain, err := SafeResolveWithSymlinkChain(root, link2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 2 {
			t.Fatalf("expected chain length 2, got %d", len(chain))
		}
	})

	t.Run("returns empty chain for non-symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "regular.txt")
		os.WriteFile(target, []byte("hello"), 0644)

		resolved, chain, err := SafeResolveWithSymlinkChain(root, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTarget, _ := filepath.EvalSymlinks(target)
		if resolved != expectedTarget {
			t.Fatalf("expected %s, got %s", expectedTarget, resolved)
		}
		if len(chain) != 0 {
			t.Fatalf("expected empty chain, got %d", len(chain))
		}
	})

	t.Run("blocks escaping symlinks", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		target := filepath.Join(outside, "secret.txt")
		os.WriteFile(target, []byte("secret"), 0644)

		link := filepath.Join(root, "escape.txt")
		os.Symlink(target, link)

		_, _, err := SafeResolveWithSymlinkChain(root, link)
		if err == nil {
			t.Fatal("expected error for escaping symlink")
		}
	})
}
