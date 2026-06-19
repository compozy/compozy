package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	runparallel "github.com/compozy/compozy/internal/core/run/parallel"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/core/workspace"
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

	t.Run("Should key per-task worktree leaves by task number", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		base := taskMultiWorktreeSpec{
			WorkspaceRoot: "/home/dev/project",
			ParentRunID:   "parallel-task-parent",
			Slug:          "alpha",
			Index:         0,
		}
		taskOne := base
		taskOne.TaskNumber = 1
		taskTwo := base
		taskTwo.TaskNumber = 2
		pathOne, err := planTaskMultiWorktreePath(root, taskOne)
		if err != nil {
			t.Fatalf("task one path error = %v", err)
		}
		pathTwo, err := planTaskMultiWorktreePath(root, taskTwo)
		if err != nil {
			t.Fatalf("task two path error = %v", err)
		}
		if pathOne == pathTwo {
			t.Fatal("different task numbers sharing wave index 0 must not collide")
		}
		if leaf := filepath.Base(pathOne); !strings.HasPrefix(leaf, "01-") {
			t.Fatalf("task one leaf = %q, want task-number prefix 01-", leaf)
		}
		if leaf := filepath.Base(pathTwo); !strings.HasPrefix(leaf, "02-") {
			t.Fatalf("task two leaf = %q, want task-number prefix 02-", leaf)
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
			{
				name: "negative task number",
				root: t.TempDir(),
				spec: func() taskMultiWorktreeSpec { s := valid; s.TaskNumber = -1; return s }(),
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

func TestTaskMultiWorktreeAllocatorCommitUnit(t *testing.T) {
	t.Run("Should no-op when the worktree is clean", func(t *testing.T) {
		t.Parallel()
		var calls []string
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/worktree" {
					t.Fatalf("dir = %q, want /worktree", dir)
				}
				call := strings.Join(args, " ")
				calls = append(calls, call)
				switch call {
				case "status --porcelain":
					return " \n", nil
				case "rev-parse HEAD":
					return "clean-head\n", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		head, err := allocator.Commit(context.Background(), "/worktree", "capture residual")
		if err != nil {
			t.Fatalf("Commit(clean) error = %v", err)
		}
		if head != "clean-head" {
			t.Fatalf("Commit(clean) head = %q, want clean-head", head)
		}
		if got, want := strings.Join(calls, "|"), "status --porcelain|rev-parse HEAD"; got != want {
			t.Fatalf("git calls = %s, want %s", got, want)
		}
	})

	t.Run("Should stage and commit residual changes when dirty", func(t *testing.T) {
		t.Parallel()
		var calls []string
		statusCalls := 0
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/worktree" {
					t.Fatalf("dir = %q, want /worktree", dir)
				}
				call := strings.Join(args, " ")
				calls = append(calls, call)
				switch call {
				case "status --porcelain":
					statusCalls++
					if statusCalls == 1 {
						return " M changed.txt\n?? new.txt\n", nil
					}
					return "M  changed.txt\nA  new.txt\n", nil
				case "add -A":
					return "", nil
				case "commit -m capture residual":
					return "", nil
				case "rev-parse HEAD":
					return "dirty-head\n", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		head, err := allocator.Commit(context.Background(), "/worktree", "capture residual")
		if err != nil {
			t.Fatalf("Commit(dirty) error = %v", err)
		}
		if head != "dirty-head" {
			t.Fatalf("Commit(dirty) head = %q, want dirty-head", head)
		}
		want := "status --porcelain|add -A|status --porcelain|commit -m capture residual|rev-parse HEAD"
		if got := strings.Join(calls, "|"); got != want {
			t.Fatalf("git calls = %s, want %s", got, want)
		}
	})
}

func TestTaskMultiWorktreeAllocatorSquashMergeUnit(t *testing.T) {
	t.Run("Should return a clean conflict set after a successful squash commit", func(t *testing.T) {
		t.Parallel()
		var calls []string
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/integration" {
					t.Fatalf("dir = %q, want /integration", dir)
				}
				call := strings.Join(args, " ")
				calls = append(calls, call)
				switch call {
				case "status --porcelain":
					return "", nil
				case "merge --squash -- worktree-ref":
					return "", nil
				case "commit --allow-empty -m task 01: add file":
					return "commit ok", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		conflicts, err := allocator.SquashMerge(
			context.Background(),
			"/integration",
			"worktree-ref",
			"task 01: add file",
		)
		if err != nil {
			t.Fatalf("SquashMerge(clean) error = %v", err)
		}
		if !conflicts.Clean || len(conflicts.Files) != 0 {
			t.Fatalf("conflicts = %#v, want clean", conflicts)
		}
		want := "status --porcelain|merge --squash -- worktree-ref|commit --allow-empty -m task 01: add file"
		if got := strings.Join(calls, "|"); got != want {
			t.Fatalf("git calls = %s, want %s", got, want)
		}
	})

	t.Run("Should return unmerged files when a squash merge conflicts", func(t *testing.T) {
		t.Parallel()
		mergeErr := errors.New("merge conflict")
		statusCalls := 0
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/integration" {
					t.Fatalf("dir = %q, want /integration", dir)
				}
				switch strings.Join(args, " ") {
				case "status --porcelain":
					statusCalls++
					if statusCalls == 1 {
						return "", nil
					}
					return "UU story.txt\nAA nested/name.txt\n", nil
				case "merge --squash -- worktree-ref":
					return "", mergeErr
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		conflicts, err := allocator.SquashMerge(
			context.Background(),
			"/integration",
			"worktree-ref",
			"task 02: overlap",
		)
		if err != nil {
			t.Fatalf("SquashMerge(conflict) error = %v, want conflict set", err)
		}
		if conflicts.Clean {
			t.Fatalf("conflicts.Clean = true, want false")
		}
		if got, want := strings.Join(conflicts.Files, ","), "nested/name.txt,story.txt"; got != want {
			t.Fatalf("conflict files = %q, want %q", got, want)
		}
	})
}

func TestTaskMultiWorktreeAllocatorLifecycleValidationUnit(t *testing.T) {
	t.Run("Should reject a missing lifecycle git runner", func(t *testing.T) {
		t.Parallel()
		if _, err := (*taskMultiWorktreeAllocator)(
			nil,
		).Commit(context.Background(), "/worktree", "message"); err == nil ||
			!strings.Contains(err.Error(), "git runner is required") {
			t.Fatalf("nil allocator Commit() error = %v, want runner validation", err)
		}
	})

	t.Run("Should validate required lifecycle values before touching git", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(context.Context, string, ...string) (string, error) {
				t.Fatal("git must not run for invalid lifecycle inputs")
				return "", nil
			},
		}
		if _, err := allocator.Commit(context.Background(), " ", "message"); err == nil ||
			!strings.Contains(err.Error(), "worktree path is required") {
			t.Fatalf("Commit(empty path) error = %v, want path validation", err)
		}
		if _, err := allocator.Commit(context.Background(), "/worktree", " "); err == nil ||
			!strings.Contains(err.Error(), "commit message is required") {
			t.Fatalf("Commit(empty message) error = %v, want message validation", err)
		}
		if err := allocator.CreateIntegrationBranch(
			context.Background(),
			" ",
			"/integration",
			"branch",
			"HEAD",
		); err == nil ||
			!strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("CreateIntegrationBranch(empty workspace) error = %v, want workspace validation", err)
		}
		if _, err := allocator.SquashMerge(context.Background(), "/integration", " ", "message"); err == nil ||
			!strings.Contains(err.Error(), "worktree ref is required") {
			t.Fatalf("SquashMerge(empty ref) error = %v, want ref validation", err)
		}
		if err := allocator.FastForward(context.Background(), "/repo", " ", "integration"); err == nil ||
			!strings.Contains(err.Error(), "target branch is required") {
			t.Fatalf("FastForward(empty target) error = %v, want target validation", err)
		}
		if err := allocator.DiscardIntegrationBranch(context.Background(), "/repo", "/integration", " "); err == nil ||
			!strings.Contains(err.Error(), "integration branch is required") {
			t.Fatalf("DiscardIntegrationBranch(empty branch) error = %v, want branch validation", err)
		}
		if err := allocator.Remove(context.Background(), "/repo", " "); err == nil ||
			!strings.Contains(err.Error(), "worktree path is required") {
			t.Fatalf("Remove(empty path) error = %v, want path validation", err)
		}
		if err := allocator.Prune(context.Background(), " "); err == nil ||
			!strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("Prune(empty workspace) error = %v, want workspace validation", err)
		}
	})
}

