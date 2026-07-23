package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Suite: task-group completion hydration
// Invariant: globaldb completion is projected additively and atomically exactly once.
// Boundary IN: real SQLite registry, completion store, lock, and filesystem writer.
// Boundary OUT: daemon trigger ordering and CLI presentation.
func TestHydratePlanCompletion(t *testing.T) {
	t.Run("UT-020 marks every completed task group", func(t *testing.T) {
		workspaceRoot, db := newCompletionHydrationFixture(t, []string{"TG-001", "TG-002"})

		marked, err := HydratePlanCompletionWithDB(
			context.Background(),
			db,
			workspaceRoot,
			"initiative",
		)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB() error = %v", err)
		}
		if !reflect.DeepEqual(marked, []string{"TG-001", "TG-002"}) {
			t.Fatalf("marked = %v, want both task groups", marked)
		}
		assertHydratedPlanState(t, workspaceRoot, true, true)
	})

	t.Run("UT-021 never clears a manually checked task group", func(t *testing.T) {
		workspaceRoot, db := newCompletionHydrationFixture(t, []string{"TG-001"})
		planPath := completionHydrationPlanPath(workspaceRoot)
		content := strings.Replace(
			mustReadFile(t, planPath),
			"## [ ] TG-002",
			"## [x] TG-002",
			1,
		)
		if err := os.WriteFile(planPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write manually completed plan: %v", err)
		}

		marked, err := HydratePlanCompletionWithDB(
			context.Background(),
			db,
			workspaceRoot,
			"initiative",
		)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB() error = %v", err)
		}
		if !reflect.DeepEqual(marked, []string{"TG-001"}) {
			t.Fatalf("marked = %v, want TG-001 only", marked)
		}
		assertHydratedPlanState(t, workspaceRoot, true, true)
	})

	t.Run("UT-022 second hydration is idempotent", func(t *testing.T) {
		workspaceRoot, db := newCompletionHydrationFixture(t, []string{"TG-001", "TG-002"})
		if _, err := HydratePlanCompletionWithDB(
			context.Background(),
			db,
			workspaceRoot,
			"initiative",
		); err != nil {
			t.Fatalf("HydratePlanCompletionWithDB(first) error = %v", err)
		}

		marked, err := HydratePlanCompletionWithDB(
			context.Background(),
			db,
			workspaceRoot,
			"initiative",
		)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB(second) error = %v", err)
		}
		if len(marked) != 0 {
			t.Fatalf("second marked = %v, want empty", marked)
		}
	})

	t.Run("UT-023 missing plan skips cleanly", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
			t.Fatalf("create workspace marker: %v", err)
		}
		db := openCompletionHydrationDB(t)
		workspace := registerCompletionHydrationWorkspace(t, db, workspaceRoot)
		seedCompletionHydrationRows(t, db, workspace, []string{"TG-001"})

		marked, err := HydratePlanCompletionWithDB(
			context.Background(),
			db,
			workspaceRoot,
			"initiative",
		)
		if err != nil {
			t.Fatalf("HydratePlanCompletionWithDB() error = %v", err)
		}
		if len(marked) != 0 {
			t.Fatalf("marked = %v, want empty", marked)
		}
	})

	t.Run("UT-024 concurrent hydration has no lost update", func(t *testing.T) {
		workspaceRoot, db := newCompletionHydrationFixture(t, []string{"TG-001", "TG-002"})
		start := make(chan struct{})
		results := make(chan []string, 2)
		errs := make(chan error, 2)
		var workers sync.WaitGroup
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				marked, err := HydratePlanCompletionWithDB(
					context.Background(),
					db,
					workspaceRoot,
					"initiative",
				)
				results <- marked
				errs <- err
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		close(errs)

		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent HydratePlanCompletionWithDB() error = %v", err)
			}
		}
		totalMarked := 0
		for marked := range results {
			totalMarked += len(marked)
		}
		if totalMarked != 2 {
			t.Fatalf("total newly marked = %d, want exactly 2", totalMarked)
		}
		assertHydratedPlanState(t, workspaceRoot, true, true)
		content := mustReadFile(t, completionHydrationPlanPath(workspaceRoot))
		if strings.Count(content, "## [x] TG-001") != 1 ||
			strings.Count(content, "## [x] TG-002") != 1 {
			t.Fatalf("concurrent hydration corrupted plan:\n%s", content)
		}
	})
}

