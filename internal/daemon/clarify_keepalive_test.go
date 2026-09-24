package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type recordingClarifyKeepalive struct {
	mu    sync.Mutex
	pings []ClarifyPingParams
	wake  chan ClarifyPingParams
	fail  error
}

func newRecordingClarifyKeepalive() *recordingClarifyKeepalive {
	return &recordingClarifyKeepalive{wake: make(chan ClarifyPingParams, 64)}
}

func (r *recordingClarifyKeepalive) Ping(_ context.Context, ping ClarifyPingParams) error {
	r.mu.Lock()
	r.pings = append(r.pings, ping)
	r.mu.Unlock()
	select {
	case r.wake <- ping:
	default:
	}
	return r.fail
}

func (r *recordingClarifyKeepalive) await(t *testing.T) ClarifyPingParams {
	t.Helper()
	select {
	case ping := <-r.wake:
		return ping
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clarification keepalive ping")
		return ClarifyPingParams{}
	}
}

func (r *recordingClarifyKeepalive) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pings)
}

func (r *recordingClarifyKeepalive) awaitStableCount(t *testing.T) int {
	t.Helper()
	deadline := time.After(2 * time.Second)
	last, quietSince := r.count(), time.Now()
	for {
		select {
		case <-deadline:
			t.Fatalf("keepalive pings never stabilized (last count %d)", last)
			return 0
		case <-time.After(time.Millisecond):
		}
		if got := r.count(); got != last {
			last, quietSince = got, time.Now()
			continue
		}
		if time.Since(quietSince) >= 100*time.Millisecond {
			return last
		}
	}
}

func (r *recordingClarifyKeepalive) seqs() []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	seqs := make([]uint64, 0, len(r.pings))
	for _, ping := range r.pings {
		seqs = append(seqs, ping.Seq)
	}
	return seqs
}

type manualClarifyTicker struct {
	ch        chan time.Time
	mu        sync.Mutex
	intervals []time.Duration
	stops     int
}

func newManualClarifyTicker() *manualClarifyTicker {
	return &manualClarifyTicker{ch: make(chan time.Time, 64)}
}

func (m *manualClarifyTicker) ticker(interval time.Duration) (<-chan time.Time, func()) {
	m.mu.Lock()
	m.intervals = append(m.intervals, interval)
	m.mu.Unlock()
	return m.ch, func() {
		m.mu.Lock()
		m.stops++
		m.mu.Unlock()
	}
}

func (m *manualClarifyTicker) fire() {
	m.ch <- time.Now()
}

func (m *manualClarifyTicker) awaitStop(t *testing.T) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		m.mu.Lock()
		stops := m.stops
		m.mu.Unlock()
		if stops >= 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("keepalive ticker was not stopped after the wait ended")
		case <-time.After(time.Millisecond):
		}
	}
}

// blockingClarifyKeepalive wedges inside Ping until released, proving the wait loop stays responsive.
type blockingClarifyKeepalive struct {
	mu      sync.Mutex
	pings   []ClarifyPingParams
	entered chan struct{}
	release chan struct{}
}

func newBlockingClarifyKeepalive() *blockingClarifyKeepalive {
	return &blockingClarifyKeepalive{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (k *blockingClarifyKeepalive) Ping(_ context.Context, ping ClarifyPingParams) error {
	select {
	case k.entered <- struct{}{}:
	default:
	}
	<-k.release
	k.mu.Lock()
	k.pings = append(k.pings, ping)
	k.mu.Unlock()
	return nil
}

func (k *blockingClarifyKeepalive) awaitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-k.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive send never started")
	}
}

func (k *blockingClarifyKeepalive) awaitCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		k.mu.Lock()
		got := len(k.pings)
		k.mu.Unlock()
		if got == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("keepalive pings = %d, want %d", got, want)
		case <-time.After(time.Millisecond):
		}
	}
}

func (k *blockingClarifyKeepalive) unblock() {
	select {
	case <-k.release:
	default:
		close(k.release)
	}
}

func (m *manualClarifyTicker) cadence() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.intervals) == 0 {
		return 0
	}
	return m.intervals[0]
}

