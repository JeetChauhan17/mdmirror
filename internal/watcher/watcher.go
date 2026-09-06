package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/JeetChauhan17/mdmirror/internal/mirror"
)

const defaultDebounce = 150 * time.Millisecond

type Watcher struct {
	source      string
	destination string
	debounce    time.Duration
	fsWatcher   *fsnotify.Watcher
}

func New(source, destination string) (*Watcher, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("resolve source path: %w", err)
	}

	destination, err = filepath.Abs(destination)
	if err != nil {
		return nil, fmt.Errorf("resolve destination path: %w", err)
	}

	source = filepath.Clean(source)
	destination = filepath.Clean(destination)

	if source == destination {
		return nil, fmt.Errorf("source and destination cannot be the same directory")
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("source is not a directory: %s", source)
	}

	if isPathInside(destination, source) {
		return nil, fmt.Errorf("destination cannot be inside source")
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create filesystem watcher: %w", err)
	}

	w := &Watcher{
		source:      source,
		destination: destination,
		debounce:    defaultDebounce,
		fsWatcher:   fsWatcher,
	}

	if err := addDirectories(fsWatcher, source); err != nil {
		_ = fsWatcher.Close()
		return nil, fmt.Errorf("watch source: %w", err)
	}

	return w, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	if err := mirror.Mirror(w.source, w.destination); err != nil {
		return fmt.Errorf("initial mirror: %w", err)
	}

	var timer *time.Timer
	var timerC <-chan time.Time

	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	scheduleSync := func() {
		if timer != nil {
			timer.Stop()
		}

		timer = time.NewTimer(w.debounce)
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}

			if err := w.handleEvent(event); err != nil {
				return err
			}

			if isRelevantEvent(event) {
				scheduleSync()
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}

			return fmt.Errorf("filesystem watcher error: %w", err)

		case <-timerC:
			timerC = nil
		}
	}
}

func (w *Watcher) Close() error {
	if w.fsWatcher == nil {
		return nil
	}

	return w.fsWatcher.Close()
}

func (w *Watcher) handleEvent(event fsnotify.Event) error {
	// Directory events need special handling because a directory can
	// contain many Markdown files.
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		if event.Op&fsnotify.Create != 0 {
			if err := addDirectories(w.fsWatcher, event.Name); err != nil {
				return fmt.Errorf("watch new directory %s: %w", event.Name, err)
			}

			return w.syncDirectory(event.Name)
		}

		return nil
	}

	// A removed/renamed path no longer exists in the source tree.
	// Remove the corresponding mirrored path.
	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		if isMarkdown(event.Name) {
			if err := w.removeMirrorFile(event.Name); err != nil {
				return err
			}
		} else {
			_ = w.removeMirrorDirectory(event.Name)
		}

		return nil
	}

	// Create/write events for Markdown files only need that one file
	// copied to the mirror.
	if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
		if isMarkdown(event.Name) {
			return w.syncFile(event.Name)
		}
	}

	return nil
}

func (w *Watcher) syncFile(sourcePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return w.removeMirrorFile(sourcePath)
		}

		return fmt.Errorf("stat changed file %s: %w", sourcePath, err)
	}

	if info.IsDir() || !isMarkdown(filepath.Base(sourcePath)) {
		return nil
	}

	target, err := w.destinationPath(sourcePath)
	if err != nil {
		return err
	}

	if err := copyFile(sourcePath, target); err != nil {
		return fmt.Errorf("copy changed file %s: %w", sourcePath, err)
	}

	return nil
}

func (w *Watcher) syncDirectory(sourcePath string) error {
	err := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		if !isMarkdown(entry.Name()) {
			return nil
		}

		return w.syncFile(path)
	})

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("sync directory %s: %w", sourcePath, err)
	}

	return nil
}

func (w *Watcher) removeMirrorFile(sourcePath string) error {
	target, err := w.destinationPath(sourcePath)
	if err != nil {
		return err
	}

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove mirrored file %s: %w", target, err)
	}

	return removeEmptyDirectories(filepath.Dir(target))
}

func (w *Watcher) removeMirrorDirectory(sourcePath string) error {
	target, err := w.destinationPath(sourcePath)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove mirrored directory %s: %w", target, err)
	}

	return nil
}

func (w *Watcher) destinationPath(sourcePath string) (string, error) {
	relative, err := filepath.Rel(w.source, sourcePath)
	if err != nil {
		return "", fmt.Errorf("calculate relative path for %s: %w", sourcePath, err)
	}

	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside source: %s", sourcePath)
	}

	return filepath.Join(w.destination, relative), nil
}

func addDirectories(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path != root && entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !entry.IsDir() {
			return nil
		}

		return w.Add(path)
	})
}

func isRelevantEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Write|
		fsnotify.Create|
		fsnotify.Remove|
		fsnotify.Rename) != 0
}

func isMarkdown(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".md" || extension == ".markdown"
}

func isPathInside(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}

	if relative == "." || relative == ".." {
		return false
	}

	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}

	output, err := os.Create(destination)
	if err != nil {
		return err
	}

	_, copyErr := output.ReadFrom(input)
	closeErr := output.Close()

	if copyErr != nil {
		return copyErr
	}

	if closeErr != nil {
		return closeErr
	}

	return os.Chmod(destination, info.Mode().Perm())
}

func removeEmptyDirectories(root string) error {
	var directories []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == root || !entry.IsDir() {
			return nil
		}

		directories = append(directories, path)
		return nil
	})
	if err != nil {
		return err
	}

	for i := len(directories) - 1; i >= 0; i-- {
		_ = os.Remove(directories[i])
	}

	return nil
}
