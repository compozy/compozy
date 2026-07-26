package deadentity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestServicePermanentFailureRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should mark on the fifth permanent failure and admit one due recovery", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		clock := newDeadEntityTestClock()
		deadStore := newRecordingDeadEntityStore()
		eventStore := &recordingDeadEntityEventStore{}
		service := New(
			deadStore,
			WithClock(clock.Now),
			WithEventStore(eventStore),
		)
		key := deadEntityTestKey("ws-recovery")

		for failure := 1; failure <= DefaultPermanentFailureThreshold-1; failure++ {
			if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid api_key=super-secret"); err != nil {
				t.Fatalf("RecordFailure(%d) error = %v", failure, err)
			}
			status, err := service.Status(ctx, key)
			if err != nil {
				t.Fatalf("Status(%d) error = %v", failure, err)
			}
			if status.Dead {
				t.Fatalf("Status(%d).Dead = true, want false before threshold", failure)
			}
		}
		if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid api_key=super-secret"); err != nil {
			t.Fatalf("RecordFailure(threshold) error = %v", err)
		}

		marked := deadStore.Marked()
		if len(marked) != 1 {
			t.Fatalf("marked entities = %#v, want one threshold transition", marked)
		}
		if strings.Contains(marked[0].Reason, "super-secret") || !strings.Contains(marked[0].Reason, "[REDACTED]") {
			t.Fatalf("marked reason = %q, want redacted diagnostic", marked[0].Reason)
		}
		if len(eventStore.Summaries()) != 1 || eventStore.Summaries()[0].Type != events.DeadEntityMarked {
			t.Fatalf("transition summaries = %#v, want one dead mark", eventStore.Summaries())
		}
		assertDeadEntityTransitionSummary(t, eventStore.Summaries()[0], key, events.DeadEntityMarked)

		decision, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(suppressed) error = %v", err)
		}
		if decision.Allowed || !decision.Dead || decision.Recovery {
			t.Fatalf("BeforeProbe(suppressed) = %#v, want dead and disallowed", decision)
		}
		clock.Advance(DefaultRecoveryInterval)
		decision, err = service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(recovery) error = %v", err)
		}
		if !decision.Allowed || !decision.Dead || !decision.Recovery {
			t.Fatalf("BeforeProbe(recovery) = %#v, want one allowed recovery", decision)
		}
		second, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(second recovery) error = %v", err)
		}
		if second.Allowed {
			t.Fatalf("BeforeProbe(second recovery) = %#v, want suppressed", second)
		}

		if err := service.RecordFailure(ctx, key, FailurePermanent, "still invalid"); err != nil {
			t.Fatalf("RecordFailure(recovery) error = %v", err)
		}
		if got := len(deadStore.Marked()); got != 2 {
			t.Fatalf("marked entities after failed recovery = %d, want refreshed upsert", got)
		}
		if got := len(eventStore.Summaries()); got != 1 {
			t.Fatalf("transition summaries after refresh = %d, want no duplicate transition", got)
		}
	})

	t.Run("Should clear a durable restart mark once after recovery succeeds", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		clock := newDeadEntityTestClock()
		deadStore := newRecordingDeadEntityStore()
		key := deadEntityTestKey("ws-restart")
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      clock.Now().Add(-time.Hour),
		}
		eventStore := &recordingDeadEntityEventStore{}
		service := New(deadStore, WithClock(clock.Now), WithEventStore(eventStore))

		decision, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(restart) error = %v", err)
		}
		if !decision.Allowed || !decision.Recovery || !decision.Dead {
			t.Fatalf("BeforeProbe(restart) = %#v, want immediate recovery", decision)
		}
		if err := service.RecordSuccess(ctx, key); err != nil {
			t.Fatalf("RecordSuccess() error = %v", err)
		}
		if err := service.RecordSuccess(ctx, key); err != nil {
			t.Fatalf("RecordSuccess(idempotent) error = %v", err)
		}
		status, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(after recovery) error = %v", err)
		}
		if status.Dead {
			t.Fatalf("Status(after recovery) = %#v, want live", status)
		}
		if got := deadStore.Clears(); got != 1 {
			t.Fatalf("clear calls = %d, want one", got)
		}
		summaries := eventStore.Summaries()
		if len(summaries) != 1 || summaries[0].Type != events.DeadEntityCleared {
			t.Fatalf("transition summaries = %#v, want one recovery", summaries)
		}
		assertDeadEntityTransitionSummary(t, summaries[0], key, events.DeadEntityCleared)
	})

	t.Run("Should preserve the remaining recovery interval after restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		clock := newDeadEntityTestClock()
		deadStore := newRecordingDeadEntityStore()
		key := deadEntityTestKey("ws-restart-wait")
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      clock.Now().Add(-DefaultRecoveryInterval / 2),
		}
		service := New(deadStore, WithClock(clock.Now))

		decision, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(before persisted interval) error = %v", err)
		}
		if decision.Allowed || decision.Recovery || !decision.Dead {
			t.Fatalf("BeforeProbe(before persisted interval) = %#v, want suppressed dead entity", decision)
		}

		clock.Advance(DefaultRecoveryInterval / 2)
		decision, err = service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(at persisted interval) error = %v", err)
		}
		if !decision.Allowed || !decision.Recovery || !decision.Dead {
			t.Fatalf("BeforeProbe(at persisted interval) = %#v, want due recovery", decision)
		}
	})
}

