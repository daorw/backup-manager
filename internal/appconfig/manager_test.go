package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManagerCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	config := manager.Get()
	if config.Language != LanguageEnglish {
		t.Fatalf("default language = %q, want %q", config.Language, LanguageEnglish)
	}
	if config.Port != 9800 || !config.OpenBrowser || config.Theme != "light" {
		t.Fatalf("unexpected defaults: %+v", config)
	}

	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}

func TestNewManagerMigratesLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := []byte(`{"port":1234,"open_browser":false,"theme":"dark"}`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	manager, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	config := manager.Get()
	if config.Language != LanguageEnglish {
		t.Fatalf("migrated language = %q, want %q", config.Language, LanguageEnglish)
	}
	if config.Port != 1234 || config.OpenBrowser || config.Theme != "dark" {
		t.Fatalf("legacy fields were not preserved: %+v", config)
	}

	persisted := readConfig(t, path)
	if persisted.Language != LanguageEnglish {
		t.Fatalf("persisted language = %q, want %q", persisted.Language, LanguageEnglish)
	}
}

func TestNewManagerNormalizesInvalidLanguage(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "null", value: "null"},
		{name: "empty", value: `""`},
		{name: "unsupported", value: `"fr"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			data := []byte(`{"port":1234,"open_browser":false,"theme":"dark","language":` + test.value + `}`)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			manager, err := NewManager(dir)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			config := manager.Get()
			if config.Language != LanguageEnglish {
				t.Fatalf("normalized language = %q, want %q", config.Language, LanguageEnglish)
			}
			if config.Port != 1234 || config.OpenBrowser || config.Theme != "dark" {
				t.Fatalf("existing fields were not preserved: %+v", config)
			}

			persisted := readConfig(t, path)
			if persisted.Language != LanguageEnglish {
				t.Fatalf("persisted language = %q, want %q", persisted.Language, LanguageEnglish)
			}
		})
	}
}

func TestSetLanguagePersists(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	updated, err := manager.SetLanguage(LanguageChineseSimplified)
	if err != nil {
		t.Fatalf("SetLanguage() error = %v", err)
	}
	if updated.Language != LanguageChineseSimplified {
		t.Fatalf("updated language = %q, want %q", updated.Language, LanguageChineseSimplified)
	}

	reloaded, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	if reloaded.Get().Language != LanguageChineseSimplified {
		t.Fatalf("reloaded language = %q, want %q", reloaded.Get().Language, LanguageChineseSimplified)
	}
}

func TestSetLanguageRejectsUnsupportedValue(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if _, err := manager.SetLanguage("fr"); err == nil {
		t.Fatal("SetLanguage() error = nil, want validation error")
	}
	if manager.Get().Language != LanguageEnglish {
		t.Fatalf("language changed after rejected update: %q", manager.Get().Language)
	}
}

func TestSetLanguageWriteFailureKeepsCurrentConfig(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	originalPath := manager.path
	manager.path = filepath.Join(dir, "missing", "config.json")

	if _, err := manager.SetLanguage(LanguageChineseSimplified); err == nil {
		t.Fatal("SetLanguage() error = nil, want write error")
	}
	if got := manager.Get().Language; got != LanguageEnglish {
		t.Fatalf("language after write failure = %q, want %q", got, LanguageEnglish)
	}
	if persisted := readConfig(t, originalPath); persisted.Language != LanguageEnglish {
		t.Fatalf("persisted language after write failure = %q, want %q", persisted.Language, LanguageEnglish)
	}
}

func readConfig(t *testing.T, path string) Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return config
}
