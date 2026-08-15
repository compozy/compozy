package attachments

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SweepInterval is the daemon-owned periodic retention cadence.
const SweepInterval = time.Hour

// Sweeper applies age retention even when no attachment calls occur.
type Sweeper struct {
	store    Store
	interval time.Duration
	onError  func(error)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSweeper builds a joined periodic retention worker.
func NewSweeper(
	store Store,
	interval time.Duration,
	onError func(error),
) *Sweeper {
	return &Sweeper{store: store, interval: interval, onError: onError}
}

// Start begins periodic sweeps until shutdown or parent cancellation.
func (w *Sweeper) Start(ctx context.Context) error {
	if w == nil || w.store == nil {
		return errors.New("attachment sweeper store is required")
	}
	if ctx == nil {
		return errors.New("attachment sweeper context is required")
	}
	if w.interval <= 0 {
		return fmt.Errorf("attachment sweep interval must be greater than zero: %s", w.interval)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return errors.New("attachment sweeper is already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.run(runCtx, w.done)
	return nil
}

// Shutdown cancels and joins the periodic sweep worker.
func (w *Sweeper) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("attachment sweeper shutdown context is required")
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
		return fmt.Errorf("shutdown attachment sweeper: %w", ctx.Err())
	}
}

func (w *Sweeper) clearRun(done <-chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done == done {
		w.cancel = nil
		w.done = nil
	}
}

func (w *Sweeper) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.store.Sweep(ctx); err != nil && w.onError != nil {
				w.onError(err)
			}
		}
	}
}
