package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
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

		waitForBridgeObserverDrain(t, observer)
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
	mu           sync.RWMutex
	subscription bridgepkg.BridgeTaskSubscription
	task         taskpkg.Task
	run          taskpkg.Run
	bridge       bridgepkg.BridgeInstance
	records      []taskpkg.EventRecord
	cursors      map[string]notifications.Cursor
	deliveries   []bridgepkg.DeliveryRequest
	deliverFn    func(context.Context, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)
	now          time.Time
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
		cursors: make(map[string]notifications.Cursor),
		now:     now,
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

	cursor, ok := s.cursors[daemonCursorStoreKey(key)]
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
	s.cursors[daemonCursorStoreKey(update.Key)] = cursor
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
	s.cursors[daemonCursorStoreKey(report.Key)] = cursor
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

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		store.mu.RLock()
		if len(store.deliveries) >= want {
			delivery := store.deliveries[want-1]
			store.mu.RUnlock()
			return delivery
		}
		store.mu.RUnlock()
		time.Sleep(5 * time.Millisecond)
	}
	observer.shutdown()
	store.mu.RLock()
	got := len(store.deliveries)
	store.mu.RUnlock()
	t.Fatalf("len(deliveries) = %d, want >= %d", got, want)
	return bridgepkg.DeliveryRequest{}
}

func waitForBridgeCursor(
	t *testing.T,
	store *daemonBridgeNotificationStore,
	key notifications.CursorKey,
) notifications.Cursor {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		cursor, err := store.GetCursor(context.Background(), key)
		if err == nil {
			return cursor
		}
		if !errors.Is(err, notifications.ErrCursorNotFound) {
			t.Fatalf("GetCursor() error = %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for bridge notification cursor")
	return notifications.Cursor{}
}

func waitForBridgeObserverDrain(t *testing.T, observer *bridgeTerminalTaskNotificationObserver) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		observer.mu.Lock()
		pending := len(observer.pending)
		backlog := len(observer.backlog)
		queued := len(observer.queue)
		observer.mu.Unlock()
		if pending == 0 && backlog == 0 && queued == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	observer.mu.Lock()
	pending := len(observer.pending)
	backlog := len(observer.backlog)
	queued := len(observer.queue)
	observer.mu.Unlock()
	t.Fatalf(
		"bridge observer did not drain: pending=%d backlog=%d queued=%d",
		pending,
		backlog,
		queued,
	)
}

type recordingTaskEventObserver struct {
	count int
}

var _ taskpkg.EventObserver = (*recordingTaskEventObserver)(nil)

func (o *recordingTaskEventObserver) OnTaskEvent(context.Context, taskpkg.EventRecord) {
	o.count++
}

func daemonCursorStoreKey(key notifications.CursorKey) string {
	return key.ConsumerID + "\x00" + key.StreamName + "\x00" + key.SubjectID
}
