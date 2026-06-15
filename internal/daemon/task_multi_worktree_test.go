package daemon

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanTaskMultiWorktreePath(t *testing.T) {
	t.Run("Should return deterministic parent-run and index scoped paths", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/home/dev/project",
			ParentRunID:   "task-multi-abcdef123456",
			Slug:          "task_01",
			Index:         0,
		}
		first, err := planTaskMultiWorktreePath(root, spec)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath() error = %v", err)
		}
		second, err := planTaskMultiWorktreePath(root, spec)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath() repeat error = %v", err)
		}
		if first != second {
			t.Fatalf("path not deterministic: %q != %q", first, second)
		}
		if !strings.HasPrefix(first, root) {
			t.Fatalf("path %q is not under worktrees root %q", first, root)
		}
		other := spec
		other.Index = 1
		indexShifted, err := planTaskMultiWorktreePath(root, other)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath(index=1) error = %v", err)
		}
		if indexShifted == first {
			t.Fatal("different child index must produce a different path")
		}
		differentParent := spec
		differentParent.ParentRunID = "task-multi-zzzzzz999999"
		parentShifted, err := planTaskMultiWorktreePath(root, differentParent)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath(parent) error = %v", err)
		}
		if parentShifted == first {
			t.Fatal("different parent run id must produce a different path")
		}
	})

	t.Run("Should isolate worktrees from different workspace roots", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		base := taskMultiWorktreeSpec{ParentRunID: "parent-1", Slug: "task_01", Index: 0}
		base.WorkspaceRoot = "/home/dev/project-a"
		pathA, err := planTaskMultiWorktreePath(root, base)
		if err != nil {
			t.Fatalf("path A error = %v", err)
		}
		base.WorkspaceRoot = "/home/dev/project-b"
		pathB, err := planTaskMultiWorktreePath(root, base)
		if err != nil {
			t.Fatalf("path B error = %v", err)
		}
		if pathA == pathB {
			t.Fatal("different workspace roots must not share a worktree parent directory")
		}
	})

	t.Run("Should isolate worktrees for parent runs that share a truncated prefix", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		base := taskMultiWorktreeSpec{WorkspaceRoot: "/home/dev/project", Slug: "task_01", Index: 0}
		// Generated run ids share a long "task-multi-<date>-..." prefix; the first
		// 12 characters are identical here, so only the full-id digest disambiguates.
		base.ParentRunID = "task-multi-20260101-000000-000000001-aaaa"
		pathA, err := planTaskMultiWorktreePath(root, base)
		if err != nil {
			t.Fatalf("path A error = %v", err)
		}
		base.ParentRunID = "task-multi-20260101-000000-000000002-bbbb"
		pathB, err := planTaskMultiWorktreePath(root, base)
		if err != nil {
			t.Fatalf("path B error = %v", err)
		}
		if pathA == pathB {
			t.Fatal("parent runs sharing a truncated id prefix must not share a worktree path")
		}
	})

	t.Run("Should sanitize slugs containing spaces and path separators", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/home/dev/project",
			ParentRunID:   "parent-1",
			Slug:          "Fix Bug ../../etc/passwd",
			Index:         2,
		}
		got, err := planTaskMultiWorktreePath(root, spec)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath() error = %v", err)
		}
		leaf := filepath.Base(got)
		if strings.ContainsAny(leaf, " /\\") {
			t.Fatalf("leaf %q still contains unsafe characters", leaf)
		}
		if strings.Contains(leaf, "..") {
			t.Fatalf("leaf %q still allows path traversal", leaf)
		}
		if !strings.HasPrefix(leaf, "02-") {
			t.Fatalf("leaf %q missing zero-padded index prefix", leaf)
		}
		if dir := filepath.Dir(got); !strings.HasPrefix(dir, root) {
			t.Fatalf("sanitized path %q escaped worktrees root %q", got, root)
		}
	})

	t.Run("Should keep paths short enough for local daemon constraints", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/home/dev/project",
			ParentRunID:   strings.Repeat("p", 64),
			Slug:          strings.Repeat("a", 120),
			Index:         3,
		}
		got, err := planTaskMultiWorktreePath(root, spec)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath() error = %v", err)
		}
		rel, err := filepath.Rel(root, got)
		if err != nil {
			t.Fatalf("filepath.Rel() error = %v", err)
		}
		if len(rel) > 80 {
			t.Fatalf("relative worktree path too long (%d): %q", len(rel), rel)
		}
		leaf := filepath.Base(got)
		if maxLeaf := taskMultiWorktreeIndexPadWidth + 1 + taskMultiWorktreeSlugMaxLen; len(leaf) > maxLeaf {
			t.Fatalf("leaf %q length %d exceeds %d", leaf, len(leaf), maxLeaf)
		}
		segments := strings.Split(rel, string(os.PathSeparator))
		if len(segments) < 3 {
			t.Fatalf("expected hash/parent/leaf segments, got %v", segments)
		}
		if len(segments[0]) != taskMultiWorktreeHashLen {
			t.Fatalf(
				"workspace hash segment %q length %d, want %d",
				segments[0],
				len(segments[0]),
				taskMultiWorktreeHashLen,
			)
		}
		maxParent := taskMultiWorktreeParentShortLen + 1 + taskMultiWorktreeParentHashLen
		if len(segments[1]) > maxParent {
			t.Fatalf("parent segment %q exceeds %d", segments[1], maxParent)
		}
	})

	t.Run("Should reject invalid inputs", func(t *testing.T) {
		t.Parallel()
		valid := taskMultiWorktreeSpec{
			WorkspaceRoot: "/home/dev/project",
			ParentRunID:   "parent-1",
			Slug:          "task_01",
			Index:         0,
		}
		cases := []struct {
			name string
			root string
			spec taskMultiWorktreeSpec
		}{
			{name: "empty worktrees root", root: "  ", spec: valid},
			{
				name: "empty workspace root",
				root: t.TempDir(),
				spec: func() taskMultiWorktreeSpec { s := valid; s.WorkspaceRoot = " "; return s }(),
			},
			{
				name: "blank parent run id",
				root: t.TempDir(),
				spec: func() taskMultiWorktreeSpec { s := valid; s.ParentRunID = "***"; return s }(),
			},
			{
				name: "blank slug",
				root: t.TempDir(),
				spec: func() taskMultiWorktreeSpec { s := valid; s.Slug = "   "; return s }(),
			},
			{
				name: "negative index",
				root: t.TempDir(),
				spec: func() taskMultiWorktreeSpec { s := valid; s.Index = -1; return s }(),
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if _, err := planTaskMultiWorktreePath(tc.root, tc.spec); err == nil {
					t.Fatalf("planTaskMultiWorktreePath(%s) error = nil, want error", tc.name)
				}
			})
		}
	})
}

