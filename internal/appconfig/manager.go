package appconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	LanguageEnglish           = "en"
	LanguageChineseSimplified = "zh-CN"
)

// Config is the application-level configuration stored in config.json.
type Config struct {
	Port        int    `json:"port"`
	OpenBrowser bool   `json:"open_browser"`
	Theme       string `json:"theme"`
	Language    string `json:"language"`
}

// Default returns the default application configuration.
func Default() Config {
	return Config{
		Port:        9800,
		OpenBrowser: true,
		Theme:       "light",
		Language:    LanguageEnglish,
	}
}

// Manager provides concurrency-safe access to the application configuration.
type Manager struct {
	mu     sync.RWMutex
	path   string
	config Config
}

// NewManager loads config.json from appDir and creates or migrates it when needed.
func NewManager(appDir string) (*Manager, error) {
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return nil, fmt.Errorf("create app config directory: %w", err)
	}

	manager := &Manager{
		path:   filepath.Join(appDir, "config.json"),
		config: Default(),
	}

	data, err := os.ReadFile(manager.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read app config: %w", err)
		}
		if err := writeConfig(manager.path, manager.config); err != nil {
			return nil, err
		}
		return manager, nil
	}

	needsSave := false
	if err := json.Unmarshal(data, &manager.config); err != nil {
		log.Printf("warning: failed to parse config, using defaults: %v", err)
		manager.config = Default()
		needsSave = true
	} else {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err == nil {
			rawLanguage, hasLanguage := fields["language"]
			var persistedLanguage string
			if !hasLanguage || json.Unmarshal(rawLanguage, &persistedLanguage) != nil || !ValidLanguage(persistedLanguage) {
				manager.config.Language = LanguageEnglish
				needsSave = true
			}
		}
	}

	if needsSave {
		if err := writeConfig(manager.path, manager.config); err != nil {
			return nil, err
		}
	}

	return manager, nil
}

// Get returns a snapshot of the current application configuration.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetLanguage validates and persists the default UI language.
func (m *Manager) SetLanguage(language string) (Config, error) {
	if !ValidLanguage(language) {
		return Config{}, fmt.Errorf("invalid language %q: supported values are %q and %q", language, LanguageEnglish, LanguageChineseSimplified)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.Language == language {
		return m.config, nil
	}

	updated := m.config
	updated.Language = language
	if err := writeConfig(m.path, updated); err != nil {
		return Config{}, err
	}
	m.config = updated
	return m.config, nil
}

// ValidLanguage reports whether language is supported by the UI.
func ValidLanguage(language string) bool {
	return language == LanguageEnglish || language == LanguageChineseSimplified
}

func writeConfig(path string, config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal app config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary app config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("set app config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write app config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync app config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close app config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace app config: %w", err)
	}
	return nil
}
