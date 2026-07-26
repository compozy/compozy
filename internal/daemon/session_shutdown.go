package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/session"
)

type shutdownStopper interface {
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type finalizationWaiter interface {
	WaitForFinalizations(ctx context.Context) error
}

type sessionManagerShutdowner interface {
	Shutdown(context.Context) error
}

func (d *Daemon) stopSessions(ctx context.Context, sessions SessionManager) error {
	if sessions == nil {
		return nil
	}

	infos := sessions.List()
	var errs []error
	for _, info := range infos {
		if info == nil {
			continue
		}
		var err error
		if stopper, ok := sessions.(shutdownStopper); ok {
			err = stopper.StopWithCause(ctx, info.ID, session.CauseShutdown, "daemon shutdown")
		} else {
			err = sessions.Stop(ctx, info.ID)
		}
		if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
			errs = append(errs, fmt.Errorf("daemon: stop session %q: %w", info.ID, err))
		}
	}
	if waiter, ok := sessions.(finalizationWaiter); ok {
		if err := waiter.WaitForFinalizations(ctx); err != nil {
			errs = append(errs, fmt.Errorf("daemon: wait for session finalizations: %w", err))
		}
	}
	if shutdowner, ok := sessions.(sessionManagerShutdowner); ok {
		if err := shutdowner.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("daemon: shutdown session manager: %w", err))
		}
	}

	return errors.Join(errs...)
}
