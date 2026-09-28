package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestTaskStatusProjectionObserver(t *testing.T) {
	t.Parallel()

	t.Run("Should project and persist a real terminal task transition", func(t *testing.T) {
		t.Parallel()

		fixture := newTaskStatusProjectionFixture(t)
		completed := fixture.completeRun(t)
		projection := fixture.nextProjection(t)

		if projection.EventType != taskEventRunCompleted || projection.TaskID != fixture.task.ID ||
			projection.RunID != fixture.run.ID {
			t.Fatalf("projection identity = %#v, want completed task/run", projection)
		}
		if projection.TaskStatus.Normalize() != taskpkg.TaskStatusCompleted ||
			projection.RunStatus.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf(
				"projection statuses = %s/%s, want completed/completed",
				projection.TaskStatus,
				projection.RunStatus,
			)
		}
		if completed.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("completed run status = %s, want completed", completed.Status)
		}
		storedTask, err := fixture.db.GetTask(fixture.ctx, fixture.task.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		storedRun, err := fixture.db.GetTaskRun(fixture.ctx, fixture.run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if storedTask.Status.Normalize() != taskpkg.TaskStatusCompleted ||
			storedRun.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf(
				"post-terminal task/run statuses = %s/%s, want completed/completed",
				storedTask.Status,
				storedRun.Status,
			)
		}
	})

	t.Run("Should persist a typed completed designation rollup", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID: "wks_status", RootDir: t.TempDir(), Name: "status-rollup",
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		actor := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "daemon.test"}
		origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "status-projection-test"}
		taskRecord := taskStatusProjectionTask("task-rollup", now, actor, origin)
		if err := db.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		seedTaskStatusProjectionTerminalRun(
			ctx,
			t,
			db,
			taskRecord,
			"run-rollup-a",
			taskpkg.TaskRunStatusCompleted,
			"tdg-status",
			now,
		)
		second := seedTaskStatusProjectionTerminalRun(
			ctx,
			t,
			db,
			taskRecord,
			"run-rollup-b",
			taskpkg.TaskRunStatusFailed,
			"tdg-status",
			now.Add(time.Second),
		)
		publisher := &recordingTaskStatusProjectionPublisher{}
		observer := &taskStatusProjectionObserver{
			tasks: db, designations: db, publisher: publisher,
			now: func() time.Time { return now.Add(time.Minute) },
		}
		err := observer.processWithContext(ctx, taskpkg.EventRecord{Event: taskpkg.Event{
			ID: "evt-rollup", TaskID: taskRecord.ID, RunID: second.ID,
			EventType: taskEventRunFailed, Actor: actor, Origin: origin, Timestamp: now,
		}})
		if err != nil {
			t.Fatalf("processWithContext() error = %v", err)
		}
		projections := publisher.snapshot()
		if len(projections) != 1 || projections[0].DesignationRollup == nil {
			t.Fatalf("projections = %#v, want one typed designation rollup", projections)
		}
		rollup := projections[0].DesignationRollup
		if !rollup.Complete || rollup.Total != 2 || rollup.Completed != 1 || rollup.Failed != 1 {
			t.Fatalf("rollup = %#v, want complete 1/1 split", rollup)
		}
		stored, err := db.ListTaskDesignationRollups(
			ctx,
			store.TaskDesignationRollupQuery{DesignationGroupID: "tdg-status", Limit: 1},
		)
		if err != nil {
			t.Fatalf("ListTaskDesignationRollups() error = %v", err)
		}
		if len(stored) != 1 || !json.Valid(stored[0].SummaryJSON) {
			t.Fatalf("stored rollups = %#v, want one valid typed JSON projection", stored)
		}
	})

	t.Run("Should apply the configured projection queue and timeout", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		publisher := &recordingTaskStatusProjectionPublisher{}
		observer := newTaskStatusProjectionObserver(
			db,
			publisher,
			withTaskStatusProjectionObserverLogger(discardLogger()),
			withTaskStatusProjectionObserverQueueSize(7),
			withTaskStatusProjectionObserverTimeout(2*time.Second),
		)
		if observer == nil {
			t.Fatal("newTaskStatusProjectionObserver() = nil, want configured observer")
		}
		t.Cleanup(observer.shutdown)
		if cap(observer.queue) != 7 || observer.timeout != 2*time.Second {
			t.Fatalf("observer queue/timeout = %d/%s, want 7/2s", cap(observer.queue), observer.timeout)
		}
	})

	t.Run(
		"Should replay every durable rollup transition after a coalesced wake and retry before advancing",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openDaemonTestGlobalDB(t)
			now := time.Date(2026, 7, 1, 13, 0, 0, 0, time.UTC)
			if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
				ID: "wks-replay", RootDir: t.TempDir(), Name: "status-replay",
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatalf("InsertWorkspace() error = %v", err)
			}
			actor := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "daemon.projection.replay"}
			origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "status-projection-replay"}
			taskRecord := taskStatusProjectionTask("task-replay", now, actor, origin)
			taskRecord.WorkspaceID = "wks-replay"
			if err := db.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			first := seedTaskStatusProjectionTerminalRun(
				ctx,
				t,
				db,
				taskRecord,
				"run-replay-a",
				taskpkg.TaskRunStatusCompleted,
				"tdg-replay",
				now,
			)
			second := seedTaskStatusProjectionTerminalRun(
				ctx,
				t,
				db,
				taskRecord,
				"run-replay-b",
				taskpkg.TaskRunStatusFailed,
				"tdg-replay",
				now.Add(time.Second),
			)
			allRecords, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
				TaskID: taskRecord.ID,
				Limit:  100,
			})
			if err != nil {
				t.Fatalf("ListTaskEventRecords() error = %v", err)
			}
			records := taskStatusProjectionTerminalRecords(allRecords, first.ID, second.ID)
			if len(records) != 2 {
				t.Fatalf("terminal records = %#v, want completed then forced-fail", records)
			}

			flakyStore := &flakyTaskStatusProjectionStore{
				GlobalDB: db,
				failed:   make(chan struct{}),
			}
			flakyStore.remainingFailures.Store(1)
			publisher := &recordingTaskStatusProjectionPublisher{
				ch: make(chan taskStatusProjection, len(allRecords)),
			}
			observerCtx, cancel := context.WithCancel(context.Background())
			observer := &taskStatusProjectionObserver{
				tasks: flakyStore, designations: flakyStore,
				events: flakyStore, cursors: flakyStore, publisher: publisher,
				logger: discardLogger(), now: func() time.Time { return now.Add(time.Minute) },
				ctx: observerCtx, cancel: cancel, queue: make(chan struct{}, 1), batchSize: 1,
				timeout: time.Second, retryDelay: time.Millisecond,
			}
			firstReplayErr := observer.catchUpTaskStatusProjections(ctx)
			if firstReplayErr == nil ||
				!strings.Contains(firstReplayErr.Error(), "injected designation rollup write failure") {
				t.Fatalf("catchUpTaskStatusProjections(first) error = %v, want injected write failure", firstReplayErr)
			}
			cursor, err := db.GetCursor(ctx, taskStatusProjectionCursorKey)
			if err != nil && !errors.Is(err, notifications.ErrCursorNotFound) {
				t.Fatalf("GetCursor(after failed projection) error = %v", err)
			}
			if err == nil && cursor.LastSequence != 0 {
				t.Fatalf("cursor after failed projection = %d, want 0", cursor.LastSequence)
			}
			observer.OnTaskEvent(ctx, records[0])
			observer.OnTaskEvent(ctx, records[1])
			if got, want := len(observer.queue), 1; got != want {
				t.Fatalf("coalesced projection wakeups = %d, want %d", got, want)
			}
			observer.start()
			t.Cleanup(observer.shutdown)

			pendingTerminalEvents := make(map[string]struct{}, len(records))
			for _, record := range records {
				pendingTerminalEvents[record.Event.ID] = struct{}{}
			}
			projectionDeadline := time.After(5 * time.Second)
			for len(pendingTerminalEvents) > 0 {
				select {
				case projection := <-publisher.ch:
					delete(pendingTerminalEvents, projection.EventID)
				case <-projectionDeadline:
					t.Fatal("timed out waiting for replayed projection")
				}
			}
			deadline := time.After(5 * time.Second)
			for {
				cursor, err = db.GetCursor(ctx, taskStatusProjectionCursorKey)
				if err == nil && cursor.LastSequence == records[len(records)-1].Sequence {
					break
				}
				select {
				case <-deadline:
					t.Fatalf("task status projection cursor = %#v, error = %v", cursor, err)
				case <-time.After(time.Millisecond):
				}
			}
			rollups, err := db.ListTaskDesignationRollups(
				ctx,
				store.TaskDesignationRollupQuery{DesignationGroupID: "tdg-replay", Limit: 1},
			)
			if err != nil {
				t.Fatalf("ListTaskDesignationRollups() error = %v", err)
			}
			if len(rollups) != 1 {
				t.Fatalf("designation rollups = %#v, want one replayed rollup", rollups)
			}
			var rollup taskDesignationRollupStatus
			if err := json.Unmarshal(rollups[0].SummaryJSON, &rollup); err != nil {
				t.Fatalf("json.Unmarshal(replayed rollup) error = %v", err)
			}
			if !rollup.Complete || rollup.Completed != 1 || rollup.Failed != 1 ||
				rollup.LatestEvent != eventspkg.TaskRunOperatorForcedFail {
				t.Fatalf("replayed rollup = %#v, want complete operator-forced terminal truth", rollup)
			}
		},
	)
}

