package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Suite: `compozy tasks run` dependent dispatch across sibling git worktrees
// (completion-worktree-union feature, task_03).
// Invariant: a prerequisite completed in a sibling worktree satisfies dispatch
// readiness via the unioned read, while a genuinely unmet dependency still blocks
// and --allow-out-of-order authorizes the run (ADR-003).
// Boundary IN: real Cobra `tasks run`/`tasks sync`, in-process daemon, real SQLite
// home DB, and real on-disk git worktrees.
// Boundary OUT: real agent execution (stubbed to a trivial success).

// TestTasksRunDependentDispatchSatisfiedViaUnion is E2E-003: with TG-001 completed
// in sibling worktree B, `compozy tasks run <initiative>/TG-002` in A is not blocked
// on dependencies — readiness is satisfied via the union projected into A's plan.
func TestTasksRunDependentDispatchSatisfiedViaUnion(t *testing.T) {
	requireGitForCLITests(t)

	const initiative = "cli-union-dispatch"
	client, _, primaryRoot := newDependentTaskGroupsCLIEnv(t, initiative)

	// Sibling B is a real linked worktree of A that carries the TG-001 completion.
	sibling := filepath.Join(t.TempDir(), "sibling-b")
	runGitForCLITests(t, primaryRoot, "worktree", "add", "--detach", sibling, "HEAD")
	seedTaskGroupCompletionRowsForCLI(t, client.globalDB, sibling, initiative, []string{"TG-001"})

	// Reconcile A's plan through the shared union read (tasks sync), the documented
	// reconciliation path (ADR-002). This marks TG-001 in A's own _task_groups.md.
	syncStdout, syncStderr, err := runParallelMultiRunCLI(t, "tasks", "sync", initiative)
	if err != nil {
		t.Fatalf("tasks sync: %v\nstdout:\n%s\nstderr:\n%s", err, syncStdout, syncStderr)
	}
	if !containsAll(syncStdout, "Newly marked: 1", initiative+": marked TG-001") {
		t.Fatalf("tasks sync did not project the sibling completion:\n%s", syncStdout)
	}

	stdout, stderr, err := runParallelMultiRunCLI(
		t,
		"tasks", "run", initiative+"/TG-002", "--stream", "--dry-run",
	)
	if err != nil {
		t.Fatalf("dependent run blocked despite sibling completion: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout, stderr)
	}
	if strings.Contains(stdout, "dependencies are not complete") ||
		strings.Contains(stderr, "dependencies are not complete") {
		t.Fatalf("dependent run was blocked on dependencies:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !containsAll(stdout, "task run started:", "run completed") {
		t.Fatalf("dependent run did not reach a completed terminal:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

// TestTasksRunDependentDispatchUnmetThenOverride is E2E-004: with TG-001 completed
// in no worktree, `compozy tasks run <initiative>/TG-002` in A is rejected with the
// dependencies-unmet problem; re-running with --allow-out-of-order authorizes it.
func TestTasksRunDependentDispatchUnmetThenOverride(t *testing.T) {
	requireGitForCLITests(t)

	const initiative = "cli-union-unmet"
	newDependentTaskGroupsCLIEnv(t, initiative)

	blockedStdout, blockedStderr, err := runParallelMultiRunCLI(
		t,
		"tasks", "run", initiative+"/TG-002", "--stream", "--dry-run",
	)
	if err == nil {
		t.Fatalf("unmet dependency run was not rejected:\n%s\nstderr:\n%s", blockedStdout, blockedStderr)
	}
	if !strings.Contains(err.Error(), "dependencies are not complete") &&
		!strings.Contains(blockedStderr, "dependencies are not complete") {
		t.Fatalf("rejection did not cite unmet dependencies: %v\nstdout:\n%s\nstderr:\n%s",
			err, blockedStdout, blockedStderr)
	}
	if strings.Contains(blockedStdout, "task run started:") {
		t.Fatalf("unmet dependency run started despite the block:\n%s", blockedStdout)
	}

	stdout, stderr, err := runParallelMultiRunCLI(
		t,
		"tasks", "run", initiative+"/TG-002", "--allow-out-of-order", "--stream", "--dry-run",
	)
	if err != nil {
		t.Fatalf("--allow-out-of-order run was not authorized: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout, stderr)
	}
	if !containsAll(stdout, "task run started:", "run completed") {
		t.Fatalf("authorized run did not reach a completed terminal:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

// newDependentTaskGroupsCLIEnv builds a committed git workspace whose initiative has
// TG-001 and a TG-002 that depends on it, wires an in-process daemon with trivial
// success stubs, and returns the daemon client and resolved paths.
func newDependentTaskGroupsCLIEnv(
	t *testing.T,
	initiative string,
) (*inProcessDaemonCommandClient, compozyconfig.HomePaths, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	writeDependentTaskGroupsCLIWorkspace(t, workspaceRoot, initiative)
	gitInitCommitCLIWorkspace(t, workspaceRoot)

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

// writeDependentTaskGroupsCLIWorkspace writes an initiative plan with TG-001 and a
// TG-002 that depends on TG-001, plus a task file per group.
func writeDependentTaskGroupsCLIWorkspace(t *testing.T, workspaceRoot, initiative string) {
	t.Helper()
	writeParallelTasksGitignoreForCLI(t, workspaceRoot)
	initiativeRoot := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative)
	if err := os.MkdirAll(initiativeRoot, 0o755); err != nil {
		t.Fatalf("mkdir initiative root: %v", err)
	}
	groups := []taskgroups.TaskGroup{
		{
			ID:         "TG-001",
			Title:      "CLI prerequisite TG-001",
			Outcome:    "Produce TG-001",
			Directory:  "_task_groups/TG-001",
			OwnedScope: []string{"tg-001.txt"},
		},
		{
			ID:         "TG-002",
			Title:      "CLI dependent TG-002",
			Outcome:    "Produce TG-002",
			Directory:  "_task_groups/TG-002",
			OwnedScope: []string{"tg-002.txt"},
		},
	}
	for i := range groups {
		group := &groups[i]
		groupRoot := filepath.Join(initiativeRoot, group.Directory)
		if err := os.MkdirAll(groupRoot, 0o755); err != nil {
			t.Fatalf("mkdir task group %s: %v", group.ID, err)
		}
		writeRawTaskFileForCLI(t, groupRoot, "task_01.md", cliTaskMarkdown(
			[]string{
				"status: pending",
				"title: Execute " + group.ID,
				"type: backend",
				"complexity: low",
			},
			"# Task 1: Execute "+group.ID,
		))
	}
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    groups,
		Edges: []taskgroups.Dependency{{
			From:      "TG-001",
			To:        "TG-002",
			Rationale: "TG-002 depends on TG-001",
		}},
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	writeRawTaskFileForCLI(t, initiativeRoot, "_prd.md", "# CLI dependent task groups\n")
	writeRawTaskFileForCLI(t, initiativeRoot, "_techspec.md", "# CLI dependent techspec\n")
	writeRawTaskFileForCLI(t, initiativeRoot, "_task_groups.md", string(plan))
}

// seedTaskGroupCompletionRowsForCLI registers workspaceRoot and marks each listed
// task group completed for the initiative, using the provided (daemon-owned) DB
// handle so no second SQLite connection is required.
func seedTaskGroupCompletionRowsForCLI(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceRoot, initiative string,
	completedTaskGroupIDs []string,
) {
	t.Helper()
	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%s) error = %v", workspaceRoot, err)
	}
	syncedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	children := make([]globaldb.WorkflowSyncInput, 0, len(completedTaskGroupIDs))
	for _, taskGroupID := range completedTaskGroupIDs {
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       initiative + "/" + taskGroupID,
			Kind:               globaldb.WorkflowKindTaskGroup,
			TaskGroupID:        taskGroupID,
			DisplayTitle:       taskGroupID,
			LifecycleCompleted: true,
			SyncedAt:           syncedAt,
			CheckpointScope:    "workflow",
		})
	}
	if _, err := db.ReconcileAggregateWorkflowSync(
		context.Background(),
		globaldb.AggregateWorkflowSyncInput{
			Parent: globaldb.WorkflowSyncInput{
				WorkspaceID:     workspace.ID,
				WorkflowSlug:    initiative,
				Kind:            globaldb.WorkflowKindInitiative,
				DisplayTitle:    initiative,
				SyncedAt:        syncedAt,
				CheckpointScope: "workflow",
			},
			Children: children,
		},
	); err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(%s) error = %v", workspaceRoot, err)
	}
}
