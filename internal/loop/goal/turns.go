package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
)

func (e *Executor) advanceSegment(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return nil, fmt.Errorf("goal: load checkpoint before advancing segment: %w", err)
	}
	segment.checkpoint = checkpoint
	if checkpoint.Phase == checkpointPhaseAwaitingControl || checkpoint.Phase == checkpointPhaseTerminal {
		control, err := e.recoveredControl(ctx, segment)
		return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
	}
	if checkpointHasPendingPrompt(checkpoint) {
		return e.recoverPendingPrompt(ctx, segment)
	}
	if control, ok, pauseErr := e.pendingPauseBoundary(ctx, segment); ok || pauseErr != nil {
		return &turnBoundary{checkpoint: segment.checkpoint, control: control, completed: true}, pauseErr
	}
	if checkpoint.CompactionRecoveryRequired {
		return e.reseedOrEscalate(ctx, segment, segment.lastResult)
	}
	if checkpoint.TurnsUsed >= checkpoint.TurnLimit {
		return e.exhaustedBoundary(ctx, segment)
	}
	if boundary, compacted, err := e.maybeCompactBeforeWork(ctx, segment); err != nil {
		return nil, err
	} else if compacted {
		return boundary, nil
	}
	return e.runWorkTurn(ctx, segment)
}

func checkpointHasPendingPrompt(checkpoint Checkpoint) bool {
	return strings.TrimSpace(checkpoint.PromptID) != "" && strings.TrimSpace(checkpoint.QueueEntryID) != ""
}

func (e *Executor) runWorkTurn(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	turn := segment.checkpoint.TurnsUsed + 1
	kind := promptKindWork
	if turn > 1 || segment.checkpoint.RecoveryStreak > 0 {
		kind = promptKindContinuation
	}
	promptID, err := deterministicPromptID(segment.key, turn, kind, segment.checkpoint.PromptAttempt)
	if err != nil {
		return nil, err
	}
	operationBase, operationBaseReported := segment.usage.snapshot()
	decision, err := e.flushBudget(
		ctx,
		segment,
		BudgetBeforeWork,
		checkpointPhasePreparing,
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
		PromptID:          promptID,
		Message:           renderWorkPrompt(segment, turn, kind),
		Kind:              kind,
		Owner:             segment.promptOwner(turn),
		UsageBaseTokens:   operationBase,
		UsageBaseReported: operationBaseReported,
		UsageReporter:     segment.usage.operationReporter(operationBase),
	}
	ticket, err := e.binder.PrepareActionPrompt(ctx, segment.binding, request)
	if err != nil {
		return e.handlePreparedPromptError(ctx, segment, err)
	}
	if strings.TrimSpace(ticket.PromptID) != promptID || strings.TrimSpace(ticket.QueueEntryID) == "" {
		return nil, fmt.Errorf("%w: managed Goal prompt ticket identity changed", loop.ErrValidation)
	}
	result, err := e.binder.AwaitActionPrompt(ctx, ticket)
	if err != nil {
		return e.recoverAwaitFailure(ctx, segment, ticket, err)
	}
	return e.processWorkResult(ctx, segment, ticket, result, operationBase)
}

func (segment *segmentState) promptOwner(turn int) loop.ActionPromptOwner {
	bindingEpoch := segment.binding.BindingEpoch
	if bindingEpoch < 1 {
		bindingEpoch = segment.checkpoint.BindingEpoch
	}
	return loop.ActionPromptOwner{
		LoopRunID:     segment.key.LoopRunID,
		TaskRunID:     segment.input.CorrelationID,
		Generation:    segment.key.Generation,
		NodeID:        segment.key.NodeID,
		ItemIndex:     segment.key.ItemIndex,
		Turn:          turn,
		PromptAttempt: segment.checkpoint.PromptAttempt,
		ControlEpoch:  segment.checkpoint.ControlEpoch,
		BindingEpoch:  bindingEpoch,
	}
}

