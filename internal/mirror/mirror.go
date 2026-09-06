package mirror

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Mirror creates a documentation-only mirror of source inside destination.
//
// Only Markdown files (.md and .markdown) are copied. Directory structure
// is preserved, but directories that contain no Markdown files are omitted.
//
// The source tree is never modified.
func Mirror(source, destination string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	destination, err = filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	// source, err = filepath.Clean(source), nil
	source = filepath.Clean(source)
	destination = filepath.Clean(destination)
	if err != nil {
		return fmt.Errorf("clean source path: %w", err)
	}

	destination, err = filepath.Clean(destination), nil
	if err != nil {
		return fmt.Errorf("clean destination path: %w", err)
	}

	if source == destination {
		return fmt.Errorf("source and destination cannot be the same directory")
	}

	if isPathInside(destination, source) {
		return fmt.Errorf("destination cannot be inside source")
	}

	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", source)
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	expected := make(map[string]struct{})

	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}

		if path == source {
			return nil
		}

		// Never follow symbolic links. This applies to both files and
		// directories. A symlink is not part of the mirrored project tree.
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

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("calculate relative path for %s: %w", path, err)
		}

		target := filepath.Join(destination, relative)
		expected[target] = struct{}{}

		if err := copyFile(path, target); err != nil {
			return fmt.Errorf("copy %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Remove Markdown files that no longer exist in the source.
	if err := removeStaleMarkdown(destination, expected); err != nil {
		return fmt.Errorf("remove stale files: %w", err)
	}

	// Remove directories that became empty after stale files were removed.
	if err := removeEmptyDirectories(destination); err != nil {
		return fmt.Errorf("remove empty directories: %w", err)
	}

	return nil
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

	if relative == "." {
		return false
	}

	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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

	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()

	if copyErr != nil {
		return copyErr
	}

	if closeErr != nil {
		return closeErr
	}

	// Preserve the source file's permission bits where practical.
	if err := os.Chmod(destination, info.Mode().Perm()); err != nil {
		return err
	}

	return nil
}

func removeStaleMarkdown(destination string, expected map[string]struct{}) error {
	return filepath.WalkDir(destination, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		if !isMarkdown(entry.Name()) {
			return nil
		}

		if _, exists := expected[path]; exists {
			return nil
		}

		return os.Remove(path)
	})
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

	// Delete deepest directories first.
	for i := len(directories) - 1; i >= 0; i-- {
		_ = os.Remove(directories[i])
	}

	return nil
}
