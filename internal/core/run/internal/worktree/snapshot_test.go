package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureReturnsUnsupportedSnapshotForBlankRoot(t *testing.T) {
	snap, err := Capture(context.Background(), "")
	if err != nil {
		t.Fatalf("Capture(\"\"): %v", err)
	}
	if snap.IsSupported() {
		t.Fatalf("expected unsupported snapshot for blank root, got supported")
	}
}

func TestCaptureReturnsUnsupportedSnapshotForNonGitWorkspace(t *testing.T) {
	dir := t.TempDir()
	snap, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture(non-git): %v", err)
	}
	if snap.IsSupported() {
		t.Fatalf("expected unsupported snapshot for non-git workspace, got supported")
	}
}

func TestCaptureProducesEqualSnapshotsWhenWorkingTreeIsUnchanged(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	snapA, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("first Capture: %v", err)
	}
	if !snapA.IsSupported() {
		t.Fatalf("expected supported snapshot, got unsupported")
	}

	snapB, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("second Capture: %v", err)
	}
	if !snapA.Equal(snapB) {
		t.Fatalf("expected unchanged working tree to yield equal snapshots")
	}
}

func TestCaptureDetectsAddedFile(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	before, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture before: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	after, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture after: %v", err)
	}
	if before.Equal(after) {
		t.Fatalf("expected snapshot to change after adding an untracked file")
	}
}

func TestCaptureDetectsModifiedTrackedFile(t *testing.T) {
	dir := initGitRepoWithCommit(t)

	before, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture before: %v", err)
	}

	tracked := filepath.Join(dir, "README.md")
	if err := os.WriteFile(tracked, []byte("# changed\n"), 0o600); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}

	after, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture after: %v", err)
	}
	if before.Equal(after) {
		t.Fatalf("expected snapshot to change after modifying a tracked file")
	}
}

func TestUnsupportedSnapshotsNeverCompareEqual(t *testing.T) {
	var a, b Snapshot
	if a.Equal(b) {
		t.Fatalf("zero-value snapshots must not compare equal — they signal an unknown state")
	}

	dir := initGitRepoWithCommit(t)
	supported, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !supported.IsSupported() {
		t.Fatalf("expected supported snapshot")
	}
	if supported.Equal(a) {
		t.Fatalf("supported snapshot must not compare equal to unsupported snapshot")
	}
	if a.Equal(supported) {
		t.Fatalf("unsupported snapshot must not compare equal to supported snapshot")
	}
}

// initGitRepoWithCommit prepares a temporary git repository with one committed
// file. Tests that compare snapshots need a non-empty HEAD plus deterministic
// committer identity so the digest is reproducible across machines.
func initGitRepoWithCommit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", "main")
	mustGit(t, dir, "config", "user.email", "snapshot@example.com")
	mustGit(t, dir, "config", "user.name", "Snapshot Tester")
	mustGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# initial\n"), 0o600); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	mustGit(t, dir, "add", "README.md")
	mustGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, string(out))
	}
}
