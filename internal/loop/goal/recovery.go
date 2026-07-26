package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
)

func validatePromptTerminal(wantPromptID string, result loop.ActionPromptResult) error {
	if strings.TrimSpace(result.PromptID) != strings.TrimSpace(wantPromptID) {
		return fmt.Errorf("%w: managed Goal prompt terminal correlation changed", loop.ErrValidation)
	}
	if result.TokensUsed < 0 || result.EventStartSeq < 0 || result.EventEndSeq < result.EventStartSeq {
		return fmt.Errorf("%w: managed Goal prompt terminal evidence is invalid", loop.ErrValidation)
	}
	if err := validatePromptTerminalOutcome(result); err != nil {
		return err
	}
	if !result.TokensReported && result.TokensUsed != 0 {
		return fmt.Errorf("%w: unreported Goal tokens must remain absent", loop.ErrValidation)
	}
	return nil
}

func validatePromptTerminalOutcome(result loop.ActionPromptResult) error {
	switch result.Outcome {
	case loop.ActionPromptOutcomeCompleted:
		return validateCompletedPromptTerminal(result)
	case loop.ActionPromptOutcomeInvalidResult:
		return validateReasonOnlyPromptTerminal(
			result,
			loop.ReasonCodeGoalStopReasonInvalid,
			"invalid Goal result",
		)
	case loop.ActionPromptOutcomeFailed:
		return validateReasonOnlyPromptTerminal(
			result,
			loop.ReasonCodeGoalPromptRequestFailed,
			"failed Goal prompt",
		)
	case loop.ActionPromptOutcomeAmbiguous:
		return validateAmbiguousPromptTerminal(result)
	case loop.ActionPromptOutcomeBudgetFenced, loop.ActionPromptOutcomeControlFenced:
		return validateFencedPromptTerminal(result)
	case loop.ActionPromptOutcomeRejectedBeforeSubmit:
		return validateRejectedPromptTerminal(result)
	default:
		return fmt.Errorf("%w: unknown managed Goal prompt outcome %q", loop.ErrValidation, result.Outcome)
	}
}

func validateCompletedPromptTerminal(result loop.ActionPromptResult) error {
	if !result.StopReason.Valid() || result.ReasonCode != "" || result.FenceDisposition != "" {
		return fmt.Errorf("%w: completed Goal prompt requires exactly one stop reason", loop.ErrValidation)
	}
	return nil
}

func validateReasonOnlyPromptTerminal(
	result loop.ActionPromptResult,
	want loop.ReasonCode,
	label string,
) error {
	if result.StopReason != "" || result.ReasonCode != want {
		return fmt.Errorf("%w: %s terminal shape changed", loop.ErrValidation, label)
	}
	return nil
}

func validateAmbiguousPromptTerminal(result loop.ActionPromptResult) error {
	validCause := result.ReasonCode == loop.ReasonCodeGoalRecoveryAmbiguous ||
		result.ReasonCode == loop.ReasonCodeGoalControlRevokedInFlight
	if result.StopReason != "" || !validCause {
		return fmt.Errorf("%w: ambiguous Goal prompt terminal shape changed", loop.ErrValidation)
	}
	return nil
}

func validateFencedPromptTerminal(result loop.ActionPromptResult) error {
	if result.StopReason != "" || !result.FenceDisposition.Valid() || result.ReasonCode == "" {
		return fmt.Errorf("%w: fenced Goal prompt terminal shape changed", loop.ErrValidation)
	}
	return nil
}

func validateRejectedPromptTerminal(result loop.ActionPromptResult) error {
	if result.StopReason != "" || result.ReasonCode == "" {
		return fmt.Errorf("%w: rejected Goal prompt terminal shape changed", loop.ErrValidation)
	}
	return nil
}

