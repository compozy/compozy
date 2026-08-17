// Suite: session badge waits
// Invariant: bounded waits observe every matching canonical badge edge without leaking registrations.
// Boundary IN: session manager wait registration, predicates, pinning, resume, limits, and terminal fan-out.
// Boundary OUT: HTTP/UDS/CLI/native wiring, owned by their existing route and command suites.

package session

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

type waitCallResult struct {
	outcome WaitOutcome
	err     error
}

func TestWaitForBadgeMatchesSnapshotsAndEdges(t *testing.T) {
	t.Parallel()

	t.Run("Should return immediately when the snapshot already satisfies the predicate", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		outcome, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeIdle},
			Timeout:   time.Minute,
		})
		if err != nil {
			t.Fatalf("WaitForBadge() error = %v", err)
		}
		if outcome.Outcome != WaitResultStateReached || outcome.State != BadgeIdle {
			t.Fatalf("WaitForBadge() = %#v, want immediate idle", outcome)
		}
	})

	t.Run("Should preserve a matching edge that is left before the waiter runs", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		info := session.Info()
		h.manager.publishWaitBadgeEdge(info, BadgeWaitingForInput)
		h.manager.publishWaitBadgeEdge(info, BadgeRunning)

		got := awaitWaitCall(t, result)
		if got.err != nil {
			t.Fatalf("WaitForBadge() error = %v", got.err)
		}
		if got.outcome.Outcome != WaitResultStateReached || got.outcome.State != BadgeWaitingForInput {
			t.Fatalf("WaitForBadge() = %#v, want buffered waiting-for-input edge", got.outcome)
		}
	})

	t.Run("Should treat done as satisfying idle", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		session.mu.Lock()
		session.Liveness = &store.SessionLivenessMeta{
			Activity: &store.SessionActivityMeta{TurnID: "turn-running"},
		}
		session.mu.Unlock()
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeIdle},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		h.manager.publishWaitBadgeEdge(session.Info(), BadgeDone)
		got := awaitWaitCall(t, result)
		if got.err != nil || got.outcome.State != BadgeDone {
			t.Fatalf("WaitForBadge(done satisfies idle) = %#v, error = %v", got.outcome, got.err)
		}
	})

	t.Run("Should use the settled set when until is omitted", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		outcome, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Timeout:   time.Minute,
		})
		if err != nil {
			t.Fatalf("WaitForBadge(default settled set) error = %v", err)
		}
		if outcome.Outcome != WaitResultStateReached || outcome.State != BadgeIdle {
			t.Fatalf("WaitForBadge(default settled set) = %#v, want idle", outcome)
		}
	})
}

func TestWaitForBadgeRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  WaitRequest
	}{
		{
			name: "Should reject an unknown badge",
			req:  WaitRequest{SessionID: "sess", Until: []Badge{"sleeping"}, Timeout: time.Second},
		},
		{name: "Should reject a zero timeout", req: WaitRequest{SessionID: "sess", Timeout: 0}},
		{name: "Should reject a negative timeout", req: WaitRequest{SessionID: "sess", Timeout: -time.Second}},
		{
			name: "Should reject a timeout above the service cap",
			req:  WaitRequest{SessionID: "sess", Timeout: WaitTimeoutMax + time.Nanosecond},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h, _, _ := newWaitTestHarness(t)
			_, err := h.manager.WaitForBadge(testutil.Context(t), test.req)
			if !errors.Is(err, ErrWaitValidation) {
				t.Fatalf("WaitForBadge() error = %v, want ErrWaitValidation", err)
			}
			if count := waitRegistrationCount(h.manager, test.req.SessionID); count != 0 {
				t.Fatalf("registered waits = %d, want 0", count)
			}
		})
	}
}

