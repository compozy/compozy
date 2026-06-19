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

// UnsupportedReason describes why a working-tree snapshot could not be
// captured.
type UnsupportedReason string

const (
	// UnsupportedReasonBlankRoot means the caller did not provide a workspace
	// root.
	UnsupportedReasonBlankRoot UnsupportedReason = "blank_root"
	// UnsupportedReasonNonGit means the workspace root does not contain git
	// metadata.
	UnsupportedReasonNonGit UnsupportedReason = "non_git"
	// UnsupportedReasonGitMissing means the git executable could not be found.
	UnsupportedReasonGitMissing UnsupportedReason = "git_missing"
	// UnsupportedReasonNoCommits means HEAD could not be resolved, most often
	// because the repository has no commits yet.
	UnsupportedReasonNoCommits UnsupportedReason = "no_commits"
)

// Snapshot is an opaque deterministic fingerprint of a working tree. Snapshots
// captured at different points in time can be compared with Equal to decide
// whether the working tree changed between the two captures.
type Snapshot struct {
	digest            string
	head              string
	branch            string
	porcelain         []byte
	unsupportedReason UnsupportedReason
	gitAvailable      bool
	gitRepo           bool
	hasCommits        bool
	supported         bool
}

// IsSupported reports whether the snapshot was successfully captured. An
// unsupported snapshot is the result of a missing prerequisite (no git
// metadata, no `git` binary, empty repo) and intentionally never compares
// equal to anything so callers fall back to the prior behavior.
func (s Snapshot) IsSupported() bool { return s.supported }

// Digest returns the deterministic snapshot digest for supported captures.
func (s Snapshot) Digest() string { return s.digest }

// Head returns the HEAD commit captured for supported snapshots.
func (s Snapshot) Head() string { return s.head }

// Branch returns the current branch name captured for supported snapshots.
func (s Snapshot) Branch() string { return s.branch }

// Porcelain returns a copy of the raw git porcelain v1 -z status payload.
func (s Snapshot) Porcelain() []byte { return append([]byte(nil), s.porcelain...) }

// UnsupportedReason returns the reason a snapshot is unsupported.
func (s Snapshot) UnsupportedReason() UnsupportedReason { return s.unsupportedReason }

// GitAvailable reports whether a git executable was available to the capture.
func (s Snapshot) GitAvailable() bool { return s.gitAvailable }

// IsGitRepo reports whether the workspace root had git metadata.
func (s Snapshot) IsGitRepo() bool { return s.gitRepo }

// HasCommits reports whether HEAD resolved during capture.
func (s Snapshot) HasCommits() bool { return s.hasCommits }

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
		return Snapshot{gitAvailable: gitAvailable(), unsupportedReason: UnsupportedReasonBlankRoot}, nil
	}
	snapshot := Snapshot{gitAvailable: gitAvailable()}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			snapshot.unsupportedReason = UnsupportedReasonNonGit
			return snapshot, nil
		}
		return Snapshot{}, fmt.Errorf("worktree: stat .git in %s: %w", root, err)
	}
	snapshot.gitRepo = true
	if !snapshot.gitAvailable {
		snapshot.unsupportedReason = UnsupportedReasonGitMissing
		return snapshot, nil
	}
	head, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		// Any rev-parse failure (missing git binary, empty repo with no commits,
		// corrupted refs) yields an unsupported Snapshot rather than an error so
		// the runner falls back to legacy completion behavior. Surfacing the
		// failure here would force every non-git or fresh-repo workspace through
		// the error path even though the no-op check is purely advisory.
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		snapshot.unsupportedReason = UnsupportedReasonNoCommits
		return snapshot, nil
	}
	snapshot.head = string(bytes.TrimSpace(head))
	snapshot.hasCommits = true
	branch, err := runGit(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("worktree: git branch in %s: %w", root, err)
	}
	snapshot.branch = string(bytes.TrimSpace(branch))
	porcelain, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("worktree: git status in %s: %w", root, err)
	}
	h := sha256.New()
	h.Write([]byte(captureSchemaVersion))
	h.Write([]byte{0})
	h.Write([]byte(snapshot.head))
	h.Write([]byte{0})
	h.Write(porcelain)
	return Snapshot{
		digest:       hex.EncodeToString(h.Sum(nil)),
		head:         snapshot.head,
		branch:       snapshot.branch,
		porcelain:    append([]byte(nil), porcelain...),
		gitAvailable: true,
		gitRepo:      true,
		hasCommits:   true,
		supported:    true,
	}, nil
}

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(sanitizedGitEnv(), "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// sanitizedGitEnv returns os.Environ() with repository-selection variables
// stripped so cmd.Dir is the only signal git uses to locate the working tree.
// If the parent process inherited GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR,
// GIT_INDEX_FILE, or GIT_NAMESPACE (e.g. invoked from a hook or a wrapper
// script that is mid-operation on another repo), those variables would take
// precedence over cmd.Dir and silently produce a snapshot of the wrong tree.
func sanitizedGitEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "GIT_DIR="),
			strings.HasPrefix(kv, "GIT_WORK_TREE="),
			strings.HasPrefix(kv, "GIT_COMMON_DIR="),
			strings.HasPrefix(kv, "GIT_INDEX_FILE="),
			strings.HasPrefix(kv, "GIT_NAMESPACE="):
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

func isExecLookupError(err error) bool {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Is(execErr.Err, exec.ErrNotFound)
	}
	return errors.Is(err, exec.ErrNotFound)
}