func (e *Executor) recoverPendingPrompt(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	checkpoint := segment.checkpoint
	if checkpoint.Phase == checkpointPhaseJudging && checkpoint.JudgeAttemptID != "" {
		ticket := loop.ActionPromptTicket{
			QueueEntryID: checkpoint.QueueEntryID,
			PromptID:     checkpoint.PromptID,
		}
		result, err := e.binder.AwaitActionPrompt(ctx, ticket)
		if err != nil {
			return e.recoverAwaitFailure(ctx, segment, ticket, err)
		}
		if err := validatePromptTerminal(ticket.PromptID, result); err != nil {
			return nil, err
		}
		segment.lastResult = result
		return e.recoverJudgeAttempt(ctx, segment)
	}
	ticket := loop.ActionPromptTicket{
		QueueEntryID: checkpoint.QueueEntryID,
		PromptID:     checkpoint.PromptID,
	}
	if checkpoint.Phase == checkpointPhasePreparing || checkpoint.Phase == checkpointPhaseQueued {
		turn := checkpoint.TurnsUsed + 1
		if checkpoint.PromptKind == promptKindCompact {
			turn = checkpoint.TurnsUsed
		}
		operationBase, operationBaseReported := segment.usage.snapshot()
		contextSequence, contextUsed := recoveredCompactionBaseline(checkpoint)
		_, err := e.binder.PrepareActionPrompt(ctx, segment.binding, loop.ActionPromptRequest{
			PromptID:             checkpoint.PromptID,
			Message:              e.recoveryPromptMessage(segment, checkpoint),
			Kind:                 checkpoint.PromptKind,
			Owner:                segment.promptOwner(turn),
			UsageBaseTokens:      operationBase,
			UsageBaseReported:    operationBaseReported,
			ContextUsageSequence: contextSequence,
			ContextUsageUsed:     contextUsed,
			UsageReporter:        segment.usage.operationReporter(operationBase),
		})
		if err != nil {
			return e.recoverAwaitFailure(ctx, segment, ticket, err)
		}
	}
	operationBase := segment.usage.operationBase()
	if checkpoint.PromptKind == promptKindCompact && checkpoint.CompactionCancel != nil {
		result, err := e.cancelAndDrainCompaction(
			ctx,
			ticket,
			segment.promptOwner(checkpoint.TurnsUsed),
		)
		if err != nil {
			return e.recoverAwaitFailure(ctx, segment, ticket, err)
		}
		return e.processCompactionResult(ctx, segment, ticket, result, operationBase, nil)
	}
	result, err := e.binder.AwaitActionPrompt(ctx, ticket)
	if err != nil {
		return e.recoverAwaitFailure(ctx, segment, ticket, err)
	}
	if checkpoint.PromptKind == promptKindCompact {
		return e.processCompactionResult(ctx, segment, ticket, result, operationBase, nil)
	}
	return e.processWorkResult(ctx, segment, ticket, result, operationBase)
}

func recoveredCompactionBaseline(checkpoint Checkpoint) (*int64, *int64) {
	if checkpoint.PromptKind != promptKindCompact || checkpoint.UsageSequence == nil ||
		checkpoint.CompactionBaselineUsed == nil {
		return nil, nil
	}
	sequence := *checkpoint.UsageSequence
	used := *checkpoint.CompactionBaselineUsed
	return &sequence, &used
}

func (e *Executor) recoveryPromptMessage(segment *segmentState, checkpoint Checkpoint) string {
	if checkpoint.PromptKind == promptKindCompact {
		return renderCompactionPrompt(segment)
	}
	turn := checkpoint.TurnsUsed + 1
	return renderWorkPrompt(segment, turn, checkpoint.PromptKind)
}

