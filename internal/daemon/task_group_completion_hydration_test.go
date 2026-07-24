package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Suite: daemon task-group completion hydration
// Invariant: DB completion reaches plan files before readiness reads and after termination.
// Boundary IN: RunManager hydration hooks, real SQLite, filesystem, and Git worktree registry.
// Boundary OUT: public CLI status rendering.
func TestRunManagerTaskGroupCompletionHydration(t *testing.T) {
	t.Run("IT-017 hydrate-on-run precedes preflight readiness", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeHydrationDependencyPlan(t, env.workspaceRoot, "initiative")
		seedDaemonCompletionRows(
			t,
			env.globalDB,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-001", "TG-002"},
		)
		env.manager.hydratePlanCompletion = func(
			context.Context,
			string,
			string,
		) ([]string, error) {
			return nil, nil
		}

		control, err := env.manager.resolveTaskGroupPreflightEvidence(
			context.Background(),
			env.workspaceRoot,
			"initiative/TG-005",
		)
		if err != nil {
			t.Fatalf("resolveTaskGroupPreflightEvidence(control) error = %v", err)
		}
		if control.readiness.Eligible {
			t.Fatal("control readiness eligible without hydration")
		}
		if _, err := taskGroupPreflightDecision(control, false, nil); err == nil {
			t.Fatal("control preflight accepted unmet task-group dependencies")
		}

		env.manager.hydratePlanCompletion = func(
			ctx context.Context,
			workspaceRoot, initiative string,
		) ([]string, error) {
			return corepkg.HydratePlanCompletionWithDB(
				ctx,
				env.globalDB,
				workspaceRoot,
				initiative,
			)
		}
		hydrated, err := env.manager.resolveTaskGroupPreflightEvidence(
			context.Background(),
			env.workspaceRoot,
			"initiative/TG-005",
		)
		if err != nil {
			t.Fatalf("resolveTaskGroupPreflightEvidence(hydrated) error = %v", err)
		}
		if !hydrated.readiness.Eligible {
			t.Fatalf("hydrated readiness = %#v, want eligible", hydrated.readiness)
		}
		if _, err := taskGroupPreflightDecision(hydrated, false, nil); err != nil {
			t.Fatalf("hydrated preflight rejected dependencies: %v", err)
		}
		assertDaemonHydrationPlanState(
			t,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-001", "TG-002"},
		)
	})

	t.Run("IT-019 hydrate-on-completion covers canonical and owned siblings", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		initializeHydrationGitRepository(t, env.workspaceRoot)
		writeHydrationPeerPlan(t, env.workspaceRoot, "initiative")

		siblingRoots := []string{
			filepath.Join(env.paths.WorktreesDir, "repo", "tg-003"),
			filepath.Join(env.paths.WorktreesDir, "repo", "tg-004"),
		}
		missingRoot := filepath.Join(env.paths.WorktreesDir, "repo", "missing")
		for index, root := range append(append([]string(nil), siblingRoots...), missingRoot) {
			runGitOutput(
				t,
				env.workspaceRoot,
				"worktree",
				"add",
				"--detach",
				root,
				"HEAD",
			)
			writeHydrationPeerPlan(t, root, "initiative")
			if index < len(siblingRoots) {
				seedDaemonCompletionRows(
					t,
					env.globalDB,
					root,
					"initiative",
					[]string{[]string{"TG-003", "TG-004"}[index]},
				)
			}
		}
		seedDaemonCompletionRows(
			t,
			env.globalDB,
			env.workspaceRoot,
			"initiative",
			nil,
		)
		if err := os.RemoveAll(missingRoot); err != nil {
			t.Fatalf("remove missing sibling path: %v", err)
		}
		workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister(canonical) error = %v", err)
		}
		parent := globaldb.Run{
			RunID:            "parallel-parent",
			WorkspaceID:      workspace.ID,
			Mode:             runModeTask,
			Status:           runStatusRunning,
			PresentationMode: defaultPresentationMode,
			StartedAt:        time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
		}
		if _, err := env.globalDB.PutRun(context.Background(), parent); err != nil {
			t.Fatalf("PutRun(parent) error = %v", err)
		}
		for index, siblingRoot := range siblingRoots {
			taskGroupID := []string{"TG-003", "TG-004"}[index]
			sibling, err := env.globalDB.Get(context.Background(), siblingRoot)
			if err != nil {
				t.Fatalf("Get(sibling workspace) error = %v", err)
			}
			env.manager.hydrateTaskGroupCompletionAfterRun(
				context.Background(),
				&activeRun{
					workspaceRoot: siblingRoot,
					workflowSlug:  "initiative/" + taskGroupID,
				},
				globaldb.Run{
					RunID:       "task-group-complete-" + taskGroupID,
					WorkspaceID: sibling.ID,
					ParentRunID: parent.RunID,
				},
			)
		}

		for _, root := range append([]string{env.workspaceRoot}, siblingRoots...) {
			assertDaemonHydrationPlanState(
				t,
				root,
				"initiative",
				[]string{"TG-003", "TG-004"},
			)
		}
		assertDaemonHydrationRows(
			t,
			env.globalDB,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-003", "TG-004"},
		)
	})

	t.Run("IT-032 projection failure is logged and next run reconciles", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeHydrationSimpleDependencyPlan(t, env.workspaceRoot, "initiative")
		seedDaemonCompletionRows(
			t,
			env.globalDB,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-001"},
		)
		var logs bytes.Buffer
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
		t.Cleanup(func() {
			slog.SetDefault(previousLogger)
		})
		env.manager.hydratePlanCompletion = func(
			context.Context,
			string,
			string,
		) ([]string, error) {
			return nil, errors.New("injected projection failure")
		}

		env.manager.hydrateTaskGroupPlanBestEffort(
			context.Background(),
			env.workspaceRoot,
			"initiative",
		)
		if !strings.Contains(logs.String(), "injected projection failure") {
			t.Fatalf("projection failure was not logged: %s", logs.String())
		}
		assertDaemonHydrationPlanState(t, env.workspaceRoot, "initiative", nil)

		env.manager.hydratePlanCompletion = func(
			ctx context.Context,
			workspaceRoot, initiative string,
		) ([]string, error) {
			return corepkg.HydratePlanCompletionWithDB(
				ctx,
				env.globalDB,
				workspaceRoot,
				initiative,
			)
		}
		evidence, err := env.manager.resolveTaskGroupPreflightEvidence(
			context.Background(),
			env.workspaceRoot,
			"initiative/TG-002",
		)
		if err != nil {
			t.Fatalf("resolveTaskGroupPreflightEvidence(retry) error = %v", err)
		}
		if !evidence.readiness.Eligible {
			t.Fatalf("retry readiness = %#v, want eligible", evidence.readiness)
		}
		assertDaemonHydrationPlanState(
			t,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-001"},
		)
	})

	t.Run("E2E-014 completed parallel peers unlock their dependent group", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		writeHydrationParallelDependencyPlan(t, env.workspaceRoot, "initiative")
		seedDaemonCompletionRows(
			t,
			env.globalDB,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-003", "TG-004"},
		)

		evidence, err := env.manager.resolveTaskGroupPreflightEvidence(
			context.Background(),
			env.workspaceRoot,
			"initiative/TG-005",
		)
		if err != nil {
			t.Fatalf("resolveTaskGroupPreflightEvidence() error = %v", err)
		}
		if !evidence.readiness.Eligible {
			t.Fatalf("dependent readiness = %#v, want eligible", evidence.readiness)
		}
		if _, err := taskGroupPreflightDecision(evidence, false, nil); err != nil {
			t.Fatalf("dependent preflight rejected completed peers: %v", err)
		}
		assertDaemonHydrationPlanState(
			t,
			env.workspaceRoot,
			"initiative",
			[]string{"TG-003", "TG-004"},
		)
	})
}

