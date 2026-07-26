package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestTaskStatusProjectionObserver(t *testing.T) {
	t.Parallel()

	t.Run("Should project a real terminal transition without prompting or writing conversation", func(t *testing.T) {
		t.Parallel()

		fixture := newTaskStatusProjectionFixture(t, true)
		before := fixture.messages(t)
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
		if projection.NetworkParticipation.Mode != participation.ModeLive ||
			projection.NetworkParticipation.ChannelID != "builders" {
			t.Fatalf("projection participation = %#v, want live builders snapshot", projection.NetworkParticipation)
		}
		if !projection.ConversationAvailable || projection.ConversationAvailability != taskConversationAvailable {
			t.Fatalf(
				"projection conversation = %v/%q, want available",
				projection.ConversationAvailable,
				projection.ConversationAvailability,
			)
		}
		if projection.ThreadOrigin == nil || projection.ThreadOrigin.OriginMessageID != "msg-origin" {
			t.Fatalf("projection origin = %#v, want persisted thread origin", projection.ThreadOrigin)
		}
		if completed.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("completed run status = %s, want completed", completed.Status)
		}
		if got := fixture.networkCallCount(); got != 0 {
			t.Fatalf("PromptNetwork/runtime sends = %d, want zero", got)
		}
		after := fixture.messages(t)
		if len(after) != len(before) {
			t.Fatalf("conversation messages = %d after transition, want unchanged %d", len(after), len(before))
		}

		fixture.writePostTerminalMessage(t)
		history := fixture.messages(t)
		if len(history) != len(before)+1 || !containsConversationMessage(history, "msg-after-terminal") {
			t.Fatalf("conversation messages = %#v, want post-terminal message preserved as history", history)
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

	t.Run("Should complete orchestration while marking a disabled conversation unavailable", func(t *testing.T) {
		t.Parallel()

		fixture := newTaskStatusProjectionFixture(t, false)
		completed := fixture.completeRun(t)
		projection := fixture.nextProjection(t)

		if completed.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("completed run status = %s, want completed", completed.Status)
		}
		if projection.ConversationAvailable ||
			projection.ConversationAvailability != taskConversationNetworkDisabled {
			t.Fatalf(
				"projection conversation = %v/%q, want network_disabled",
				projection.ConversationAvailable,
				projection.ConversationAvailability,
			)
		}
		if got := fixture.networkCallCount(); got != 0 {
			t.Fatalf("PromptNetwork/runtime sends = %d, want zero", got)
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
		first := taskStatusProjectionRun("run-rollup-a", taskRecord.ID, taskpkg.TaskRunStatusCompleted, now, origin)
		second := taskStatusProjectionRun("run-rollup-b", taskRecord.ID, taskpkg.TaskRunStatusFailed, now, origin)
		first.DesignationGroupID = "tdg-status"
		second.DesignationGroupID = "tdg-status"
		for _, run := range []taskpkg.Run{first, second} {
			if err := db.CreateTaskRun(ctx, run); err != nil {
				t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
			}
		}
		publisher := &recordingTaskStatusProjectionPublisher{}
		observer := &taskStatusProjectionObserver{
			tasks: db, prefs: db, availability: db, publisher: publisher,
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

	t.Run("Should construct independently of a network runtime", func(t *testing.T) {
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
			t.Fatal("newTaskStatusProjectionObserver() = nil, want observer without network runtime")
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
			first := taskStatusProjectionRun("run-replay-a", taskRecord.ID, taskpkg.TaskRunStatusCompleted, now, origin)
			second := taskStatusProjectionRun("run-replay-b", taskRecord.ID, taskpkg.TaskRunStatusFailed, now, origin)
			for index, run := range []*taskpkg.Run{&first, &second} {
				run.WorkspaceID = "wks-replay"
				run.DesignationGroupID = "tdg-replay"
				if err := db.CreateTaskRun(ctx, *run); err != nil {
					t.Fatalf("CreateTaskRun(%d) error = %v", index, err)
				}
			}
			events := []taskpkg.Event{
				{
					ID: "evt-replay-completed", TaskID: taskRecord.ID, RunID: first.ID,
					EventType: eventspkg.TaskRunCompleted, Actor: actor, Origin: origin, Timestamp: now,
				},
				{
					ID: "evt-replay-forced-fail", TaskID: taskRecord.ID, RunID: second.ID,
					EventType: eventspkg.TaskRunOperatorForcedFail, Actor: actor, Origin: origin,
					Timestamp: now.Add(time.Second),
				},
			}
			records := make([]taskpkg.EventRecord, 0, len(events))
			for _, event := range events {
				if err := db.CreateTaskEvent(ctx, event); err != nil {
					t.Fatalf("CreateTaskEvent(%q) error = %v", event.ID, err)
				}
				record, err := db.GetTaskEventRecord(ctx, event.ID)
				if err != nil {
					t.Fatalf("GetTaskEventRecord(%q) error = %v", event.ID, err)
				}
				records = append(records, record)
			}

			flakyStore := &flakyTaskStatusProjectionStore{
				GlobalDB: db,
				failed:   make(chan struct{}),
			}
			flakyStore.remainingFailures.Store(1)
			publisher := &recordingTaskStatusProjectionPublisher{
				ch: make(chan taskStatusProjection, len(records)),
			}
			observerCtx, cancel := context.WithCancel(context.Background())
			observer := &taskStatusProjectionObserver{
				tasks: flakyStore, prefs: flakyStore, availability: flakyStore,
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

			for range records {
				select {
				case <-publisher.ch:
				case <-time.After(5 * time.Second):
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
	db        taskStatusProjectionStore
	manager   *taskpkg.Service
	publisher *recordingTaskStatusProjectionPublisher
	network   *fakeNetworkRuntime
	task      taskpkg.Task
	run       taskpkg.Run
	actor     taskpkg.ActorContext
	claim     *taskpkg.ClaimResult
}

type taskStatusProjectionStore interface {
	taskStore
	store.NetworkAvailabilityStore
	store.NetworkPreferenceStore
	ListConversationMessages(
		context.Context,
		store.NetworkConversationRef,
		store.NetworkConversationMessageQuery,
	) ([]store.NetworkConversationMessage, error)
	WriteConversationMessage(
		context.Context,
		store.NetworkConversationMessage,
	) (store.NetworkConversationWriteResult, error)
}

func newTaskStatusProjectionFixture(t *testing.T, networkAvailableAtCompletion bool) *taskStatusProjectionFixture {
	t.Helper()
	ctx := testutil.Context(t)
	db := openDaemonTestGlobalDB(t)
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seedTaskStatusProjectionThread(ctx, t, db, now)
	if _, err := db.SetNetworkAvailability(ctx, true, "test:projection-start"); err != nil {
		t.Fatalf("SetNetworkAvailability() error = %v", err)
	}
	actorIdentity := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "daemon.test"}
	origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "status-projection-test"}
	taskRecord := taskStatusProjectionTask("task-status", now, actorIdentity, origin)
	if err := db.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run := taskStatusProjectionRun("run-status", taskRecord.ID, taskpkg.TaskRunStatusQueued, now, origin)
	run.SetNetworkState(daemonTestLiveParticipation("wks_status", "builders"), "", "", "")
	if err := db.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	if err := db.PutNetworkTaskThreadOrigin(ctx, store.NetworkTaskThreadOrigin{
		TaskID: taskRecord.ID, WorkspaceID: "wks_status", Channel: "builders",
		ThreadID: "thread_status", OriginMessageID: "msg-origin", Digest: "Investigate latency",
		SourceMessageIDs: []string{"msg-origin"}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("PutNetworkTaskThreadOrigin() error = %v", err)
	}
	publisher := &recordingTaskStatusProjectionPublisher{ch: make(chan taskStatusProjection, 4)}
	notifier := newHooksNotifier(discardLogger(), func() time.Time { return now })
	notifier.AddTaskStatusProjectionObserver(publisher)
	networkRuntime := &fakeNetworkRuntime{}
	daemon := &Daemon{now: func() time.Time { return now.Add(time.Minute) }}
	eventObserver, _, observer := daemon.composeTaskEventObserver(&bootState{
		logger:   discardLogger(),
		notifier: notifier,
		network:  networkRuntime,
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
	if !networkAvailableAtCompletion {
		if _, err := db.SetNetworkAvailability(ctx, false, "test:projection-mid-run"); err != nil {
			t.Fatalf("SetNetworkAvailability(mid-run) error = %v", err)
		}
	}
	return &taskStatusProjectionFixture{
		ctx: ctx, db: db, manager: manager, publisher: publisher, network: networkRuntime,
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

func (f *taskStatusProjectionFixture) messages(t *testing.T) []store.NetworkConversationMessage {
	t.Helper()
	messages, err := f.db.ListConversationMessages(f.ctx, store.NetworkConversationRef{
		WorkspaceID: "wks_status", Channel: "builders", Surface: store.NetworkSurfaceThread,
		ThreadID: "thread_status",
	}, store.NetworkConversationMessageQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	return messages
}

func (f *taskStatusProjectionFixture) writePostTerminalMessage(t *testing.T) {
	t.Helper()
	_, err := f.db.WriteConversationMessage(f.ctx, store.NetworkConversationMessage{
		MessageID: "msg-after-terminal", SessionID: "sess-origin", WorkspaceID: "wks_status",
		Channel: "builders", Surface: store.NetworkSurfaceThread, ThreadID: "thread_status",
		Direction: "received", PeerFrom: "reviewer.sess-origin", Kind: store.NetworkKindSay,
		Body: json.RawMessage(`{"text":"post-terminal note"}`), Text: "post-terminal note",
		SizeBytes: 29, Timestamp: time.Date(2026, 7, 1, 12, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("WriteConversationMessage(post-terminal) error = %v", err)
	}
}

func containsConversationMessage(messages []store.NetworkConversationMessage, messageID string) bool {
	for _, message := range messages {
		if message.MessageID == messageID {
			return true
		}
	}
	return false
}

func (f *taskStatusProjectionFixture) networkCallCount() int {
	f.network.mu.Lock()
	defer f.network.mu.Unlock()
	return len(f.network.runtimeSendCalls)
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
		ID: id, Scope: taskpkg.ScopeWorkspace, WorkspaceID: "wks_status",
		Title: "Investigate latency", MaxAttempts: 3, Status: taskpkg.TaskStatusReady,
		CreatedBy: actor, Origin: origin, CreatedAt: now, UpdatedAt: now,
	}
}

func taskStatusProjectionRun(
	id string,
	taskID string,
	status taskpkg.RunStatus,
	now time.Time,
	origin taskpkg.Origin,
) taskpkg.Run {
	run := taskpkg.Run{
		ID: id, TaskID: taskID, Status: status, Attempt: 1, Origin: origin,
		QueuedAt: now,
	}
	if status.Normalize() != taskpkg.TaskRunStatusQueued {
		run.StartedAt = now.Add(time.Second)
		run.EndedAt = now.Add(2 * time.Second)
	}
	return run
}

func seedTaskStatusProjectionThread(
	ctx context.Context,
	t *testing.T,
	db interface {
		InsertWorkspace(context.Context, workspacepkg.Workspace) error
		RegisterSession(context.Context, store.SessionInfo) error
		WriteNetworkChannel(context.Context, store.NetworkChannelEntry) error
		WriteConversationMessage(
			context.Context,
			store.NetworkConversationMessage,
		) (store.NetworkConversationWriteResult, error)
	},
	now time.Time,
) {
	t.Helper()
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: "wks_status", RootDir: t.TempDir(), Name: "status-projection",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	if err := db.RegisterSession(ctx, store.SessionInfo{
		ID: "sess-origin", AgentName: "reviewer", WorkspaceID: "wks_status",
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
		SessionType:         "system", State: "stopped", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	if err := db.WriteNetworkChannel(ctx, store.NetworkChannelEntry{
		WorkspaceID: "wks_status", Channel: "builders", Purpose: "Coordination",
		FanoutPolicy: store.NetworkFanoutPolicyCapabilityMatch, CreatedBy: "daemon.test",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}
	_, err := db.WriteConversationMessage(ctx, store.NetworkConversationMessage{
		MessageID: "msg-origin", SessionID: "sess-origin", WorkspaceID: "wks_status",
		Channel: "builders", Surface: store.NetworkSurfaceThread, ThreadID: "thread_status",
		Direction: "received", PeerFrom: "reviewer.sess-origin", Kind: store.NetworkKindSay,
		Body: json.RawMessage(`{"text":"please investigate latency"}`),
		Text: "please investigate latency", SizeBytes: 37, Timestamp: now,
	})
	if err != nil {
		t.Fatalf("WriteConversationMessage() error = %v", err)
	}
}