// Suite: task-group completion persistence ordering
// Invariant: no catalog completion precedes a durable file mark; retries converge.
// Boundary IN: completion service, real plan files, and real SQLite synchronization.
// Boundary OUT: daemon follow-on projection, covered by daemon hydration tests.
func TestTaskGroupCompletionPersistenceOrdering(t *testing.T) {
	t.Run("IT-020 concurrent scoped sync preserves both completion rows", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupCompletionFixture(
			t,
			initiativeDir,
			map[string]string{"TG-001": "completed", "TG-002": "completed"},
		)
		db, workspace := openCompletionHydrationHomeDB(t, workspaceRoot)
		scopes := []model.ExecutionScope{
			resolveCompletionHydrationScope(t, workspaceRoot, "initiative/TG-001"),
			resolveCompletionHydrationScope(t, workspaceRoot, "initiative/TG-002"),
		}
		start := make(chan struct{})
		errs := make(chan error, len(scopes))
		var workers sync.WaitGroup
		for _, scope := range scopes {
			scope := scope
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				errs <- syncCompletedTaskGroupInitiative(context.Background(), workspaceRoot, scope)
			}()
		}
		close(start)
		workers.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent completion sync error = %v", err)
			}
		}

		workflows, err := db.ListWorkflows(
			context.Background(),
			globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID},
		)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		completed, err := completedTaskGroupIDsForInitiative(workflows, "initiative")
		if err != nil {
			t.Fatalf("completedTaskGroupIDsForInitiative() error = %v", err)
		}
		if !reflect.DeepEqual(completed, []string{"TG-001", "TG-002"}) {
			t.Fatalf("completed rows = %v, want both task groups", completed)
		}
	})

	t.Run("IT-030 file failure prevents globaldb sync and preserves readiness", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupCompletionFixture(
			t,
			initiativeDir,
			map[string]string{"TG-001": "pending", "TG-002": "pending"},
		)
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskGraphTaskBody("completed", "TG-001 task"),
		)
		db, workspace := openCompletionHydrationHomeDB(t, workspaceRoot)
		seedCompletionHydrationRows(t, db, workspace, nil)
		beforePlan, err := taskgroups.NewStore().Load(context.Background(), initiativeDir)
		if err != nil {
			t.Fatalf("Load(before) error = %v", err)
		}
		beforeReadiness, err := taskgroups.EvaluateReadiness(beforePlan, "TG-002")
		if err != nil {
			t.Fatalf("EvaluateReadiness(before) error = %v", err)
		}
		syncCalled := false
		service := NewTaskGroupCompletionService()
		service.store = completionStoreFunc(
			func(
				context.Context,
				string,
				string,
				taskgroups.CompletionValidator,
			) (taskgroups.CompletionResult, error) {
				return taskgroups.CompletionResult{}, taskgroups.ErrPlanReadOnly
			},
		)
		service.sync = func(ctx context.Context, root string, scope model.ExecutionScope) error {
			syncCalled = true
			return syncCompletedTaskGroupInitiative(ctx, root, scope)
		}

		result, err := service.Complete(context.Background(), TaskGroupCompletionRequest{
			WorkspaceRoot:      workspaceRoot,
			Reference:          "initiative/TG-001",
			VerificationPassed: true,
		})
		if !errors.Is(err, taskgroups.ErrPlanReadOnly) {
			t.Fatalf("Complete() error = %v, want ErrPlanReadOnly", err)
		}
		if syncCalled {
			t.Fatal("globaldb sync ran after file failure")
		}
		if result.CompletionRecorded {
			t.Fatalf("CompletionRecorded = true after file failure: %#v", result)
		}
		afterPlan, loadErr := taskgroups.NewStore().Load(context.Background(), initiativeDir)
		if loadErr != nil {
			t.Fatalf("Load(after) error = %v", loadErr)
		}
		afterReadiness, readinessErr := taskgroups.EvaluateReadiness(afterPlan, "TG-002")
		if readinessErr != nil {
			t.Fatalf("EvaluateReadiness(after) error = %v", readinessErr)
		}
		if !reflect.DeepEqual(afterReadiness, beforeReadiness) {
			t.Fatalf("readiness changed after file failure\nbefore=%#v\nafter=%#v", beforeReadiness, afterReadiness)
		}
		workflows, listErr := db.ListWorkflows(
			context.Background(),
			globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID},
		)
		if listErr != nil {
			t.Fatalf("ListWorkflows() error = %v", listErr)
		}
		completed, completedErr := completedTaskGroupIDsForInitiative(workflows, "initiative")
		if completedErr != nil {
			t.Fatalf("completedTaskGroupIDsForInitiative() error = %v", completedErr)
		}
		if len(completed) != 0 {
			t.Fatalf("completed rows after file failure = %v, want none", completed)
		}
	})

	t.Run("IT-031 sync failure reports pending and retry converges", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		setSyncTestHome(t)
		initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
		writeTaskGroupCompletionFixture(
			t,
			initiativeDir,
			map[string]string{"TG-001": "pending", "TG-002": "pending"},
		)
		writeSyncWorkflowFile(
			t,
			filepath.Join(initiativeDir, "_task_groups", "TG-001"),
			"task_01.md",
			taskGraphTaskBody("completed", "TG-001 task"),
		)
		service := NewTaskGroupCompletionService()
		syncAttempts := 0
		service.sync = func(ctx context.Context, root string, scope model.ExecutionScope) error {
			syncAttempts++
			if syncAttempts == 1 {
				return errors.New("catalog temporarily unavailable")
			}
			return syncCompletedTaskGroupInitiative(ctx, root, scope)
		}
		request := TaskGroupCompletionRequest{
			WorkspaceRoot:      workspaceRoot,
			Reference:          "initiative/TG-001",
			VerificationPassed: true,
		}

		first, err := service.Complete(context.Background(), request)
		if err == nil || !first.CompletionRecorded || !first.SyncPending {
			t.Fatalf("first completion result=%#v error=%v", first, err)
		}
		second, err := service.Complete(context.Background(), request)
		if err != nil {
			t.Fatalf("Complete(retry) error = %v", err)
		}
		if second.CompletionRecorded || !second.AlreadyCompleted || second.SyncPending {
			t.Fatalf("retry completion result = %#v", second)
		}

		db, workspace := openCompletionHydrationHomeDB(t, workspaceRoot)
		workflows, err := db.ListWorkflows(
			context.Background(),
			globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID},
		)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		completed, err := completedTaskGroupIDsForInitiative(workflows, "initiative")
		if err != nil {
			t.Fatalf("completedTaskGroupIDsForInitiative() error = %v", err)
		}
		if !reflect.DeepEqual(completed, []string{"TG-001"}) {
			t.Fatalf("completed rows = %v, want TG-001", completed)
		}
	})
}

