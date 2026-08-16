package update

import (
	"context"
	"errors"
)

func (c *Coordinator) applyRuntime(ctx context.Context, state *coordinatorState) (returnErr error) {
	operation := state.snapshot()
	release, err := c.manager.ResolveReleaseByTag(ctx, operation.Runtime.ReleaseTag)
	if err != nil {
		return c.failRuntime(ctx, state, err)
	}
	var mutationLock MutationLock
	applied, err := c.manager.ApplyReleaseObserved(ctx, release, func(stepCtx context.Context, step ApplyStep) error {
		current := state.snapshot()
		if step.Phase == PhaseSwapping && len(current.Targets) == 1 {
			installedAppVersion, versionErr := c.runtime.InstalledAppVersion(stepCtx)
			if versionErr != nil {
				return versionErr
			}
			if step.Compatibility == nil {
				return errors.New("update: verified compatibility is required before runtime swap")
			}
			if compatibilityErr := CheckRuntimeCompatibility(*step.Compatibility, installedAppVersion); compatibilityErr != nil {
				return c.failRuntime(stepCtx, state, compatibilityErr)
			}
		}
		updated, transitionErr := state.transition(stepCtx, Transition{
			Kind: TransitionPhase, Actor: current.Holder.Surface, Target: TargetRuntime,
			Phase: step.Phase, Percent: step.Percent, BackupPath: step.BackupPath, Outcome: "started",
		})
		if transitionErr != nil {
			return transitionErr
		}
		if step.Phase != PhaseSwapping {
			return nil
		}
		mutationLock, transitionErr = c.runtime.AcquireMutationLock(stepCtx)
		if transitionErr != nil {
			return transitionErr
		}
		if fenceErr := state.fence(stepCtx); fenceErr != nil {
			releaseErr := mutationLock.Release()
			mutationLock = nil
			return errors.Join(fenceErr, releaseErr)
		}
		if updated.Runtime.BackupPath == "" {
			return errors.New("update: swap transition did not persist its backup path")
		}
		return nil
	})
	if mutationLock != nil {
		err = errors.Join(err, mutationLock.Release())
	}
	if err != nil {
		return c.failRuntime(ctx, state, err)
	}
	return c.restartAndVerify(ctx, state, applied)
}

func (c *Coordinator) restartAndVerify(
	ctx context.Context,
	state *coordinatorState,
	applied AppliedBinary,
) error {
	operation := state.snapshot()
	updated := operation
	if operation.Runtime.Phase != PhaseRestarting {
		var err error
		updated, err = state.transition(ctx, Transition{
			Kind: TransitionPhase, Actor: operation.Holder.Surface, Target: TargetRuntime,
			Phase: PhaseRestarting, Percent: -1, Outcome: "started",
		})
		if err != nil {
			return err
		}
	}
	if err := state.fence(ctx); err != nil {
		return err
	}
	if err := c.runtime.RestartDaemon(ctx); err != nil {
		return c.rollback(ctx, state, applied, err)
	}
	if _, err := state.transition(ctx, Transition{
		Kind: TransitionPhase, Actor: updated.Holder.Surface, Target: TargetRuntime,
		Phase: PhaseHealthChecking, Percent: -1, Outcome: "started",
	}); err != nil {
		return err
	}
	return c.healthAndFinalize(ctx, state, applied)
}

func (c *Coordinator) healthAndFinalize(
	ctx context.Context,
	state *coordinatorState,
	applied AppliedBinary,
) error {
	if err := state.fence(ctx); err != nil {
		return err
	}
	if err := c.runtime.HealthCheck(ctx); err != nil {
		return c.rollback(ctx, state, applied, err)
	}
	if err := state.fence(ctx); err != nil {
		return err
	}
	if err := c.manager.Finalize(applied); err != nil {
		return c.rollback(ctx, state, applied, err)
	}
	operation := state.snapshot()
	_, err := state.transition(ctx, Transition{
		Kind: TransitionPhase, Actor: operation.Holder.Surface, Target: TargetRuntime,
		Phase: PhaseFinalized, Percent: 100, Outcome: "updated",
	})
	return err
}

func (c *Coordinator) rollback(
	ctx context.Context,
	state *coordinatorState,
	applied AppliedBinary,
	cause error,
) error {
	if err := state.fence(ctx); err != nil {
		return errors.Join(cause, err)
	}
	if err := c.manager.Restore(applied); err != nil {
		return c.failRuntime(ctx, state, errors.Join(cause, err))
	}
	operation := state.snapshot()
	_, transitionErr := state.transition(ctx, Transition{
		Kind: TransitionPhase, Actor: operation.Holder.Surface, Target: TargetRuntime,
		Phase: PhaseRolledBack, Percent: -1, LastError: cause.Error(), Outcome: "rolled-back",
	})
	return errors.Join(cause, transitionErr)
}

func (c *Coordinator) failRuntime(ctx context.Context, state *coordinatorState, cause error) error {
	operation := state.snapshot()
	if operation == nil || operation.Runtime == nil || operation.Runtime.Phase == PhaseFailed ||
		operation.Runtime.Phase == PhaseRolledBack {
		return cause
	}
	_, err := state.transition(ctx, Transition{
		Kind: TransitionPhase, Actor: operation.Holder.Surface, Target: TargetRuntime,
		Phase: PhaseFailed, Percent: -1, LastError: cause.Error(), Outcome: "failed",
	})
	if errors.Is(err, ErrOperationNotFound) {
		return cause
	}
	return errors.Join(cause, err)
}
