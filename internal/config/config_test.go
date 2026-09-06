package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
vault = "/tmp/mdmirror-vault"

[[projects]]
name = "project-a"
source = "/tmp/project-a"

[[projects]]
name = "project-b"
source = "/tmp/project-b"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Vault != "/tmp/mdmirror-vault" {
		t.Fatalf("Vault = %q, want %q", cfg.Vault, "/tmp/mdmirror-vault")
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("Projects length = %d, want 2", len(cfg.Projects))
	}

	if cfg.Projects[0].Name != "project-a" {
		t.Fatalf("first project name = %q", cfg.Projects[0].Name)
	}

	if cfg.Projects[1].Source != "/tmp/project-b" {
		t.Fatalf("second project source = %q", cfg.Projects[1].Source)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid",
			config: Config{
				Vault: "/tmp/vault",
				Projects: []Project{
					{Name: "project-a", Source: "/tmp/project-a"},
				},
			},
		},
		{
			name: "empty vault",
			config: Config{
				Projects: []Project{
					{Name: "project-a", Source: "/tmp/project-a"},
				},
			},
			wantErr: true,
		},
		{
			name: "no projects",
			config: Config{
				Vault: "/tmp/vault",
			},
			wantErr: true,
		},
		{
			name: "empty project name",
			config: Config{
				Vault: "/tmp/vault",
				Projects: []Project{
					{Source: "/tmp/project-a"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty source",
			config: Config{
				Vault: "/tmp/vault",
				Projects: []Project{
					{Name: "project-a"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate names",
			config: Config{
				Vault: "/tmp/vault",
				Projects: []Project{
					{Name: "project-a", Source: "/tmp/project-a"},
					{Name: "project-a", Source: "/tmp/project-b"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	if filepath.Base(path) != "config.toml" {
		t.Fatalf("DefaultPath() = %q, expected config.toml", path)
	}

	if filepath.Base(filepath.Dir(path)) != "mdmirror" {
		t.Fatalf("DefaultPath() = %q, expected mdmirror config directory", path)
	}
}
