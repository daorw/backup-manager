package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// DetectMIMEReadSize is the number of bytes read for MIME detection.
	DetectMIMEReadSize = 512
)

// CopyFile copies a file from src to dst. If the destination file already
// exists, it will be overwritten. The destination directory must exist.
func CopyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, use CopyDir instead")
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Preserve file permissions
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst.
// The destination directory will be created if it does not exist.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// DetectMIME reads the first 512 bytes of a file and returns its MIME type.
func DetectMIME(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, DetectMIMEReadSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}
	buf = buf[:n]

	// DetectContentType uses the first bytes to determine MIME type.
	mimeType := http.DetectContentType(buf)
	return mimeType, nil
}

// IsTextFile checks whether a file is a text file by detecting its MIME type.
func IsTextFile(filePath string) (bool, error) {
	mime, err := DetectMIME(filePath)
	if err != nil {
		return false, err
	}

	// Common text MIME types
	switch mime {
	case "text/plain",
		"text/html",
		"text/css",
		"text/javascript",
		"application/json",
		"application/xml",
		"application/x-yaml",
		"application/x-sh",
		"application/x-tcl",
		"application/x-httpd-php",
		"application/x-perl",
		"application/x-python",
		"application/x-ruby",
		"application/x-go",
		"application/x-msdos-program",
		"application/x-csh":
		return true, nil
	}

	// Also check if it starts with "text/"
	if len(mime) >= 5 && mime[:5] == "text/" {
		return true, nil
	}

	return false, nil
}

// FileSize returns the size of the file at the given path in bytes.
func FileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return 0, fmt.Errorf("path is a directory")
	}

	return info.Size(), nil
}