func newTestClarifyBridgeWithKeepalive(
	t *testing.T,
	timeout time.Duration,
	publisher clarifyEventPublisher,
	keepalive ClarifyKeepalive,
	ticker *manualClarifyTicker,
	options ...clarifyBridgeOption,
) *clarifyBridge {
	t.Helper()
	args := []clarifyBridgeOption{
		withClarifyIDGenerator(func() string { return "clarify-request" }),
		withClarifyKeepalive(keepalive),
	}
	if ticker != nil {
		args = append(args, withClarifyPingTicker(ticker.ticker))
	}
	args = append(args, options...)
	bridge, err := newClarifyBridge(
		timeout,
		publisher,
		&clarifySummaryStub{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		args...,
	)
	if err != nil {
		t.Fatalf("newClarifyBridge() error = %v", err)
	}
	return bridge
}

func assertClarifyPingSeqs(t *testing.T, keepalive *recordingClarifyKeepalive, want ...uint64) {
	t.Helper()
	deadline := time.After(time.Second)
	for keepalive.count() < len(want) {
		select {
		case <-deadline:
			t.Fatalf("keepalive pings = %v, want seqs %v", keepalive.seqs(), want)
		case <-time.After(time.Millisecond):
		}
	}
	if got := keepalive.seqs(); !equalUint64s(got, want) {
		t.Fatalf("keepalive seqs = %v, want %v", got, want)
	}
}

func equalUint64s(left, right []uint64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestClarifyBridgeKeepaliveTicker(t *testing.T) {
	t.Parallel()

	t.Run("Should ping immediately with seq 1 then on the 30s cadence", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge := newTestClarifyBridgeWithKeepalive(t, 0, publisher, keepalive, ticker)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Which env first?"})
		publisher.await(t)
		first := keepalive.await(t)
		if first.SessionID != scope.SessionID || first.RequestID != "clarify-request" || first.Seq != 1 {
			t.Fatalf("first ping = %#v, want session/request identity with seq 1", first)
		}
		if !first.Deadline.IsZero() {
			t.Fatalf("first ping deadline = %s, want zero for unbounded", first.Deadline)
		}
		if clarifyKeepaliveInterval != 30*time.Second {
			t.Fatalf("keepalive cadence = %s, want the fixed 30s protocol margin", clarifyKeepaliveInterval)
		}
		if got := ticker.cadence(); got != clarifyKeepaliveInterval {
			t.Fatalf("ticker cadence = %s, want fixed %s", got, clarifyKeepaliveInterval)
		}
		ticker.fire()
		second := keepalive.await(t)
		if second.Seq != 2 || second.RequestID != first.RequestID {
			t.Fatalf("second ping = %#v, want seq 2 for the same request", second)
		}
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should ping exactly on cadence over a long wait", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge := newTestClarifyBridgeWithKeepalive(t, 0, publisher, keepalive, ticker)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		keepalive.await(t)
		const ticks = 10
		for range ticks {
			ticker.fire()
		}
		want := make([]uint64, 0, ticks+1)
		for seq := uint64(1); seq <= ticks+1; seq++ {
			want = append(want, seq)
		}
		assertClarifyPingSeqs(t, keepalive, want...)
		bridge.CancelSession(scope.SessionID)
		awaitClarifyResult(t, result)
	})

	t.Run("Should keep waiting and resolve a late answer when every ping fails", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		keepalive.fail = errors.New("agent gone")
		ticker := newManualClarifyTicker()
		bridge, err := newClarifyBridge(
			0,
			publisher,
			&clarifySummaryStub{},
			logger,
			withClarifyIDGenerator(func() string { return "clarify-request" }),
			withClarifyKeepalive(keepalive),
			withClarifyPingTicker(ticker.ticker),
		)
		if err != nil {
			t.Fatalf("newClarifyBridge() error = %v", err)
		}
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		keepalive.await(t)
		ticker.fire()
		keepalive.await(t)
		assertClarifyBlocked(t, result)
		pending := awaitPendingClarification(t, bridge, scope)
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{Text: "staging"},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "staging" {
			t.Fatalf("Ask() result = %#v, want staging", got)
		}
		if logged := logs.String(); !strings.Contains(logged, "clarification keepalive ping failed") {
			t.Fatalf("bridge logs = %q, want a debug line per failed ping", logged)
		}
	})

	t.Run("Should stop pinging after answer expiry and cancel", func(t *testing.T) {
		t.Parallel()

		runTerminal := func(t *testing.T, name string, settle func(
			t *testing.T,
			bridge *clarifyBridge,
			scope toolspkg.Scope,
			result <-chan clarifyResult,
			keepalive *recordingClarifyKeepalive,
			ticker *manualClarifyTicker,
		)) {
			t.Helper()
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				publisher := newClarifyPublisherStub()
				keepalive := newRecordingClarifyKeepalive()
				ticker := newManualClarifyTicker()
				bridge := newTestClarifyBridgeWithKeepalive(t, 0, publisher, keepalive, ticker)
				scope := testClarifyScope()
				result := askClarification(
					t,
					bridge,
					scope,
					toolspkg.ClarifyQuestion{Question: "Continue?"},
				)
				publisher.await(t)
				keepalive.await(t)
				settle(t, bridge, scope, result, keepalive, ticker)
				settled := keepalive.count()
				ticker.fire()
				ticker.fire()
				ticker.awaitStop(t)
				if got := keepalive.count(); got != settled {
					t.Fatalf("keepalive pings after terminal = %d, want %d", got, settled)
				}
			})
		}
		runTerminal(t, "Should stop pinging after an answer", func(
			t *testing.T,
			bridge *clarifyBridge,
			scope toolspkg.Scope,
			result <-chan clarifyResult,
			_ *recordingClarifyKeepalive,
			_ *manualClarifyTicker,
		) {
			t.Helper()
			pending := awaitPendingClarification(t, bridge, scope)
			if _, err := bridge.Answer(
				testutil.Context(t),
				scope,
				pending[0].RequestID,
				toolspkg.ClarifyAnswerRequest{Text: "yes"},
			); err != nil {
				t.Fatalf("Answer() error = %v", err)
			}
			if got := awaitClarifyResult(t, result); got.err != nil {
				t.Fatalf("Ask() error = %v", got.err)
			}
		})
		runTerminal(t, "Should stop pinging after cancel", func(
			t *testing.T,
			bridge *clarifyBridge,
			scope toolspkg.Scope,
			result <-chan clarifyResult,
			_ *recordingClarifyKeepalive,
			_ *manualClarifyTicker,
		) {
			t.Helper()
			bridge.CancelSession(scope.SessionID)
			if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
				t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
			}
		})
	})

	t.Run("Should stop pinging after finite expiry with one fallback resolution", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge := newTestClarifyBridgeWithKeepalive(t, 30*time.Millisecond, publisher, keepalive, ticker)
		result := askClarification(
			t,
			bridge,
			testClarifyScope(),
			toolspkg.ClarifyQuestion{Question: "Continue?"},
		)
		publisher.await(t)
		keepalive.await(t)
		ticker.fire()
		keepalive.await(t)
		got := awaitClarifyResult(t, result)
		if got.err != nil || !got.answer.Fallback {
			t.Fatalf("Ask() result = %#v, want one fallback resolution", got)
		}
		if statuses := publisher.statuses(); !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusTimedOut,
		}) {
			t.Fatalf("published statuses = %v, want pending then timed_out", statuses)
		}
		settled := keepalive.count()
		ticker.fire()
		ticker.fire()
		ticker.awaitStop(t)
		if count := keepalive.count(); count != settled {
			t.Fatalf("keepalive pings after expiry = %d, want %d", count, settled)
		}
	})

	t.Run("Should wait exactly as before without a keepalive", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 0, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		assertClarifyBlocked(t, result)
		pending := awaitPendingClarification(t, bridge, scope)
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{Text: "yes"},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "yes" {
			t.Fatalf("Ask() result = %#v, want yes", got)
		}
	})

	t.Run("Should resolve an answer racing a tick exactly once", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge := newTestClarifyBridgeWithKeepalive(t, 0, publisher, keepalive, ticker)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		keepalive.await(t)

		const rounds = 8
		var wg sync.WaitGroup
		for range rounds {
			wg.Add(2)
			go func() {
				defer wg.Done()
				// Race losers return not-found; exactly-once is asserted on
				// the published statuses below.
				if _, err := bridge.Answer(
					context.Background(),
					scope,
					"clarify-request",
					toolspkg.ClarifyAnswerRequest{Text: "yes"},
				); err != nil && !errors.Is(err, toolspkg.ErrClarifyNotFound) {
					t.Errorf("Answer() error = %v, want nil or not-found for race losers", err)
				}
			}()
			go func() {
				defer wg.Done()
				ticker.fire()
			}()
		}
		wg.Wait()
		got := awaitClarifyResult(t, result)
		if got.err != nil || got.answer.Text != "yes" || got.answer.Fallback {
			t.Fatalf("Ask() result = %#v, want exactly one winning answer", got)
		}
		if statuses := publisher.statuses(); !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusResolved,
		}) {
			t.Fatalf("published statuses = %v, want pending plus one terminal event", statuses)
		}
		// The sender runs off the wait loop, so in-flight tick sends may land
		// after the terminal result; stabilize before snapshotting.
		settled := keepalive.awaitStableCount(t)
		if settled == 0 || settled > 1+rounds {
			t.Fatalf("keepalive pings = %d, want between 1 and %d (no bursts)", settled, 1+rounds)
		}
		for range 4 {
			ticker.fire()
		}
		ticker.awaitStop(t)
		if count := keepalive.count(); count != settled {
			t.Fatalf("keepalive pings after terminal = %d, want %d", count, settled)
		}
	})

	t.Run("Should resolve a terminal answer while a ping send is wedged", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		keepalive := newBlockingClarifyKeepalive()
		t.Cleanup(keepalive.unblock)
		ticker := newManualClarifyTicker()
		bridge := newTestClarifyBridgeWithKeepalive(t, 0, publisher, keepalive, ticker)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		keepalive.awaitEntered(t)
		pending := awaitPendingClarification(t, bridge, scope)
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{Text: "yes"},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		select {
		case got := <-result:
			if got.err != nil || got.answer.Text != "yes" {
				t.Fatalf("Ask() result = %#v, want yes while the ping send is wedged", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Ask() stayed blocked on a wedged keepalive send")
		}
		keepalive.unblock()
		// The wedged send lands exactly once post-terminal; the sender then
		// exits and later ticks stay silent.
		keepalive.awaitCount(t, 1)
		ticker.awaitStop(t)
		ticker.fire()
		ticker.fire()
		keepalive.awaitCount(t, 1)
	})

	t.Run("Should carry identity only with a null unbounded deadline", func(t *testing.T) {
		t.Parallel()

		clock := newStubClarifyClock(time.Now().UTC())
		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge, err := newClarifyBridge(
			0,
			publisher,
			&clarifySummaryStub{},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			withClarifyIDGenerator(func() string { return "clarify-request" }),
			withClarifyClock(clock.Now),
			withClarifyKeepalive(keepalive),
			withClarifyPingTicker(ticker.ticker),
		)
		if err != nil {
			t.Fatalf("newClarifyBridge() error = %v", err)
		}
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		ping := keepalive.await(t)
		if ping.SessionID != scope.SessionID || ping.RequestID != "clarify-request" || ping.Seq != 1 {
			t.Fatalf("ping identity = %#v, want session/request with seq 1", ping)
		}
		if !ping.AskedAt.Equal(clock.Now()) {
			t.Fatalf("ping asked_at = %s, want stub-clock %s", ping.AskedAt, clock.Now())
		}
		if !ping.Deadline.IsZero() {
			t.Fatalf("ping deadline = %s, want zero for unbounded", ping.Deadline)
		}
		bridge.CancelSession(scope.SessionID)
		awaitClarifyResult(t, result)
	})

	t.Run("Should carry the finite deadline on each ping", func(t *testing.T) {
		t.Parallel()

		clock := newStubClarifyClock(time.Now().UTC())
		publisher := newClarifyPublisherStub()
		keepalive := newRecordingClarifyKeepalive()
		ticker := newManualClarifyTicker()
		bridge, err := newClarifyBridge(
			5*time.Minute,
			publisher,
			&clarifySummaryStub{},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			withClarifyIDGenerator(func() string { return "clarify-request" }),
			withClarifyClock(clock.Now),
			withClarifyKeepalive(keepalive),
			withClarifyPingTicker(ticker.ticker),
		)
		if err != nil {
			t.Fatalf("newClarifyBridge() error = %v", err)
		}
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		ping := keepalive.await(t)
		wantDeadline := clock.Now().Add(5 * time.Minute)
		if !ping.Deadline.Equal(wantDeadline) {
			t.Fatalf("ping deadline = %s, want pinned %s", ping.Deadline, wantDeadline)
		}
		bridge.CancelSession(scope.SessionID)
		awaitClarifyResult(t, result)
	})
}

func TestClarifyKeepaliveResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve nil sessions to a nil keepalive", func(t *testing.T) {
		t.Parallel()

		if keepalive := clarifyKeepaliveForSessions(nil); keepalive != nil {
			t.Fatalf("clarifyKeepaliveForSessions(nil) = %#v, want nil", keepalive)
		}
		if keepalive := newClarifyKeepaliveResolver(nil); keepalive != nil {
			t.Fatalf("newClarifyKeepaliveResolver(nil) = %#v, want nil", keepalive)
		}
		var resolver *clarifyKeepaliveResolver
		if err := resolver.Ping(
			testutil.Context(t),
			ClarifyPingParams{SessionID: "session-one", RequestID: "request-one", Seq: 1},
		); err != nil {
			t.Fatalf("nil resolver Ping() error = %v, want fail-open nil", err)
		}
	})

	t.Run("Should forward the identity-only payload over the extension method", func(t *testing.T) {
		t.Parallel()

		var (
			gotSession string
			gotMethod  string
			gotParams  any
		)
		keepalive := newClarifyKeepaliveResolver(clarifyNotifierFunc(
			func(_ context.Context, sessionID, method string, params any) error {
				gotSession, gotMethod, gotParams = sessionID, method, params
				return nil
			},
		))
		askedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
		deadline := askedAt.Add(5 * time.Minute)
		if err := keepalive.Ping(testutil.Context(t), ClarifyPingParams{
			SessionID: "session-one",
			RequestID: "request-one",
			Seq:       7,
			AskedAt:   askedAt,
			Deadline:  deadline,
		}); err != nil {
			t.Fatalf("Ping() error = %v", err)
		}
		if gotSession != "session-one" || gotMethod != "_compozy/clarify_ping" {
			t.Fatalf("Ping() target = %q/%q, want session-one/_compozy/clarify_ping", gotSession, gotMethod)
		}
		wire, ok := gotParams.(map[string]any)
		if !ok {
			t.Fatalf("Ping() params = %#v, want the wire map", gotParams)
		}
		payload, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("json.Marshal(params) error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(params) error = %v", err)
		}
		want := map[string]any{
			"session_id": "session-one",
			"request_id": "request-one",
			"seq":        float64(7),
			"asked_at":   "2026-09-17T10:00:00Z",
			"deadline":   "2026-09-17T10:05:00Z",
		}
		if len(decoded) != len(want) {
			t.Fatalf("Ping() params = %v, want exactly %v", decoded, want)
		}
		for key, value := range want {
			if decoded[key] != value {
				t.Fatalf("Ping() params[%q] = %v, want %v (full %v)", key, decoded[key], value, decoded)
			}
		}
	})

	t.Run("Should render a null deadline for unbounded waits", func(t *testing.T) {
		t.Parallel()

		var gotParams any
		keepalive := newClarifyKeepaliveResolver(clarifyNotifierFunc(
			func(_ context.Context, _, _ string, params any) error {
				gotParams = params
				return nil
			},
		))
		if err := keepalive.Ping(testutil.Context(t), ClarifyPingParams{
			SessionID: "session-one",
			RequestID: "request-one",
			Seq:       1,
			AskedAt:   time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("Ping() error = %v", err)
		}
		payload, err := json.Marshal(gotParams)
		if err != nil {
			t.Fatalf("json.Marshal(params) error = %v", err)
		}
		if !strings.Contains(string(payload), `"deadline":null`) {
			t.Fatalf("Ping() params = %s, want a null deadline", payload)
		}
	})

	t.Run("Should reject pings without identity", func(t *testing.T) {
		t.Parallel()

		keepalive := newClarifyKeepaliveResolver(clarifyNotifierFunc(
			func(context.Context, string, string, any) error { return nil },
		))
		for name, ping := range map[string]ClarifyPingParams{
			"missing session":    {RequestID: "request-one", Seq: 1},
			"missing request":    {SessionID: "session-one", Seq: 1},
			"zero sequence":      {SessionID: "session-one", RequestID: "request-one"},
			"missing everything": {},
		} {
			if err := keepalive.Ping(testutil.Context(t), ping); err == nil {
				t.Fatalf("Ping(%s) error = nil, want identity validation", name)
			}
		}
	})

	t.Run("Should surface delivery failures to the fail-open caller", func(t *testing.T) {
		t.Parallel()

		keepalive := newClarifyKeepaliveResolver(clarifyNotifierFunc(
			func(context.Context, string, string, any) error { return errors.New("no live process") },
		))
		err := keepalive.Ping(testutil.Context(t), ClarifyPingParams{
			SessionID: "session-one",
			RequestID: "request-one",
			Seq:       1,
			AskedAt:   time.Now().UTC(),
		})
		if err == nil || !strings.Contains(err.Error(), "no live process") {
			t.Fatalf("Ping() error = %v, want the delivery failure", err)
		}
	})
}

type clarifyNotifierFunc func(ctx context.Context, sessionID, method string, params any) error

func (f clarifyNotifierFunc) NotifyAgentExtension(
	ctx context.Context,
	sessionID, method string,
	params any,
) error {
	return f(ctx, sessionID, method, params)
}
