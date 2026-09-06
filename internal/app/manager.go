package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/JeetChauhan17/mdmirror/internal/config"
	"github.com/JeetChauhan17/mdmirror/internal/watcher"
)

type ProjectManager struct {
	mu       sync.Mutex
	projects map[string]*managedProject
}

type managedProject struct {
	config  config.ResolvedProject
	watcher *watcher.Watcher
	cancel  context.CancelFunc
	done    chan error
}

func NewProjectManager() *ProjectManager {
	return &ProjectManager{
		projects: make(map[string]*managedProject),
	}
}

func (m *ProjectManager) Start(
	ctx context.Context,
	projects []config.ResolvedProject,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, project := range projects {
		if _, exists := m.projects[project.Name]; exists {
			continue
		}

		if err := m.startProjectLocked(ctx, project); err != nil {
			m.stopAllLocked()
			return err
		}
	}

	return nil
}

func (m *ProjectManager) Reconcile(
	ctx context.Context,
	projects []config.ResolvedProject,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]config.ResolvedProject, len(projects))

	for _, project := range projects {
		if _, exists := desired[project.Name]; exists {
			return fmt.Errorf("duplicate project name: %q", project.Name)
		}

		desired[project.Name] = project
	}

	// Stop projects that no longer exist in the configuration.
	for name, project := range m.projects {
		if _, exists := desired[name]; exists {
			continue
		}

		m.stopProjectLocked(project)
		delete(m.projects, name)
	}

	// Start new projects and restart projects whose paths changed.
	for name, project := range desired {
		existing, exists := m.projects[name]

		if !exists {
			if err := m.startProjectLocked(ctx, project); err != nil {
				return err
			}
			continue
		}

		if existing.config.Source == project.Source &&
			existing.config.Destination == project.Destination {
			continue
		}

		m.stopProjectLocked(existing)

		if err := m.startProjectLocked(ctx, project); err != nil {
			return err
		}
	}

	return nil
}

func (m *ProjectManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopAllLocked()
}

func (m *ProjectManager) startProjectLocked(
	ctx context.Context,
	project config.ResolvedProject,
) error {
	projectCtx, cancel := context.WithCancel(ctx)

	w, err := watcher.New(project.Source, project.Destination)
	if err != nil {
		cancel()
		return fmt.Errorf("create watcher for project %q: %w", project.Name, err)
	}

	done := make(chan error, 1)

	managed := &managedProject{
		config:  project,
		watcher: w,
		cancel:  cancel,
		done:    done,
	}

	m.projects[project.Name] = managed

	go func() {
		err := w.Start(projectCtx)
		_ = w.Close()
		done <- err
	}()

	return nil
}

func (m *ProjectManager) stopProjectLocked(project *managedProject) {
	project.cancel()
	_ = project.watcher.Close()

	select {
	case <-project.done:
	case <-context.Background().Done():
	}
}

func (m *ProjectManager) stopAllLocked() {
	for name, project := range m.projects {
		m.stopProjectLocked(project)
		delete(m.projects, name)
	}
}
