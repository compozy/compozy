package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
)

// Execute advances ordinary Goal turns serially until the first control boundary.
func (e *Executor) Execute(
	ctx context.Context,
	node dsl.Node,
	in loop.ActionExecutionInput,
) (loop.ActionRawResult, error) {
	segmentCtx, cancel, err := goalExecutionContext(ctx, node.Timeout)
	if err != nil {
		return loop.ActionRawResult{}, err
	}
	defer cancel()

	segment, err := e.initializeSegment(segmentCtx, node, in)
	if err != nil {
		return loop.ActionRawResult{}, err
	}
	for {
		boundary, err := e.advanceSegment(segmentCtx, segment)
		if err != nil {
			return loop.ActionRawResult{}, e.resolveExecutionError(segmentCtx, segment, err)
		}
		if boundary == nil || boundary.control == nil {
			continue
		}
		return e.rawResult(segment, boundary.control)
	}
}

func (e *Executor) initializeSegment(
	ctx context.Context,
	node dsl.Node,
	in loop.ActionExecutionInput,
) (*segmentState, error) {
	if err := validateGoalExecutionInput(e, node, in); err != nil {
		return nil, err
	}
	params, err := decodeGoalParams(node)
	if err != nil {
		return nil, err
	}
	key := TurnKey{
		WorkspaceID: in.WorkspaceID,
		LoopRunID:   in.LoopRunID,
		Generation:  in.Generation,
		NodeID:      node.ID,
		ItemIndex:   in.ItemIndex,
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	checkpoint, err := e.loadOrCreateCheckpoint(ctx, key, params, in)
	if err != nil {
		return nil, err
	}
	if checkpoint.ControlEpoch != in.GoalSegmentEpoch {
		return nil, fmt.Errorf(
			"%w: Goal segment epoch %d does not own checkpoint epoch %d",
			loop.ErrTransitionConflict,
			in.GoalSegmentEpoch,
			checkpoint.ControlEpoch,
		)
	}
	segment := &segmentState{
		input:      in,
		key:        key,
		node:       node,
		params:     params,
		checkpoint: checkpoint,
		usage:      newUsageTracker(in.PersistedTaskTokensUsed, in.UsageReporter),
	}
	if err := e.hydratePriorJudgeContext(ctx, segment); err != nil {
		return nil, err
	}
	if checkpoint.Phase == checkpointPhaseAwaitingControl || checkpoint.Phase == checkpointPhaseTerminal {
		return segment, nil
	}
	if err := e.bindSegment(ctx, segment); err != nil {
		return nil, err
	}
	return segment, nil
}

func validateGoalExecutionInput(e *Executor, node dsl.Node, in loop.ActionExecutionInput) error {
	if e == nil || e.store == nil || e.binder == nil || e.judge == nil || e.budget == nil ||
		e.context == nil || e.recovery == nil {
		return fmt.Errorf("%w: Goal executor is not fully initialized", loop.ErrActionDependencyMissing)
	}
	if dsl.ActionKind(node.Kind) != dsl.ActionGoal {
		return fmt.Errorf("%w: Goal executor received action kind %q", loop.ErrValidation, node.Kind)
	}
	if in.GoalSegmentEpoch < 1 || strings.TrimSpace(in.CorrelationID) == "" {
		return fmt.Errorf("%w: Goal segment epoch and task run ID are required", loop.ErrValidation)
	}
	return nil
}

func decodeGoalParams(node dsl.Node) (dsl.GoalParams, error) {
	var params dsl.GoalParams
	if err := node.Params.Decode(&params); err != nil {
		return dsl.GoalParams{}, fmt.Errorf("decode Goal params: %w", err)
	}
	if strings.TrimSpace(params.Objective) == "" || params.MaxTurns < 1 || len(params.Judge) == 0 {
		return dsl.GoalParams{}, fmt.Errorf(
			"%w: Goal objective, max_turns, and judge are required",
			loop.ErrValidation,
		)
	}
	return params, nil
}

func (e *Executor) loadOrCreateCheckpoint(
	ctx context.Context,
	key TurnKey,
	params dsl.GoalParams,
	in loop.ActionExecutionInput,
) (Checkpoint, error) {
	contextNudgeRatio, err := pinnedContextNudgeRatio(in)
	if err != nil {
		return Checkpoint{}, err
	}
	checkpoint, err := e.store.LoadCheckpoint(ctx, key)
	if err == nil {
		return checkpoint, nil
	}
	if !errors.Is(err, ErrCheckpointNotFound) {
		return Checkpoint{}, err
	}
	return e.store.CreateCheckpoint(ctx, CreateCheckpointRequest{Checkpoint: Checkpoint{
		Key:               key,
		ControlEpoch:      in.GoalSegmentEpoch,
		Phase:             checkpointPhaseIdle,
		Status:            goalStatusActive,
		TurnLimit:         params.MaxTurns,
		TaskRunID:         in.CorrelationID,
		ContextState:      contextStateUnknown,
		ContextNudgeRatio: contextNudgeRatio,
		UpdatedAt:         e.now().UTC(),
	}})
}

func pinnedContextNudgeRatio(in loop.ActionExecutionInput) (float64, error) {
	if in.GoalContextNudgeRatio == nil {
		return 0, fmt.Errorf("%w: Goal action requires a Run-pinned context nudge ratio", loop.ErrValidation)
	}
	policy := loop.GoalRunPolicy{ContextNudgeRatio: *in.GoalContextNudgeRatio}
	if err := policy.Validate(); err != nil {
		return 0, err
	}
	return policy.ContextNudgeRatio, nil
}

func (e *Executor) bindSegment(ctx context.Context, segment *segmentState) error {
	request, err := e.actionSessionBindRequest(segment)
	if err != nil {
		return err
	}
	maxAttempts := goalCreationAttempts(segment.node)
	for {
		binding, bindErr := e.binder.BindActionSession(ctx, request)
		if bindErr == nil {
			if strings.TrimSpace(binding.SessionID) == "" ||
				binding.ControlEpoch != segment.checkpoint.ControlEpoch || binding.BindingEpoch < 1 {
				return fmt.Errorf("%w: managed Goal binding identity is incomplete", loop.ErrValidation)
			}
			segment.binding = binding
			return nil
		}
		var creationErr *loop.ActionSessionCreationError
		if !errors.As(bindErr, &creationErr) || !creationErr.EffectKnownFalse {
			return fmt.Errorf("bind Goal session: %w", bindErr)
		}
		retryWithFreshSession := goalFreshSessionOnFailure(segment.node) &&
			segment.checkpoint.PromptAttempt+1 < maxAttempts
		if err := e.binder.AdvanceActionSessionRetry(ctx, &loop.ActionSessionRetryRequest{
			BindRequest:           request,
			FailedBinding:         binding,
			FailureCode:           creationErr.Code,
			ExpectedPromptAttempt: segment.checkpoint.PromptAttempt,
			RetryWithFreshSession: retryWithFreshSession,
		}); err != nil {
			return fmt.Errorf("advance Goal session retry: %w", err)
		}
		checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
		if err != nil {
			return err
		}
		segment.checkpoint = checkpoint
		if !retryWithFreshSession {
			segment.binding = binding
			_, err := e.checkpointBoundary(
				ctx,
				segment,
				loop.ActionDispositionNeedsApproval,
				goalStatusPaused,
				loop.ReasonCodeGoalPresubmitRetriesExhausted,
				automaticActor(),
			)
			return err
		}
		request.TargetBindingEpoch = checkpoint.BindingEpoch
		request.BindingAttemptID, request.DesiredSessionID = deterministicBindingIdentity(
			segment.key,
			request.Handle,
			checkpoint.BindingEpoch,
		)
	}
}

func (e *Executor) actionSessionBindRequest(
	segment *segmentState,
) (loop.ActionSessionBindRequest, error) {
	handleLabel := string(segment.node.ID)
	mode := dsl.SessionModeContinuous
	if segment.node.Session != nil {
		if value := strings.TrimSpace(segment.node.Session.Handle); value != "" {
			handleLabel = value
		}
		if value := strings.TrimSpace(segment.node.Session.Mode); value != "" {
			mode = value
		}
	}
	handle, err := dsl.DeriveGoalHandle(segment.node.ID, handleLabel, segment.key.ItemIndex)
	if err != nil {
		return loop.ActionSessionBindRequest{}, err
	}
	targetEpoch := max(segment.checkpoint.BindingEpoch, 1)
	bindingAttemptID, desiredSessionID := deterministicBindingIdentity(segment.key, handle, targetEpoch)
	contract := dsl.Contract{}
	if segment.input.Contract != nil {
		contract = *segment.input.Contract
	}
	return loop.ActionSessionBindRequest{
		WorkspaceID:                    segment.key.WorkspaceID,
		LoopRunID:                      segment.key.LoopRunID,
		Generation:                     segment.key.Generation,
		NodeID:                         segment.key.NodeID,
		ItemIndex:                      segment.key.ItemIndex,
		Agent:                          strings.TrimSpace(segment.params.Agent),
		CWD:                            strings.TrimSpace(segment.input.CWD),
		Handle:                         handle,
		Mode:                           mode,
		OriginSessionID:                strings.TrimSpace(segment.input.OriginSessionID),
		TargetBindingEpoch:             targetEpoch,
		ExpectedControlEpoch:           segment.checkpoint.ControlEpoch,
		ExpectedCheckpointPhase:        segment.checkpoint.Phase,
		ExpectedTaskRunID:              segment.checkpoint.TaskRunID,
		ExpectedQueueEntryID:           segment.checkpoint.QueueEntryID,
		ExpectedPromptID:               segment.checkpoint.PromptID,
		ExpectedCheckpointBindingEpoch: segment.checkpoint.BindingEpoch,
		ExpectedCheckpointSessionID:    segment.checkpoint.SessionID,
		ExpectedCheckpointHandle:       segment.checkpoint.BindingHandle,
		ReseedGrantID:                  activeReseedGrant(segment.checkpoint),
		BindingAttemptID:               bindingAttemptID,
		DesiredSessionID:               desiredSessionID,
		PinnedCreationProfileRef:       strings.TrimSpace(segment.input.OriginCreationProfileRef),
		PinnedCreationDigest:           strings.TrimSpace(segment.input.OriginCreationDigest),
		StaticPolicySpecDigest:         strings.TrimSpace(segment.input.OriginPolicySpecDigest),
		Isolated:                       segment.node.Session != nil && segment.node.Session.Isolated,
		Model:                          strings.TrimSpace(segment.input.WorkerModel),
		AllowedTools:                   append([]string(nil), segment.input.AllowedTools...),
		MaxTurns:                       segment.params.MaxTurns,
		ContractBlock:                  loop.RenderContractBlock(contract),
		NetworkParticipation:           segment.input.NetworkParticipation,
	}, nil
}

func goalCreationAttempts(node dsl.Node) int {
	if node.Retry == nil || node.Retry.MaxAttempts < 1 {
		return 1
	}
	return node.Retry.MaxAttempts
}

func goalFreshSessionOnFailure(node dsl.Node) bool {
	return node.Retry != nil && strings.TrimSpace(node.Retry.OnFailure) == dsl.RetryOnFailureFreshSession
}

func (e *Executor) rawResult(
	segment *segmentState,
	control *loop.ActionControl,
) (loop.ActionRawResult, error) {
	if err := control.Validate(); err != nil {
		return loop.ActionRawResult{}, err
	}
	structured := append([]byte(nil), segment.lastResult.Structured...)
	if control.Disposition == loop.ActionDispositionSucceeded && segment.params.OutputSchema != nil {
		result := segment.lastResult
		structuredResult, err := authoritativeGoalStructuredResult(result, control.GoalStatus)
		if err != nil {
			return loop.ActionRawResult{}, err
		}
		result.Structured = structuredResult
		validated, err := loop.ValidateActionStructured(*segment.params.OutputSchema, result)
		if err != nil {
			return loop.ActionRawResult{}, err
		}
		structured = validated
	}
	tokensUsed, _ := segment.usage.snapshot()
	return loop.ActionRawResult{
		Structured:    structured,
		Text:          segment.lastResult.Text,
		SessionID:     segment.binding.SessionID,
		EventStartSeq: segment.lastResult.EventStartSeq,
		EventEndSeq:   segment.lastResult.EventEndSeq,
		TokensUsed:    tokensUsed,
		Status:        control.GoalStatus,
		Control:       control,
	}, nil
}

func goalExecutionContext(
	ctx context.Context,
	rawTimeout string,
) (context.Context, context.CancelFunc, error) {
	trimmed := strings.TrimSpace(rawTimeout)
	if trimmed == "" {
		return ctx, func() {}, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil || timeout <= 0 {
		return nil, nil, fmt.Errorf("%w: invalid Goal action timeout %q", loop.ErrValidation, trimmed)
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	return child, cancel, nil
}