func TestWaitForBadgeHandlesIdentityConcurrencyAndCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should return session gone when the pinned transcript epoch changes", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		info := session.Info()
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
			Epoch:     info.TranscriptEpoch,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		changed := *info
		changed.TranscriptEpoch++
		h.manager.publishWaitBadgeEdge(&changed, BadgeWaitingForInput)

		got := awaitWaitCall(t, result)
		if got.err != nil || got.outcome.Outcome != WaitResultSessionGone {
			t.Fatalf("WaitForBadge(epoch mismatch) = %#v, error = %v", got.outcome, got.err)
		}
	})

	t.Run("Should fan one transition out to concurrent waiters", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		first := startBadgeWait(
			t,
			h.manager,
			WaitRequest{SessionID: session.ID, Until: []Badge{BadgeWaitingForAuth}, Timeout: time.Minute},
		)
		second := startBadgeWait(
			t,
			h.manager,
			WaitRequest{SessionID: session.ID, Until: []Badge{BadgeWaitingForAuth}, Timeout: time.Minute},
		)
		awaitWaitRegistrationCount(t, h.manager, session.ID, 2)
		h.manager.publishWaitBadgeEdge(session.Info(), BadgeWaitingForAuth)

		for index, result := range []<-chan waitCallResult{first, second} {
			got := awaitWaitCall(t, result)
			if got.err != nil || got.outcome.State != BadgeWaitingForAuth {
				t.Fatalf("waiter %d = %#v, error = %v", index, got.outcome, got.err)
			}
		}
		awaitWaitRegistrationCount(t, h.manager, session.ID, 0)
	})

	t.Run("Should remove a canceled waiter", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		result := startBadgeWaitWithContext(ctx, t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		cancel()

		got := awaitWaitCall(t, result)
		if got.err != nil || got.outcome.Outcome != WaitResultCanceled {
			t.Fatalf("WaitForBadge(canceled) = %#v, error = %v", got.outcome, got.err)
		}
		awaitWaitRegistrationCount(t, h.manager, session.ID, 0)
	})

	t.Run("Should report session gone when deletion fans out", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		h.manager.publishWaitSessionGone(session.Info())

		got := awaitWaitCall(t, result)
		if got.err != nil || got.outcome.Outcome != WaitResultSessionGone {
			t.Fatalf("WaitForBadge(gone) = %#v, error = %v", got.outcome, got.err)
		}
		awaitWaitRegistrationCount(t, h.manager, session.ID, 0)
	})
}

func TestWaitForBadgeEnforcesBoundsAndGaplessResume(t *testing.T) {
	t.Parallel()

	t.Run("Should reject the thirty-third concurrent wait", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		results := make([]<-chan waitCallResult, 0, WaitMaxPerSession)
		for range WaitMaxPerSession {
			results = append(results, startBadgeWaitWithContext(ctx, t, h.manager, WaitRequest{
				SessionID: session.ID,
				Until:     []Badge{BadgeWaitingForInput},
				Timeout:   time.Minute,
			}))
		}
		awaitWaitRegistrationCount(t, h.manager, session.ID, WaitMaxPerSession)
		_, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Second,
		})
		if !errors.Is(err, ErrWaitLimit) {
			t.Fatalf("WaitForBadge(33rd) error = %v, want ErrWaitLimit", err)
		}
		cancel()
		for _, result := range results {
			got := awaitWaitCall(t, result)
			if got.err != nil || got.outcome.Outcome != WaitResultCanceled {
				t.Fatalf("canceled bounded waiter = %#v, error = %v", got.outcome, got.err)
			}
		}
	})

	t.Run("Should keep a timed out registration gapless through resume", func(t *testing.T) {
		t.Parallel()

		h, session, clock := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitTimerCount(t, clock, 1)
		clock.Advance(time.Minute)
		timedOut := awaitWaitCall(t, result)
		if timedOut.err != nil || timedOut.outcome.Outcome != WaitResultTimeout ||
			timedOut.outcome.Waited != time.Minute || timedOut.outcome.ResumeID == "" {
			t.Fatalf("WaitForBadge(timeout) = %#v, error = %v", timedOut.outcome, timedOut.err)
		}
		h.manager.publishWaitBadgeEdge(session.Info(), BadgeWaitingForInput)

		resumed, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
			ResumeID:  timedOut.outcome.ResumeID,
		})
		if err != nil {
			t.Fatalf("WaitForBadge(resume) error = %v", err)
		}
		if resumed.Outcome != WaitResultStateReached || resumed.State != BadgeWaitingForInput {
			t.Fatalf("WaitForBadge(resume) = %#v, want buffered state", resumed)
		}
	})

	t.Run("Should expire a detached registration after the resume grace", func(t *testing.T) {
		t.Parallel()

		h, session, clock := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitTimerCount(t, clock, 1)
		clock.Advance(time.Minute)
		timedOut := awaitWaitCall(t, result)
		awaitWaitTimerCount(t, clock, 1)
		clock.Advance(WaitResumeGrace)

		_, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
			ResumeID:  timedOut.outcome.ResumeID,
		})
		if !errors.Is(err, ErrWaitExpired) {
			t.Fatalf("WaitForBadge(expired resume) error = %v, want ErrWaitExpired", err)
		}
	})

	t.Run("Should report overflow instead of silently dropping the sixty-fifth edge", func(t *testing.T) {
		t.Parallel()

		h, session, clock := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitTimerCount(t, clock, 1)
		clock.Advance(time.Minute)
		timedOut := awaitWaitCall(t, result)
		for index := 0; index <= WaitEdgeBufferSize; index++ {
			badge := BadgeRunning
			if index%2 == 1 {
				badge = BadgeHung
			}
			h.manager.publishWaitBadgeEdge(session.Info(), badge)
		}

		resumed, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
			ResumeID:  timedOut.outcome.ResumeID,
		})
		if err != nil {
			t.Fatalf("WaitForBadge(overflow resume) error = %v", err)
		}
		if resumed.Outcome != WaitResultOverflow {
			t.Fatalf("WaitForBadge(overflow resume) = %#v, want overflow", resumed)
		}
	})

	t.Run("Should deliver stopped before removing the active registration", func(t *testing.T) {
		t.Parallel()

		h, session, _ := newWaitTestHarness(t)
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeStopped},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		h.manager.publishWaitBadgeEdge(session.Info(), BadgeStopped)
		got := awaitWaitCall(t, result)
		if got.err != nil || got.outcome.Outcome != WaitResultStateReached || got.outcome.State != BadgeStopped {
			t.Fatalf("WaitForBadge(stopped) = %#v, error = %v", got.outcome, got.err)
		}
		awaitWaitRegistrationCount(t, h.manager, session.ID, 0)
	})
}

