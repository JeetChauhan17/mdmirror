package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const configDirName = "mdmirror"

type Config struct {
	Vault    string    `toml:"vault"`
	Projects []Project `toml:"projects"`
}

type Project struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
}

type ResolvedConfig struct {
	Vault    string
	Projects []ResolvedProject
}

type ResolvedProject struct {
	Name        string
	Source      string
	Destination string
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Vault) == "" {
		return errors.New("vault path cannot be empty")
	}

	if len(c.Projects) == 0 {
		return errors.New("config must contain at least one project")
	}

	seenNames := make(map[string]struct{})

	for i, project := range c.Projects {
		// name := strings.TrimSpace(project.Name)
		// source := strings.TrimSpace(project.Source)

		name := project.Name
		source := strings.TrimSpace(project.Source)

		if name == "" {
			return fmt.Errorf("project %d: name cannot be empty", i+1)
		}

		if source == "" {
			return fmt.Errorf("project %q: source path cannot be empty", name)
		}

		if !isValidProjectName(name) {
			return fmt.Errorf("project %q: invalid project name", name)
		}

		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("duplicate project name: %q", name)
		}

		seenNames[name] = struct{}{}
	}

	return nil
}

func (c Config) Resolve(baseDir string) (ResolvedConfig, error) {
	if err := c.Validate(); err != nil {
		return ResolvedConfig{}, err
	}

	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return ResolvedConfig{}, fmt.Errorf("resolve base directory: %w", err)
	}

	vault, err := resolvePath(c.Vault, baseDir)
	if err != nil {
		return ResolvedConfig{}, fmt.Errorf("resolve vault: %w", err)
	}

	if err := os.MkdirAll(vault, 0o755); err != nil {
		return ResolvedConfig{}, fmt.Errorf("create vault: %w", err)
	}

	resolved := ResolvedConfig{
		Vault: vault,
	}

	seenSources := make(map[string]string)
	seenDestinations := make(map[string]string)

	for _, project := range c.Projects {
		source, err := resolvePath(project.Source, baseDir)
		if err != nil {
			return ResolvedConfig{}, fmt.Errorf(
				"resolve source for project %q: %w",
				project.Name,
				err,
			)
		}

		info, err := os.Stat(source)
		if err != nil {
			return ResolvedConfig{}, fmt.Errorf(
				"project %q: source %q: %w",
				project.Name,
				source,
				err,
			)
		}

		if !info.IsDir() {
			return ResolvedConfig{}, fmt.Errorf(
				"project %q: source is not a directory: %s",
				project.Name,
				source,
			)
		}

		destination := filepath.Join(vault, project.Name)

		if other, exists := seenSources[source]; exists {
			return ResolvedConfig{}, fmt.Errorf(
				"projects %q and %q use the same source: %s",
				other,
				project.Name,
				source,
			)
		}

		if other, exists := seenDestinations[destination]; exists {
			return ResolvedConfig{}, fmt.Errorf(
				"projects %q and %q use the same destination: %s",
				other,
				project.Name,
				destination,
			)
		}

		if isPathInside(destination, source) {
			return ResolvedConfig{}, fmt.Errorf(
				"project %q: destination cannot be inside source",
				project.Name,
			)
		}

		seenSources[source] = project.Name
		seenDestinations[destination] = project.Name

		resolved.Projects = append(resolved.Projects, ResolvedProject{
			Name:        project.Name,
			Source:      source,
			Destination: destination,
		})
	}

	return resolved, nil
}

func resolvePath(path, baseDir string) (string, error) {
	path = strings.TrimSpace(path)

	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}

		return filepath.Abs(home)
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}

		path = filepath.Join(home, path[2:])
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	return filepath.Abs(filepath.Clean(path))
}

func isValidProjectName(name string) bool {
	if name == "." || name == ".." {
		return false
	}

	if strings.ContainsAny(name, `/\`) {
		return false
	}

	return strings.TrimSpace(name) == name
}

func isPathInside(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}

	if relative == "." || relative == ".." {
		return false
	}

	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(configDir, configDirName, "config.toml"), nil
}
