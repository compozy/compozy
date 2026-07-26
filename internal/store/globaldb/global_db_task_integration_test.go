//go:build integration

package globaldb

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBTaskPersistenceSurvivesReopenWithGlobalAndWorkspaceTasks(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		first,
		"task-integration-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)
	globalTask := taskRecordForTest("task-integration-global")
	globalTask.Status = taskpkg.TaskStatusDraft
	globalTask.Priority = taskpkg.PriorityLow
	globalTask.MaxAttempts = 2
	workspaceTask := taskRecordForTest("task-integration-workspace-child")
	workspaceTask.Scope = taskpkg.ScopeWorkspace
	workspaceTask.WorkspaceID = workspaceID
	workspaceTask.ParentTaskID = globalTask.ID
	workspaceTask.Priority = taskpkg.PriorityUrgent
	workspaceTask.MaxAttempts = 5
	workspaceTask.ApprovalPolicy = taskpkg.ApprovalPolicyManual
	workspaceTask.ApprovalState = taskpkg.ApprovalStateApproved
	workspaceTask.Owner = ownershipForTest(taskpkg.OwnerKindPool, "backlog")

	if err := first.CreateTask(ctx, globalTask); err != nil {
		t.Fatalf("CreateTask(global) error = %v", err)
	}
	if err := first.CreateTask(ctx, workspaceTask); err != nil {
		t.Fatalf("CreateTask(workspace) error = %v", err)
	}

	aliceTriage := taskpkg.TriageState{
		TaskID:             workspaceTask.ID,
		Actor:              taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"},
		Read:               true,
		Archived:           false,
		Dismissed:          false,
		LastSeenActivityAt: workspaceTask.UpdatedAt.Add(3 * time.Minute),
		UpdatedAt:          workspaceTask.UpdatedAt.Add(4 * time.Minute),
	}
	if err := first.UpsertTaskTriageState(ctx, aliceTriage); err != nil {
		t.Fatalf("UpsertTaskTriageState(alice) error = %v", err)
	}
	bobTriage := taskpkg.TriageState{
		TaskID:    workspaceTask.ID,
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:bob"},
		Read:      false,
		Archived:  true,
		Dismissed: true,
		UpdatedAt: workspaceTask.UpdatedAt.Add(5 * time.Minute),
	}
	if err := first.UpsertTaskTriageState(ctx, bobTriage); err != nil {
		t.Fatalf("UpsertTaskTriageState(bob) error = %v", err)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	globalTasks, err := second.ListTasks(ctx, taskpkg.Query{Scope: taskpkg.ScopeGlobal})
	if err != nil {
		t.Fatalf("ListTasks(global) error = %v", err)
	}
	if got, want := len(globalTasks), 1; got != want {
		t.Fatalf("len(ListTasks(global)) = %d, want %d", got, want)
	}
	assertTaskSummaryMatchesTask(t, &globalTasks[0], globalTask)
	if !globalTasks[0].Draft {
		t.Fatalf("globalTasks[0].Draft = false, want true")
	}

	workspaceTasks, err := second.ListTasks(ctx, taskpkg.Query{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ListTasks(workspace) error = %v", err)
	}
	if got, want := len(workspaceTasks), 1; got != want {
		t.Fatalf("len(ListTasks(workspace)) = %d, want %d", got, want)
	}
	assertTaskSummaryMatchesTask(t, &workspaceTasks[0], workspaceTask)

	reloadedTask, err := second.GetTask(ctx, workspaceTask.ID)
	if err != nil {
		t.Fatalf("GetTask(workspace) error = %v", err)
	}
	assertTaskEqual(t, reloadedTask, workspaceTask)

	storedAlice, err := second.GetTaskTriageState(ctx, workspaceTask.ID, aliceTriage.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(alice) error = %v", err)
	}
	if storedAlice != aliceTriage {
		t.Fatalf("storedAlice = %#v, want %#v", storedAlice, aliceTriage)
	}

	storedBob, err := second.GetTaskTriageState(ctx, workspaceTask.ID, bobTriage.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(bob) error = %v", err)
	}
	if storedBob != bobTriage {
		t.Fatalf("storedBob = %#v, want %#v", storedBob, bobTriage)
	}
}

func TestGlobalDBTaskRunSessionAttachmentSurvivesReopen(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	taskRecord := taskRecordForTest("task-integration-run")
	if err := first.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run := taskRunForTest("run-integration", taskRecord.ID)
	if err := first.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(queued) error = %v", err)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}

	storedQueued, err := second.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(queued) error = %v", err)
	}
	if storedQueued.SessionID != "" {
		t.Fatalf("GetTaskRun(queued).SessionID = %q, want empty", storedQueued.SessionID)
	}

	storedQueued.Status = taskpkg.TaskRunStatusRunning
	storedQueued.SessionID = "sess-persisted"
	storedQueued.StartedAt = storedQueued.QueuedAt.Add(45 * time.Second)
	storedQueued.ClaimedAt = storedQueued.QueuedAt.Add(15 * time.Second)
	storedQueued.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
	storedQueued.ClaimTokenHash = "sha256:" + strings.Repeat("b", 64)
	storedQueued.LeaseUntil = storedQueued.ClaimedAt.Add(20 * time.Minute)
	storedQueued.HeartbeatAt = storedQueued.ClaimedAt.Add(30 * time.Second)
	storedQueued.NetworkSpec = participation.LocalSpec()
	storedQueued.RequiredCapabilities = []string{"golang", "sqlite"}
	storedQueued.PreferredCapabilities = []string{"claude", "codex"}
	if err := second.UpdateTaskRun(ctx, storedQueued); err != nil {
		t.Fatalf("UpdateTaskRun(attached) error = %v", err)
	}

	if err := second.Close(ctx); err != nil {
		t.Fatalf("Close(second) error = %v", err)
	}

	third, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(third) error = %v", err)
	}
	t.Cleanup(func() {
		if err := third.Close(ctx); err != nil {
			t.Fatalf("Close(third) error = %v", err)
		}
	})

	reloadedRun, err := third.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(attached) error = %v", err)
	}
	assertTaskRunEqual(t, reloadedRun, storedQueued)

	runs, err := third.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(ListTaskRuns()) = %d, want %d", got, want)
	}
	assertTaskRunEqual(t, runs[0], storedQueued)

}

