package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestGoalSessionOutboxRelay(t *testing.T) {
	t.Parallel()

	t.Run("Should retry an identical append after an acknowledgement crash boundary", func(t *testing.T) {
		t.Parallel()

		boundSessionID := "session-bound"
		createdAt := time.Date(2026, 7, 10, 19, 30, 0, 0, time.UTC)
		store := &goalSessionOutboxStoreStub{
			event: goalpkg.SessionOutboxEvent{
				ID: 7, EventID: "goal-snapshot:session-origin:run-1:start:1",
				WorkspaceID: "workspace-1", OriginSessionID: "session-origin",
				LoopRunID: "run-1", BoundSessionID: &boundSessionID,
				Cause: goalpkg.SessionOutboxCauseStart, CreatedAt: createdAt,
			},
			ackErrs: []error{errors.New("crash before ack"), nil},
		}
		appender := &goalSessionEventAppenderStub{}
		relay := &goalSessionOutboxRelay{
			store: store, appender: appender, batchSize: 7,
			now: func() time.Time { return createdAt.Add(time.Second) },
		}

		if err := relay.DeliverPending(testutil.Context(t)); err == nil {
			t.Fatal("DeliverPending(first) error = nil")
		}
		if err := relay.DeliverPending(testutil.Context(t)); err != nil {
			t.Fatalf("DeliverPending(retry) error = %v", err)
		}
		if len(appender.events) != 2 || !reflect.DeepEqual(appender.events[0], appender.events[1]) {
			t.Fatalf("appended events = %#v, want identical retry", appender.events)
		}
		if store.ackCalls != 2 || !store.delivered {
			t.Fatalf("ack calls/delivered = %d/%t, want 2/true", store.ackCalls, store.delivered)
		}
		if store.claimLimit != 7 {
			t.Fatalf("outbox claim limit = %d, want configured 7", store.claimLimit)
		}
		var payload goalSnapshotChangedPayload
		if err := json.Unmarshal(appender.events[0].Content, &payload); err != nil {
			t.Fatalf("Unmarshal(payload) error = %v", err)
		}
		if payload.SessionID != "session-origin" || payload.RunID == nil || *payload.RunID != "run-1" ||
			payload.BoundSessionID == nil || *payload.BoundSessionID != boundSessionID ||
			payload.Revision != 7 || payload.Cause != goalpkg.SessionOutboxCauseStart {
			t.Fatalf("payload = %#v", payload)
		}
	})

	t.Run("Should emit nullable run and binding fields after clear", func(t *testing.T) {
		t.Parallel()

		event, err := goalSessionProjectionEvent(goalpkg.SessionOutboxEvent{
			ID: 8, EventID: "goal-snapshot:session-origin:run-1:clear:2",
			WorkspaceID: "workspace-1", OriginSessionID: "session-origin", LoopRunID: looppkg.RunID("run-1"),
			Cause:     goalpkg.SessionOutboxCauseClear,
			CreatedAt: time.Date(2026, 7, 10, 19, 31, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("goalSessionProjectionEvent() error = %v", err)
		}
		var payload goalSnapshotChangedPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("Unmarshal(payload) error = %v", err)
		}
		if payload.RunID != nil || payload.BoundSessionID != nil || payload.Cause != goalpkg.SessionOutboxCauseClear {
			t.Fatalf("clear payload = %#v", payload)
		}
	})

	t.Run("Should retry cleanup after Stop failure and acknowledge only success", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
		cleanupStore := &goalSessionCleanupStoreStub{obligation: goalpkg.SessionCleanupObligation{
			ID: 1, CleanupID: "goal-session-cleanup:run-1:goal-handle:2",
			WorkspaceID: "workspace-1", LoopRunID: "run-1", Handle: "goal-handle",
			BindingEpoch: 2, SessionID: "session-run-owned", Cause: goalpkg.SessionCleanupCauseReseed,
			CreatedAt: createdAt,
		}}
		stopper := &goalSessionCleanupStopperStub{errs: []error{errors.New("provider stop failed"), nil}}
		var logOutput bytes.Buffer
		relay := &goalSessionOutboxRelay{
			store: &goalSessionOutboxStoreStub{delivered: true}, appender: &goalSessionEventAppenderStub{},
			cleanupStore: cleanupStore, stopper: stopper,
			logger: slog.New(slog.NewJSONHandler(&logOutput, nil)),
			now:    func() time.Time { return createdAt.Add(time.Second) },
		}

		if err := relay.DeliverPending(testutil.Context(t)); err == nil {
			t.Fatal("DeliverPending(first cleanup) error = nil")
		}
		if cleanupStore.ackCalls != 0 || cleanupStore.completed {
			t.Fatalf(
				"cleanup ack after stop failure = %d/%t, want 0/false",
				cleanupStore.ackCalls,
				cleanupStore.completed,
			)
		}
		var failureLog map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(logOutput.Bytes()), &failureLog); err != nil {
			t.Fatalf("Unmarshal(cleanup failure log) error = %v", err)
		}
		if failureLog["lane"] != "session-cleanup" || failureLog["phase"] != "stop" ||
			failureLog["binding_handle"] != "goal-handle" || failureLog["binding_epoch"] != float64(2) ||
			failureLog["cleanup_id"] != cleanupStore.obligation.CleanupID {
			t.Fatalf("cleanup failure log = %#v", failureLog)
		}
		if err := relay.DeliverPending(testutil.Context(t)); err != nil {
			t.Fatalf("DeliverPending(cleanup retry) error = %v", err)
		}
		if stopper.calls != 2 || cleanupStore.ackCalls != 1 || !cleanupStore.completed {
			t.Fatalf(
				"cleanup retry = stops:%d acks:%d completed:%t, want 2/1/true",
				stopper.calls,
				cleanupStore.ackCalls,
				cleanupStore.completed,
			)
		}
	})

	t.Run("Should deliver cleanup even when projection delivery is poisoned", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 7, 11, 8, 10, 0, 0, time.UTC)
		cleanupStore := &goalSessionCleanupStoreStub{obligation: goalpkg.SessionCleanupObligation{
			ID: 2, CleanupID: "goal-session-cleanup:run-2:goal-handle:1",
			WorkspaceID: "workspace-1", LoopRunID: "run-2", Handle: "goal-handle",
			BindingEpoch: 1, SessionID: "session-cleanup-despite-projection",
			Cause: goalpkg.SessionCleanupCauseTerminal, CreatedAt: createdAt,
		}}
		relay := &goalSessionOutboxRelay{
			store: &goalSessionOutboxStoreStub{event: goalpkg.SessionOutboxEvent{
				ID: 9, EventID: "goal-snapshot:session-origin:run-2:start:1",
				WorkspaceID: "workspace-1", OriginSessionID: "session-origin", LoopRunID: "run-2",
				Cause: goalpkg.SessionOutboxCauseStart, CreatedAt: createdAt,
			}},
			appender:     &goalSessionEventAppenderStub{err: errors.New("permanent projection failure")},
			cleanupStore: cleanupStore,
			stopper:      &goalSessionCleanupStopperStub{},
			now:          func() time.Time { return createdAt.Add(time.Second) },
		}

		if err := relay.DeliverPending(testutil.Context(t)); err == nil {
			t.Fatal("DeliverPending(poisoned projection) error = nil")
		}
		if !cleanupStore.completed || cleanupStore.ackCalls != 1 {
			t.Fatalf(
				"cleanup after projection failure = completed:%t acks:%d",
				cleanupStore.completed,
				cleanupStore.ackCalls,
			)
		}
	})

	t.Run("Should continue cleanup batch after one permanent Stop failure", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 7, 11, 8, 20, 0, 0, time.UTC)
		cleanupStore := &goalSessionCleanupBatchStoreStub{obligations: []goalpkg.SessionCleanupObligation{
			{ID: 3, CleanupID: "cleanup-failing", WorkspaceID: "workspace-1", LoopRunID: "run-3",
				Handle: "goal-a", BindingEpoch: 1, SessionID: "session-failing",
				Cause: goalpkg.SessionCleanupCauseStop, CreatedAt: createdAt},
			{ID: 4, CleanupID: "cleanup-success", WorkspaceID: "workspace-1", LoopRunID: "run-4",
				Handle: "goal-b", BindingEpoch: 1, SessionID: "session-success",
				Cause: goalpkg.SessionCleanupCauseStop, CreatedAt: createdAt},
		}}
		stopper := &goalSessionCleanupStopperBySessionStub{
			errs: map[string]error{"session-failing": errors.New("permanent Stop failure")},
		}
		relay := &goalSessionOutboxRelay{
			store: &goalSessionOutboxStoreStub{delivered: true}, appender: &goalSessionEventAppenderStub{},
			cleanupStore: cleanupStore, stopper: stopper,
			now: func() time.Time { return createdAt.Add(time.Second) },
		}

		if err := relay.DeliverPending(testutil.Context(t)); err == nil {
			t.Fatal("DeliverPending(partial cleanup failure) error = nil")
		}
		if cleanupStore.completed["cleanup-failing"] || !cleanupStore.completed["cleanup-success"] {
			t.Fatalf("cleanup batch completion = %#v", cleanupStore.completed)
		}
	})
}

