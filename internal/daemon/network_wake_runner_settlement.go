package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (r *networkWakeRunner) terminalizeWake(
	ctx context.Context,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
	reservation store.WakeReservation,
	outcome store.NetworkWakeOutcome,
) error {
	terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	wakeID, targetSessionID, ownerKey := claim.Run.NetworkWakeCorrelation()
	_, settlementErr := r.tasks.SettleNetworkWake(terminalCtx, taskpkg.NetworkWakeSettlement{
		WorkspaceID:     reservation.WorkspaceID,
		WakeID:          wakeID,
		TaskRunID:       claim.Run.ID,
		ClaimToken:      claim.ClaimToken,
		TargetSessionID: targetSessionID,
		OwnerKey:        ownerKey,
		Outcome:         outcome,
		Now:             r.now().UTC(),
	}, actor)
	if settlementErr != nil {
		terminalErr := fmt.Errorf("daemon: terminalize network wake task run: %w", settlementErr)
		if releaseErr := r.releaseClaimedWake(
			ctx,
			actor,
			claim,
			"network wake terminal transition failed",
		); releaseErr != nil {
			return errors.Join(terminalErr, releaseErr)
		}
		return terminalErr
	}
	return nil
}

func (r *networkWakeRunner) releaseClaimedWake(
	ctx context.Context,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
	reason string,
) error {
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	if _, err := r.tasks.ReleaseRunLease(releaseCtx, taskpkg.LeaseRelease{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Reason:     reason,
		Now:        r.now().UTC(),
	}, actor); err != nil {
		return fmt.Errorf("daemon: release network wake task run after failure: %w", err)
	}
	return nil
}

func (r *networkWakeRunner) failClaimedWake(
	ctx context.Context,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
	reservation store.WakeReservation,
	reason string,
) error {
	return r.terminalizeWake(ctx, actor, claim, reservation, store.NetworkWakeOutcome{
		State:      store.NetworkWakeStateFailed,
		UsageState: store.NetworkWakeUsageUnavailable,
		Reason:     reason,
	})
}
