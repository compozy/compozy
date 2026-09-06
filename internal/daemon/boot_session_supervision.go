package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/retention"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/toolruntime"
)

type heartbeatWakeSweeper interface {
	SweepHeartbeatWakeEvents(context.Context, time.Time, int) (int64, error)
}

func (d *Daemon) bootSessionSupervision(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	if normalizer, ok := state.registry.(interface {
		BackfillLoopWaitAdmissionDeadlines(context.Context, int) (int, error)
	}); ok {
		for {
			count, err := normalizer.BackfillLoopWaitAdmissionDeadlines(ctx, 100)
			if err != nil {
				return fmt.Errorf("daemon: normalize persisted wait deadlines: %w", err)
			}
			if count < 100 {
				break
			}
		}
	}
	sources := sessionWorkSources{state: state, manager: manager, now: d.now}
	manager.SetWorkSignalSources(
		session.WorkSignalSource{Kind: session.WorkSignalToolRunning, Source: session.SignalSourceFunc(sources.tools)},
		session.WorkSignalSource{
			Kind:   session.WorkSignalActiveChild,
			Source: session.SignalSourceFunc(sources.children),
		},
		session.WorkSignalSource{Kind: session.WorkSignalTaskLease, Source: session.SignalSourceFunc(sources.leases)},
		session.WorkSignalSource{Kind: session.WorkSignalLoopRun, Source: session.SignalSourceFunc(sources.loops)},
		session.WorkSignalSource{
			Kind:   session.WorkSignalScheduledWait,
			Source: session.SignalSourceFunc(sources.waits),
		},
	)
	interval := toolruntime.SweepInterval
	for _, bound := range []time.Duration{
		state.cfg.Session.Supervision.ActivityHeartbeatInterval,
		state.cfg.Session.Supervision.QuietAfter,
		state.cfg.Session.Supervision.StopGrace,
	} {
		if bound > 0 && bound < interval {
			interval = bound
		}
	}
	worker := retention.NewPeriodicWorker("session supervision", func(ctx context.Context) error {
		var registryErr error
		if state.processRegistry != nil {
			registryErr = state.processRegistry.Reconcile(ctx)
		}
		supervisionErr := manager.Supervise(ctx, d.now())
		var sweepErr error
		if sweeper, ok := state.registry.(heartbeatWakeSweeper); ok {
			_, sweepErr = sweeper.SweepHeartbeatWakeEvents(ctx, d.now(), 1000)
		}
		return errors.Join(registryErr, supervisionErr, sweepErr)
	}, interval, func(err error) { state.logger.Error("session supervision failed", "error", err) })
	if err := worker.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start session supervision: %w", err)
	}
	cleanup.add(worker.Shutdown)
	state.runtimeWorkers.supervision = worker
	return nil
}