func TestWaitForBadgeEmitsCompletionObservability(t *testing.T) {
	t.Parallel()

	t.Run("Should record state reached", func(t *testing.T) {
		t.Parallel()

		logs := newCaptureLogHandler()
		h, session, _ := newWaitTestHarness(t, WithLogger(slog.New(logs)))
		outcome, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeIdle},
			Timeout:   time.Minute,
		})
		if err != nil || outcome.Outcome != WaitResultStateReached {
			t.Fatalf("WaitForBadge() = %#v, error = %v", outcome, err)
		}
		assertWaitCompletionLog(t, logs, WaitResultStateReached)
	})

	t.Run("Should record timeout and resumed overflow", func(t *testing.T) {
		t.Parallel()

		logs := newCaptureLogHandler()
		h, session, clock := newWaitTestHarness(t, WithLogger(slog.New(logs)))
		result := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitTimerCount(t, clock, 1)
		clock.Advance(time.Minute)
		timedOut := awaitWaitCall(t, result)
		if timedOut.err != nil || timedOut.outcome.Outcome != WaitResultTimeout {
			t.Fatalf("WaitForBadge(timeout) = %#v, error = %v", timedOut.outcome, timedOut.err)
		}
		for index := 0; index <= WaitEdgeBufferSize; index++ {
			h.manager.publishWaitBadgeEdge(session.Info(), BadgeRunning)
		}
		resumed, err := h.manager.WaitForBadge(testutil.Context(t), WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
			ResumeID:  timedOut.outcome.ResumeID,
		})
		if err != nil || resumed.Outcome != WaitResultOverflow {
			t.Fatalf("WaitForBadge(overflow) = %#v, error = %v", resumed, err)
		}
		assertWaitCompletionLog(t, logs, WaitResultTimeout)
		assertWaitCompletionLog(t, logs, WaitResultOverflow)
	})

	t.Run("Should record cancellation and session loss", func(t *testing.T) {
		t.Parallel()

		logs := newCaptureLogHandler()
		h, session, _ := newWaitTestHarness(t, WithLogger(slog.New(logs)))
		ctx, cancel := context.WithCancel(testutil.Context(t))
		canceled := startBadgeWaitWithContext(ctx, t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		cancel()
		if got := awaitWaitCall(t, canceled); got.err != nil || got.outcome.Outcome != WaitResultCanceled {
			t.Fatalf("WaitForBadge(canceled) = %#v, error = %v", got.outcome, got.err)
		}

		gone := startBadgeWait(t, h.manager, WaitRequest{
			SessionID: session.ID,
			Until:     []Badge{BadgeWaitingForInput},
			Timeout:   time.Minute,
		})
		awaitWaitRegistrationCount(t, h.manager, session.ID, 1)
		h.manager.publishWaitSessionGone(session.Info())
		if got := awaitWaitCall(t, gone); got.err != nil || got.outcome.Outcome != WaitResultSessionGone {
			t.Fatalf("WaitForBadge(gone) = %#v, error = %v", got.outcome, got.err)
		}
		assertWaitCompletionLog(t, logs, WaitResultCanceled)
		assertWaitCompletionLog(t, logs, WaitResultSessionGone)
	})
}

