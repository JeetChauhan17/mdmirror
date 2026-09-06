package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

		if err := removeProjectMirror(project.config.Destination); err != nil {
			return fmt.Errorf(
				"remove mirror for project %q: %w",
				name,
				err,
			)
		}

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

		if err := removeProjectMirror(existing.config.Destination); err != nil {
			return fmt.Errorf(
				"remove old mirror for project %q: %w",
				name,
				err,
			)
		}

		delete(m.projects, name)

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

	<-project.done
}

func (m *ProjectManager) stopAllLocked() {
	for name, project := range m.projects {
		m.stopProjectLocked(project)
		delete(m.projects, name)
	}
}

// removeProjectMirror removes only Markdown files and empty directories from
// a project's destination. Non-Markdown files are never deleted.
func removeProjectMirror(destination string) error {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}

	info, err := os.Stat(destination)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", destination)
	}

	if err := removeMarkdownFiles(destination); err != nil {
		return err
	}

	entries, err := os.ReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func removeMarkdownFiles(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())

		if entry.IsDir() {
			if err := removeMarkdownFiles(path); err != nil {
				return err
			}

			remaining, err := os.ReadDir(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}

			if len(remaining) == 0 {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
			}

			continue
		}

		if isMarkdownFile(entry.Name()) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

func isMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}
