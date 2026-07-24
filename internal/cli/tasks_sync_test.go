package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Suite: task-group completion sync command
// Invariant: the CLI projects authoritative DB completion into current-workspace plans.
// Boundary IN: real Cobra command, workspace discovery, SQLite, and filesystem.
// Boundary OUT: daemon automatic hydration.
func TestTasksSyncCommand(t *testing.T) {
	t.Run("tasks command registers the sync surface", func(t *testing.T) {
		command, _, err := NewRootCommand().Find([]string{"tasks", "sync"})
		if err != nil {
			t.Fatalf("Find(tasks sync) error = %v", err)
		}
		if command == nil || command.Use != "sync [initiative]" {
			t.Fatalf("tasks sync command = %#v", command)
		}
	})

	t.Run("E2E-015 no-argument sync updates every current-workspace plan", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv(compozyconfig.HomeEnvVar, homeDir)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		writeTasksSyncPlan(t, workspaceRoot, "initiative", []string{"TG-001", "TG-002"})
		seedTasksSyncCompletion(
			t,
			workspaceRoot,
			"initiative",
			[]string{"TG-001", "TG-002"},
		)
		t.Chdir(workspaceRoot)

		firstOutput := executeTasksSyncCommand(t)
		for _, want := range []string{
			"Initiatives checked: 1",
			"Newly marked: 2",
			"initiative: marked TG-001, TG-002",
		} {
			if !strings.Contains(firstOutput, want) {
				t.Fatalf("first sync output = %q, want %q", firstOutput, want)
			}
		}
		assertTasksSyncCompletion(
			t,
			workspaceRoot,
			"initiative",
			[]string{"TG-001", "TG-002"},
		)

		secondOutput := executeTasksSyncCommand(t)
		for _, want := range []string{
			"Newly marked: 0",
			"initiative: up to date",
		} {
			if !strings.Contains(secondOutput, want) {
				t.Fatalf("second sync output = %q, want %q", secondOutput, want)
			}
		}
	})

	t.Run("explicit initiative limits hydration to that plan", func(t *testing.T) {
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		for _, initiative := range []string{"alpha", "beta"} {
			writeTasksSyncPlan(t, workspaceRoot, initiative, []string{"TG-001"})
		}
		t.Chdir(workspaceRoot)
		var hydrated []string
		state := &tasksSyncCommandState{
			hydrate: func(_ context.Context, root, initiative string) ([]string, error) {
				gotInfo, gotErr := os.Stat(root)
				wantInfo, wantErr := os.Stat(workspaceRoot)
				if gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo) {
					t.Fatalf("workspace root = %q, want %q", root, workspaceRoot)
				}
				hydrated = append(hydrated, initiative)
				return nil, nil
			},
		}
		command := newTasksSyncCommand()
		command.RunE = state.run
		command.SetArgs([]string{"beta"})
		command.SetOut(&bytes.Buffer{})

		if err := command.Execute(); err != nil {
			t.Fatalf("tasks sync beta error = %v", err)
		}
		if !slices.Equal(hydrated, []string{"beta"}) {
			t.Fatalf("hydrated initiatives = %v, want [beta]", hydrated)
		}
	})
}

// Suite: task-group completion sync across sibling git worktrees
// Invariant: `tasks sync` inherits the read-side union, marking sibling
// completions once while excluding unrelated repositories.
// Boundary IN: real Cobra command, real on-disk git worktrees, real SQLite home DB.
// Boundary OUT: daemon preflight dispatch (covered in task_03).
func TestTasksSyncCommandSiblingUnion(t *testing.T) {
	t.Run("E2E-001 sibling completion marks once and re-sync is a no-op", func(t *testing.T) {
		t.Setenv(compozyconfig.HomeEnvVar, t.TempDir())
		repoParent := t.TempDir()
		primaryA := filepath.Join(repoParent, "A")
		if err := os.MkdirAll(primaryA, 0o755); err != nil {
			t.Fatalf("create primary workspace: %v", err)
		}
		initTasksSyncGitRepo(t, primaryA)
		writeTasksSyncPlan(t, primaryA, "initiative", []string{"TG-001", "TG-002"})
		siblingB := filepath.Join(repoParent, "B")
		runTasksSyncGit(t, primaryA, "worktree", "add", "--detach", siblingB)
		// A registers lazily on read; only sibling B carries the TG-002 completion.
		seedTasksSyncCompletion(t, siblingB, "initiative", []string{"TG-002"})
		t.Chdir(primaryA)

		planPath := filepath.Join(
			model.TasksBaseDirForWorkspace(primaryA), "initiative", taskgroups.ManifestFileName)
		firstOutput := executeTasksSyncArgs(t, "initiative")
		for _, want := range []string{"Newly marked: 1", "initiative: marked TG-002"} {
			if !strings.Contains(firstOutput, want) {
				t.Fatalf("first sync output = %q, want %q", firstOutput, want)
			}
		}
		assertTasksSyncCompletion(t, primaryA, "initiative", []string{"TG-002"})
		afterFirst := mustReadTasksSyncFile(t, planPath)

		secondOutput := executeTasksSyncArgs(t, "initiative")
		for _, want := range []string{"Newly marked: 0", "initiative: up to date"} {
			if !strings.Contains(secondOutput, want) {
				t.Fatalf("second sync output = %q, want %q", secondOutput, want)
			}
		}
		if got := mustReadTasksSyncFile(t, planPath); got != afterFirst {
			t.Fatalf("second sync changed the plan file")
		}
	})

	t.Run("E2E-002 unrelated repository never leaks into the plan", func(t *testing.T) {
		t.Setenv(compozyconfig.HomeEnvVar, t.TempDir())
		primaryA := filepath.Join(t.TempDir(), "A")
		if err := os.MkdirAll(primaryA, 0o755); err != nil {
			t.Fatalf("create primary workspace: %v", err)
		}
		initTasksSyncGitRepo(t, primaryA)
		writeTasksSyncPlan(t, primaryA, "initiative", []string{"TG-001", "TG-002"})
		// An unrelated, separately-initialized repository reusing the initiative name.
		unrelated := filepath.Join(t.TempDir(), "other")
		if err := os.MkdirAll(unrelated, 0o755); err != nil {
			t.Fatalf("create unrelated workspace: %v", err)
		}
		initTasksSyncGitRepo(t, unrelated)
		seedTasksSyncCompletion(t, unrelated, "initiative", []string{"TG-002"})
		t.Chdir(primaryA)

		output := executeTasksSyncArgs(t, "initiative")
		if !strings.Contains(output, "Newly marked: 0") {
			t.Fatalf("sync output = %q, want no newly marked task groups", output)
		}
		assertTasksSyncCompletion(t, primaryA, "initiative", nil)
	})
}

