// Package worktree captures a deterministic fingerprint of a workspace's
// uncommitted state so callers can detect whether an arbitrary operation (an
// agent session, a hook, a task job) actually modified any files.
//
// The fingerprint is derived from `git rev-parse HEAD` plus
// `git status --porcelain=v1 -z --untracked-files=all`. When the workspace is
// not a git repository, has no commits yet, or the `git` binary is missing,
// Capture returns an unsupported Snapshot rather than an error so callers can
// degrade gracefully and preserve current behavior.
package worktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const captureSchemaVersion = "compozy-worktree-v1"

// Snapshot is an opaque deterministic fingerprint of a working tree. Snapshots
// captured at different points in time can be compared with Equal to decide
// whether the working tree changed between the two captures.
type Snapshot struct {
	digest    string
	supported bool
}

// IsSupported reports whether the snapshot was successfully captured. An
// unsupported snapshot is the result of a missing prerequisite (no git
// metadata, no `git` binary, empty repo) and intentionally never compares
// equal to anything so callers fall back to the prior behavior.
func (s Snapshot) IsSupported() bool { return s.supported }

// Equal reports whether two snapshots represent the same working-tree state.
// Unsupported snapshots never compare equal — including against each other —
// so the no-op detection only triggers when both pre and post captures
// succeeded.
func (s Snapshot) Equal(other Snapshot) bool {
	if !s.supported || !other.supported {
		return false
	}
	return s.digest == other.digest
}

// Capture takes a working-tree fingerprint of the workspace at root using
// `git`. A blank root, missing `.git` directory, missing `git` binary, or
// repository without any commits all yield an unsupported Snapshot with a nil
// error. Genuine I/O or process errors propagate.
func Capture(ctx context.Context, root string) (Snapshot, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Snapshot{}, nil
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("worktree: stat .git in %s: %w", root, err)
	}
	head, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		// Any rev-parse failure (missing git binary, empty repo with no commits,
		// corrupted refs) yields an unsupported Snapshot rather than an error so
		// the runner falls back to legacy completion behavior. Surfacing the
		// failure here would force every non-git or fresh-repo workspace through
		// the error path even though the no-op check is purely advisory.
		if isExecLookupError(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, nil
	}
	porcelain, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		if isExecLookupError(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("worktree: git status in %s: %w", root, err)
	}
	h := sha256.New()
	h.Write([]byte(captureSchemaVersion))
	h.Write([]byte{0})
	h.Write(bytes.TrimSpace(head))
	h.Write([]byte{0})
	h.Write(porcelain)
	return Snapshot{digest: hex.EncodeToString(h.Sum(nil)), supported: true}, nil
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func isExecLookupError(err error) bool {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Is(execErr.Err, exec.ErrNotFound)
	}
	return errors.Is(err, exec.ErrNotFound)
}
