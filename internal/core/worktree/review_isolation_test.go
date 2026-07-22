package worktree

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewIsolationAppliesIndependentJobChanges(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	issuePath := filepath.Join(reviewsDir, "issue_001.md")
	if err := os.WriteFile(issuePath, []byte("status: pending\n"), 0o600); err != nil {
		t.Fatalf("write review issue: %v", err)
	}
	techspecPath := filepath.Join(filepath.Dir(reviewsDir), "_techspec.md")
	if err := os.WriteFile(techspecPath, []byte("# TechSpec\n"), 0o600); err != nil {
		t.Fatalf("write workflow artifact: %v", err)
	}

	isolation, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a", "batch-b"},
	)
	if err != nil {
		t.Fatalf("NewReviewIsolation() error = %v", err)
	}
	first, err := isolation.Workspace(0)
	if err != nil {
		t.Fatalf("Workspace(0) error = %v", err)
	}
	second, err := isolation.Workspace(1)
	if err != nil {
		t.Fatalf("Workspace(1) error = %v", err)
	}
	if first.Root == second.Root || first.Root == repo || second.Root == repo {
		t.Fatalf("review roots are not isolated: source=%q first=%q second=%q", repo, first.Root, second.Root)
	}
	if info, statErr := os.Stat(first.ReviewsDir); statErr != nil || !info.IsDir() {
		t.Fatalf("workspace reviews path %q is not a directory: %v", first.ReviewsDir, statErr)
	}
	if body, readErr := os.ReadFile(filepath.Join(first.ReviewsDir, "issue_001.md")); readErr != nil ||
		string(body) != "status: pending\n" {
		t.Fatalf("mirrored issue = %q, error = %v", body, readErr)
	}
	if body, readErr := os.ReadFile(filepath.Join(filepath.Dir(first.ReviewsDir), "_techspec.md")); readErr != nil ||
		string(body) != "# TechSpec\n" {
		t.Fatalf("mirrored workflow artifact = %q, error = %v", body, readErr)
	}

	baseline, err := Capture(context.Background(), first.Root)
	if err != nil {
		t.Fatalf("Capture(first baseline) error = %v", err)
	}
	if len(baseline.Entries()) != 0 {
		t.Fatalf("isolated baseline is dirty: %#v", baseline.Entries())
	}
	if err := os.WriteFile(filepath.Join(first.Root, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatalf("write first result: %v", err)
	}
	if err := Reset(context.Background(), first.Root, baseline); err != nil {
		t.Fatalf("Reset(isolated workspace) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(first.Root, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("reset left first attempt output: %v", err)
	}

	if err := os.WriteFile(filepath.Join(first.Root, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatalf("write first retry result: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(first.ReviewsDir, "issue_001.md"),
		[]byte("status: resolved\n"),
		0o600,
	); err != nil {
		t.Fatalf("resolve first issue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(second.Root, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatalf("write second result: %v", err)
	}

	if err := isolation.Apply(context.Background(), 0, false, "fix: batch a"); err != nil {
		t.Fatalf("Apply(first) error = %v", err)
	}
	if err := isolation.Apply(context.Background(), 1, false, "fix: batch b"); err != nil {
		t.Fatalf("Apply(second) error = %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join(repo, "a.txt"): "a\n",
		filepath.Join(repo, "b.txt"): "b\n",
		issuePath:                    "status: resolved\n",
	} {
		body, readErr := os.ReadFile(path)
		if readErr != nil || string(body) != want {
			t.Fatalf("integrated %s = %q, error = %v, want %q", path, body, readErr, want)
		}
	}
}

func TestReviewIsolationPreservesConflictingWorkspace(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}

	isolation, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a", "batch-b"},
	)
	if err != nil {
		t.Fatalf("NewReviewIsolation() error = %v", err)
	}
	first, _ := isolation.Workspace(0)
	second, _ := isolation.Workspace(1)
	if err := os.WriteFile(filepath.Join(first.Root, "README.md"), []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write first conflict: %v", err)
	}
	if err := os.WriteFile(filepath.Join(second.Root, "README.md"), []byte("second\n"), 0o600); err != nil {
		t.Fatalf("write second conflict: %v", err)
	}
	if err := isolation.Apply(context.Background(), 0, false, "fix: first"); err != nil {
		t.Fatalf("Apply(first) error = %v", err)
	}
	err = isolation.Apply(context.Background(), 1, false, "fix: second")
	if err == nil || !strings.Contains(err.Error(), "apply isolated review changes") {
		t.Fatalf("Apply(second) error = %v, want integration conflict", err)
	}
	if _, statErr := os.Stat(second.Root); statErr != nil {
		t.Fatalf("conflicting worktree was not preserved: %v", statErr)
	}
}

func TestReviewIsolationCommitsExactIntegratedBatch(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	issuePath := filepath.Join(reviewsDir, "issue_001.md")
	if err := os.WriteFile(issuePath, []byte("status: pending\n"), 0o600); err != nil {
		t.Fatalf("write issue: %v", err)
	}

	isolation, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a"},
	)
	if err != nil {
		t.Fatalf("NewReviewIsolation() error = %v", err)
	}
	workspace, _ := isolation.Workspace(0)
	if err := os.WriteFile(
		filepath.Join(workspace.ReviewsDir, "issue_001.md"),
		[]byte("status: resolved\n"),
		0o600,
	); err != nil {
		t.Fatalf("resolve issue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Root, "fix.go"), []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}
	if err := isolation.Apply(context.Background(), 0, true, "fix: batch a"); err != nil {
		t.Fatalf("Apply(auto commit) error = %v", err)
	}
	if got := strings.TrimSpace(string(mustRunGit(t, repo, "rev-list", "--count", "HEAD"))); got != "2" {
		t.Fatalf("source commit count = %q, want 2", got)
	}
	if got := strings.TrimSpace(string(mustRunGit(t, repo, "log", "-1", "--format=%s"))); got != "fix: batch a" {
		t.Fatalf("source commit subject = %q, want fix: batch a", got)
	}
	if status := strings.TrimSpace(string(mustRunGit(t, repo, "status", "--porcelain"))); status != "" {
		t.Fatalf("source status after exact batch commit = %q, want clean", status)
	}
}

func TestReviewIsolationRestoresSourceWhenAutoCommitHookRejects(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	issuePath := filepath.Join(reviewsDir, "issue_001.md")
	if err := os.WriteFile(issuePath, []byte("status: pending\n"), 0o600); err != nil {
		t.Fatalf("write issue: %v", err)
	}

	isolation, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a"},
	)
	if err != nil {
		t.Fatalf("NewReviewIsolation() error = %v", err)
	}
	workspace, _ := isolation.Workspace(0)
	if err := os.WriteFile(
		filepath.Join(workspace.ReviewsDir, "issue_001.md"),
		[]byte("status: resolved\n"),
		0o600,
	); err != nil {
		t.Fatalf("resolve issue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Root, "fix.go"), []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}
	hookPath := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write rejecting hook: %v", err)
	}
	statusBefore := mustRunGit(t, repo, "status", "--porcelain=v1", "-z")
	headBefore := strings.TrimSpace(string(mustRunGit(t, repo, "rev-parse", "HEAD")))

	err = isolation.Apply(context.Background(), 0, true, "fix: batch a")
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		t.Fatalf("Apply(auto commit) error = %v, want wrapped Git exit error", err)
	}
	if statusAfter := mustRunGit(
		t,
		repo,
		"status",
		"--porcelain=v1",
		"-z",
	); !bytes.Equal(statusAfter, statusBefore) {
		t.Fatalf("source status after rejected commit = %q, want original %q", statusAfter, statusBefore)
	}
	if headAfter := strings.TrimSpace(string(mustRunGit(t, repo, "rev-parse", "HEAD"))); headAfter != headBefore {
		t.Fatalf("source HEAD after rejected commit = %q, want %q", headAfter, headBefore)
	}
	if body, readErr := os.ReadFile(issuePath); readErr != nil || string(body) != "status: pending\n" {
		t.Fatalf("source issue after rejected commit = %q, error = %v", body, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "fix.go")); !os.IsNotExist(statErr) {
		t.Fatalf("source fix remains after rejected commit: %v", statErr)
	}
}

func TestReviewIsolationRestoresSourceWhenAutoCommitStagingFails(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}

	isolation, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a"},
	)
	if err != nil {
		t.Fatalf("NewReviewIsolation() error = %v", err)
	}
	workspace, _ := isolation.Workspace(0)
	if err := os.WriteFile(filepath.Join(workspace.Root, "fix.go"), []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "index.lock"), []byte("locked\n"), 0o600); err != nil {
		t.Fatalf("lock source index: %v", err)
	}
	statusBefore := mustRunGit(t, repo, "status", "--porcelain=v1", "-z")

	err = isolation.Apply(context.Background(), 0, true, "fix: batch a")
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		t.Fatalf("Apply(auto commit) error = %v, want wrapped Git exit error", err)
	}
	if statusAfter := mustRunGit(
		t,
		repo,
		"status",
		"--porcelain=v1",
		"-z",
	); !bytes.Equal(statusAfter, statusBefore) {
		t.Fatalf("source status after staging failure = %q, want original %q", statusAfter, statusBefore)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "fix.go")); !os.IsNotExist(statErr) {
		t.Fatalf("source fix remains after staging failure: %v", statErr)
	}
}

func TestReviewIsolationRejectsUncommittedSourceCode(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("dirty source\n"), 0o600); err != nil {
		t.Fatalf("dirty source: %v", err)
	}

	_, err := NewReviewIsolation(
		context.Background(),
		repo,
		reviewsDir,
		filepath.Dir(reviewsDir),
		filepath.Join(t.TempDir(), "worktrees"),
		[]string{"batch-a"},
	)
	if err == nil || !strings.Contains(err.Error(), "changes outside") {
		t.Fatalf("NewReviewIsolation() error = %v, want dirty-source rejection", err)
	}
}

func mustRunGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	output, err := runGit(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return output
}
