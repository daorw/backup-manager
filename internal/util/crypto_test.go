package util

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyManager_GenerateAndLoadKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")

	t.Run("generate new key and load it", func(t *testing.T) {
		km, err := NewKeyManager(keyPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if km == nil {
			t.Fatal("expected non-nil KeyManager")
		}
		if len(km.Key) != 32 {
			t.Fatalf("expected key length 32, got %d", len(km.Key))
		}

		// Verify key file was created
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			t.Fatal("key file was not created")
		}

		// Check permissions
		info, _ := os.Stat(keyPath)
		if info.Mode() != 0600 {
			t.Fatalf("expected 0600 permissions, got %o", info.Mode())
		}

		// Load key again
		km2, err := NewKeyManager(keyPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(km.Key, km2.Key) {
			t.Fatal("loaded key differs from generated key")
		}
	})

	t.Run("non-existent directory returns error", func(t *testing.T) {
		_, err := NewKeyManager("/nonexistent/path/master.key")
		if err == nil {
			t.Fatal("expected error for non-existent directory")
		}
	})
}

func TestKeyManager_EncryptDecrypt(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")
	km, err := NewKeyManager(keyPath)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	t.Run("encrypt and decrypt plaintext", func(t *testing.T) {
		plaintext := []byte("hello, world! this is a secret message.")
		ciphertext, err := km.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ciphertext) == 0 {
			t.Fatal("ciphertext should not be empty")
		}
		if bytes.Equal(ciphertext, plaintext) {
			t.Fatal("ciphertext should not equal plaintext")
		}

		decrypted, err := km.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(decrypted, plaintext) {
			t.Fatalf("expected %s, got %s", plaintext, decrypted)
		}
	})

	t.Run("encrypt empty data", func(t *testing.T) {
		ciphertext, err := km.Encrypt([]byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		decrypted, err := km.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(decrypted) != 0 {
			t.Fatal("expected empty decrypted data")
		}
	})

	t.Run("decrypt invalid ciphertext", func(t *testing.T) {
		_, err := km.Decrypt([]byte("invalid ciphertext"))
		if err == nil {
			t.Fatal("expected error for invalid ciphertext")
		}
	})

	t.Run("decrypt with wrong key", func(t *testing.T) {
		// Create a second key manager with a different key
		tmpDir2 := t.TempDir()
		keyPath2 := filepath.Join(tmpDir2, "master.key")
		km2, err := NewKeyManager(keyPath2)
		if err != nil {
			t.Fatalf("failed to create KeyManager: %v", err)
		}

		ciphertext, _ := km.Encrypt([]byte("test data"))
		_, err = km2.Decrypt(ciphertext)
		if err == nil {
			t.Fatal("expected error when decrypting with wrong key")
		}
	})

	t.Run("encrypt large data", func(t *testing.T) {
		largeData := make([]byte, 1024*1024) // 1MB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}
		ciphertext, err := km.Encrypt(largeData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		decrypted, err := km.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(decrypted, largeData) {
			t.Fatal("decrypted data does not match original")
		}
	})
}

func TestKeyManager_Destroy(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")
	km, err := NewKeyManager(keyPath)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}

	km.Destroy()
	if km.Key != nil {
		t.Fatal("key should be nil after Destroy")
	}
}
