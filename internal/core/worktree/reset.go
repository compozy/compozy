package worktree

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrResetUnsupported means the baseline snapshot cannot anchor a reset: it
	// was captured outside a git repository, without git available, or before any
	// commit existed.
	ErrResetUnsupported = errors.New("worktree: baseline snapshot cannot anchor a clean reset")
	// ErrResetDirtyBaseline means the baseline carried uncommitted or untracked
	// changes. A snapshot fingerprints that content but does not store it, so the
	// baseline cannot be reconstructed and a hard reset would destroy real work.
	ErrResetDirtyBaseline = errors.New("worktree: baseline snapshot has uncommitted changes")
	// ErrResetRootMismatch means the baseline was captured from a different root
	// than the one the caller asked to reset.
	ErrResetRootMismatch = errors.New("worktree: baseline snapshot was captured from a different root")
	// ErrResetIncomplete means git reported success but the worktree did not come
	// back to the baseline fingerprint.
	ErrResetIncomplete = errors.New("worktree: reset did not restore the baseline state")
)

// Reset restores root to the exact state recorded by baseline, discarding every
// commit, staged change, and untracked file produced since the snapshot was
// taken. Ignored files are left alone.
//
// Only a clean baseline can be restored. Callers should treat any error as
// "a clean reset is not possible for this worktree" and fall back to whatever
// their non-destructive path is; the worktree is left untouched when Reset
// returns before invoking git.
func Reset(ctx context.Context, root string, baseline Snapshot) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("%w: workspace root is required", ErrResetUnsupported)
	}
	if !baseline.IsSupported() {
		return fmt.Errorf("%w: %s", ErrResetUnsupported, baseline.UnsupportedReason())
	}
	if len(baseline.entries) > 0 {
		return fmt.Errorf("%w: %d dirty path(s) at baseline", ErrResetDirtyBaseline, len(baseline.entries))
	}
	if baseline.root != "" && !sameRoot(baseline.root, root) {
		return fmt.Errorf("%w: baseline captured at %s", ErrResetRootMismatch, baseline.root)
	}
	if _, err := runGit(ctx, root, "reset", "--hard", baseline.head); err != nil {
		return fmt.Errorf("worktree: reset %s to %s: %w", root, baseline.head, err)
	}
	if _, err := runGit(ctx, root, "clean", "-fd"); err != nil {
		return fmt.Errorf("worktree: clean %s: %w", root, err)
	}
	final, err := Capture(ctx, root)
	if err != nil {
		return fmt.Errorf("worktree: verify reset of %s: %w", root, err)
	}
	if !final.Equal(baseline) {
		return fmt.Errorf("%w: %s", ErrResetIncomplete, root)
	}
	return nil
}

func sameRoot(a string, b string) bool {
	absA, err := filepath.Abs(filepath.Clean(a))
	if err != nil {
		return false
	}
	absB, err := filepath.Abs(filepath.Clean(b))
	if err != nil {
		return false
	}
	return absA == absB
}
