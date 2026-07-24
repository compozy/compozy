// Suite: isolated review integration
// Invariant: review integration changes only batch-owned source state.
// Boundary IN: real Git worktrees, indexes, commits, and rollback behavior.
// Boundary OUT: review workflow orchestration outside the worktree package.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

func TestRunGitMergeFileMergesAndDetectsConflicts(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	dir := t.TempDir()
	base := "line1\nline2\nline3\n"
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}
	basePath := write("base", base)

	t.Run("Should merge non-overlapping edits to the same file", func(t *testing.T) {
		ours := write("ours-clean", "line1-source\nline2\nline3\n")
		theirs := write("theirs-clean", "line1\nline2\nline3-review\n")
		conflicted, err := runGitMergeFile(context.Background(), dir, ours, basePath, theirs)
		if err != nil {
			t.Fatalf("runGitMergeFile() error = %v", err)
		}
		if conflicted {
			t.Fatal("non-overlapping edits reported a conflict")
		}
		merged, readErr := os.ReadFile(ours)
		if readErr != nil || string(merged) != "line1-source\nline2\nline3-review\n" {
			t.Fatalf("merged = %q, error = %v, want both edits", merged, readErr)
		}
	})

	t.Run("Should report a conflict for overlapping edits", func(t *testing.T) {
		ours := write("ours-conflict", "line1-source\nline2\nline3\n")
		theirs := write("theirs-conflict", "line1-review\nline2\nline3\n")
		conflicted, err := runGitMergeFile(context.Background(), dir, ours, basePath, theirs)
		if err != nil {
			t.Fatalf("runGitMergeFile() error = %v", err)
		}
		if !conflicted {
			t.Fatal("overlapping edits did not report a conflict")
		}
	})
}

