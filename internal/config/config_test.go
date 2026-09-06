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
			wantErr: false,
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

func TestResolve(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "projects", "alpha")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{
				Name:   "alpha",
				Source: filepath.Join(root, "projects", "alpha"),
			},
		},
	}

	resolved, err := cfg.Resolve(root)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expectedVault := filepath.Join(root, "vault")
	expectedSource := filepath.Join(root, "projects", "alpha")
	expectedDestination := filepath.Join(root, "vault", "alpha")

	if resolved.Vault != expectedVault {
		t.Fatalf("Vault = %q, want %q", resolved.Vault, expectedVault)
	}

	if len(resolved.Projects) != 1 {
		t.Fatalf("Projects length = %d, want 1", len(resolved.Projects))
	}

	project := resolved.Projects[0]

	if project.Name != "alpha" {
		t.Fatalf("Name = %q, want alpha", project.Name)
	}

	if project.Source != expectedSource {
		t.Fatalf("Source = %q, want %q", project.Source, expectedSource)
	}

	if project.Destination != expectedDestination {
		t.Fatalf("Destination = %q, want %q", project.Destination, expectedDestination)
	}

	if _, err := os.Stat(expectedVault); err != nil {
		t.Fatalf("expected vault to be created: %v", err)
	}
}

func TestResolveRelativePaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "projects", "alpha")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}

	cfg := Config{
		Vault: "vault",
		Projects: []Project{
			{
				Name:   "alpha",
				Source: "projects/alpha",
			},
		},
	}

	resolved, err := cfg.Resolve(root)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved.Vault != filepath.Join(root, "vault") {
		t.Fatalf("Vault = %q", resolved.Vault)
	}

	if resolved.Projects[0].Source != source {
		t.Fatalf("Source = %q, want %q", resolved.Projects[0].Source, source)
	}
}

func TestResolveRejectsDuplicateSources(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "project")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{Name: "alpha", Source: source},
			{Name: "beta", Source: source},
		},
	}

	if _, err := cfg.Resolve(root); err == nil {
		t.Fatal("Resolve() expected duplicate source error, got nil")
	}
}

func TestResolveRejectsInvalidProjectNames(t *testing.T) {
	tests := []string{
		".",
		"..",
		"foo/bar",
		`foo\bar`,
		" leading-space",
		"trailing-space ",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := Config{
				Vault: "/tmp/vault",
				Projects: []Project{
					{
						Name:   name,
						Source: "/tmp/project",
					},
				},
			}

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() accepted invalid project name %q", name)
			}
		})
	}
}

func TestResolveExpandsHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	source := filepath.Join(home, "mdmirror-test-source")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	defer os.RemoveAll(source)

	cfg := Config{
		Vault: "~/mdmirror-test-vault",
		Projects: []Project{
			{
				Name:   "test",
				Source: "~/mdmirror-test-source",
			},
		},
	}

	resolved, err := cfg.Resolve(home)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved.Vault != filepath.Join(home, "mdmirror-test-vault") {
		t.Fatalf("Vault = %q", resolved.Vault)
	}

	if resolved.Projects[0].Source != source {
		t.Fatalf(
			"Source = %q, want %q",
			resolved.Projects[0].Source,
			source,
		)
	}

	_ = os.RemoveAll(resolved.Vault)
}

func TestResolveRejectsMissingSource(t *testing.T) {
	root := t.TempDir()

	cfg := Config{
		Vault: filepath.Join(root, "vault"),
		Projects: []Project{
			{
				Name:   "missing",
				Source: filepath.Join(root, "does-not-exist"),
			},
		},
	}

	if _, err := cfg.Resolve(root); err == nil {
		t.Fatal("Resolve() expected missing source error, got nil")
	}
}
