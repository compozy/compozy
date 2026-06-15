package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/daemon"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// tasksRunFlagRegexp captures long-flag tokens (e.g. --parallel-limit) from
// README command snippets.
var tasksRunFlagRegexp = regexp.MustCompile(`--([a-zA-Z][a-zA-Z0-9-]*)`)

// TestREADMETasksRunSnippetsMatchCLIHelp keeps the README aligned with the
// actual CLI surface: every long flag the README shows on a `tasks run` line
// must be a real flag on the command, and the documented parallel defaults must
// match the registered defaults.
func TestREADMETasksRunSnippetsMatchCLIHelp(t *testing.T) {
	t.Parallel()

	readmePath := mustCLIRepoRootPath(t, "README.md")
	body, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}
	readme := string(body)

	cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())

	t.Run("Should register every flag the README documents on tasks run", func(t *testing.T) {
		t.Parallel()
		seen := map[string]bool{}
		for _, line := range strings.Split(readme, "\n") {
			if !strings.Contains(line, "tasks run") {
				continue
			}
			for _, match := range tasksRunFlagRegexp.FindAllStringSubmatch(line, -1) {
				flag := match[1]
				if seen[flag] {
					continue
				}
				seen[flag] = true
				if cmd.Flags().Lookup(flag) == nil {
					t.Fatalf("README documents `tasks run --%s` but the command has no such flag", flag)
				}
			}
		}
		// Guard the new parallel surface is actually exercised by the README.
		for _, flag := range []string{"multiple", "parallel", "parallel-limit"} {
			if !seen[flag] {
				t.Fatalf("expected README tasks run snippets to document --%s", flag)
			}
		}
	})

	t.Run("Should match the documented parallel flag types and defaults", func(t *testing.T) {
		t.Parallel()
		parallel := cmd.Flags().Lookup("parallel")
		if parallel == nil || parallel.Value.Type() != "bool" || parallel.DefValue != "false" {
			t.Fatalf("--parallel flag = %#v, want bool default false", parallel)
		}
		limit := cmd.Flags().Lookup("parallel-limit")
		if limit == nil || limit.Value.Type() != "int" || limit.DefValue != "2" {
			t.Fatalf("--parallel-limit flag = %#v, want int default 2", limit)
		}
		if !strings.Contains(readme, "run_multiple_parallel_limit = 2") {
			t.Fatal("expected README to document run_multiple_parallel_limit = 2")
		}
	})

	t.Run("Should omit the obsolete enqueued-fallback wording", func(t *testing.T) {
		t.Parallel()
		for _, stale := range []string{
			"prints a fallback message",
			"still runs the queue as",
			"runs the queue in enqueued order",
		} {
			if strings.Contains(readme, stale) {
				t.Fatalf("expected README to omit obsolete parallel-fallback wording %q", stale)
			}
		}
	})
}

