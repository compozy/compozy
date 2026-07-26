package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type recordingTaskEventCommitObserver struct {
	db      *GlobalDB
	records []taskpkg.EventRecord
	tasks   []taskpkg.Task
	err     error
}

func (o *recordingTaskEventCommitObserver) OnTaskEvent(ctx context.Context, record taskpkg.EventRecord) {
	o.records = append(o.records, record)
	if o.db == nil || o.err != nil {
		return
	}
	taskRecord, err := o.db.GetTask(ctx, record.Event.TaskID)
	if err != nil {
		o.err = err
		return
	}
	o.tasks = append(o.tasks, taskRecord)
}

func TestGlobalDBTaskEventCommitObserverShouldPublishRecoveredAfterCommit(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-recovered-commit-observer")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	markedAt := taskRecord.UpdatedAt.Add(time.Minute)
	if _, err := globalDB.MarkTaskNeedsAttention(ctx, taskpkg.NeedsAttentionMutation{
		Origin:   coordinatorActorContextForTest().Origin,
		TaskID:   taskRecord.ID,
		Reason:   "operator input required",
		Actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
		MarkedAt: markedAt,
	}); err != nil {
		t.Fatalf("MarkTaskNeedsAttention() error = %v", err)
	}

	observer := &recordingTaskEventCommitObserver{db: globalDB}
	globalDB.SetTaskEventCommitObserver(observer)
	recoveryNote := "operator reviewed escalation"
	if _, err := globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
		TaskID:    taskRecord.ID,
		Note:      recoveryNote,
		Actor:     operatorActorContextForTest("operator"),
		ClearedAt: markedAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("ClearTaskNeedsAttention() error = %v", err)
	}
	if observer.err != nil {
		t.Fatalf("observer GetTask() error = %v", observer.err)
	}
	if got, want := len(observer.records), 1; got != want {
		t.Fatalf("len(observer.records) = %d, want %d", got, want)
	}
	if got, want := observer.records[0].Event.EventType, string(hookspkg.HookTaskRecovered); got != want {
		t.Fatalf("observer event type = %q, want %q", got, want)
	}
	var payload taskRecoveredWatchEventPayload
	if err := json.Unmarshal(observer.records[0].Event.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal(recovered payload) error = %v", err)
	}
	if got, want := payload.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("recovered payload status = %q, want %q", got, want)
	}
	if got, want := payload.Note, recoveryNote; got != want {
		t.Fatalf("recovered payload note = %q, want %q", got, want)
	}
	if got, want := len(observer.tasks), 1; got != want {
		t.Fatalf("len(observer.tasks) = %d, want %d", got, want)
	}
	if observer.tasks[0].NeedsAttention != nil {
		t.Fatalf("observer task NeedsAttention = %#v, want committed nil", observer.tasks[0].NeedsAttention)
	}
}

func TestTaskExecutionCommandShouldRollbackPublishWhenEnqueuedEventFails(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	actor := operatorActorContextForTest("operator")
	actor.Authority.CreateGlobal = true
	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Atomic publish rollback",
		Draft: true,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	installTaskEventInsertFailureTriggerForType(t, globalDB, "task.run_enqueued")
	_, err = manager.PublishTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
		IdempotencyKey: "publish-rollback",
	}, actor)
	assertForcedTaskEventInsertError(t, err, "PublishTask()")

	storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := storedTask.Status, taskpkg.TaskStatusDraft; got != want {
		t.Fatalf("stored task status = %q, want rollback to %q", got, want)
	}
	runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("len(ListTaskRuns()) = %d, want 0 after rollback", len(runs))
	}
	if _, err := globalDB.GetTaskRunByIdempotencyKey(
		ctx,
		"task.publish:"+taskRecord.ID+":publish-rollback",
		actor.Origin,
	); !errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound) {
		t.Fatalf("GetTaskRunByIdempotencyKey() error = %v, want %v", err, taskpkg.ErrTaskRunIdempotencyNotFound)
	}
	events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	for _, event := range events {
		if event.EventType == "task.published" || event.EventType == "task.run_enqueued" {
			t.Fatalf("rolled-back event persisted: %q", event.EventType)
		}
	}
}

