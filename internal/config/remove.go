package config

import (
	"fmt"
)

// Remove removes a project from the configuration file.
//
// The project must exist. The configuration is only modified after the
// project has been found.
func Remove(path, name string) (Project, error) {
	cfg, err := Load(path)
	if err != nil {
		return Project{}, fmt.Errorf("load configuration: %w", err)
	}

	for i, project := range cfg.Projects {
		if project.Name != name {
			continue
		}

		removed := project
		cfg.Projects = append(cfg.Projects[:i], cfg.Projects[i+1:]...)

		if err := writeConfig(path, cfg); err != nil {
			return Project{}, fmt.Errorf("write configuration: %w", err)
		}

		return removed, nil
	}

	return Project{}, fmt.Errorf("project not found: %q", name)
}
