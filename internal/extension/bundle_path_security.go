package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveBundlePathWithinRoot(
	rootDir string,
	relativePath string,
	resourceLabel string,
) (string, error) {
	root, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return "", fmt.Errorf("extension: resolve bundle root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("extension: resolve bundle root symlink: %w", err)
	}

	trimmedPath := strings.TrimSpace(relativePath)
	candidate := filepath.Join(root, filepath.Clean(trimmedPath))
	if bundlePathEscapesRoot(root, candidate) {
		return "", bundlePathEscapeError(resourceLabel, trimmedPath)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf(
				"%w: %s path %q does not exist",
				ErrBundleInvalid,
				resourceLabel,
				trimmedPath,
			)
		}
		return "", fmt.Errorf(
			"extension: resolve %s path %q symlink: %w",
			resourceLabel,
			trimmedPath,
			err,
		)
	}
	if bundlePathEscapesRoot(root, resolved) {
		return "", bundlePathEscapeError(resourceLabel, trimmedPath)
	}
	return resolved, nil
}

func bundlePathEscapeError(resourceLabel string, relativePath string) error {
	return fmt.Errorf(
		"%w: %s path %q escapes extension root",
		ErrBundleInvalid,
		resourceLabel,
		relativePath,
	)
}

func bundlePathEscapesRoot(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return true
	}
	return filepath.IsAbs(relative) ||
		relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
