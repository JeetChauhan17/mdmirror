package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalUserConfigDir == "" {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
			return
		}

		_ = os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
	})

	configRoot := t.TempDir()

	if err := os.Setenv("XDG_CONFIG_HOME", configRoot); err != nil {
		t.Fatal(err)
	}

	path, err := Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	wantPath := filepath.Join(configRoot, "mdmirror", "config.toml")

	if path != wantPath {
		t.Fatalf("Init() path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !strings.Contains(string(data), `vault = "~/mdmirror-vault"`) {
		t.Fatal("starter configuration does not contain the default vault")
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("configuration directory was not created: %v", err)
	}
}

func TestInitDoesNotOverwriteExistingConfig(t *testing.T) {
	originalUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalUserConfigDir == "" {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
			return
		}

		_ = os.Setenv("XDG_CONFIG_HOME", originalUserConfigDir)
	})

	configRoot := t.TempDir()

	if err := os.Setenv("XDG_CONFIG_HOME", configRoot); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(configRoot, "mdmirror", "config.toml")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	original := []byte("vault = \"original\"\n")

	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(); err == nil {
		t.Fatal("Init() expected an error for existing configuration")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != string(original) {
		t.Fatal("Init() overwrote the existing configuration")
	}
}
