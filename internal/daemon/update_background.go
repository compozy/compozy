package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

const defaultUpdateRecoveryInterval = time.Minute

type backgroundUpdateManager interface {
	settingsUpdateManager
	CheckAll(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error)
}

type backgroundUpdateRuntime struct {
	manager          backgroundUpdateManager
	logger           *slog.Logger
	checkEnabled     bool
	checkInterval    time.Duration
	recoveryInterval time.Duration
	recoverOperation func(context.Context) error

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newBackgroundUpdateRuntime(
	manager backgroundUpdateManager,
	config compozyconfig.AppConfig,
	logger *slog.Logger,
	recoverOperation func(context.Context) error,
) (*backgroundUpdateRuntime, error) {
	if manager == nil || logger == nil || recoverOperation == nil {
		return nil, errors.New("daemon: background update dependencies are required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	recoveryInterval := min(config.UpdateCheckInterval, defaultUpdateRecoveryInterval)
	return &backgroundUpdateRuntime{
		manager: manager, logger: logger, checkEnabled: config.UpdateCheck,
		checkInterval: config.UpdateCheckInterval, recoveryInterval: recoveryInterval,
		recoverOperation: recoverOperation,
	}, nil
}

func (r *backgroundUpdateRuntime) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: background update context is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return errors.New("daemon: background update runtime already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})
	go r.run(runCtx, r.done)
	return nil
}

func (r *backgroundUpdateRuntime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		r.mu.Lock()
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *backgroundUpdateRuntime) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	recoveryTicker := time.NewTicker(r.recoveryInterval)
	defer recoveryTicker.Stop()
	var checkTicker *time.Ticker
	var checkChannel <-chan time.Time
	if r.checkEnabled {
		checkTicker = time.NewTicker(r.checkInterval)
		checkChannel = checkTicker.C
		defer checkTicker.Stop()
		r.check(ctx)
	}
	r.recover(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-checkChannel:
			r.check(ctx)
		case <-recoveryTicker.C:
			r.recover(ctx)
		}
	}
}

func (r *backgroundUpdateRuntime) check(ctx context.Context) {
	if _, _, err := r.manager.CheckAll(ctx, compozyupdate.CheckOptions{
		ForceRefresh: true, AllowCachedOnFailure: true,
	}); err != nil {
		r.logger.Warn("daemon: background update check failed", "error", err)
	}
}

func (r *backgroundUpdateRuntime) recover(ctx context.Context) {
	if err := r.recoverOperation(ctx); err != nil {
		r.logger.Warn("daemon: update operation recovery failed", "error", err)
	}
}

func (d *Daemon) startBackgroundUpdates(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	manager, ok := state.updateManager.(backgroundUpdateManager)
	if !ok {
		return errors.New("daemon: settings update manager cannot run background checks")
	}
	runtime, err := newBackgroundUpdateRuntime(
		manager,
		state.cfg.App,
		state.logger,
		func(recoveryCtx context.Context) error {
			return recoverDaemonUpdateOperation(recoveryCtx, d, manager)
		},
	)
	if err != nil {
		return fmt.Errorf("daemon: create background update runtime: %w", err)
	}
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	cleanup.add(runtime.Shutdown)
	state.backgroundUpdates = runtime
	return nil
}

func recoverDaemonUpdateOperation(
	ctx context.Context,
	d *Daemon,
	manager settingsUpdateManager,
) error {
	operation, err := manager.OperationStore().Read(ctx)
	if err != nil || operation == nil {
		return err
	}
	if operation.Waiting == compozyupdate.WaitingForApp || !daemonCanRecoverUpdate(operation) {
		return nil
	}
	if manager.OperationStore().HolderLive(operation.Holder) {
		return nil
	}
	expired := !operation.Deadline.IsZero() && !operation.Deadline.After(time.Now().UTC())
	holder, err := newDaemonUpdateHolder(d, compozyupdate.ActorDaemon)
	if err != nil {
		return err
	}
	recovered, err := manager.OperationStore().Transition(
		ctx,
		operation.ID,
		"",
		operation.Revision,
		compozyupdate.Transition{
			Kind: compozyupdate.TransitionRecover, Actor: compozyupdate.ActorDaemon,
			Target: operation.ActiveTarget, Holder: &holder, Percent: operation.Percent,
			Outcome: "recovered",
		},
	)
	if err != nil {
		return err
	}
	detachedCtx := context.WithoutCancel(ctx)
	if expired && updateCanFailWithoutRollback(recovered) {
		activeTarget := recovered.ActiveTarget
		_, transitionErr := manager.OperationStore().Transition(
			detachedCtx,
			recovered.ID,
			recovered.Holder.ExecutorGeneration,
			recovered.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
				Target: activeTarget, Phase: compozyupdate.PhaseFailed,
				Percent: -1, LastError: "update operation deadline expired",
				Outcome:              string(compozyupdate.StatusFailed),
				IncrementAppFailures: activeTarget == compozyupdate.TargetApp,
			},
		)
		return transitionErr
	}
	if err := spawnDetachedUpdateCoordinator(detachedCtx, d, recovered); err != nil {
		_, transitionErr := manager.OperationStore().Transition(
			detachedCtx,
			recovered.ID,
			recovered.Holder.ExecutorGeneration,
			recovered.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
				Target: compozyupdate.TargetRuntime, Phase: compozyupdate.PhaseFailed,
				Percent: -1, LastError: err.Error(), Outcome: string(compozyupdate.StatusFailed),
			},
		)
		return errors.Join(err, transitionErr)
	}
	return nil
}

func daemonCanRecoverUpdate(operation *compozyupdate.Operation) bool {
	if operation == nil {
		return false
	}
	switch operation.ActiveTarget {
	case compozyupdate.TargetRuntime:
		return operation.Runtime != nil
	case compozyupdate.TargetApp:
		return operation.App != nil && operation.App.Phase == compozyupdate.PhasePending
	}
	return false
}

func updateCanFailWithoutRollback(operation *compozyupdate.Operation) bool {
	if operation == nil {
		return false
	}
	if operation.ActiveTarget == compozyupdate.TargetApp {
		return operation.App != nil && operation.App.Phase == compozyupdate.PhasePending
	}
	return operation.Runtime != nil && slices.Contains(
		[]compozyupdate.OperationPhase{
			compozyupdate.PhasePending,
			compozyupdate.PhaseDownloading,
			compozyupdate.PhaseVerifying,
		},
		operation.Runtime.Phase,
	)
}
