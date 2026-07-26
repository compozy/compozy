package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/compozy/agh/internal/doctor"
)

const (
	runtimeMemoryPhaseBaseline       = "baseline"
	runtimeMemoryPhasePeriodic       = "periodic"
	runtimeMemoryPhaseShutdown       = "shutdown"
	runtimeMemoryPhaseDisabled       = "disabled"
	runtimeMemoryResidentKindCurrent = "current"
)

type residentMemoryCollector func() (bytes uint64, kind string, err error)

type runtimeMemoryTicker interface {
	C() <-chan time.Time
	Stop()
}

type runtimeMemoryMonitorOptions struct {
	now            func() time.Time
	residentMemory residentMemoryCollector
	newTicker      func(time.Duration) runtimeMemoryTicker
	report         func(doctor.RuntimeMemorySnapshot)
}

type runtimeMemoryMonitor struct {
	interval  time.Duration
	startedAt time.Time
	logger    *slog.Logger
	opts      runtimeMemoryMonitorOptions

	mu       sync.RWMutex
	snapshot doctor.RuntimeMemorySnapshot
	cancel   context.CancelFunc
	done     chan struct{}
	started  bool
}

var _ doctor.RuntimeMemorySnapshotSource = (*runtimeMemoryMonitor)(nil)

func newRuntimeMemoryMonitor(
	interval time.Duration,
	startedAt time.Time,
	logger *slog.Logger,
	options ...func(*runtimeMemoryMonitorOptions),
) *runtimeMemoryMonitor {
	opts := runtimeMemoryMonitorOptions{
		now:            time.Now,
		residentMemory: collectResidentMemory,
		newTicker: func(interval time.Duration) runtimeMemoryTicker {
			return wallClockMemoryTicker{Ticker: time.NewTicker(interval)}
		},
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	monitor := &runtimeMemoryMonitor{
		interval:  interval,
		startedAt: startedAt.UTC(),
		logger:    logger,
		opts:      opts,
	}
	if monitor.opts.report == nil {
		monitor.opts.report = monitor.logSnapshot
	}
	return monitor
}

func (m *runtimeMemoryMonitor) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: runtime memory monitor context is required")
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errors.New("daemon: runtime memory monitor already started")
	}
	m.started = true
	if m.interval == 0 {
		m.snapshot = doctor.RuntimeMemorySnapshot{Phase: runtimeMemoryPhaseDisabled}
		m.mu.Unlock()
		return nil
	}
	if m.interval < 0 {
		m.mu.Unlock()
		return fmt.Errorf("daemon: runtime memory monitor interval must be zero or positive: %s", m.interval)
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done
	m.mu.Unlock()

	m.capture(runtimeMemoryPhaseBaseline, true)
	ticker := m.opts.newTicker(m.interval)
	go m.run(runCtx, ticker, done)
	return nil
}

func (m *runtimeMemoryMonitor) RuntimeMemorySnapshot() doctor.RuntimeMemorySnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

func (m *runtimeMemoryMonitor) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: runtime memory monitor shutdown context is required")
	}
	m.mu.RLock()
	started := m.started
	cancel := m.cancel
	done := m.done
	m.mu.RUnlock()
	if !started || cancel == nil || done == nil {
		return nil
	}

	cancel()
	select {
	case <-done:
		m.capture(runtimeMemoryPhaseShutdown, false)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: join runtime memory monitor: %w", ctx.Err())
	}
}

func (m *runtimeMemoryMonitor) run(
	ctx context.Context,
	ticker runtimeMemoryTicker,
	done chan<- struct{},
) {
	defer close(done)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			m.capture(runtimeMemoryPhasePeriodic, true)
		}
	}
}

func (m *runtimeMemoryMonitor) capture(phase string, active bool) {
	var memoryStats runtime.MemStats
	runtime.ReadMemStats(&memoryStats)
	now := m.opts.now().UTC()
	residentBytes, residentKind, residentErr := m.opts.residentMemory()
	snapshot := doctor.RuntimeMemorySnapshot{
		Enabled:                 true,
		Active:                  active,
		CapturedAt:              now,
		Phase:                   phase,
		ResidentMemoryBytes:     residentBytes,
		ResidentMemoryKind:      residentKind,
		ResidentMemoryAvailable: residentErr == nil,
		HeapAllocBytes:          memoryStats.HeapAlloc,
		HeapInUseBytes:          memoryStats.HeapInuse,
		Goroutines:              runtime.NumGoroutine(),
		UptimeSeconds:           int64(max(now.Sub(m.startedAt).Seconds(), 0)),
	}
	if residentErr != nil {
		m.logger.Warn("[memory] resident memory unavailable", "error", residentErr, "phase", phase)
	}
	m.mu.Lock()
	m.snapshot = snapshot
	m.mu.Unlock()
	m.opts.report(snapshot)
}

func (m *runtimeMemoryMonitor) logSnapshot(snapshot doctor.RuntimeMemorySnapshot) {
	m.logger.Info(
		"[memory] runtime snapshot",
		"phase", snapshot.Phase,
		"resident_memory_bytes", snapshot.ResidentMemoryBytes,
		"resident_memory_kind", snapshot.ResidentMemoryKind,
		"resident_memory_available", snapshot.ResidentMemoryAvailable,
		"heap_alloc_bytes", snapshot.HeapAllocBytes,
		"heap_in_use_bytes", snapshot.HeapInUseBytes,
		"goroutines", snapshot.Goroutines,
		"uptime_seconds", snapshot.UptimeSeconds,
	)
}

type wallClockMemoryTicker struct {
	*time.Ticker
}

func (t wallClockMemoryTicker) C() <-chan time.Time { return t.Ticker.C }
