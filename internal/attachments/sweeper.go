package attachments

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/retention"
)

// SweepInterval is the daemon-owned periodic retention cadence.
const SweepInterval = time.Hour

// ErrAttachmentSweeperStoreRequired reports a periodic worker without an attachment store.
var ErrAttachmentSweeperStoreRequired = errors.New("attachments: sweeper store is required")

// Sweeper applies age retention even when no attachment calls occur.
type Sweeper struct {
	store  AttachmentSweeper
	worker *retention.PeriodicWorker
}

// NewSweeper builds a joined periodic retention worker.
func NewSweeper(store AttachmentSweeper, interval time.Duration, onError func(error)) *Sweeper {
	var sweep func(context.Context) error
	if store != nil {
		sweep = store.Sweep
	}
	return &Sweeper{
		store:  store,
		worker: retention.NewPeriodicWorker("attachment sweeper", sweep, interval, onError),
	}
}

// Start begins periodic sweeps until shutdown or parent cancellation.
func (w *Sweeper) Start(ctx context.Context) error {
	if w == nil || w.store == nil {
		return ErrAttachmentSweeperStoreRequired
	}
	return w.worker.Start(ctx)
}

// Shutdown cancels and joins the periodic sweep worker.
func (w *Sweeper) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	return w.worker.Shutdown(ctx)
}
