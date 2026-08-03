package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveResourcePathWithinRoot(rootDir string, relativePath string, resourceLabel string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return "", fmt.Errorf("extension: resolve resource root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("extension: resolve resource root symlink: %w", err)
	}

	trimmedPath := strings.TrimSpace(relativePath)
	candidate := filepath.Join(root, filepath.Clean(trimmedPath))
	if resourcePathEscapesRoot(root, candidate) {
		return "", invalidResourcePathError(resourceLabel, trimmedPath, "path escapes extension root")
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", invalidResourcePathError(resourceLabel, trimmedPath, "path does not exist")
		}
		return "", fmt.Errorf("extension: resolve %s path %q symlink: %w", resourceLabel, trimmedPath, err)
	}
	if resourcePathEscapesRoot(root, resolved) {
		return "", invalidResourcePathError(resourceLabel, trimmedPath, "path escapes extension root")
	}
	return resolved, nil
}

func invalidResourcePathError(resourceLabel string, relativePath string, message string) error {
	return &ManifestValidationError{
		Field:   strings.TrimSpace(resourceLabel),
		Value:   strings.TrimSpace(relativePath),
		Message: message,
	}
}

func resourcePathEscapesRoot(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return true
	}
	return filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func rejectResourceTreeSymlinks(rootDir string, resourceLabel string) error {
	return filepath.WalkDir(rootDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("extension: inspect %s path %q: %w", resourceLabel, path, walkErr)
		}
		if entry.Type()&os.ModeSymlink == 0 {
			return nil
		}
		relative, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("extension: resolve %s path %q: %w", resourceLabel, path, err)
		}
		return invalidResourcePathError(resourceLabel, filepath.ToSlash(relative), "symlinks are not allowed")
	})
}
