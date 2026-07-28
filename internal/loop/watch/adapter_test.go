package watch

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAdapterTick(t *testing.T) {
	t.Parallel()

	t.Run("Should confirm a ready settled source", func(t *testing.T) {
		t.Parallel()

		settledAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		adapter, err := NewAdapter(PollerFunc(func(_ context.Context, req PollRequest) (PollResponse, error) {
			if string(req.Spec) != `{"kind":"reviews"}` {
				t.Fatalf("PollRequest.Spec = %s, want reviews spec", string(req.Spec))
			}
			if req.ExpectedStateDigest != "sha256:prev" {
				t.Fatalf("PollRequest.ExpectedStateDigest = %q, want sha256:prev", req.ExpectedStateDigest)
			}
			return PollResponse{
				Ready:       true,
				StateDigest: "sha256:next",
				Payload:     json.RawMessage(`{"review":"r1"}`),
				SettledAt:   &settledAt,
			}, nil
		}))
		if err != nil {
			t.Fatalf("NewAdapter() error = %v", err)
		}

		result, err := adapter.Tick(context.Background(), TickRequest{
			Spec:                json.RawMessage(`{"kind":"reviews"}`),
			ExpectedStateDigest: "sha256:prev",
			LastProgressAt:      settledAt.Add(-time.Minute),
			SilenceWindow:       time.Hour,
			Now:                 settledAt,
		})
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}
		if result.Outcome != OutcomeReady {
			t.Fatalf("Outcome = %q, want ready", result.Outcome)
		}
		assertTransitions(t, result.Transitions, []Transition{
			{Event: watchEventPoll, From: watchStateIdle, To: watchStatePolling},
			{Event: watchEventReady, From: watchStatePolling, To: watchStateReady},
			{Event: watchEventSettle, From: watchStateReady, To: watchStateSettling},
			{Event: watchEventConfirm, From: watchStateSettling, To: watchStateConfirmed},
		})
		if string(result.Response.Payload) != `{"review":"r1"}` {
			t.Fatalf("Response.Payload = %s, want review payload", string(result.Response.Payload))
		}
	})

	t.Run("Should wait when source is not ready before silence window", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		adapter, err := NewAdapter(PollerFunc(func(context.Context, PollRequest) (PollResponse, error) {
			return PollResponse{Ready: false, StateDigest: "sha256:current"}, nil
		}))
		if err != nil {
			t.Fatalf("NewAdapter() error = %v", err)
		}

		result, err := adapter.Tick(context.Background(), TickRequest{
			Spec:           json.RawMessage(`{"kind":"reviews"}`),
			LastProgressAt: now.Add(-time.Minute),
			SilenceWindow:  2 * time.Minute,
			Now:            now,
		})
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}
		if result.Outcome != OutcomeWaiting {
			t.Fatalf("Outcome = %q, want waiting", result.Outcome)
		}
		if result.Response.StateDigest != "sha256:current" {
			t.Fatalf("StateDigest = %q, want sha256:current", result.Response.StateDigest)
		}
	})

	t.Run("Should stall when source is silent past window", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		adapter, err := NewAdapter(PollerFunc(func(context.Context, PollRequest) (PollResponse, error) {
			return PollResponse{Ready: false, StateDigest: "sha256:old"}, nil
		}))
		if err != nil {
			t.Fatalf("NewAdapter() error = %v", err)
		}

		result, err := adapter.Tick(context.Background(), TickRequest{
			Spec:           json.RawMessage(`{"kind":"reviews"}`),
			LastProgressAt: now.Add(-3 * time.Minute),
			SilenceWindow:  2 * time.Minute,
			Now:            now,
		})
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}
		if result.Outcome != OutcomeStalled {
			t.Fatalf("Outcome = %q, want stalled", result.Outcome)
		}
	})

	t.Run("Should surface poll failures", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("provider down")
		adapter, err := NewAdapter(PollerFunc(func(context.Context, PollRequest) (PollResponse, error) {
			return PollResponse{}, wantErr
		}))
		if err != nil {
			t.Fatalf("NewAdapter() error = %v", err)
		}

		_, err = adapter.Tick(context.Background(), TickRequest{
			Spec: json.RawMessage(`{"kind":"reviews"}`),
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("Tick() error = %v, want provider down", err)
		}
	})

	t.Run("Should reject malformed specs distinctly from missing specs", func(t *testing.T) {
		t.Parallel()

		adapter, err := NewAdapter(PollerFunc(func(context.Context, PollRequest) (PollResponse, error) {
			return PollResponse{}, errors.New("poller should not be called")
		}))
		if err != nil {
			t.Fatalf("NewAdapter() error = %v", err)
		}

		if _, err := adapter.Tick(context.Background(), TickRequest{}); !errors.Is(err, ErrSpecRequired) {
			t.Fatalf("Tick(empty spec) error = %v, want ErrSpecRequired", err)
		}
		_, err = adapter.Tick(context.Background(), TickRequest{Spec: json.RawMessage(`{`)})
		if !errors.Is(err, ErrSpecInvalid) {
			t.Fatalf("Tick(malformed spec) error = %v, want ErrSpecInvalid", err)
		}
	})
}

