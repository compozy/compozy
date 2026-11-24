package cloud

import (
	"context"
	"errors"
	"time"
)

func (a *App) runAccountLifecycle(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, time.Minute)
		err := errors.Join(a.Accounts.SweepAccountDeletions(work), a.Accounts.SweepAccountEmail(work))
		cancel()
		if err != nil && ctx.Err() == nil {
			a.logger.WarnContext(ctx, "Cloud account lifecycle work will be retried", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
