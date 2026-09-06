package app

import (
	"context"
	"fmt"

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
