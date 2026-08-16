package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrPathOutsideRoot identifies an existing path whose resolved target escapes
// the resolved root.
var ErrPathOutsideRoot = errors.New("fileutil: resolved path is outside root")

// CanonicalPathWithExistingPrefix resolves symlinks in the deepest existing
// prefix while preserving any missing suffix components.
func CanonicalPathWithExistingPrefix(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", fmt.Errorf("fileutil: make path %q absolute: %w", path, err)
	}
	existing := absolute
	missing := make([]string, 0, 4)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(existing)
		if resolveErr == nil {
			for _, component := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, component)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", fmt.Errorf("fileutil: resolve path %q: %w", existing, resolveErr)
		}
		if info, lstatErr := os.Lstat(existing); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("fileutil: path %q contains an unresolved symlink", path)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("fileutil: resolve path %q: %w", absolute, resolveErr)
		}
		missing = append(missing, filepath.Base(existing))
		existing = parent
	}
}

// CanonicalExistingDirectory returns the absolute symlink-resolved identity of
// an existing directory.
func CanonicalExistingDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", fmt.Errorf("fileutil: make directory %q absolute: %w", path, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("fileutil: resolve directory %q: %w", absolute, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("fileutil: inspect directory %q: %w", canonical, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("fileutil: path %q is not a directory", canonical)
	}
	return filepath.Clean(canonical), nil
}

// PathWithinRoot reports whether candidate is lexically contained by root.
// Callers that need physical identity containment must resolve paths first.
func PathWithinRoot(root string, candidate string) (bool, error) {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false, err
	}
	return relative == "." ||
		(relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
			!filepath.IsAbs(relative)), nil
}

// ResolveExistingPathWithinRoot resolves two existing paths and verifies that
// candidate remains within root after following symlinks.
func ResolveExistingPathWithinRoot(root string, candidate string) (
	resolvedRoot string,
	resolvedCandidate string,
	err error,
) {
	absRoot, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", "", fmt.Errorf("fileutil: make root %q absolute: %w", root, err)
	}
	resolvedRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", fmt.Errorf("fileutil: resolve root %q: %w", absRoot, err)
	}

	absCandidate, err := filepath.Abs(strings.TrimSpace(candidate))
	if err != nil {
		return resolvedRoot, "", fmt.Errorf("fileutil: make candidate %q absolute: %w", candidate, err)
	}
	resolvedCandidate, err = filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return resolvedRoot, "", fmt.Errorf("fileutil: resolve candidate %q: %w", absCandidate, err)
	}

	contained, err := PathWithinRoot(resolvedRoot, resolvedCandidate)
	if err != nil {
		return resolvedRoot, resolvedCandidate, fmt.Errorf(
			"fileutil: relate candidate %q to root %q: %w",
			resolvedCandidate,
			resolvedRoot,
			err,
		)
	}
	if !contained {
		return resolvedRoot, resolvedCandidate, fmt.Errorf(
			"%w: candidate %q root %q",
			ErrPathOutsideRoot,
			resolvedCandidate,
			resolvedRoot,
		)
	}
	return resolvedRoot, resolvedCandidate, nil
}
