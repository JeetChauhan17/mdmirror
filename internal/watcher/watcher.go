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
	var err error

	source, err = filepath.Abs(source)
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

// Start performs an initial synchronization and then watches the source
// tree for changes. Multiple filesystem events within the debounce window
// are collapsed into a single full mirror operation.
func (w *Watcher) Start(ctx context.Context) error {
	if err := mirror.Mirror(w.source, w.destination); err != nil {
		return fmt.Errorf("initial mirror: %w", err)
	}

	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer == nil {
			return
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		timerC = nil
	}

	defer stopTimer()

	scheduleSync := func() {
		if timer == nil {
			timer = time.NewTimer(w.debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			timer.Reset(w.debounce)
		}

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

			if err := mirror.Mirror(w.source, w.destination); err != nil {
				return fmt.Errorf("sync after filesystem change: %w", err)
			}
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
	// When a new directory appears, fsnotify does not automatically watch
	// its children. Add the directory tree to the watcher immediately.
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := addDirectories(w.fsWatcher, event.Name); err != nil {
				return fmt.Errorf("watch new directory %s: %w", event.Name, err)
			}
		}
	}

	// Remove/Rename events are intentionally not synchronized here.
	// The subsequent debounced full mirror reconciles additions, changes,
	// renames, and deletions against the current source tree.
	return nil
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
