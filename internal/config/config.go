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
		if strings.TrimSpace(project.Name) == "" {
			return fmt.Errorf("project %d: name cannot be empty", i+1)
		}

		if strings.TrimSpace(project.Source) == "" {
			return fmt.Errorf("project %q: source path cannot be empty", project.Name)
		}

		if _, exists := seenNames[project.Name]; exists {
			return fmt.Errorf("duplicate project name: %q", project.Name)
		}

		seenNames[project.Name] = struct{}{}
	}

	return nil
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(configDir, configDirName, "config.toml"), nil
}
