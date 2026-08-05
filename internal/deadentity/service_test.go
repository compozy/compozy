package deadentity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should retain an open Loop target but reset its pre-threshold streak after restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		clock := newDeadEntityTestClock()
		deadStore := newRecordingDeadEntityStore()
		key := loopTargetTestKey("ws-loop-restart", "toolcall:compozy__search")
		first := New(
			deadStore,
			WithClock(clock.Now),
			WithPermanentFailureThreshold(2),
			WithRecoveryInterval(time.Minute),
		)
		if err := first.RecordFailure(ctx, key, FailurePermanent, "transport failure"); err != nil {
			t.Fatalf("RecordFailure(first process) error = %v", err)
		}

		restarted := New(
			deadStore,
			WithClock(clock.Now),
			WithPermanentFailureThreshold(2),
			WithRecoveryInterval(time.Minute),
		)
		if err := restarted.RecordFailure(ctx, key, FailurePermanent, "transport failure"); err != nil {
			t.Fatalf("RecordFailure(after restart) error = %v", err)
		}
		if status, err := restarted.Status(ctx, key); err != nil || status.Dead {
			t.Fatalf("Status(after one restarted failure) = %#v, %v; want live", status, err)
		}
		if err := restarted.RecordFailure(ctx, key, FailurePermanent, "transport failure"); err != nil {
			t.Fatalf("RecordFailure(restarted threshold) error = %v", err)
		}

		reopened := New(
			deadStore,
			WithClock(clock.Now),
			WithPermanentFailureThreshold(2),
			WithRecoveryInterval(time.Minute),
		)
		if status, err := reopened.Status(ctx, key); err != nil || !status.Dead {
			t.Fatalf("Status(durable open after restart) = %#v, %v; want dead", status, err)
		}
		if decision, err := reopened.BeforeProbe(ctx, key); err != nil || decision.Allowed {
			t.Fatalf("BeforeProbe(before interval) = %#v, %v; want suppressed", decision, err)
		}
		clock.Advance(time.Minute)
		if decision, err := reopened.BeforeProbe(ctx, key); err != nil ||
			!decision.Allowed || !decision.Recovery || !decision.Dead {
			t.Fatalf("BeforeProbe(after interval) = %#v, %v; want half-open recovery", decision, err)
		}
	})
}

