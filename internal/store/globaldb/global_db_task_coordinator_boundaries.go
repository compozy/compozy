package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func applyCoordinatorBudgetBoundary(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	snapshot taskpkg.GenerationSnapshot,
	loopRun looppkg.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	budgetStatus := looppkg.StatusExhausted
	if loopRun.BudgetOnExceeded == dsl.BudgetExceededEscalate {
		budgetStatus = looppkg.StatusNeedsApproval
	}
	payload, err := looppkg.GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	if err := updateLoopBoundaryStatusWithEffects(
		ctx,
		exec,
		loopRun,
		budgetStatus,
		looppkg.TransitionCauseBudget,
		completion.Now,
		snapshot.Generation,
		nil,
		payload.BoundaryEffects[budgetStatus],
	); err != nil {
		return err
	}
	if budgetStatus == looppkg.StatusNeedsApproval {
		if err := activateLoopApprovalWithExecutor(
			ctx,
			exec,
			loopRun.ID,
			looppkg.BudgetGateID,
			nil,
			true,
		); err != nil {
			return err
		}
		if err := appendLoopNeedsApprovalEventWithExecutor(
			ctx,
			exec,
			loopRun,
			looppkg.BudgetGateID,
			snapshot.Generation,
			"Budget approval requested",
			[]map[string]string{
				loopApprovalFact("Tokens used", fmt.Sprintf("%d", loopRun.TokensUsed)),
				loopApprovalFact("Token budget", fmt.Sprintf("%d", loopRun.BudgetTokens)),
				loopApprovalFact("Wall-clock budget", fmt.Sprintf("%ds", loopRun.BudgetWallSec)),
			},
			completion.Now,
		); err != nil {
			return err
		}
	}
	if budgetStatus.Terminal() {
		if err := sweepOrphanedLoopOutputBlobsWithExecutor(ctx, exec); err != nil {
			return err
		}
	}
	if err := updateCoordinatorResultContext(result, snapshot.Generation, budgetStatus); err != nil {
		return err
	}
	result.Terminal = loopStatusIsTerminalOrApproval(budgetStatus)
	return nil
}

func applyCoordinatorTerminalBoundary(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	snapshot taskpkg.GenerationSnapshot,
	loopRun looppkg.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	terminalStatus := looppkg.Status(completion.Plan.Terminal.Status)
	if !terminalStatus.Valid() {
		return fmt.Errorf(
			"%w: coordinator terminal status is invalid: %q",
			taskpkg.ErrValidation,
			completion.Plan.Terminal.Status,
		)
	}
	cause := looppkg.TransitionCause(strings.TrimSpace(completion.Plan.Terminal.Cause))
	if strings.TrimSpace(string(cause)) == "" {
		cause = looppkg.TransitionCauseContract
	}
	payload, err := looppkg.GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	if err := updateLoopBoundaryStatusWithEffects(
		ctx,
		exec,
		loopRun,
		terminalStatus,
		cause,
		completion.Now,
		snapshot.Generation,
		nil,
		payload.BoundaryEffects[terminalStatus],
	); err != nil {
		return err
	}
	if terminalStatus == looppkg.StatusNeedsApproval {
		gateID := looppkg.NodeID(strings.TrimSpace(completion.Plan.Terminal.GateID))
		if gateID == "" {
			return fmt.Errorf("%w: coordinator terminal gate_id is required for needs-approval", taskpkg.ErrValidation)
		}
		criteria, err := activeHumanCriteriaFromTerminal(completion.Plan.Terminal)
		if err != nil {
			return err
		}
		if err := activateLoopApprovalWithExecutor(ctx, exec, loopRun.ID, gateID, criteria, false); err != nil {
			return err
		}
		if err := appendLoopNeedsApprovalEventWithExecutor(
			ctx,
			exec,
			loopRun,
			gateID,
			snapshot.Generation,
			"Human approval requested",
			[]map[string]string{
				loopApprovalFact("Gate", string(gateID)),
				loopApprovalFact("Criteria", fmt.Sprintf("%d", len(criteria))),
			},
			completion.Now,
		); err != nil {
			return err
		}
	}
	if terminalStatus.Terminal() {
		if err := sweepOrphanedLoopOutputBlobsWithExecutor(ctx, exec); err != nil {
			return err
		}
	}
	if err := updateCoordinatorResultContext(result, snapshot.Generation, terminalStatus); err != nil {
		return err
	}
	result.Terminal = loopStatusIsTerminalOrApproval(terminalStatus)
	return nil
}