func TestTaskMultiWorktreeAllocatorLifecycleFailureUnit(t *testing.T) {
	t.Run("Should reject residual commits when the worktree has unmerged files", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				if strings.Join(args, " ") != "status --porcelain" {
					t.Fatalf("unexpected git args: %v", args)
				}
				return "UU story.txt\n", nil
			},
		}
		if _, err := allocator.Commit(context.Background(), "/worktree", "capture residual"); err == nil ||
			!strings.Contains(err.Error(), "unresolved merge conflicts: story.txt") {
			t.Fatalf("Commit(unmerged) error = %v, want unresolved conflict validation", err)
		}
	})

	t.Run("Should reject fast-forward from the wrong branch", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				if strings.Join(args, " ") != "rev-parse --abbrev-ref HEAD" {
					t.Fatalf("unexpected git args: %v", args)
				}
				return "feature\n", nil
			},
		}
		err := allocator.FastForward(context.Background(), "/repo", "main", "compozy/parallel")
		if err == nil || !strings.Contains(err.Error(), "want target branch") {
			t.Fatalf("FastForward(wrong branch) error = %v, want branch validation", err)
		}
	})

	t.Run("Should reject fast-forward from a dirty target branch", func(t *testing.T) {
		t.Parallel()
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return "main\n", nil
				case "status --porcelain":
					return " M dirty.txt\n", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		err := allocator.FastForward(context.Background(), "/repo", "main", "compozy/parallel")
		if err == nil || !strings.Contains(err.Error(), "must be clean before fast-forward") {
			t.Fatalf("FastForward(dirty) error = %v, want clean-tree validation", err)
		}
	})

	t.Run("Should reject non-fast-forward integration branches", func(t *testing.T) {
		t.Parallel()
		ancestorErr := errors.New("not ancestor")
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return "main\n", nil
				case "status --porcelain":
					return "", nil
				case "merge-base --is-ancestor main compozy/parallel":
					return "", ancestorErr
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}
		err := allocator.FastForward(context.Background(), "/repo", "main", "compozy/parallel")
		if !errors.Is(err, ancestorErr) {
			t.Fatalf("FastForward(non-ancestor) error = %v, want wrapped %v", err, ancestorErr)
		}
	})

	t.Run("Should delete an integration branch even when its worktree path is already gone", func(t *testing.T) {
		t.Parallel()
		missingPath := filepath.Join(t.TempDir(), "missing")
		var calls []string
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, dir string, args ...string) (string, error) {
				if dir != "/repo" {
					t.Fatalf("dir = %q, want /repo", dir)
				}
				calls = append(calls, strings.Join(args, " "))
				return "", nil
			},
		}
		if err := allocator.DiscardIntegrationBranch(
			context.Background(),
			"/repo",
			missingPath,
			"compozy/parallel",
		); err != nil {
			t.Fatalf("DiscardIntegrationBranch(missing path) error = %v", err)
		}
		if got, want := strings.Join(calls, "|"), "branch -D compozy/parallel"; got != want {
			t.Fatalf("git calls = %s, want %s", got, want)
		}
	})

	t.Run("Should no-op removing an already missing worktree", func(t *testing.T) {
		t.Parallel()
		missingPath := filepath.Join(t.TempDir(), "missing")
		allocator := &taskMultiWorktreeAllocator{
			run: func(context.Context, string, ...string) (string, error) {
				t.Fatal("git must not run when the worktree path is missing")
				return "", nil
			},
		}
		if err := allocator.Remove(context.Background(), "/repo", missingPath); err != nil {
			t.Fatalf("Remove(missing path) error = %v", err)
		}
	})
}

