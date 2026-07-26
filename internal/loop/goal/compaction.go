package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
)

func (e *Executor) recoverFromMaxTokens(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	if control, ok, err := e.pendingPauseBoundary(ctx, segment); ok || err != nil {
		return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control, completed: true}, err
	}
	usage, usageErr := e.context.Usage(ctx, segment.binding)
	usageCurrent := false
	if usageErr == nil && usage.Known {
		usageCurrent, usageErr = e.recordContextUsage(ctx, segment, usage)
		if usageErr != nil {
			return nil, usageErr
		}
		if segment.checkpoint.CompactionRecoveryRequired {
			return e.reseedOrEscalate(ctx, segment, result)
		}
	}
	if segment.checkpoint.RecoveryStreak >= 2 {
		return e.reseedApprovalBoundary(ctx, segment, result)
	}
	hasCompact, commandErr := e.context.HasAdvertisedCommand(ctx, segment.binding, "compact")
	if usageErr == nil && commandErr == nil && usageCurrent && hasCompact {
		return e.runCompaction(ctx, segment, &usage)
	}
	if usageErr != nil || commandErr != nil {
		segment.lastVerdict = "context-telemetry-unavailable"
	}
	return e.reseedOrEscalate(ctx, segment, result)
}

func (e *Executor) maybeCompactBeforeWork(
	ctx context.Context,
	segment *segmentState,
) (*turnBoundary, bool, error) {
	if segment.checkpoint.ContextNudgeRatio <= 0 || checkpointHasPendingPrompt(segment.checkpoint) ||
		hasWorkAndSettleGrant(segment.checkpoint) {
		return nil, false, nil
	}
	usage, err := e.context.Usage(ctx, segment.binding)
	if err != nil || !usage.Known || usage.Size <= 0 {
		return nil, false, nil
	}
	current, err := e.recordContextUsage(ctx, segment, usage)
	if err != nil {
		return nil, false, err
	}
	if !current {
		return nil, false, nil
	}
	if segment.checkpoint.CompactionRecoveryRequired {
		boundary, recoveryErr := e.reseedOrEscalate(ctx, segment, segment.lastResult)
		return boundary, true, recoveryErr
	}
	if float64(usage.Used)/float64(usage.Size) < segment.checkpoint.ContextNudgeRatio {
		return nil, false, nil
	}
	hasCompact, err := e.context.HasAdvertisedCommand(ctx, segment.binding, "compact")
	if err != nil || !hasCompact {
		return nil, false, nil
	}
	boundary, err := e.runCompaction(ctx, segment, &usage)
	return boundary, true, err
}

func hasWorkAndSettleGrant(checkpoint Checkpoint) bool {
	return checkpoint.ControlGrant != nil && !checkpoint.ControlGrant.Consumed &&
		checkpoint.ControlGrant.Kind == ControlGrantBudget &&
		checkpoint.ControlGrant.Scope == ControlGrantScopeWorkAndSettle &&
		checkpoint.ControlGrant.Turn == checkpoint.TurnsUsed+1
}

func (e *Executor) runCompaction(
	ctx context.Context,
	segment *segmentState,
	beforeUsage *ContextUsage,
) (*turnBoundary, error) {
	turn := segment.checkpoint.TurnsUsed
	promptID, err := deterministicPromptID(
		segment.key,
		turn,
		promptKindCompact,
		segment.checkpoint.PromptAttempt,
	)
	if err != nil {
		return nil, err
	}
	operationBase, operationBaseReported := segment.usage.snapshot()
	decision, err := e.flushBudget(
		ctx,
		segment,
		BudgetBeforeCompact,
		checkpointPhaseCompacting,
		turn,
		promptID,
		operationBase,
	)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		control, controlErr := e.budgetBoundary(ctx, segment, decision)
		return &turnBoundary{checkpoint: segment.checkpoint, control: control}, controlErr
	}
	request := loop.ActionPromptRequest{
		PromptID:             promptID,
		Message:              renderCompactionPrompt(segment),
		Kind:                 promptKindCompact,
		Owner:                segment.promptOwner(turn),
		UsageBaseTokens:      operationBase,
		UsageBaseReported:    operationBaseReported,
		ContextUsageSequence: contextUsageSequence(beforeUsage),
		ContextUsageUsed:     contextUsageUsed(beforeUsage),
		UsageReporter:        segment.usage.operationReporter(operationBase),
	}
	ticket, err := e.binder.PrepareActionPrompt(ctx, segment.binding, request)
	if err != nil {
		return e.handlePreparedPromptError(ctx, segment, err)
	}
	if ticket.PromptID != promptID || strings.TrimSpace(ticket.QueueEntryID) == "" {
		return nil, fmt.Errorf("%w: managed Goal compaction ticket identity changed", loop.ErrValidation)
	}
	result, err := e.awaitCompaction(ctx, segment, ticket, request.Owner)
	if err != nil {
		return e.recoverAwaitFailure(ctx, segment, ticket, err)
	}
	return e.processCompactionResult(ctx, segment, ticket, result, operationBase, beforeUsage)
}