func TestGlobalDBUpdateTaskStatusShouldAppendStatusChangedEvent(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-status-changed")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	updated := taskRecord
	updated.Status = taskpkg.TaskStatusReady
	updated.UpdatedAt = taskRecord.UpdatedAt.Add(time.Minute)
	if err := globalDB.UpdateTask(ctx, updated, operatorActorContextForTest("user:alice")); err != nil {
		t.Fatalf("UpdateTask(status) error = %v", err)
	}

	event := requireTaskEventRecordForTest(t, globalDB, taskRecord.ID, string(hookspkg.HookTaskStatusChanged))
	if got, want := event.Event.Timestamp, updated.UpdatedAt; !got.Equal(want) {
		t.Fatalf("status_changed timestamp = %s, want task UpdatedAt %s", got, want)
	}
	var payload struct {
		FromStatus string `json:"from_status"`
		ToStatus   string `json:"to_status"`
	}
	if err := json.Unmarshal(event.Event.Payload, &payload); err != nil {
		t.Fatalf("Unmarshal(status_changed payload) error = %v", err)
	}
	if got, want := payload.FromStatus, string(taskpkg.TaskStatusPending); got != want {
		t.Fatalf("from_status = %q, want %q", got, want)
	}
	if got, want := payload.ToStatus, string(taskpkg.TaskStatusReady); got != want {
		t.Fatalf("to_status = %q, want %q", got, want)
	}
}

func TestGlobalDBCompleteRunLeaseShouldAppendRunCompletedWatchEvent(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-run-watch-completed")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	rawToken := "claim-token-watch-completed"
	leased := storeLeasedTaskRunForBlockTest(
		ctx,
		t,
		globalDB,
		taskRecord.ID,
		"run-watch-completed",
		"sess-watch-completed",
		rawToken,
		time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC),
	)

	if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		Actor:      coordinatorActorContextForTest(),
		RunID:      leased.ID,
		ClaimToken: rawToken,
		Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
		Now:        leased.LeaseUntil.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}

	event := requireTaskEventRecordForTest(t, globalDB, taskRecord.ID, string(hookspkg.HookTaskRunCompleted))
	if got, want := event.Event.RunID, leased.ID; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
}

func TestGlobalDBTaskStatusProjectionShouldCommitWithTransition(t *testing.T) {
	t.Parallel()

	t.Run("Should freeze transition state and exact recipients in one transaction", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord, transitionAt := seedTaskStatusProjectionForTest(ctx, t, globalDB, "snapshot")

		if _, err := globalDB.MarkTaskNeedsAttention(ctx, taskpkg.NeedsAttentionMutation{
			Origin: coordinatorActorContextForTest().Origin, TaskID: taskRecord.ID,
			Reason:   "operator input required",
			Actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
			MarkedAt: transitionAt.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("MarkTaskNeedsAttention() error = %v", err)
		}
		if _, err := globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
			TaskID: taskRecord.ID, Note: "operator reviewed escalation",
			Actor: operatorActorContextForTest("operator"), ClearedAt: transitionAt,
		}); err != nil {
			t.Fatalf("ClearTaskNeedsAttention() error = %v", err)
		}

		projections, err := globalDB.ListNetworkTaskStatusProjections(ctx, store.NetworkTaskStatusProjectionQuery{
			WorkspaceID: taskRecord.WorkspaceID, TaskID: taskRecord.ID, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListNetworkTaskStatusProjections() error = %v", err)
		}
		if got, want := len(projections), 1; got != want {
			t.Fatalf("len(projections) = %d, want %d exact full recipient", got, want)
		}
		projection := projections[0]
		if got, want := projection.RecipientSessionID, "sess-full-snapshot"; got != want {
			t.Fatalf("recipient = %q, want %q", got, want)
		}
		if !projection.ProjectedAt.Equal(transitionAt) {
			t.Fatalf("projected_at = %s, want transition time %s", projection.ProjectedAt, transitionAt)
		}
		var payload store.NetworkTaskStatusProjectionPayload
		if err := json.Unmarshal(projection.ProjectionJSON, &payload); err != nil {
			t.Fatalf("Unmarshal(projection) error = %v", err)
		}
		if got, want := payload.TaskStatus, string(taskpkg.TaskStatusReady); got != want {
			t.Fatalf("projection task_status = %q, want %q", got, want)
		}

		for _, subscription := range []store.NetworkSubscriptionEntry{
			{
				WorkspaceID: taskRecord.WorkspaceID, Channel: "builders", ThreadID: "thread_snapshot",
				SessionID: "sess-full-snapshot", Mode: store.NetworkSubscriptionModeMute,
				CreatedAt: transitionAt, UpdatedAt: transitionAt.Add(time.Minute),
			},
			{
				WorkspaceID: taskRecord.WorkspaceID, Channel: "builders", ThreadID: "thread_snapshot",
				SessionID: "sess-muted-snapshot", Mode: store.NetworkSubscriptionModeFull,
				CreatedAt: transitionAt, UpdatedAt: transitionAt.Add(time.Minute),
			},
		} {
			if err := globalDB.PutNetworkSubscription(ctx, subscription); err != nil {
				t.Fatalf("PutNetworkSubscription(%q) error = %v", subscription.SessionID, err)
			}
		}
		projections, err = globalDB.ListNetworkTaskStatusProjections(ctx, store.NetworkTaskStatusProjectionQuery{
			WorkspaceID: taskRecord.WorkspaceID, TaskID: taskRecord.ID, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListNetworkTaskStatusProjections(after subscription change) error = %v", err)
		}
		if got, want := projections[0].RecipientSessionID, "sess-full-snapshot"; got != want {
			t.Fatalf("frozen recipient = %q, want original %q", got, want)
		}
	})

	t.Run("Should roll back the transition when projection persistence fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord, transitionAt := seedTaskStatusProjectionForTest(ctx, t, globalDB, "rollback")
		if _, err := globalDB.db.ExecContext(ctx, `CREATE TRIGGER fail_task_status_projection
			BEFORE INSERT ON network_task_status_projections
			BEGIN SELECT RAISE(FAIL, 'forced projection failure'); END`); err != nil {
			t.Fatalf("install projection failure trigger error = %v", err)
		}

		if _, err := globalDB.MarkTaskNeedsAttention(ctx, taskpkg.NeedsAttentionMutation{
			Origin: coordinatorActorContextForTest().Origin, TaskID: taskRecord.ID,
			Reason:   "must roll back",
			Actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
			MarkedAt: transitionAt.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("MarkTaskNeedsAttention() error = %v", err)
		}
		_, err := globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
			TaskID: taskRecord.ID, Note: "must fail",
			Actor: operatorActorContextForTest("operator"), ClearedAt: transitionAt,
		})
		if err == nil || !strings.Contains(err.Error(), "forced projection failure") {
			t.Fatalf("ClearTaskNeedsAttention() error = %v, want forced projection failure", err)
		}
		stored, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.NeedsAttention == nil || stored.Status != taskRecord.Status {
			t.Fatalf("stored task = %#v, want uncleared attention state", stored)
		}
		events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: taskRecord.ID, EventType: string(hookspkg.HookTaskRecovered),
		})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("needs-attention events = %#v, want rollback", events)
		}
	})
}

func seedTaskStatusProjectionForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
) (taskpkg.Task, time.Time) {
	t.Helper()
	now := time.Date(2026, 7, 17, 16, 0, 0, 0, time.UTC)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "projection-"+suffix, t.TempDir())
	for _, sessionID := range []string{"sess-origin-" + suffix, "sess-full-" + suffix, "sess-muted-" + suffix} {
		if err := globalDB.RegisterSession(ctx, store.SessionInfo{
			ID: sessionID, AgentName: "agent-" + sessionID, WorkspaceID: workspaceID,
			SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
			SessionType:         "agent", State: "running", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
		}
	}
	if err := globalDB.WriteNetworkChannel(ctx, store.NetworkChannelEntry{
		WorkspaceID: workspaceID, Channel: "builders", Purpose: "Task projection",
		CreatedBy: "test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}
	threadID := "thread_" + suffix
	messageID := "msg-origin-" + suffix
	if _, err := globalDB.WriteConversationMessage(ctx, store.NetworkConversationMessage{
		MessageID: messageID, SessionID: "sess-origin-" + suffix, WorkspaceID: workspaceID,
		Channel: "builders", Surface: store.NetworkSurfaceThread, ThreadID: threadID,
		Direction: "received", PeerFrom: "agent.origin", Kind: store.NetworkKindSay,
		Text: "promote this thread", Body: json.RawMessage(`{"text":"promote this thread"}`),
		SizeBytes: 20, Timestamp: now,
	}); err != nil {
		t.Fatalf("WriteConversationMessage() error = %v", err)
	}
	taskRecord := taskRecordForTest("task-projection-" + suffix)
	taskRecord.Scope = taskpkg.ScopeWorkspace
	taskRecord.WorkspaceID = workspaceID
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := globalDB.PutNetworkTaskThreadOrigin(ctx, store.NetworkTaskThreadOrigin{
		TaskID: taskRecord.ID, WorkspaceID: workspaceID, Channel: "builders", ThreadID: threadID,
		OriginMessageID: messageID, Digest: "sha256:projection-" + suffix,
		SourceMessageIDs: []string{messageID}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("PutNetworkTaskThreadOrigin() error = %v", err)
	}
	for _, subscription := range []store.NetworkSubscriptionEntry{
		{
			WorkspaceID: workspaceID, Channel: "builders", ThreadID: threadID,
			SessionID: "sess-full-" + suffix, Mode: store.NetworkSubscriptionModeFull,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			WorkspaceID: workspaceID, Channel: "builders", ThreadID: threadID,
			SessionID: "sess-muted-" + suffix, Mode: store.NetworkSubscriptionModeMute,
			CreatedAt: now, UpdatedAt: now,
		},
	} {
		if err := globalDB.PutNetworkSubscription(ctx, subscription); err != nil {
			t.Fatalf("PutNetworkSubscription(%q) error = %v", subscription.SessionID, err)
		}
	}
	return taskRecord, now.Add(time.Minute)
}

func TestGlobalDBTaskEventAppendFailureShouldRollbackOwningState(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back task and profile creation when task.created append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-definition-create-rollback")
		profile := taskpkg.DefaultExecutionProfile(taskRecord.ID)
		event := taskpkg.Event{
			ID:        "event-definition-create-rollback",
			TaskID:    taskRecord.ID,
			EventType: "task.created",
			Actor:     taskRecord.CreatedBy,
			Origin:    taskRecord.Origin,
			Payload:   json.RawMessage(`{"status":"pending"}`),
			Timestamp: taskRecord.CreatedAt,
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, event.EventType)

		err := globalDB.CreateTaskDefinition(ctx, taskpkg.CreateTaskDefinitionMutation{
			Task: taskRecord, Profile: &profile, Events: []taskpkg.Event{event},
		})
		assertForcedTaskEventInsertError(t, err, "CreateTaskDefinition()")
		if _, err = globalDB.GetTask(ctx, taskRecord.ID); !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("GetTask() error = %v, want %v", err, taskpkg.ErrTaskNotFound)
		}
		if _, err = globalDB.GetExecutionProfile(ctx, taskRecord.ID); !errors.Is(
			err,
			taskpkg.ErrExecutionProfileNotFound,
		) {
			t.Fatalf("GetExecutionProfile() error = %v, want %v", err, taskpkg.ErrExecutionProfileNotFound)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back task and profile updates when task.updated append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-definition-update-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		profile := taskpkg.DefaultExecutionProfile(taskRecord.ID)
		if _, err := globalDB.UpsertExecutionProfile(ctx, &profile); err != nil {
			t.Fatalf("UpsertExecutionProfile() error = %v", err)
		}
		updated := taskRecord
		updated.Title = "This title must roll back"
		updated.UpdatedAt = taskRecord.UpdatedAt.Add(time.Minute)
		updatedParticipation := &participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyNamed),
			ChannelID:       new("ops"),
		}
		event := taskpkg.Event{
			ID:        "event-definition-update-rollback",
			TaskID:    taskRecord.ID,
			EventType: "task.updated",
			Actor:     taskRecord.CreatedBy,
			Origin:    taskRecord.Origin,
			Payload:   json.RawMessage(`{"changed_fields":["title","network_participation"]}`),
			Timestamp: updated.UpdatedAt,
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, event.EventType)

		_, err := globalDB.UpdateTaskDefinition(ctx, &taskpkg.UpdateTaskDefinitionMutation{
			Task:                      updated,
			UpdateTaskRow:             true,
			PatchNetworkParticipation: true,
			NetworkParticipation:      updatedParticipation,
			Actor:                     operatorActorContextForTest("operator"),
			Events:                    []taskpkg.Event{event},
		})
		assertForcedTaskEventInsertError(t, err, "UpdateTaskDefinition()")
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Title, taskRecord.Title; got != want {
			t.Fatalf("stored task title = %q, want rollback to %q", got, want)
		}
		storedProfile, err := globalDB.GetExecutionProfile(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetExecutionProfile() error = %v", err)
		}
		if storedProfile.NetworkParticipation != nil {
			t.Fatalf("stored network participation = %#v, want rollback to nil", storedProfile.NetworkParticipation)
		}
	})

	t.Run("Should roll back status update when status_changed append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-status-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskStatusChanged))

		updated := taskRecord
		updated.Status = taskpkg.TaskStatusReady
		updated.UpdatedAt = taskRecord.UpdatedAt.Add(time.Minute)
		err := globalDB.UpdateTask(ctx, updated, operatorActorContextForTest("user:alice"))
		assertForcedTaskEventInsertError(t, err, "UpdateTask(status)")
		stored, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskStatusPending; got != want {
			t.Fatalf("stored.Status = %q, want %q", got, want)
		}
	})

	t.Run("Should roll back child completion when parent rollup event append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		parent := taskRecordForTest("task-parent-rollup-rollback")
		if err := globalDB.CreateTask(ctx, parent); err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		parentRun := taskRunForTest("run-parent-rollup-rollback", parent.ID)
		if err := globalDB.CreateTaskRun(ctx, parentRun); err != nil {
			t.Fatalf("CreateTaskRun(parent) error = %v", err)
		}
		if _, err := globalDB.MarkTaskRunNeedsAttention(ctx, parentRun.ID, "starved"); err != nil {
			t.Fatalf("MarkTaskRunNeedsAttention(parent) error = %v", err)
		}

		completedChild := taskRecordForTest("task-child-completed-rollup-rollback")
		completedChild.ParentTaskID = parent.ID
		completedChild.Status = taskpkg.TaskStatusCompleted
		if err := globalDB.CreateTask(ctx, completedChild); err != nil {
			t.Fatalf("CreateTask(completed child) error = %v", err)
		}
		settlingChild := taskRecordForTest("task-child-settling-rollup-rollback")
		settlingChild.ParentTaskID = parent.ID
		if err := globalDB.CreateTask(ctx, settlingChild); err != nil {
			t.Fatalf("CreateTask(settling child) error = %v", err)
		}
		rawToken := "claim-token-parent-rollup-rollback"
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			settlingChild.ID,
			"run-child-settling-rollup-rollback",
			"sess-child-settling-rollup-rollback",
			rawToken,
			time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
		)

		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForTaskAndType(
			t,
			globalDB,
			parent.ID,
			string(hookspkg.HookTaskStatusChanged),
		)
		_, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      leased.ID,
			ClaimToken: rawToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			Now:        leased.LeaseUntil.Add(-time.Minute),
		})
		assertForcedTaskEventInsertError(t, err, "CompleteRunLease(parent rollup)")

		storedChildRun, err := globalDB.GetTaskRun(ctx, leased.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(child) error = %v", err)
		}
		if got, want := storedChildRun.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("child run status = %q, want rollback to %q", got, want)
		}
		storedChild, err := globalDB.GetTask(ctx, settlingChild.ID)
		if err != nil {
			t.Fatalf("GetTask(child) error = %v", err)
		}
		if got, want := storedChild.Status, taskpkg.TaskStatusPending; got != want {
			t.Fatalf("child task status = %q, want rollback to %q", got, want)
		}
		storedParent, err := globalDB.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent) error = %v", err)
		}
		if got, want := storedParent.Status, taskpkg.TaskStatusPending; got != want {
			t.Fatalf("parent task status = %q, want rollback to %q", got, want)
		}
		storedParentRun, err := globalDB.GetTaskRun(ctx, parentRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(parent) error = %v", err)
		}
		if got, want := storedParentRun.Status, taskpkg.TaskRunStatusNeedsAttention; got != want {
			t.Fatalf("parent run status = %q, want rollback to %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back block creation when task.blocked append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-block-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskBlocked))

		_, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskBlockRecordForTest(
				"block-rollback",
				taskRecord.ID,
				taskpkg.BlockKindNeedsInput,
				taskRecord.UpdatedAt,
			),
			RecurrenceLimit: 2,
		})
		assertForcedTaskEventInsertError(t, err, "CreateTaskBlock()")
		blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, true)
		if err != nil {
			t.Fatalf("ListTaskBlocks() error = %v", err)
		}
		if len(blocks) != 0 {
			t.Fatalf("blocks = %#v, want rollback to remove block row", blocks)
		}
	})

	t.Run("Should roll back block clear when task.unblocked append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-unblocked-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		created, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskBlockRecordForTest(
				"block-unblocked-rollback",
				taskRecord.ID,
				taskpkg.BlockKindTransient,
				taskRecord.UpdatedAt,
			),
			RecurrenceLimit: 2,
		})
		if err != nil {
			t.Fatalf("CreateTaskBlock(setup) error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskUnblocked))

		err = clearTaskBlockForRollbackTest(
			ctx,
			globalDB,
			taskRecord.ID,
			created.Block.ID,
			taskRecord.UpdatedAt.Add(time.Minute),
		)
		assertForcedTaskEventInsertError(t, err, "ClearTaskBlock()")
		blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, false)
		if err != nil {
			t.Fatalf("ListTaskBlocks(open) error = %v", err)
		}
		assertTaskBlockIDs(t, blocks, []string{created.Block.ID})
	})

	t.Run("Should roll back breaker escalation when task.needs_attention append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-attention-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		first, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskBlockRecordForTest(
				"block-attention-first",
				taskRecord.ID,
				taskpkg.BlockKindNeedsInput,
				taskRecord.UpdatedAt,
			),
			RecurrenceLimit: 1,
		})
		if err != nil {
			t.Fatalf("CreateTaskBlock(first) error = %v", err)
		}
		if err := clearTaskBlockForRollbackTest(
			ctx,
			globalDB,
			taskRecord.ID,
			first.Block.ID,
			taskRecord.UpdatedAt.Add(time.Minute),
		); err != nil {
			t.Fatalf("ClearTaskBlock(first) error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskNeedsAttention))

		_, err = globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskBlockRecordForTest(
				"block-attention-second",
				taskRecord.ID,
				taskpkg.BlockKindNeedsInput,
				taskRecord.UpdatedAt.Add(2*time.Minute),
			),
			RecurrenceLimit: 1,
		})
		assertForcedTaskEventInsertError(t, err, "CreateTaskBlock(escalating)")
		stored, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.NeedsAttention != nil {
			t.Fatalf("NeedsAttention = %#v, want rollback to keep task clear", stored.NeedsAttention)
		}
		blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, true)
		if err != nil {
			t.Fatalf("ListTaskBlocks(all) error = %v", err)
		}
		assertTaskBlockIDs(t, blocks, []string{first.Block.ID})
	})

	t.Run("Should roll back attention clear when task.recovered append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-recovered-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		markedAt := taskRecord.UpdatedAt.Add(time.Minute)
		if _, err := globalDB.MarkTaskNeedsAttention(ctx, taskpkg.NeedsAttentionMutation{
			Origin:   coordinatorActorContextForTest().Origin,
			TaskID:   taskRecord.ID,
			Reason:   "operator input required",
			Actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
			MarkedAt: markedAt,
		}); err != nil {
			t.Fatalf("MarkTaskNeedsAttention() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskRecovered))

		_, err := globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
			TaskID:    taskRecord.ID,
			Actor:     operatorActorContextForTest("operator"),
			ClearedAt: markedAt.Add(time.Minute),
		})
		assertForcedTaskEventInsertError(t, err, "ClearTaskNeedsAttention()")
		stored, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.NeedsAttention == nil {
			t.Fatal("NeedsAttention = nil, want rollback to keep escalation metadata")
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back lease completion when task.run.completed append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-completed-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		rawToken := "claim-token-completed-rollback"
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-completed-rollback",
			"sess-completed-rollback",
			rawToken,
			time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
		)
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskRunCompleted))

		_, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      leased.ID,
			ClaimToken: rawToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			Now:        leased.LeaseUntil.Add(-time.Minute),
		})
		assertForcedTaskEventInsertError(t, err, "CompleteRunLease()")
		stored, err := globalDB.GetTaskRun(ctx, leased.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("stored.Status = %q, want rollback to %q", got, want)
		}
		if stored.Result != nil || !stored.EndedAt.IsZero() {
			t.Fatalf("stored terminal fields = result %s ended_at %v, want rollback", stored.Result, stored.EndedAt)
		}
	})

	t.Run("Should roll back expired lease recovery when task.run_lease_expired append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-lease-expired-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-lease-expired-rollback",
			"sess-lease-expired-rollback",
			"claim-token-lease-expired-rollback",
			time.Date(2026, 4, 14, 15, 30, 0, 0, time.UTC),
		)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunLeaseExpired)

		_, err := globalDB.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
			Now:    leased.LeaseUntil.Add(time.Second),
			Reason: "orphaned_on_boot",
		})
		assertForcedTaskEventInsertError(t, err, "RecoverExpiredRunLeases()")
		stored, err := globalDB.GetTaskRun(ctx, leased.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("stored.Status = %q, want rollback to %q", got, want)
		}
		if got, want := stored.SessionID, leased.SessionID; got != want {
			t.Fatalf("stored.SessionID = %q, want rollback to %q", got, want)
		}
		if got, want := stored.ClaimTokenHash, leased.ClaimTokenHash; got != want {
			t.Fatalf("stored.ClaimTokenHash = %q, want rollback to %q", got, want)
		}
		if got, want := stored.LeaseUntil, leased.LeaseUntil; !got.Equal(want) {
			t.Fatalf("stored.LeaseUntil = %s, want rollback to %s", got, want)
		}
	})

	t.Run("Should roll back lease failure when task.run.failed append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-failed-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		rawToken := "claim-token-failed-rollback"
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-failed-rollback",
			"sess-failed-rollback",
			rawToken,
			time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC),
		)
		installTaskEventInsertFailureTriggerForType(t, globalDB, string(hookspkg.HookTaskRunFailed))

		_, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
			Actor:      coordinatorActorContextForTest(),
			RunID:      leased.ID,
			ClaimToken: rawToken,
			Failure:    taskpkg.RunFailure{Error: "worker failed"},
			Now:        leased.LeaseUntil.Add(-time.Minute),
		})
		assertForcedTaskEventInsertError(t, err, "FailRunLease()")
		stored, err := globalDB.GetTaskRun(ctx, leased.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("stored.Status = %q, want rollback to %q", got, want)
		}
		if stored.Error != "" || !stored.EndedAt.IsZero() {
			t.Fatalf("stored terminal fields = error %q ended_at %v, want rollback", stored.Error, stored.EndedAt)
		}
	})
}