func TestTaskMultiWorktreeStatusParsing(t *testing.T) {
	t.Parallel()
	status := "UU story.txt\nM  clean.txt\nAA \"dir/file with spaces.txt\"\nR  old.txt -> new.txt\n"
	files := taskMultiWorktreeUnmergedFiles(status)
	if got, want := strings.Join(files, "|"), "dir/file with spaces.txt|story.txt"; got != want {
		t.Fatalf("unmerged files = %q, want %q", got, want)
	}
	if got := taskMultiWorktreeStatusPath("old.txt -> renamed.txt"); got != "renamed.txt" {
		t.Fatalf("renamed status path = %q, want renamed.txt", got)
	}
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

	t.Run("Should squash two task worktrees into ordered commits and fast-forward main", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		ctx := context.Background()
		root := t.TempDir()
		allocator := newTaskMultiWorktreeAllocator(root)
		base, err := allocator.ResolveBase(ctx, repo)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		integrationBranch := "compozy/parallel-clean"
		integrationPath := filepath.Join(t.TempDir(), "integration")
		if err := allocator.CreateIntegrationBranch(
			ctx,
			repo,
			integrationPath,
			integrationBranch,
			base.Commit,
		); err != nil {
			t.Fatalf("CreateIntegrationBranch() error = %v", err)
		}

		first := allocateTaskMultiWorktreeForTest(t, allocator, repo, base, "task_01", 1)
		second := allocateTaskMultiWorktreeForTest(t, allocator, repo, base, "task_02", 2)
		writeTaskMultiWorktreeFile(t, first.Path, "task-01.txt", "task one\n")
		writeTaskMultiWorktreeFile(t, second.Path, "task-02.txt", "task two\n")
		firstRef, err := allocator.Commit(ctx, first.Path, "capture task 01")
		if err != nil {
			t.Fatalf("Commit(first) error = %v", err)
		}
		secondRef, err := allocator.Commit(ctx, second.Path, "capture task 02")
		if err != nil {
			t.Fatalf("Commit(second) error = %v", err)
		}
		conflicts, err := allocator.SquashMerge(ctx, integrationPath, firstRef, "task 01: Add task one")
		if err != nil {
			t.Fatalf("SquashMerge(first) error = %v", err)
		}
		if !conflicts.Clean {
			t.Fatalf("SquashMerge(first) conflicts = %#v, want clean", conflicts)
		}
		conflicts, err = allocator.SquashMerge(ctx, integrationPath, secondRef, "task 02: Add task two")
		if err != nil {
			t.Fatalf("SquashMerge(second) error = %v", err)
		}
		if !conflicts.Clean {
			t.Fatalf("SquashMerge(second) conflicts = %#v, want clean", conflicts)
		}
		messages := strings.Split(
			runGitOutput(t, repo, "log", "--reverse", "--format=%s", "main.."+integrationBranch),
			"\n",
		)
		wantMessages := "task 01: Add task one|task 02: Add task two"
		if got := strings.Join(messages, "|"); got != wantMessages {
			t.Fatalf("integration commit messages = %s, want %s", got, wantMessages)
		}
		if _, err := os.Stat(filepath.Join(repo, "task-01.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("main has task-01 before fast-forward: %v", err)
		}
		if err := allocator.FastForward(ctx, repo, "main", integrationBranch); err != nil {
			t.Fatalf("FastForward() error = %v", err)
		}
		if got, want := runGitOutput(
			t,
			repo,
			"rev-parse",
			"main",
		), runGitOutput(
			t,
			repo,
			"rev-parse",
			integrationBranch,
		); got != want {
			t.Fatalf("main head = %q, want integration head %q", got, want)
		}
		for _, name := range []string{"task-01.txt", "task-02.txt"} {
			if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
				t.Fatalf("expected %s after fast-forward: %v", name, err)
			}
		}
		if err := allocator.Remove(ctx, repo, first.Path); err != nil {
			t.Fatalf("Remove(first) error = %v", err)
		}
		if err := allocator.Remove(ctx, repo, second.Path); err != nil {
			t.Fatalf("Remove(second) error = %v", err)
		}
		if err := allocator.DiscardIntegrationBranch(ctx, repo, integrationPath, integrationBranch); err != nil {
			t.Fatalf("DiscardIntegrationBranch() error = %v", err)
		}
		if err := allocator.Prune(ctx, repo); err != nil {
			t.Fatalf("Prune() error = %v", err)
		}
		for _, path := range []string{first.Path, second.Path, integrationPath} {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("worktree path %s stat error = %v, want not exist", path, err)
			}
		}
	})

	t.Run("Should report conflicts and discard integration branch without moving main", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		ctx := context.Background()
		writeTaskMultiWorktreeFile(t, repo, "story.txt", "base\n")
		runGitOutput(t, repo, "add", "story.txt")
		runGitOutput(t, repo, "commit", "-q", "-m", "add story")
		preHead := runGitOutput(t, repo, "rev-parse", "main")
		preContent := readTaskMultiWorktreeFile(t, repo, "story.txt")

		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		base, err := allocator.ResolveBase(ctx, repo)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		integrationBranch := "compozy/parallel-conflict"
		integrationPath := filepath.Join(t.TempDir(), "integration")
		if err := allocator.CreateIntegrationBranch(
			ctx,
			repo,
			integrationPath,
			integrationBranch,
			base.Commit,
		); err != nil {
			t.Fatalf("CreateIntegrationBranch() error = %v", err)
		}
		first := allocateTaskMultiWorktreeForTest(t, allocator, repo, base, "task_01", 1)
		second := allocateTaskMultiWorktreeForTest(t, allocator, repo, base, "task_02", 2)
		writeTaskMultiWorktreeFile(t, first.Path, "story.txt", "first\n")
		writeTaskMultiWorktreeFile(t, second.Path, "story.txt", "second\n")
		firstRef, err := allocator.Commit(ctx, first.Path, "capture task 01")
		if err != nil {
			t.Fatalf("Commit(first) error = %v", err)
		}
		secondRef, err := allocator.Commit(ctx, second.Path, "capture task 02")
		if err != nil {
			t.Fatalf("Commit(second) error = %v", err)
		}
		if conflicts, err := allocator.SquashMerge(
			ctx,
			integrationPath,
			firstRef,
			"task 01: first story",
		); err != nil ||
			!conflicts.Clean {
			t.Fatalf("SquashMerge(first) conflicts = %#v, err = %v, want clean", conflicts, err)
		}
		conflicts, err := allocator.SquashMerge(ctx, integrationPath, secondRef, "task 02: second story")
		if err != nil {
			t.Fatalf("SquashMerge(second) error = %v, want conflict set", err)
		}
		if conflicts.Clean || strings.Join(conflicts.Files, ",") != "story.txt" {
			t.Fatalf("SquashMerge(second) conflicts = %#v, want story.txt conflict", conflicts)
		}
		if err := allocator.DiscardIntegrationBranch(ctx, repo, integrationPath, integrationBranch); err != nil {
			t.Fatalf("DiscardIntegrationBranch() error = %v", err)
		}
		if got := runGitOutput(t, repo, "rev-parse", "main"); got != preHead {
			t.Fatalf("main head = %q, want unchanged %q", got, preHead)
		}
		if got := readTaskMultiWorktreeFile(t, repo, "story.txt"); got != preContent {
			t.Fatalf("main story content = %q, want unchanged %q", got, preContent)
		}
		if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
			t.Fatalf("main status after rollback = %q, want clean", got)
		}
		if _, err := runGitOutputContext(
			ctx,
			repo,
			"rev-parse",
			"--verify",
			"refs/heads/"+integrationBranch,
		); err == nil {
			t.Fatalf("integration branch %s still exists after discard", integrationBranch)
		}
		if err := allocator.Remove(ctx, repo, first.Path); err != nil {
			t.Fatalf("Remove(first) error = %v", err)
		}
		if err := allocator.Remove(ctx, repo, second.Path); err != nil {
			t.Fatalf("Remove(second) error = %v", err)
		}
	})
}