type flakyTaskStatusProjectionStore struct {
	*globaldb.GlobalDB
	remainingFailures atomic.Int32
	failed            chan struct{}
	failureOnce       sync.Once
}

func (s *flakyTaskStatusProjectionStore) PutTaskDesignationRollup(
	ctx context.Context,
	rollup store.TaskDesignationRollup,
) error {
	if s.remainingFailures.Add(-1) >= 0 {
		s.failureOnce.Do(func() { close(s.failed) })
		return errors.New("injected designation rollup write failure")
	}
	return s.GlobalDB.PutTaskDesignationRollup(ctx, rollup)
}

type taskStatusProjectionFixture struct {
	ctx       context.Context
	db        taskStore
	manager   *taskpkg.Service
	publisher *recordingTaskStatusProjectionPublisher
	task      taskpkg.Task
	run       taskpkg.Run
	actor     taskpkg.ActorContext
	claim     *taskpkg.ClaimResult
}

func newTaskStatusProjectionFixture(t *testing.T) *taskStatusProjectionFixture {
	t.Helper()
	ctx := testutil.Context(t)
	db := openDaemonTestGlobalDB(t)
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: "wks_status", RootDir: t.TempDir(), Name: "status-projection",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	actorIdentity := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "daemon.test"}
	origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "status-projection-test"}
	taskRecord := taskStatusProjectionTask("task-status", now, actorIdentity, origin)
	if err := db.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	seedActor, err := taskpkg.DeriveDaemonActorContext("status-projection", "daemon.test")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(seed run) error = %v", err)
	}
	seedManager := newTaskStatusProjectionManagerForRunID(
		t,
		db,
		now,
		"run-status",
	)
	queued, err := seedManager.EnqueueRun(ctx, taskpkg.EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: "projection-run-status",
	}, seedActor)
	if err != nil {
		t.Fatalf("EnqueueRun(run-status) error = %v", err)
	}
	run := *queued
	publisher := &recordingTaskStatusProjectionPublisher{ch: make(chan taskStatusProjection, 4)}
	notifier := newHooksNotifier(discardLogger(), func() time.Time { return now })
	notifier.AddTaskStatusProjectionObserver(publisher)
	daemon := &Daemon{now: func() time.Time { return now.Add(time.Minute) }}
	eventObserver, observer := daemon.composeTaskEventObserver(&bootState{
		logger:   discardLogger(),
		notifier: notifier,
	}, db, nil)
	if observer == nil {
		t.Fatal("composeTaskEventObserver() status projection = nil")
	}
	t.Cleanup(observer.shutdown)
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db), taskpkg.WithEventObserver(eventObserver),
		taskpkg.WithManagerNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveAgentSessionActorContext("sess-worker", "wks_status")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID: run.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: "wks_status",
		ClaimerSessionID: "sess-worker", ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindAgentSession, Ref: "sess-worker",
		},
	}, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	return &taskStatusProjectionFixture{
		ctx: ctx, db: db, manager: manager, publisher: publisher,
		task: taskRecord, run: run, actor: actor, claim: claim,
	}
}