func TestReviewIsolationMergesSharedSecondaryFileAcrossBatches(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	// A secondary file both batches edit even though their primary files differ —
	// exactly the collision the ticket describes.
	routes := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	if err := os.WriteFile(filepath.Join(repo, "routes.ts"), []byte(routes), 0o600); err != nil {
		t.Fatalf("write shared secondary baseline: %v", err)
	}
	mustRunGit(t, repo, "add", "routes.ts")
	mustRunGit(t, repo, "commit", "-q", "-m", "add shared secondary file")
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

	// batch-a fixes its primary file and edits line 1 of the shared secondary.
	if err := os.WriteFile(filepath.Join(first.Root, "a.ts"), []byte("a\n"), 0o600); err != nil {
		t.Fatalf("write batch-a primary: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(first.Root, "routes.ts"),
		[]byte(strings.Replace(routes, "l1\n", "l1-a\n", 1)),
		0o600,
	); err != nil {
		t.Fatalf("write batch-a shared edit: %v", err)
	}
	// batch-b fixes its primary file and edits line 10 of the shared secondary.
	if err := os.WriteFile(filepath.Join(second.Root, "b.ts"), []byte("b\n"), 0o600); err != nil {
		t.Fatalf("write batch-b primary: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(second.Root, "routes.ts"),
		[]byte(strings.Replace(routes, "l10\n", "l10-b\n", 1)),
		0o600,
	); err != nil {
		t.Fatalf("write batch-b shared edit: %v", err)
	}

	// Both batches integrate: batch-b's non-overlapping edit to the already-changed
	// shared file merges instead of colliding on "patch does not apply".
	if err := isolation.Apply(context.Background(), 0, false, "fix: batch a"); err != nil {
		t.Fatalf("Apply(batch-a) error = %v", err)
	}
	if err := isolation.Apply(context.Background(), 1, false, "fix: batch b"); err != nil {
		t.Fatalf("Apply(batch-b) error = %v", err)
	}

	wantRoutes := strings.Replace(strings.Replace(routes, "l1\n", "l1-a\n", 1), "l10\n", "l10-b\n", 1)
	if body, readErr := os.ReadFile(filepath.Join(repo, "routes.ts")); readErr != nil || string(body) != wantRoutes {
		t.Fatalf("merged shared secondary = %q, error = %v, want %q", body, readErr, wantRoutes)
	}
	for path, want := range map[string]string{
		filepath.Join(repo, "a.ts"): "a\n",
		filepath.Join(repo, "b.ts"): "b\n",
	} {
		if body, readErr := os.ReadFile(path); readErr != nil || string(body) != want {
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
	// batch-a and batch-b rewrite the same file to different content: a true
	// overlapping-hunk conflict, which parks the second batch (its worktree is
	// preserved for triage) instead of silently dropping it.
	err = isolation.Apply(context.Background(), 1, false, "fix: second")
	if err == nil || !errors.Is(err, ErrOverlappingReviewEdits) {
		t.Fatalf("Apply(second) error = %v, want ErrOverlappingReviewEdits", err)
	}
	if !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("Apply(second) error = %v, want the conflicting path README.md", err)
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
	workspace, err := isolation.Workspace(0)
	if err != nil {
		t.Fatalf("Workspace(0) error = %v", err)
	}
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

func TestReviewIsolationReportsCommittedBatchWhenIndexRefreshFails(t *testing.T) {
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
	first, err := isolation.Workspace(0)
	if err != nil {
		t.Fatalf("Workspace(0) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(first.Root, "first.go"), []byte("package first\n"), 0o600); err != nil {
		t.Fatalf("write first fix: %v", err)
	}

	captureSourceIndex := isolation.captureSourceIndex
	refreshes := 0
	refreshHadDeadline := false
	refreshWasCanceled := false
	applyCtx, cancelApply := context.WithCancel(context.Background())
	isolation.captureSourceIndex = func(ctx context.Context, root string) (gitIndexBackup, error) {
		refreshes++
		if refreshes == 1 {
			cancelApply()
			_, refreshHadDeadline = ctx.Deadline()
			refreshWasCanceled = ctx.Err() != nil
			return gitIndexBackup{}, errors.New("forced post-commit index refresh failure")
		}
		return captureSourceIndex(ctx, root)
	}

	if err := isolation.Apply(applyCtx, 0, true, "fix: batch a"); err != nil {
		t.Fatalf("Apply(first) error after commit = %v", err)
	}
	if !refreshHadDeadline || refreshWasCanceled {
		t.Fatalf(
			"post-commit refresh context: deadline=%t canceled=%t, want bounded and active",
			refreshHadDeadline,
			refreshWasCanceled,
		)
	}
	if got := commitSubjectCount(t, repo, "fix: batch a"); got != 1 {
		t.Fatalf("first batch commit count = %d, want 1", got)
	}
	err = isolation.Apply(context.Background(), 0, true, "fix: batch a")
	if err == nil || !strings.Contains(err.Error(), "source paths changed since review isolation began: first.go") {
		t.Fatalf("Apply(retry) error after reconciliation = %v, want committed-path rejection", err)
	}
	if got := commitSubjectCount(t, repo, "fix: batch a"); got != 1 {
		t.Fatalf("first batch commit count after reconciliation = %d, want 1", got)
	}
}

func TestReviewIsolationAutoCommitPreservesPreStagedWorkflowArtifact(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	artifactRel := filepath.ToSlash(filepath.Join(".compozy", "tasks", "demo", "_techspec.md"))
	artifactPath := filepath.Join(repo, filepath.FromSlash(artifactRel))
	if err := os.WriteFile(artifactPath, []byte("# Staged TechSpec\n"), 0o600); err != nil {
		t.Fatalf("write staged artifact: %v", err)
	}
	mustRunGit(t, repo, "add", "--", artifactRel)

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
	workspace, err := isolation.Workspace(0)
	if err != nil {
		t.Fatalf("Workspace(0) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Root, "fix.go"), []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}

	if err := isolation.Apply(context.Background(), 0, true, "fix: batch a"); err != nil {
		t.Fatalf("Apply(auto commit) error = %v", err)
	}
	if staged := strings.TrimSpace(
		string(mustRunGit(t, repo, "diff", "--cached", "--name-only")),
	); staged != artifactRel {
		t.Fatalf("staged paths after commit = %q, want %q", staged, artifactRel)
	}
	if body := string(mustRunGit(t, repo, "show", ":"+artifactRel)); body != "# Staged TechSpec\n" {
		t.Fatalf("staged artifact content = %q, want original content", body)
	}
}

func TestReviewIsolationValidationFailureRestoresExactIndex(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	reviewsDir := filepath.Join(repo, ".compozy", "tasks", "demo", "reviews-001")
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	artifactRel := filepath.ToSlash(filepath.Join(".compozy", "tasks", "demo", "_techspec.md"))
	artifactPath := filepath.Join(repo, filepath.FromSlash(artifactRel))
	if err := os.WriteFile(artifactPath, []byte("# Staged TechSpec\n"), 0o600); err != nil {
		t.Fatalf("write staged artifact: %v", err)
	}
	mustRunGit(t, repo, "add", "--", artifactRel)

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
	workspace, err := isolation.Workspace(0)
	if err != nil {
		t.Fatalf("Workspace(0) error = %v", err)
	}
	fixPath := filepath.Join(workspace.Root, "fix.go")
	if err := os.WriteFile(fixPath, []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}
	sourceRoot, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("resolve source root: %v", err)
	}
	hook := fmt.Sprintf(
		"#!/bin/sh\nif [ \"$(pwd -P)\" != %q ]; then\n  exit 0\nfi\nprintf 'package changed\\n' > %q\nunset GIT_INDEX_FILE GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR\ngit -c core.hooksPath=/dev/null -C %q add -A\n",
		sourceRoot,
		fixPath,
		workspace.Root,
	)
	if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-index-change"), []byte(hook), 0o700); err != nil {
		t.Fatalf("write post-index-change hook: %v", err)
	}
	indexBackup, err := captureGitIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("captureGitIndex(baseline) error = %v", err)
	}

	err = isolation.Apply(context.Background(), 0, true, "fix: batch a")
	if err == nil || !strings.Contains(err.Error(), "staged source entries differ from isolated review results") {
		t.Fatalf("Apply(auto commit) error = %v, want staged-entry validation failure", err)
	}
	indexAfter, readErr := os.ReadFile(indexBackup.path)
	if readErr != nil {
		t.Fatalf("read restored source index: %v", readErr)
	}
	if !bytes.Equal(indexAfter, indexBackup.content) {
		t.Fatal("source index after validation failure differs from byte-identical baseline")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "fix.go")); !os.IsNotExist(statErr) {
		t.Fatalf("source fix remains after validation failure: %v; apply error: %v", statErr, err)
	}
	if body := string(mustRunGit(t, repo, "show", ":"+artifactRel)); body != "# Staged TechSpec\n" {
		t.Fatalf("staged artifact content = %q, want original content", body)
	}
}

func TestReviewIsolationMergesNonConflictingSourceDrift(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	baseline := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte(baseline), 0o600); err != nil {
		t.Fatalf("write source baseline: %v", err)
	}
	mustRunGit(t, repo, "add", "README.md")
	mustRunGit(t, repo, "commit", "-q", "-m", "expand baseline")
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
	batchResult := strings.Replace(baseline, "one\n", "batch\n", 1)
	if err := os.WriteFile(filepath.Join(workspace.Root, "README.md"), []byte(batchResult), 0o600); err != nil {
		t.Fatalf("write isolated result: %v", err)
	}
	externalResult := strings.Replace(baseline, "ten\n", "external\n", 1)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte(externalResult), 0o600); err != nil {
		t.Fatalf("write external source edit: %v", err)
	}
	headBefore := strings.TrimSpace(string(mustRunGit(t, repo, "rev-parse", "HEAD")))

	// The batch changed line 1 while the source drifted on line 10: non-overlapping
	// edits to a shared file merge cleanly and commit both, instead of parking.
	if err := isolation.Apply(context.Background(), 0, true, "fix: batch a"); err != nil {
		t.Fatalf("Apply(auto commit) error = %v, want a clean 3-way merge", err)
	}
	merged := strings.Replace(externalResult, "one\n", "batch\n", 1)
	if headAfter := strings.TrimSpace(string(mustRunGit(t, repo, "rev-parse", "HEAD"))); headAfter == headBefore {
		t.Fatalf("source HEAD after merge = %q, want a new commit", headAfter)
	}
	if body, readErr := os.ReadFile(filepath.Join(repo, "README.md")); readErr != nil || string(body) != merged {
		t.Fatalf("source README after merge = %q, error = %v, want %q", body, readErr, merged)
	}
	if committed := string(mustRunGit(t, repo, "show", "HEAD:README.md")); committed != merged {
		t.Fatalf("committed README after merge = %q, want %q", committed, merged)
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

func TestRollbackReviewApplyPreservesConcurrentStaging(t *testing.T) {
	t.Parallel()
	requireScopeGit(t)
	repo := initScopeGitRepo(t)
	concurrentPath := filepath.Join(repo, "concurrent.txt")
	if err := os.WriteFile(concurrentPath, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("write concurrent baseline: %v", err)
	}
	mustRunGit(t, repo, "add", "concurrent.txt")
	mustRunGit(t, repo, "commit", "-q", "-m", "track concurrent path")
	indexBackup, err := captureGitIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("captureGitIndex(baseline) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fix.go"), []byte("package fix\n"), 0o600); err != nil {
		t.Fatalf("write fix: %v", err)
	}
	mustRunGit(t, repo, "add", "fix.go")
	patch := mustRunGit(t, repo, "diff", "--cached", "--binary", "--full-index", "--no-renames", "HEAD")
	stagedIndex, err := captureGitIndex(context.Background(), repo)
	if err != nil {
		t.Fatalf("captureGitIndex(staged review) error = %v", err)
	}
	if err := os.WriteFile(concurrentPath, []byte("concurrent staged\n"), 0o600); err != nil {
		t.Fatalf("write concurrent change: %v", err)
	}
	mustRunGit(t, repo, "add", "concurrent.txt")

	err = rollbackReviewApply(context.Background(), repo, patch, indexBackup, stagedIndex, nil)
	if err == nil || !strings.Contains(err.Error(), "preserved concurrent index state") {
		t.Fatalf("rollbackReviewApply() error = %v, want index conflict", err)
	}
	if staged := strings.TrimSpace(
		string(mustRunGit(t, repo, "diff", "--cached", "--name-only")),
	); staged != "concurrent.txt\nfix.go" {
		t.Fatalf("staged paths after rollback conflict = %q, want concurrent.txt and fix.go", staged)
	}
	if stagedBody := string(mustRunGit(t, repo, "show", ":concurrent.txt")); stagedBody != "concurrent staged\n" {
		t.Fatalf("staged concurrent content = %q, want external staged content", stagedBody)
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

func commitSubjectCount(t *testing.T, dir string, subject string) int {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(mustRunGit(t, dir, "log", "--format=%s"))), "\n")
	count := 0
	for _, line := range lines {
		if line == subject {
			count++
		}
	}
	return count
}