type goalSessionOutboxStoreStub struct {
	event      goalpkg.SessionOutboxEvent
	ackErrs    []error
	ackCalls   int
	delivered  bool
	claimLimit int
}

func (s *goalSessionOutboxStoreStub) ClaimGoalSessionOutbox(
	_ context.Context,
	limit int,
) ([]goalpkg.SessionOutboxEvent, error) {
	s.claimLimit = limit
	if s.delivered {
		return []goalpkg.SessionOutboxEvent{}, nil
	}
	return []goalpkg.SessionOutboxEvent{s.event}, nil
}

func (s *goalSessionOutboxStoreStub) AcknowledgeGoalSessionOutbox(
	_ context.Context,
	_ string,
	_ time.Time,
) error {
	s.ackCalls++
	if len(s.ackErrs) == 0 {
		s.delivered = true
		return nil
	}
	err := s.ackErrs[0]
	s.ackErrs = s.ackErrs[1:]
	if err == nil {
		s.delivered = true
	}
	return err
}

type goalSessionEventAppenderStub struct {
	events []session.GoalEvent
	err    error
}

type goalSessionCleanupBatchStoreStub struct {
	obligations []goalpkg.SessionCleanupObligation
	completed   map[string]bool
}

func (s *goalSessionCleanupBatchStoreStub) ClaimGoalSessionCleanup(
	context.Context,
	int,
) ([]goalpkg.SessionCleanupObligation, error) {
	pending := make([]goalpkg.SessionCleanupObligation, 0, len(s.obligations))
	for _, obligation := range s.obligations {
		if s.completed == nil || !s.completed[obligation.CleanupID] {
			pending = append(pending, obligation)
		}
	}
	return pending, nil
}

