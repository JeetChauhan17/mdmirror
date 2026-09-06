package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/config"
	"github.com/JeetChauhan17/mdmirror/internal/mirror"
)

type App struct {
	config  config.ResolvedConfig
	manager *ProjectManager
}

func New(cfg config.ResolvedConfig) *App {
	return &App{
		config:  cfg,
		manager: NewProjectManager(),
	}
}

// Sync mirrors all configured projects once.
func (a *App) Sync() error {
	for _, project := range a.config.Projects {
		if err := mirror.Mirror(project.Source, project.Destination); err != nil {
			return fmt.Errorf(
				"sync project %q: %w",
				project.Name,
				err,
			)
		}
	}

	return nil
}

// Watch keeps all configured projects synchronized until the context is canceled.
func (a *App) Watch(ctx context.Context) error {
	if err := a.manager.Start(ctx, a.config.Projects); err != nil {
		return fmt.Errorf("start project manager: %w", err)
	}

	defer a.manager.Stop()

	<-ctx.Done()

	return ctx.Err()
}

// Reconcile updates the running projects to match the supplied configuration.
func (a *App) Reconcile(ctx context.Context, cfg config.ResolvedConfig) error {
	if err := a.manager.Reconcile(ctx, cfg.Projects); err != nil {
		return err
	}

	a.config = cfg
	return nil
}

// ReloadConfig loads, resolves, and applies a configuration file.
func (a *App) ReloadConfig(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	resolved, err := cfg.Resolve(filepath.Dir(configPath))
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	if err := a.Reconcile(ctx, resolved); err != nil {
		return fmt.Errorf("reconcile projects: %w", err)
	}

	return nil
}

// RunReloadLoop starts the configured projects and periodically reloads the
// configuration. Invalid reloads are ignored so the last known-good state
// continues running.
func (a *App) RunReloadLoop(
	ctx context.Context,
	configPath string,
	interval time.Duration,
) error {
	if interval <= 0 {
		return fmt.Errorf("reload interval must be positive")
	}

	if err := a.manager.Start(ctx, a.config.Projects); err != nil {
		return fmt.Errorf("start project manager: %w", err)
	}

	defer a.manager.Stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := a.ReloadConfig(ctx, configPath); err != nil {
				fmt.Printf("Warning: configuration reload failed: %v\n", err)
			}
		}
	}
}