func (f *taskStatusProjectionFixture) completeRun(t *testing.T) *taskpkg.Run {
	t.Helper()
	run, err := f.manager.CompleteRunLease(f.ctx, taskpkg.LeaseCompletion{
		RunID: f.run.ID, ClaimToken: f.claim.ClaimToken,
		Result: taskpkg.RunResult{Value: json.RawMessage(`{"status":"done"}`)},
	}, f.actor)
	if err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}
	return run
}

func (f *taskStatusProjectionFixture) nextProjection(t *testing.T) taskStatusProjection {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case projection := <-f.publisher.ch:
			if projection.EventType == taskEventRunCompleted {
				return projection
			}
		case <-deadline:
			cursorStore, ok := f.db.(notifications.CursorStore)
			if !ok {
				t.Fatal("timed out waiting for completed task status projection; cursor store unavailable")
			}
			cursor, err := cursorStore.GetCursor(f.ctx, taskStatusProjectionCursorKey)
			t.Fatalf("timed out waiting for completed task status projection; cursor = %#v, error = %v", cursor, err)
			return taskStatusProjection{}
		}
	}
}

type recordingTaskStatusProjectionPublisher struct {
	mu          sync.Mutex
	projections []taskStatusProjection
	ch          chan taskStatusProjection
}

