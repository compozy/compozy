package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type recordingTaskEventCommitObserver struct {
	db      *GlobalDB
	records []taskpkg.EventRecord
	tasks   []taskpkg.Task
	runs    map[string]taskpkg.Run
	err     error
}

// Invariant: coordinator completion commits canonical task events and the matching final task/run
// state atomically. The GlobalDB task-event transaction suite is the canonical owner.
func TestGlobalDBCoordinatorCompletionShouldCommitCanonicalEventAtomically(t *testing.T) {
	t.Run("Should roll back state when the canonical completion event fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t)
		now := time.Date(2026, 8, 2, 18, 0, 0, 0, time.UTC)
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-completion-event-atomic", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			loopRun.ID,
			"completion-event-atomic",
			now.Add(time.Second),
		)
		beforeTask, err := globalDB.GetTask(ctx, claim.Run.TaskID)
		if err != nil {
			t.Fatalf("GetTask(before) error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunCompleted)

		_, err = globalDB.CompleteCoordinatorAndEnqueueNext(
			ctx,
			taskpkg.CoordinatorCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Plan: taskpkg.CoordinatorCompletionPlan{
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  string(loopRun.ID),
						Generation: 1,
					},
					Yield: true,
				},
				Actor: coordinatorActorContextForTest(),
				Now:   now.Add(2 * time.Second),
			},
			looppkg.NewStoreFinalizer(),
		)
		assertForcedTaskEventInsertError(t, err, "CompleteCoordinatorAndEnqueueNext()")

		afterRun, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after rollback) error = %v", err)
		}
		if afterRun.Status != claim.Run.Status || afterRun.EndedAt != claim.Run.EndedAt {
			t.Fatalf("run after event rollback = %#v, want original %#v", afterRun, claim.Run)
		}
		afterTask, err := globalDB.GetTask(ctx, claim.Run.TaskID)
		if err != nil {
			t.Fatalf("GetTask(after rollback) error = %v", err)
		}
		if !reflect.DeepEqual(afterTask, beforeTask) {
			t.Fatalf("task after event rollback = %#v, want original %#v", afterTask, beforeTask)
		}
		afterLoop, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID(after rollback) error = %v", err)
		}
		if afterLoop.Status != loopRun.Status || afterLoop.Generation != loopRun.Generation {
			t.Fatalf("loop after event rollback = %#v, want original %#v", afterLoop, loopRun)
		}
	})

	t.Run("Should commit partial completion state with its terminal status event", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t)
		now := time.Date(2026, 8, 2, 19, 0, 0, 0, time.UTC)
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-partial-terminal", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim := claimCoordinatorRunForTest(ctx, t, globalDB, loopRun.ID, "partial-terminal", now)
		_, err = globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
			RunID: claim.Run.ID, ClaimToken: claim.ClaimToken,
			Actor: coordinatorActorContextForTest(), Now: now.Add(time.Second),
			Plan: taskpkg.CoordinatorCompletionPlan{
				Snapshot: taskpkg.GenerationSnapshot{
					LoopRunID: string(loopRun.ID), Generation: 1,
					Payload: looppkg.GenerationSnapshotPayload{Outputs: []looppkg.GenerationOutput{
						{
							NodeID:    "collect",
							Status:    "partial",
							OutputRef: `{"total":3,"succeeded":2,"failed":0,"canceled":1,"coverage_rate":0.67,"partial":true}`,
						},
					}},
				},
				Terminal: &taskpkg.CoordinatorTerminal{
					Status: string(looppkg.StatusDone), Cause: string(looppkg.TransitionCauseContract),
				},
			},
		}, looppkg.NewStoreFinalizer())
		if err != nil {
			t.Fatalf("CompleteCoordinatorAndEnqueueNext() error = %v", err)
		}
		stored, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID() error = %v", err)
		}
		if stored.CompletionStateSnapshot() != looppkg.CompletionPartial {
			t.Fatalf("completion state = %q, want %q", stored.CompletionStateSnapshot(), looppkg.CompletionPartial)
		}
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: loopRun.WorkspaceID, RunID: loopRun.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		var terminalPayload map[string]any
		for _, event := range events {
			if event.Kind == loopRunEventStatusChanged {
				if err := json.Unmarshal(event.Payload, &terminalPayload); err != nil {
					t.Fatalf("unmarshal status event: %v", err)
				}
			}
		}
		if got, want := terminalPayload["completion_state"], string(looppkg.CompletionPartial); got != want {
			t.Fatalf("terminal event completion_state = %v, want %q", got, want)
		}
	})
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
	if strings.TrimSpace(record.Event.RunID) == "" {
		return
	}
	run, err := o.db.GetTaskRun(ctx, record.Event.RunID)
	if err != nil {
		o.err = err
		return
	}
	if o.runs == nil {
		o.runs = make(map[string]taskpkg.Run)
	}
	o.runs[record.Event.ID] = run
}

