package cli

import (
	"bytes"
	"context"
	"os"
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

func executeTasksSyncCommand(t *testing.T) string {
	t.Helper()
	var output bytes.Buffer
	command := newTasksSyncCommand()
	command.SetArgs([]string{})
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("tasks sync error = %v\noutput:\n%s", err, output.String())
	}
	return output.String()
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