func (e *Executor) processWorkResult(
	ctx context.Context,
	segment *segmentState,
	ticket loop.ActionPromptTicket,
	result loop.ActionPromptResult,
	operationBase int64,
) (*turnBoundary, error) {
	postDecision, checkpoint, err := e.prepareWorkResult(
		ctx,
		segment,
		ticket,
		result,
		operationBase,
	)
	if err != nil {
		return nil, err
	}
	if promptControlWasRevoked(result) {
		boundary, _, err := e.routePromptOutcome(ctx, segment, result)
		return boundary, err
	}
	if matchingBlockedReport(checkpoint, result.PromptID) {
		return e.settleBlockedReport(ctx, segment, checkpoint, result)
	}
	if boundary, handled, err := e.routePromptOutcome(ctx, segment, result); handled || err != nil {
		return boundary, err
	}
	return e.routeCompletedStop(ctx, segment, result, postDecision)
}

func promptControlWasRevoked(result loop.ActionPromptResult) bool {
	return result.Outcome == loop.ActionPromptOutcomeAmbiguous &&
		result.ReasonCode == loop.ReasonCodeGoalControlRevokedInFlight
}

func (e *Executor) prepareWorkResult(
	ctx context.Context,
	segment *segmentState,
	ticket loop.ActionPromptTicket,
	result loop.ActionPromptResult,
	operationBase int64,
) (BudgetDecision, Checkpoint, error) {
	if err := validatePromptTerminal(ticket.PromptID, result); err != nil {
		return BudgetDecision{}, Checkpoint{}, err
	}
	segment.usage.observeOperation(operationBase, result.TokensUsed, result.TokensReported)
	segment.lastResult = result
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return BudgetDecision{}, Checkpoint{}, fmt.Errorf(
			"goal: load checkpoint after work prompt %q: %w",
			result.PromptID,
			err,
		)
	}
	segment.checkpoint = checkpoint
	turn := checkpoint.TurnsUsed
	if turn < 1 {
		turn = segment.promptOwner(1).Turn
	}
	decision, err := e.flushBudget(
		ctx,
		segment,
		BudgetAfterWork,
		checkpoint.Phase,
		turn,
		result.PromptID,
		operationBase,
	)
	if err != nil {
		return BudgetDecision{}, Checkpoint{}, err
	}
	return decision, checkpoint, nil
}

func (e *Executor) routePromptOutcome(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, bool, error) {
	switch result.Outcome {
	case loop.ActionPromptOutcomeBudgetFenced, loop.ActionPromptOutcomeControlFenced:
		control, controlErr := e.fencedPromptBoundary(ctx, segment, result)
		boundary := &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control}
		return boundary, true, controlErr
	case loop.ActionPromptOutcomeRejectedBeforeSubmit:
		boundary, err := e.retryRejectedPrompt(ctx, segment, result)
		return boundary, true, err
	case loop.ActionPromptOutcomeInvalidResult:
		boundary, err := e.settleFailureBoundary(ctx, segment, result, loop.ReasonCodeGoalStopReasonInvalid)
		return boundary, true, err
	case loop.ActionPromptOutcomeFailed:
		boundary, err := e.settleFailureBoundary(ctx, segment, result, loop.ReasonCodeGoalPromptRequestFailed)
		return boundary, true, err
	case loop.ActionPromptOutcomeAmbiguous:
		boundary, err := e.ambiguousPromptBoundary(ctx, segment, result)
		return boundary, true, err
	case loop.ActionPromptOutcomeCompleted:
		return nil, false, nil
	default:
		err := fmt.Errorf("%w: unknown managed Goal prompt outcome %q", loop.ErrValidation, result.Outcome)
		return nil, true, err
	}
}

func (e *Executor) settleBlockedReport(
	ctx context.Context,
	segment *segmentState,
	checkpoint Checkpoint,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	settled, err := e.completeTurn(ctx, segment, result, nil)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = settled
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionBlocked,
		goalStatusBlocked,
		loop.ReasonCodeGoalReportedBlocked,
		reportActor(checkpoint),
	)
	return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, err
}

