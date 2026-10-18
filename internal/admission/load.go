package admission

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"sync"
	"time"
)

// DefaultLoadSampleInterval is how often the load monitor reads the host load average.
const DefaultLoadSampleInterval = 5 * time.Second

// ErrLoadAverageUnsupported reports a platform without a cheap load-average source.
var ErrLoadAverageUnsupported = errors.New("admission: load average is unsupported on this platform")

// LoadSnapshot is one immutable view of host CPU pressure as seen by admission.
type LoadSnapshot struct {
	Enabled        bool
	Supported      bool
	Pressured      bool
	Threshold      float64
	MaxWait        time.Duration
	LogicalCPUs    int
	Load1          float64
	NormalizedLoad float64
	SampledAt      time.Time
	Waiting        int
	Deferred       uint64
	WaitExpired    uint64
}

// LoadMonitorConfig configures host-load-aware admission.
type LoadMonitorConfig struct {
	// Threshold is the 1-minute load average divided by logical CPUs above which
	// background work waits. Zero disables the monitor entirely.
	Threshold float64
	// MaxWait bounds how long one background admission waits before proceeding.
	MaxWait time.Duration
	// Interval is the sampling cadence; nonpositive uses DefaultLoadSampleInterval.
	Interval time.Duration
	Logger   *slog.Logger

	// Sample and LogicalCPUs are test seams; nil/zero use the platform source.
	Sample      func() (float64, error)
	LogicalCPUs int
	Now         func() time.Time
}

// LoadMonitor samples host load and holds background admissions while the host is saturated.
type LoadMonitor struct {
	cfg    LoadMonitorConfig
	logger *slog.Logger

	mu       sync.Mutex
	snapshot LoadSnapshot
	relief   chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	started  bool
}

// NewLoadMonitor constructs a monitor; Start begins sampling.
func NewLoadMonitor(cfg LoadMonitorConfig) *LoadMonitor {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultLoadSampleInterval
	}
	if cfg.Sample == nil {
		cfg.Sample = sampleLoadAverage
	}
	if cfg.LogicalCPUs <= 0 {
		cfg.LogicalCPUs = runtime.NumCPU()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &LoadMonitor{
		cfg:    cfg,
		logger: logger,
		snapshot: LoadSnapshot{
			Enabled:     cfg.Threshold > 0,
			Threshold:   cfg.Threshold,
			MaxWait:     cfg.MaxWait,
			LogicalCPUs: cfg.LogicalCPUs,
		},
	}
}

// Start takes a baseline sample and starts the sampling goroutine. A zero
// threshold or an unsupported platform leaves the monitor permanently idle.
func (m *LoadMonitor) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("admission: load monitor context is required")
	}
	if m.cfg.Threshold < 0 || math.IsNaN(m.cfg.Threshold) || math.IsInf(m.cfg.Threshold, 0) {
		return fmt.Errorf("admission: load threshold must be a finite non-negative number: %v", m.cfg.Threshold)
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errors.New("admission: load monitor already started")
	}
	m.started = true
	m.mu.Unlock()
	if m.cfg.Threshold == 0 {
		return nil
	}
	if !m.sample() {
		return nil
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	m.mu.Lock()
	m.cancel = cancel
	m.done = done
	m.mu.Unlock()
	go m.run(runCtx, done)
	return nil
}

// Shutdown stops sampling, releases waiters, and joins the sampling goroutine.
func (m *LoadMonitor) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("admission: load monitor shutdown context is required")
	}
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.releaseLocked()
	m.snapshot.Pressured = false
	m.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("admission: join load monitor: %w", ctx.Err())
	}
}

// Snapshot returns the latest pressure observation.
func (m *LoadMonitor) Snapshot() LoadSnapshot {
	if m == nil {
		return LoadSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshot
}

// Wait holds the caller while the host is pressured, up to MaxWait. It returns
// nil once pressure clears or the bound elapses, and ctx's error if ctx ends first.
func (m *LoadMonitor) Wait(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	relief := m.relief
	if !m.snapshot.Pressured || relief == nil {
		m.mu.Unlock()
		return nil
	}
	m.snapshot.Waiting++
	m.snapshot.Deferred++
	m.mu.Unlock()

	timer := time.NewTimer(m.cfg.MaxWait)
	defer timer.Stop()
	expired := false
	var err error
	select {
	case <-relief:
	case <-timer.C:
		expired = true
	case <-ctx.Done():
		err = ctx.Err()
	}

	m.mu.Lock()
	m.snapshot.Waiting--
	if expired {
		m.snapshot.WaitExpired++
	}
	m.mu.Unlock()
	if expired {
		m.logger.Info("admission: background work proceeding after load wait bound",
			"max_wait", m.cfg.MaxWait)
	}
	return err
}

func (m *LoadMonitor) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.sample() {
				return
			}
		}
	}
}

// sample records one observation and reports whether sampling should continue.
func (m *LoadMonitor) sample() bool {
	load, err := m.cfg.Sample()
	if err != nil {
		m.mu.Lock()
		m.snapshot.Supported = false
		m.snapshot.Pressured = false
		m.releaseLocked()
		m.mu.Unlock()
		if errors.Is(err, ErrLoadAverageUnsupported) {
			m.logger.Debug("admission: host load sampling unsupported; load-aware admission disabled")
		} else {
			m.logger.Warn("admission: host load sampling failed; load-aware admission disabled", "error", err)
		}
		return false
	}
	normalized := load / float64(m.cfg.LogicalCPUs)
	pressured := normalized > m.cfg.Threshold

	m.mu.Lock()
	wasPressured := m.snapshot.Pressured
	m.snapshot.Supported = true
	m.snapshot.Load1 = load
	m.snapshot.NormalizedLoad = normalized
	m.snapshot.SampledAt = m.cfg.Now().UTC()
	m.snapshot.Pressured = pressured
	switch {
	case pressured && !wasPressured:
		m.relief = make(chan struct{})
	case !pressured && wasPressured:
		m.releaseLocked()
	}
	m.mu.Unlock()

	if pressured != wasPressured {
		m.logger.Info("admission: host CPU pressure changed",
			"pressured", pressured,
			"load1", load,
			"normalized_load", normalized,
			"threshold", m.cfg.Threshold,
			"logical_cpus", m.cfg.LogicalCPUs,
		)
	}
	return true
}

func (m *LoadMonitor) releaseLocked() {
	if m.relief != nil {
		close(m.relief)
		m.relief = nil
	}
}
