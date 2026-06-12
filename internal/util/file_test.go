package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")
	content := []byte("hello, world!")
	os.WriteFile(src, content, 0644)

	t.Run("copy file successfully", func(t *testing.T) {
		err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify content
		readBack, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read destination: %v", err)
		}
		if string(readBack) != string(content) {
			t.Fatalf("expected %s, got %s", content, readBack)
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		newContent := []byte("overwritten!")
		err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Change source and copy again
		os.WriteFile(src, newContent, 0644)
		err = CopyFile(src, dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		readBack, _ := os.ReadFile(dst)
		if string(readBack) != string(newContent) {
			t.Fatalf("expected %s, got %s", newContent, readBack)
		}
	})

	t.Run("non-existent source", func(t *testing.T) {
		err := CopyFile(filepath.Join(tmpDir, "nonexistent.txt"), dst)
		if err == nil {
			t.Fatal("expected error for non-existent source")
		}
	})

	t.Run("source is a directory", func(t *testing.T) {
		err := CopyFile(tmpDir, dst)
		if err == nil {
			t.Fatal("expected error when source is a directory")
		}
	})
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(tmpDir, "dstdir")

	// Create source directory structure
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("file a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("file b"), 0644)

	t.Run("copy directory successfully", func(t *testing.T) {
		err := CopyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files exist
		if _, err := os.Stat(filepath.Join(dstDir, "a.txt")); os.IsNotExist(err) {
			t.Fatal("a.txt not copied")
		}
		if _, err := os.Stat(filepath.Join(dstDir, "sub", "b.txt")); os.IsNotExist(err) {
			t.Fatal("sub/b.txt not copied")
		}

		// Verify content
		content, _ := os.ReadFile(filepath.Join(dstDir, "a.txt"))
		if string(content) != "file a" {
			t.Fatalf("expected 'file a', got %s", content)
		}
	})

	t.Run("non-existent source directory", func(t *testing.T) {
		err := CopyDir(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dest2"))
		if err == nil {
			t.Fatal("expected error for non-existent source")
		}
	})

	t.Run("source is a file not directory", func(t *testing.T) {
		err := CopyDir(filepath.Join(tmpDir, "a.txt"), filepath.Join(tmpDir, "dest3"))
		if err == nil {
			t.Fatal("expected error when source is a file")
		}
	})
}

func TestDetectMIME(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("text file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("hello world"), 0644)
		mime, err := DetectMIME(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != "text/plain" && mime != "text/plain; charset=utf-8" {
			t.Fatalf("expected text/plain, got %s", mime)
		}
	})

	t.Run("json file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.json")
		os.WriteFile(path, []byte(`{"key": "value"}`), 0644)
		mime, err := DetectMIME(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != "application/json" && mime != "text/plain; charset=utf-8" {
			t.Fatalf("expected application/json or text/plain, got %s", mime)
		}
	})

	t.Run("binary file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.bin")
		data := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
		os.WriteFile(path, data, 0644)
		mime, err := DetectMIME(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime == "" {
			t.Fatal("MIME type should not be empty")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := DetectMIME(filepath.Join(tmpDir, "nonexistent.txt"))
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})
}

func TestIsTextFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("text file returns true", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("hello world"), 0644)
		isText, err := IsTextFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isText {
			t.Fatal("expected true for text file")
		}
	})

	t.Run("binary file returns false", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.bin")
		os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644)
		isText, err := IsTextFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isText {
			t.Fatal("expected false for binary file")
		}
	})
}

func TestFileSize(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("get file size", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("hello"), 0644)
		size, err := FileSize(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != 5 {
			t.Fatalf("expected 5, got %d", size)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := FileSize(filepath.Join(tmpDir, "nonexistent.txt"))
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})
}
