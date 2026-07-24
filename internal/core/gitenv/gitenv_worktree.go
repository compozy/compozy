package gitenv

import (
	"path/filepath"
	"strings"
)

// ParseWorktreeList parses raw `git worktree list --porcelain` output into
// cleaned worktree paths in listed order. It is pure: no git execution, no I/O,
// and no policy. Every `worktree ` path is returned, primary and prunable
// entries included; callers own primary-drop, de-duplication, sorting, and any
// ownership filter (ADR-004). Attribute lines (HEAD, branch, detached, bare,
// prunable) are skipped. Only the leading prefix is stripped, so paths with
// spaces stay intact; each path is filepath.Clean-normalized for stable dedup.
// Empty or malformed input yields a non-nil empty slice and never panics.
func ParseWorktreeList(output string) []string {
	roots := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		roots = append(roots, filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))))
	}
	return roots
}