func (p *recordingTaskStatusProjectionPublisher) PublishTaskStatusProjection(
	_ context.Context,
	projection taskStatusProjection,
) {
	p.mu.Lock()
	p.projections = append(p.projections, projection)
	p.mu.Unlock()
	if p.ch != nil {
		p.ch <- projection
	}
}

func (p *recordingTaskStatusProjectionPublisher) OnTaskStatusProjection(
	ctx context.Context,
	projection taskStatusProjection,
) error {
	p.PublishTaskStatusProjection(ctx, projection)
	return nil
}

func (p *recordingTaskStatusProjectionPublisher) snapshot() []taskStatusProjection {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]taskStatusProjection(nil), p.projections...)
}

func taskStatusProjectionTask(
	id string,
	now time.Time,
	actor taskpkg.ActorIdentity,
	origin taskpkg.Origin,
) taskpkg.Task {
	return taskpkg.Task{
		ID: id, ProfileID: store.DefaultProfileID,
		Scope: taskpkg.ScopeWorkspace, WorkspaceID: "wks_status",
		Title: "Investigate latency", MaxAttempts: 3, Status: taskpkg.TaskStatusReady,
		CreatedBy: actor, Origin: origin, CreatedAt: now, UpdatedAt: now,
	}
}

func newTaskStatusProjectionManagerForRunID(
	t *testing.T,
	db *globaldb.GlobalDB,
	now time.Time,
	runID string,
	options ...taskpkg.Option,
) *taskpkg.Service {
	t.Helper()

	sequence := 0
	baseOptions := []taskpkg.Option{
		taskpkg.WithStore(db),
		taskpkg.WithManagerNow(func() time.Time { return now }),
		taskpkg.WithIDGenerator(func(prefix string) (string, error) {
			if prefix == "run" {
				return runID, nil
			}
			sequence++
			return prefix + "-" + runID + "-" + strconv.Itoa(sequence), nil
		}),
	}
	manager, err := taskpkg.NewManager(append(baseOptions, options...)...)
	if err != nil {
		t.Fatalf("task.NewManager(%q) error = %v", runID, err)
	}
	return manager
}

func seedTaskStatusProjectionTerminalRun(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	taskRecord taskpkg.Task,
	runID string,
	status taskpkg.RunStatus,
	designationGroupID string,
	now time.Time,
) taskpkg.Run {
	t.Helper()

	manager := newTaskStatusProjectionManagerForRunID(t, db, now, runID)
	actor, err := taskpkg.DeriveDaemonActorContext("status-projection", "daemon.test")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(%q) error = %v", runID, err)
	}
	queued, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{
		TaskID:             taskRecord.ID,
		IdempotencyKey:     "projection-" + runID,
		DesignationGroupID: designationGroupID,
	}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(%q) error = %v", runID, err)
	}

	var terminal *taskpkg.Run
	switch status.Normalize() {
	case taskpkg.TaskRunStatusCompleted:
		claim, claimErr := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            runID,
			Scope:            taskRecord.Scope,
			WorkspaceID:      taskRecord.WorkspaceID,
			RunKind:          queued.RunKind,
			ClaimerSessionID: "projection-" + runID,
		}, actor)
		if claimErr != nil {
			t.Fatalf("ClaimNextRun(%q) error = %v", runID, claimErr)
		}
		terminal, err = manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:      runID,
			ClaimToken: claim.ClaimToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			Now:        now,
		}, actor)
	case taskpkg.TaskRunStatusFailed:
		terminal, err = manager.ForceFailRun(ctx, runID, taskpkg.ForceFailRun{
			Reason: "projection terminal fixture",
		}, actor)
	default:
		t.Fatalf("unsupported projection terminal status %q", status)
	}
	if err != nil {
		t.Fatalf("settle projection run %q as %q error = %v", runID, status, err)
	}
	if terminal.Status.Normalize() != status.Normalize() {
		t.Fatalf("projection run %q status = %q, want %q", runID, terminal.Status, status)
	}
	return *terminal
}

func taskStatusProjectionTerminalRecords(
	records []taskpkg.EventRecord,
	completedRunID string,
	failedRunID string,
) []taskpkg.EventRecord {
	terminal := make([]taskpkg.EventRecord, 0, 2)
	for _, record := range records {
		event := record.Event
		eventType := taskpkg.StatusProjectionEventType(event.EventType)
		if event.RunID == completedRunID && eventType == eventspkg.TaskRunCompleted {
			terminal = append(terminal, record)
		}
		if event.RunID == failedRunID && eventType == eventspkg.TaskRunOperatorForcedFail {
			terminal = append(terminal, record)
		}
	}
	return terminal
}
