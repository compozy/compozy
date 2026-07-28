package daemon

import (
	"context"
	"errors"
	"fmt"
)

type goalSessionCleanupReconciler interface {
	ReconcileGoalSessionCleanup(context.Context) error
}

func (d *Daemon) bootGoalSessionOutboxRelay(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil || state.registry == nil {
		return nil
	}
	store, ok := state.registry.(goalSessionOutboxStore)
	if !ok {
		return nil
	}
	appender, ok := state.sessions.(goalSessionEventAppender)
	if !ok {
		return errors.New("daemon: session manager does not implement Goal outbox event append")
	}
	cleanupStore, ok := state.registry.(goalSessionCleanupStore)
	if !ok {
		return errors.New("daemon: registry does not implement Goal session cleanup")
	}
	reconciler, ok := state.registry.(goalSessionCleanupReconciler)
	if !ok {
		return errors.New("daemon: registry does not implement Goal session cleanup reconciliation")
	}
	if err := reconciler.ReconcileGoalSessionCleanup(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile Goal session cleanup: %w", err)
	}
	stopper, ok := state.sessions.(goalSessionCleanupStopper)
	if !ok {
		return errors.New("daemon: session manager does not implement Goal session cleanup stop")
	}
	relayCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	relay := &goalSessionOutboxRelay{
		store: store, appender: appender, cleanupStore: cleanupStore, stopper: stopper,
		logger: state.logger, now: d.now,
		batchSize:    state.cfg.Goals.OutboxBatchSize,
		pollInterval: state.cfg.Goals.OutboxPollInterval,
	}
	go func() {
		defer close(done)
		relay.Run(relayCtx)
	}()
	state.goalOutboxCancel = cancel
	state.goalOutboxDone = done
	cleanup.add(func(cleanupCtx context.Context) error {
		return stopGoalSessionOutboxRelay(cleanupCtx, cancel, done)
	})
	return nil
}

func stopGoalSessionOutboxRelay(
	ctx context.Context,
	cancel context.CancelFunc,
	done <-chan struct{},
) error {
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for Goal session outbox relay: %w", ctx.Err())
	}
}