func (e *Executor) recoverAwaitFailure(
	ctx context.Context,
	segment *segmentState,
	ticket loop.ActionPromptTicket,
	awaitErr error,
) (*turnBoundary, error) {
	checkpoint, err := e.store.LoadCheckpoint(context.WithoutCancel(ctx), segment.key)
	if err != nil {
		return nil, errors.Join(awaitErr, err)
	}
	segment.checkpoint = checkpoint
	if checkpoint.Phase != checkpointPhasePrompting && checkpoint.Phase != checkpointPhaseCompacting {
		return nil, awaitErr
	}
	result, found, recoveryErr := e.recovery.ReconcileTerminalFromEvents(
		context.WithoutCancel(ctx),
		PromptRecoveryIdentity{
			Key:                  segment.key,
			ExpectedControlEpoch: checkpoint.ControlEpoch,
			ExpectedBindingEpoch: checkpoint.BindingEpoch,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             ticket.PromptID,
			SessionID:            checkpoint.SessionID,
		},
	)
	if recoveryErr != nil {
		return nil, errors.Join(awaitErr, recoveryErr)
	}
	if found {
		if checkpoint.PromptKind == promptKindCompact {
			return e.processCompactionResult(
				context.WithoutCancel(ctx), segment, ticket, result, segment.usage.operationBase(), nil,
			)
		}
		return e.processWorkResult(
			context.WithoutCancel(ctx), segment, ticket, result, segment.usage.operationBase(),
		)
	}
	if err := e.store.MarkAmbiguous(context.WithoutCancel(ctx), AmbiguousRequest{
		Key:                  segment.key,
		ExpectedControlEpoch: checkpoint.ControlEpoch,
		ExpectedBindingEpoch: checkpoint.BindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		QueueEntryID:         ticket.QueueEntryID,
		PromptID:             ticket.PromptID,
		Cause:                loop.ReasonCodeGoalRecoveryAmbiguous,
	}); err != nil {
		return nil, errors.Join(awaitErr, err)
	}
	updated, err := e.store.LoadCheckpoint(context.WithoutCancel(ctx), segment.key)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = updated
	control, err := e.recoveredControl(context.WithoutCancel(ctx), segment)
	return &turnBoundary{checkpoint: updated, control: control}, err
}

func (e *Executor) handlePreparedPromptError(
	ctx context.Context,
	segment *segmentState,
	prepareErr error,
) (*turnBoundary, error) {
	checkpoint, err := e.store.LoadCheckpoint(context.WithoutCancel(ctx), segment.key)
	if err != nil {
		return nil, errors.Join(prepareErr, err)
	}
	segment.checkpoint = checkpoint
	if checkpointHasPendingPrompt(checkpoint) {
		return e.recoverPendingPrompt(context.WithoutCancel(ctx), segment)
	}
	return nil, prepareErr
}

func (e *Executor) ambiguousPromptBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	if result.ReasonCode == loop.ReasonCodeGoalControlRevokedInFlight {
		return nil, newControlRevokedError()
	}
	if segment.checkpoint.Phase != checkpointPhaseAwaitingControl {
		if err := e.store.MarkAmbiguous(ctx, AmbiguousRequest{
			Key:                  segment.key,
			ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
			ExpectedBindingEpoch: segment.checkpoint.BindingEpoch,
			TaskRunID:            segment.input.CorrelationID,
			QueueEntryID:         segment.checkpoint.QueueEntryID,
			PromptID:             result.PromptID,
			Cause:                loop.ReasonCodeGoalRecoveryAmbiguous,
		}); err != nil {
			return nil, err
		}
		updated, err := e.store.LoadCheckpoint(ctx, segment.key)
		if err != nil {
			return nil, err
		}
		segment.checkpoint = updated
	}
	control, err := e.recoveredControl(ctx, segment)
	return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control, completed: true}, err
}

func (e *Executor) retryRejectedPrompt(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = checkpoint
	if checkpoint.PromptID == result.PromptID || checkpointHasPendingPrompt(checkpoint) {
		return nil, fmt.Errorf(
			"%w: managed Goal pre-submit rejection did not advance its durable attempt",
			loop.ErrTransitionConflict,
		)
	}
	maxAttempts := 1
	if segment.node.Retry != nil && segment.node.Retry.MaxAttempts > 0 {
		maxAttempts = segment.node.Retry.MaxAttempts
	}
	if checkpoint.PromptAttempt < maxAttempts {
		if goalFreshSessionOnFailure(segment.node) {
			if err := e.rotateBinding(ctx, segment); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusPaused,
		loop.ReasonCodeGoalPresubmitRetriesExhausted,
		automaticActor(),
	)
	return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control}, err
}
