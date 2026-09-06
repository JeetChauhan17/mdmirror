package mirror

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMirrorOnlyCopiesMarkdown(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	writeTestFile(t, source, "README.md", "# Project")
	writeTestFile(t, source, "src/main.go", "package main")
	writeTestFile(t, source, "data/data.json", "{}")
	writeTestFile(t, source, "docs/architecture.md", "# Architecture")
	writeTestFile(t, source, "docs/notes.txt", "not markdown")
	writeTestFile(t, source, "deployment/prod/deploy.markdown", "# Deploy")
	writeTestFile(t, source, "empty-folder/file.txt", "nothing")

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("Mirror() failed: %v", err)
	}

	assertExists(t, filepath.Join(destination, "README.md"))
	assertExists(t, filepath.Join(destination, "docs", "architecture.md"))
	assertExists(t, filepath.Join(destination, "deployment", "prod", "deploy.markdown"))

	assertNotExists(t, filepath.Join(destination, "src"))
	assertNotExists(t, filepath.Join(destination, "data"))
	assertNotExists(t, filepath.Join(destination, "docs", "notes.txt"))
	assertNotExists(t, filepath.Join(destination, "empty-folder"))
}

func TestMirrorPreservesContent(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	content := "# Hello\n\nThis came from the project."

	writeTestFile(t, source, "docs/test.md", content)

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("Mirror() failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destination, "docs", "test.md"))
	if err != nil {
		t.Fatalf("read mirrored file: %v", err)
	}

	if string(got) != content {
		t.Fatalf("content mismatch:\nwant: %q\ngot:  %q", content, string(got))
	}
}

func TestMirrorRemovesDeletedMarkdown(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	writeTestFile(t, source, "docs/keep.md", "keep")
	writeTestFile(t, source, "docs/remove.md", "remove")

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("initial Mirror() failed: %v", err)
	}

	if err := os.Remove(filepath.Join(source, "docs", "remove.md")); err != nil {
		t.Fatalf("remove source file: %v", err)
	}

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("second Mirror() failed: %v", err)
	}

	assertExists(t, filepath.Join(destination, "docs", "keep.md"))
	assertNotExists(t, filepath.Join(destination, "docs", "remove.md"))
}

func TestMirrorRemovesDirectoriesWithNoMarkdown(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	writeTestFile(t, source, "docs/notes.md", "notes")
	writeTestFile(t, source, "deployment/deploy.md", "deploy")

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("initial Mirror() failed: %v", err)
	}

	if err := os.Remove(filepath.Join(source, "deployment", "deploy.md")); err != nil {
		t.Fatalf("remove source file: %v", err)
	}

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("second Mirror() failed: %v", err)
	}

	assertNotExists(t, filepath.Join(destination, "deployment"))
	assertExists(t, filepath.Join(destination, "docs"))
}

func TestMarkdownExtensionIsCaseInsensitive(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	writeTestFile(t, source, "lower.md", "lower")
	writeTestFile(t, source, "upper.MD", "upper")
	writeTestFile(t, source, "mixed.MaRkDoWn", "mixed")

	if err := Mirror(source, destination); err != nil {
		t.Fatalf("Mirror() failed: %v", err)
	}

	assertExists(t, filepath.Join(destination, "lower.md"))
	assertExists(t, filepath.Join(destination, "upper.MD"))
	assertExists(t, filepath.Join(destination, "mixed.MaRkDoWn"))
}

func writeTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(relativePath))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to exist", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking %s: %v", path, err)
	}
}

func TestMirrorRejectsDestinationInsideSource(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(source, "mirror")

	writeTestFile(t, source, "README.md", "# Project")

	if err := Mirror(source, destination); err == nil {
		t.Fatal("Mirror() should reject destination inside source")
	}
}

func TestMirrorRejectsSameSourceAndDestination(t *testing.T) {
	source := t.TempDir()

	writeTestFile(t, source, "README.md", "# Project")

	if err := Mirror(source, source); err == nil {
		t.Fatal("Mirror() should reject identical source and destination")
	}
}
