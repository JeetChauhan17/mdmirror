package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Add adds a project to the configuration file.
//
// The source must exist and be a directory. The configuration is only
// modified after the new project passes validation against the existing
// configuration.
func Add(path, name, source string) error {
	name = strings.TrimSpace(name)
	source = strings.TrimSpace(source)

	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if !isValidProjectName(name) {
		return fmt.Errorf("project %q: invalid project name", name)
	}

	if source == "" {
		return fmt.Errorf("project %q: source path cannot be empty", name)
	}

	cfg, err := Load(path)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	baseDir := filepath.Dir(path)

	resolvedSource, err := resolvePath(source, baseDir)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}

	info, err := os.Stat(resolvedSource)
	if err != nil {
		return fmt.Errorf("source %q: %w", resolvedSource, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", resolvedSource)
	}

	resolvedVault, err := resolvePath(cfg.Vault, baseDir)
	if err != nil {
		return fmt.Errorf("resolve vault: %w", err)
	}

	for _, project := range cfg.Projects {
		if project.Name == name {
			return fmt.Errorf("project already exists: %q", name)
		}

		existingSource, err := resolvePath(project.Source, baseDir)
		if err != nil {
			return fmt.Errorf(
				"resolve source for existing project %q: %w",
				project.Name,
				err,
			)
		}

		if existingSource == resolvedSource {
			return fmt.Errorf(
				"source is already registered by project %q",
				project.Name,
			)
		}
	}

	destination := filepath.Join(resolvedVault, name)

	if isPathInside(destination, resolvedSource) {
		return fmt.Errorf("project %q: destination cannot be inside source", name)
	}

	cfg.Projects = append(cfg.Projects, Project{
		Name:   name,
		Source: source,
	})

	if err := writeConfig(path, cfg); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}

	return nil
}

func writeConfig(path string, cfg Config) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, ".config.toml.tmp-*")
	if err != nil {
		return err
	}

	tempPath := temp.Name()
	defer os.Remove(tempPath)

	encoder := toml.NewEncoder(temp)

	if err := encoder.Encode(cfg); err != nil {
		_ = temp.Close()
		return err
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}