func (e *Executor) awaitCompaction(
	ctx context.Context,
	segment *segmentState,
	ticket loop.ActionPromptTicket,
	owner loop.ActionPromptOwner,
) (loop.ActionPromptResult, error) {
	waitCtx, cancel := context.WithTimeout(ctx, e.compactionTimeout)
	result, err := e.binder.AwaitActionPrompt(waitCtx, ticket)
	cancel()
	if !errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
		return result, err
	}
	checkpoint, loadErr := e.store.LoadCheckpoint(context.WithoutCancel(ctx), segment.key)
	if loadErr != nil {
		return loop.ActionPromptResult{}, errors.Join(err, loadErr)
	}
	segment.checkpoint = checkpoint
	if beginErr := e.store.BeginCompactionCancel(context.WithoutCancel(ctx), BeginCompactionCancelRequest{
		Key:                  segment.key,
		ExpectedControlEpoch: checkpoint.ControlEpoch,
		ExpectedBindingEpoch: checkpoint.BindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		QueueEntryID:         ticket.QueueEntryID,
		PromptID:             ticket.PromptID,
		Cause:                "timeout",
	}); beginErr != nil {
		return loop.ActionPromptResult{}, errors.Join(err, beginErr)
	}
	drained, drainErr := e.cancelAndDrainCompaction(ctx, ticket, owner)
	if drainErr != nil {
		return loop.ActionPromptResult{}, errors.Join(err, drainErr)
	}
	return drained, nil
}

func (e *Executor) cancelAndDrainCompaction(
	ctx context.Context,
	ticket loop.ActionPromptTicket,
	owner loop.ActionPromptOwner,
) (loop.ActionPromptResult, error) {
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.compactionDrain)
	defer cancel()
	if err := e.binder.CancelActionPrompts(drainCtx, owner); err != nil {
		return loop.ActionPromptResult{}, err
	}
	return e.binder.AwaitActionPrompt(drainCtx, ticket)
}

func (e *Executor) processCompactionResult(
	ctx context.Context,
	segment *segmentState,
	ticket loop.ActionPromptTicket,
	result loop.ActionPromptResult,
	operationBase int64,
	beforeUsage *ContextUsage,
) (*turnBoundary, error) {
	if err := validatePromptTerminal(ticket.PromptID, result); err != nil {
		return nil, err
	}
	segment.usage.observeOperation(operationBase, result.TokensUsed, result.TokensReported)
	segment.lastResult = result
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = checkpoint
	postDecision, err := e.flushBudget(
		ctx,
		segment,
		BudgetAfterCompact,
		checkpointPhaseCompacting,
		checkpoint.TurnsUsed,
		result.PromptID,
		operationBase,
	)
	if err != nil {
		return nil, err
	}
	if result.Outcome == loop.ActionPromptOutcomeBudgetFenced ||
		result.Outcome == loop.ActionPromptOutcomeControlFenced {
		control, controlErr := e.fencedPromptBoundary(ctx, segment, result)
		return &turnBoundary{checkpoint: checkpoint, result: result, control: control}, controlErr
	}
	if result.Outcome == loop.ActionPromptOutcomeAmbiguous {
		return e.ambiguousPromptBoundary(ctx, segment, result)
	}
	timedOut := checkpoint.CompactionCancel != nil && checkpoint.CompactionCancel.PromptID == result.PromptID
	outcome, cause := classifyCompactionResult(result, timedOut)
	updated, err := e.store.CompleteCompaction(ctx, CompleteCompactionRequest{
		Key:                  segment.key,
		ExpectedControlEpoch: checkpoint.ControlEpoch,
		ExpectedBindingEpoch: checkpoint.BindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		QueueEntryID:         ticket.QueueEntryID,
		PromptID:             result.PromptID,
		Result: CompactionResult{
			PromptResult:      result,
			Outcome:           outcome,
			UsageSequence:     contextUsageSequence(beforeUsage),
			UsageBaselineUsed: contextUsageUsed(beforeUsage),
		},
	})
	if err != nil {
		return nil, err
	}
	segment.checkpoint = updated
	updated, err = e.recordPostCompactionUsage(ctx, segment, outcome)
	if err != nil {
		return nil, err
	}
	if cause != "" {
		control, err := e.checkpointBoundary(
			ctx, segment, loop.ActionDispositionNeedsApproval, goalStatusPaused, cause, automaticActor(),
		)
		return &turnBoundary{checkpoint: updated, result: result, control: control, completed: true}, err
	}
	if !postDecision.Allowed {
		control, err := e.budgetBoundary(ctx, segment, postDecision)
		return &turnBoundary{checkpoint: updated, result: result, control: control, completed: true}, err
	}
	if outcome == CompactionSucceeded {
		if segment.checkpoint.CompactionRecoveryRequired {
			return e.reseedOrEscalate(ctx, segment, result)
		}
		return &turnBoundary{checkpoint: updated, result: result, completed: true}, nil
	}
	return e.reseedOrEscalate(ctx, segment, result)
}

