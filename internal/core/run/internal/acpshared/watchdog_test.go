package acpshared

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
)

// fakeClock drives the activity monitor and stall watchdog under test control.
// Now advances only via set/advance; advance additionally delivers one blocking
// tick to every registered ticker so the watchdog goroutine processes idle
// samples in a deterministic order without real sleeps.
type fakeClock struct {
	mu         sync.Mutex
	now        time.Time
	tickers    []*fakeTicker
	registered chan struct{}
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start, registered: make(chan struct{}, 4)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func (c *fakeClock) NewTicker(time.Duration) runshared.Ticker {
	tk := &fakeTicker{ch: make(chan time.Time)}
	c.mu.Lock()
	c.tickers = append(c.tickers, tk)
	c.mu.Unlock()
	select {
	case c.registered <- struct{}{}:
	default:
	}
	return tk
}

func (c *fakeClock) waitForTicker(t *testing.T) {
	t.Helper()
	select {
	case <-c.registered:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog ticker was never created")
	}
}

// advance moves the clock forward and delivers a blocking tick to each ticker.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	tickers := append([]*fakeTicker(nil), c.tickers...)
	c.mu.Unlock()
	for _, tk := range tickers {
		tk.ch <- now
	}
}

var _ runshared.Clock = (*fakeClock)(nil)

type fakeTicker struct {
	ch chan time.Time
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop()               {}

// recordingHandler captures emitted slog records so idle-threshold diagnostics
// can be asserted deterministically.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) thresholds() []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []int
	for i := range h.records {
		r := h.records[i]
		if r.Message != "acp stall watchdog idle threshold crossed" {
			continue
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "threshold_pct" {
				out = append(out, int(a.Value.Int64()))
			}
			return true
		})
	}
	return out
}