func TestGlobalDBTaskRunLeaseSettlementShouldPublishCommittedSequence(t *testing.T) {
	t.Run("Should publish committed claim heartbeat and release state in sequence", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"committed-sequence",
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)

		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            run.ID,
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-committed-sequence",
			LeaseDuration:    2 * time.Minute,
			Now:              now,
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		heartbeat, err := manager.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			RunID:         run.ID,
			ClaimToken:    claim.ClaimToken,
			LeaseDuration: 3 * time.Minute,
			TokensUsed:    7,
			Now:           now.Add(30 * time.Second),
		}, actor)
		if err != nil {
			t.Fatalf("HeartbeatRunLease() error = %v", err)
		}

		globalDB.SetTaskEventCommitObserver(nil)
		running := *heartbeat
		running.Status = taskpkg.TaskRunStatusRunning
		running.StartedAt = now.Add(40 * time.Second)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, running); err != nil {
			t.Fatalf("UpdateTaskRun(running) error = %v", err)
		}
		inProgress, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(in progress setup) error = %v", err)
		}
		inProgress.Status = taskpkg.TaskStatusInProgress
		inProgress.UpdatedAt = now.Add(40 * time.Second)
		if err := globalDB.UpdateTask(ctx, inProgress, actor); err != nil {
			t.Fatalf("UpdateTask(in progress setup) error = %v", err)
		}
		globalDB.SetTaskEventCommitObserver(observer)

		released, err := manager.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Reason:     "handoff",
			Now:        now.Add(50 * time.Second),
		}, actor)
		if err != nil {
			t.Fatalf("ReleaseRunLease() error = %v", err)
		}
		if got, want := released.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("released.Status = %q, want %q", got, want)
		}
		if observer.err != nil {
			t.Fatalf("post-commit observer error = %v", observer.err)
		}

		wantTypes := []string{
			eventspkg.TaskRunClaimed,
			eventspkg.TaskRunLeaseExtended,
			string(hookspkg.HookTaskStatusChanged),
			eventspkg.TaskRunReleased,
		}
		if got, want := len(observer.records), len(wantTypes); got != want {
			t.Fatalf("len(observer.records) = %d, want %d: %#v", got, want, observer.records)
		}
		for idx, record := range observer.records {
			if got, want := record.Event.EventType, wantTypes[idx]; got != want {
				t.Fatalf("observer.records[%d].EventType = %q, want %q", idx, got, want)
			}
			if record.Sequence <= 0 || (idx > 0 && record.Sequence <= observer.records[idx-1].Sequence) {
				t.Fatalf("observer event sequences = %#v, want positive strict increase", observer.records)
			}
		}
		wantTimestamps := []time.Time{
			now,
			now.Add(30 * time.Second),
			now.Add(50 * time.Second),
			now.Add(50 * time.Second),
		}
		for idx, record := range observer.records {
			if got, want := record.Event.Timestamp, wantTimestamps[idx]; !got.Equal(want) {
				t.Fatalf("observer.records[%d].Timestamp = %s, want command time %s", idx, got, want)
			}
		}

		var claimedPayload struct {
			ClaimTokenHash string `json:"claim_token_hash"`
		}
		if err := json.Unmarshal(observer.records[0].Event.Payload, &claimedPayload); err != nil {
			t.Fatalf("Unmarshal(task.run_claimed payload) error = %v", err)
		}
		if claimedPayload.ClaimTokenHash != claim.Run.ClaimTokenHash {
			t.Fatalf("claim_token_hash = %q, want %q", claimedPayload.ClaimTokenHash, claim.Run.ClaimTokenHash)
		}
		if claim.ClaimToken == "" || strings.Contains(string(observer.records[0].Event.Payload), claim.ClaimToken) ||
			strings.Contains(string(observer.records[0].Event.Payload), `"claim_token"`) {
			t.Fatalf("task.run_claimed payload leaked raw claim token: %s", observer.records[0].Event.Payload)
		}

		if got := observer.tasks[0]; got.Status != taskpkg.TaskStatusReady || got.CurrentRunID != run.ID {
			t.Fatalf("task observed after claim = %#v, want committed ready/current run projection", got)
		}
		for _, idx := range []int{2, 3} {
			if got := observer.tasks[idx]; got.Status != taskpkg.TaskStatusReady || got.CurrentRunID != "" {
				t.Fatalf(
					"task observed after release event %d = %#v, want committed ready/cleared projection",
					idx,
					got,
				)
			}
			if got, want := observer.tasks[idx].UpdatedAt, now.Add(50*time.Second); !got.Equal(want) {
				t.Fatalf("task observed after release event %d UpdatedAt = %s, want %s", idx, got, want)
			}
		}
		if got := observer.runs[observer.records[0].Event.ID]; got.Status != taskpkg.TaskRunStatusClaimed {
			t.Fatalf("run observed after claim = %#v, want claimed", got)
		}
		if got := observer.runs[observer.records[1].Event.ID]; got.Status != taskpkg.TaskRunStatusClaimed ||
			got.TokensUsed != 7 {
			t.Fatalf("run observed after heartbeat = %#v, want extended claimed lease", got)
		}
		if got := observer.runs[observer.records[3].Event.ID]; got.Status != taskpkg.TaskRunStatusQueued ||
			got.SessionID != "" || got.ClaimTokenHash != "" {
			t.Fatalf("run observed after release = %#v, want reusable queued run", got)
		}
	})

	t.Run("Should publish a committed failure and reconciled task in sequence", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"failed-committed-sequence",
		)
		globalDB.SetTaskEventCommitObserver(nil)
		claim := claimTaskRunLeaseForSettlementTest(
			ctx,
			t,
			manager,
			run,
			actor,
			now,
			"sess-failed-committed-sequence",
		)

		running := claim.Run
		running.Status = taskpkg.TaskRunStatusRunning
		running.StartedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, running); err != nil {
			t.Fatalf("UpdateTaskRun(running) error = %v", err)
		}
		inProgress, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(in progress setup) error = %v", err)
		}
		inProgress.Status = taskpkg.TaskStatusInProgress
		inProgress.UpdatedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateTask(ctx, inProgress, actor); err != nil {
			t.Fatalf("UpdateTask(in progress setup) error = %v", err)
		}

		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		failureAt := now.Add(time.Minute)
		wantFailureError := "worker failed after claiming the run with compozy_claim_[REDACTED]"
		failed, err := manager.FailRunLease(ctx, taskpkg.LeaseFailure{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Failure: taskpkg.RunFailure{
				Error: "worker failed after claiming the run with " + claim.ClaimToken,
				Metadata: json.RawMessage(
					`{"kind":"worker_failure","detail":"claim ` + claim.ClaimToken + ` rejected"}`,
				),
			},
			TokensUsed: 23,
			Now:        failureAt,
		}, actor)
		if err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}
		if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("failed.Status = %q, want %q", got, want)
		}
		if got, want := failed.Error, wantFailureError; got != want {
			t.Fatalf("failed.Error = %q, want %q", got, want)
		}
		if got, want := failed.TokensUsed, int64(23); got != want {
			t.Fatalf("failed.TokensUsed = %d, want %d", got, want)
		}
		if observer.err != nil {
			t.Fatalf("post-commit observer error = %v", observer.err)
		}

		wantTypes := []string{
			string(hookspkg.HookTaskStatusChanged),
			string(hookspkg.HookTaskRunFailed),
		}
		if got, want := len(observer.records), len(wantTypes); got != want {
			t.Fatalf("len(observer.records) = %d, want %d: %#v", got, want, observer.records)
		}
		for idx, record := range observer.records {
			if got, want := record.Event.EventType, wantTypes[idx]; got != want {
				t.Fatalf("observer.records[%d].EventType = %q, want %q", idx, got, want)
			}
			if !record.Event.Timestamp.Equal(failureAt) {
				t.Fatalf("observer.records[%d].Timestamp = %s, want %s", idx, record.Event.Timestamp, failureAt)
			}
			if record.Sequence <= 0 || (idx > 0 && record.Sequence <= observer.records[idx-1].Sequence) {
				t.Fatalf("observer event sequences = %#v, want positive strict increase", observer.records)
			}
			if got := observer.tasks[idx]; got.Status != taskpkg.TaskStatusReady || got.CurrentRunID != "" {
				t.Fatalf(
					"task observed after failure event %d = %#v, want committed ready/cleared projection",
					idx,
					got,
				)
			}
			if got, want := observer.tasks[idx].UpdatedAt, failureAt; !got.Equal(want) {
				t.Fatalf("task observed after failure event %d UpdatedAt = %s, want %s", idx, got, want)
			}
			if idx == 1 {
				if got := observer.runs[record.Event.ID]; got.Status != taskpkg.TaskRunStatusFailed ||
					got.Error != wantFailureError || got.TokensUsed != 23 {
					t.Fatalf("run observed after task.run.failed = %#v, want committed failure", got)
				}
			}
		}

		var payload taskpkg.RunLeaseFailedEventPayload
		failedEvent := observer.records[1].Event
		if err := json.Unmarshal(failedEvent.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal(task.run.failed payload) error = %v", err)
		}
		if got, want := payload.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("failed payload status = %q, want %q", got, want)
		}
		if got, want := payload.Error, wantFailureError; got != want {
			t.Fatalf("failed payload error = %q, want %q", got, want)
		}
		var metadata map[string]string
		if err := json.Unmarshal(payload.Metadata, &metadata); err != nil {
			t.Fatalf("Unmarshal(task.run.failed metadata) error = %v", err)
		}
		if got, want := metadata["kind"], "worker_failure"; got != want {
			t.Fatalf("failed payload metadata kind = %q, want %q", got, want)
		}
		if got, want := metadata["detail"], "claim compozy_claim_[REDACTED] rejected"; got != want {
			t.Fatalf("failed payload metadata detail = %q, want %q", got, want)
		}
		if got, want := payload.ClaimTokenHash, claim.Run.ClaimTokenHash; got != want {
			t.Fatalf("failed payload claim_token_hash = %q, want %q", got, want)
		}
		if strings.Contains(string(failedEvent.Payload), claim.ClaimToken) ||
			strings.Contains(string(failedEvent.Payload), `"claim_token"`) ||
			strings.Contains(string(failedEvent.Payload), `"task_status"`) {
			t.Fatalf("task.run.failed payload changed or leaked raw claim state: %s", failedEvent.Payload)
		}
	})

	t.Run("Should drain open children before a public coordinator lease failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openLoopTestGlobalDB(t)
		now := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-public-failure-event", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim := claimCoordinatorRunForTest(
			ctx,
			t,
			globalDB,
			loopRun.ID,
			"public-failure-event",
			now.Add(time.Second),
		)
		childTask := workspaceTaskRecordForTest(
			"loop.looprun-public-failure-event.g2.node.ready.0",
			string(loopRun.WorkspaceID),
		)
		childTask.ParentTaskID = claim.Run.TaskID
		childTask.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, childTask); err != nil {
			t.Fatalf("CreateTask(coordinator child) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		actor := coordinatorActorContextForTest()
		failureAt := now.Add(2 * time.Second)
		suppliedFailure := taskpkg.RunFailure{
			Error: "untyped coordinator failure with " + claim.ClaimToken,
			Metadata: json.RawMessage(
				`{"detail":"non-canonical ` + claim.ClaimToken + `"}`,
			),
		}
		failed, err := globalDB.FailRunLease(ctx, taskpkg.LeaseFailure{
			Actor:      actor,
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Failure:    suppliedFailure,
			Now:        failureAt,
		})
		if err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}
		if observer.err != nil {
			t.Fatalf("post-commit observer error = %v", observer.err)
		}
		if got, want := len(observer.records), 3; got != want {
			t.Fatalf("observer records = %d, want %d: %#v", got, want, observer.records)
		}
		if got, want := observer.records[0].Event.EventType, string(hookspkg.HookTaskStatusChanged); got != want {
			t.Fatalf("first event type = %q, want %q", got, want)
		}
		if got, want := observer.records[0].Event.TaskID, childTask.ID; got != want {
			t.Fatalf("first event task = %q, want drained child %q", got, want)
		}

		coordinatorEvent := observer.records[1].Event
		if got, want := coordinatorEvent.EventType, string(hookspkg.HookTaskStatusChanged); got != want {
			t.Fatalf("coordinator settlement event type = %q, want %q", got, want)
		}
		var settlementPayload taskStatusChangedEventPayload
		if err := json.Unmarshal(coordinatorEvent.Payload, &settlementPayload); err != nil {
			t.Fatalf("decode coordinator settlement event error = %v", err)
		}
		if settlementPayload.Reason != loopRunTerminalReason {
			t.Fatalf("coordinator settlement reason = %q, want %q", settlementPayload.Reason, loopRunTerminalReason)
		}
		if got, want := settlementPayload.ToStatus, string(taskpkg.TaskStatusFailed); got != want {
			t.Fatalf("coordinator settlement to_status = %q, want %q", got, want)
		}
		if settlementPayload.TaskID != claim.Run.TaskID ||
			settlementPayload.RunID != claim.Run.ID ||
			settlementPayload.LoopRunID != string(loopRun.ID) ||
			settlementPayload.WorkspaceID != string(loopRun.WorkspaceID) ||
			settlementPayload.ActorKind != string(taskpkg.ActorKindDaemon) ||
			settlementPayload.ReleaseReason != loopRunTerminalReason {
			t.Fatalf("coordinator settlement payload = %#v, want complete daemon repair anchors", settlementPayload)
		}

		event := observer.records[2].Event
		if got, want := event.EventType, string(hookspkg.HookTaskRunFailed); got != want {
			t.Fatalf("event type = %q, want %q", got, want)
		}
		if event.TaskID != claim.Run.TaskID || event.RunID != claim.Run.ID {
			t.Fatalf(
				"event anchors = task %q run %q, want task %q run %q",
				event.TaskID,
				event.RunID,
				claim.Run.TaskID,
				claim.Run.ID,
			)
		}
		if event.Actor != actor.Actor || event.Origin != actor.Origin {
			t.Fatalf("event actor/origin = %#v/%#v, want %#v/%#v", event.Actor, event.Origin, actor.Actor, actor.Origin)
		}
		if !event.Timestamp.Equal(failureAt) {
			t.Fatalf("event timestamp = %s, want %s", event.Timestamp, failureAt)
		}

		taskRecord, err := globalDB.GetTask(ctx, event.TaskID)
		if err != nil {
			t.Fatalf("GetTask(event anchor) error = %v", err)
		}
		if taskRecord.WorkspaceID == "" || taskRecord.WorkspaceID != failed.WorkspaceID {
			t.Fatalf(
				"event workspace anchor = %q, failed run workspace = %q",
				taskRecord.WorkspaceID,
				failed.WorkspaceID,
			)
		}
		storedChild, err := globalDB.GetTask(ctx, childTask.ID)
		if err != nil {
			t.Fatalf("GetTask(coordinator child) error = %v", err)
		}
		if storedChild.Status != taskpkg.TaskStatusCanceled {
			t.Fatalf("coordinator child status = %q, want canceled", storedChild.Status)
		}
		storedCoordinator, err := globalDB.GetTask(ctx, claim.Run.TaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		if storedCoordinator.Status != taskpkg.TaskStatusFailed {
			t.Fatalf("coordinator final status = %q, want failed", storedCoordinator.Status)
		}
		var liveRecords int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_runs
			WHERE loop_run_id = ? AND status IN ('queued','claimed','starting','running','needs_attention')`,
			loopRun.ID).Scan(&liveRecords); err != nil {
			t.Fatalf("count live Loop records error = %v", err)
		}
		if liveRecords != 0 {
			t.Fatalf("live Loop records = %d, want zero", liveRecords)
		}

		events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: event.TaskID,
			RunID:  event.RunID,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		failureEventCount := 0
		for _, storedEvent := range events {
			if storedEvent.EventType == string(hookspkg.HookTaskRunFailed) {
				failureEventCount++
			}
		}
		if got, want := failureEventCount, 1; got != want {
			t.Fatalf("task.run.failed event count = %d, want %d", got, want)
		}

		var payload taskpkg.RunLeaseFailedEventPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal(task.run.failed payload) error = %v", err)
		}
		if got, want := payload.Error, failed.Error; got != want {
			t.Fatalf("payload error = %q, want persisted error %q", got, want)
		}
		canonicalFailure := taskpkg.RunFailure{Error: payload.Error, Metadata: payload.Metadata}
		if _, ok := taskpkg.CoordinatorFailureFromRunFailure(canonicalFailure); !ok {
			t.Fatalf("event failure = %#v, want canonical coordinator failure", canonicalFailure)
		}
		if got := taskpkg.RedactClaimTokens(strings.TrimSpace(suppliedFailure.Error)); payload.Error == got {
			t.Fatalf("payload error = %q, want invalid coordinator input replaced canonically", payload.Error)
		}
		if payload.ClaimTokenHash != claim.Run.ClaimTokenHash ||
			strings.Contains(string(event.Payload), claim.ClaimToken) {
			t.Fatalf("task.run.failed payload changed hash or leaked raw bearer: %s", event.Payload)
		}
	})
}

func TestGlobalDBSessionLeaseReleaseShouldSettleOneSessionAtomically(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back every run and network ledger when a later task event fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 1, 16, 0, 0, 0, time.UTC)
		sessionID := "sess-session-release-rollback"
		daemon := coordinatorActorContextForTest()
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		firstTask, firstRun := seedLeasedSessionRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"session-release-rollback-first",
			sessionID,
			now,
		)
		_, secondRun := seedLeasedSessionRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"session-release-rollback-second",
			sessionID,
			now.Add(time.Second),
		)
		registerNetworkWakeRunSessionsForClaimTest(t, globalDB, now, sessionID)
		wake := networkWakeRunForClaimTest(
			"run-session-release-rollback-wake",
			"wake-session-release-rollback",
			sessionID,
			"session:"+sessionID,
			now.Add(2*time.Second),
		)
		createNetworkWakeRunForClaimTest(t, globalDB, wake, now)
		leasedWake := leaseNetworkWakeForSessionReleaseTest(
			t,
			globalDB,
			wake,
			sessionID,
			daemon,
			now.Add(3*time.Minute),
		)

		installTaskEventInsertFailureTriggerForTaskAndType(
			t,
			globalDB,
			firstTask.ID,
			eventspkg.TaskRunReleased,
		)
		_, err = manager.ReleaseSessionRunLeases(ctx, taskpkg.SessionLeaseRelease{
			SessionID: sessionID,
			Reason:    "runtime cleanup",
			Now:       now.Add(30 * time.Second),
		}, daemon)
		assertForcedTaskEventInsertError(t, err, "ReleaseSessionRunLeases()")

		for _, want := range []taskpkg.Run{firstRun, secondRun, leasedWake} {
			got, getErr := globalDB.GetTaskRun(ctx, want.ID)
			if getErr != nil {
				t.Fatalf("GetTaskRun(%q) error = %v", want.ID, getErr)
			}
			assertTaskRunLeaseRollbackState(t, got, want)
		}
		if got := networkWakeEventCount(
			t,
			globalDB,
			"wake-session-release-rollback",
			networkWakeEventReleased,
		); got != 0 {
			t.Fatalf("released network wake events = %d, want 0 after rollback", got)
		}
		for _, taskID := range []string{firstTask.ID, "task-session-release-rollback-second"} {
			events, listErr := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskID})
			if listErr != nil {
				t.Fatalf("ListTaskEvents(%q) error = %v", taskID, listErr)
			}
			if len(events) != 0 {
				t.Fatalf("task events for %q = %#v, want no partial ledger", taskID, events)
			}
		}
	})

	t.Run("Should commit all task and network release ledgers together", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 1, 17, 0, 0, 0, time.UTC)
		sessionID := "sess-session-release-commit"
		daemon := coordinatorActorContextForTest()
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		firstTask, firstRun := seedLeasedSessionRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"session-release-commit-first",
			sessionID,
			now,
		)
		secondTask, secondRun := seedLeasedSessionRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"session-release-commit-second",
			sessionID,
			now.Add(time.Second),
		)
		registerNetworkWakeRunSessionsForClaimTest(t, globalDB, now, sessionID)
		wake := networkWakeRunForClaimTest(
			"run-session-release-commit-wake",
			"wake-session-release-commit",
			sessionID,
			"session:"+sessionID,
			now.Add(2*time.Second),
		)
		createNetworkWakeRunForClaimTest(t, globalDB, wake, now)
		leasedWake := leaseNetworkWakeForSessionReleaseTest(
			t,
			globalDB,
			wake,
			sessionID,
			daemon,
			now.Add(3*time.Minute),
		)

		results, err := manager.ReleaseSessionRunLeases(ctx, taskpkg.SessionLeaseRelease{
			SessionID: sessionID,
			Reason:    "runtime cleanup",
			Now:       now.Add(30 * time.Second),
		}, daemon)
		if err != nil {
			t.Fatalf("ReleaseSessionRunLeases() error = %v", err)
		}
		if got, want := len(results), 3; got != want {
			t.Fatalf("len(ReleaseSessionRunLeases()) = %d, want %d", got, want)
		}
		for _, previous := range []taskpkg.Run{firstRun, secondRun, leasedWake} {
			got, getErr := globalDB.GetTaskRun(ctx, previous.ID)
			if getErr != nil {
				t.Fatalf("GetTaskRun(%q) error = %v", previous.ID, getErr)
			}
			if got.Status != taskpkg.TaskRunStatusQueued || got.SessionID != "" || got.ClaimTokenHash != "" {
				t.Fatalf("released run %q = %#v, want queued and unleased", previous.ID, got)
			}
		}
		for _, taskID := range []string{firstTask.ID, secondTask.ID} {
			events, listErr := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
				TaskID:    taskID,
				EventType: eventspkg.TaskRunReleased,
			})
			if listErr != nil {
				t.Fatalf("ListTaskEvents(%q) error = %v", taskID, listErr)
			}
			if len(events) != 1 {
				t.Fatalf("released task events for %q = %#v, want one", taskID, events)
			}
		}
		if got := networkWakeEventCount(
			t,
			globalDB,
			"wake-session-release-commit",
			networkWakeEventReleased,
		); got != 1 {
			t.Fatalf("released network wake events = %d, want 1", got)
		}
	})

	t.Run("Should reject a stale run snapshot without overwriting a renewed lease", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 1, 18, 0, 0, 0, time.UTC)
		sessionID := "sess-session-release-stale"
		daemon := coordinatorActorContextForTest()
		_, stale := seedLeasedSessionRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"session-release-stale",
			sessionID,
			now,
		)
		renewed := stale
		renewed.LeaseUntil = stale.LeaseUntil.Add(time.Minute)
		renewed.HeartbeatAt = stale.HeartbeatAt.Add(time.Minute)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, renewed); err != nil {
			t.Fatalf("UpdateTaskRun(renewed) error = %v", err)
		}

		err := globalDB.WithLeaseSettlementTransaction(
			ctx,
			"test stale session lease release",
			func(store taskpkg.LeaseSettlementMutationStore) error {
				_, releaseErr := store.ReleaseSessionRunLease(ctx, stale, taskpkg.SessionLeaseRelease{
					SessionID: sessionID,
					Reason:    "runtime cleanup",
					Now:       now.Add(30 * time.Second),
				}, daemon)
				return releaseErr
			},
		)
		if !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
			t.Fatalf("ReleaseSessionRunLease(stale) error = %v, want %v", err, taskpkg.ErrTaskRunNotFound)
		}
		stored, getErr := globalDB.GetTaskRun(ctx, stale.ID)
		if getErr != nil {
			t.Fatalf("GetTaskRun() error = %v", getErr)
		}
		assertTaskRunLeaseRollbackState(t, stored, renewed)
	})
}

func seedLeasedSessionRunForEventTransaction(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
	sessionID string,
	now time.Time,
) (taskpkg.Task, taskpkg.Run) {
	t.Helper()

	taskRecord := taskRecordForTest("task-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusReady
	taskRecord.UpdatedAt = now
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask(%q) error = %v", taskRecord.ID, err)
	}
	rawToken, err := taskpkg.NewClaimToken()
	if err != nil {
		t.Fatalf("NewClaimToken() error = %v", err)
	}
	run := leasedRunForGlobalTest(
		t,
		"run-"+suffix,
		taskRecord.ID,
		sessionID,
		rawToken,
		now.Add(5*time.Minute),
	)
	run.QueuedAt = now
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
	}
	return taskRecord, run
}

func leaseNetworkWakeForSessionReleaseTest(
	t *testing.T,
	globalDB *GlobalDB,
	wake taskpkg.Run,
	sessionID string,
	actor taskpkg.ActorContext,
	leaseUntil time.Time,
) taskpkg.Run {
	t.Helper()

	rawToken, err := taskpkg.NewClaimToken()
	if err != nil {
		t.Fatalf("NewClaimToken() error = %v", err)
	}
	claimTokenHash, err := taskpkg.ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}
	wake.Status = taskpkg.TaskRunStatusClaimed
	wake.ClaimedBy = &actor.Actor
	wake.SessionID = sessionID
	wake.ClaimTokenHash = claimTokenHash
	wake.ClaimedAt = leaseUntil.Add(-time.Minute)
	wake.HeartbeatAt = leaseUntil.Add(-30 * time.Second)
	wake.LeaseUntil = leaseUntil
	if err := globalDB.UpdateNonTerminalTaskRun(testutil.Context(t), wake); err != nil {
		t.Fatalf("UpdateTaskRun(%q) error = %v", wake.ID, err)
	}
	return wake
}

func seedTaskRunLeaseSettlementForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
) (*taskpkg.Service, taskpkg.Task, taskpkg.Run, taskpkg.ActorContext, time.Time) {
	t.Helper()

	now := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)
	taskRecord := taskRecordForTest("task-lease-settlement-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusReady
	taskRecord.UpdatedAt = now.Add(-time.Minute)
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-lease-settlement-"+suffix, taskRecord.ID)
	run.QueuedAt = now.Add(-time.Minute)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, taskRecord, run, operatorActorContextForTest("operator:" + suffix), now
}

func claimTaskRunLeaseForSettlementTest(
	ctx context.Context,
	t *testing.T,
	manager *taskpkg.Service,
	run taskpkg.Run,
	actor taskpkg.ActorContext,
	now time.Time,
	sessionID string,
) *taskpkg.ClaimResult {
	t.Helper()

	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            run.ID,
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: sessionID,
		LeaseDuration:    2 * time.Minute,
		Now:              now,
	}, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	return claim
}

func assertTaskRunLeaseRollbackState(t *testing.T, got taskpkg.Run, want taskpkg.Run) {
	t.Helper()

	if got.Status != want.Status ||
		got.SessionID != want.SessionID ||
		got.ClaimTokenHash != want.ClaimTokenHash ||
		!got.LeaseUntil.Equal(want.LeaseUntil) ||
		!got.HeartbeatAt.Equal(want.HeartbeatAt) ||
		!got.ClaimedAt.Equal(want.ClaimedAt) ||
		!got.StartedAt.Equal(want.StartedAt) ||
		got.TokensUsed != want.TokensUsed {
		t.Fatalf("stored run lease = %#v, want rollback to %#v", got, want)
	}
	if (got.ClaimedBy == nil) != (want.ClaimedBy == nil) {
		t.Fatalf("stored run ClaimedBy = %#v, want rollback to %#v", got.ClaimedBy, want.ClaimedBy)
	}
	if got.ClaimedBy != nil &&
		(got.ClaimedBy.Kind != want.ClaimedBy.Kind || got.ClaimedBy.Ref != want.ClaimedBy.Ref) {
		t.Fatalf("stored run ClaimedBy = %#v, want rollback to %#v", got.ClaimedBy, want.ClaimedBy)
	}
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

	t.Run("Should detach and bound the committed observer context", func(t *testing.T) {
		contextObserver := &taskEventContextObserver{}
		globalDB.SetTaskEventCommitObserver(contextObserver)
		callerCtx, cancelCaller := context.WithCancel(context.Background())
		cancelCaller()
		globalDB.notifyCommittedTaskEvents(callerCtx, observer.records)
		if contextObserver.err != nil {
			t.Fatalf("OnTaskEvent() context error = %v, want active detached context", contextObserver.err)
		}
		if !contextObserver.hasDeadline {
			t.Fatal("OnTaskEvent() context has no deadline")
		}
		remaining := time.Until(contextObserver.deadline)
		if remaining <= 0 || remaining > taskEventObserverTimeout {
			t.Fatalf("OnTaskEvent() deadline remaining = %s, want within (0,%s]", remaining, taskEventObserverTimeout)
		}
	})
}

type taskEventContextObserver struct {
	err         error
	deadline    time.Time
	hasDeadline bool
}

func (o *taskEventContextObserver) OnTaskEvent(ctx context.Context, _ taskpkg.EventRecord) {
	o.err = ctx.Err()
	o.deadline, o.hasDeadline = ctx.Deadline()
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
			RuntimeStatus:       store.SessionRuntimeUnbound,
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

	t.Run("Should roll back force release and input invalidation when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		actor := operatorActorContextForTest("operator:force-release-rollback")
		taskRecord := taskRecordForTest("task-force-release-event-rollback")
		taskRecord.Status = taskpkg.TaskStatusInProgress
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-force-release-event-rollback", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "force-release-rollback", t.TempDir())
		sessionID := "sess-force-release-event-rollback"
		if err := globalDB.RegisterSession(ctx, store.SessionInfo{
			ID: sessionID, AgentName: "worker", WorkspaceID: workspaceID,
			RuntimeStatus: store.SessionRuntimeUnbound, State: "active",
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		claimed := run
		claimed.Status = taskpkg.TaskRunStatusClaimed
		claimed.ClaimedBy = actorForTest(taskpkg.ActorKindAgentSession, sessionID)
		claimed.SessionID = sessionID
		claimed.ClaimTokenHash = "sha256:" + strings.Repeat("a", 64)
		claimed.ClaimedAt = run.QueuedAt.Add(time.Second)
		claimed.HeartbeatAt = run.QueuedAt.Add(time.Second)
		claimed.LeaseUntil = run.QueuedAt.Add(5 * time.Minute)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, claimed); err != nil {
			t.Fatalf("UpdateNonTerminalTaskRun(claimed) error = %v", err)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		storedTask.CurrentRunID = claimed.ID
		storedTask.UpdatedAt = claimed.ClaimedAt
		if err := globalDB.UpdateTask(ctx, storedTask, actor); err != nil {
			t.Fatalf("UpdateTask(current run) error = %v", err)
		}
		before, err := globalDB.GetTaskRun(ctx, claimed.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before) error = %v", err)
		}
		generationBefore, err := globalDB.CurrentSessionInputGeneration(ctx, sessionID)
		if err != nil {
			t.Fatalf("CurrentSessionInputGeneration(before) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunReleased)
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		_, err = manager.ForceReleaseRun(ctx, claimed.ID, taskpkg.ForceReleaseRun{
			Reason: "operator handoff",
		}, actor)
		assertForcedTaskEventInsertError(t, err, "ForceReleaseRun()")
		after, err := globalDB.GetTaskRun(ctx, claimed.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after) error = %v", err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("run after rollback = %#v, want %#v", after, before)
		}
		generationAfter, err := globalDB.CurrentSessionInputGeneration(ctx, sessionID)
		if err != nil {
			t.Fatalf("CurrentSessionInputGeneration(after) error = %v", err)
		}
		if generationAfter != generationBefore {
			t.Fatalf("session generation after rollback = %d, want %d", generationAfter, generationBefore)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back force failure when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"force-fail-event-rollback",
			taskpkg.TaskRunStatusQueued,
		)
		before, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunOperatorForcedFail)

		_, err = manager.ForceFailRun(ctx, run.ID, taskpkg.ForceFailRun{Reason: "operator recovery"}, actor)
		assertForcedTaskEventInsertError(t, err, "ForceFailRun()")
		after, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after) error = %v", err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("run after rollback = %#v, want %#v", after, before)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskRecord.Status; got != want {
			t.Fatalf("task status after rollback = %q, want %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back retry creation and reconciliation when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"retry-event-rollback",
			taskpkg.TaskRunStatusQueued,
		)
		failed, err := manager.ForceFailRun(
			ctx,
			run.ID,
			taskpkg.ForceFailRun{Reason: "operator recovery"},
			actor,
		)
		if err != nil {
			t.Fatalf("ForceFailRun() error = %v", err)
		}
		beforeTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(before) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunOperatorRetry)

		_, err = manager.RetryRun(ctx, failed.ID, taskpkg.RetryRunRequest{}, actor)
		assertForcedTaskEventInsertError(t, err, "RetryRun()")
		afterSource, err := globalDB.GetTaskRun(ctx, failed.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(source after) error = %v", err)
		}
		if !reflect.DeepEqual(afterSource, *failed) {
			t.Fatalf("source run after rollback = %#v, want %#v", afterSource, *failed)
		}
		runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) after rollback = %d, want %d", got, want)
		}
		afterTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(after) error = %v", err)
		}
		if !reflect.DeepEqual(afterTask, beforeTask) {
			t.Fatalf("task after rollback = %#v, want %#v", afterTask, beforeTask)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back attention recovery and retry creation when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		actor := operatorActorContextForTest("operator:recover-event-rollback")
		taskRecord := taskRecordForTest("task-recover-event-rollback")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-recover-event-rollback", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		sessionID := registerInputQueueSession(t, globalDB)
		claimed := run
		claimed.Status = taskpkg.TaskRunStatusClaimed
		claimed.ClaimedBy = actorForTest(taskpkg.ActorKindAgentSession, sessionID)
		claimed.SessionID = sessionID
		claimed.ClaimTokenHash = "sha256:" + strings.Repeat("c", 64)
		claimed.ClaimedAt = run.QueuedAt.Add(time.Second)
		claimed.HeartbeatAt = claimed.ClaimedAt
		claimed.LeaseUntil = claimed.ClaimedAt.Add(5 * time.Minute)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, claimed); err != nil {
			t.Fatalf("UpdateNonTerminalTaskRun(claimed) error = %v", err)
		}
		input, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-recover-event-rollback",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "stale recovery prompt",
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               claimed.ClaimedAt,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(ctx, run.ID, "operator review", actor); err != nil {
			t.Fatalf("MarkRunNeedsAttention() error = %v", err)
		}
		before, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunRecoveredFromAttention)

		_, err = manager.RecoverRun(ctx, run.ID, taskpkg.RecoverRunRequest{Reason: "resume work"}, actor)
		assertForcedTaskEventInsertError(t, err, "RecoverRun()")
		after, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after) error = %v", err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("source run after rollback = %#v, want %#v", after, before)
		}
		generation, err := globalDB.CurrentSessionInputGeneration(ctx, sessionID)
		if err != nil {
			t.Fatalf("CurrentSessionInputGeneration(after) error = %v", err)
		}
		if generation != 0 {
			t.Fatalf("session generation after rollback = %d, want 0", generation)
		}
		storedInput, err := getSessionInputQueueEntry(ctx, globalDB.db, sessionID, input.ID)
		if err != nil {
			t.Fatalf("getSessionInputQueueEntry(after) error = %v", err)
		}
		if got, want := storedInput.Status, store.SessionInputQueueStatusQueued; got != want {
			t.Fatalf("input status after rollback = %q, want %q", got, want)
		}
		runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) after rollback = %d, want %d", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
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
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(
			ctx,
			parentRun.ID,
			"starved",
			operatorActorContextForTest("operator:parent-rollup"),
		); err != nil {
			t.Fatalf("MarkRunNeedsAttention(parent) error = %v", err)
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
		_, err = globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
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

	t.Run("Should roll back claim settlement when task.run_claimed append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"claim-rollback",
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunClaimed)

		_, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            run.ID,
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-claim-rollback",
			LeaseDuration:    2 * time.Minute,
			Now:              now,
		}, actor)
		assertForcedTaskEventInsertError(t, err, "ClaimNextRun()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("stored run status = %q, want rollback to %q", got, want)
		}
		if storedRun.SessionID != "" || storedRun.ClaimTokenHash != "" ||
			!storedRun.LeaseUntil.IsZero() || !storedRun.HeartbeatAt.IsZero() ||
			!storedRun.ClaimedAt.IsZero() || storedRun.ClaimedBy != nil {
			t.Fatalf("stored run lease fields = %#v, want unclaimed rollback state", storedRun)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("stored task status = %q, want rollback to %q", got, want)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("stored task CurrentRunID = %q, want rollback to empty", storedTask.CurrentRunID)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back direct execution admission when task.run_claimed append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		executor := &terminalRunMutationExecutor{}
		now := time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC)
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(globalDB),
			taskpkg.WithSessionExecutor(executor),
			taskpkg.WithManagerNow(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		actor := operatorActorContextForTest("operator:direct-admission-rollback")
		actor.Authority.CreateGlobal = true
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Direct execution admission rollback",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		beforeTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(before) error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunClaimed)

		_, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
		assertForcedTaskEventInsertError(t, err, "StartRun(direct)")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status.Normalize(), taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("stored run status = %q, want rollback to %q", got, want)
		}
		if storedRun.SessionID != "" || storedRun.ClaimTokenHash != "" ||
			!storedRun.LeaseUntil.IsZero() || !storedRun.HeartbeatAt.IsZero() ||
			!storedRun.ClaimedAt.IsZero() || storedRun.ClaimedBy != nil {
			t.Fatalf("stored direct run = %#v, want unclaimed rollback state", storedRun)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(after) error = %v", err)
		}
		if storedTask.Status != beforeTask.Status ||
			storedTask.CurrentRunID != beforeTask.CurrentRunID ||
			!storedTask.UpdatedAt.Equal(beforeTask.UpdatedAt) {
			t.Fatalf("stored task = %#v, want rollback to %#v", storedTask, beforeTask)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
		if got := executor.startCalls; got != 0 {
			t.Fatalf("StartTaskSession calls after rejected admission = %d, want 0", got)
		}
	})

	t.Run("Should roll back heartbeat settlement when task.run_lease_extended append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, _, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"heartbeat-rollback",
		)
		globalDB.SetTaskEventCommitObserver(nil)
		claim := claimTaskRunLeaseForSettlementTest(
			ctx,
			t,
			manager,
			run,
			actor,
			now,
			"sess-heartbeat-rollback",
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunLeaseExtended)

		_, err := manager.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			RunID:         run.ID,
			ClaimToken:    claim.ClaimToken,
			LeaseDuration: 3 * time.Minute,
			TokensUsed:    17,
			Now:           now.Add(30 * time.Second),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "HeartbeatRunLease()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		assertTaskRunLeaseRollbackState(t, storedRun, claim.Run)
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back release settlement when task.run_released append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"release-rollback",
		)
		globalDB.SetTaskEventCommitObserver(nil)
		claim := claimTaskRunLeaseForSettlementTest(
			ctx,
			t,
			manager,
			run,
			actor,
			now,
			"sess-release-rollback",
		)

		running := claim.Run
		running.Status = taskpkg.TaskRunStatusRunning
		running.StartedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, running); err != nil {
			t.Fatalf("UpdateTaskRun(running) error = %v", err)
		}
		inProgress, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(in progress setup) error = %v", err)
		}
		inProgress.Status = taskpkg.TaskStatusInProgress
		inProgress.UpdatedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateTask(ctx, inProgress, actor); err != nil {
			t.Fatalf("UpdateTask(in progress setup) error = %v", err)
		}

		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunReleased)
		_, err = manager.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Reason:     "failed handoff",
			Now:        now.Add(time.Minute),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "ReleaseRunLease()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		assertTaskRunLeaseRollbackState(t, storedRun, running)
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusInProgress; got != want {
			t.Fatalf("stored task status = %q, want rollback to %q", got, want)
		}
		if got, want := storedTask.CurrentRunID, run.ID; got != want {
			t.Fatalf("stored task CurrentRunID = %q, want rollback to %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should roll back the full failure settlement when status reconciliation append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor, now := seedTaskRunLeaseSettlementForTest(
			ctx,
			t,
			globalDB,
			"failure-reconciliation-rollback",
		)
		globalDB.SetTaskEventCommitObserver(nil)
		claim := claimTaskRunLeaseForSettlementTest(
			ctx,
			t,
			manager,
			run,
			actor,
			now,
			"sess-failure-reconciliation-rollback",
		)

		running := claim.Run
		running.Status = taskpkg.TaskRunStatusRunning
		running.StartedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, running); err != nil {
			t.Fatalf("UpdateTaskRun(running) error = %v", err)
		}
		inProgress, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(in progress setup) error = %v", err)
		}
		inProgress.Status = taskpkg.TaskStatusInProgress
		inProgress.UpdatedAt = now.Add(30 * time.Second)
		if err := globalDB.UpdateTask(ctx, inProgress, actor); err != nil {
			t.Fatalf("UpdateTask(in progress setup) error = %v", err)
		}

		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(
			t,
			globalDB,
			string(hookspkg.HookTaskStatusChanged),
		)
		_, err = manager.FailRunLease(ctx, taskpkg.LeaseFailure{
			RunID:      run.ID,
			ClaimToken: claim.ClaimToken,
			Failure: taskpkg.RunFailure{
				Error:    "worker failed after claiming the run",
				Metadata: json.RawMessage(`{"kind":"worker_failure"}`),
			},
			TokensUsed: 23,
			Now:        now.Add(time.Minute),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "FailRunLease()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		assertTaskRunLeaseRollbackState(t, storedRun, running)
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusInProgress; got != want {
			t.Fatalf("stored task status = %q, want rollback to %q", got, want)
		}
		if got, want := storedTask.CurrentRunID, run.ID; got != want {
			t.Fatalf("stored task CurrentRunID = %q, want rollback to %q", got, want)
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

	t.Run("Should roll back a completion when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"completion-rollback",
			taskpkg.TaskRunStatusRunning,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunCompleted)

		_, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "CompleteRun()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("stored run status = %q, want rollback to %q", got, want)
		}
		if storedRun.Result != nil || !storedRun.EndedAt.IsZero() {
			t.Fatalf(
				"stored completion fields = result %s ended_at %v, want rollback",
				storedRun.Result,
				storedRun.EndedAt,
			)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusInProgress; got != want {
			t.Fatalf("stored task status = %q, want rollback to %q", got, want)
		}
		if got, want := storedTask.CurrentRunID, run.ID; got != want {
			t.Fatalf("stored task CurrentRunID = %q, want rollback to %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should commit a completion and audit together before publication", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"completion-commit",
			taskpkg.TaskRunStatusRunning,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)

		completed, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor)
		if err != nil {
			t.Fatalf("CompleteRun() error = %v", err)
		}
		if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
			t.Fatalf("completed status = %q, want %q", got, want)
		}

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusCompleted; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("stored task status = %q, want %q", got, want)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("stored task CurrentRunID = %q, want cleared after completion", storedTask.CurrentRunID)
		}
		assertTaskRunAuditEventCommittedAfterState(
			ctx,
			t,
			globalDB,
			observer,
			taskRecord.ID,
			run.ID,
			eventspkg.TaskRunCompleted,
			taskpkg.TaskRunStatusCompleted,
		)
	})

	t.Run("Should roll back a failure when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"failure-rollback",
			taskpkg.TaskRunStatusRunning,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunFailed)

		_, err := manager.FailRun(ctx, run.ID, taskpkg.RunFailure{Error: "worker failed"}, actor)
		assertForcedTaskEventInsertError(t, err, "FailRun()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("stored run status = %q, want rollback to %q", got, want)
		}
		if storedRun.Error != "" || !storedRun.EndedAt.IsZero() {
			t.Fatalf("stored failure fields = error %q ended_at %v, want rollback", storedRun.Error, storedRun.EndedAt)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusInProgress; got != want {
			t.Fatalf("stored task status = %q, want rollback to %q", got, want)
		}
		if got, want := storedTask.CurrentRunID, run.ID; got != want {
			t.Fatalf("stored task CurrentRunID = %q, want rollback to %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
	})

	t.Run("Should commit a failure and audit together before publication", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"failure-commit",
			taskpkg.TaskRunStatusRunning,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)

		failed, err := manager.FailRun(ctx, run.ID, taskpkg.RunFailure{Error: "worker failed"}, actor)
		if err != nil {
			t.Fatalf("FailRun() error = %v", err)
		}
		if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("failed status = %q, want %q", got, want)
		}

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("stored task status = %q, want %q", got, want)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("stored task CurrentRunID = %q, want cleared after failure", storedTask.CurrentRunID)
		}
		assertTaskRunAuditEventCommittedAfterState(
			ctx,
			t,
			globalDB,
			observer,
			taskRecord.ID,
			run.ID,
			eventspkg.TaskRunFailed,
			taskpkg.TaskRunStatusFailed,
		)
	})

	t.Run("Should roll back needs attention when its audit append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"needs-attention-rollback",
			taskpkg.TaskRunStatusQueued,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunNeedsAttention)

		_, err := manager.MarkRunNeedsAttention(ctx, run.ID, "subprocess unhealthy", actor)
		assertForcedTaskEventInsertError(t, err, "MarkRunNeedsAttention()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("stored run status = %q, want rollback to %q", got, want)
		}
		if got := len(observer.records); got != 0 {
			t.Fatalf("len(observer.records) after rollback = %d, want 0", got)
		}
		storedEvents, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: eventspkg.TaskRunNeedsAttention,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if got := len(storedEvents); got != 0 {
			t.Fatalf("needs_attention audit events after rollback = %d, want 0", got)
		}
	})

	t.Run("Should commit needs attention and audit together before publication", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		manager, taskRecord, run, actor := seedAuthoritativeTaskRunAuditMutationForTest(
			ctx,
			t,
			globalDB,
			"needs-attention-commit",
			taskpkg.TaskRunStatusQueued,
		)
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)

		updated, err := manager.MarkRunNeedsAttention(ctx, run.ID, "subprocess unhealthy", actor)
		if err != nil {
			t.Fatalf("MarkRunNeedsAttention() error = %v", err)
		}
		if got, want := updated.Status, taskpkg.TaskRunStatusNeedsAttention; got != want {
			t.Fatalf("updated status = %q, want %q", got, want)
		}
		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusNeedsAttention; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
		assertTaskRunAuditEventCommittedAfterState(
			ctx,
			t,
			globalDB,
			observer,
			taskRecord.ID,
			run.ID,
			eventspkg.TaskRunNeedsAttention,
			taskpkg.TaskRunStatusNeedsAttention,
		)
	})
}

func assertForcedTaskEventInsertError(t *testing.T, err error, operation string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), "forced task event insert failure") {
		t.Fatalf("%s error = %v, want forced task event insert failure", operation, err)
	}
}

func seedAuthoritativeTaskRunAuditMutationForTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
	status taskpkg.RunStatus,
) (*taskpkg.Service, taskpkg.Task, taskpkg.Run, taskpkg.ActorContext) {
	t.Helper()

	actor := operatorActorContextForTest("operator:audit-" + suffix)
	taskRecord := taskRecordForTest("task-authoritative-audit-" + suffix)
	if status.Normalize() != taskpkg.TaskRunStatusQueued {
		taskRecord.Status = taskpkg.TaskStatusInProgress
	}
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-authoritative-audit-"+suffix, taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	if status.Normalize() != taskpkg.TaskRunStatusQueued {
		run.Status = status
		run.StartedAt = run.QueuedAt.Add(time.Minute)
		if err := globalDB.UpdateNonTerminalTaskRun(ctx, run); err != nil {
			t.Fatalf("UpdateTaskRun() error = %v", err)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		storedTask.CurrentRunID = run.ID
		storedTask.UpdatedAt = run.StartedAt
		if err := globalDB.UpdateTask(ctx, storedTask, actor); err != nil {
			t.Fatalf("UpdateTask(in progress) error = %v", err)
		}
		taskRecord = storedTask
	}
	manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, taskRecord, run, actor
}

func assertTaskRunAuditEventCommittedAfterState(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	observer *recordingTaskEventCommitObserver,
	taskID string,
	runID string,
	eventType string,
	wantRunStatus taskpkg.RunStatus,
) {
	t.Helper()

	if observer.err != nil {
		t.Fatalf("post-commit observer error = %v", observer.err)
	}
	events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
		TaskID:    taskID,
		RunID:     runID,
		EventType: eventType,
	})
	if err != nil {
		t.Fatalf("ListTaskEvents(%q) error = %v", eventType, err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("%s audit event count = %d, want %d", eventType, got, want)
	}

	for _, record := range observer.records {
		if record.Event.EventType != eventType {
			continue
		}
		if record.Event.TaskID != taskID || record.Event.RunID != runID {
			t.Fatalf(
				"published audit anchors = task %q run %q, want task %q run %q",
				record.Event.TaskID,
				record.Event.RunID,
				taskID,
				runID,
			)
		}
		observedRun, ok := observer.runs[record.Event.ID]
		if !ok {
			t.Fatalf("published %s event did not observe its committed run", eventType)
		}
		if got, want := observedRun.Status, wantRunStatus; got != want {
			t.Fatalf("run observed at %s publication = %q, want committed %q", eventType, got, want)
		}
		return
	}
	t.Fatalf("published observer events = %#v, want %s", observer.records, eventType)
}

func TestGlobalDBNonLeaseTerminalRunCommandsShouldFenceAndRecover(t *testing.T) {
	t.Parallel()

	// Invariant: only the command admitted for a running run may terminalize it;
	// the losing command cannot stop its session or append an audit event.
	// Owning layer: GlobalDB-backed task terminal mutation. Canonical suite:
	// global_db_task_event_tx_test.go.
	t.Run("Should admit exactly one terminal command against completion", func(t *testing.T) {
		t.Parallel()

		for _, contender := range []struct {
			name string
			call func(context.Context, *taskpkg.Service, string, taskpkg.ActorContext) error
		}{
			{
				name: "failure",
				call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
					_, err := manager.FailRun(ctx, runID, taskpkg.RunFailure{Error: "competing failure"}, actor)
					return err
				},
			},
			{
				name: "needs attention",
				call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
					_, err := manager.MarkRunNeedsAttention(ctx, runID, "competing diagnostic", actor)
					return err
				},
			},
			{
				name: "cancellation",
				call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
					_, err := manager.CancelRun(ctx, runID, taskpkg.CancelRun{Reason: "competing cancel"}, actor)
					return err
				},
			},
		} {
			t.Run("Should reject "+contender.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				globalDB := openTestGlobalDB(t)
				taskRecord, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
					ctx,
					t,
					globalDB,
					"completion-race-"+strings.ReplaceAll(contender.name, " ", "-"),
				)
				releaseRequest := make(chan struct{})
				executor := &terminalRunMutationExecutor{
					requestEntered: make(chan struct{}, 1),
					releaseRequest: releaseRequest,
				}
				manager, err := taskpkg.NewManager(
					taskpkg.WithStore(globalDB),
					taskpkg.WithSessionExecutor(executor),
					taskpkg.WithCancelGracePeriod(0),
				)
				if err != nil {
					t.Fatalf("NewManager() error = %v", err)
				}
				contenderExecutor := &terminalRunMutationExecutor{}
				contenderManager, err := taskpkg.NewManager(
					taskpkg.WithStore(globalDB),
					taskpkg.WithSessionExecutor(contenderExecutor),
					taskpkg.WithCancelGracePeriod(0),
				)
				if err != nil {
					t.Fatalf("NewManager(contender) error = %v", err)
				}

				completionResult := make(chan error, 1)
				go func() {
					_, commandErr := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
						Value: json.RawMessage(`{"ok":true}`),
					}, actor)
					completionResult <- commandErr
				}()
				select {
				case <-executor.requestEntered:
				case <-ctx.Done():
					t.Fatalf("CompleteRun() did not reach the session stop: %v", ctx.Err())
				}

				contenderStarted := make(chan struct{})
				contenderResult := make(chan error, 1)
				go func() {
					close(contenderStarted)
					contenderResult <- contender.call(ctx, contenderManager, run.ID, actor)
				}()
				select {
				case <-contenderStarted:
				case <-ctx.Done():
					t.Fatalf("%s contender did not start: %v", contender.name, ctx.Err())
				}
				select {
				case commandErr := <-contenderResult:
					if !errors.Is(commandErr, taskpkg.ErrTerminalRunCommandInProgress) {
						t.Fatalf(
							"%s contender error = %v, want %v",
							contender.name,
							commandErr,
							taskpkg.ErrTerminalRunCommandInProgress,
						)
					}
				case <-ctx.Done():
					t.Fatalf("%s contender did not lose durable admission: %v", contender.name, ctx.Err())
				}

				close(releaseRequest)
				if commandErr := <-completionResult; commandErr != nil {
					t.Fatalf("CompleteRun() error = %v", commandErr)
				}

				storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
				if err != nil {
					t.Fatalf("GetTaskRun() error = %v", err)
				}
				if got, want := storedRun.Status, taskpkg.TaskRunStatusCompleted; got != want {
					t.Fatalf("stored run status = %q, want %q", got, want)
				}
				events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID, RunID: run.ID})
				if err != nil {
					t.Fatalf("ListTaskEvents() error = %v", err)
				}
				if got, want := len(events), 1; got != want || events[0].EventType != eventspkg.TaskRunCompleted {
					t.Fatalf("terminal events = %#v, want one %q event", events, eventspkg.TaskRunCompleted)
				}
				if got, want := len(executor.requestCalls), 1; got != want {
					t.Fatalf("RequestTaskStop calls = %d, want %d", got, want)
				}
				if got, want := len(executor.forceCalls), 1; got != want {
					t.Fatalf("ForceTaskStop calls = %d, want %d", got, want)
				}
				if got := len(contenderExecutor.requestCalls) + len(contenderExecutor.forceCalls); got != 0 {
					t.Fatalf("losing manager session-stop calls = %d, want 0", got)
				}
			})
		}
	})

	// Invariant: after a successful physical stop, an audit rollback retains the
	// exact stop-confirmed intent; retry settles it without another stop.
	// Owning layer: task terminal command and mutation transaction.
	t.Run("Should retain and retry a stop-confirmed completion after audit rollback", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"post-stop-rollback",
		)
		executor := &terminalRunMutationExecutor{}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(globalDB),
			taskpkg.WithSessionExecutor(executor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		observer := &recordingTaskEventCommitObserver{db: globalDB}
		globalDB.SetTaskEventCommitObserver(observer)
		installTaskEventInsertFailureTriggerForType(t, globalDB, eventspkg.TaskRunCompleted)

		_, err = manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "CompleteRun()")

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("stored run status = %q, want %q after post-stop rollback", got, want)
		}
		if storedRun.Error != "" || storedRun.Result != nil || !storedRun.EndedAt.IsZero() {
			t.Fatalf("stored terminal fields = %#v, want rollback", storedRun)
		}
		completedEvents, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: taskRecord.ID, RunID: run.ID, EventType: eventspkg.TaskRunCompleted,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(completed) error = %v", err)
		}
		if len(completedEvents) != 0 {
			t.Fatalf("completed events = %#v, want rollback", completedEvents)
		}
		commands, err := globalDB.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands() error = %v", err)
		}
		if got, want := len(commands), 1; got != want ||
			commands[0].Phase() != taskpkg.TerminalRunCommandPhaseStopConfirmed {
			t.Fatalf("pending terminal commands = %#v, want one stop-confirmed command", commands)
		}
		if got, want := len(executor.requestCalls), 1; got != want {
			t.Fatalf("RequestTaskStop calls = %d, want %d", got, want)
		}
		if got, want := len(executor.forceCalls), 1; got != want {
			t.Fatalf("ForceTaskStop calls = %d, want %d", got, want)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_task_event_insert`); err != nil {
			t.Fatalf("drop task_event failure trigger error = %v", err)
		}
		completed, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor)
		if err != nil {
			t.Fatalf("CompleteRun(retry) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("CompleteRun(retry) status = %q, want completed", completed.Status)
		}
		if got := len(executor.requestCalls); got != 1 {
			t.Fatalf("RequestTaskStop calls after retry = %d, want 1", got)
		}
		if got := len(executor.forceCalls); got != 1 {
			t.Fatalf("ForceTaskStop calls after retry = %d, want 1", got)
		}
		commands, err = globalDB.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands(after retry) error = %v", err)
		}
		if len(commands) != 0 {
			t.Fatalf("pending terminal commands after retry = %#v, want none", commands)
		}
		assertTaskRunAuditEventCommittedAfterState(
			ctx, t, globalDB, observer, taskRecord.ID, run.ID,
			eventspkg.TaskRunCompleted, taskpkg.TaskRunStatusCompleted,
		)
	})

	// Invariant: once stop_requested is durable, an ambiguous external failure
	// retains the original intent across reopen and excludes a different intent.
	// Owning layer: task service over the GlobalDB command and mutation stores.
	t.Run("Should recover the original stop-requested intent after an ambiguous stop failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		copyCurrentSchemaGlobalDBSeed(t, path)
		first := openGlobalDBForTest(t, path)
		taskRecord, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			first,
			"stop-failure",
		)
		stopErr := errors.New("session stop unavailable")
		executor := &terminalRunMutationExecutor{forceErr: stopErr}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(first),
			taskpkg.WithSessionExecutor(executor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if _, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, actor); !errors.Is(err, stopErr) {
			t.Fatalf("CompleteRun(stop failure) error = %v, want %v", err, stopErr)
		}
		storedRun, err := first.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("stored run status = %q, want %q after stop failure", got, want)
		}
		commands, err := first.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands() error = %v", err)
		}
		if got, want := len(commands), 1; got != want ||
			commands[0].Phase() != taskpkg.TerminalRunCommandPhaseStopRequested {
			t.Fatalf("terminal commands after stop failure = %#v, want one stop-requested command", commands)
		}

		contenderExecutor := &terminalRunMutationExecutor{}
		contender, err := taskpkg.NewManager(
			taskpkg.WithStore(first),
			taskpkg.WithSessionExecutor(contenderExecutor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(contender) error = %v", err)
		}
		if _, err := contender.FailRun(
			ctx,
			run.ID,
			taskpkg.RunFailure{Error: "competing failure"},
			actor,
		); !errors.Is(err, taskpkg.ErrTerminalRunCommandInProgress) {
			t.Fatalf("FailRun(competing intent) error = %v, want %v", err, taskpkg.ErrTerminalRunCommandInProgress)
		}
		if got := len(contenderExecutor.requestCalls) + len(contenderExecutor.forceCalls); got != 0 {
			t.Fatalf("competing intent stop calls = %d, want 0", got)
		}

		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}
		second := openGlobalDBForTest(t, path)
		recoveryExecutor := &terminalRunMutationExecutor{}
		recoveryManager, err := taskpkg.NewManager(
			taskpkg.WithStore(second),
			taskpkg.WithSessionExecutor(recoveryExecutor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(recovery) error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		completed, err := recoveryManager.RecoverTerminalRunCommandOnBoot(
			ctx,
			taskpkg.TerminalRunRecoveryEvidence{
				RunID:       run.ID,
				SessionID:   run.SessionID,
				State:       "stopped",
				Disposition: taskpkg.TerminalRunRecoveryStopObserved,
			},
			daemonActor,
		)
		if err != nil {
			t.Fatalf("RecoverTerminalRunCommandOnBoot() error = %v", err)
		}
		if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
			t.Fatalf("recovered run status = %q, want %q", got, want)
		}
		if got := len(recoveryExecutor.requestCalls) + len(recoveryExecutor.forceCalls); got != 0 {
			t.Fatalf("observed-stop recovery calls = %d, want 0", got)
		}
		events, err := second.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: taskRecord.ID, RunID: run.ID, EventType: eventspkg.TaskRunCompleted,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(completed) error = %v", err)
		}
		if got, want := len(events), 1; got != want || events[0].Actor != actor.Actor {
			t.Fatalf("completed events = %#v, want one event owned by original actor %#v", events, actor.Actor)
		}
		commands, err = second.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands(after recovery) error = %v", err)
		}
		if len(commands) != 0 {
			t.Fatalf("terminal commands after recovery = %#v, want none", commands)
		}
	})

	// Invariant: boot recovery cannot terminalize a run while a daemon-observed
	// subprocess still needs to be stopped; a failed stop remains recoverable.
	// Owning layer: Task boot recovery over real GlobalDB persistence.
	t.Run("Should stop a live orphan before committing boot failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		copyCurrentSchemaGlobalDBSeed(t, path)
		first := openGlobalDBForTest(t, path)
		taskRecord, run, _ := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			first,
			"boot-live-orphan",
		)
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		stopErr := errors.New("orphan process stop unavailable")
		executor := &terminalRunMutationExecutor{forceErr: stopErr}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(first),
			taskpkg.WithSessionExecutor(executor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(first) error = %v", err)
		}
		recovery := taskpkg.RunBootRecovery{
			Action:         taskpkg.RunBootRecoveryFail,
			Reason:         "daemon_restart",
			SessionState:   "stopped",
			Classification: "orphaned",
			Detail:         "recorded subprocess is still alive",
			StopRequired:   true,
		}
		if _, err := manager.RecoverRunOnBoot(ctx, run.ID, recovery, daemonActor); !errors.Is(err, stopErr) {
			t.Fatalf("RecoverRunOnBoot(force failure) error = %v, want %v", err, stopErr)
		}
		stored, err := first.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after force failure) error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("run status after force failure = %q, want %q", got, want)
		}
		commands, err := first.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands() error = %v", err)
		}
		if got, want := len(commands), 1; got != want ||
			commands[0].Phase() != taskpkg.TerminalRunCommandPhaseStopRequested {
			t.Fatalf("pending recovery commands = %#v, want one stop-requested command", commands)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second := openGlobalDBForTest(t, path)
		recoveryExecutor := &terminalRunMutationExecutor{}
		recoveryManager, err := taskpkg.NewManager(
			taskpkg.WithStore(second),
			taskpkg.WithSessionExecutor(recoveryExecutor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(reopened) error = %v", err)
		}
		failed, err := recoveryManager.RecoverTerminalRunCommandOnBoot(
			ctx,
			taskpkg.TerminalRunRecoveryEvidence{
				RunID:       run.ID,
				SessionID:   run.SessionID,
				State:       "stopped-with-live-subprocess",
				Disposition: taskpkg.TerminalRunRecoveryResumeStop,
			},
			daemonActor,
		)
		if err != nil {
			t.Fatalf("RecoverTerminalRunCommandOnBoot() error = %v", err)
		}
		if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("recovered run status = %q, want %q", got, want)
		}
		if got, want := len(recoveryExecutor.requestCalls), 0; got != want {
			t.Fatalf("recovery RequestTaskStop calls = %d, want %d", got, want)
		}
		if got, want := len(recoveryExecutor.forceCalls), 1; got != want {
			t.Fatalf("recovery ForceTaskStop calls = %d, want %d", got, want)
		}
		for _, eventType := range []string{eventspkg.TaskRunFailed, eventspkg.TaskRunRecovered} {
			events, eventErr := second.ListTaskEvents(ctx, taskpkg.EventQuery{
				TaskID: taskRecord.ID, RunID: run.ID, EventType: eventType,
			})
			if eventErr != nil {
				t.Fatalf("ListTaskEvents(%s) error = %v", eventType, eventErr)
			}
			if got, want := len(events), 1; got != want {
				t.Fatalf("%s event count = %d, want %d", eventType, got, want)
			}
		}
	})

	// Invariant: retrying the same needs-attention intent is idempotent on real
	// SQLite and produces one canonical event. Owning layer: task + GlobalDB.
	t.Run("Should persist one needs-attention event across an idempotent retry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		_, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"idempotent-needs-attention",
		)
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		first, err := manager.MarkRunNeedsAttention(ctx, run.ID, "operator intervention", actor)
		if err != nil {
			t.Fatalf("MarkRunNeedsAttention(first) error = %v", err)
		}
		second, err := manager.MarkRunNeedsAttention(ctx, run.ID, "operator intervention", actor)
		if err != nil {
			t.Fatalf("MarkRunNeedsAttention(second) error = %v", err)
		}
		if first.Status != taskpkg.TaskRunStatusNeedsAttention || second.Status != first.Status {
			t.Fatalf("idempotent needs_attention runs = %#v, %#v", first, second)
		}
		events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{
			RunID: run.ID, EventType: eventspkg.TaskRunNeedsAttention,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(needs attention) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("needs_attention events = %d, want %d", got, want)
		}
	})

	// Invariant: a stop-confirmed command survives database close/reopen and
	// daemon recovery settles its original intent without a second stop.
	// Owning layers: GlobalDB command persistence and task boot recovery.
	t.Run("Should resume a stop-confirmed command after reopen without stopping twice", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		copyCurrentSchemaGlobalDBSeed(t, path)
		first := openGlobalDBForTest(t, path)
		taskRecord, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			first,
			"boot-terminal-recovery",
		)
		firstExecutor := &terminalRunMutationExecutor{}
		firstManager, err := taskpkg.NewManager(
			taskpkg.WithStore(first),
			taskpkg.WithSessionExecutor(firstExecutor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(first) error = %v", err)
		}
		installTaskEventInsertFailureTriggerForType(t, first, eventspkg.TaskRunCompleted)
		_, err = firstManager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"boot":true}`),
		}, actor)
		assertForcedTaskEventInsertError(t, err, "CompleteRun(before reopen)")
		if _, err := first.db.ExecContext(ctx, `DROP TRIGGER fail_task_event_insert`); err != nil {
			t.Fatalf("drop task_event failure trigger error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second := openGlobalDBForTest(t, path)
		recoveryExecutor := &terminalRunMutationExecutor{}
		recoveryManager, err := taskpkg.NewManager(
			taskpkg.WithStore(second),
			taskpkg.WithSessionExecutor(recoveryExecutor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager(reopened) error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		pending, err := recoveryManager.ListPendingTerminalRunCommands(ctx, daemonActor)
		if err != nil {
			t.Fatalf("ListPendingTerminalRunCommands() error = %v", err)
		}
		if got, want := len(pending), 1; got != want ||
			pending[0].Phase != taskpkg.TerminalRunCommandPhaseStopConfirmed {
			t.Fatalf("pending terminal commands = %#v, want one stop-confirmed command", pending)
		}
		completed, err := recoveryManager.RecoverTerminalRunCommandOnBoot(
			ctx,
			taskpkg.TerminalRunRecoveryEvidence{
				RunID: run.ID, SessionID: run.SessionID, State: "stopped",
				Disposition: taskpkg.TerminalRunRecoveryStopObserved,
			},
			daemonActor,
		)
		if err != nil {
			t.Fatalf("RecoverTerminalRunCommandOnBoot() error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted ||
			string(completed.Result) != `{"boot":true}` {
			t.Fatalf("recovered run = %#v, want original completion intent", completed)
		}
		if got := len(recoveryExecutor.requestCalls) + len(recoveryExecutor.forceCalls); got != 0 {
			t.Fatalf("recovery session-stop calls = %d, want 0 for stopped evidence", got)
		}
		events, err := second.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID: taskRecord.ID, RunID: run.ID, EventType: eventspkg.TaskRunCompleted,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(completed) error = %v", err)
		}
		if got, want := len(events), 1; got != want || events[0].Actor != actor.Actor {
			t.Fatalf("completed events = %#v, want one event owned by original actor %#v", events, actor.Actor)
		}
		pending, err = recoveryManager.ListPendingTerminalRunCommands(ctx, daemonActor)
		if err != nil {
			t.Fatalf("ListPendingTerminalRunCommands(after recovery) error = %v", err)
		}
		if len(pending) != 0 {
			t.Fatalf("pending terminal commands after recovery = %#v, want none", pending)
		}
	})

	// Invariant: a stale stop failure cannot release a command that another
	// recovery path has already advanced. Owning layer: terminal command store.
	t.Run("Should reject stale release after the command phase advances", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		_, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"stale-release-phase",
		)
		command := terminalRunCommandForEventTransactionTest(
			t,
			run,
			actor,
			"terminal-command-stale-release",
		)
		reserved, inserted, err := globalDB.ReserveTerminalRunCommand(ctx, command)
		if err != nil || !inserted {
			t.Fatalf("ReserveTerminalRunCommand() = inserted %t, error %v", inserted, err)
		}
		advanced, err := globalDB.AdvanceTerminalRunCommandPhase(
			ctx,
			reserved,
			taskpkg.TerminalRunCommandPhaseStopRequested,
			command.CommandAt().Add(time.Second),
		)
		if err != nil {
			t.Fatalf("AdvanceTerminalRunCommandPhase() error = %v", err)
		}
		if err := globalDB.ReleaseTerminalRunCommand(ctx, reserved); !errors.Is(
			err,
			taskpkg.ErrTerminalRunCommandInProgress,
		) {
			t.Fatalf(
				"ReleaseTerminalRunCommand(stale) error = %v, want %v",
				err,
				taskpkg.ErrTerminalRunCommandInProgress,
			)
		}
		pending, err := globalDB.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands() error = %v", err)
		}
		if got, want := len(pending), 1; got != want || pending[0].Phase() != advanced.Phase() {
			t.Fatalf("pending terminal commands = %#v, want advanced command", pending)
		}
		if err := globalDB.ReleaseTerminalRunCommand(ctx, advanced); err != nil {
			t.Fatalf("ReleaseTerminalRunCommand(advanced) error = %v", err)
		}
	})

	// Invariant: ambiguous boot evidence leaves the original command and run
	// untouched so generic recovery cannot invent a terminal outcome.
	t.Run("Should preserve an admitted command when boot evidence is ambiguous", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		_, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
			ctx,
			t,
			globalDB,
			"ambiguous-boot-evidence",
		)
		command := terminalRunCommandForEventTransactionTest(
			t,
			run,
			actor,
			"terminal-command-ambiguous-boot",
		)
		reserved, inserted, err := globalDB.ReserveTerminalRunCommand(ctx, command)
		if err != nil || !inserted {
			t.Fatalf("ReserveTerminalRunCommand() = inserted %t, error %v", inserted, err)
		}
		executor := &terminalRunMutationExecutor{}
		manager, err := taskpkg.NewManager(
			taskpkg.WithStore(globalDB),
			taskpkg.WithSessionExecutor(executor),
			taskpkg.WithCancelGracePeriod(0),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		if _, err := manager.RecoverTerminalRunCommandOnBoot(
			ctx,
			taskpkg.TerminalRunRecoveryEvidence{
				RunID: run.ID, SessionID: run.SessionID, State: "unknown",
				Disposition: taskpkg.TerminalRunRecoveryAmbiguous,
			},
			daemonActor,
		); !errors.Is(err, taskpkg.ErrTerminalRunCommandInProgress) {
			t.Fatalf(
				"RecoverTerminalRunCommandOnBoot(ambiguous) error = %v, want %v",
				err,
				taskpkg.ErrTerminalRunCommandInProgress,
			)
		}
		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
		pending, err := globalDB.ListTerminalRunCommands(ctx)
		if err != nil {
			t.Fatalf("ListTerminalRunCommands() error = %v", err)
		}
		if got, want := len(pending), 1; got != want || pending[0].Phase() != reserved.Phase() {
			t.Fatalf("pending terminal commands = %#v, want original admitted command", pending)
		}
		if got := len(executor.requestCalls) + len(executor.forceCalls); got != 0 {
			t.Fatalf("ambiguous recovery session-stop calls = %d, want 0", got)
		}
		if err := globalDB.ReleaseTerminalRunCommand(ctx, reserved); err != nil {
			t.Fatalf("ReleaseTerminalRunCommand(cleanup) error = %v", err)
		}
	})
}

func TestGlobalDBTerminalRunCommandsShouldSerializeAcrossDatabaseHandles(t *testing.T) {
	t.Parallel()

	// Invariant: the durable terminal-command row plus the full run fence admits
	// one authoritative command across independent processes/handles. The loser
	// cannot stop the session or append its event. Owning layer: GlobalDB-backed
	// task mutation boundary. Canonical suite: global_db_task_event_tx_test.go.
	commands := terminalRunCommandsForCrossHandleTest()
	for _, winner := range commands {
		for _, loser := range commands {
			if winner.name == loser.name {
				continue
			}
			t.Run("Should admit "+winner.name+" before "+loser.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				path := filepath.Join(t.TempDir(), GlobalDatabaseName)
				copyCurrentSchemaGlobalDBSeed(t, path)
				winnerDB := openGlobalDBForTest(t, path)
				loserDB := openGlobalDBForTest(t, path)
				taskRecord, run, actor := seedNonLeaseRunningTerminalRunForEventTransaction(
					ctx,
					t,
					winnerDB,
					strings.ReplaceAll("cross-handle-"+winner.name+"-before-"+loser.name, " ", "-"),
				)

				releaseWinner := make(chan struct{})
				winnerStore := &blockingTerminalRunCommandStore{
					GlobalDB: winnerDB,
					entered:  make(chan struct{}),
					release:  releaseWinner,
				}
				winnerExecutor := &terminalRunMutationExecutor{}
				loserExecutor := &terminalRunMutationExecutor{}
				winnerManager, err := taskpkg.NewManager(
					taskpkg.WithStore(winnerStore),
					taskpkg.WithSessionExecutor(winnerExecutor),
					taskpkg.WithCancelGracePeriod(0),
				)
				if err != nil {
					t.Fatalf("NewManager(winner) error = %v", err)
				}
				loserManager, err := taskpkg.NewManager(
					taskpkg.WithStore(loserDB),
					taskpkg.WithSessionExecutor(loserExecutor),
					taskpkg.WithCancelGracePeriod(0),
				)
				if err != nil {
					t.Fatalf("NewManager(loser) error = %v", err)
				}

				winnerResult := make(chan error, 1)
				go func() {
					winnerResult <- winner.call(ctx, winnerManager, run.ID, actor)
				}()
				select {
				case <-winnerStore.entered:
				case <-ctx.Done():
					t.Fatalf("%s winner did not persist terminal admission: %v", winner.name, ctx.Err())
				}
				if _, metadataErr := loserDB.UpdateTaskRunMetadata(ctx, taskpkg.RunMetadataMutation{
					RunID: run.ID, ExpectedMetadata: run.Metadata, Metadata: run.Metadata,
				}); !errors.Is(metadataErr, taskpkg.ErrTerminalRunCommandInProgress) {
					t.Fatalf(
						"UpdateTaskRunMetadata(during admission) error = %v, want %v",
						metadataErr,
						taskpkg.ErrTerminalRunCommandInProgress,
					)
				}
				if deleteErr := loserDB.DeleteTask(ctx, taskRecord.ID); !errors.Is(
					deleteErr,
					taskpkg.ErrTerminalRunCommandInProgress,
				) {
					t.Fatalf(
						"DeleteTask(during admission) error = %v, want %v",
						deleteErr,
						taskpkg.ErrTerminalRunCommandInProgress,
					)
				}
				if _, scopeErr := sqlcgen.New(loserDB.db).GetTaskRunTerminalCommandByScope(
					ctx,
					sqlcgen.GetTaskRunTerminalCommandByScopeParams{
						RunID: run.ID, TaskID: run.TaskID, WorkspaceID: "workspace-foreign",
					},
				); !errors.Is(scopeErr, sql.ErrNoRows) {
					t.Fatalf("foreign workspace terminal-command lookup error = %v, want no rows", scopeErr)
				}

				loserResult := make(chan error, 1)
				go func() {
					loserResult <- loser.call(ctx, loserManager, run.ID, actor)
				}()
				select {
				case commandErr := <-loserResult:
					if !errors.Is(commandErr, taskpkg.ErrTerminalRunCommandInProgress) {
						t.Fatalf(
							"%s loser error = %v, want %v",
							loser.name,
							commandErr,
							taskpkg.ErrTerminalRunCommandInProgress,
						)
					}
				case <-ctx.Done():
					t.Fatalf("%s loser did not observe durable admission: %v", loser.name, ctx.Err())
				}
				close(releaseWinner)

				if commandErr := <-winnerResult; commandErr != nil {
					t.Fatalf("%s winner error = %v", winner.name, commandErr)
				}
				storedRun, err := winnerDB.GetTaskRun(ctx, run.ID)
				if err != nil {
					t.Fatalf("GetTaskRun() error = %v", err)
				}
				if got, want := storedRun.Status.Normalize(), winner.status; got != want {
					t.Fatalf("stored run status = %q, want winner status %q", got, want)
				}
				events, err := winnerDB.ListTaskEvents(ctx, taskpkg.EventQuery{
					TaskID: taskRecord.ID,
					RunID:  run.ID,
				})
				if err != nil {
					t.Fatalf("ListTaskEvents() error = %v", err)
				}
				if got := countTaskEventsForType(events, loser.eventType); got != 0 {
					t.Fatalf("losing %s events = %d, want 0", loser.eventType, got)
				}
				if got := countTaskEventsForType(events, winner.eventType); got != 1 {
					t.Fatalf("winning %s events = %d, want 1", winner.eventType, got)
				}
				if got := len(loserExecutor.requestCalls) + len(loserExecutor.forceCalls); got != 0 {
					t.Fatalf("losing manager session-stop calls = %d, want 0", got)
				}
				wantWinnerStops := 0
				if winner.stopsSession {
					wantWinnerStops = 2
				}
				if got := len(winnerExecutor.requestCalls) + len(winnerExecutor.forceCalls); got != wantWinnerStops {
					t.Fatalf("winning manager session-stop calls = %d, want %d", got, wantWinnerStops)
				}
			})
		}
	}
}

func TestGlobalDBRunMutationTransactionsShouldEnforceContract(t *testing.T) {
	t.Parallel()

	// Invariant: the GlobalDB adapter rejects an absent mutation capability, and
	// accepted commands compare workspace, task, status, session, claim hash,
	// and lease generation as one indivisible snapshot. Owning layer: GlobalDB
	// task mutation transaction. Canonical suite: global_db_task_event_tx_test.go.
	t.Run("Should reject a nil mutation capability without panicking", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		err := globalDB.WithTaskMutationTransaction(ctx, "reject nil mutation", nil)
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("WithTaskMutationTransaction(nil) error = %v, want %v", err, taskpkg.ErrValidation)
		}
	})

	fences := []struct {
		name   string
		mutate func(*taskpkg.Run)
	}{
		{name: "status", mutate: func(run *taskpkg.Run) { run.Status = taskpkg.TaskRunStatusStarting }},
		{name: "task", mutate: func(run *taskpkg.Run) { run.TaskID += "-stale" }},
		{name: "workspace", mutate: func(run *taskpkg.Run) { run.WorkspaceID = "workspace-stale" }},
		{name: "session", mutate: func(run *taskpkg.Run) { run.SessionID = "sess-stale" }},
		{name: "claim hash", mutate: func(run *taskpkg.Run) { run.ClaimTokenHash = "sha256:stale" }},
		{name: "lease", mutate: func(run *taskpkg.Run) { run.LeaseUntil = run.LeaseUntil.Add(time.Second) }},
	}
	operations := []struct {
		name string
		call func(context.Context, *taskMutationTxStore, taskpkg.Run, taskpkg.Run) error
	}{
		{
			name: "terminal transition",
			call: func(
				ctx context.Context,
				store *taskMutationTxStore,
				expected taskpkg.Run,
				next taskpkg.Run,
			) error {
				_, err := store.TransitionTerminalRun(
					ctx,
					taskpkg.NewTerminalRunMutation(expected, next),
				)
				return err
			},
		},
		{
			name: "needs attention",
			call: func(
				ctx context.Context,
				store *taskMutationTxStore,
				expected taskpkg.Run,
				_ taskpkg.Run,
			) error {
				_, err := store.MarkRunNeedsAttentionMutation(
					ctx,
					taskpkg.NewRunNeedsAttentionCommand(expected, "operator review"),
				)
				return err
			},
		},
	}
	for _, operation := range operations {
		for _, fence := range fences {
			t.Run("Should reject stale "+fence.name+" for "+operation.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				globalDB := openTestGlobalDB(t)
				running := seedLeasedRunningRunForMutationFenceTest(ctx, t, globalDB, operation.name+"-"+fence.name)
				expected := running
				fence.mutate(&expected)
				next := running
				next.Status = taskpkg.TaskRunStatusCanceled
				next.LeaseUntil = time.Time{}
				next.HeartbeatAt = time.Time{}
				next.EndedAt = running.StartedAt.Add(time.Minute)

				err := globalDB.withTaskMutationTransactionForTest(
					ctx,
					"reject stale run mutation",
					func(store *taskMutationTxStore) error {
						return operation.call(ctx, store, expected, next)
					},
				)
				if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
					t.Fatalf("stale %s error = %v, want %v", fence.name, err, taskpkg.ErrInvalidStatusTransition)
				}
				stored, err := globalDB.GetTaskRun(ctx, running.ID)
				if err != nil {
					t.Fatalf("GetTaskRun() error = %v", err)
				}
				if stored.Status.Normalize() != taskpkg.TaskRunStatusRunning ||
					stored.SessionID != running.SessionID ||
					stored.ClaimTokenHash != running.ClaimTokenHash ||
					!stored.LeaseUntil.Equal(running.LeaseUntil) {
					t.Fatalf("stored run = %#v, want exact running ownership snapshot", stored)
				}
				events, err := globalDB.ListTaskEvents(ctx, taskpkg.EventQuery{RunID: running.ID})
				if err != nil {
					t.Fatalf("ListTaskEvents() error = %v", err)
				}
				if len(events) != 0 {
					t.Fatalf("stale mutation events = %#v, want none", events)
				}
			})
		}
	}
}

func TestGlobalDBFailTasklessRunOnBootShouldEnforceDedicatedBoundary(t *testing.T) {
	t.Parallel()

	// Invariant: boot recovery may fail a durable taskless network wake through
	// its explicit full-fence boundary, but it cannot use that boundary to
	// terminalize a task-anchored run. Owning layer: GlobalDB task-run lifecycle.
	// Canonical suite: global_db_task_event_tx_test.go.
	t.Run("Should fail a taskless network wake using its exact ownership fence", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		registerNetworkWakeRunSessionsForClaimTest(t, globalDB, now, "sess-taskless-boot")
		wake := networkWakeRunForClaimTest(
			"run-taskless-boot-failure",
			"wake-taskless-boot-failure",
			"sess-taskless-boot",
			"owner-taskless-boot",
			now,
		)
		createNetworkWakeRunForClaimTest(t, globalDB, wake, now)
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:                 wake.ID,
			RunKind:               taskpkg.RunKindNetworkWake,
			Scope:                 taskpkg.ScopeWorkspace,
			WorkspaceID:           wake.WorkspaceID,
			TargetSessionID:       "sess-taskless-boot",
			ClaimerSessionID:      "sess-taskless-boot",
			LeaseDuration:         time.Minute,
			Now:                   now,
			WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(taskless wake) error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "boot-recovery")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		runningResult, err := recoverNetworkWakeOnBootForTest(
			ctx,
			globalDB,
			claim.Run,
			taskpkg.RunBootRecovery{Action: taskpkg.RunBootRecoveryMarkRunning},
			daemonActor,
			now.Add(time.Second),
		)
		if err != nil {
			t.Fatalf("RecoverNetworkWakeOnBoot(mark running) error = %v", err)
		}
		failureReason := "orphaned taskless wake on boot"
		expectedFailure := `orphaned on boot: session "sess-taskless-boot" is missing`
		failedResult, err := recoverNetworkWakeOnBootForTest(
			ctx,
			globalDB,
			runningResult.Run,
			taskpkg.RunBootRecovery{
				Action:       taskpkg.RunBootRecoveryFail,
				Reason:       failureReason,
				SessionState: "missing",
			},
			daemonActor,
			now.Add(2*time.Second),
		)
		if err != nil {
			t.Fatalf("RecoverNetworkWakeOnBoot(fail) error = %v", err)
		}
		updated := failedResult.Run
		if updated.Status.Normalize() != taskpkg.TaskRunStatusFailed ||
			updated.TaskID != "" ||
			!updated.IsNetworkWake() ||
			updated.WorkspaceID != wake.WorkspaceID ||
			updated.Error != expectedFailure ||
			!updated.LeaseUntil.IsZero() ||
			!updated.HeartbeatAt.IsZero() {
			t.Fatalf("updated taskless wake = %#v, want failed wake with released lease", updated)
		}
		if got := networkWakeEventCount(t, globalDB, wake.NetworkWakeID, networkWakeEventRecovered); got != 2 {
			t.Fatalf("recovered event count = %d, want 2", got)
		}
	})

	t.Run("Should reject a task-anchored run at the taskless recovery boundary", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		running := seedLeasedRunningRunForMutationFenceTest(ctx, t, globalDB, "task-anchored-boot-failure")
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "boot-recovery")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		_, err = recoverNetworkWakeOnBootForTest(
			ctx,
			globalDB,
			running,
			taskpkg.RunBootRecovery{
				Action: taskpkg.RunBootRecoveryFail,
				Reason: "forged taskless recovery",
			},
			daemonActor,
			running.StartedAt.Add(time.Minute),
		)
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf(
				"RecoverNetworkWakeOnBoot(task anchored) error = %v, want %v",
				err,
				taskpkg.ErrInvalidStatusTransition,
			)
		}
		stored, err := globalDB.GetTaskRun(ctx, running.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(task anchored) error = %v", err)
		}
		if stored.Status.Normalize() != taskpkg.TaskRunStatusRunning ||
			stored.RunKind.Normalize() != running.RunKind.Normalize() ||
			stored.Error != running.Error ||
			!stored.LeaseUntil.Equal(running.LeaseUntil) {
			t.Fatalf("stored task-anchored run = %#v, want unchanged running snapshot", stored)
		}
	})

	t.Run("Should roll back the taskless run when its recovered ledger event fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC)
		registerNetworkWakeRunSessionsForClaimTest(t, globalDB, now, "sess-taskless-rollback")
		wake := networkWakeRunForClaimTest(
			"run-taskless-rollback",
			"wake-taskless-rollback",
			"sess-taskless-rollback",
			"owner-taskless-rollback",
			now,
		)
		createNetworkWakeRunForClaimTest(t, globalDB, wake, now)
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:                 wake.ID,
			RunKind:               taskpkg.RunKindNetworkWake,
			Scope:                 taskpkg.ScopeWorkspace,
			WorkspaceID:           wake.WorkspaceID,
			TargetSessionID:       "sess-taskless-rollback",
			ClaimerSessionID:      "sess-taskless-rollback",
			LeaseDuration:         time.Minute,
			Now:                   now,
			WorkspaceActiveRunCap: 1,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun(taskless wake) error = %v", err)
		}
		daemonActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "boot-recovery")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		installNetworkWakeEventInsertFailureTriggerForType(t, globalDB, networkWakeEventRecovered)

		_, err = recoverNetworkWakeOnBootForTest(
			ctx,
			globalDB,
			claim.Run,
			taskpkg.RunBootRecovery{Action: taskpkg.RunBootRecoveryMarkRunning},
			daemonActor,
			now.Add(time.Second),
		)
		if err == nil {
			t.Fatal("RecoverNetworkWakeOnBoot() error = nil, want recovered ledger failure")
		}
		stored, err := globalDB.GetTaskRun(ctx, claim.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if stored.Status.Normalize() != claim.Run.Status.Normalize() ||
			stored.SessionID != claim.Run.SessionID ||
			stored.ClaimTokenHash != claim.Run.ClaimTokenHash ||
			!stored.LeaseUntil.Equal(claim.Run.LeaseUntil) ||
			!stored.StartedAt.Equal(claim.Run.StartedAt) {
			t.Fatalf("stored taskless wake = %#v, want exact claimed snapshot %#v", stored, claim.Run)
		}
		if got := networkWakeEventCount(t, globalDB, wake.NetworkWakeID, networkWakeEventRecovered); got != 0 {
			t.Fatalf("recovered event count = %d, want 0", got)
		}
	})
}

func recoverNetworkWakeOnBootForTest(
	ctx context.Context,
	globalDB *GlobalDB,
	previous taskpkg.Run,
	recovery taskpkg.RunBootRecovery,
	actor taskpkg.ActorContext,
	now time.Time,
) (result taskpkg.NominalRunMutationResult, err error) {
	err = globalDB.withTaskMutationTransactionForTest(
		ctx,
		"test recover network wake on boot",
		func(store *taskMutationTxStore) error {
			result, err = store.RecoverNetworkWakeOnBoot(
				ctx,
				taskpkg.NewNetworkWakeBootRecoveryMutation(previous, recovery, actor, now),
			)
			return err
		},
	)
	return result, err
}

type terminalRunCommandForCrossHandleTest struct {
	name         string
	status       taskpkg.RunStatus
	eventType    string
	stopsSession bool
	call         func(context.Context, *taskpkg.Service, string, taskpkg.ActorContext) error
}

func terminalRunCommandsForCrossHandleTest() []terminalRunCommandForCrossHandleTest {
	return []terminalRunCommandForCrossHandleTest{
		{
			name: "completion", status: taskpkg.TaskRunStatusCompleted,
			eventType: eventspkg.TaskRunCompleted, stopsSession: true,
			call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
				_, err := manager.CompleteRun(
					ctx,
					runID,
					taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
					actor,
				)
				return err
			},
		},
		{
			name: "failure", status: taskpkg.TaskRunStatusFailed,
			eventType: eventspkg.TaskRunFailed, stopsSession: true,
			call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
				_, err := manager.FailRun(ctx, runID, taskpkg.RunFailure{Error: "worker failed"}, actor)
				return err
			},
		},
		{
			name: "needs attention", status: taskpkg.TaskRunStatusNeedsAttention,
			eventType: eventspkg.TaskRunNeedsAttention,
			call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
				_, err := manager.MarkRunNeedsAttention(ctx, runID, "operator review", actor)
				return err
			},
		},
		{
			name: "cancellation", status: taskpkg.TaskRunStatusCanceled,
			eventType: eventspkg.TaskRunCanceled, stopsSession: true,
			call: func(ctx context.Context, manager *taskpkg.Service, runID string, actor taskpkg.ActorContext) error {
				_, err := manager.CancelRun(ctx, runID, taskpkg.CancelRun{Reason: "operator canceled"}, actor)
				return err
			},
		},
	}
}

type blockingTerminalRunCommandStore struct {
	*GlobalDB
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (s *blockingTerminalRunCommandStore) ReserveTerminalRunCommand(
	ctx context.Context,
	command taskpkg.TerminalRunCommand,
) (taskpkg.TerminalRunCommand, bool, error) {
	authoritative, inserted, err := s.GlobalDB.ReserveTerminalRunCommand(ctx, command)
	if err != nil || !inserted {
		return authoritative, inserted, err
	}
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
		return authoritative, inserted, nil
	case <-ctx.Done():
		return taskpkg.TerminalRunCommand{}, false, ctx.Err()
	}
}

func countTaskEventsForType(events []taskpkg.Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}

func seedLeasedRunningRunForMutationFenceTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
) taskpkg.Run {
	t.Helper()

	taskRecord := taskRecordForTest("task-fence-" + strings.ReplaceAll(suffix, " ", "-"))
	taskRecord.Status = taskpkg.TaskStatusReady
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-fence-"+strings.ReplaceAll(suffix, " ", "-"), taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            run.ID,
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: "sess-fence-" + strings.ReplaceAll(suffix, " ", "-"),
		LeaseDuration:    5 * time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	running := claim.Run
	running.Status = taskpkg.TaskRunStatusRunning
	running.StartedAt = now.Add(time.Second)
	if err := globalDB.UpdateNonTerminalTaskRun(ctx, running); err != nil {
		t.Fatalf("UpdateNonTerminalTaskRun(running) error = %v", err)
	}
	return running
}

type terminalRunMutationExecutor struct {
	requestEntered chan struct{}
	releaseRequest <-chan struct{}
	requestErr     error
	forceErr       error
	requestCalls   []string
	forceCalls     []string
	startCalls     int
}

func (e *terminalRunMutationExecutor) StartTaskSession(
	_ context.Context,
	_ *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	e.startCalls++
	return &taskpkg.SessionRef{SessionID: "unused-terminal-mutation-session"}, nil
}

func (e *terminalRunMutationExecutor) AttachTaskSession(
	_ context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: sessionID}, nil
}

func (e *terminalRunMutationExecutor) RequestTaskStop(
	ctx context.Context,
	sessionID string,
	_ taskpkg.StopReason,
) error {
	e.requestCalls = append(e.requestCalls, sessionID)
	if e.requestEntered != nil {
		select {
		case e.requestEntered <- struct{}{}:
		default:
		}
	}
	if e.releaseRequest != nil {
		select {
		case <-e.releaseRequest:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return e.requestErr
}

func (e *terminalRunMutationExecutor) ForceTaskStop(
	_ context.Context,
	sessionID string,
	_ taskpkg.StopReason,
) error {
	e.forceCalls = append(e.forceCalls, sessionID)
	return e.forceErr
}

func seedNonLeaseRunningTerminalRunForEventTransaction(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	suffix string,
) (taskpkg.Task, taskpkg.Run, taskpkg.ActorContext) {
	t.Helper()

	actor := operatorActorContextForTest("operator:terminal-" + suffix)
	taskRecord := taskRecordForTest("task-terminal-" + suffix)
	taskRecord.Status = taskpkg.TaskStatusInProgress
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskRunForTest("run-terminal-"+suffix, taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	claimedBy := actor.Actor
	run.Status = taskpkg.TaskRunStatusRunning
	run.ClaimedBy = &claimedBy
	run.ClaimedAt = run.QueuedAt.Add(time.Minute)
	run.StartedAt = run.ClaimedAt.Add(time.Minute)
	run.SessionID = "sess-terminal-" + suffix
	if err := globalDB.UpdateNonTerminalTaskRun(ctx, run); err != nil {
		t.Fatalf("UpdateTaskRun(running) error = %v", err)
	}
	taskRecord.CurrentRunID = run.ID
	taskRecord.UpdatedAt = run.StartedAt
	if err := globalDB.UpdateTask(ctx, taskRecord, actor); err != nil {
		t.Fatalf("UpdateTask(current run) error = %v", err)
	}
	return taskRecord, run, actor
}

func terminalRunCommandForEventTransactionTest(
	t *testing.T,
	run taskpkg.Run,
	actor taskpkg.ActorContext,
	commandID string,
) taskpkg.TerminalRunCommand {
	t.Helper()

	actorJSON, err := json.Marshal(actor)
	if err != nil {
		t.Fatalf("json.Marshal(actor) error = %v", err)
	}
	commandAt := time.Now().UTC()
	command, err := taskpkg.RestoreTerminalRunCommand(taskpkg.TerminalRunCommandRecord{
		CommandID:        commandID,
		RunID:            run.ID,
		TaskID:           run.TaskID,
		WorkspaceID:      run.WorkspaceID,
		Kind:             taskpkg.TerminalRunCommandCompleted,
		Phase:            taskpkg.TerminalRunCommandPhaseAdmitted,
		SourceStatus:     run.Status,
		SourceSession:    run.SessionID,
		SourceClaimHash:  run.ClaimTokenHash,
		SourceLeaseUntil: run.LeaseUntil,
		IntentJSON: json.RawMessage(
			`{"version":2,"event_ids":["evt-terminal-command-recovery"],` +
				`"result":{"ok":true},"stop_required":true,"stop_reason":"completed"}`,
		),
		ActorJSON:  actorJSON,
		CommandAt:  commandAt,
		AdmittedAt: commandAt,
		UpdatedAt:  commandAt,
	})
	if err != nil {
		t.Fatalf("RestoreTerminalRunCommand() error = %v", err)
	}
	return command
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
		commands := workspacepkg.NewCoordinationService(globalDB, nil)
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
		commands := workspacepkg.NewCoordinationService(globalDB, nil)
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
		commands := workspacepkg.NewCoordinationService(globalDB, nil)
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

func installNetworkWakeEventInsertFailureTriggerForType(
	t *testing.T,
	globalDB *GlobalDB,
	eventType string,
) {
	t.Helper()

	_, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`CREATE TRIGGER fail_network_wake_event_insert
		 BEFORE INSERT ON network_wake_events
		 WHEN NEW.event_type = '`+strings.ReplaceAll(eventType, "'", "''")+`'
		 BEGIN
		   SELECT RAISE(ABORT, 'forced network wake event insert failure');
		 END;`,
	)
	if err != nil {
		t.Fatalf("install network_wake_event insert failure trigger error = %v", err)
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
