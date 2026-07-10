package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/worktree"
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
		for _, flag := range []string{"multiple", "parallel", "parallel-limit", "parallel-tasks"} {
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
		parallelTasks := cmd.Flags().Lookup("parallel-tasks")
		if parallelTasks == nil || parallelTasks.Value.Type() != "bool" || parallelTasks.DefValue != "false" {
			t.Fatalf("--parallel-tasks flag = %#v, want bool default false", parallelTasks)
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

// TestTasksRunSingleWithoutParallelChoiceEndToEndStaysStandard proves that an
// ambient parallel-task config does not authorize worktrees for a single-spec
// run. The per-run execution descriptor remains the source of truth.
func TestTasksRunSingleWithoutParallelChoiceEndToEndStaysStandard(t *testing.T) {
	t.Run("Should ignore ambient worktree enablement without a per-run choice", func(t *testing.T) {
		requireGitForCLITests(t)

		const slug = "alpha"
		client, paths, workspaceRoot := newParallelMultiRunCLIEnv(t, []string{slug})
		writeCLIWorkspaceConfig(t, workspaceRoot, `
[tasks.run.parallel]
enabled = true
`)

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", slug, "--stream", "--dry-run",
		)
		if err != nil {
			t.Fatalf("execute standard single run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(stdout, "task run started:", "(mode=stream)", "run completed") {
			t.Fatalf("expected standard run to reach terminal output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if !containsAll(
			stderr,
			"execution: Standard task run",
			"kind=task_standard",
			"worktrees=false",
			"source=per-run default",
		) {
			t.Fatalf("expected explicit standard execution resolution, got stderr:\n%s", stderr)
		}

		runID := taskRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetRunSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetRunSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.Run.Status != "completed" {
			t.Fatalf("snapshot status = %q, want completed", snapshot.Run.Status)
		}
		page, err := client.ListRunEvents(context.Background(), runID, apicore.StreamCursor{}, 500)
		if err != nil {
			t.Fatalf("ListRunEvents(%q) error = %v", runID, err)
		}
		for _, kind := range []eventspkg.EventKind{
			eventspkg.EventKindTaskParallelPlanStarted,
			eventspkg.EventKindTaskParallelWaveStarted,
			eventspkg.EventKindTaskParallelCompleted,
		} {
			if hasRunEventKind(page.Events, kind) {
				t.Fatalf("standard run emitted parallel lifecycle %q: %v", kind, eventKindsFromCoreEvents(page.Events))
			}
		}
		assertNoCLIWorktreesUnderRoot(t, workspaceRoot, paths.WorktreesDir)
	})
}

// TestTasksRunMultipleEnqueuedEndToEndStaysWorktreeFree exercises the serial
// multi-spec path through the real in-process daemon and proves the observer
// exits at the parent terminal without allocating worktrees.
func TestTasksRunMultipleEnqueuedEndToEndStaysWorktreeFree(t *testing.T) {
	t.Run("Should complete the serial queue without allocating worktrees", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths, workspaceRoot := newParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", "--multiple", "alpha,beta", "--stream", "--dry-run",
		)
		if err != nil {
			t.Fatalf("execute enqueued multi-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(
			stdout,
			"task multi-run started:",
			"task queue started | mode=enqueued total=2",
			"task queue completed | total=2",
			"task multi-run handoff:",
		) {
			t.Fatalf("expected serial queue to exit at terminal, got:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if !containsAll(
			stderr,
			"execution: Serial queue (no worktrees)",
			"kind=task_multi_enqueued",
			"worktrees=false",
			"source=built-in default",
		) {
			t.Fatalf("expected explicit enqueued execution resolution, got stderr:\n%s", stderr)
		}

		runID := taskMultiRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.ExecutionKind != apicore.ExecutionKindTaskMultiEnqueued {
			t.Fatalf("execution kind = %q, want %q", snapshot.ExecutionKind, apicore.ExecutionKindTaskMultiEnqueued)
		}
		if snapshot.Run.Status != "completed" {
			t.Fatalf("snapshot status = %q, want completed", snapshot.Run.Status)
		}
		for i := range snapshot.Items {
			item := &snapshot.Items[i]
			if item.WorktreePath != "" || item.WorktreeStatus != "" || item.ResultBranch != "" {
				t.Fatalf("enqueued item %q unexpectedly has worktree metadata: %#v", item.Slug, item)
			}
		}
		assertNoCLIWorktreesUnderRoot(t, workspaceRoot, paths.WorktreesDir)
	})
}

// TestTasksRunMultipleParallelEndToEndReportsWorktreePaths exercises the full
// CLI -> in-process daemon -> worktree-backed parallel scheduler path and
// asserts the parent starts in parallel mode while each temporary worktree is
// removed after its output becomes reachable through a named result branch.
func TestTasksRunMultipleParallelEndToEndReportsWorktreePaths(t *testing.T) {
	t.Run("Should report removed worktrees and retained result branches in the parallel handoff", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths, workspaceRoot := newParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

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
		assertNoCLIWorktreesUnderRoot(t, workspaceRoot, paths.WorktreesDir)
	})
}

// TestTasksRunMultipleParallelLimitOneEndToEnd verifies that --parallel-limit 1
// flows through to the daemon (the resolved limit is emitted) and that the run
// still completes every child with a final handoff.
func TestTasksRunMultipleParallelLimitOneEndToEnd(t *testing.T) {
	t.Run("Should flow --parallel-limit through to the daemon and complete every child", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths, workspaceRoot := newParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

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
		assertNoCLIWorktreesUnderRoot(t, workspaceRoot, paths.WorktreesDir)
	})
}

func TestTasksRunParallelTasksEndToEndRoutesSingleWorkflowThroughParallelOrchestrator(t *testing.T) {
	t.Run("Should run one workflow in dependency-aware parallel task mode", func(t *testing.T) {
		requireGitForCLITests(t)

		const slug = "demo"
		client, _, workspaceRoot := newParallelTasksCLIEnv(t, slug)
		assertNoCompozyTaskFilesTrackedForCLI(t, workspaceRoot)

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", slug,
			"--parallel-tasks",
			"--stream",
			"--dry-run",
		)
		if err != nil {
			t.Fatalf("execute --parallel-tasks run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(stdout, "task run started:", "(mode=stream)", "run completed") {
			t.Fatalf("expected started and completed output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}
		runID := taskRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.Run.Mode != "task_multi" || snapshot.Run.Status != "completed" {
			t.Fatalf(
				"snapshot run = (mode=%q,status=%q), want (task_multi,completed)",
				snapshot.Run.Mode,
				snapshot.Run.Status,
			)
		}
		for taskNumber := 1; taskNumber <= 3; taskNumber++ {
			path := filepath.Join(workspaceRoot, fmt.Sprintf("cli-task-%02d.txt", taskNumber))
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected merged task output %s: %v", path, err)
			}
		}
		page, err := client.ListRunEvents(context.Background(), runID, apicore.StreamCursor{}, 500)
		if err != nil {
			t.Fatalf("ListRunEvents(%q) error = %v", runID, err)
		}
		if !hasRunEventKind(page.Events, eventspkg.EventKindTaskParallelWaveStarted) ||
			!hasRunEventKind(page.Events, eventspkg.EventKindTaskParallelPlanStarted) ||
			!hasRunEventKind(page.Events, eventspkg.EventKindTaskParallelMerged) ||
			!hasRunEventKind(page.Events, eventspkg.EventKindTaskParallelWaveCompleted) {
			t.Fatalf("parallel task events missing from parent run: %v", eventKindsFromCoreEvents(page.Events))
		}
	})
}

// TestTasksRunParallelTasksFailureEndToEndSettlesAndPreservesOutput proves the
// streamed observer exits non-zero at a failed parent terminal and that cleanup
// distinguishes safely removable failed work from successful partial output
// that still needs a retention point.
func TestTasksRunParallelTasksFailureEndToEndSettlesAndPreservesOutput(t *testing.T) {
	t.Run("Should settle every task and preserve only unintegrated output after failure", func(t *testing.T) {
		requireGitForCLITests(t)

		const slug = "demo"
		client, paths, workspaceRoot := newParallelTasksCLIEnvWithExecutor(
			t,
			slug,
			func(cfg *model.RuntimeConfig, taskNumber int) error {
				if taskNumber == 2 {
					if err := writeCLITaskResultFixture(cfg, "failed", 1, "forced parallel task failure"); err != nil {
						return err
					}
					return errors.New("forced parallel task failure")
				}
				if err := writeCLITaskOutput(cfg, taskNumber); err != nil {
					return err
				}
				return writeCLITaskResultFixture(cfg, "succeeded", 0, "")
			},
		)
		assertNoCompozyTaskFilesTrackedForCLI(t, workspaceRoot)
		baseHead := strings.TrimSpace(runGitOutputForCLITests(t, workspaceRoot, "rev-parse", "HEAD"))

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", slug,
			"--parallel-tasks",
			"--stream",
			"--dry-run",
		)
		if err == nil {
			t.Fatalf("expected failed parallel task run\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		var exitErr *commandExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("stream error = %v, want exit code 1", err)
		}
		if !containsAll(stdout, "task run started:", "(mode=stream)", "run failed") {
			t.Fatalf("expected observer to exit at failed terminal, got:\n%s\nstderr:\n%s", stdout, stderr)
		}

		runID := taskRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.ExecutionKind != apicore.ExecutionKindTaskParallel || snapshot.Run.Status != "failed" {
			t.Fatalf(
				"snapshot = (kind=%q,status=%q), want (%q,failed)",
				snapshot.ExecutionKind,
				snapshot.Run.Status,
				apicore.ExecutionKindTaskParallel,
			)
		}

		page, err := client.ListRunEvents(context.Background(), runID, apicore.StreamCursor{}, 500)
		if err != nil {
			t.Fatalf("ListRunEvents(%q) error = %v", runID, err)
		}
		settlements := make(map[string]kinds.TaskParallelPayload)
		parallelTerminals := 0
		for _, event := range page.Events {
			switch event.Kind {
			case eventspkg.EventKindTaskParallelTaskCompleted:
				payload, ok := decodeObservedPayload[kinds.TaskParallelPayload](event)
				if !ok {
					t.Fatalf("decode task settlement payload: %#v", event.Payload)
				}
				settlements[payload.TaskID] = payload
			case eventspkg.EventKindTaskParallelFailed:
				parallelTerminals++
			}
		}
		if parallelTerminals != 1 {
			t.Fatalf("parallel failed terminals = %d, want one", parallelTerminals)
		}
		wantStatuses := map[string]string{
			"task_01": "merged",
			"task_02": "failed",
			"task_03": "skipped",
		}
		for taskID, wantStatus := range wantStatuses {
			settlement, ok := settlements[taskID]
			if !ok {
				t.Fatalf("missing settlement for %s: %#v", taskID, settlements)
			}
			if settlement.Status != wantStatus {
				t.Fatalf("settlement %s status = %q, want %q", taskID, settlement.Status, wantStatus)
			}
		}
		if settlement := settlements["task_01"]; settlement.WorktreeStatus != "preserved" ||
			strings.TrimSpace(settlement.WorktreeReason) == "" {
			t.Fatalf("successful partial task settlement = %#v, want preserved with reason", settlement)
		} else if _, err := os.Stat(settlement.WorktreePath); err != nil {
			t.Fatalf("preserved task worktree %s missing: %v", settlement.WorktreePath, err)
		}
		if settlement := settlements["task_02"]; settlement.WorktreeStatus != "removed" {
			t.Fatalf("failed no-change task settlement = %#v, want removed", settlement)
		} else if _, err := os.Stat(settlement.WorktreePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed task worktree %s stat = %v, want absent", settlement.WorktreePath, err)
		}
		if settlement := settlements["task_03"]; settlement.WorktreePath != "" || settlement.WorktreeStatus != "" {
			t.Fatalf("skipped task unexpectedly allocated a worktree: %#v", settlement)
		}
		if got := strings.TrimSpace(runGitOutputForCLITests(t, workspaceRoot, "rev-parse", "HEAD")); got != baseHead {
			t.Fatalf("workspace head = %q, want unchanged %q after failed parallel run", got, baseHead)
		}
		if !strings.HasPrefix(settlements["task_01"].WorktreePath, paths.WorktreesDir) {
			t.Fatalf("preserved path = %q, want under %q", settlements["task_01"].WorktreePath, paths.WorktreesDir)
		}
	})
}

func TestTasksRunParallelTasksEndToEndFromLinkedWorktree(t *testing.T) {
	t.Run("Should merge parallel task output back into the linked worktree branch", func(t *testing.T) {
		requireGitForCLITests(t)

		const slug = "demo"
		client, paths, primaryRoot, linkedRoot, _ := newLinkedParallelTasksCLIEnv(t, slug)
		assertNoCompozyTaskFilesTrackedForCLI(t, linkedRoot)

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", slug,
			"--parallel-tasks",
			"--stream",
			"--dry-run",
		)
		if err != nil {
			t.Fatalf("execute linked --parallel-tasks run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(stdout, "task run started:", "(mode=stream)", "run completed") {
			t.Fatalf("expected started and completed output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}
		runID := taskRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.Run.Mode != "task_multi" || snapshot.Run.Status != "completed" {
			t.Fatalf(
				"snapshot run = (mode=%q,status=%q), want (task_multi,completed)",
				snapshot.Run.Mode,
				snapshot.Run.Status,
			)
		}
		for taskNumber := 1; taskNumber <= 3; taskNumber++ {
			linkedPath := filepath.Join(linkedRoot, fmt.Sprintf("cli-task-%02d.txt", taskNumber))
			if _, err := os.Stat(linkedPath); err != nil {
				t.Fatalf("expected merged linked task output %s: %v", linkedPath, err)
			}
			primaryPath := filepath.Join(primaryRoot, fmt.Sprintf("cli-task-%02d.txt", taskNumber))
			if _, err := os.Stat(primaryPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("primary checkout task output %s stat = %v, want absent", primaryPath, err)
			}
		}
		if got := strings.TrimSpace(runGitOutputForCLITests(t, primaryRoot, "status", "--porcelain")); got != "" {
			t.Fatalf("primary checkout status = %q, want clean", got)
		}
		assertNoCLIWorktreesUnderRoot(t, primaryRoot, paths.WorktreesDir)
	})
}

func TestTasksRunMultipleParallelEndToEndFromLinkedWorktree(t *testing.T) {
	t.Run("Should run multiple workflows in linked worktree and purge owned worktrees", func(t *testing.T) {
		requireGitForCLITests(t)

		client, paths, primaryRoot, linkedRoot, branch := newLinkedParallelMultiRunCLIEnv(t, []string{"alpha", "beta"})

		stdout, stderr, err := runParallelMultiRunCLI(
			t,
			"tasks", "run", "--multiple", "alpha,beta", "--parallel", "--stream", "--dry-run",
		)
		if err != nil {
			t.Fatalf("execute linked parallel multi-run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		if !containsAll(
			stdout,
			"task multi-run started:",
			"task queue started | mode=parallel total=2",
			"task multi-run handoff:",
			"branch="+branch,
		) {
			t.Fatalf("expected linked parallel start + worktree handoff output, got:\n%s\nstderr:\n%s", stdout, stderr)
		}

		runID := taskMultiRunIDFromCLIOutput(t, stdout)
		snapshot, err := client.GetTaskRunMultipleSnapshot(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetTaskRunMultipleSnapshot(%q) error = %v", runID, err)
		}
		if snapshot.Run.Mode != "task_multi" || snapshot.Run.Status != "completed" {
			t.Fatalf("snapshot parent = (mode=%q,status=%q), want (task_multi,completed)",
				snapshot.Run.Mode, snapshot.Run.Status)
		}
		if len(snapshot.Items) != 2 {
			t.Fatalf("snapshot item count = %d, want 2: %#v", len(snapshot.Items), snapshot.Items)
		}
		for i := range snapshot.Items {
			item := &snapshot.Items[i]
			if item.BaseBranch != branch {
				t.Fatalf("snapshot item %q base branch = %q, want %q", item.Slug, item.BaseBranch, branch)
			}
			if !strings.HasPrefix(item.WorktreePath, paths.WorktreesDir) {
				t.Fatalf("snapshot item %q worktree path = %q, want under %q",
					item.Slug, item.WorktreePath, paths.WorktreesDir)
			}
		}
		if got := strings.TrimSpace(runGitOutputForCLITests(t, primaryRoot, "status", "--porcelain")); got != "" {
			t.Fatalf("primary checkout status = %q, want clean", got)
		}
		if got := strings.TrimSpace(runGitOutputForCLITests(t, linkedRoot, "status", "--porcelain")); got != "" {
			t.Fatalf("linked checkout status = %q, want clean", got)
		}

		result, err := client.manager.Purge(context.Background(), daemon.RunLifecycleSettings{
			KeepTerminalDays: 0,
			KeepMax:          0,
		})
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}
		if !containsCLIString(result.PurgedRunIDs, runID) {
			t.Fatalf("purged run ids = %v, want parent run %q included", result.PurgedRunIDs, runID)
		}
		assertNoCLIWorktreesUnderRoot(t, primaryRoot, paths.WorktreesDir)
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
		if item.WorktreeStatus != "removed" {
			t.Fatalf("snapshot item %q worktree status = %q, want removed", item.Slug, item.WorktreeStatus)
		}
		if strings.TrimSpace(item.WorktreeReason) == "" {
			t.Fatalf("snapshot item %q is missing its cleanup reason", item.Slug)
		}
		if strings.TrimSpace(item.ResultBranch) == "" {
			t.Fatalf("snapshot item %q is missing its retained result branch", item.Slug)
		}
		if _, err := os.Stat(item.WorktreePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot item %q worktree %s stat = %v, want absent", item.Slug, item.WorktreePath, err)
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
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string) {
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
	return client, paths, workspaceRoot
}

func newParallelTasksCLIEnv(
	t *testing.T,
	slug string,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string) {
	t.Helper()
	return newParallelTasksCLIEnvWithExecutor(
		t,
		slug,
		func(cfg *model.RuntimeConfig, taskNumber int) error {
			if err := writeCLITaskOutput(cfg, taskNumber); err != nil {
				return err
			}
			return writeCLITaskResultFixture(cfg, "succeeded", 0, "")
		},
	)
}

func newParallelTasksCLIEnvWithExecutor(
	t *testing.T,
	slug string,
	executeTask func(*model.RuntimeConfig, int) error,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	writeParallelTasksGitignoreForCLI(t, workspaceRoot)
	writeParallelTasksWorkflowForCLI(t, workspaceRoot, slug)
	gitInitCommitCLIWorkspace(t, workspaceRoot)

	prepareInProcessCLIDaemonHome(t)
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := installInProcessCLIDaemonBootstrapWithConfigClient(t, daemon.RunManagerConfig{
		WorktreesRoot: paths.WorktreesDir,
		Prepare: func(
			_ context.Context,
			cfg *model.RuntimeConfig,
			scope model.RunScope,
		) (*model.SolvePreparation, error) {
			taskNumber, err := requireCLITargetTaskNumber(cfg)
			if err != nil {
				return nil, err
			}
			if scope == nil {
				return nil, errors.New("run scope is required")
			}
			return &model.SolvePreparation{
				Jobs: []model.Job{{
					SafeName: fmt.Sprintf("task-%02d", taskNumber),
				}},
				RunArtifacts: scope.RunArtifacts(),
			}, nil
		},
		Execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			taskNumber, err := requireCLITargetTaskNumber(cfg)
			if err != nil {
				return err
			}
			return executeTask(cfg, taskNumber)
		},
	})
	return client, paths, workspaceRoot
}

func writeCLITaskOutput(cfg *model.RuntimeConfig, taskNumber int) error {
	return os.WriteFile(
		filepath.Join(cfg.WorkspaceRoot, fmt.Sprintf("cli-task-%02d.txt", taskNumber)),
		[]byte(fmt.Sprintf("task %02d\n", taskNumber)),
		0o600,
	)
}

func newLinkedParallelTasksCLIEnv(
	t *testing.T,
	slug string,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string, string, string) {
	t.Helper()

	primaryRoot, linkedRoot, branch := initLinkedCLIWorkspace(t)
	writeParallelTasksWorkflowForCLI(t, linkedRoot, slug)

	prepareInProcessCLIDaemonHome(t)
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	withWorkingDir(t, linkedRoot)

	client := installInProcessCLIDaemonBootstrapWithConfigClient(t, daemon.RunManagerConfig{
		WorktreesRoot: paths.WorktreesDir,
		Prepare: func(
			_ context.Context,
			cfg *model.RuntimeConfig,
			scope model.RunScope,
		) (*model.SolvePreparation, error) {
			taskNumber, err := requireCLITargetTaskNumber(cfg)
			if err != nil {
				return nil, err
			}
			if scope == nil {
				return nil, errors.New("run scope is required")
			}
			return &model.SolvePreparation{
				Jobs: []model.Job{{
					SafeName: fmt.Sprintf("task-%02d", taskNumber),
				}},
				RunArtifacts: scope.RunArtifacts(),
			}, nil
		},
		Execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			taskNumber, err := requireCLITargetTaskNumber(cfg)
			if err != nil {
				return err
			}
			if err := os.WriteFile(
				filepath.Join(cfg.WorkspaceRoot, fmt.Sprintf("cli-task-%02d.txt", taskNumber)),
				[]byte(fmt.Sprintf("task %02d\n", taskNumber)),
				0o600,
			); err != nil {
				return err
			}
			return writeCLITaskResultFixture(cfg, "succeeded", 0, "")
		},
	})
	return client, paths, primaryRoot, linkedRoot, branch
}

func newLinkedParallelMultiRunCLIEnv(
	t *testing.T,
	slugs []string,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string, string, string) {
	t.Helper()

	primaryRoot, linkedRoot, branch := initLinkedCLIWorkspace(t)
	for _, slug := range slugs {
		writeTaskWorkflowForCLI(t, linkedRoot, slug)
	}

	prepareInProcessCLIDaemonHome(t)
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	withWorkingDir(t, linkedRoot)

	client := installInProcessCLIDaemonBootstrapWithConfigClient(t, daemon.RunManagerConfig{
		WorktreesRoot: paths.WorktreesDir,
		Prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		Execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	return client, paths, primaryRoot, linkedRoot, branch
}

func initLinkedCLIWorkspace(t *testing.T) (string, string, string) {
	t.Helper()

	primaryRoot := t.TempDir()
	writeParallelTasksGitignoreForCLI(t, primaryRoot)
	if err := os.WriteFile(filepath.Join(primaryRoot, "README.md"), []byte("primary\n"), 0o600); err != nil {
		t.Fatalf("write primary README: %v", err)
	}
	gitInitCommitCLIWorkspace(t, primaryRoot)
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	branch := "feature-cli-linked"
	runGitForCLITests(t, primaryRoot, "worktree", "add", "-q", "-b", branch, linkedRoot)
	return primaryRoot, linkedRoot, branch
}

func writeParallelTasksGitignoreForCLI(t *testing.T, workspaceRoot string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workspaceRoot, ".gitignore"), []byte(".compozy/**\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}

func writeParallelTasksWorkflowForCLI(t *testing.T, workspaceRoot string, slug string) {
	t.Helper()
	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir task workflow %s: %v", slug, err)
	}
	writeRawTaskFileForCLI(t, tasksDir, "_tasks.md", strings.Join([]string{
		"---",
		"schema_version: \"compozy.tasks/v2\"",
		"workflow: " + slug,
		"graph:",
		"  nodes:",
		"    - id: task_01",
		"      file: task_01.md",
		"    - id: task_02",
		"      file: task_02.md",
		"    - id: task_03",
		"      file: task_03.md",
		"  edges:",
		"    - from: task_01",
		"      to: task_03",
		"    - from: task_02",
		"      to: task_03",
		"---",
		"",
		"# " + slug + " Tasks",
		"",
	}, "\n"))
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: First parallel task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: First parallel task",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_02.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Second parallel task",
			"type: backend",
			"complexity: low",
		},
		"# Task 2: Second parallel task",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_03.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: Dependent parallel task",
			"type: backend",
			"complexity: low",
		},
		"# Task 3: Dependent parallel task",
	))
}

func runParallelMultiRunCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return false }
	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	return executeCommandCapturingProcessIO(t, cmd, nil, args...)
}

func taskRunIDFromCLIOutput(t *testing.T, stdout string) string {
	t.Helper()
	const prefix = "task run started: "
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		runPart := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		runID, _, ok := strings.Cut(runPart, " ")
		if !ok || strings.TrimSpace(runID) == "" {
			t.Fatalf("parse task run id from line %q", line)
		}
		return runID
	}
	t.Fatalf("task run start line not found in output:\n%s", stdout)
	return ""
}