func TestSanitizeTaskMultiWorktreeSegment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		value  string
		maxLen int
		want   string
	}{
		{name: "Should lowercase and preserve underscores", value: "Task_01", maxLen: 40, want: "task_01"},
		{name: "Should collapse spaces to a single dash", value: "fix   bug", maxLen: 40, want: "fix-bug"},
		{name: "Should map path separators to a dash", value: "a/b\\c", maxLen: 40, want: "a-b-c"},
		{name: "Should strip traversal dots", value: "../etc", maxLen: 40, want: "etc"},
		{name: "Should trim leading and trailing dashes", value: "  -hello-  ", maxLen: 40, want: "hello"},
		{name: "Should cap to the max length", value: strings.Repeat("a", 50), maxLen: 8, want: "aaaaaaaa"},
		{name: "Should be empty when only separators", value: "///", maxLen: 40, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeTaskMultiWorktreeSegment(tc.value, tc.maxLen); got != tc.want {
				t.Fatalf("sanitizeTaskMultiWorktreeSegment(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestTaskMultiWorktreeAllocatorResolveBaseUnit(t *testing.T) {
	t.Run("Should resolve branch and commit once", func(t *testing.T) {
		t.Parallel()
		var calls [][]string
		allocator := &taskMultiWorktreeAllocator{
			worktreesRoot: t.TempDir(),
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/repo" {
					t.Fatalf("dir = %q, want /repo", dir)
				}
				calls = append(calls, append([]string(nil), args...))
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return "feature/parallel\n", nil
				case "rev-parse HEAD":
					return "abc123\n", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		base, err := allocator.ResolveBase(context.Background(), "/repo")
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		if base.Branch != "feature/parallel" || base.Commit != "abc123" {
			t.Fatalf("base = %#v, want feature/parallel @ abc123", base)
		}
		if len(calls) != 2 {
			t.Fatalf("expected exactly 2 read commands, got %v", calls)
		}
	})

	t.Run("Should reject a detached parent checkout", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				if strings.Join(args, " ") == "rev-parse --abbrev-ref HEAD" {
					return taskMultiWorktreeHeadRef, nil
				}
				t.Fatalf("commit read must not run for detached HEAD: %v", args)
				return "", nil
			},
		}
		_, err := allocator.ResolveBase(context.Background(), "/repo")
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("ResolveBase() error = %v, want detached HEAD validation", err)
		}
	})

	t.Run("Should wrap branch and commit read failures", func(t *testing.T) {
		t.Parallel()
		branchErr := errors.New("branch read failed")
		commitErr := errors.New("commit read failed")
		cases := []struct {
			name    string
			run     taskMultiWorktreeGitRunner
			wantErr error
		}{
			{
				name: "branch read failure",
				run: func(context.Context, string, ...string) (string, error) {
					return "", branchErr
				},
				wantErr: branchErr,
			},
			{
				name: "commit read failure",
				run: func(_ context.Context, _ string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --abbrev-ref HEAD" {
						return "main", nil
					}
					return "", commitErr
				},
				wantErr: commitErr,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				allocator := &taskMultiWorktreeAllocator{run: tc.run}
				_, err := allocator.ResolveBase(context.Background(), "/repo")
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ResolveBase() error = %v, want wrapped %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("Should reject an unresolvable empty HEAD commit", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				if strings.Join(args, " ") == "rev-parse --abbrev-ref HEAD" {
					return "main", nil
				}
				return "  \n", nil
			},
		}
		_, err := allocator.ResolveBase(context.Background(), "/repo")
		if err == nil || !strings.Contains(err.Error(), "no resolvable HEAD commit") {
			t.Fatalf("ResolveBase() error = %v, want empty commit validation", err)
		}
	})

	t.Run("Should validate runner and workspace", func(t *testing.T) {
		t.Parallel()
		if _, err := (*taskMultiWorktreeAllocator)(nil).ResolveBase(context.Background(), "/repo"); err == nil {
			t.Fatal("nil allocator ResolveBase() error = nil, want runner validation")
		}
		empty := &taskMultiWorktreeAllocator{run: func(context.Context, string, ...string) (string, error) {
			t.Fatal("git must not run for empty workspace")
			return "", nil
		}}
		if _, err := empty.ResolveBase(context.Background(), "  "); err == nil ||
			!strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("empty workspace ResolveBase() error = %v, want workspace validation", err)
		}
	})
}

