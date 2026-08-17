package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Run executes or recovers one operation under its fenced executor generation.
func (c *Coordinator) Run(ctx context.Context, operationID string, executorGeneration string) error {
	if ctx == nil {
		return errors.New("update: coordinator context is required")
	}
	operation, err := c.store.Read(ctx)
	if err != nil {
		return err
	}
	if operation == nil || operation.ID != strings.TrimSpace(operationID) {
		return ErrOperationNotFound
	}
	if operation.Holder == nil || operation.Holder.ExecutorGeneration != strings.TrimSpace(executorGeneration) {
		return ErrExecutorFenced
	}
	if !c.store.HolderLive(operation.Holder) {
		return ErrExecutorFenced
	}

	state := &coordinatorState{store: c.store, operation: operation, generation: executorGeneration}
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	done := make(chan struct{})
	heartbeatExited := make(chan struct{})
	heartbeatErrors := make(chan error, 1)
	go func() {
		defer close(heartbeatExited)
		c.renewLease(heartbeatCtx, state, done, heartbeatErrors)
	}()
	runErr := c.runOperation(ctx, state)
	close(done)
	cancelHeartbeat()
	<-heartbeatExited
	select {
	case heartbeatErr := <-heartbeatErrors:
		runErr = errors.Join(runErr, fmt.Errorf("update: renew coordinator lease: %w", heartbeatErr))
	default:
	}
	return runErr
}

func (c *Coordinator) runOperation(ctx context.Context, state *coordinatorState) error {
	operation := state.snapshot()
	if operation.Runtime != nil {
		if err := c.runRuntime(ctx, state); err != nil {
			return err
		}
	}
	operation = state.snapshot()
	if operation.App != nil && operation.App.Phase == PhasePending {
		staged, err := state.transition(ctx, Transition{
			Kind: TransitionPhase, Actor: operation.Holder.Surface, Target: TargetApp,
			Phase: PhaseStaged, Percent: 100, Outcome: operationOutcomeStaged,
		})
		if err != nil {
			return err
		}
		_, err = state.transition(ctx, Transition{
			Kind: TransitionWaitForApp, Actor: staged.Holder.Surface, Target: TargetApp,
			Percent: -1, Outcome: operationOutcomeWaiting,
		})
		return err
	}
	return nil
}

func (c *Coordinator) runRuntime(ctx context.Context, state *coordinatorState) error {
	operation := state.snapshot()
	switch operation.Runtime.Phase {
	case PhasePending, PhaseDownloading, PhaseVerifying:
		return c.applyRuntime(ctx, state)
	case PhaseSwapping:
		return c.recoverSwap(ctx, state)
	case PhaseRestarting:
		return c.restartAndVerify(ctx, state, c.appliedFromOperation(operation))
	case PhaseHealthChecking:
		return c.healthAndFinalize(ctx, state, c.appliedFromOperation(operation))
	case PhaseFinalized:
		return nil
	case PhaseRolledBack, PhaseFailed:
		return nil
	default:
		return fmt.Errorf("update: unsupported coordinator runtime phase %q", operation.Runtime.Phase)
	}
}

func (c *Coordinator) recoverSwap(ctx context.Context, state *coordinatorState) error {
	operation := state.snapshot()
	backupPath := strings.TrimSpace(operation.Runtime.BackupPath)
	if backupPath == "" {
		return c.applyRuntime(ctx, state)
	}
	if _, err := os.Stat(backupPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.applyRuntime(ctx, state)
		}
		return fmt.Errorf("update: inspect recovery backup: %w", err)
	}
	return c.rollback(
		ctx,
		state,
		c.appliedFromOperation(operation),
		errors.New("update: recovered interrupted runtime swap"),
	)
}

func (c *Coordinator) appliedFromOperation(operation *Operation) AppliedBinary {
	return AppliedBinary{
		TargetPath:    c.binaryManager.RuntimeTargetPath(),
		BackupPath:    operation.Runtime.BackupPath,
		Version:       operation.Runtime.ToVersion,
		InstallMethod: operation.Runtime.InstallMethod,
	}
}