func TestServiceReasonNormalization(t *testing.T) {
	t.Parallel()

	t.Run("Should persist a useful default for an empty reason", func(t *testing.T) {
		t.Parallel()

		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore, WithPermanentFailureThreshold(1))
		if err := service.RecordFailure(
			testutil.Context(t),
			deadEntityTestKey("ws-empty-reason"),
			FailurePermanent,
			"   ",
		); err != nil {
			t.Fatalf("RecordFailure(empty reason) error = %v", err)
		}
		marked := deadStore.Marked()
		if len(marked) != 1 || marked[0].Reason != "permanent runtime failure" {
			t.Fatalf("marked entities = %#v, want default permanent failure reason", marked)
		}
	})

	t.Run("Should truncate a long reason only at a valid UTF-8 boundary", func(t *testing.T) {
		t.Parallel()

		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore, WithPermanentFailureThreshold(1))
		reason := strings.Repeat("界", maxPersistedReasonBytes)
		if err := service.RecordFailure(
			testutil.Context(t),
			deadEntityTestKey("ws-long-reason"),
			FailurePermanent,
			reason,
		); err != nil {
			t.Fatalf("RecordFailure(long reason) error = %v", err)
		}
		marked := deadStore.Marked()
		if len(marked) != 1 {
			t.Fatalf("marked entities = %#v, want one durable mark", marked)
		}
		if !utf8.ValidString(marked[0].Reason) || len(marked[0].Reason) > maxPersistedReasonBytes {
			t.Fatalf(
				"marked reason bytes = %d and valid UTF-8 = %t, want at most %d valid bytes",
				len(marked[0].Reason),
				utf8.ValidString(marked[0].Reason),
				maxPersistedReasonBytes,
			)
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

	t.Run("Should isolate every workspace kind and entity tuple", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		service := New(deadStore)
		failed := deadEntityTestKey("ws-a")
		differentWorkspace := deadEntityTestKey("ws-b")
		differentKind := failed
		differentKind.Kind = store.DeadEntityKindBridge
		differentEntity := failed
		differentEntity.EntityID = "gitlab"
		for range DefaultPermanentFailureThreshold {
			if err := service.RecordFailure(ctx, failed, FailurePermanent, "invalid configuration"); err != nil {
				t.Fatalf("RecordFailure(failed tuple) error = %v", err)
			}
		}
		failedDecision, err := service.BeforeProbe(ctx, failed)
		if err != nil {
			t.Fatalf("BeforeProbe(failed tuple) error = %v", err)
		}
		if failedDecision.Allowed || !failedDecision.Dead {
			t.Fatalf("BeforeProbe(failed tuple) = %#v, want suppressed dead entity", failedDecision)
		}
		for _, independent := range []store.DeadEntityKey{
			differentWorkspace,
			differentKind,
			differentEntity,
		} {
			decision, err := service.BeforeProbe(ctx, independent)
			if err != nil {
				t.Fatalf("BeforeProbe(independent tuple %#v) error = %v", independent, err)
			}
			if !decision.Allowed || decision.Dead {
				t.Fatalf("BeforeProbe(independent tuple %#v) = %#v, want live admission", independent, decision)
			}
		}
	})

	t.Run("Should isolate the Loop target policy from the shared runtime policy", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		shared := New(deadStore)
		loopTargets := New(deadStore, WithPermanentFailureThreshold(2))
		sharedKey := deadEntityTestKey("ws-policy-isolation")
		loopKey := loopTargetTestKey("ws-policy-isolation", "run-agent:reviewer")
		for range 2 {
			if err := shared.RecordFailure(ctx, sharedKey, FailurePermanent, "runtime failure"); err != nil {
				t.Fatalf("shared RecordFailure() error = %v", err)
			}
			if err := loopTargets.RecordFailure(ctx, loopKey, FailurePermanent, "transport failure"); err != nil {
				t.Fatalf("loop RecordFailure() error = %v", err)
			}
		}
		sharedDecision, err := shared.BeforeProbe(ctx, sharedKey)
		if err != nil {
			t.Fatalf("shared BeforeProbe() error = %v", err)
		}
		loopDecision, err := loopTargets.BeforeProbe(ctx, loopKey)
		if err != nil {
			t.Fatalf("loop BeforeProbe() error = %v", err)
		}
		if !sharedDecision.Allowed || sharedDecision.Dead {
			t.Fatalf("shared decision = %#v, want default policy still live", sharedDecision)
		}
		if loopDecision.Allowed || !loopDecision.Dead {
			t.Fatalf("loop decision = %#v, want separately configured target open", loopDecision)
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

func TestServiceList(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize the workspace and return only its durable marks", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		deadStore := newRecordingDeadEntityStore()
		key := deadEntityTestKey("ws-list")
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		other := deadEntityTestKey("ws-list-other")
		deadStore.entities[other] = store.DeadEntity{
			DeadEntityKey: other,
			Reason:        "other failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		service := New(deadStore)

		entities, err := service.List(ctx, "  ws-list  ")
		if err != nil {
			t.Fatalf("List(normalized workspace) error = %v", err)
		}
		if len(entities) != 1 || entities[0].DeadEntityKey != key {
			t.Fatalf("List(normalized workspace) = %#v, want only %#v", entities, key)
		}
		if calls := deadStore.ListCalls(); len(calls) != 1 || calls[0] != key.WorkspaceID {
			t.Fatalf("ListDeadEntities calls = %#v, want normalized workspace", calls)
		}
	})

	t.Run("Should reject invalid requests before reading durable state", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			ctx         context.Context
			workspaceID string
			want        error
		}{
			{name: "Should reject nil context", workspaceID: "ws-list", want: errors.New("context is required")},
			{
				name:        "Should reject blank workspace",
				ctx:         testutil.Context(t),
				workspaceID: "   ",
				want:        store.ErrInvalidDeadEntity,
			},
		}
		canceledCtx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		tests = append(tests, struct {
			name        string
			ctx         context.Context
			workspaceID string
			want        error
		}{name: "Should reject canceled context", ctx: canceledCtx, workspaceID: "ws-list", want: context.Canceled})

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				deadStore := newRecordingDeadEntityStore()
				_, err := New(deadStore).List(test.ctx, test.workspaceID)
				if test.ctx == nil {
					if err == nil || !strings.Contains(err.Error(), test.want.Error()) {
						t.Fatalf("List(invalid request) error = %v, want %q", err, test.want)
					}
				} else if !errors.Is(err, test.want) {
					t.Fatalf("List(invalid request) error = %v, want %v", err, test.want)
				}
				if calls := deadStore.ListCalls(); len(calls) != 0 {
					t.Fatalf("ListDeadEntities calls = %#v, want none for invalid request", calls)
				}
			})
		}
	})

	t.Run("Should return an initialized empty result without a durable store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		services := []*Service{nil, New(nil)}
		for _, service := range services {
			entities, err := service.List(ctx, "ws-list-empty")
			if err != nil {
				t.Fatalf("List(without store) error = %v", err)
			}
			if entities == nil || len(entities) != 0 {
				t.Fatalf("List(without store) = %#v, want initialized empty result", entities)
			}
		}
	})

	t.Run("Should propagate genuine store errors", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("database unavailable")
		deadStore := newRecordingDeadEntityStore()
		deadStore.listErr = wantErr
		_, err := New(deadStore).List(testutil.Context(t), "ws-list-error")
		if !errors.Is(err, wantErr) {
			t.Fatalf("List(store failure) error = %v, want %v", err, wantErr)
		}
	})

	t.Run("Should propagate cancellation while listing durable state", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			listGate:                 gate,
		}
		service := New(deadStore)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			_, err := service.List(ctx, "ws-list-canceled")
			result <- err
		}()

		<-gate.started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("List(canceled store read) error = %v, want context.Canceled", err)
		}
	})
}

func TestServiceContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a pre-canceled context before reading durable state", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		cancel()
		service := New(newRecordingDeadEntityStore())

		decision, err := service.BeforeProbe(ctx, deadEntityTestKey("ws-pre-canceled"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("BeforeProbe(pre-canceled) error = %v, want context.Canceled", err)
		}
		if decision != (ProbeDecision{}) {
			t.Fatalf("BeforeProbe(pre-canceled) = %#v, want zero decision", decision)
		}
	})

	t.Run("Should propagate cancellation while loading durable state", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			findGate:                 gate,
		}
		service := New(deadStore)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			_, err := service.BeforeProbe(ctx, deadEntityTestKey("ws-canceled-load"))
			result <- err
		}()

		<-gate.started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("BeforeProbe(canceled load) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should propagate cancellation while marking and retain live state", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			markGate:                 gate,
		}
		service := New(deadStore, WithPermanentFailureThreshold(1))
		key := deadEntityTestKey("ws-canceled-mark")
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			result <- service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration")
		}()

		<-gate.started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("RecordFailure(canceled mark) error = %v, want context.Canceled", err)
		}
		status, err := service.Status(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("Status(after canceled mark) error = %v", err)
		}
		if status.Dead {
			t.Fatalf("Status(after canceled mark) = %#v, want live", status)
		}
	})

	t.Run("Should cancel while waiting for an in-flight per-key operation", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			markGate:                 gate,
		}
		service := New(deadStore, WithPermanentFailureThreshold(1))
		key := deadEntityTestKey("ws-canceled-wait")
		ownerCtx := testutil.Context(t)
		owner := make(chan error, 1)
		go func() {
			owner <- service.RecordFailure(
				ownerCtx,
				key,
				FailurePermanent,
				"invalid configuration",
			)
		}()

		<-gate.started
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		waiter := make(chan error, 1)
		go func() {
			_, err := service.Status(ctx, key)
			waiter <- err
		}()
		cancel()
		gate.Release()
		if err := <-waiter; !errors.Is(err, context.Canceled) {
			t.Fatalf("Status(waiting owner) error = %v, want context.Canceled", err)
		}
		if err := <-owner; err != nil {
			t.Fatalf("RecordFailure(owner) error = %v", err)
		}
	})

	t.Run("Should propagate cancellation while clearing and retain the durable mark", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		key := deadEntityTestKey("ws-canceled-clear")
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			clearGate:                gate,
		}
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		service := New(deadStore)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			result <- service.RecordSuccess(ctx, key)
		}()

		<-gate.started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("RecordSuccess(canceled clear) error = %v, want context.Canceled", err)
		}
		status, err := service.Status(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("Status(after canceled clear) error = %v", err)
		}
		if !status.Dead {
			t.Fatalf("Status(after canceled clear) = %#v, want durable dead state", status)
		}
	})

	t.Run("Should publish the marked transition after the caller cancels post-commit", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		eventStore := &blockingDeadEntityEventStore{
			recordingDeadEntityEventStore: &recordingDeadEntityEventStore{},
			gate:                          gate,
		}
		service := New(
			newRecordingDeadEntityStore(),
			WithPermanentFailureThreshold(1),
			WithEventStore(eventStore),
			WithTransitionEventTimeout(time.Minute),
		)
		key := deadEntityTestKey("ws-canceled-event")
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			result <- service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration")
		}()

		<-gate.started
		cancel()
		gate.Release()
		if err := <-result; err != nil {
			t.Fatalf("RecordFailure(canceled event) error = %v, want nil after a committed durable mark", err)
		}
		status, err := service.Status(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("Status(after canceled event) error = %v", err)
		}
		if !status.Dead {
			t.Fatalf("Status(after canceled event) = %#v, want committed dead state", status)
		}
		summaries := eventStore.Summaries()
		if len(summaries) != 1 || summaries[0].Type != events.DeadEntityMarked {
			t.Fatalf("transition summaries = %#v, want one marked publication", summaries)
		}
	})

	t.Run("Should publish the cleared transition after the caller cancels post-commit", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		key := deadEntityTestKey("ws-canceled-clear-event")
		deadStore := newRecordingDeadEntityStore()
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		eventStore := &blockingDeadEntityEventStore{
			recordingDeadEntityEventStore: &recordingDeadEntityEventStore{},
			gate:                          gate,
		}
		service := New(deadStore, WithEventStore(eventStore), WithTransitionEventTimeout(time.Minute))
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		result := make(chan error, 1)
		go func() {
			result <- service.RecordSuccess(ctx, key)
		}()

		<-gate.started
		cancel()
		gate.Release()
		if err := <-result; err != nil {
			t.Fatalf("RecordSuccess(canceled event) error = %v, want nil after a committed durable clear", err)
		}
		status, err := service.Status(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("Status(after canceled clear event) error = %v", err)
		}
		if status.Dead {
			t.Fatalf("Status(after canceled clear event) = %#v, want live state", status)
		}
		summaries := eventStore.Summaries()
		if len(summaries) != 1 || summaries[0].Type != events.DeadEntityCleared {
			t.Fatalf("transition summaries = %#v, want one cleared publication", summaries)
		}
	})

	t.Run("Should release a canceled publisher without letting its successor overtake", func(t *testing.T) {
		t.Parallel()

		markedGate := newDeadEntityTestGate()
		clearGate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			clearGate:                clearGate,
		}
		eventStore := &firstMarkedBlockingDeadEntityEventStore{
			recordingDeadEntityEventStore: &recordingDeadEntityEventStore{},
			markedGate:                    markedGate,
		}
		service := New(
			deadStore,
			WithPermanentFailureThreshold(1),
			WithEventStore(eventStore),
			WithTransitionEventTimeout(time.Minute),
		)
		key := deadEntityTestKey("ws-canceled-publication")
		ctx := testutil.Context(t)

		firstMarked := make(chan error, 1)
		go func() {
			firstMarked <- service.RecordFailure(ctx, key, FailurePermanent, "first permanent failure")
		}()
		<-markedGate.started

		clearCtx, cancelClear := context.WithCancel(ctx)
		defer cancelClear()
		cleared := make(chan error, 1)
		go func() {
			cleared <- service.RecordSuccess(clearCtx, key)
		}()
		<-clearGate.started

		statusCtx := newDoneObservedContext(ctx)
		statusResult := make(chan struct {
			status Status
			err    error
		}, 1)
		go func() {
			status, err := service.Status(statusCtx, key)
			statusResult <- struct {
				status Status
				err    error
			}{status: status, err: err}
		}()
		<-statusCtx.doneObserved
		clearGate.Release()
		observed := <-statusResult
		if observed.err != nil {
			t.Fatalf("Status(after clear commit) error = %v", observed.err)
		}
		if observed.status.Dead {
			t.Fatalf("Status(after clear commit) = %#v, want live", observed.status)
		}

		cancelClear()
		if err := <-cleared; !errors.Is(err, context.Canceled) {
			t.Fatalf("RecordSuccess(canceled publisher) error = %v, want context.Canceled", err)
		}

		secondMarkGate := newDeadEntityTestGate()
		deadStore.markGate = secondMarkGate
		secondMarked := make(chan error, 1)
		go func() {
			secondMarked <- service.RecordFailure(ctx, key, FailurePermanent, "second permanent failure")
		}()
		<-secondMarkGate.started

		successorStatusCtx := newDoneObservedContext(ctx)
		successorStatusResult := make(chan struct {
			status Status
			err    error
		}, 1)
		go func() {
			status, err := service.Status(successorStatusCtx, key)
			successorStatusResult <- struct {
				status Status
				err    error
			}{status: status, err: err}
		}()
		<-successorStatusCtx.doneObserved
		secondMarkGate.Release()
		successorObserved := <-successorStatusResult
		if successorObserved.err != nil {
			t.Fatalf("Status(after successor commit) error = %v", successorObserved.err)
		}
		if !successorObserved.status.Dead || successorObserved.status.Reason != "second permanent failure" {
			t.Fatalf(
				"Status(after successor commit) = %#v, want successor dead state",
				successorObserved.status,
			)
		}

		markedGate.Release()
		if err := <-firstMarked; err != nil {
			t.Fatalf("RecordFailure(first publisher) error = %v", err)
		}
		if err := <-secondMarked; err != nil {
			t.Fatalf("RecordFailure(successor publisher) error = %v", err)
		}
		summaries := eventStore.Summaries()
		if len(summaries) != 2 {
			t.Fatalf("transition summaries = %#v, want two marked publications", summaries)
		}
		var first transitionContent
		if err := json.Unmarshal(summaries[0].Content, &first); err != nil {
			t.Fatalf("json.Unmarshal(first transition) error = %v", err)
		}
		var second transitionContent
		if err := json.Unmarshal(summaries[1].Content, &second); err != nil {
			t.Fatalf("json.Unmarshal(second transition) error = %v", err)
		}
		if first.Reason != "first permanent failure" || second.Reason != "second permanent failure" {
			t.Fatalf("transition reasons = %q then %q, want commit order", first.Reason, second.Reason)
		}
	})
}

func TestServiceForgetWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should forget cache only and lazily rehydrate an unchanged durable mark", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		key := deadEntityTestKey("ws-forget-rehydrate")
		deadStore := newRecordingDeadEntityStore()
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		eventStore := &recordingDeadEntityEventStore{}
		service := New(deadStore, WithEventStore(eventStore))

		initial, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(initial) error = %v", err)
		}
		if !initial.Dead {
			t.Fatalf("Status(initial) = %#v, want dead durable mark", initial)
		}
		service.ForgetWorkspace(key.WorkspaceID)
		service.ForgetWorkspace(key.WorkspaceID)

		rehydrated, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(rehydrated) error = %v", err)
		}
		if !rehydrated.Dead || rehydrated.Reason != "durable failure" {
			t.Fatalf("Status(rehydrated) = %#v, want rehydrated durable mark", rehydrated)
		}
		if got := deadStore.Clears(); got != 0 {
			t.Fatalf("ForgetWorkspace clear calls = %d, want zero", got)
		}
		if got := len(eventStore.Summaries()); got != 0 {
			t.Fatalf("ForgetWorkspace transition summaries = %d, want zero", got)
		}
	})

	t.Run("Should not let a retired load repopulate cache", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		key := deadEntityTestKey("ws-forget-inflight")
		gate := newDeadEntityTestGate()
		deadStore := &staleFindDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			findGate:                 gate,
		}
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "stale durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		service := New(deadStore)
		result := make(chan struct {
			status Status
			err    error
		}, 1)
		go func() {
			status, err := service.Status(ctx, key)
			result <- struct {
				status Status
				err    error
			}{status: status, err: err}
		}()

		<-gate.started
		service.ForgetWorkspace(key.WorkspaceID)
		deadStore.deleteEntity(key)
		gate.Release()
		stale := <-result
		if stale.err != nil {
			t.Fatalf("Status(retired load) error = %v", stale.err)
		}
		if stale.status.Dead {
			t.Fatalf("Status(retired load) = %#v, want retired cache ignored", stale.status)
		}
		live, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(after retired load) error = %v", err)
		}
		if live.Dead {
			t.Fatalf("Status(after retired load) = %#v, want fresh durable live state", live)
		}
	})
}

