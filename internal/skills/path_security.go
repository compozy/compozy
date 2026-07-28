package skills

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ensurePathWithinRoot(root string, candidate string) error {
	absRoot, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return fmt.Errorf("skills: resolve allowed root %q: %w", root, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("skills: resolve allowed root %q: %w", absRoot, err)
	}

	absCandidate, err := filepath.Abs(strings.TrimSpace(candidate))
	if err != nil {
		return fmt.Errorf("skills: resolve path %q: %w", candidate, err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return fmt.Errorf("skills: resolve path %q: %w", absCandidate, err)
	}

	relToRoot, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return fmt.Errorf("skills: relate path %q to root %q: %w", resolvedCandidate, resolvedRoot, err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return fmt.Errorf("skills: path %q escapes skill root %q", resolvedCandidate, resolvedRoot)
	}
	return nil
}
