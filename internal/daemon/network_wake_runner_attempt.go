package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (r *networkWakeRunner) processNotification(
	ctx context.Context,
	actor taskpkg.ActorContext,
	notification store.CommittedNetworkNotification,
) (bool, error) {
	targetSessionID := strings.TrimSpace(notification.RecipientSessionID)
	workspaceID := strings.TrimSpace(notification.WorkspaceID)
	claim, err := r.tasks.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            strings.TrimSpace(notification.TaskRunID),
		RunKind:          taskpkg.RunKindNetworkWake,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		TargetSessionID:  targetSessionID,
		ClaimerSessionID: targetSessionID,
		LeaseDuration:    taskpkg.DefaultRunLeaseDuration,
		Now:              r.now().UTC(),
	}, actor)
	if errors.Is(err, taskpkg.ErrNoClaimableRun) || errors.Is(err, taskpkg.ErrActiveRunLease) {
		return false, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return false, nil
		}
		return false, fmt.Errorf("daemon: claim network wake: %w", err)
	}
	if claim == nil {
		return false, errors.New("daemon: network wake claim violated run-kind fence")
	}
	if !claim.Run.IsNetworkWake() || claim.Task != nil {
		claimErr := errors.New("daemon: network wake claim violated run-kind fence")
		if releaseErr := r.releaseClaimedWake(
			ctx,
			actor,
			claim,
			"network wake run-kind fence violation",
		); releaseErr != nil {
			return false, errors.Join(claimErr, releaseErr)
		}
		return false, claimErr
	}
	if ctx.Err() != nil || !r.networkEnabled() {
		return true, r.releaseClaimedWake(ctx, actor, claim, "network wake execution unavailable")
	}
	wakeID, claimedTarget, ownerKey := claim.Run.NetworkWakeCorrelation()
	reservation := store.WakeReservation{
		WakeID:      wakeID,
		TaskRunID:   claim.Run.ID,
		WorkspaceID: claim.Run.WorkspaceID,
		OwnerKey:    ownerKey,
	}
	if claimedTarget != targetSessionID || strings.TrimSpace(claim.Run.WorkspaceID) != workspaceID {
		return false, r.failClaimedWake(ctx, actor, claim, reservation, "network wake target mismatch")
	}
	loadedReservation, messages, err := r.store.LoadNetworkWake(ctx, claim.Run.WorkspaceID, wakeID)
	if err != nil {
		if ctx.Err() != nil || !r.networkEnabled() {
			return true, r.releaseClaimedWake(ctx, actor, claim, "network wake execution unavailable")
		}
		return false, r.failClaimedWake(
			ctx,
			actor,
			claim,
			reservation,
			"network wake evidence unavailable",
		)
	}
	return r.executeClaimedWake(ctx, actor, claim, loadedReservation, messages, targetSessionID)
}

func (r *networkWakeRunner) executeClaimedWake(
	ctx context.Context,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
	reservation store.WakeReservation,
	messages []store.NetworkMessageEntry,
	targetSessionID string,
) (bool, error) {
	wallLimit, err := time.ParseDuration(reservation.ReservedWallTime)
	if err != nil || wallLimit <= 0 {
		return false, r.failClaimedWake(ctx, actor, claim, reservation, "network wake deadline is invalid")
	}
	prompt, meta, err := network.FormatNetworkWakePrompt(messages, targetSessionID)
	if err != nil {
		return false, r.failClaimedWake(ctx, actor, claim, reservation, "network wake prompt is invalid")
	}
	turnCtx, cancel := context.WithTimeout(ctx, wallLimit)
	r.registerActive(claim.Run.ID, cancel)
	defer func() {
		cancel()
		r.unregisterActive(claim.Run.ID)
	}()
	startedAt := r.now().UTC()
	heartbeatDone := r.startHeartbeat(turnCtx, cancel, actor, claim)

	_, promptErr := r.prompter.Resume(turnCtx, targetSessionID)
	if errors.Is(promptErr, session.ErrPromptInProgress) {
		return r.releasePromptContention(turnCtx, cancel, heartbeatDone, actor, claim)
	}
	if promptErr != nil {
		cancel()
		heartbeatErr := <-heartbeatDone
		if heartbeatErr != nil {
			promptErr = errors.Join(promptErr, heartbeatErr)
		}
		outcome := r.finishWakeUsage(store.NetworkWakeOutcome{
			State: store.NetworkWakeStateFailed, Reason: "network wake resume failed",
		}, acp.TokenUsage{}, startedAt)
		return false, errors.Join(
			r.terminalizeWake(ctx, actor, claim, reservation, outcome),
			promptErr,
		)
	}

	events, promptErr := r.prompter.PromptNetwork(turnCtx, targetSessionID, prompt, meta)
	if errors.Is(promptErr, session.ErrPromptInProgress) {
		return r.releasePromptContention(turnCtx, cancel, heartbeatDone, actor, claim)
	}
	providerStarted := promptErr == nil
	outcome := r.collectWakeOutcome(turnCtx, events, promptErr, startedAt)
	if turnCtx.Err() != nil && providerStarted {
		if cancelErr := r.cancelProviderPrompt(ctx, targetSessionID); cancelErr != nil {
			outcome.State = store.NetworkWakeStateFailed
			outcome.Reason = "network wake provider cancellation failed"
			r.logger.Error(
				"daemon.network_wake.provider_cancel_failed",
				"task_run_id", claim.Run.ID,
				"recipient_session_id", targetSessionID,
				"error", cancelErr,
			)
		}
	}
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		outcome.State = store.NetworkWakeStateFailed
		outcome.Reason = "network wake lease heartbeat failed"
	}
	return false, r.terminalizeWake(ctx, actor, claim, reservation, outcome)
}

func (r *networkWakeRunner) releasePromptContention(
	ctx context.Context,
	cancel context.CancelFunc,
	heartbeatDone <-chan error,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
) (bool, error) {
	cancel()
	heartbeatErr := <-heartbeatDone
	releaseErr := r.releaseClaimedWake(ctx, actor, claim, "network wake prompt already in progress")
	return true, errors.Join(heartbeatErr, releaseErr)
}

func (r *networkWakeRunner) cancelProviderPrompt(ctx context.Context, targetSessionID string) error {
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	if err := r.prompter.CancelPrompt(cancelCtx, targetSessionID); err != nil {
		return fmt.Errorf("daemon: cancel network wake provider prompt: %w", err)
	}
	return nil
}

func (r *networkWakeRunner) startHeartbeat(
	ctx context.Context,
	cancel context.CancelFunc,
	actor taskpkg.ActorContext,
	claim *taskpkg.ClaimResult,
) <-chan error {
	done := make(chan error, 1)
	go func() {
		interval := taskpkg.DefaultRunLeaseDuration / 3
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticker.C:
				_, err := r.tasks.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
					RunID:         claim.Run.ID,
					ClaimToken:    claim.ClaimToken,
					LeaseDuration: taskpkg.DefaultRunLeaseDuration,
					Now:           r.now().UTC(),
				}, actor)
				if err == nil {
					continue
				}
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					done <- nil
					return
				}
				cancel()
				done <- fmt.Errorf("daemon: heartbeat network wake lease: %w", err)
				return
			}
		}
	}()
	return done
}