func (e *Executor) routeCompletedStop(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	postDecision BudgetDecision,
) (*turnBoundary, error) {
	switch result.StopReason {
	case loop.ActionStopEndTurn, loop.ActionStopMaxTurnRequests:
		if !matchingCompleteReport(segment.checkpoint, result.PromptID) {
			boundary, paused, pauseErr := e.settlePromptForPendingPause(ctx, segment, result)
			if paused || pauseErr != nil {
				return boundary, pauseErr
			}
		}
		if !postDecision.Allowed {
			control, controlErr := e.budgetBoundary(ctx, segment, postDecision)
			return &turnBoundary{checkpoint: segment.checkpoint, result: result, control: control}, controlErr
		}
		return e.judgeWorkTurn(ctx, segment, result)
	case loop.ActionStopMaxTokens:
		settled, err := e.completeTurn(ctx, segment, result, nil)
		if err != nil {
			return nil, err
		}
		segment.checkpoint = settled
		if !postDecision.Allowed {
			control, controlErr := e.budgetBoundary(ctx, segment, postDecision)
			return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, controlErr
		}
		return e.recoverFromMaxTokens(ctx, segment, result)
	case loop.ActionStopRefusal:
		return e.settleFailureBoundary(ctx, segment, result, loop.ReasonCodeGoalAgentRefused)
	case loop.ActionStopCancelled:
		return e.settleCancelledBoundary(ctx, segment, result)
	default:
		return nil, fmt.Errorf("%w: Goal stop reason %q is not routable", loop.ErrValidation, result.StopReason)
	}
}

func (e *Executor) settlePromptForPendingPause(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, bool, error) {
	checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
	if err != nil {
		return nil, false, fmt.Errorf("goal: load checkpoint before settling pending pause: %w", err)
	}
	segment.checkpoint = checkpoint
	if !checkpointHasPendingPause(checkpoint) {
		return nil, false, nil
	}
	settled, err := e.completeTurn(ctx, segment, result, nil)
	if err != nil {
		return nil, true, err
	}
	segment.checkpoint = settled
	control, ok, err := e.pendingPauseBoundary(ctx, segment)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, true, fmt.Errorf("%w: settled Goal pause lost its durable actor", loop.ErrTransitionConflict)
	}
	return &turnBoundary{
		checkpoint: settled,
		result:     result,
		control:    control,
		completed:  true,
	}, true, nil
}

func (e *Executor) completeTurn(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	verdict *gate.Verdict,
) (Checkpoint, error) {
	bindingEpoch := segment.binding.BindingEpoch
	if bindingEpoch < 1 {
		bindingEpoch = segment.checkpoint.BindingEpoch
	}
	checkpoint, err := e.store.CompleteTurn(ctx, CompleteTurnRequest{
		Key:                  segment.key,
		ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
		ExpectedBindingEpoch: bindingEpoch,
		TaskRunID:            segment.input.CorrelationID,
		QueueEntryID:         segment.checkpoint.QueueEntryID,
		PromptID:             result.PromptID,
		Result:               result,
		Verdict:              verdict,
		DispatchActorKind:    string(segment.input.Actor.Actor.Kind),
		DispatchActorID:      segment.input.Actor.Actor.Ref,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("goal: complete turn for prompt %q: %w", result.PromptID, err)
	}
	return checkpoint, nil
}

func matchingBlockedReport(checkpoint Checkpoint, promptID string) bool {
	return checkpoint.ReportIntent != nil && checkpoint.ReportIntent.Status == reportStatusBlocked &&
		checkpoint.ReportIntent.PromptID == strings.TrimSpace(promptID) &&
		checkpoint.ReportIntent.BindingEpoch == checkpoint.BindingEpoch &&
		strings.TrimSpace(checkpoint.ReportIntent.EvidenceRef) != ""
}

func matchingCompleteReport(checkpoint Checkpoint, promptID string) bool {
	return checkpoint.ReportIntent != nil && checkpoint.ReportIntent.Status == reportStatusComplete &&
		checkpoint.ReportIntent.PromptID == strings.TrimSpace(promptID) &&
		checkpoint.ReportIntent.BindingEpoch == checkpoint.BindingEpoch
}
