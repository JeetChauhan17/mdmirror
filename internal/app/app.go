package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/JeetChauhan17/mdmirror/internal/config"
	"github.com/JeetChauhan17/mdmirror/internal/mirror"
	"github.com/JeetChauhan17/mdmirror/internal/watcher"
)

type App struct {
	config config.ResolvedConfig
}

func New(cfg config.ResolvedConfig) *App {
	return &App{
		config: cfg,
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
	var wg sync.WaitGroup
	errCh := make(chan error, len(a.config.Projects))

	for _, project := range a.config.Projects {
		project := project

		wg.Add(1)

		go func() {
			defer wg.Done()

			w, err := watcher.New(project.Source, project.Destination)
			if err != nil {
				errCh <- fmt.Errorf(
					"watch project %q: %w",
					project.Name,
					err,
				)
				return
			}
			defer w.Close()

			if err := w.Start(ctx); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf(
					"watch project %q: %w",
					project.Name,
					err,
				)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()

	case err := <-errCh:
		return err

	case <-done:
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	}
}