func assertWaitCompletionLog(t *testing.T, logs *captureLogHandler, outcome WaitResult) {
	t.Helper()
	for _, record := range logs.Records() {
		if record.Message != "session.wait.completed" || record.Attrs["outcome"] != string(outcome) {
			continue
		}
		if record.Attrs["session_id"] == "" || record.Attrs["duration_ms"] == "" ||
			record.Attrs["predicate"] == "" {
			t.Fatalf("wait completion attrs = %#v, want session, duration, and predicate", record.Attrs)
		}
		return
	}
	t.Fatalf("wait completion outcome %q missing from records %#v", outcome, logs.Records())
}

func newWaitTestHarness(t *testing.T, opts ...Option) (*harness, *Session, *waitTestClock) {
	t.Helper()

	clock := newWaitTestClock(time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC))
	managerOpts := []Option{
		WithNow(clock.Now),
		func(manager *Manager) {
			manager.newWaitTimer = clock.NewTimer
			manager.newWaitAfterFunc = clock.AfterFunc
		},
	}
	managerOpts = append(managerOpts, opts...)
	h := newHarness(t, managerOpts...)
	cleanupTestManager(t, h.manager)
	session := createSession(t, h)
	t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
	return h, session, clock
}

func startBadgeWait(t *testing.T, manager *Manager, req WaitRequest) <-chan waitCallResult {
	t.Helper()
	return startBadgeWaitWithContext(testutil.Context(t), t, manager, req)
}

func startBadgeWaitWithContext(
	ctx context.Context,
	t *testing.T,
	manager *Manager,
	req WaitRequest,
) <-chan waitCallResult {
	t.Helper()
	result := make(chan waitCallResult, 1)
	go func() {
		outcome, err := manager.WaitForBadge(ctx, req)
		result <- waitCallResult{outcome: outcome, err: err}
	}()
	return result
}

func awaitWaitCall(t *testing.T, result <-chan waitCallResult) waitCallResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for badge wait result")
		return waitCallResult{}
	}
}

func awaitWaitRegistrationCount(t *testing.T, manager *Manager, sessionID string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := waitRegistrationCount(manager, sessionID); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("wait registrations for %q did not reach %d", sessionID, want)
		}
		runtime.Gosched()
	}
}

func waitRegistrationCount(manager *Manager, sessionID string) int {
	manager.waitRegistry.mu.Lock()
	defer manager.waitRegistry.mu.Unlock()
	return len(manager.waitRegistry.bySession[sessionID])
}

type waitTestClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*waitTestTimer]struct{}
}

type waitTestTimer struct {
	clock    *waitTestClock
	deadline time.Time
	channel  chan time.Time
	callback func()
	stopped  bool
}

func newWaitTestClock(now time.Time) *waitTestClock {
	return &waitTestClock{now: now, timers: make(map[*waitTestTimer]struct{})}
}

func (c *waitTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *waitTestClock) NewTimer(duration time.Duration) waitTimer {
	return c.addTimer(duration, nil)
}

func (c *waitTestClock) AfterFunc(duration time.Duration, callback func()) waitTimer {
	return c.addTimer(duration, callback)
}

func (c *waitTestClock) addTimer(duration time.Duration, callback func()) *waitTestTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &waitTestTimer{
		clock: c, deadline: c.now.Add(duration), channel: make(chan time.Time, 1), callback: callback,
	}
	c.timers[timer] = struct{}{}
	return timer
}

func (c *waitTestClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	now := c.now
	due := make([]*waitTestTimer, 0)
	for timer := range c.timers {
		if timer.stopped || timer.deadline.After(now) {
			continue
		}
		timer.stopped = true
		delete(c.timers, timer)
		due = append(due, timer)
	}
	c.mu.Unlock()
	for _, timer := range due {
		if timer.callback != nil {
			timer.callback()
			continue
		}
		timer.channel <- now
	}
}

func (c *waitTestClock) activeTimerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (t *waitTestTimer) Channel() <-chan time.Time {
	return t.channel
}

func (t *waitTestTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	delete(t.clock.timers, t)
	return true
}

func awaitWaitTimerCount(t *testing.T, clock *waitTestClock, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := clock.activeTimerCount(); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("active wait timers did not reach %d", want)
		}
		runtime.Gosched()
	}
}