func TestServiceForgetEntity(t *testing.T) {
	t.Parallel()

	t.Run("Should retire only the target cache without durable writes or events", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		target := deadEntityTestKey("ws-forget-entity")
		sibling := target
		sibling.EntityID = "gitlab"
		deadStore := newRecordingDeadEntityStore()
		for _, key := range []store.DeadEntityKey{target, sibling} {
			deadStore.entities[key] = store.DeadEntity{
				DeadEntityKey: key,
				Reason:        "durable failure",
				MarkedAt:      newDeadEntityTestClock().Now(),
			}
		}
		eventStore := &recordingDeadEntityEventStore{}
		service := New(deadStore, WithEventStore(eventStore))
		for _, key := range []store.DeadEntityKey{target, sibling} {
			status, err := service.Status(ctx, key)
			if err != nil || !status.Dead {
				t.Fatalf("Status(%#v) = %#v, %v, want cached dead state", key, status, err)
			}
		}

		service.ForgetEntity(store.DeadEntityKey{
			WorkspaceID: "  " + target.WorkspaceID + "  ",
			Kind:        target.Kind,
			EntityID:    "  " + target.EntityID + "  ",
		})
		deadStore.deleteEntity(target)

		forgotten, err := service.Status(ctx, target)
		if err != nil {
			t.Fatalf("Status(forgotten target) error = %v", err)
		}
		if forgotten.Dead {
			t.Fatalf("Status(forgotten target) = %#v, want fresh durable live state", forgotten)
		}
		retained, err := service.Status(ctx, sibling)
		if err != nil {
			t.Fatalf("Status(retained sibling) error = %v", err)
		}
		if !retained.Dead {
			t.Fatalf("Status(retained sibling) = %#v, want untouched cached dead state", retained)
		}
		if got := deadStore.Clears(); got != 0 {
			t.Fatalf("ForgetEntity clear calls = %d, want zero", got)
		}
		if got := len(eventStore.Summaries()); got != 0 {
			t.Fatalf("ForgetEntity transition summaries = %d, want zero", got)
		}
	})

	t.Run("Should not let a retired entity load repopulate cache", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		key := deadEntityTestKey("ws-forget-entity-inflight")
		gate := newDeadEntityTestGate()
		deadStore := &staleFindDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			findGate:                 gate,
		}
		deadStore.entities[key] = store.DeadEntity{
			DeadEntityKey: key,
			Reason:        "stale durable failure",
			MarkedAt:      newDeadEntityTestClock().Now(),
		}
		service := New(deadStore)
		result := make(chan struct {
			status Status
			err    error
		}, 1)
		go func() {
			status, err := service.Status(ctx, key)
			result <- struct {
				status Status
				err    error
			}{status: status, err: err}
		}()

		<-gate.started
		service.ForgetEntity(key)
		deadStore.deleteEntity(key)
		gate.Release()
		stale := <-result
		if stale.err != nil {
			t.Fatalf("Status(retired load) error = %v", stale.err)
		}
		if stale.status.Dead {
			t.Fatalf("Status(retired load) = %#v, want retired cache ignored", stale.status)
		}
		live, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(after retired load) error = %v", err)
		}
		if live.Dead {
			t.Fatalf("Status(after retired load) = %#v, want fresh durable live state", live)
		}
	})
}