func TestGlobalDBTaskSearchFiltersAndOrderingSurviveReopen(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceA := registerWorkspaceForGlobalTests(
		t,
		first,
		"task-search-reopen-a",
		filepath.Join(t.TempDir(), "workspace-a"),
	)
	workspaceB := registerWorkspaceForGlobalTests(
		t,
		first,
		"task-search-reopen-b",
		filepath.Join(t.TempDir(), "workspace-b"),
	)

	alpha := taskRecordForTest("task-reopen-alpha")
	alpha.Scope = taskpkg.ScopeWorkspace
	alpha.WorkspaceID = workspaceA
	alpha.Status = taskpkg.TaskStatusReady
	alpha.Title = "Alpha planning"
	alpha.Identifier = "OPS-100"
	alpha.UpdatedAt = alpha.UpdatedAt.Add(time.Minute)

	beta := taskRecordForTest("task-reopen-beta")
	beta.Scope = taskpkg.ScopeWorkspace
	beta.WorkspaceID = workspaceA
	beta.Status = taskpkg.TaskStatusReady
	beta.Title = "Alpha rollout"
	beta.Identifier = "OPS-200"
	beta.CreatedAt = beta.CreatedAt.Add(2 * time.Minute)
	beta.UpdatedAt = beta.UpdatedAt.Add(2 * time.Minute)

	otherWorkspace := taskRecordForTest("task-reopen-other-workspace")
	otherWorkspace.Scope = taskpkg.ScopeWorkspace
	otherWorkspace.WorkspaceID = workspaceB
	otherWorkspace.Status = taskpkg.TaskStatusReady
	otherWorkspace.Title = "Alpha outside workspace"
	otherWorkspace.Identifier = "OPS-300"

	otherStatus := taskRecordForTest("task-reopen-other-status")
	otherStatus.Scope = taskpkg.ScopeWorkspace
	otherStatus.WorkspaceID = workspaceA
	otherStatus.Status = taskpkg.TaskStatusBlocked
	otherStatus.Title = "Alpha blocked"
	otherStatus.Identifier = "OPS-400"

	for _, record := range []taskpkg.Task{alpha, beta, otherWorkspace, otherStatus} {
		if err := first.CreateTask(ctx, record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}
	if err := first.CreateTaskRun(ctx, taskpkg.Run{
		ID:        "run-reopen-beta",
		TaskID:    beta.ID,
		Status:    taskpkg.TaskRunStatusRunning,
		Attempt:   1,
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
		QueuedAt:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		StartedAt: time.Date(2026, 4, 17, 12, 5, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	summaries, err := second.ListTasks(ctx, taskpkg.Query{
		WorkspaceID: workspaceA,
		Status:      taskpkg.TaskStatusReady,
		Search:      "alpha",
	})
	if err != nil {
		t.Fatalf("ListTasks(search+filters) error = %v", err)
	}
	gotIDs := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		gotIDs = append(gotIDs, summary.ID)
	}
	if got, want := gotIDs, []string{beta.ID, alpha.ID}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("ListTasks(search+filters) ids = %#v, want %#v", got, want)
	}
}