func TestTaskMultiWorktreeAllocatorAllocateUnit(t *testing.T) {
	t.Run("Should require a base commit", func(t *testing.T) {
		t.Parallel()
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		allocator.run = func(context.Context, string, ...string) (string, error) {
			t.Fatal("git must not run without a base commit")
			return "", nil
		}
		spec := taskMultiWorktreeSpec{WorkspaceRoot: "/repo", ParentRunID: "p1", Slug: "task_01"}
		if _, err := allocator.Allocate(context.Background(), spec); err == nil ||
			!strings.Contains(err.Error(), "base commit is required") {
			t.Fatalf("Allocate() error = %v, want base commit validation", err)
		}
	})

	t.Run("Should report an existing target path as a collision", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/repo",
			ParentRunID:   "p1",
			Slug:          "task_01",
			Index:         0,
			Base:          taskMultiWorktreeBase{Branch: "main", Commit: "abc123"},
		}
		planned, err := planTaskMultiWorktreePath(root, spec)
		if err != nil {
			t.Fatalf("planTaskMultiWorktreePath() error = %v", err)
		}
		if err := os.MkdirAll(planned, 0o750); err != nil {
			t.Fatalf("seed existing target: %v", err)
		}
		allocator := newTaskMultiWorktreeAllocator(root)
		allocator.run = func(context.Context, string, ...string) (string, error) {
			t.Fatal("git worktree add must not run when target exists")
			return "", nil
		}
		if _, err := allocator.Allocate(context.Background(), spec); err == nil ||
			!strings.Contains(err.Error(), "already exists") {
			t.Fatalf("Allocate() error = %v, want collision error", err)
		}
	})

	t.Run("Should wrap git worktree add failures", func(t *testing.T) {
		t.Parallel()
		addErr := errors.New("worktree add failed")
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		allocator.run = func(_ context.Context, _ string, args ...string) (string, error) {
			if strings.Join(args, " ") == "worktree add --detach" {
				t.Fatalf("unexpected worktree args without path and commit: %v", args)
			}
			return "", addErr
		}
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/repo",
			ParentRunID:   "p1",
			Slug:          "task_01",
			Index:         0,
			Base:          taskMultiWorktreeBase{Branch: "main", Commit: "abc123"},
		}
		_, err := allocator.Allocate(context.Background(), spec)
		if !errors.Is(err, addErr) {
			t.Fatalf("Allocate() error = %v, want wrapped %v", err, addErr)
		}
	})

	t.Run("Should propagate path planning errors before touching git", func(t *testing.T) {
		t.Parallel()
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		allocator.run = func(context.Context, string, ...string) (string, error) {
			t.Fatal("git must not run when path planning fails")
			return "", nil
		}
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: "/repo",
			ParentRunID:   "p1",
			Slug:          "   ",
			Base:          taskMultiWorktreeBase{Commit: "abc123"},
		}
		if _, err := allocator.Allocate(context.Background(), spec); err == nil {
			t.Fatalf("Allocate() error = nil, want path planning error")
		}
	})
}

