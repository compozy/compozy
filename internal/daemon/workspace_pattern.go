package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func workspaceRelativePattern(root string, pattern string) (string, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", fmt.Errorf("workspace root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root %q: %w", rootAbs, err)
	}
	patternAbs, err := filepath.Abs(filepath.Join(rootAbs, strings.TrimSpace(pattern)))
	if err != nil {
		return "", fmt.Errorf("pattern: %w", err)
	}
	if err := requirePathWithinResolvedRoot(rootAbs, patternAbs); err != nil {
		return "", fmt.Errorf("relative pattern %q escapes %q: %w", pattern, rootAbs, err)
	}

	anchor, err := realpathDeepestExistingPath(globStaticDirectory(patternAbs))
	if err != nil {
		return "", fmt.Errorf("resolve pattern directory %q: %w", patternAbs, err)
	}
	if err := requirePathWithinResolvedRoot(resolvedRoot, anchor); err != nil {
		return "", fmt.Errorf("pattern directory %q escapes workspace root %q: %w", anchor, resolvedRoot, err)
	}
	matches, err := filepath.Glob(patternAbs)
	if err != nil {
		return "", fmt.Errorf("glob pattern %q: %w", patternAbs, err)
	}
	for _, match := range matches {
		resolvedMatch, resolveErr := filepath.EvalSymlinks(match)
		if resolveErr != nil {
			return "", fmt.Errorf("resolve glob match %q: %w", match, resolveErr)
		}
		if err := requirePathWithinResolvedRoot(resolvedRoot, resolvedMatch); err != nil {
			return "", fmt.Errorf("glob match %q escapes workspace root %q: %w", match, resolvedRoot, err)
		}
	}
	return patternAbs, nil
}

func globStaticDirectory(pattern string) string {
	meta := strings.IndexAny(pattern, "*?[")
	if meta < 0 {
		return filepath.Dir(pattern)
	}
	prefix := pattern[:meta]
	if strings.HasSuffix(prefix, string(filepath.Separator)) {
		return filepath.Clean(prefix)
	}
	return filepath.Dir(prefix)
}

func realpathDeepestExistingPath(target string) (string, error) {
	current := filepath.Clean(target)
	suffix := make([]string, 0, 4)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for _, segment := range slices.Backward(suffix) {
				resolved = filepath.Join(resolved, segment)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func requirePathWithinResolvedRoot(root string, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return fmt.Errorf("relate path %q to root %q: %w", candidate, root, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside %q", candidate, root)
	}
	return nil
}
