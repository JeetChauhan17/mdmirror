package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/mirror"
	"github.com/fsnotify/fsnotify"
)

const defaultDebounce = 150 * time.Millisecond

// Watch monitors source for filesystem changes and keeps destination
// synchronized with it.
//
// A full mirror is performed initially. Subsequent filesystem events are
// debounced and trigger another synchronization.
func Watch(source, destination string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	destination, err = filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	source = filepath.Clean(source)
	destination = filepath.Clean(destination)

	if err := mirror.Mirror(source, destination); err != nil {
		return fmt.Errorf("initial mirror: %w", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create filesystem watcher: %w", err)
	}
	defer w.Close()

	if err := addDirectories(w, source); err != nil {
		return fmt.Errorf("watch source: %w", err)
	}

	fmt.Printf("Watching %s\n", source)
	fmt.Printf("Mirroring to %s\n", destination)
	fmt.Println("Press Ctrl+C to stop.")

	var timer *time.Timer
	var timerC <-chan time.Time

	scheduleSync := func() {
		if timer != nil {
			timer.Stop()
		}

		timer = time.NewTimer(defaultDebounce)
		timerC = timer.C
	}

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}

			if err := handleEvent(w, source, event); err != nil {
				return err
			}

			if isRelevantEvent(event) {
				scheduleSync()
			}

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}

			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-timerC:
			timerC = nil

			if err := mirror.Mirror(source, destination); err != nil {
				fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
			} else {
				fmt.Println("✓ Mirror updated")
			}
		}
	}
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

func handleEvent(w *fsnotify.Watcher, source string, event fsnotify.Event) error {
	if event.Op&fsnotify.Create == 0 {
		return nil
	}

	info, err := os.Stat(event.Name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return nil
	}

	return addDirectories(w, event.Name)
}

func isRelevantEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Write|
		fsnotify.Create|
		fsnotify.Remove|
		fsnotify.Rename) != 0
}