func requireCLITargetTaskNumber(cfg *model.RuntimeConfig) (int, error) {
	if cfg == nil || cfg.TargetTaskNumber == nil {
		return 0, errors.New("parallel CLI child run missing target task number")
	}
	return *cfg.TargetTaskNumber, nil
}

func writeCLITaskResultFixture(
	cfg *model.RuntimeConfig,
	status string,
	exitCode int,
	errText string,
) error {
	if cfg == nil {
		return errors.New("cli task result fixture: runtime config is required")
	}
	artifacts, err := model.ResolveRuntimeRunArtifacts(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(artifacts.ResultPath), 0o755); err != nil {
		return err
	}
	taskNumber, err := requireCLITargetTaskNumber(cfg)
	if err != nil {
		return err
	}
	payload := struct {
		SchemaVersion int    `json:"schema_version"`
		RunID         string `json:"run_id"`
		Status        string `json:"status"`
		ArtifactsDir  string `json:"artifacts_dir"`
		ResultPath    string `json:"result_path"`
		Jobs          []struct {
			SafeName string `json:"safe_name"`
			Status   string `json:"status"`
			ExitCode int    `json:"exit_code"`
			Error    string `json:"error,omitempty"`
		} `json:"jobs"`
	}{
		SchemaVersion: 1,
		RunID:         cfg.RunID,
		Status:        status,
		ArtifactsDir:  artifacts.RunDir,
		ResultPath:    artifacts.ResultPath,
		Jobs: []struct {
			SafeName string `json:"safe_name"`
			Status   string `json:"status"`
			ExitCode int    `json:"exit_code"`
			Error    string `json:"error,omitempty"`
		}{{
			SafeName: fmt.Sprintf("task-%02d", taskNumber),
			Status:   status,
			ExitCode: exitCode,
			Error:    errText,
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.WriteFile(artifacts.ResultPath, raw, 0o600); err != nil {
		return err
	}
	if status != "succeeded" {
		return nil
	}
	scope := worktree.Scope{
		Supported:     true,
		ProducedPaths: []string{fmt.Sprintf("cli-task-%02d.txt", taskNumber)},
	}
	scopePath := artifacts.JobArtifacts(fmt.Sprintf("task-%02d", taskNumber)).WorktreeScopePath
	return worktree.WriteScope(scopePath, scope)
}

func hasRunEventKind(events []eventspkg.Event, kind eventspkg.EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func eventKindsFromCoreEvents(events []eventspkg.Event) []eventspkg.EventKind {
	kinds := make([]eventspkg.EventKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
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

func assertNoCompozyTaskFilesTrackedForCLI(t *testing.T, root string) {
	t.Helper()
	names := runGitOutputForCLITests(t, root, "ls-tree", "-r", "--name-only", "HEAD")
	if strings.Contains(names, ".compozy/tasks") {
		t.Fatalf("parallel task fixture must keep .compozy/tasks untracked, got tree:\n%s", names)
	}
}

func runGitForCLITests(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = runGitOutputForCLITests(t, dir, args...)
}

func runGitOutputForCLITests(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

func assertNoCLIWorktreesUnderRoot(t *testing.T, repo string, worktreesRoot string) {
	t.Helper()
	list := runGitOutputForCLITests(t, repo, "worktree", "list", "--porcelain")
	for _, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if strings.HasPrefix(path, worktreesRoot) {
			t.Fatalf("found leaked Compozy worktree under %s in list:\n%s", worktreesRoot, list)
		}
	}
}

func containsCLIString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