func TestTaskMultiWorktreeAllocatorRealRepo(t *testing.T) {
	t.Run("Should resolve the current branch and HEAD", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		wantCommit := runGitOutput(t, repo, "rev-parse", "HEAD")
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		base, err := allocator.ResolveBase(context.Background(), repo)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		if base.Branch != "main" {
			t.Fatalf("base.Branch = %q, want main", base.Branch)
		}
		if base.Commit != wantCommit {
			t.Fatalf("base.Commit = %q, want %q", base.Commit, wantCommit)
		}
	})

	t.Run("Should reject a detached parent checkout", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		runGitOutput(t, repo, "checkout", "--detach")
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		_, err := allocator.ResolveBase(context.Background(), repo)
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("ResolveBase() error = %v, want detached HEAD validation", err)
		}
	})

	t.Run("Should create a detached worktree at the resolved commit", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		root := t.TempDir()
		allocator := newTaskMultiWorktreeAllocator(root)
		base, err := allocator.ResolveBase(context.Background(), repo)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		spec := taskMultiWorktreeSpec{
			WorkspaceRoot: repo,
			ParentRunID:   "task-multi-realrepo01",
			Slug:          "task_01",
			Index:         0,
			Base:          base,
		}
		alloc, err := allocator.Allocate(context.Background(), spec)
		if err != nil {
			t.Fatalf("Allocate() error = %v", err)
		}
		if !strings.HasPrefix(alloc.Path, root) {
			t.Fatalf("worktree path %q is not under root %q", alloc.Path, root)
		}
		if alloc.BaseBranch != "main" || alloc.BaseCommit != base.Commit {
			t.Fatalf("allocation metadata = %#v, want main @ %s", alloc, base.Commit)
		}
		if alloc.WorktreeStatus != taskMultiWorktreeStatusPreserved {
			t.Fatalf("WorktreeStatus = %q, want %q", alloc.WorktreeStatus, taskMultiWorktreeStatusPreserved)
		}
		if gotHead := runGitOutput(t, alloc.Path, "rev-parse", "HEAD"); gotHead != base.Commit {
			t.Fatalf("worktree HEAD = %q, want resolved base commit %q", gotHead, base.Commit)
		}
		if gotRef := runGitOutput(
			t,
			alloc.Path,
			"rev-parse",
			"--abbrev-ref",
			"HEAD",
		); gotRef != taskMultiWorktreeHeadRef {
			t.Fatalf("worktree ref = %q, want detached %q", gotRef, taskMultiWorktreeHeadRef)
		}
		if _, err := allocator.Allocate(context.Background(), spec); err == nil ||
			!strings.Contains(err.Error(), "already exists") {
			t.Fatalf("second Allocate() error = %v, want collision error", err)
		}
	})
}

