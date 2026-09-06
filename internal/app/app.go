package app

import (
	"context"
	"fmt"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/config"
	"github.com/JeetChauhan17/mdmirror/internal/mirror"
	"path/filepath"
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

func (a *App) ReloadConfig(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	baseDir := filepath.Dir(configPath)

	resolved, err := cfg.Resolve(baseDir)
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	if err := a.Reconcile(ctx, resolved); err != nil {
		return fmt.Errorf("reconcile projects: %w", err)
	}

	return nil
}

// RunReloadLoop periodically reloads and reconciles the supplied configuration.
// func (a *App) RunReloadLoop(
// 	ctx context.Context,
// 	configPath string,
// 	interval time.Duration,
// ) error {

func (a *App) RunReloadLoop(
	ctx context.Context,
	configPath string,
	baseDir string,
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
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("reload configuration: %w", err)
			}

			// resolved, err := cfg.Resolve("")
			resolved, err := cfg.Resolve(baseDir)
			if err != nil {
				return fmt.Errorf("resolve reloaded configuration: %w", err)
			}

			if err := a.manager.Reconcile(ctx, resolved.Projects); err != nil {
				return fmt.Errorf("reconcile projects: %w", err)
			}

			a.config = resolved
		}
	}
}