func (e *Executor) recordPostCompactionUsage(
	ctx context.Context,
	segment *segmentState,
	outcome CompactionOutcome,
) (Checkpoint, error) {
	if outcome != CompactionSucceeded {
		return segment.checkpoint, nil
	}
	afterUsage, err := e.context.Usage(ctx, segment.binding)
	if err != nil || !afterUsage.Known {
		return segment.checkpoint, nil
	}
	if _, err := e.recordContextUsage(ctx, segment, afterUsage); err != nil {
		return Checkpoint{}, err
	}
	return segment.checkpoint, nil
}

func contextUsageUsed(usage *ContextUsage) *int64 {
	if usage == nil || !usage.Known {
		return nil
	}
	value := usage.Used
	return &value
}

func classifyCompactionResult(
	result loop.ActionPromptResult,
	timedOut bool,
) (CompactionOutcome, loop.ReasonCode) {
	switch result.Outcome {
	case loop.ActionPromptOutcomeInvalidResult:
		return CompactionFailed, loop.ReasonCodeGoalStopReasonInvalid
	case loop.ActionPromptOutcomeFailed:
		return CompactionFailed, loop.ReasonCodeGoalPromptRequestFailed
	case loop.ActionPromptOutcomeRejectedBeforeSubmit:
		return CompactionFailed, loop.ReasonCodeGoalPresubmitRetriesExhausted
	case loop.ActionPromptOutcomeCompleted:
	default:
		return CompactionFailed, loop.ReasonCodeGoalCompactionCancelled
	}
	switch result.StopReason {
	case loop.ActionStopEndTurn, loop.ActionStopMaxTurnRequests:
		return CompactionSucceeded, ""
	case loop.ActionStopMaxTokens:
		return CompactionIneffective, ""
	case loop.ActionStopRefusal:
		return CompactionRefused, ""
	case loop.ActionStopCancelled:
		if timedOut {
			return CompactionTimedOut, ""
		}
		return CompactionCancelled, loop.ReasonCodeGoalCompactionCancelled
	default:
		return CompactionFailed, loop.ReasonCodeGoalStopReasonInvalid
	}
}

func (e *Executor) reseedOrEscalate(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	grantAllowsReseed := segment.checkpoint.ControlGrant != nil &&
		segment.checkpoint.ControlGrant.Kind == ControlGrantReseed &&
		segment.checkpoint.ControlGrant.Scope == ControlGrantScopeRotateBinding &&
		!segment.checkpoint.ControlGrant.Consumed
	if segment.binding.Ownership == string(BindingOwnershipOriginBorrowed) && !grantAllowsReseed {
		return e.reseedApprovalBoundary(ctx, segment, result)
	}
	if segment.checkpoint.RecoveryStreak >= 2 && !grantAllowsReseed {
		return e.reseedApprovalBoundary(ctx, segment, result)
	}
	if err := e.rotateBinding(ctx, segment); err != nil {
		return nil, err
	}
	return &turnBoundary{checkpoint: segment.checkpoint, result: result, completed: true}, nil
}

func (e *Executor) reseedApprovalBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusUsageLimited,
		loop.ReasonCodeGoalReseedConfirmationRequired,
		automaticActor(),
	)
	return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control, completed: true}, err
}

func (e *Executor) rotateBinding(ctx context.Context, segment *segmentState) error {
	oldBinding := segment.binding
	segment.checkpoint.BindingEpoch = oldBinding.BindingEpoch + 1
	if err := e.bindSegment(ctx, segment); err != nil {
		segment.checkpoint.BindingEpoch = oldBinding.BindingEpoch
		return err
	}
	return nil
}