func writeHydrationDependencyPlan(t *testing.T, workspaceRoot, initiative string) {
	t.Helper()
	writeDaemonHydrationPlan(t, workspaceRoot, initiative, []taskgroups.TaskGroup{
		hydrationTaskGroup("TG-001"),
		hydrationTaskGroup("TG-002"),
		{
			ID:         "TG-005",
			Title:      "Dependent",
			Outcome:    "Consume prerequisites",
			Directory:  "_task_groups/TG-005",
			OwnedScope: []string{"dependent"},
			Dependencies: []taskgroups.Dependency{
				{From: "TG-001", Rationale: "uses TG-001"},
				{From: "TG-002", Rationale: "uses TG-002"},
			},
		},
	})
}

func writeHydrationPeerPlan(t *testing.T, workspaceRoot, initiative string) {
	t.Helper()
	writeDaemonHydrationPlan(t, workspaceRoot, initiative, []taskgroups.TaskGroup{
		hydrationTaskGroup("TG-003"),
		hydrationTaskGroup("TG-004"),
	})
}

func writeHydrationParallelDependencyPlan(t *testing.T, workspaceRoot, initiative string) {
	t.Helper()
	writeDaemonHydrationPlan(t, workspaceRoot, initiative, []taskgroups.TaskGroup{
		hydrationTaskGroup("TG-003"),
		hydrationTaskGroup("TG-004"),
		{
			ID:         "TG-005",
			Title:      "Dependent",
			Outcome:    "Consume parallel peers",
			Directory:  "_task_groups/TG-005",
			OwnedScope: []string{"dependent"},
			Dependencies: []taskgroups.Dependency{
				{From: "TG-003", Rationale: "uses TG-003"},
				{From: "TG-004", Rationale: "uses TG-004"},
			},
		},
	})
}

func writeHydrationSimpleDependencyPlan(t *testing.T, workspaceRoot, initiative string) {
	t.Helper()
	writeDaemonHydrationPlan(t, workspaceRoot, initiative, []taskgroups.TaskGroup{
		hydrationTaskGroup("TG-001"),
		{
			ID:         "TG-002",
			Title:      "Dependent",
			Outcome:    "Consume TG-001",
			Directory:  "_task_groups/TG-002",
			OwnedScope: []string{"dependent"},
			Dependencies: []taskgroups.Dependency{
				{From: "TG-001", Rationale: "uses TG-001"},
			},
		},
	})
}