func TestOutputRef(t *testing.T) {
	t.Parallel()

	t.Run("Should carry pending digest into next poll", func(t *testing.T) {
		t.Parallel()

		ref, err := PendingOutputRef(PollResponse{StateDigest: "sha256:current"})
		if err != nil {
			t.Fatalf("PendingOutputRef() error = %v", err)
		}
		digest, err := ExpectedStateDigestFromOutputRef(ref)
		if err != nil {
			t.Fatalf("ExpectedStateDigestFromOutputRef() error = %v", err)
		}
		if digest != "sha256:current" {
			t.Fatalf("digest = %q, want sha256:current", digest)
		}
	})

	t.Run("Should ignore unrelated output refs", func(t *testing.T) {
		t.Parallel()

		digest, err := ExpectedStateDigestFromOutputRef(
			`{"kind":"action_output","state_digest":"sha256:other"}`,
		)
		if err != nil {
			t.Fatalf("ExpectedStateDigestFromOutputRef() error = %v", err)
		}
		if digest != "" {
			t.Fatalf("digest = %q, want empty", digest)
		}
	})

	t.Run("Should reject corrupt output refs", func(t *testing.T) {
		t.Parallel()

		if _, err := ExpectedStateDigestFromOutputRef("sha256:other"); err == nil {
			t.Fatal("ExpectedStateDigestFromOutputRef(corrupt) error = nil, want error")
		} else if !strings.Contains(err.Error(), "output ref") {
			t.Fatalf("ExpectedStateDigestFromOutputRef(corrupt) error = %v, want output ref parse error", err)
		}
	})

	t.Run("Should recover watch-events pending subscriptions and cursors", func(t *testing.T) {
		t.Parallel()

		ref, err := EventsPendingOutputRef(EventsPendingState{
			Subscriptions: []EventSubscriptionRef{{
				Kind:   "task.status_changed",
				Filter: `event.payload.to_status == "blocked"`,
			}},
			Cursors: map[string]int64{"task_events": 42},
		})
		if err != nil {
			t.Fatalf("EventsPendingOutputRef() error = %v", err)
		}
		state, ok, err := EventsPendingFromOutputRef(ref)
		if err != nil {
			t.Fatalf("EventsPendingFromOutputRef() error = %v", err)
		}
		if !ok {
			t.Fatal("EventsPendingFromOutputRef() ok = false, want true")
		}
		if got, want := len(state.Subscriptions), 1; got != want {
			t.Fatalf("subscriptions len = %d, want %d", got, want)
		}
		if got, want := state.Subscriptions[0].Kind, "task.status_changed"; got != want {
			t.Fatalf("subscription kind = %q, want %q", got, want)
		}
		if got, want := state.Cursors["task_events"], int64(42); got != want {
			t.Fatalf("cursor = %d, want %d", got, want)
		}
	})

	t.Run("Should encode watch-events confirmed events and cursors", func(t *testing.T) {
		t.Parallel()

		ref, err := EventsConfirmedOutputRef(
			[]map[string]any{{"kind": "task.status_changed", "seq": 43}},
			map[string]int64{"task_events": 43},
		)
		if err != nil {
			t.Fatalf("EventsConfirmedOutputRef() error = %v", err)
		}
		var decoded OutputRef
		if err := json.Unmarshal([]byte(ref), &decoded); err != nil {
			t.Fatalf("Unmarshal confirmed ref error = %v", err)
		}
		if got, want := decoded.Kind, "watch_events_confirmed"; got != want {
			t.Fatalf("decoded.Kind = %q, want %q", got, want)
		}
		if got, want := decoded.Cursors["task_events"], int64(43); got != want {
			t.Fatalf("cursor = %d, want %d", got, want)
		}
		if !strings.Contains(string(decoded.Events), `"seq":43`) {
			t.Fatalf("events = %s, want encoded event", string(decoded.Events))
		}
	})

	t.Run("Should handle watch-events output ref edge cases", func(t *testing.T) {
		t.Parallel()

		if state, ok, err := EventsPendingFromOutputRef(""); err != nil || ok || len(state.Cursors) != 0 {
			t.Fatalf("EventsPendingFromOutputRef(empty) = (%#v, %v, %v), want zero/false/nil", state, ok, err)
		}
		if state, ok, err := EventsPendingFromOutputRef(`{"kind":"watch_source_pending"}`); err != nil ||
			ok || len(state.Subscriptions) != 0 {
			t.Fatalf("EventsPendingFromOutputRef(other) = (%#v, %v, %v), want zero/false/nil", state, ok, err)
		}
		if _, _, err := EventsPendingFromOutputRef("{"); err == nil {
			t.Fatal("EventsPendingFromOutputRef(corrupt) error = nil, want error")
		}
		if _, err := EventsConfirmedOutputRef(make(chan int), map[string]int64{"task_events": 1}); err == nil {
			t.Fatal("EventsConfirmedOutputRef(unmarshalable) error = nil, want error")
		}
		if _, err := marshalOutputRef(OutputRef{}); err == nil {
			t.Fatal("marshalOutputRef(empty kind) error = nil, want error")
		}
	})
}

func assertTransitions(t *testing.T, got []Transition, want []Transition) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(Transitions) = %d, want %d: %#v", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("Transitions[%d] = %#v, want %#v", idx, got[idx], want[idx])
		}
	}
}