// TestTasksRunMultipleParallelEndToEndReportsWorktreePaths exercises the full
// CLI -> in-process daemon -> worktree-backed parallel scheduler path and
// asserts the parent starts in parallel mode and the final handoff reports each
// child's preserved worktree.
func TestTasksRunMultipleParallelEndToEndReportsWorktreePaths(t *testing.T) {
	t.Run("Should report each child's preserved worktree in the parallel handoff", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths := newParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", "--multiple", "alpha,beta", "--parallel", "--stream", "--dry-run",
		)
		if err != nil {
			t.Fatalf("execute parallel multi-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(
			stdout,
			"task multi-run started:",
			"task queue started | mode=parallel total=2",
			"task multi-run handoff:",
			"branch=main",
		) {
			t.Fatalf("expected parallel start + worktree handoff output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}

		runID := taskMultiRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		assertParallelWorktreeSnapshot(t, snapshot, []string{"alpha", "beta"}, paths)
	})
}

// TestTasksRunMultipleParallelLimitOneEndToEnd verifies that --parallel-limit 1
// flows through to the daemon (the resolved limit is emitted) and that the run
// still completes every child with a final handoff.
func TestTasksRunMultipleParallelLimitOneEndToEnd(t *testing.T) {
	t.Run("Should flow --parallel-limit through to the daemon and complete every child", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths := newParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", "--multiple", "alpha,beta",
			"--parallel", "--parallel-limit", "1", "--stream", "--dry-run",
		)
		if err != nil {
			t.Fatalf("execute parallel-limit run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(stdout, "task queue started | mode=parallel total=2", "task multi-run handoff:") {
			t.Fatalf("expected bounded parallel handoff output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}

		runID := taskMultiRunIDFromCLIOutput(t, stdout)
		if limit := taskMultiStartedParallelLimit(t, client, runID); limit != 1 {
			t.Fatalf("resolved parallel limit on started event = %d, want 1", limit)
		}
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		assertParallelWorktreeSnapshot(t, snapshot, []string{"alpha", "beta"}, paths)
	})
}

func assertParallelWorktreeSnapshot(
	t *testing.T,
	snapshot apicore.TaskRunMultipleSnapshot,
	wantSlugs []string,
	paths compozyconfig.HomePaths,
) {
	t.Helper()
	if snapshot.Run.Mode != "task_multi" || snapshot.Run.Status != "completed" {
		t.Fatalf("snapshot parent = (mode=%q,status=%q), want (task_multi,completed)",
			snapshot.Run.Mode, snapshot.Run.Status)
	}
	if len(snapshot.Items) != len(wantSlugs) {
		t.Fatalf("snapshot item count = %d, want %d: %#v", len(snapshot.Items), len(wantSlugs), snapshot.Items)
	}
	for i := range snapshot.Items {
		item := &snapshot.Items[i]
		if item.Slug != wantSlugs[i] {
			t.Fatalf("snapshot item %d slug = %q, want %q", i, item.Slug, wantSlugs[i])
		}
		if item.Status != "completed" {
			t.Fatalf("snapshot item %q status = %q, want completed", item.Slug, item.Status)
		}
		if strings.TrimSpace(item.RunID) == "" {
			t.Fatalf("snapshot item %q is missing a child run id", item.Slug)
		}
		if !strings.HasPrefix(item.WorktreePath, paths.WorktreesDir) {
			t.Fatalf("snapshot item %q worktree path = %q, want under %q",
				item.Slug, item.WorktreePath, paths.WorktreesDir)
		}
		if item.WorktreeStatus != "preserved" {
			t.Fatalf("snapshot item %q worktree status = %q, want preserved", item.Slug, item.WorktreeStatus)
		}
		if item.BaseBranch != "main" {
			t.Fatalf("snapshot item %q base branch = %q, want main", item.Slug, item.BaseBranch)
		}
	}
}

// newParallelMultiRunCLIEnv builds a committed git workspace with the requested
// task workflows and an in-process daemon whose run manager is pointed at a
// home-scoped worktrees root, returning the daemon client and resolved paths.
func newParallelMultiRunCLIEnv(
	t *testing.T,
	slugs []string,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths) {
	t.Helper()

	workspaceRoot := t.TempDir()
	for _, slug := range slugs {
		writeTaskWorkflowForCLI(t, workspaceRoot, slug)
	}
	gitInitCommitCLIWorkspace(t, workspaceRoot)

	// Establish the daemon home before constructing the run manager so the
	// worktrees root can be passed into the manager config.
	prepareInProcessCLIDaemonHome(t)
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := installInProcessCLIDaemonBootstrapWithConfigClient(t, daemon.RunManagerConfig{
		WorktreesRoot: paths.WorktreesDir,
		Prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		Execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	return client, paths
}

func runParallelMultiRunCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	return executeCommandCapturingProcessIO(t, cmd, nil, args...)
}

func taskMultiStartedParallelLimit(t *testing.T, client *inProcessDaemonCommandClient, runID string) int {
	t.Helper()
	page, err := client.ListRunEvents(context.Background(), runID, apicore.StreamCursor{}, 500)
	if err != nil {
		t.Fatalf("ListRunEvents(%q) error = %v", runID, err)
	}
	for i := range page.Events {
		event := page.Events[i]
		if event.Kind != eventspkg.EventKindTaskRunMultipleStarted {
			continue
		}
		var payload kinds.TaskRunMultiplePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode task.multi.started payload: %v", err)
		}
		return payload.ParallelLimit
	}
	t.Fatalf("task.multi.started event not found for run %q", runID)
	return 0
}

func requireGitForCLITests(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

func gitInitCommitCLIWorkspace(t *testing.T, root string) {
	t.Helper()
	runGitForCLITests(t, root, "init", "-q", "-b", "main")
	runGitForCLITests(t, root, "config", "user.email", "multi-run@example.com")
	runGitForCLITests(t, root, "config", "user.name", "Multi Run Tester")
	runGitForCLITests(t, root, "config", "commit.gpgsign", "false")
	runGitForCLITests(t, root, "add", "-A")
	runGitForCLITests(t, root, "commit", "-q", "-m", "seed parallel multi-run workspace")
}

func runGitForCLITests(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}