func assertDeadEntityTransitionSummary(
	t *testing.T,
	summary store.EventSummary,
	key store.DeadEntityKey,
	wantType string,
) {
	t.Helper()
	if summary.WorkspaceID != key.WorkspaceID || summary.Type != wantType {
		t.Fatalf("transition summary = %#v, want workspace %q and type %q", summary, key.WorkspaceID, wantType)
	}
	var content transitionContent
	if err := json.Unmarshal(summary.Content, &content); err != nil {
		t.Fatalf("json.Unmarshal(transition content) error = %v", err)
	}
	if content.Kind != key.Kind || content.EntityID != key.EntityID {
		t.Fatalf("transition content = %#v, want kind %q and entity_id %q", content, key.Kind, key.EntityID)
	}
}

func TestServiceFailureClassificationAndIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an invalid failure class without changing state", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore)
		key := deadEntityTestKey("ws-invalid-class")

		err := service.RecordFailure(ctx, key, FailureClass(2), "invalid class")
		if !errors.Is(err, ErrInvalidFailureClass) {
			t.Fatalf("RecordFailure(invalid class) error = %v, want ErrInvalidFailureClass", err)
		}
		if got := len(deadStore.Marked()); got != 0 {
			t.Fatalf("marked entities = %d, want no transition", got)
		}
		status, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(after invalid class) error = %v", err)
		}
		if status.Dead {
			t.Fatalf("Status(after invalid class) = %#v, want live", status)
		}
	})

	t.Run("Should reset a live permanent streak on a transient failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore)
		key := deadEntityTestKey("ws-transient")
		for range DefaultPermanentFailureThreshold - 1 {
			if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration"); err != nil {
				t.Fatalf("RecordFailure(before transient) error = %v", err)
			}
		}
		if err := service.RecordFailure(ctx, key, FailureTransient, "deadline exceeded"); err != nil {
			t.Fatalf("RecordFailure(transient) error = %v", err)
		}
		for range DefaultPermanentFailureThreshold - 1 {
			if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration"); err != nil {
				t.Fatalf("RecordFailure(after transient) error = %v", err)
			}
		}
		if got := len(deadStore.Marked()); got != 0 {
			t.Fatalf("marked entities = %d, want reset streak", got)
		}
		if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration"); err != nil {
			t.Fatalf("RecordFailure(new threshold) error = %v", err)
		}
		if got := len(deadStore.Marked()); got != 1 {
			t.Fatalf("marked entities = %d, want one after new streak", got)
		}
	})

	t.Run("Should isolate identical entity identities by workspace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore)
		workspaceA := deadEntityTestKey("ws-a")
		workspaceB := deadEntityTestKey("ws-b")
		for range DefaultPermanentFailureThreshold {
			if err := service.RecordFailure(ctx, workspaceA, FailurePermanent, "invalid configuration"); err != nil {
				t.Fatalf("RecordFailure(workspace A) error = %v", err)
			}
		}
		decisionA, err := service.BeforeProbe(ctx, workspaceA)
		if err != nil {
			t.Fatalf("BeforeProbe(workspace A) error = %v", err)
		}
		decisionB, err := service.BeforeProbe(ctx, workspaceB)
		if err != nil {
			t.Fatalf("BeforeProbe(workspace B) error = %v", err)
		}
		if decisionA.Allowed || !decisionA.Dead {
			t.Fatalf("BeforeProbe(workspace A) = %#v, want suppressed dead entity", decisionA)
		}
		if !decisionB.Allowed || decisionB.Dead {
			t.Fatalf("BeforeProbe(workspace B) = %#v, want independent live entity", decisionB)
		}
	})
}

func TestServicePersistenceFailuresFailOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should keep probes admitted when loading or marking fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		deadStore.findErr = errors.New("database unavailable")
		service := New(deadStore)
		key := deadEntityTestKey("ws-store-failure")
		decision, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(load failure) error = %v", err)
		}
		if !decision.Allowed {
			t.Fatalf("BeforeProbe(load failure) = %#v, want fail-open admission", decision)
		}

		deadStore.findErr = nil
		deadStore.markErr = errors.New("database read-only")
		for failure := range DefaultPermanentFailureThreshold {
			if err := service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration"); err != nil {
				t.Fatalf("RecordFailure(%d) error = %v", failure, err)
			}
		}
		decision, err = service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(mark failure) error = %v", err)
		}
		if !decision.Allowed || decision.Dead {
			t.Fatalf("BeforeProbe(mark failure) = %#v, want fail-open live state", decision)
		}
	})

	t.Run("Should restore live admission while retrying a failed durable clear", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		key := deadEntityTestKey("ws-clear-failure")
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      time.Now().UTC(),
		}
		deadStore.clearErr = errors.New("database busy")
		service := New(deadStore)
		if err := service.RecordSuccess(ctx, key); err != nil {
			t.Fatalf("RecordSuccess(clear failure) error = %v", err)
		}
		decision, err := service.BeforeProbe(ctx, key)
		if err != nil {
			t.Fatalf("BeforeProbe(after clear failure) error = %v", err)
		}
		if !decision.Allowed || decision.Dead {
			t.Fatalf("BeforeProbe(after clear failure) = %#v, want live fail-open state", decision)
		}
		deadStore.clearErr = nil
		if err := service.RecordSuccess(ctx, key); err != nil {
			t.Fatalf("RecordSuccess(clear retry) error = %v", err)
		}
		if got := deadStore.Clears(); got != 2 {
			t.Fatalf("clear calls = %d, want failed attempt plus retry", got)
		}
	})
}

type recordingDeadEntityStore struct {
	mu sync.Mutex

	entities map[store.DeadEntityKey]store.DeadEntity
	marked   []store.DeadEntity
	clears   int
	findErr  error
	markErr  error
	clearErr error
}

func newRecordingDeadEntityStore() *recordingDeadEntityStore {
	return &recordingDeadEntityStore{entities: make(map[store.DeadEntityKey]store.DeadEntity)}
}

func (s *recordingDeadEntityStore) MarkDeadEntity(_ context.Context, entity store.DeadEntity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markErr != nil {
		return s.markErr
	}
	s.entities[entity.DeadEntityKey] = entity
	s.marked = append(s.marked, entity)
	return nil
}

func (s *recordingDeadEntityStore) ClearDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clears++
	if s.clearErr != nil {
		return s.clearErr
	}
	delete(s.entities, store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID})
	return nil
}

func (s *recordingDeadEntityStore) FindDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findErr != nil {
		return store.DeadEntity{}, false, s.findErr
	}
	entity, ok := s.entities[store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID}]
	return entity, ok, nil
}

func (s *recordingDeadEntityStore) ListDeadEntities(
	_ context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entities := make([]store.DeadEntity, 0)
	for key, entity := range s.entities {
		if key.WorkspaceID == workspaceID {
			entities = append(entities, entity)
		}
	}
	return entities, nil
}

func (s *recordingDeadEntityStore) Marked() []store.DeadEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.DeadEntity(nil), s.marked...)
}

func (s *recordingDeadEntityStore) Clears() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clears
}

type recordingDeadEntityEventStore struct {
	mu        sync.Mutex
	summaries []store.EventSummary
	writeErr  error
}

func (s *recordingDeadEntityEventStore) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeErr != nil {
		return s.writeErr
	}
	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *recordingDeadEntityEventStore) ListEventSummaries(
	_ context.Context,
	_ store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return nil, nil
}

func (s *recordingDeadEntityEventStore) Summaries() []store.EventSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.EventSummary(nil), s.summaries...)
}

type deadEntityTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newDeadEntityTestClock() *deadEntityTestClock {
	return &deadEntityTestClock{now: time.Date(2026, 7, 15, 15, 0, 0, 0, time.UTC)}
}

func (c *deadEntityTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *deadEntityTestClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta)
}

func deadEntityTestKey(workspaceID string) store.DeadEntityKey {
	return store.DeadEntityKey{
		WorkspaceID: workspaceID,
		Kind:        store.DeadEntityKindMCPSidecar,
		EntityID:    "github",
	}
}