func newCompletionHydrationFixture(
	t *testing.T,
	completedTaskGroupIDs []string,
) (string, *globaldb.GlobalDB) {
	t.Helper()
	workspaceRoot := t.TempDir()
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "initiative")
	writeTaskGroupCompletionFixture(
		t,
		initiativeDir,
		map[string]string{"TG-001": "pending", "TG-002": "pending"},
	)
	db := openCompletionHydrationDB(t)
	workspace := registerCompletionHydrationWorkspace(t, db, workspaceRoot)
	seedCompletionHydrationRows(t, db, workspace, completedTaskGroupIDs)
	return workspaceRoot, db
}

func resolveCompletionHydrationScope(
	t *testing.T,
	workspaceRoot, reference string,
) model.ExecutionScope {
	t.Helper()
	target, err := (taskgroups.TargetResolver{}).ResolveTaskGroup(
		context.Background(),
		workspaceRoot,
		reference,
	)
	if err != nil {
		t.Fatalf("ResolveTaskGroup(%s) error = %v", reference, err)
	}
	scope, err := taskgroups.BuildExecutionScope(target)
	if err != nil {
		t.Fatalf("BuildExecutionScope(%s) error = %v", reference, err)
	}
	return scope
}

func openCompletionHydrationHomeDB(
	t *testing.T,
	workspaceRoot string,
) (*globaldb.GlobalDB, globaldb.Workspace) {
	t.Helper()
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	db, err := globaldb.Open(context.Background(), homePaths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close globaldb: %v", err)
		}
	})
	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	return db, workspace
}

func openCompletionHydrationDB(t *testing.T) *globaldb.GlobalDB {
	t.Helper()
	db, err := globaldb.Open(
		context.Background(),
		filepath.Join(t.TempDir(), "global.db"),
	)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close globaldb: %v", err)
		}
	})
	return db
}

func registerCompletionHydrationWorkspace(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspaceRoot string,
) globaldb.Workspace {
	t.Helper()
	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	return workspace
}

func seedCompletionHydrationRows(
	t *testing.T,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	completedTaskGroupIDs []string,
) {
	t.Helper()
	completed := make(map[string]struct{}, len(completedTaskGroupIDs))
	for _, taskGroupID := range completedTaskGroupIDs {
		completed[taskGroupID] = struct{}{}
	}
	syncedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	children := make([]globaldb.WorkflowSyncInput, 0, 2)
	for _, taskGroupID := range []string{"TG-001", "TG-002"} {
		_, lifecycleCompleted := completed[taskGroupID]
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       "initiative/" + taskGroupID,
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
				WorkflowSlug:    "initiative",
				Kind:            globaldb.WorkflowKindInitiative,
				DisplayTitle:    "Initiative",
				SyncedAt:        syncedAt,
				CheckpointScope: "workflow",
			},
			Children: children,
		},
	); err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync() error = %v", err)
	}
}

func completionHydrationPlanPath(workspaceRoot string) string {
	return filepath.Join(
		workspaceRoot,
		".compozy",
		"tasks",
		"initiative",
		taskgroups.ManifestFileName,
	)
}

func assertHydratedPlanState(
	t *testing.T,
	workspaceRoot string,
	tg001Completed, tg002Completed bool,
) {
	t.Helper()
	plan, err := taskgroups.NewStore().Load(
		context.Background(),
		filepath.Dir(completionHydrationPlanPath(workspaceRoot)),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := plan.IsComplete("TG-001"); got != tg001Completed {
		t.Fatalf("TG-001 completion = %t, want %t", got, tg001Completed)
	}
	if got := plan.IsComplete("TG-002"); got != tg002Completed {
		t.Fatalf("TG-002 completion = %t, want %t", got, tg002Completed)
	}
}
