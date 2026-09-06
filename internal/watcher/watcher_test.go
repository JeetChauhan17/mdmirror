package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherSyncsChanges(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "mirror")

	writeTestFile(t, filepath.Join(source, "README.md"), "# Initial")
	writeTestFile(t, filepath.Join(source, "docs", "guide.md"), "# Guide")
	writeTestFile(t, filepath.Join(source, "src", "main.go"), "package main")

	w, err := New(source, destination)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- w.Start(ctx)
	}()

	waitForFile(t, filepath.Join(destination, "README.md"))
	waitForFile(t, filepath.Join(destination, "docs", "guide.md"))

	assertFileExists(t, filepath.Join(destination, "README.md"))
	assertFileExists(t, filepath.Join(destination, "docs", "guide.md"))
	assertFileNotExists(t, filepath.Join(destination, "src", "main.go"))

	// Create a Markdown file.
	newFile := filepath.Join(source, "new.md")
	writeTestFile(t, newFile, "# New")

	waitForFile(t, filepath.Join(destination, "new.md"))

	// Modify it.
	writeTestFile(t, newFile, "# Modified")

	waitForFileContent(t, filepath.Join(destination, "new.md"), "# Modified")

	// Create a non-Markdown file. It must not appear in the mirror.
	txtFile := filepath.Join(source, "ignored.txt")
	writeTestFile(t, txtFile, "ignore me")

	time.Sleep(300 * time.Millisecond)

	assertFileNotExists(t, filepath.Join(destination, "ignored.txt"))

	// Delete the Markdown file.
	if err := os.Remove(newFile); err != nil {
		t.Fatalf("remove source Markdown file: %v", err)
	}

	waitForFileNotExists(t, filepath.Join(destination, "new.md"))

	// Create a new nested directory with Markdown.
	nestedFile := filepath.Join(source, "research", "sub", "test.md")
	writeTestFile(t, nestedFile, "# Research")

	waitForFile(t, filepath.Join(destination, "research", "sub", "test.md"))

	// Rename the Markdown file.
	renamedFile := filepath.Join(source, "research", "sub", "renamed.md")

	if err := os.Rename(nestedFile, renamedFile); err != nil {
		t.Fatalf("rename source Markdown file: %v", err)
	}

	waitForFile(t, filepath.Join(destination, "research", "sub", "renamed.md"))
	waitForFileNotExists(t, filepath.Join(destination, "research", "sub", "test.md"))

	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errorsIsContextCanceled(err) {
			t.Fatalf("watcher returned error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after context cancellation")
	}
}

func TestWatcherRejectsInvalidPaths(t *testing.T) {
	source := t.TempDir()

	tests := []struct {
		name        string
		destination string
	}{
		{
			name:        "same directory",
			destination: source,
		},
		{
			name:        "destination inside source",
			destination: filepath.Join(source, "mirror"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(source, tt.destination); err == nil {
				t.Fatal("New() expected an error, got nil")
			}
		})
	}
}

func TestWatcherAllowsSiblingDestination(t *testing.T) {
	root := t.TempDir()

	source := filepath.Join(root, "project")
	destination := filepath.Join(root, "mirror")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}

	w, err := New(source, destination)
	if err != nil {
		t.Fatalf("New() rejected sibling destination: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for file %s", path)
}

func waitForFileNotExists(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for file to disappear: %s", path)
}

func waitForFileContent(t *testing.T, path, expected string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil && string(content) == expected {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	content, _ := os.ReadFile(path)
	t.Fatalf(
		"timed out waiting for %s to contain expected content %q; got %q",
		path,
		expected,
		string(content),
	)
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist %s: %v", path, err)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file not to exist %s", path)
	}
}

func errorsIsContextCanceled(err error) bool {
	return err != nil && err.Error() == context.Canceled.Error()
}

func TestWatcherDebouncesChanges(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "mirror")

	writeTestFile(t, filepath.Join(source, "README.md"), "# Initial")

	w, err := New(source, destination)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer w.Close()

	// Make the debounce window intentionally large enough that we can
	// generate several writes inside a single window.
	// w.debounce = 200 * time.Millisecond
	w.debounce = 300 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- w.Start(ctx)
	}()

	waitForFileContent(t, filepath.Join(destination, "README.md"), "# Initial")

	// Perform several writes rapidly. The watcher should eventually
	// produce the final state rather than treating every write as a
	// separate synchronization.
	// for i := 0; i < 5; i++ {
	// 	writeTestFile(
	// 		t,
	// 		filepath.Join(source, "README.md"),
	// 		"# Update",
	// 	)
	// 	time.Sleep(25 * time.Millisecond)
	// }
	//
	// waitForFileContent(
	// 	t,
	// 	filepath.Join(destination, "README.md"),
	// 	"# Update",
	// )
	writeTestFile(t, filepath.Join(source, "README.md"), "# Update")

	time.Sleep(50 * time.Millisecond)

	content, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil {
		t.Fatalf("read mirrored file before debounce: %v", err)
	}

	if string(content) != "# Initial" {
		t.Fatalf("file synced before debounce expired: got %q", string(content))
	}

	waitForFileContent(t, filepath.Join(destination, "README.md"), "# Update")

	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errorsIsContextCanceled(err) {
			t.Fatalf("watcher returned error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after context cancellation")
	}
}
