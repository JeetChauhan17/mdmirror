package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestAdd(t *testing.T) {
	root := t.TempDir()

	configDir := filepath.Join(root, "config")
	configPath := filepath.Join(configDir, "config.toml")

	sourceA := filepath.Join(root, "project-a")
	sourceB := filepath.Join(root, "project-b")

	if err := os.MkdirAll(sourceA, 0o755); err != nil {
		t.Fatalf("create source A: %v", err)
	}

	if err := os.MkdirAll(sourceB, 0o755); err != nil {
		t.Fatalf("create source B: %v", err)
	}

	initial := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{
				Name:   "project-a",
				Source: sourceA,
			},
		},
	}

	if err := writeConfig(configPath, initial); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	if err := Add(configPath, "project-b", sourceB); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("Projects length = %d, want 2", len(cfg.Projects))
	}

	project := cfg.Projects[1]

	if project.Name != "project-b" {
		t.Fatalf("Name = %q, want project-b", project.Name)
	}

	if project.Source != sourceB {
		t.Fatalf("Source = %q, want %q", project.Source, sourceB)
	}
}

func TestAddRejectsDuplicateName(t *testing.T) {
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
		},
	}

	if err := writeConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	err := Add(configPath, "project-a", sourceB)

	if err == nil {
		t.Fatal("Add() expected duplicate-name error")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRejectsDuplicateSource(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")

	source := filepath.Join(root, "project")

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

	err := Add(configPath, "project-b", source)

	if err == nil {
		t.Fatal("Add() expected duplicate-source error")
	}

	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRejectsMissingSource(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{Name: "project-a", Source: filepath.Join(root, "project-a")},
		},
	}

	if err := writeConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	err := Add(
		configPath,
		"project-b",
		filepath.Join(root, "does-not-exist"),
	)

	if err == nil {
		t.Fatal("Add() expected missing-source error")
	}
}

func TestAddRejectsInvalidProjectName(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	source := filepath.Join(root, "project")

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

	err := Add(configPath, "../bad", source)

	if err == nil {
		t.Fatal("Add() expected invalid-name error")
	}
}

func TestAddPreservesConfigOnFailure(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")

	source := filepath.Join(root, "project")
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

	err = Add(configPath, "project-a", source)
	if err == nil {
		t.Fatal("Add() expected error")
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Fatal("configuration changed after failed Add()")
	}
}

func TestWriteConfigProducesValidTOML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")

	cfg := Config{
		Vault: "~/mdmirror-vault",
		Projects: []Project{
			{
				Name:   "project-a",
				Source: "~/Projects/project-a",
			},
		},
	}

	if err := writeConfig(path, cfg); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Config
	if _, err := toml.Decode(string(data), &decoded); err != nil {
		t.Fatalf("generated TOML is invalid: %v", err)
	}
}
