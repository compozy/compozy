package tools

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/retention"
)

// ToolArtifactSweepInterval is the daemon-owned periodic retention cadence.
const ToolArtifactSweepInterval = time.Hour

// ToolArtifactSweeper applies age retention even when no artifact calls occur.
type ToolArtifactSweeper struct {
	store  ToolArtifactStore
	worker *retention.PeriodicWorker
}

// NewToolArtifactSweeper builds a joined periodic retention worker.
func NewToolArtifactSweeper(
	store ToolArtifactStore,
	interval time.Duration,
	onError func(error),
) *ToolArtifactSweeper {
	var sweep func(context.Context) error
	if store != nil {
		sweep = store.Sweep
	}
	return &ToolArtifactSweeper{
		store:  store,
		worker: retention.NewPeriodicWorker("tool artifact sweeper", sweep, interval, onError),
	}
}

// Start begins periodic sweeps until shutdown or parent cancellation.
func (w *ToolArtifactSweeper) Start(ctx context.Context) error {
	if w == nil || w.store == nil {
		return errors.New("tool artifact sweeper store is required")
	}
	return w.worker.Start(ctx)
}

// Shutdown cancels and joins the periodic sweep worker.
func (w *ToolArtifactSweeper) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if w.worker == nil {
		if ctx == nil {
			return errors.New("tool artifact sweeper shutdown context is required")
		}
		return nil
	}
	return w.worker.Shutdown(ctx)
}
