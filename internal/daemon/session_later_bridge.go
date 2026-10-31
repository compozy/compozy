package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/compozy/compozy/internal/retention"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/toolruntime"
)

// sessionLaterProcessReleaser terminates the processes and terminals a parked
// session started, using the daemon-owned process attribution.
type sessionLaterProcessReleaser struct {
	processes *sessionprocess.Service
	supported atomic.Bool
}

type laterProcessReleaserSetter interface {
	SetLaterProcessReleaser(session.LaterProcessReleaser)
}

type laterReturnSweeper interface {
	ReturnDueLaterSessions(ctx context.Context) (int, error)
}

func (r *sessionLaterProcessReleaser) CaptureSessionProcesses(ctx context.Context) error {
	snapshot, err := r.processes.Refresh(ctx)
	if err != nil {
		return err
	}
	r.supported.Store(snapshot.Supported)
	return nil
}

func (r *sessionLaterProcessReleaser) TerminateSessionProcesses(ctx context.Context, info *session.Info) error {
	if info == nil || !r.supported.Load() {
		return nil
	}
	results, err := r.processes.Terminate(ctx, info.ID, nil, true, func(owner sessionprocess.Owner) bool {
		return owner.ProfileID == info.ProfileID && owner.WorkspaceID == info.WorkspaceID
	})
	if err != nil {
		return err
	}
	var failures error
	for _, result := range results {
		if !sessionProcessTerminationSettled(result.Outcome) {
			failures = errors.Join(failures, fmt.Errorf("process %d termination is %s", result.PID, result.Outcome))
		}
	}
	return failures
}

// bootSessionLaterReturns sweeps due Later return times. The first sweep runs
// at boot, so return times that passed while the daemon was down fire at once.
func (d *Daemon) bootSessionLaterReturns(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	manager, ok := state.sessions.(laterReturnSweeper)
	if !ok {
		return nil
	}
	worker := retention.NewPeriodicWorker("session later returns", func(ctx context.Context) error {
		_, err := manager.ReturnDueLaterSessions(ctx)
		return err
	}, toolruntime.SweepInterval, func(err error) { state.logger.Error("session later returns failed", "error", err) })
	if err := worker.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start session later returns: %w", err)
	}
	cleanup.add(worker.Shutdown)
	return nil
}
