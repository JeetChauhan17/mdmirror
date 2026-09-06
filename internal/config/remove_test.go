package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemove(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")

	sourceA := filepath.Join(root, "project-a")
	sourceB := filepath.Join(root, "project-b")

	if err := os.MkdirAll(sourceA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceB, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{Name: "project-a", Source: sourceA},
			{Name: "project-b", Source: sourceB},
		},
	}

	if err := writeConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	removed, err := Remove(configPath, "project-a")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if removed.Name != "project-a" {
		t.Fatalf("removed.Name = %q, want project-a", removed.Name)
	}

	result, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("Projects length = %d, want 1", len(result.Projects))
	}

	if result.Projects[0].Name != "project-b" {
		t.Fatalf("remaining project = %q, want project-b", result.Projects[0].Name)
	}
}

func TestRemoveRejectsMissingProject(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	source := filepath.Join(root, "project-a")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{Name: "project-a", Source: source},
		},
	}

	if err := writeConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	err = func() error {
		_, err := Remove(configPath, "does-not-exist")
		return err
	}()

	if err == nil {
		t.Fatal("Remove() expected error")
	}

	if !strings.Contains(err.Error(), "project not found") {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Fatal("configuration changed after failed Remove()")
	}
}
