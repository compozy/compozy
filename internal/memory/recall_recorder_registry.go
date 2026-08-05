package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
	memoryrecall "github.com/compozy/compozy/internal/memory/recall"
)

type recallRecorderRegistry struct {
	mu        sync.Mutex
	recorders map[string]*recallRecorderEntry
	closing   bool
	closeOp   *recallRecorderCloseOperation
}

type recallRecorderEntry struct {
	recorder *memoryrecall.SignalRecorder
	leases   int
	stopping bool
}

type recallRecorderLease struct {
	registry *recallRecorderRegistry
	key      string
	entry    *recallRecorderEntry
	released bool
}

type recallRecorderCloseOperation struct {
	done chan struct{}
}

type recallRecorderFactory func(
	shouldStop func() bool,
	onStopped func(),
) (*memoryrecall.SignalRecorder, error)

func newRecallRecorderRegistry() *recallRecorderRegistry {
	return &recallRecorderRegistry{recorders: make(map[string]*recallRecorderEntry)}
}

func (r *recallRecorderRegistry) acquire(
	ctx context.Context,
	key string,
	factory recallRecorderFactory,
) (*recallRecorderLease, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	if ctx == nil {
		return nil, false, errors.New("memory: recall recorder acquire context is required")
	}
	if factory == nil {
		return nil, false, errors.New("memory: recall recorder factory is required")
	}

	for {
		r.mu.Lock()
		if r.closing {
			r.mu.Unlock()
			return nil, true, nil
		}
		entry := r.recorders[key]
		if entry == nil {
			entry = &recallRecorderEntry{leases: 1}
			recorder, err := factory(
				func() bool { return r.beginIdleStop(key, entry) },
				func() { r.stopped(key, entry) },
			)
			if err != nil {
				r.mu.Unlock()
				return nil, false, err
			}
			if recorder == nil {
				r.mu.Unlock()
				return nil, false, errors.New("memory: recall recorder factory returned nil recorder")
			}
			entry.recorder = recorder
			r.recorders[key] = entry
			r.mu.Unlock()
			return &recallRecorderLease{registry: r, key: key, entry: entry}, false, nil
		}
		if !entry.stopping {
			entry.leases++
			r.mu.Unlock()
			return &recallRecorderLease{registry: r, key: key, entry: entry}, false, nil
		}
		done := entry.recorder.Done()
		r.mu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			return nil, true, nil
		}
	}
}

func (r *recallRecorderRegistry) stats(key string) memoryrecall.SignalRecorderStats {
	if r == nil {
		return memoryrecall.SignalRecorderStats{}
	}
	r.mu.Lock()
	entry := r.recorders[key]
	r.mu.Unlock()
	if entry == nil {
		return memoryrecall.SignalRecorderStats{}
	}
	return entry.recorder.Stats()
}

func (r *recallRecorderRegistry) close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("memory: recall recorder close context is required")
	}

	r.mu.Lock()
	op := r.closeOp
	toStop := make([]*memoryrecall.SignalRecorder, 0, len(r.recorders))
	if op == nil {
		op = &recallRecorderCloseOperation{done: make(chan struct{})}
		r.closeOp = op
		r.closing = true
		for _, entry := range r.recorders {
			if entry.leases == 0 && !entry.stopping {
				entry.stopping = true
				toStop = append(toStop, entry.recorder)
			}
		}
		r.completeCloseLocked()
	}
	r.mu.Unlock()

	for _, recorder := range toStop {
		recorder.RequestStop()
	}

	select {
	case <-op.done:
		return nil
	case <-ctx.Done():
		select {
		case <-op.done:
			return nil
		default:
		}
		r.cancelRecorders()
		return fmtCloseRecallRecorders(ctx.Err())
	}
}

func (r *recallRecorderRegistry) cancelRecorders() {
	if r == nil {
		return
	}
	r.mu.Lock()
	recorders := make([]*memoryrecall.SignalRecorder, 0, len(r.recorders))
	for _, entry := range r.recorders {
		if entry != nil && entry.recorder != nil {
			recorders = append(recorders, entry.recorder)
		}
	}
	r.mu.Unlock()
	for _, recorder := range recorders {
		recorder.Cancel()
	}
}

func (l *recallRecorderLease) Submit(
	ctx context.Context,
	query memcontract.Query,
	signals []memoryrecall.Signal,
) memoryrecall.SignalRecorderSubmitResult {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return memoryrecall.SignalRecorderSubmitResult{}
	}
	return l.entry.recorder.Submit(ctx, query, signals)
}

func (l *recallRecorderLease) Release() {
	if l == nil || l.registry == nil || l.entry == nil || l.released {
		return
	}
	l.released = true

	registry := l.registry
	registry.mu.Lock()
	entry := registry.recorders[l.key]
	shouldStop := entry == l.entry && entry.leases > 0
	if shouldStop {
		entry.leases--
		shouldStop = registry.closing && entry.leases == 0 && !entry.stopping
		if shouldStop {
			entry.stopping = true
		}
	}
	registry.mu.Unlock()

	if shouldStop {
		entry.recorder.RequestStop()
		return
	}
	l.entry.recorder.NotifyIdle()
}

func (r *recallRecorderRegistry) beginIdleStop(key string, entry *recallRecorderEntry) bool {
	if r == nil || entry == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing || r.recorders[key] != entry || entry.leases != 0 || entry.stopping {
		return false
	}
	entry.stopping = true
	return true
}

func (r *recallRecorderRegistry) stopped(key string, entry *recallRecorderEntry) {
	if r == nil || entry == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recorders[key] == entry {
		delete(r.recorders, key)
	}
	r.completeCloseLocked()
}

func (r *recallRecorderRegistry) completeCloseLocked() {
	if r.closeOp != nil && len(r.recorders) == 0 {
		select {
		case <-r.closeOp.done:
		default:
			close(r.closeOp.done)
		}
	}
}

func fmtCloseRecallRecorders(err error) error {
	return fmt.Errorf("memory: close recall signal recorders: %w", err)
}