func TestWatchdogIdleTimeoutArmsIndependentlyOfTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  *config
		want time.Duration
	}{
		{
			name: "armed with Timeout=0 when stall enabled",
			cfg:  &config{Timeout: 0, Stall: model.StallPolicy{Enabled: true, IdleTimeout: 3 * time.Minute}},
			want: 3 * time.Minute,
		},
		{
			name: "disabled stall never arms even with a positive Timeout",
			cfg: &config{
				Timeout: 10 * time.Minute,
				Stall:   model.StallPolicy{Enabled: false, IdleTimeout: 3 * time.Minute},
			},
			want: 0,
		},
		{
			name: "zero idle window never arms",
			cfg:  &config{Stall: model.StallPolicy{Enabled: true, IdleTimeout: 0}},
			want: 0,
		},
		{
			name: "nil config never arms",
			cfg:  nil,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := watchdogIdleTimeout(tt.cfg); got != tt.want {
				t.Fatalf("watchdogIdleTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWatchdogTickFiresAndReapsTerminalsAfterIdleWindow(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	clk := newFakeClock(start)
	monitor := runshared.NewActivityMonitorWithClock(clk)
	monitor.RecordActivity()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	idle := 3 * time.Minute
	killed := 0
	cfg := ActivityWatchdogConfig{
		Monitor:       monitor,
		IdleTimeout:   idle,
		Cancel:        cancel,
		KillTerminals: func() { killed++ },
		Logger:        silentLogger(),
	}
	warner := &idleThresholdWarner{}

	t.Run("Should not fire before the idle window elapses", func(t *testing.T) {
		clk.set(start.Add(idle - time.Second))
		if watchdogTick(cfg, warner) {
			t.Fatal("watchdog fired before the idle window elapsed")
		}
		if ctx.Err() != nil {
			t.Fatalf("attempt canceled early: %v", context.Cause(ctx))
		}
	})

	t.Run("Should fire and reap terminals once the idle window elapses", func(t *testing.T) {
		clk.set(start.Add(idle))
		if !watchdogTick(cfg, warner) {
			t.Fatal("watchdog did not fire at the idle window")
		}
		if killed != 1 {
			t.Fatalf("killTerminals calls = %d, want 1 (reap before cancel)", killed)
		}
		if !IsActivityTimeout(context.Cause(ctx)) {
			t.Fatalf("cancel cause = %v, want typed activity timeout", context.Cause(ctx))
		}
	})
}

func TestWatchdogTickResetsIdleWindowOnLateUpdate(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	clk := newFakeClock(start)
	monitor := runshared.NewActivityMonitorWithClock(clk)
	monitor.RecordActivity()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	idle := 100 * time.Second
	cfg := ActivityWatchdogConfig{Monitor: monitor, IdleTimeout: idle, Cancel: cancel, Logger: silentLogger()}
	warner := &idleThresholdWarner{}

	// 90% of the window with no update: must not fire.
	clk.set(start.Add(90 * time.Second))
	if watchdogTick(cfg, warner) {
		t.Fatal("watchdog fired at 90% of the idle window")
	}
	// A late update resets the idle clock.
	monitor.RecordActivity()
	// 90 seconds after the reset (< full window): must not fire.
	clk.set(start.Add(180 * time.Second))
	if watchdogTick(cfg, warner) {
		t.Fatal("watchdog fired within a fresh idle window after the reset")
	}
	if ctx.Err() != nil {
		t.Fatalf("attempt canceled during a fresh idle window: %v", context.Cause(ctx))
	}
	// A full fresh window after the reset: now it fires.
	clk.set(start.Add(190 * time.Second))
	if !watchdogTick(cfg, warner) {
		t.Fatal("watchdog did not fire after a full fresh idle window post-reset")
	}
}

func TestWatchdogTickEmitsIdleThresholdDiagnosticsOncePerAttempt(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	clk := newFakeClock(start)
	monitor := runshared.NewActivityMonitorWithClock(clk)
	monitor.RecordActivity()
	_, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	idle := 100 * time.Second
	rec := &recordingHandler{}
	cfg := ActivityWatchdogConfig{
		Monitor:     monitor,
		IdleTimeout: idle,
		Cancel:      cancel,
		Logger:      slog.New(rec),
		SessionID:   "sess-1",
	}
	warner := &idleThresholdWarner{}

	samples := []time.Duration{55, 60, 85, 90}
	for _, sec := range samples {
		clk.set(start.Add(sec * time.Second))
		if watchdogTick(cfg, warner) {
			t.Fatalf("watchdog fired early at %ds of idle", sec)
		}
	}
	clk.set(start.Add(idle))
	if !watchdogTick(cfg, warner) {
		t.Fatal("watchdog did not fire at the idle window")
	}
	if got := rec.thresholds(); !slices.Equal(got, []int{50, 80}) {
		t.Fatalf("emitted idle thresholds = %v, want exactly one 50 and one 80", got)
	}
}

// terminalReapClient is a fake agent.Client that also exposes CloseTerminals so
// watchdogTerminalKiller's reap path can be exercised.
type terminalReapClient struct {
	*pauseResumeClient
	closed int
}

func (c *terminalReapClient) CloseTerminals() error {
	c.closed++
	return nil
}

func TestWatchdogTerminalKillerReapsTerminalsThroughClient(t *testing.T) {
	t.Parallel()
	t.Run("Should return a reaper that closes the client's terminals", func(t *testing.T) {
		t.Parallel()
		client := &terminalReapClient{pauseResumeClient: &pauseResumeClient{}}
		reap := watchdogTerminalKiller(&SessionExecution{Client: client, Logger: silentLogger()})
		if reap == nil {
			t.Fatal("expected a terminal reaper for a CloseTerminals-capable client")
		}
		reap()
		if client.closed != 1 {
			t.Fatalf("CloseTerminals calls = %d, want 1", client.closed)
		}
	})
	t.Run("Should skip clients that do not expose terminal teardown", func(t *testing.T) {
		t.Parallel()
		if reap := watchdogTerminalKiller(&SessionExecution{Client: &pauseResumeClient{}}); reap != nil {
			t.Fatal("expected nil reaper for a client without CloseTerminals")
		}
		if reap := watchdogTerminalKiller(nil); reap != nil {
			t.Fatal("expected nil reaper for a nil execution")
		}
	})
}

func TestActivityWatchdogCancelsSilentTurnAsRetryable(t *testing.T) {
	// Arm-by-default: Timeout=0 but stall enabled must still guard the run.
	cfg := &config{Timeout: 0, Stall: model.StallPolicy{Enabled: true, IdleTimeout: 100 * time.Second}}
	idle := watchdogIdleTimeout(cfg)
	if idle <= 0 {
		t.Fatalf("watchdog not armed with Timeout=0: idle=%v", idle)
	}

	clk := newFakeClock(time.Unix(0, 0))
	monitor := runshared.NewActivityMonitorWithClock(clk)
	monitor.RecordActivity()

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	session := newControlledPromptSession("sess-silent")
	defer session.finish(context.Canceled)
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    ctx,
		Index:      0,
		SessionID:  session.ID(),
		RunID:      "run-1",
		RunJournal: &stubRuntimeEventSubmitter{},
	})
	execution := &SessionExecution{Session: session, Handler: handler, Logger: silentLogger()}
	j := &job{SafeName: "task_01"}

	killed := make(chan struct{}, 1)
	stop := StartACPActivityWatchdog(ActivityWatchdogConfig{
		Ctx:           ctx,
		Monitor:       monitor,
		IdleTimeout:   idle,
		Cancel:        cancel,
		KillTerminals: func() { killed <- struct{}{} },
		Clock:         clk,
	})
	defer stop()

	resultCh := make(chan JobAttemptResult, 1)
	go func() {
		res, _ := executeSingleSessionTurn(ctx, idle, execution, j, 0, false, nil)
		resultCh <- res
	}()

	clk.waitForTicker(t)
	clk.advance(idle)

	select {
	case res := <-resultCh:
		if res.Status != attemptStatusTimeout {
			t.Fatalf("attempt status = %v, want timeout", res.Status)
		}
		if !res.Retryable {
			t.Fatal("stall should be classified retryable")
		}
		if res.Failure == nil || !IsActivityTimeout(res.Failure.Err) {
			t.Fatalf("failure = %#v, want typed activity timeout", res.Failure)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("silent turn was not canceled within the idle window")
	}
	select {
	case <-killed:
	case <-time.After(time.Second):
		t.Fatal("session terminals were not reaped on stall")
	}
}
