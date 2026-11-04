package globaldb

import (
	"context"
	"log/slog"
	"time"
)

type eventSummaryRetry struct {
	base   *repoBase
	cancel context.CancelFunc
	done   chan struct{}
}

func startEventSummaryRetry(base *repoBase) *eventSummaryRetry {
	ctx, cancel := context.WithCancel(context.Background())
	retry := &eventSummaryRetry{base: base, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(retry.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := base.flushEventSummaryOutbox(ctx); err != nil && ctx.Err() == nil {
					slog.WarnContext(ctx, "store: retry telemetry delivery", "error", err)
				}
			}
		}
	}()
	return retry
}

func (r *eventSummaryRetry) close(ctx context.Context) error {
	r.cancel()
	// Cancellation releases an in-flight writer before the owning pools close.
	<-r.done
	for {
		count, err := r.base.flushEventSummaryBatch(ctx)
		if err != nil || count < eventSummaryOutboxBatchSize {
			return err
		}
	}
}
