package daemon

import (
	"context"
	"errors"
	"log/slog"

	"github.com/compozy/compozy/internal/profile"
)

func (r *providerUsageRuntime) startProfileWorker(logger *slog.Logger) {
	ctx, cancel := context.WithCancel(r.ctx)
	r.profileCancel = cancel
	r.profileEvents = make(chan profile.Event, 1)
	r.profileDone = make(chan struct{})
	go func() {
		defer close(r.profileDone)
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-r.profileEvents:
				if ctx.Err() != nil {
					return
				}
				if err := r.syncProfiles(ctx); err != nil && ctx.Err() == nil && logger != nil {
					logger.Warn("profile lifecycle event recording failed", "event", event.Name,
						"operation_id", event.OperationID, "operation", "provider_usage", "error", err)
				}
			}
		}
	}()
}

func (r *providerUsageRuntime) requestProfileSync(event profile.Event) {
	select {
	case r.profileEvents <- event:
	default:
	}
}

func (r *providerUsageRuntime) Shutdown(ctx context.Context) error {
	var joinErr error
	if r.profileCancel != nil {
		r.profileCancel()
		select {
		case <-r.profileDone:
		case <-ctx.Done():
			joinErr = ctx.Err()
		}
	}
	return errors.Join(joinErr, r.Manager.Shutdown(ctx))
}