func TestParallelOrchestratorConflictResolverIntegration(t *testing.T) {
	t.Run("Should merge a deterministic conflict resolved by a stub resolver", func(t *testing.T) {
		t.Parallel()
		repo, base := initTaskMultiWorktreeStoryRepo(t)
		ctx := context.Background()
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		integrationPath := filepath.Join(t.TempDir(), "integration")
		plan := parallelConflictIntegrationPlan(t, repo, base, integrationPath, "resolved")
		launcher := &daemonConflictTaskLauncher{
			allocator: allocator,
			repo:      repo,
			writes: map[runparallel.TaskID]string{
				"task_01": "first\n",
				"task_02": "first\nsecond\n",
			},
		}
		resolver := &daemonStubConflictResolver{resolution: "first\nsecond\n"}

		outcome, err := runparallel.NewParallelExecutionOrchestrator(
			parallelWorktreeLifecycle{allocator: allocator},
			launcher,
			runparallel.WithConflictResolver(resolver),
		).Run(ctx, plan)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if outcome.Status != runparallel.ParallelOutcomeCompleted {
			t.Fatalf("status = %q, want completed", outcome.Status)
		}
		if resolver.calls != 1 {
			t.Fatalf("resolver calls = %d, want 1", resolver.calls)
		}
		if got := readTaskMultiWorktreeFile(t, repo, "story.txt"); got != "first\nsecond\n" {
			t.Fatalf("main story = %q, want resolved content", got)
		}
		if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
			t.Fatalf("main status = %q, want clean", got)
		}
		messages := runGitOutput(t, repo, "log", "--reverse", "--format=%s", base.Commit+"..main")
		if !strings.Contains(messages, "task 02: task_02") {
			t.Fatalf("main log missing resolved squash commit:\n%s", messages)
		}
	})

	t.Run("Should drive the real agentic resolver through real git status and make verify", func(t *testing.T) {
		t.Parallel()
		repo := initTaskMultiWorktreeRepo(t)
		writeTaskMultiWorktreeFile(t, repo, "story.txt", "base\n")
		writeTaskMultiWorktreeFile(t, repo, "Makefile", "verify:\n\t@test ! -f FAIL_VERIFY\n")
		runGitOutput(t, repo, "add", "story.txt", "Makefile")
		runGitOutput(t, repo, "commit", "-q", "-m", "add story and verify gate")
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		base, err := allocator.ResolveBase(context.Background(), repo)
		if err != nil {
			t.Fatalf("ResolveBase() error = %v", err)
		}
		ctx := context.Background()
		integrationPath := filepath.Join(t.TempDir(), "integration")
		plan := parallelConflictIntegrationPlan(t, repo, base, integrationPath, "real-resolver")
		launcher := &daemonConflictTaskLauncher{
			allocator: allocator,
			repo:      repo,
			writes: map[runparallel.TaskID]string{
				"task_01": "first\n",
				"task_02": "first\nsecond\n",
			},
		}
		executorCalls := 0
		resolver := runparallel.NewAgenticConflictResolution(
			runparallel.WithConflictPreparedPromptExecutor(func(
				ctx context.Context,
				cfg *model.RuntimeConfig,
				_ string,
				_ *reusableagents.ExecutionContext,
				_ execpkg.SessionMCPBuilder,
			) (execpkg.PreparedPromptResult, error) {
				executorCalls++
				if cfg == nil {
					return execpkg.PreparedPromptResult{}, errors.New("conflict runtime config is required")
				}
				if cfg.WorkspaceRoot != integrationPath {
					return execpkg.PreparedPromptResult{}, fmt.Errorf(
						"resolver workspace = %q, want %q",
						cfg.WorkspaceRoot,
						integrationPath,
					)
				}
				if cfg.ParentRunID != plan.RunID {
					return execpkg.PreparedPromptResult{}, fmt.Errorf(
						"parent run id = %q, want %q",
						cfg.ParentRunID,
						plan.RunID,
					)
				}
				if !strings.Contains(cfg.SystemPrompt, "File: story.txt") ||
					!strings.Contains(cfg.SystemPrompt, "<<<<<<<") {
					return execpkg.PreparedPromptResult{}, fmt.Errorf(
						"system prompt missing conflict hunk:\n%s",
						cfg.SystemPrompt,
					)
				}
				content := readTaskMultiWorktreeFile(t, integrationPath, "story.txt")
				if !strings.Contains(content, "<<<<<<<") || !strings.Contains(content, ">>>>>>>") {
					return execpkg.PreparedPromptResult{}, fmt.Errorf("story.txt has no conflict markers:\n%s", content)
				}
				writeTaskMultiWorktreeFile(t, integrationPath, "story.txt", "first\nsecond\n")
				if _, err := runGitOutputContext(ctx, integrationPath, "add", "story.txt"); err != nil {
					return execpkg.PreparedPromptResult{}, err
				}
				return execpkg.PreparedPromptResult{RunID: "conflict-resolver-run", Output: "resolved"}, nil
			}),
		)

		outcome, err := runparallel.NewParallelExecutionOrchestrator(
			parallelWorktreeLifecycle{allocator: allocator},
			launcher,
			runparallel.WithConflictResolver(resolver),
		).Run(ctx, plan)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if outcome.Status != runparallel.ParallelOutcomeCompleted {
			t.Fatalf("status = %q, want completed", outcome.Status)
		}
		if executorCalls != 1 {
			t.Fatalf("executor calls = %d, want 1", executorCalls)
		}
		if got := readTaskMultiWorktreeFile(t, repo, "story.txt"); got != "first\nsecond\n" {
			t.Fatalf("main story = %q, want resolved content", got)
		}
		if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
			t.Fatalf("main status = %q, want clean", got)
		}
		messages := runGitOutput(t, repo, "log", "--reverse", "--format=%s", base.Commit+"..main")
		if !strings.Contains(messages, "task 02: task_02") {
			t.Fatalf("main log missing resolved squash commit:\n%s", messages)
		}
	})

	t.Run("Should roll back when the stub resolver exhausts", func(t *testing.T) {
		t.Parallel()
		repo, base := initTaskMultiWorktreeStoryRepo(t)
		preHead := runGitOutput(t, repo, "rev-parse", "main")
		preContent := readTaskMultiWorktreeFile(t, repo, "story.txt")
		ctx := context.Background()
		allocator := newTaskMultiWorktreeAllocator(t.TempDir())
		integrationPath := filepath.Join(t.TempDir(), "integration")
		plan := parallelConflictIntegrationPlan(t, repo, base, integrationPath, "exhausted")
		launcher := &daemonConflictTaskLauncher{
			allocator: allocator,
			repo:      repo,
			writes: map[runparallel.TaskID]string{
				"task_01": "first\n",
				"task_02": "first\nsecond\n",
			},
		}
		resolver := &daemonStubConflictResolver{exhaust: true}

		outcome, err := runparallel.NewParallelExecutionOrchestrator(
			parallelWorktreeLifecycle{allocator: allocator},
			launcher,
			runparallel.WithConflictResolver(resolver),
		).Run(ctx, plan)
		if err == nil {
			t.Fatal("Run() error = nil, want conflict exhaustion")
		}
		if outcome.Status != runparallel.ParallelOutcomeRolledBack {
			t.Fatalf("status = %q, want rolled_back", outcome.Status)
		}
		if resolver.calls != 1 {
			t.Fatalf("resolver calls = %d, want 1", resolver.calls)
		}
		if got := runGitOutput(t, repo, "rev-parse", "main"); got != preHead {
			t.Fatalf("main head = %q, want unchanged %q", got, preHead)
		}
		if got := readTaskMultiWorktreeFile(t, repo, "story.txt"); got != preContent {
			t.Fatalf("main story = %q, want unchanged %q", got, preContent)
		}
		if got := runGitOutput(t, repo, "status", "--porcelain"); got != "" {
			t.Fatalf("main status = %q, want clean", got)
		}
		if _, err := runGitOutputContext(
			ctx,
			repo,
			"rev-parse",
			"--verify",
			"refs/heads/"+plan.IntegrationBranch,
		); err == nil {
			t.Fatalf("integration branch %s still exists after rollback", plan.IntegrationBranch)
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

func allocateTaskMultiWorktreeForTest(
	t *testing.T,
	allocator *taskMultiWorktreeAllocator,
	repo string,
	base taskMultiWorktreeBase,
	slug string,
	taskNumber int,
) taskMultiWorktreeAllocation {
	t.Helper()
	alloc, err := allocator.Allocate(context.Background(), taskMultiWorktreeSpec{
		WorkspaceRoot: repo,
		ParentRunID:   "parallel-writeback-test",
		Slug:          slug,
		Index:         0,
		TaskNumber:    taskNumber,
		Base:          base,
	})
	if err != nil {
		t.Fatalf("Allocate(%s) error = %v", slug, err)
	}
	return alloc
}

func writeTaskMultiWorktreeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readTaskMultiWorktreeFile(t *testing.T, dir string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func initTaskMultiWorktreeStoryRepo(t *testing.T) (string, taskMultiWorktreeBase) {
	t.Helper()
	repo := initTaskMultiWorktreeRepo(t)
	writeTaskMultiWorktreeFile(t, repo, "story.txt", "base\n")
	runGitOutput(t, repo, "add", "story.txt")
	runGitOutput(t, repo, "commit", "-q", "-m", "add story")
	allocator := newTaskMultiWorktreeAllocator(t.TempDir())
	base, err := allocator.ResolveBase(context.Background(), repo)
	if err != nil {
		t.Fatalf("ResolveBase() error = %v", err)
	}
	return repo, base
}

func parallelConflictIntegrationPlan(
	t *testing.T,
	repo string,
	base taskMultiWorktreeBase,
	integrationPath string,
	runSuffix string,
) runparallel.ParallelPlan {
	t.Helper()
	entries := []model.TaskEntry{
		{ID: "task_01", Title: "task_01", Status: "pending"},
		{ID: "task_02", Title: "task_02", Status: "pending"},
	}
	waves, err := runparallel.BuildWaves(entries)
	if err != nil {
		t.Fatalf("BuildWaves() error = %v", err)
	}
	enabled := true
	maxConcurrency := 2
	return runparallel.ParallelPlan{
		RunID:             "parallel-conflict-" + runSuffix,
		WorkspaceRoot:     repo,
		BaseBranch:        base.Branch,
		BaseCommit:        base.Commit,
		IntegrationBranch: "compozy/parallel-conflict-" + runSuffix,
		IntegrationPath:   integrationPath,
		Waves:             waves,
		Tasks: []runparallel.TaskSpec{
			{ID: "task_01", Number: 1, Title: "task_01", Slug: "task_01"},
			{ID: "task_02", Number: 2, Title: "task_02", Slug: "task_02"},
		},
		Config: workspace.ParallelTasksConfig{
			Enabled:        &enabled,
			MaxConcurrency: &maxConcurrency,
		},
	}
}

type daemonConflictTaskLauncher struct {
	allocator *taskMultiWorktreeAllocator
	repo      string
	writes    map[runparallel.TaskID]string
}

func (l *daemonConflictTaskLauncher) PrepareTask(
	ctx context.Context,
	spec runparallel.TaskLaunchSpec,
) (runparallel.PreparedTaskRun, error) {
	if l == nil || l.allocator == nil {
		return nil, errors.New("missing conflict task launcher allocator")
	}
	alloc, err := l.allocator.Allocate(ctx, taskMultiWorktreeSpec{
		WorkspaceRoot: l.repo,
		ParentRunID:   spec.RunID,
		Slug:          spec.Task.Slug,
		Index:         spec.Task.Number - 1,
		TaskNumber:    spec.Task.Number,
		Base: taskMultiWorktreeBase{
			Branch: spec.Base.Branch,
			Commit: spec.Base.Commit,
		},
	})
	if err != nil {
		return nil, err
	}
	return &daemonConflictPreparedTaskRun{
		result: runparallel.TaskRunResult{
			Task:         spec.Task,
			RunID:        fmt.Sprintf("run-task-%02d", spec.Task.Number),
			WorktreePath: alloc.Path,
			BaseBranch:   alloc.BaseBranch,
			BaseCommit:   alloc.BaseCommit,
		},
		content: l.writes[spec.Task.ID],
	}, nil
}

type daemonConflictPreparedTaskRun struct {
	result  runparallel.TaskRunResult
	content string
}

func (r *daemonConflictPreparedTaskRun) Execute(context.Context) (recovery.RunOutcome, error) {
	if err := os.WriteFile(filepath.Join(r.result.WorktreePath, "story.txt"), []byte(r.content), 0o600); err != nil {
		return recovery.RunOutcome{}, err
	}
	return recovery.RunOutcome{
		RunID:  r.result.RunID,
		Status: recovery.StatusSucceeded,
		Jobs: []recovery.JobOutcome{{
			SafeName: fmt.Sprintf("task-%02d", r.result.Task.Number),
			Status:   recovery.StatusSucceeded,
		}},
	}, nil
}

func (r *daemonConflictPreparedTaskRun) RestartFailed(
	context.Context,
	[]string,
) (recovery.RunOutcome, error) {
	return recovery.RunOutcome{}, errors.New("conflict integration task should not restart")
}

func (r *daemonConflictPreparedTaskRun) Result() runparallel.TaskRunResult {
	return r.result
}

func (r *daemonConflictPreparedTaskRun) FailedConfig() *model.RuntimeConfig {
	return &model.RuntimeConfig{WorkspaceRoot: r.result.WorktreePath}
}

type daemonStubConflictResolver struct {
	resolution string
	exhaust    bool
	calls      int
}

func (r *daemonStubConflictResolver) Resolve(
	ctx context.Context,
	in runparallel.ConflictInput,
) (runparallel.ConflictResult, error) {
	r.calls++
	if r.exhaust {
		return runparallel.ConflictResult{Resolved: false, Builds: false, Attempts: in.MaxAttempts}, nil
	}
	if err := os.WriteFile(
		filepath.Join(in.IntegrationWorktree, "story.txt"),
		[]byte(r.resolution),
		0o600,
	); err != nil {
		return runparallel.ConflictResult{}, err
	}
	if _, err := runGitOutputContext(ctx, in.IntegrationWorktree, "add", "story.txt"); err != nil {
		return runparallel.ConflictResult{}, err
	}
	return runparallel.ConflictResult{Resolved: true, Builds: true, Attempts: 1}, nil
}