func TestServiceReleasesStateOwnershipForIO(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should retain operation ownership without holding the state lock during durable marking",
		func(t *testing.T) {
			t.Parallel()

			gate := newDeadEntityTestGate()
			deadStore := &blockingDeadEntityStore{
				recordingDeadEntityStore: newRecordingDeadEntityStore(),
				markGate:                 gate,
			}
			service := New(deadStore, WithPermanentFailureThreshold(1))
			key := deadEntityTestKey("ws-unlocked-mark")
			result := make(chan error, 1)
			ctx := testutil.Context(t)
			go func() {
				result <- service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration")
			}()
			defer gate.Release()

			<-gate.started
			state := service.stateFor(key)
			if !state.mu.TryLock() {
				t.Fatal("RecordFailure held the state lock during MarkDeadEntity")
			}
			operation := state.operation
			state.mu.Unlock()
			if operation == nil || operation.revision == 0 {
				t.Fatalf("RecordFailure operation = %#v, want active revision owner", operation)
			}
			gate.Release()
			if err := <-result; err != nil {
				t.Fatalf("RecordFailure(blocked mark) error = %v", err)
			}
		},
	)

	t.Run("Should commit and release ownership before transition event IO", func(t *testing.T) {
		t.Parallel()

		gate := newDeadEntityTestGate()
		eventStore := &blockingDeadEntityEventStore{
			recordingDeadEntityEventStore: &recordingDeadEntityEventStore{},
			gate:                          gate,
		}
		service := New(
			newRecordingDeadEntityStore(),
			WithPermanentFailureThreshold(1),
			WithEventStore(eventStore),
			WithTransitionEventTimeout(time.Minute),
		)
		key := deadEntityTestKey("ws-unlocked-event")
		result := make(chan error, 1)
		ctx := testutil.Context(t)
		go func() {
			result <- service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration")
		}()
		defer gate.Release()

		<-gate.started
		state := service.stateFor(key)
		if !state.mu.TryLock() {
			t.Fatal("RecordFailure held the state lock during WriteEventSummary")
		}
		operation := state.operation
		state.mu.Unlock()
		if operation != nil {
			t.Fatalf("RecordFailure operation = %#v, want ownership released before event IO", operation)
		}
		status, err := service.Status(ctx, key)
		if err != nil {
			t.Fatalf("Status(during blocked event) error = %v", err)
		}
		if !status.Dead {
			t.Fatalf("Status(during blocked event) = %#v, want committed dead state", status)
		}
		gate.Release()
		if err := <-result; err != nil {
			t.Fatalf("RecordFailure(blocked event) error = %v", err)
		}
	})

	t.Run("Should publish concurrent transitions in commit revision order", func(t *testing.T) {
		t.Parallel()

		markedGate := newDeadEntityTestGate()
		clearGate := newDeadEntityTestGate()
		deadStore := &blockingDeadEntityStore{
			recordingDeadEntityStore: newRecordingDeadEntityStore(),
			clearGate:                clearGate,
		}
		eventStore := &firstMarkedBlockingDeadEntityEventStore{
			recordingDeadEntityEventStore: &recordingDeadEntityEventStore{},
			markedGate:                    markedGate,
		}
		service := New(
			deadStore,
			WithPermanentFailureThreshold(1),
			WithEventStore(eventStore),
			WithTransitionEventTimeout(time.Minute),
		)
		key := deadEntityTestKey("ws-transition-order")
		ctx := testutil.Context(t)

		markedResult := make(chan error, 1)
		go func() {
			markedResult <- service.RecordFailure(ctx, key, FailurePermanent, "invalid configuration")
		}()
		<-markedGate.started

		state := service.stateFor(key)
		if !state.mu.TryLock() {
			t.Fatal("RecordFailure held the state lock during marked event IO")
		}
		if state.operation != nil {
			state.mu.Unlock()
			t.Fatalf("RecordFailure operation = %#v, want owner released before marked event IO", state.operation)
		}
		state.mu.Unlock()

		clearedResult := make(chan error, 1)
		go func() {
			clearedResult <- service.RecordSuccess(ctx, key)
		}()
		<-clearGate.started

		statusCtx := newDoneObservedContext(ctx)
		statusResult := make(chan struct {
			status Status
			err    error
		}, 1)
		go func() {
			status, err := service.Status(statusCtx, key)
			statusResult <- struct {
				status Status
				err    error
			}{status: status, err: err}
		}()
		<-statusCtx.doneObserved
		clearGate.Release()

		observed := <-statusResult
		if observed.err != nil {
			t.Fatalf("Status(after concurrent clear commit) error = %v", observed.err)
		}
		if observed.status.Dead {
			t.Fatalf("Status(after concurrent clear commit) = %#v, want live", observed.status)
		}
		markedGate.Release()

		if err := <-markedResult; err != nil {
			t.Fatalf("RecordFailure(marked publisher) error = %v", err)
		}
		if err := <-clearedResult; err != nil {
			t.Fatalf("RecordSuccess(cleared publisher) error = %v", err)
		}
		summaries := eventStore.Summaries()
		if len(summaries) != 2 ||
			summaries[0].Type != events.DeadEntityMarked ||
			summaries[1].Type != events.DeadEntityCleared {
			t.Fatalf("transition summaries = %#v, want marked then cleared commit order", summaries)
		}
	})
}

