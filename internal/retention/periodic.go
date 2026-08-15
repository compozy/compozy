// Package retention provides lifecycle management for periodic retention work.
package retention

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrPeriodicWorkerCallbackRequired reports a missing retention callback.
	ErrPeriodicWorkerCallbackRequired = errors.New("periodic retention worker callback is required")
	// ErrPeriodicWorkerContextRequired reports a nil start context.
	ErrPeriodicWorkerContextRequired = errors.New("periodic retention worker context is required")
	// ErrPeriodicWorkerInvalidInterval reports a non-positive sweep cadence.
	ErrPeriodicWorkerInvalidInterval = errors.New("periodic retention worker interval must be greater than zero")
	// ErrPeriodicWorkerAlreadyStarted reports an attempt to start an active worker.
	ErrPeriodicWorkerAlreadyStarted = errors.New("periodic retention worker is already started")
)

// PeriodicWorker runs one retention callback at a fixed interval and joins it on shutdown.
type PeriodicWorker struct {
	label    string
	sweep    func(context.Context) error
	interval time.Duration
	onError  func(error)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewPeriodicWorker builds a joined periodic worker for one retention callback.
func NewPeriodicWorker(
	label string,
	sweep func(context.Context) error,
	interval time.Duration,
	onError func(error),
) *PeriodicWorker {
	return &PeriodicWorker{
		label:    label,
		sweep:    sweep,
		interval: interval,
		onError:  onError,
	}
}

// Start begins periodic work until shutdown or parent cancellation.
func (w *PeriodicWorker) Start(ctx context.Context) error {
	if w == nil || w.sweep == nil {
		return fmt.Errorf("%s: %w", w.label, ErrPeriodicWorkerCallbackRequired)
	}
	if ctx == nil {
		return fmt.Errorf("%s context is required: %w", w.label, ErrPeriodicWorkerContextRequired)
	}
	if w.interval <= 0 {
		return fmt.Errorf("%s interval must be greater than zero: %s: %w", w.label, w.interval, ErrPeriodicWorkerInvalidInterval)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return fmt.Errorf("%s is already started: %w", w.label, ErrPeriodicWorkerAlreadyStarted)
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.run(runCtx, w.done)
	return nil
}

// Shutdown cancels and joins periodic work.
func (w *PeriodicWorker) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		return errors.New(w.label + " shutdown context is required")
	}
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		w.clearRun(done)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown %s: %w", w.label, ctx.Err())
	}
}

func (w *PeriodicWorker) clearRun(done <-chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done == done {
		w.cancel = nil
		w.done = nil
	}
}

func (w *PeriodicWorker) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.sweep(ctx); err != nil && w.onError != nil {
				w.onError(err)
			}
		}
	}
}