func (s *goalSessionCleanupBatchStoreStub) AcknowledgeGoalSessionCleanup(
	_ context.Context,
	cleanupID string,
	_ time.Time,
) error {
	if s.completed == nil {
		s.completed = make(map[string]bool)
	}
	s.completed[cleanupID] = true
	return nil
}

type goalSessionCleanupStoreStub struct {
	obligation goalpkg.SessionCleanupObligation
	ackCalls   int
	completed  bool
}

func (s *goalSessionCleanupStoreStub) ClaimGoalSessionCleanup(
	context.Context,
	int,
) ([]goalpkg.SessionCleanupObligation, error) {
	if s.completed {
		return []goalpkg.SessionCleanupObligation{}, nil
	}
	return []goalpkg.SessionCleanupObligation{s.obligation}, nil
}

func (s *goalSessionCleanupStoreStub) AcknowledgeGoalSessionCleanup(
	_ context.Context,
	cleanupID string,
	_ time.Time,
) error {
	s.ackCalls++
	if cleanupID != s.obligation.CleanupID {
		return errors.New("unexpected cleanup identity")
	}
	s.completed = true
	return nil
}

type goalSessionCleanupStopperStub struct {
	errs  []error
	calls int
}

type goalSessionCleanupStopperBySessionStub struct {
	errs map[string]error
}

func (s *goalSessionCleanupStopperBySessionStub) Stop(_ context.Context, sessionID string) error {
	return s.errs[sessionID]
}

func (s *goalSessionCleanupStopperStub) Stop(context.Context, string) error {
	s.calls++
	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

func (s *goalSessionEventAppenderStub) AppendSessionEventIfAbsent(
	_ context.Context,
	event session.GoalEvent,
) error {
	s.events = append(s.events, event)
	return s.err
}