type doneObservedContext struct {
	context.Context
	doneObserved chan struct{}
	doneOnce     sync.Once
}

func newDoneObservedContext(ctx context.Context) *doneObservedContext {
	return &doneObservedContext{
		Context:      ctx,
		doneObserved: make(chan struct{}),
	}
}

func (c *doneObservedContext) Done() <-chan struct{} {
	c.doneOnce.Do(func() {
		close(c.doneObserved)
	})
	return c.Context.Done()
}

type deadEntityTestGate struct {
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newDeadEntityTestGate() *deadEntityTestGate {
	return &deadEntityTestGate{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (g *deadEntityTestGate) Wait(ctx context.Context) error {
	g.startedOnce.Do(func() {
		close(g.started)
	})
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *deadEntityTestGate) Release() {
	g.releaseOnce.Do(func() {
		close(g.release)
	})
}

type blockingDeadEntityStore struct {
	*recordingDeadEntityStore
	findGate  *deadEntityTestGate
	markGate  *deadEntityTestGate
	clearGate *deadEntityTestGate
	listGate  *deadEntityTestGate
}

func (s *blockingDeadEntityStore) MarkDeadEntity(ctx context.Context, entity store.DeadEntity) error {
	if s.markGate != nil {
		if err := s.markGate.Wait(ctx); err != nil {
			return err
		}
	}
	return s.recordingDeadEntityStore.MarkDeadEntity(ctx, entity)
}

func (s *blockingDeadEntityStore) ClearDeadEntity(
	ctx context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) error {
	if s.clearGate != nil {
		if err := s.clearGate.Wait(ctx); err != nil {
			return err
		}
	}
	return s.recordingDeadEntityStore.ClearDeadEntity(ctx, workspaceID, kind, entityID)
}

func (s *blockingDeadEntityStore) FindDeadEntity(
	ctx context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	if s.findGate != nil {
		if err := s.findGate.Wait(ctx); err != nil {
			return store.DeadEntity{}, false, err
		}
	}
	return s.recordingDeadEntityStore.FindDeadEntity(ctx, workspaceID, kind, entityID)
}

func (s *blockingDeadEntityStore) ListDeadEntities(
	ctx context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	if s.listGate != nil {
		if err := s.listGate.Wait(ctx); err != nil {
			return nil, err
		}
	}
	return s.recordingDeadEntityStore.ListDeadEntities(ctx, workspaceID)
}

type blockingDeadEntityEventStore struct {
	*recordingDeadEntityEventStore
	gate *deadEntityTestGate
}

func (s *blockingDeadEntityEventStore) WriteEventSummary(
	ctx context.Context,
	summary store.EventSummary,
) error {
	if err := s.gate.Wait(ctx); err != nil {
		return err
	}
	return s.recordingDeadEntityEventStore.WriteEventSummary(ctx, summary)
}

type firstMarkedBlockingDeadEntityEventStore struct {
	*recordingDeadEntityEventStore
	markedGate  *deadEntityTestGate
	mu          sync.Mutex
	markedCalls int
}

func (s *firstMarkedBlockingDeadEntityEventStore) WriteEventSummary(
	ctx context.Context,
	summary store.EventSummary,
) error {
	if summary.Type == events.DeadEntityMarked {
		s.mu.Lock()
		s.markedCalls++
		firstMarked := s.markedCalls == 1
		s.mu.Unlock()
		if firstMarked {
			if err := s.markedGate.Wait(ctx); err != nil {
				return err
			}
		}
	}
	return s.recordingDeadEntityEventStore.WriteEventSummary(ctx, summary)
}

type staleFindDeadEntityStore struct {
	*recordingDeadEntityStore
	findGate *deadEntityTestGate
}

func (s *staleFindDeadEntityStore) FindDeadEntity(
	ctx context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	s.mu.Lock()
	entity, found := s.entities[store.DeadEntityKey{
		WorkspaceID: workspaceID,
		Kind:        kind,
		EntityID:    entityID,
	}]
	s.mu.Unlock()
	if err := s.findGate.Wait(ctx); err != nil {
		return store.DeadEntity{}, false, err
	}
	return entity, found, nil
}

func (s *staleFindDeadEntityStore) deleteEntity(key store.DeadEntityKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entities, key)
}

type recordingDeadEntityStore struct {
	mu sync.Mutex

	entities  map[store.DeadEntityKey]store.DeadEntity
	marked    []store.DeadEntity
	clears    int
	findErr   error
	markErr   error
	clearErr  error
	listErr   error
	listCalls []string
}

func newRecordingDeadEntityStore() *recordingDeadEntityStore {
	return &recordingDeadEntityStore{entities: make(map[store.DeadEntityKey]store.DeadEntity)}
}

func (s *recordingDeadEntityStore) deleteEntity(key store.DeadEntityKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entities, key)
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
	s.listCalls = append(s.listCalls, workspaceID)
	if s.listErr != nil {
		return nil, s.listErr
	}
	entities := make([]store.DeadEntity, 0)
	for key, entity := range s.entities {
		if key.WorkspaceID == workspaceID {
			entities = append(entities, entity)
		}
	}
	return entities, nil
}

func (s *recordingDeadEntityStore) ListCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.listCalls...)
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

func loopTargetTestKey(workspaceID, entityID string) store.DeadEntityKey {
	return store.DeadEntityKey{
		WorkspaceID: workspaceID,
		Kind:        store.DeadEntityKindLoopTarget,
		EntityID:    entityID,
	}
}
