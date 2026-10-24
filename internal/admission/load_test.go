package admission

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type fakeLoadSource struct {
	mu    sync.Mutex
	load  float64
	err   error
	calls int
}

func (s *fakeLoadSource) set(load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load = load
}

func (s *fakeLoadSource) sample() (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.load, s.err
}

func (s *fakeLoadSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func newTestLoadGate(
	t *testing.T,
	source *fakeLoadSource,
	threshold float64,
	maxWait time.Duration,
) (*Gate, *LoadMonitor) {
	t.Helper()
	monitor := NewLoadMonitor(LoadMonitorConfig{
		Threshold:   threshold,
		MaxWait:     maxWait,
		Interval:    time.Second,
		Sample:      source.sample,
		LogicalCPUs: 4,
	})
	if err := monitor.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := monitor.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup Shutdown() error = %v", err)
		}
	})
	gate := &Gate{}
	gate.SetLoadMonitor(monitor)
	return gate, monitor
}

func TestGateLoadAwareAdmission(t *testing.T) {
	t.Parallel()

	t.Run("Should hold background work under pressure until load drops", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 12} // 12 / 4 CPUs = 3.0 > 1.5
			gate, monitor := newTestLoadGate(t, source, 1.5, time.Hour)
			if !monitor.Snapshot().Pressured {
				t.Fatalf("Snapshot() = %#v, want pressured after baseline sample", monitor.Snapshot())
			}

			admitted := make(chan error, 1)
			go func() { admitted <- gate.Check(WithBackgroundWork(t.Context())) }()
			synctest.Wait()
			select {
			case err := <-admitted:
				t.Fatalf("background Check() returned %v while pressured, want it to wait", err)
			default:
			}
			if got := monitor.Snapshot().Waiting; got != 1 {
				t.Fatalf("Snapshot().Waiting = %d, want 1", got)
			}

			source.set(2) // 0.5 normalized
			time.Sleep(time.Second)
			synctest.Wait()
			if err := <-admitted; err != nil {
				t.Fatalf("background Check() error = %v, want nil after pressure clears", err)
			}
			snapshot := monitor.Snapshot()
			if snapshot.Pressured || snapshot.Deferred != 1 || snapshot.WaitExpired != 0 || snapshot.Waiting != 0 {
				t.Fatalf("Snapshot() = %#v, want cleared pressure with one deferred admission", snapshot)
			}
		})
	})

	t.Run("Should admit user work immediately under pressure", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 40}
			gate, monitor := newTestLoadGate(t, source, 1.5, time.Hour)

			if err := gate.Check(t.Context()); err != nil {
				t.Fatalf("user Check() error = %v, want immediate admission", err)
			}
			if got := monitor.Snapshot().Deferred; got != 0 {
				t.Fatalf("Snapshot().Deferred = %d, want 0 for user work", got)
			}
		})
	})

	t.Run("Should proceed after the max wait bound elapses", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 40}
			gate, monitor := newTestLoadGate(t, source, 1.5, 30*time.Second)

			start := time.Now()
			if err := gate.Check(WithBackgroundWork(t.Context())); err != nil {
				t.Fatalf("background Check() error = %v, want nil after max wait", err)
			}
			if waited := time.Since(start); waited != 30*time.Second {
				t.Fatalf("background Check() waited %s, want 30s", waited)
			}
			if got := monitor.Snapshot().WaitExpired; got != 1 {
				t.Fatalf("Snapshot().WaitExpired = %d, want 1", got)
			}
		})
	})

	t.Run("Should return the context error when the caller gives up", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 40}
			gate, _ := newTestLoadGate(t, source, 1.5, time.Hour)

			ctx, cancel := context.WithTimeout(WithBackgroundWork(t.Context()), 5*time.Second)
			defer cancel()
			if err := gate.Check(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("background Check() error = %v, want context.DeadlineExceeded", err)
			}
		})
	})

	t.Run("Should reject background work that waited through a drain", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 40}
			gate, _ := newTestLoadGate(t, source, 1.5, 10*time.Second)

			admitted := make(chan error, 1)
			go func() { admitted <- gate.Check(WithBackgroundWork(t.Context())) }()
			synctest.Wait()
			gate.Drain()
			if err := <-admitted; !errors.Is(err, ErrDraining) {
				t.Fatalf("background Check() error = %v, want ErrDraining", err)
			}
		})
	})

	t.Run("Should never sample or wait when the threshold is zero", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{load: 400}
			gate, monitor := newTestLoadGate(t, source, 0, time.Hour)

			time.Sleep(time.Minute)
			if err := gate.Check(WithBackgroundWork(t.Context())); err != nil {
				t.Fatalf("background Check() error = %v, want immediate admission", err)
			}
			if got := source.callCount(); got != 0 {
				t.Fatalf("sampler calls = %d, want 0 when disabled", got)
			}
			if snapshot := monitor.Snapshot(); snapshot.Enabled || snapshot.Pressured {
				t.Fatalf("Snapshot() = %#v, want disabled and unpressured", snapshot)
			}
		})
	})

	t.Run("Should disable admission when the platform has no load average", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			source := &fakeLoadSource{err: ErrLoadAverageUnsupported}
			gate, monitor := newTestLoadGate(t, source, 1.5, time.Hour)

			time.Sleep(time.Minute)
			if err := gate.Check(WithBackgroundWork(t.Context())); err != nil {
				t.Fatalf("background Check() error = %v, want immediate admission", err)
			}
			if got := source.callCount(); got != 1 {
				t.Fatalf("sampler calls = %d, want only the baseline probe", got)
			}
			if snapshot := monitor.Snapshot(); !snapshot.Enabled || snapshot.Supported {
				t.Fatalf("Snapshot() = %#v, want enabled but unsupported", snapshot)
			}
		})
	})
}

// Invariant: boot recovery alone can admit work before readiness; its capability cannot bypass shutdown.
// Owner: admission gate; canonical gate suite.
func TestGateBootRecovery(t *testing.T) {
	t.Parallel()
	t.Run("Should fence external work until recovery and fence every caller during shutdown", func(t *testing.T) {
		t.Parallel()
		gate := &Gate{}
		recovery := gate.BeginBoot(t.Context())
		if err := gate.Check(t.Context()); !errors.Is(err, ErrDraining) {
			t.Fatalf("external admission during boot = %v", err)
		}
		if err := gate.Check(recovery); err != nil {
			t.Fatalf("recovery admission = %v", err)
		}
		gate.Undrain()
		if err := gate.Check(t.Context()); !errors.Is(err, ErrDraining) {
			t.Fatalf("public undrain bypassed boot recovery: %v", err)
		}
		gate.FinishBoot()
		if err := gate.Check(t.Context()); err != nil {
			t.Fatalf("ready admission = %v", err)
		}
		gate.Drain()
		if err := gate.Check(recovery); !errors.Is(err, ErrDraining) {
			t.Fatalf("recovery capability bypassed shutdown: %v", err)
		}
	})
}

func TestSampleLoadAverage(t *testing.T) {
	t.Parallel()

	t.Run("Should report a non-negative host load average or unsupported", func(t *testing.T) {
		t.Parallel()
		load, err := sampleLoadAverage()
		if errors.Is(err, ErrLoadAverageUnsupported) {
			return
		}
		if err != nil {
			t.Fatalf("sampleLoadAverage() error = %v", err)
		}
		if load < 0 {
			t.Fatalf("sampleLoadAverage() = %v, want non-negative", load)
		}
	})
}
