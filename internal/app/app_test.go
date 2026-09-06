package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/config"
)

func TestSync(t *testing.T) {
	root := t.TempDir()

	sourceA := filepath.Join(root, "source-a")
	sourceB := filepath.Join(root, "source-b")
	vault := filepath.Join(root, "vault")

	for _, source := range []string{sourceA, sourceB} {
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(sourceA, "README.md"), "# Project A")
	writeFile(t, filepath.Join(sourceA, "notes.txt"), "ignore me")
	writeFile(t, filepath.Join(sourceB, "README.md"), "# Project B")

	cfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "project-a",
				Source:      sourceA,
				Destination: filepath.Join(vault, "project-a"),
			},
			{
				Name:        "project-b",
				Source:      sourceB,
				Destination: filepath.Join(vault, "project-b"),
			},
		},
	}

	application := New(cfg)

	if err := application.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertFileContents(t,
		filepath.Join(vault, "project-a", "README.md"),
		"# Project A",
	)

	assertFileContents(t,
		filepath.Join(vault, "project-b", "README.md"),
		"# Project B",
	)

	if _, err := os.Stat(filepath.Join(vault, "project-a", "notes.txt")); !os.IsNotExist(err) {
		t.Fatal("non-Markdown file was mirrored")
	}
}

func TestWatch(t *testing.T) {
	root := t.TempDir()

	sourceA := filepath.Join(root, "source-a")
	sourceB := filepath.Join(root, "source-b")
	vault := filepath.Join(root, "vault")

	for _, source := range []string{sourceA, sourceB} {
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "project-a",
				Source:      sourceA,
				Destination: filepath.Join(vault, "project-a"),
			},
			{
				Name:        "project-b",
				Source:      sourceB,
				Destination: filepath.Join(vault, "project-b"),
			},
		},
	}

	application := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- application.Watch(ctx)
	}()

	waitForFile(t, filepath.Join(vault, "project-a"))
	waitForFile(t, filepath.Join(vault, "project-b"))

	writeFile(t, filepath.Join(sourceA, "new.md"), "# New A")
	writeFile(t, filepath.Join(sourceB, "new.md"), "# New B")

	waitForFile(t, filepath.Join(vault, "project-a", "new.md"))
	waitForFile(t, filepath.Join(vault, "project-b", "new.md"))

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("Watch() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch() did not stop after cancellation")
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	if got := string(data); got != want {
		t.Fatalf("file %q = %q, want %q", path, got, want)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", path)
}
