package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	keySize = 32 // AES-256
)

// KeyManager manages an AES-256-GCM encryption key stored on disk.
type KeyManager struct {
	Key     []byte
	keyPath string
}

// NewKeyManager creates a KeyManager by loading the key from keyPath,
// or generating a new one if the file does not exist.
func NewKeyManager(keyPath string) (*KeyManager, error) {
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Generate a new key
		key := make([]byte, keySize)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("failed to generate key: %w", err)
		}

		// Ensure parent directory exists
		dir := filepath.Dir(keyPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create key directory: %w", err)
		}

		// Write key file with restricted permissions
		if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600); err != nil {
			return nil, fmt.Errorf("failed to write key file: %w", err)
		}

		return &KeyManager{Key: key, keyPath: keyPath}, nil
	}

	// Load existing key
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	key, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	if len(key) != keySize {
		return nil, fmt.Errorf("invalid key length: expected %d, got %d", keySize, len(key))
	}

	return &KeyManager{Key: key, keyPath: keyPath}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns: nonce + ciphertext + tag.
func (km *KeyManager) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(km.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the encrypted data to nonce (nonce || ciphertext || tag)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext previously produced by Encrypt.
func (km *KeyManager) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(km.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// Destroy zeroes the key in memory.
func (km *KeyManager) Destroy() {
	for i := range km.Key {
		km.Key[i] = 0
	}
	km.Key = nil
}