func hydrationTaskGroup(taskGroupID string) taskgroups.TaskGroup {
	return taskgroups.TaskGroup{
		ID:         taskGroupID,
		Title:      taskGroupID,
		Outcome:    "Deliver " + taskGroupID,
		Directory:  "_task_groups/" + taskGroupID,
		OwnedScope: []string{strings.ToLower(taskGroupID)},
	}
}

func writeDaemonHydrationPlan(
	t *testing.T,
	workspaceRoot, initiative string,
	groups []taskgroups.TaskGroup,
) {
	t.Helper()
	edges := make([]taskgroups.Dependency, 0)
	for index := range groups {
		for _, dependency := range groups[index].Dependencies {
			if strings.TrimSpace(dependency.To) == "" {
				dependency.To = groups[index].ID
			}
			edges = append(edges, dependency)
		}
		groups[index].Dependencies = nil
	}
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    groups,
		Edges:         edges,
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
		t.Fatalf("create plan directory: %v", err)
	}
	if err := os.WriteFile(planPath, plan, 0o600); err != nil {
		t.Fatalf("write task-group plan: %v", err)
	}
	initiativeDir := filepath.Dir(planPath)
	for name, content := range map[string]string{
		"_prd.md":      "# Hydration PRD\n",
		"_techspec.md": "# Hydration TechSpec\n",
	} {
		if err := os.WriteFile(filepath.Join(initiativeDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for index := range groups {
		taskPath := filepath.Join(
			initiativeDir,
			groups[index].Directory,
			"task_01.md",
		)
		if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
			t.Fatalf("create task-group directory: %v", err)
		}
		if err := os.WriteFile(
			taskPath,
			[]byte(daemonTaskBody("pending", "Hydration fixture task")),
			0o600,
		); err != nil {
			t.Fatalf("write task-group task: %v", err)
		}
	}
}

func seedDaemonCompletionRows(
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
	plan, err := taskgroups.NewStore().Load(
		context.Background(),
		filepath.Join(model.TasksBaseDirForWorkspace(workspaceRoot), initiative),
	)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", workspaceRoot, err)
	}
	completed := make(map[string]struct{}, len(completedTaskGroupIDs))
	for _, taskGroupID := range completedTaskGroupIDs {
		completed[taskGroupID] = struct{}{}
	}
	syncedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	children := make([]globaldb.WorkflowSyncInput, 0, len(plan.TaskGroups))
	for index := range plan.TaskGroups {
		group := &plan.TaskGroups[index]
		_, lifecycleCompleted := completed[group.ID]
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       initiative + "/" + group.ID,
			Kind:               globaldb.WorkflowKindTaskGroup,
			TaskGroupID:        group.ID,
			DisplayTitle:       group.Title,
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
		t.Fatalf("ReconcileAggregateWorkflowSync(%s) error = %v", workspaceRoot, err)
	}
}

func assertDaemonHydrationPlanState(
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
		t.Fatalf("Load(%s) error = %v", workspaceRoot, err)
	}
	got := make([]string, 0)
	for index := range plan.TaskGroups {
		if plan.TaskGroups[index].Completed {
			got = append(got, plan.TaskGroups[index].ID)
		}
	}
	expected := append([]string(nil), completedTaskGroupIDs...)
	if !slices.Equal(got, expected) {
		t.Fatalf("completed task groups in %s = %v, want %v", workspaceRoot, got, expected)
	}
}

func assertDaemonHydrationRows(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceRoot, initiative string,
	completedTaskGroupIDs []string,
) {
	t.Helper()
	workspace, err := db.Get(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", workspaceRoot, err)
	}
	workflows, err := db.ListWorkflows(
		context.Background(),
		globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID},
	)
	if err != nil {
		t.Fatalf("ListWorkflows(%s) error = %v", workspaceRoot, err)
	}
	got := make([]string, 0)
	for index := range workflows {
		workflow := &workflows[index]
		if workflow.Kind == globaldb.WorkflowKindTaskGroup &&
			workflow.LifecycleCompleted &&
			strings.HasPrefix(workflow.Slug, initiative+"/") {
			got = append(got, workflow.TaskGroupID)
		}
	}
	if !slices.Equal(got, completedTaskGroupIDs) {
		t.Fatalf("completed DB rows in %s = %v, want %v", workspaceRoot, got, completedTaskGroupIDs)
	}
}

func initializeHydrationGitRepository(t *testing.T, workspaceRoot string) {
	t.Helper()
	readmePath := filepath.Join(workspaceRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("hydration fixture\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitOutput(t, workspaceRoot, "init", "--initial-branch=feature")
	runGitOutput(t, workspaceRoot, "config", "user.email", "hydration@example.com")
	runGitOutput(t, workspaceRoot, "config", "user.name", "Hydration Test")
	runGitOutput(t, workspaceRoot, "add", "README.md")
	runGitOutput(t, workspaceRoot, "commit", "-m", "initial")
}