func executeTasksSyncCommand(t *testing.T) string {
	t.Helper()
	return executeTasksSyncArgs(t)
}

func executeTasksSyncArgs(t *testing.T, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	command := newTasksSyncCommand()
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("tasks sync %v error = %v\noutput:\n%s", args, err, output.String())
	}
	return output.String()
}

func runTasksSyncGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(context.Background(), "git", cmdArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, strings.TrimSpace(string(output)))
	}
}

func initTasksSyncGitRepo(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("sync fixture\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runTasksSyncGit(t, root, "init", "--initial-branch=main")
	runTasksSyncGit(t, root, "config", "user.email", "sync@example.com")
	runTasksSyncGit(t, root, "config", "user.name", "Sync Test")
	runTasksSyncGit(t, root, "add", "README.md")
	runTasksSyncGit(t, root, "commit", "-m", "initial")
}

func mustReadTasksSyncFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func writeTasksSyncPlan(
	t *testing.T,
	workspaceRoot, initiative string,
	taskGroupIDs []string,
) {
	t.Helper()
	groups := make([]taskgroups.TaskGroup, 0, len(taskGroupIDs))
	for _, taskGroupID := range taskGroupIDs {
		groups = append(groups, taskgroups.TaskGroup{
			ID:         taskGroupID,
			Title:      taskGroupID,
			Outcome:    "Deliver " + taskGroupID,
			Directory:  "_task_groups/" + taskGroupID,
			OwnedScope: []string{strings.ToLower(taskGroupID)},
		})
	}
	content, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    groups,
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	planPath := filepath.Join(
		model.TasksBaseDirForWorkspace(workspaceRoot),
		initiative,
		taskgroups.ManifestFileName,
	)
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("create tasks sync plan directory: %v", err)
	}
	if err := os.WriteFile(planPath, content, 0o600); err != nil {
		t.Fatalf("write tasks sync plan: %v", err)
	}
}

func seedTasksSyncCompletion(
	t *testing.T,
	workspaceRoot, initiative string,
	completedTaskGroupIDs []string,
) {
	t.Helper()
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	db, err := globaldb.Open(context.Background(), paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close globaldb: %v", closeErr)
		}
	}()
	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	syncedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	completed := make(map[string]struct{}, len(completedTaskGroupIDs))
	for _, taskGroupID := range completedTaskGroupIDs {
		completed[taskGroupID] = struct{}{}
	}
	children := make([]globaldb.WorkflowSyncInput, 0, len(completedTaskGroupIDs))
	for _, taskGroupID := range completedTaskGroupIDs {
		_, lifecycleCompleted := completed[taskGroupID]
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       initiative + "/" + taskGroupID,
			Kind:               globaldb.WorkflowKindTaskGroup,
			TaskGroupID:        taskGroupID,
			DisplayTitle:       taskGroupID,
			LifecycleCompleted: lifecycleCompleted,
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
		t.Fatalf("ReconcileAggregateWorkflowSync() error = %v", err)
	}
}

func assertTasksSyncCompletion(
	t *testing.T,
	workspaceRoot, initiative string,
	completedTaskGroupIDs []string,
) {
	t.Helper()
	plan, err := taskgroups.NewStore().Load(
		context.Background(),
		filepath.Join(model.TasksBaseDirForWorkspace(workspaceRoot), initiative),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := make([]string, 0, len(plan.TaskGroups))
	for index := range plan.TaskGroups {
		if plan.TaskGroups[index].Completed {
			got = append(got, plan.TaskGroups[index].ID)
		}
	}
	if !slices.Equal(got, completedTaskGroupIDs) {
		t.Fatalf("completed task groups = %v, want %v", got, completedTaskGroupIDs)
	}
}
