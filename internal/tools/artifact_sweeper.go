package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ToolArtifactSweepInterval is the daemon-owned periodic retention cadence.
const ToolArtifactSweepInterval = time.Hour

// ToolArtifactSweeper applies age retention even when no artifact calls occur.
type ToolArtifactSweeper struct {
	store    ToolArtifactStore
	interval time.Duration
	onError  func(error)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewToolArtifactSweeper builds a joined periodic retention worker.
func NewToolArtifactSweeper(
	store ToolArtifactStore,
	interval time.Duration,
	onError func(error),
) *ToolArtifactSweeper {
	return &ToolArtifactSweeper{store: store, interval: interval, onError: onError}
}

// Start begins periodic sweeps until shutdown or parent cancellation.
func (w *ToolArtifactSweeper) Start(ctx context.Context) error {
	if w == nil || w.store == nil {
		return errors.New("tool artifact sweeper store is required")
	}
	if ctx == nil {
		return errors.New("tool artifact sweeper context is required")
	}
	if w.interval <= 0 {
		return fmt.Errorf("tool artifact sweep interval must be greater than zero: %s", w.interval)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return errors.New("tool artifact sweeper is already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.run(runCtx, w.done)
	return nil
}

// Shutdown cancels and joins the periodic sweep worker.
func (w *ToolArtifactSweeper) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("tool artifact sweeper shutdown context is required")
	}
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown tool artifact sweeper: %w", ctx.Err())
	}
}

func (w *ToolArtifactSweeper) run(ctx context.Context, done chan<- struct{}) {
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
