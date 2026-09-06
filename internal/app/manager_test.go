package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/config"
)

func TestProjectManagerReconcile(t *testing.T) {
	root := t.TempDir()

	sourceA := filepath.Join(root, "source-a")
	sourceB := filepath.Join(root, "source-b")
	sourceC := filepath.Join(root, "source-c")

	destinationA := filepath.Join(root, "vault", "a")
	destinationB := filepath.Join(root, "vault", "b")
	destinationC := filepath.Join(root, "vault", "c")

	for _, path := range []string{sourceA, sourceB, sourceC} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeManagerTestFile(t, filepath.Join(sourceA, "README.md"), "# A")
	writeManagerTestFile(t, filepath.Join(sourceB, "README.md"), "# B")
	writeManagerTestFile(t, filepath.Join(sourceC, "README.md"), "# C")

	projects := []config.ResolvedProject{
		{
			Name:        "a",
			Source:      sourceA,
			Destination: destinationA,
		},
		{
			Name:        "b",
			Source:      sourceB,
			Destination: destinationB,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewProjectManager()

	if err := manager.Start(ctx, projects); err != nil {
		t.Fatalf("start: %v", err)
	}

	waitForManagerFile(t, filepath.Join(destinationA, "README.md"), "# A")
	waitForManagerFile(t, filepath.Join(destinationB, "README.md"), "# B")

	if len(manager.projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(manager.projects))
	}

	reconciled := []config.ResolvedProject{
		projects[0],
		{
			Name:        "c",
			Source:      sourceC,
			Destination: destinationC,
		},
	}

	if err := manager.Reconcile(ctx, reconciled); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if len(manager.projects) != 2 {
		t.Fatalf("expected 2 projects after reconcile, got %d", len(manager.projects))
	}

	if _, exists := manager.projects["a"]; !exists {
		t.Fatal("project a should still exist")
	}

	if _, exists := manager.projects["b"]; exists {
		t.Fatal("project b should have been removed")
	}

	if _, exists := manager.projects["c"]; !exists {
		t.Fatal("project c should have been added")
	}

	waitForManagerFile(t, filepath.Join(destinationC, "README.md"), "# C")

	writeManagerTestFile(t, filepath.Join(sourceA, "README.md"), "# A Updated")

	waitForManagerFile(
		t,
		filepath.Join(destinationA, "README.md"),
		"# A Updated",
	)

	manager.Stop()

	if len(manager.projects) != 0 {
		t.Fatalf("expected manager to be empty after Stop, got %d", len(manager.projects))
	}
}

func TestProjectManagerKeepsUnchangedProject(t *testing.T) {
	root := t.TempDir()

	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	writeManagerTestFile(t, filepath.Join(source, "README.md"), "# Initial")

	project := config.ResolvedProject{
		Name:        "test",
		Source:      source,
		Destination: destination,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewProjectManager()

	if err := manager.Start(ctx, []config.ResolvedProject{project}); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer manager.Stop()

	managedBefore := manager.projects["test"]

	if managedBefore == nil {
		t.Fatal("project was not started")
	}

	if err := manager.Reconcile(ctx, []config.ResolvedProject{project}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	managedAfter := manager.projects["test"]

	if managedBefore != managedAfter {
		t.Fatal("unchanged project should not have been restarted")
	}
}

func writeManagerTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitForManagerFile(t *testing.T, path, expected string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && string(data) == expected {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("waiting for %s: %v", path, err)
	}

	t.Fatalf("file %s: expected %q, got %q", path, expected, string(data))
}

func TestProjectManagerStopWaitsForProjects(t *testing.T) {
	root := t.TempDir()

	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	writeManagerTestFile(t, filepath.Join(source, "README.md"), "# Test")

	ctx := context.Background()

	manager := NewProjectManager()

	project := config.ResolvedProject{
		Name:        "test",
		Source:      source,
		Destination: destination,
	}

	if err := manager.Start(ctx, []config.ResolvedProject{project}); err != nil {
		t.Fatalf("start: %v", err)
	}

	manager.Stop()

	if len(manager.projects) != 0 {
		t.Fatalf("expected no managed projects after Stop, got %d", len(manager.projects))
	}
}
