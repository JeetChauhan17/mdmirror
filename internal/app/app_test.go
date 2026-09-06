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

func TestWatchStopsProjectManager(t *testing.T) {
	root := t.TempDir()

	source := filepath.Join(root, "source")
	vault := filepath.Join(root, "vault")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "project",
				Source:      source,
				Destination: filepath.Join(vault, "project"),
			},
		},
	}

	application := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		errCh <- application.Watch(ctx)
	}()

	waitForFile(t, filepath.Join(vault, "project"))

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("Watch() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch() did not stop after cancellation")
	}

	if len(application.manager.projects) != 0 {
		t.Fatalf(
			"project manager still has %d projects after Watch() stopped",
			len(application.manager.projects),
		)
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

func TestReconcile(t *testing.T) {
	root := t.TempDir()

	sourceA := filepath.Join(root, "source-a")
	sourceB := filepath.Join(root, "source-b")
	vault := filepath.Join(root, "vault")

	for _, source := range []string{sourceA, sourceB} {
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(sourceA, "README.md"), "# A")
	writeFile(t, filepath.Join(sourceB, "README.md"), "# B")

	cfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "a",
				Source:      sourceA,
				Destination: filepath.Join(vault, "a"),
			},
		},
	}

	application := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := application.manager.Start(ctx, cfg.Projects); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer application.manager.Stop()

	waitForFile(t, filepath.Join(vault, "a", "README.md"))

	updatedCfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "a",
				Source:      sourceA,
				Destination: filepath.Join(vault, "a"),
			},
			{
				Name:        "b",
				Source:      sourceB,
				Destination: filepath.Join(vault, "b"),
			},
		},
	}

	if err := application.Reconcile(ctx, updatedCfg); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	waitForFile(t, filepath.Join(vault, "b", "README.md"))

	if len(application.manager.projects) != 2 {
		t.Fatalf(
			"expected 2 projects after reconcile, got %d",
			len(application.manager.projects),
		)
	}
}

func TestReconcileAddsProject(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sourceA := filepath.Join(t.TempDir(), "source-a")
	sourceB := filepath.Join(t.TempDir(), "source-b")
	vault := filepath.Join(t.TempDir(), "vault")

	if err := os.MkdirAll(sourceA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceB, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sourceA, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ResolvedConfig{
		Vault: vault,
		Projects: []config.ResolvedProject{
			{
				Name:        "project-a",
				Source:      sourceA,
				Destination: filepath.Join(vault, "project-a"),
			},
		},
	}

	application := New(cfg)

	if err := application.manager.Start(ctx, cfg.Projects); err != nil {
		t.Fatal(err)
	}
	defer application.manager.Stop()

	if err := application.Reconcile(ctx, config.ResolvedConfig{
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
	}); err != nil {
		t.Fatal(err)
	}

	destinationB := filepath.Join(vault, "project-b")

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(destinationB); err == nil {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("project-b destination was not created within 2 seconds")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if len(application.manager.projects) != 2 {
		t.Fatalf("expected 2 managed projects, got %d", len(application.manager.projects))
	}
}