func TestEnsureTaskMultiWorktreeTargetFree(t *testing.T) {
	t.Run("Should allow a missing target path", func(t *testing.T) {
		t.Parallel()
		if err := ensureTaskMultiWorktreeTargetFree(filepath.Join(t.TempDir(), "missing")); err != nil {
			t.Fatalf("ensureTaskMultiWorktreeTargetFree(missing) error = %v, want nil", err)
		}
	})

	t.Run("Should reject an existing target path", func(t *testing.T) {
		t.Parallel()
		existing := filepath.Join(t.TempDir(), "existing")
		if err := os.MkdirAll(existing, 0o750); err != nil {
			t.Fatalf("seed existing target: %v", err)
		}
		if err := ensureTaskMultiWorktreeTargetFree(existing); err == nil ||
			!strings.Contains(err.Error(), "already exists") {
			t.Fatalf("ensureTaskMultiWorktreeTargetFree(existing) error = %v, want collision", err)
		}
	})

	t.Run("Should wrap stat failures other than not-exist", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
			t.Fatalf("seed file: %v", err)
		}
		// Stat on a path whose parent component is a file yields ENOTDIR, which is
		// not os.ErrNotExist and must surface as a wrapped stat error.
		err := ensureTaskMultiWorktreeTargetFree(filepath.Join(file, "child"))
		if err == nil || !strings.Contains(err.Error(), "stat worktree target") {
			t.Fatalf("ensureTaskMultiWorktreeTargetFree(under file) error = %v, want wrapped stat error", err)
		}
	})
}

func TestRunTaskMultiWorktreeGitCommand(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	out, err := runTaskMultiWorktreeGitCommand(context.Background(), t.TempDir(), "version")
	if err != nil {
		t.Fatalf("runTaskMultiWorktreeGitCommand(version) error = %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Fatalf("runTaskMultiWorktreeGitCommand(version) = %q, want git version", out)
	}
	if _, err := runTaskMultiWorktreeGitCommand(
		context.Background(),
		t.TempDir(),
		"not-a-real-git-command",
	); err == nil {
		t.Fatal("runTaskMultiWorktreeGitCommand(invalid) error = nil, want error")
	}
}

// initTaskMultiWorktreeRepo prepares a temporary git repository on branch main
// with a single commit so worktree allocation has a resolvable named branch and
// HEAD commit.
func initTaskMultiWorktreeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	runGitOutput(t, dir, "init", "-q", "-b", "main")
	runGitOutput(t, dir, "config", "user.email", "worktree@example.com")
	runGitOutput(t, dir, "config", "user.name", "Worktree Tester")
	runGitOutput(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# initial\n"), 0o600); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	runGitOutput(t, dir, "add", "README.md")
	runGitOutput(t, dir, "commit", "-q", "-m", "initial")
	return dir
}
