package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestBridgeTerminalTaskNotificationObserver(t *testing.T) {
	t.Run("Should deliver terminal task notification and advance cursor", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC)
		store := newDaemonBridgeNotificationStore(now)
		observer := newBridgeTerminalTaskNotificationObserver(
			store,
			store,
			store,
			store,
			store,
			nil,
			discardLogger(),
			func() time.Time { return now },
			time.Second,
		)
		if observer == nil {
			t.Fatal("newBridgeTerminalTaskNotificationObserver() = nil, want observer")
		}
		t.Cleanup(observer.shutdown)

		observer.OnTaskEvent(context.Background(), store.records[0])

		delivery := waitForBridgeDelivery(t, observer, store, 1)
		if got, want := delivery.Event.BridgeInstanceID, store.bridge.ID; got != want {
			t.Fatalf("delivery bridge id = %q, want %q", got, want)
		}
		if got, want := delivery.Event.Seq, store.records[0].Sequence; got != want {
			t.Fatalf("delivery sequence = %d, want %d", got, want)
		}
		cursor := waitForBridgeCursor(t, store, store.subscription.CursorKey())
		if got, want := cursor.LastSequence, store.records[0].Sequence; got != want {
			t.Fatalf("cursor.LastSequence = %d, want %d", got, want)
		}
		if cursor.LastDeliveryID != delivery.Event.DeliveryID {
			t.Fatalf("cursor.LastDeliveryID = %q, want %q", cursor.LastDeliveryID, delivery.Event.DeliveryID)
		}
	})

	t.Run("Should ignore non-terminal task events", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 1, 5, 0, 0, time.UTC)
		store := newDaemonBridgeNotificationStore(now)
		observer := newBridgeTerminalTaskNotificationObserver(
			store,
			store,
			store,
			store,
			store,
			nil,
			discardLogger(),
			func() time.Time { return now },
			time.Second,
		)
		if observer == nil {
			t.Fatal("newBridgeTerminalTaskNotificationObserver() = nil, want observer")
		}
		t.Cleanup(observer.shutdown)

		record := store.records[0]
		record.Event.EventType = "task.run_started"
		observer.OnTaskEvent(context.Background(), record)
		waitForCondition(t, "bridge observer wake drain", func() bool {
			observer.mu.Lock()
			defer observer.mu.Unlock()
			return len(observer.pending) == 0 && len(observer.backlog) == 0 && len(observer.queue) == 0
		})

		if got := store.deliveryCount(); got != 0 {
			t.Fatalf("len(deliveries) = %d, want 0", got)
		}
		if _, err := store.GetCursor(context.Background(), store.subscription.CursorKey()); !errors.Is(
			err,
			notifications.ErrCursorNotFound,
		) {
			t.Fatalf("GetCursor(non-terminal) error = %v, want ErrCursorNotFound", err)
		}
	})

	t.Run("Should return from task event fanout without waiting for bridge delivery", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 1, 10, 0, 0, time.UTC)
		store := newDaemonBridgeNotificationStore(now)
		started := make(chan struct{})
		release := make(chan struct{})
		store.deliverFn = func(ctx context.Context, req bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return bridgepkg.DeliveryAck{}, ctx.Err()
			}
			return bridgepkg.DeliveryAck{DeliveryID: req.Event.DeliveryID, Seq: req.Event.Seq}, nil
		}
		observer := newBridgeTerminalTaskNotificationObserver(
			store,
			store,
			store,
			store,
			store,
			nil,
			discardLogger(),
			func() time.Time { return now },
			time.Second,
		)
		if observer == nil {
			t.Fatal("newBridgeTerminalTaskNotificationObserver() = nil, want observer")
		}
		t.Cleanup(func() {
			close(release)
			observer.shutdown()
		})

		done := make(chan struct{})
		go func() {
			observer.OnTaskEvent(context.Background(), store.records[0])
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("OnTaskEvent() blocked on bridge delivery, want immediate return")
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("bridge delivery worker did not start")
		}
	})

	t.Run("Should dispatch the task's explicit scope to notification presets", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 1, 15, 0, 0, time.UTC)
		store := newDaemonBridgeNotificationStore(now)
		store.task.Scope = taskpkg.ScopeWorkspace
		store.task.WorkspaceID = " global "
		dispatcher := &recordingNotificationPresetDispatcher{events: make(chan presetspkg.Event, 1)}
		observer := newBridgeTerminalTaskNotificationObserver(
			nil,
			store,
			nil,
			store,
			store,
			dispatcher,
			discardLogger(),
			func() time.Time { return now },
			time.Second,
		)
		if observer == nil {
			t.Fatal("newBridgeTerminalTaskNotificationObserver() = nil, want preset observer")
		}
		t.Cleanup(observer.shutdown)

		record := store.records[0]
		record.Event.ID = " evt:scope "
		observer.OnTaskEvent(context.Background(), record)

		select {
		case event := <-dispatcher.events:
			wantScope := notifications.ScopeRef{
				Kind:        notifications.ScopeKindWorkspace,
				WorkspaceID: " global ",
			}
			if event.Scope != wantScope {
				t.Fatalf("preset event scope = %#v, want %#v", event.Scope, wantScope)
			}
			if event.ID != " evt:scope " {
				t.Fatalf("preset event ID = %q, want opaque event ID", event.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("notification preset dispatcher did not receive task event")
		}
	})

	t.Run("Should keep whitespace-distinct preset wake identities independent", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 6, 1, 15, 0, 0, time.UTC)
		store := newDaemonBridgeNotificationStore(now)
		release := make(chan struct{})
		releaseDispatch := sync.OnceFunc(func() { close(release) })
		dispatcher := &recordingNotificationPresetDispatcher{
			events:  make(chan presetspkg.Event, 2),
			release: release,
		}
		observer := newBridgeTerminalTaskNotificationObserver(
			nil,
			store,
			nil,
			store,
			store,
			dispatcher,
			discardLogger(),
			func() time.Time { return now },
			time.Second,
		)
		if observer == nil {
			t.Fatal("newBridgeTerminalTaskNotificationObserver() = nil, want preset observer")
		}
		t.Cleanup(observer.shutdown)
		t.Cleanup(releaseDispatch)

		first := store.records[0]
		first.Event.ID = "evt-opaque"
		second := first
		second.Event.ID = " evt-opaque"
		observer.OnTaskEvent(context.Background(), first)

		select {
		case event := <-dispatcher.events:
			if got, want := event.ID, first.Event.ID; got != want {
				t.Fatalf("first preset event ID = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("first preset dispatch did not start")
		}

		observer.OnTaskEvent(context.Background(), second)
		observer.mu.Lock()
		pending := len(observer.pending)
		observer.mu.Unlock()
		if got, want := pending, 2; got != want {
			t.Fatalf("pending preset wakes = %d, want %d for opaque-distinct event IDs", got, want)
		}

		releaseDispatch()
		select {
		case event := <-dispatcher.events:
			if got, want := event.ID, second.Event.ID; got != want {
				t.Fatalf("second preset event ID = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("second opaque-distinct preset dispatch did not start")
		}
	})
}

func TestTaskEventObserverFanout(t *testing.T) {
	t.Run("Should notify every task event observer", func(t *testing.T) {
		t.Parallel()

		first := &recordingTaskEventObserver{}
		second := &recordingTaskEventObserver{}
		fanout := newTaskEventObserverFanout(discardLogger(), first, nil, second)
		if fanout == nil {
			t.Fatal("newTaskEventObserverFanout() = nil, want fanout")
		}

		record := taskpkg.EventRecord{Event: taskpkg.Event{ID: "evt-1", TaskID: "task-1"}}
		fanout.OnTaskEvent(context.Background(), record)

		if got, want := first.count, 1; got != want {
			t.Fatalf("first observer count = %d, want %d", got, want)
		}
		if got, want := second.count, 1; got != want {
			t.Fatalf("second observer count = %d, want %d", got, want)
		}
	})
}

type daemonBridgeNotificationStore struct {
	mu            sync.RWMutex
	subscription  bridgepkg.BridgeTaskSubscription
	task          taskpkg.Task
	run           taskpkg.Run
	bridge        bridgepkg.BridgeInstance
	records       []taskpkg.EventRecord
	cursors       map[notifications.CursorKey]notifications.Cursor
	deliveries    []bridgepkg.DeliveryRequest
	deliverFn     func(context.Context, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)
	deliveryReady chan struct{}
	cursorReady   chan struct{}
	now           time.Time
}

var _ bridgepkg.BridgeTaskSubscriptionStore = (*daemonBridgeNotificationStore)(nil)
var _ bridgepkg.TerminalTaskEventReader = (*daemonBridgeNotificationStore)(nil)
var _ bridgepkg.BridgeInstanceReader = (*daemonBridgeNotificationStore)(nil)
var _ notifications.CursorStore = (*daemonBridgeNotificationStore)(nil)
var _ bridgepkg.DeliveryTransport = (*daemonBridgeNotificationStore)(nil)

func newDaemonBridgeNotificationStore(now time.Time) *daemonBridgeNotificationStore {
	subscription := bridgepkg.BridgeTaskSubscription{
		SubscriptionID:   "sub-1",
		TaskID:           "task-1",
		BridgeInstanceID: "brg-1",
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		PeerID:           "12345",
		ThreadID:         "1",
		DeliveryMode:     bridgepkg.DeliveryModeReply,
		CreatedBy:        taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return &daemonBridgeNotificationStore{
		subscription: subscription,
		task: taskpkg.Task{
			ID:             "task-1",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    "ws-1",
			Status:         taskpkg.TaskStatusCompleted,
			LatestEventSeq: 7,
		},
		run: taskpkg.Run{
			ID:     "run-1",
			TaskID: "task-1",
			Status: taskpkg.TaskRunStatusCompleted,
		},
		bridge: bridgepkg.BridgeInstance{
			ID:            "brg-1",
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-1",
			Platform:      "telegram",
			ExtensionName: "telegram",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		records: []taskpkg.EventRecord{{
			Sequence: 7,
			Event: taskpkg.Event{
				ID:        "evt-7",
				TaskID:    "task-1",
				RunID:     "run-1",
				EventType: "task.run_completed",
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
				Timestamp: now,
			},
		}},
		cursors:       make(map[notifications.CursorKey]notifications.Cursor),
		deliveryReady: make(chan struct{}, 1),
		cursorReady:   make(chan struct{}, 1),
		now:           now,
	}
}

func (s *daemonBridgeNotificationStore) PutBridgeTaskSubscription(
	context.Context,
	bridgepkg.BridgeTaskSubscription,
) error {
	return nil
}

func (s *daemonBridgeNotificationStore) GetBridgeTaskSubscription(
	context.Context,
	string,
) (bridgepkg.BridgeTaskSubscription, error) {
	return s.subscription, nil
}

func (s *daemonBridgeNotificationStore) ListBridgeTaskSubscriptions(
	_ context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if query.TaskID != "" && query.TaskID != s.subscription.TaskID {
		return nil, nil
	}
	return []bridgepkg.BridgeTaskSubscription{s.subscription}, nil
}

func (s *daemonBridgeNotificationStore) DeleteBridgeTaskSubscription(context.Context, string) error {
	return nil
}

func (s *daemonBridgeNotificationStore) GetTask(context.Context, string) (taskpkg.Task, error) {
	return s.task, nil
}

func (s *daemonBridgeNotificationStore) GetTaskRun(context.Context, string) (taskpkg.Run, error) {
	return s.run, nil
}

func (s *daemonBridgeNotificationStore) ListTaskEventRecords(
	_ context.Context,
	query taskpkg.EventRecordQuery,
) ([]taskpkg.EventRecord, error) {
	records := make([]taskpkg.EventRecord, 0, len(s.records))
	for _, record := range s.records {
		if query.TaskID != "" && record.Event.TaskID != query.TaskID {
			continue
		}
		if record.Sequence <= query.AfterSequence {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *daemonBridgeNotificationStore) GetBridgeInstance(
	context.Context,
	string,
) (bridgepkg.BridgeInstance, error) {
	return s.bridge, nil
}

func (s *daemonBridgeNotificationStore) GetCursor(
	_ context.Context,
	key notifications.CursorKey,
) (notifications.Cursor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cursor, ok := s.cursors[key]
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursor, nil
}

func (s *daemonBridgeNotificationStore) ListCursors(
	context.Context,
	notifications.CursorQuery,
) ([]notifications.Cursor, error) {
	return nil, nil
}

func (s *daemonBridgeNotificationStore) AdvanceCursor(
	_ context.Context,
	update notifications.AdvanceCursor,
) (notifications.Cursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cursor := notifications.Cursor{
		Key:             update.Key,
		LastSequence:    update.LastSequence,
		LastDeliveryID:  update.DeliveryID,
		LastDeliveredAt: update.LastDeliveredAt,
		UpdatedAt:       update.Now,
	}
	s.cursors[update.Key] = cursor
	select {
	case s.cursorReady <- struct{}{}:
	default:
	}
	return cursor, nil
}

func (s *daemonBridgeNotificationStore) ResetCursor(
	context.Context,
	notifications.ResetCursor,
) (notifications.Cursor, error) {
	return notifications.Cursor{}, nil
}

func (s *daemonBridgeNotificationStore) RecordCursorError(
	_ context.Context,
	report notifications.CursorError,
) (notifications.Cursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cursor := notifications.Cursor{
		Key:       report.Key,
		LastError: report.LastError,
		UpdatedAt: report.Now,
	}
	s.cursors[report.Key] = cursor
	return cursor, nil
}

func (s *daemonBridgeNotificationStore) DeliverBridge(
	ctx context.Context,
	_ string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if s.deliverFn != nil {
		return s.deliverFn(ctx, req)
	}
	s.mu.Lock()
	s.deliveries = append(s.deliveries, req)
	s.mu.Unlock()
	select {
	case s.deliveryReady <- struct{}{}:
	default:
	}
	return bridgepkg.DeliveryAck{DeliveryID: req.Event.DeliveryID, Seq: req.Event.Seq}, nil
}

func (s *daemonBridgeNotificationStore) deliveryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.deliveries)
}

func waitForBridgeDelivery(
	t *testing.T,
	observer *bridgeTerminalTaskNotificationObserver,
	store *daemonBridgeNotificationStore,
	want int,
) bridgepkg.DeliveryRequest {
	t.Helper()

	for {
		store.mu.RLock()
		if len(store.deliveries) >= want {
			delivery := store.deliveries[want-1]
			store.mu.RUnlock()
			return delivery
		}
		store.mu.RUnlock()
		select {
		case <-store.deliveryReady:
		case <-time.After(time.Second):
			observer.shutdown()
			t.Fatalf("timed out waiting for bridge delivery %d", want)
		}
	}
}

func waitForBridgeCursor(
	t *testing.T,
	store *daemonBridgeNotificationStore,
	key notifications.CursorKey,
) notifications.Cursor {
	t.Helper()

	for {
		cursor, err := store.GetCursor(context.Background(), key)
		if err == nil {
			return cursor
		}
		if !errors.Is(err, notifications.ErrCursorNotFound) {
			t.Fatalf("GetCursor() error = %v", err)
		}
		select {
		case <-store.cursorReady:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for bridge notification cursor")
		}
	}
}

type recordingTaskEventObserver struct {
	count int
}

var _ taskpkg.EventObserver = (*recordingTaskEventObserver)(nil)

func (o *recordingTaskEventObserver) OnTaskEvent(context.Context, taskpkg.EventRecord) {
	o.count++
}

type recordingNotificationPresetDispatcher struct {
	events  chan presetspkg.Event
	release <-chan struct{}
}

var _ notificationPresetDispatcher = (*recordingNotificationPresetDispatcher)(nil)

func (d *recordingNotificationPresetDispatcher) Dispatch(
	_ context.Context,
	event presetspkg.Event,
) (presetspkg.DispatchResult, error) {
	d.events <- event
	if d.release != nil {
		<-d.release
	}
	return presetspkg.DispatchResult{}, nil
}