func assertForcedTaskEventInsertError(t *testing.T, err error, operation string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), "forced task event insert failure") {
		t.Fatalf("%s error = %v, want forced task event insert failure", operation, err)
	}
}

func TestTaskCoordinationCommandShouldCommitProfileAndEventsAtomically(t *testing.T) {
	t.Parallel()

	t.Run("Should commit task coordination profile events and preserve the active run snapshot", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "coordination-task", t.TempDir())
		taskRecord := workspaceTaskRecordForTest("task-coordination-atomic", workspaceID)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-coordination-active", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		ref := workspacepkg.CoordinationRef{
			WorkspaceID: workspaceID,
			ScopeKind:   workspacepkg.InvitationScopeTask,
			TaskID:      taskRecord.ID,
			RunID:       run.ID,
		}
		commands := workspacepkg.NewCoordinationService(globalDB)
		view, err := commands.Set(ctx, workspacepkg.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 0,
		}, operatorActorContextForTest("operator:coordination"))
		if err != nil {
			t.Fatalf("Set(task coordination) error = %v", err)
		}
		if !view.Setting.Enabled || view.Setting.Revision != 1 ||
			view.Setting.UpdatedBy != "operator:coordination" {
			t.Fatalf("coordination setting = %#v, want enabled revision one", view.Setting)
		}
		profile, err := globalDB.GetExecutionProfile(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetExecutionProfile() error = %v", err)
		}
		if profile.NetworkParticipation == nil || profile.NetworkParticipation.Mode == nil ||
			*profile.NetworkParticipation.Mode != participation.ModeLive ||
			profile.NetworkParticipation.ChannelStrategy == nil ||
			*profile.NetworkParticipation.ChannelStrategy != participation.StrategyRun {
			t.Fatalf(
				"task network participation = %#v, want explicit Live/run future intent",
				profile.NetworkParticipation,
			)
		}
		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if storedRun.RunNetworkState == nil || storedRun.NetworkSpec.Mode != participation.ModeLocal {
			t.Fatalf("active run network state = %#v, want immutable Local snapshot", storedRun.RunNetworkState)
		}
		events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if len(events) != 1 || events[0].EventType != eventspkg.TaskExecutionProfileUpdated {
			t.Fatalf("task events = %#v, want one execution profile event", events)
		}
		summaries, err := globalDB.ListEventSummaries(ctx, EventSummaryQuery{
			WorkspaceID: workspaceID,
			TaskID:      taskRecord.ID,
			Type:        eventspkg.NetworkCoordinationSettingChanged,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if len(summaries) != 1 || summaries[0].ActorID != "operator:coordination" {
			t.Fatalf("coordination summaries = %#v, want committed operator event", summaries)
		}
		if len(observer.records) != 1 || observer.err != nil {
			t.Fatalf("post-commit observer = %#v err=%v, want one committed event", observer.records, observer.err)
		}
	})

	t.Run("Should reject a task from another workspace without writes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		ownerWorkspaceID := registerWorkspaceForGlobalTests(t, globalDB, "coordination-owner", t.TempDir())
		otherWorkspaceID := registerWorkspaceForGlobalTests(t, globalDB, "coordination-other", t.TempDir())
		taskRecord := workspaceTaskRecordForTest("task-coordination-scope", ownerWorkspaceID)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		commands := workspacepkg.NewCoordinationService(globalDB)
		_, err := commands.Set(ctx, workspacepkg.SetCoordination{
			Ref: workspacepkg.CoordinationRef{
				WorkspaceID: otherWorkspaceID,
				ScopeKind:   workspacepkg.InvitationScopeTask,
				TaskID:      taskRecord.ID,
			},
			Enabled:          true,
			ExpectedRevision: 0,
		}, operatorActorContextForTest("operator:scope"))
		if !errors.Is(err, workspacepkg.ErrCoordinationScopeInvalid) {
			t.Fatalf("Set(wrong workspace) error = %v, want %v", err, workspacepkg.ErrCoordinationScopeInvalid)
		}
		if _, err := globalDB.GetExecutionProfile(ctx, taskRecord.ID); !errors.Is(
			err,
			taskpkg.ErrExecutionProfileNotFound,
		) {
			t.Fatalf("GetExecutionProfile() error = %v, want no profile write", err)
		}
	})

	t.Run("Should roll back setting profile and summaries when task event append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "coordination-rollback", t.TempDir())
		taskRecord := workspaceTaskRecordForTest("task-coordination-rollback", workspaceID)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskExecutionProfileUpdated)
		ref := workspacepkg.CoordinationRef{
			WorkspaceID: workspaceID,
			ScopeKind:   workspacepkg.InvitationScopeTask,
			TaskID:      taskRecord.ID,
		}
		commands := workspacepkg.NewCoordinationService(globalDB)
		_, err := commands.Set(ctx, workspacepkg.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 0,
		}, operatorActorContextForTest("operator:rollback"))
		assertForcedTaskEventInsertError(t, err, "SetCoordination(task)")
		view, err := commands.Get(ctx, ref, operatorActorContextForTest("operator:reader"))
		if err != nil {
			t.Fatalf("GetCoordination() error = %v", err)
		}
		if view.Setting.Revision != 0 || view.Setting.Enabled {
			t.Fatalf("coordination setting = %#v, want rolled-back absent row", view.Setting)
		}
		if _, err := globalDB.GetExecutionProfile(ctx, taskRecord.ID); !errors.Is(
			err,
			taskpkg.ErrExecutionProfileNotFound,
		) {
			t.Fatalf("GetExecutionProfile() error = %v, want rolled-back profile", err)
		}
		summaries, err := globalDB.ListEventSummaries(ctx, EventSummaryQuery{
			WorkspaceID: workspaceID,
			TaskID:      taskRecord.ID,
			Type:        eventspkg.NetworkCoordinationSettingChanged,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if len(summaries) != 0 || len(observer.records) != 0 {
			t.Fatalf("summaries/observer = %d/%d, want no rolled-back events", len(summaries), len(observer.records))
		}
	})
}

