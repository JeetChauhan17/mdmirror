package main

import (
	"fmt"
	"path/filepath"

	"github.com/JeetChauhan17/mdmirror/internal/config"
)

func loadConfig() (config.ResolvedConfig, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.ResolvedConfig{}, err
	}

	return loadConfigFromPath(path)
}

func loadConfigFromPath(path string) (config.ResolvedConfig, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.ResolvedConfig{}, fmt.Errorf(
			"load config %s: %w",
			path,
			err,
		)
	}

	resolved, err := cfg.Resolve(filepath.Dir(path))
	if err != nil {
		return config.ResolvedConfig{}, fmt.Errorf(
			"resolve config: %w",
			err,
		)
	}

	return resolved, nil
}
