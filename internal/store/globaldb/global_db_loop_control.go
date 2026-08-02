package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.CoordinatorControlStore = (*LoopRepo)(nil)

// ReactivateLoopCoordinator atomically transitions a parked Run and reserves its next coordinator.
func (g *LoopRepo) ReactivateLoopCoordinator(
	ctx context.Context,
	req *looppkg.CoordinatorReactivationRequest,
) (looppkg.CoordinatorReactivationResult, error) {
	if err := g.checkReady(ctx, "reactivate Loop coordinator"); err != nil {
		return looppkg.CoordinatorReactivationResult{}, err
	}
	if err := validateLoopCoordinatorReactivation(req); err != nil {
		return looppkg.CoordinatorReactivationResult{}, err
	}

	var result looppkg.CoordinatorReactivationResult
	err := g.withTaskImmediateTransaction(ctx, "reactivate Loop coordinator", func(exec taskSQLExecutor) error {
		current, err := getLoopRunByIDWithExecutor(ctx, exec, req.Run.ID)
		if err != nil {
			return err
		}
		if current.WorkspaceID != req.Run.WorkspaceID || current.Status != req.Run.Status ||
			current.ActiveGateID != req.Run.ActiveGateID {
			return fmt.Errorf("%w: loop run %q control state changed", looppkg.ErrTransitionConflict, req.Run.ID)
		}
		for _, decision := range req.Decisions {
			if err := insertLoopGateDecision(ctx, exec, decision); err != nil {
				return err
			}
		}
		if req.Cause == looppkg.TransitionCauseApproval && len(req.Decisions) > 0 {
			if err := claimActiveApprovalWait(ctx, exec, current, req.Decisions[0]); err != nil {
				return err
			}
		}
		if err := compareAndSwapLoopRunStatusWithExecutor(
			ctx,
			exec,
			current.ID,
			current.Status,
			looppkg.StatusRunning,
			req.Cause,
			req.ReactivatedAt,
		); err != nil {
			return err
		}
		current.Status = looppkg.StatusRunning
		current.PauseRequested = false
		const reactivationRunID = ""
		const reactivationIdempotencyKey = ""
		coordinator, added, err := g.reserveLoopCoordinatorRunWithExecutor(
			ctx,
			exec,
			current,
			req.Actor.Origin,
			req.ReactivatedAt,
			reactivationRunID,
			reactivationIdempotencyKey,
		)
		if err != nil {
			return err
		}
		if !added {
			return fmt.Errorf("%w: Loop coordinator control wake was not reserved", looppkg.ErrTransitionConflict)
		}
		result = looppkg.CoordinatorReactivationResult{Run: coordinator}
		return nil
	})
	if err != nil {
		return looppkg.CoordinatorReactivationResult{}, err
	}
	return result, nil
}

func validateLoopCoordinatorReactivation(req *looppkg.CoordinatorReactivationRequest) error {
	if req == nil {
		return fmt.Errorf("%w: coordinator reactivation request is required", looppkg.ErrValidation)
	}
	if strings.TrimSpace(string(req.Run.ID)) == "" || strings.TrimSpace(string(req.Run.WorkspaceID)) == "" {
		return fmt.Errorf("%w: workspace_id and run_id are required", looppkg.ErrValidation)
	}
	if err := req.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: control actor: %w", looppkg.ErrValidation, err)
	}
	if req.ReactivatedAt.IsZero() {
		return fmt.Errorf("%w: reactivated_at is required", looppkg.ErrValidation)
	}
	switch req.Cause {
	case looppkg.TransitionCauseOperatorResume:
		if req.Run.Status != looppkg.StatusPaused || len(req.Decisions) != 0 {
			return fmt.Errorf("%w: operator resume requires a paused Run without gate decisions", looppkg.ErrValidation)
		}
	case looppkg.TransitionCauseApproval:
		if req.Run.Status != looppkg.StatusNeedsApproval || len(req.Decisions) != 1 {
			return fmt.Errorf("%w: approval requires a gated Run and one decision", looppkg.ErrValidation)
		}
	default:
		return fmt.Errorf("%w: unsupported coordinator reactivation cause %q", looppkg.ErrValidation, req.Cause)
	}
	for _, decision := range req.Decisions {
		if decision.RunID != req.Run.ID || decision.WorkspaceID != req.Run.WorkspaceID {
			return fmt.Errorf("%w: gate decision does not belong to the reactivated Run", looppkg.ErrValidation)
		}
	}
	return nil
}