func clearTaskBlockForRollbackTest(
	ctx context.Context,
	globalDB *GlobalDB,
	taskID string,
	blockID string,
	clearedAt time.Time,
) error {
	_, err := globalDB.ClearTaskBlock(ctx, taskpkg.ClearTaskBlockMutation{
		TaskID:    taskID,
		BlockID:   blockID,
		ClearedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:resolver"},
		ClearedAt: clearedAt,
		ClearNote: "resolved by operator",
		Actor:     operatorActorContextForTest("user:resolver"),
	})
	return err
}

func installTaskEventInsertFailureTriggerForType(t *testing.T, globalDB *GlobalDB, eventType string) {
	t.Helper()

	_, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`CREATE TRIGGER fail_task_event_insert
		 BEFORE INSERT ON task_events
		 WHEN NEW.event_type = '`+strings.ReplaceAll(eventType, "'", "''")+`'
		 BEGIN
		 	SELECT RAISE(ABORT, 'forced task event insert failure');
		 END;`,
	)
	if err != nil {
		t.Fatalf("install task_event insert failure trigger error = %v", err)
	}
}

func installTaskEventInsertFailureTriggerForTaskAndType(
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	eventType string,
) {
	t.Helper()

	_, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`CREATE TRIGGER fail_task_event_insert_for_task
		 BEFORE INSERT ON task_events
		 WHEN NEW.task_id = '`+strings.ReplaceAll(taskID, "'", "''")+`'
		  AND NEW.event_type = '`+strings.ReplaceAll(eventType, "'", "''")+`'
		 BEGIN
		 SELECT RAISE(ABORT, 'forced task event insert failure');
		 END;`,
	)
	if err != nil {
		t.Fatalf("install task_event task/type failure trigger error = %v", err)
	}
}

func requireTaskEventRecordForTest(
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	eventType string,
) taskpkg.EventRecord {
	t.Helper()

	records, err := globalDB.ListTaskEventRecords(testutil.Context(t), taskpkg.EventRecordQuery{TaskID: taskID})
	if err != nil {
		t.Fatalf("ListTaskEventRecords() error = %v", err)
	}
	for _, record := range records {
		if record.Event.EventType == eventType {
			return record
		}
	}
	t.Fatalf("task event %q not found in %#v", eventType, records)
	return taskpkg.EventRecord{}
}
