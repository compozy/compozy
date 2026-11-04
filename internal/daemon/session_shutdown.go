package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/internal/session"
)

type shutdownStopper interface {
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

// daemonShutdownStopReader exposes the classification captured when shutdown was prepared.
type daemonShutdownStopReader interface {
	DaemonShutdownStop() (session.StopCause, string, bool)
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
	d.mu.Lock()
	cause, detail := session.DaemonShutdownStopCause(d.restartRequested)
	d.mu.Unlock()
	if recorded, ok := sessions.(daemonShutdownStopReader); ok {
		if recordedCause, recordedDetail, prepared := recorded.DaemonShutdownStop(); prepared {
			cause, detail = recordedCause, recordedDetail
		}
	}
	errs := make([]error, len(infos))
	var workers sync.WaitGroup
	for index, info := range infos {
		if info == nil {
			continue
		}
		workers.Go(func() {
			var err error
			if stopper, ok := sessions.(shutdownStopper); ok {
				err = stopper.StopWithCause(ctx, info.ID, cause, detail)
			} else {
				err = sessions.Stop(ctx, info.ID)
			}
			if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
				errs[index] = fmt.Errorf("daemon: stop session %q: %w", info.ID, err)
			}
		})
	}
	workers.Wait()
	if waiter, ok := sessions.(finalizationWaiter); ok {
		if err := waiter.WaitForFinalizations(ctx); err != nil {
			errs = append(errs, fmt.Errorf("daemon: wait for session finalizations: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (d *Daemon) shutdownSessionManager(ctx context.Context, sessions SessionManager) error {
	if sessions == nil {
		return nil
	}
	shutdowner, ok := sessions.(sessionManagerShutdowner)
	if !ok {
		return nil
	}
	if err := shutdowner.Shutdown(ctx); err != nil {
		return fmt.Errorf("daemon: shutdown session manager: %w", err)
	}
	return nil
}
