package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const starterConfig = `vault = "~/mdmirror-vault"

# Add your projects below.
# Example:
#
# [[projects]]
# name = "project-a"
# source = "~/Projects/project-a"
`

// Init creates the default configuration file and its parent directory.
//
// It never overwrites an existing configuration file.
func Init() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("configuration already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check configuration: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create configuration directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
		return "", fmt.Errorf("write configuration: %w", err)
	}

	return path, nil
}
