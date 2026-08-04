package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
)

type coordinatorActionRunContext struct {
	loopRun   Run
	resolved  *ResolvedDefinition
	effective EffectiveConfig
	node      dsl.Node
	meta      coordinatorActionRunMetadata
}

// CoordinatorControlKindLoopAction identifies a Loop-owned action-control completion envelope.
const CoordinatorControlKindLoopAction = "loop_action"

const coordinatorControlKindLoopAction = CoordinatorControlKindLoopAction

// ExecuteActionRun executes one queued loop action node run through the configured action registry.
func (r *CoordinatorRunner) ExecuteActionRun(
	ctx context.Context,
	taskRun task.Run,
	actor task.ActorContext,
) (task.RunResult, error) {
	if r == nil || r.actionRegistry == nil {
		return task.RunResult{}, fmt.Errorf(
			"%w: coordinator action registry is required",
			ErrActionDependencyMissing,
		)
	}
	if taskRun.RunKind.Normalize() != task.RunKindWorker {
		return task.RunResult{}, fmt.Errorf(
			"%w: task run %q is %q, not %q",
			ErrValidation,
			taskRun.ID,
			taskRun.RunKind.Normalize(),
			task.RunKindWorker,
		)
	}
	actionCtx, err := r.resolveActionRun(ctx, taskRun)
	if err != nil {
		return task.RunResult{}, err
	}
	outputs, err := r.readActionGenerationOutputs(ctx, &actionCtx)
	if err != nil {
		return task.RunResult{}, err
	}
	history, err := r.readGenerationHistory(ctx, actionCtx.loopRun, actionCtx.meta.Generation)
	if err != nil {
		return task.RunResult{}, err
	}
	input, err := actionExecutionInput(
		taskRun,
		actor,
		actionCtx.loopRun,
		actionCtx.resolved,
		actionCtx.effective,
		actionCtx.node,
		actionCtx.meta,
		outputs,
		history,
	)
	if err != nil {
		return task.RunResult{}, err
	}
	input.RetryFailure, err = r.actionRetryFailure(ctx, actionCtx.loopRun, actionCtx.meta)
	if err != nil {
		return task.RunResult{}, err
	}
	input.UsageReporter = actionUsageReporterFromContext(ctx)
	if input.RuntimeSelection == nil {
		input.RuntimeSelection = &ActionRuntimeSelection{}
	}
	if r.runtimeCatalog != nil {
		input.RuntimeSelection.Catalog, err = r.runtimeCatalog.ForWorkspace(ctx, actionCtx.loopRun.WorkspaceID)
		if err != nil {
			return task.RunResult{}, err
		}
	}
	if recorder, ok := r.outputs.(ActionAppliedRuntimeRecorder); ok {
		input.RuntimeSelection.Recorder = recorder
	}
	input.PersistedTaskTokensUsed = taskRun.TokensUsed
	executor, err := r.resolvePinnedActionExecutor(ctx, input.ToolScope, &actionCtx)
	if err != nil {
		return task.RunResult{}, err
	}
	if err := r.admitActionTarget(ctx, taskRun.ID, actionCtx.node, input); err != nil {
		return task.RunResult{}, err
	}
	return executeActionTaskRun(ctx, executor, actionCtx.node, input)
}

func (r *CoordinatorRunner) readActionGenerationOutputs(
	ctx context.Context,
	actionCtx *coordinatorActionRunContext,
) ([]GenerationOutput, error) {
	outputs, err := r.outputs.ListGenerationOutputs(
		ctx,
		actionCtx.loopRun.WorkspaceID,
		actionCtx.loopRun.ID,
		actionCtx.meta.Generation,
	)
	if err != nil {
		return nil, err
	}
	return generationOutputRuntimeView(ctx, r.outputs, generationOutputRuntimeScope{
		workspaceID: actionCtx.loopRun.WorkspaceID,
		runID:       actionCtx.loopRun.ID,
		generation:  actionCtx.meta.Generation,
	}, outputs)
}

func (r *CoordinatorRunner) resolvePinnedActionExecutor(
	ctx context.Context,
	scope tools.Scope,
	actionCtx *coordinatorActionRunContext,
) (ActionExecutor, error) {
	var pinnedSchema *ToolSchemaSnapshot
	if !dsl.IsReservedActionKind(actionCtx.node.Kind) {
		snapshot, ok := actionCtx.resolved.ToolSchemas[actionCtx.node.Kind]
		if !ok {
			return nil, reasonError(
				ReasonCodeActionContractStale,
				fmt.Errorf("%w: pinned action schema is missing", ErrActionSchemaInvalid),
				map[string]string{actionKindMetaKey: actionCtx.node.Kind},
			)
		}
		pinnedSchema = &snapshot
	}
	return r.actionRegistry.ResolvePinned(
		ctx,
		scope,
		actionCtx.node.Kind,
		pinnedSchema,
	)
}

func executeActionTaskRun(
	ctx context.Context,
	executor ActionExecutor,
	node dsl.Node,
	input ActionExecutionInput,
) (task.RunResult, error) {
	raw, err := executor.Execute(ctx, node, input)
	if err != nil {
		return task.RunResult{}, err
	}
	if raw.Control != nil {
		if err := raw.Control.Validate(); err != nil {
			return task.RunResult{}, err
		}
		if raw.Control.Disposition != ActionDispositionSucceeded {
			return actionControlTaskRunResult(raw)
		}
	}
	output, err := executor.Harvest(ctx, raw, node)
	if err != nil {
		return task.RunResult{}, err
	}
	payload, err := actionRunResultPayload(output)
	if err != nil {
		return task.RunResult{}, err
	}
	return task.RunResult{Value: payload, TokensUsed: raw.TokensUsed}, nil
}

func actionControlTaskRunResult(raw ActionRawResult) (task.RunResult, error) {
	payload, err := json.Marshal(raw.Control)
	if err != nil {
		return task.RunResult{}, fmt.Errorf("loop: marshal action coordinator control: %w", err)
	}
	return task.RunResult{
		TokensUsed: raw.TokensUsed,
		CoordinatorControl: &task.CoordinatorControlResult{
			Kind:    coordinatorControlKindLoopAction,
			Payload: payload,
		},
	}, nil
}

// ActionRunTimeout returns the parsed node timeout for one queued loop action run.
func (r *CoordinatorRunner) ActionRunTimeout(
	ctx context.Context,
	taskRun task.Run,
) (time.Duration, bool, error) {
	limit, err := r.actionRunLimit(ctx, taskRun)
	return limit.duration, limit.enabled, err
}

// ActionRunTimeoutReason returns the failure code selected by the same
// deterministic timeout/deadline calculation as ActionRunTimeout.
func (r *CoordinatorRunner) ActionRunTimeoutReason(
	ctx context.Context,
	taskRun task.Run,
) (string, error) {
	limit, err := r.actionRunLimit(ctx, taskRun)
	return limit.reasonCode, err
}

type actionRunLimit struct {
	duration   time.Duration
	enabled    bool
	reasonCode string
}

func (r *CoordinatorRunner) actionRunLimit(
	ctx context.Context,
	taskRun task.Run,
) (actionRunLimit, error) {
	actionCtx, err := r.resolveActionRun(ctx, taskRun)
	if err != nil {
		return actionRunLimit{}, err
	}
	outputs, err := r.outputs.ListGenerationOutputs(
		ctx, actionCtx.loopRun.WorkspaceID, actionCtx.loopRun.ID, actionCtx.meta.Generation,
	)
	if err != nil {
		return actionRunLimit{}, err
	}
	output, _ := generationOutputForActionMetadata(outputs, actionCtx.meta)
	return effectiveActionRunLimit(actionCtx.node, actionCtx.effective, output, r.now().UTC())
}

func effectiveActionRunLimit(
	node dsl.Node,
	effective EffectiveConfig,
	output GenerationOutput,
	now time.Time,
) (actionRunLimit, error) {
	timeout, err := parseActionTimeout(node.Timeout)
	if err != nil {
		return actionRunLimit{}, err
	}
	limit := actionRunLimit{
		duration: timeout, enabled: timeout > 0, reasonCode: string(ReasonCodeActionTimeout),
	}
	resolved, err := ResolveNodeLifecycleConfig(node, nil, effective.Lifecycle)
	if err != nil {
		return actionRunLimit{}, err
	}
	if resolved.Deadline == nil || output.FirstScheduledAt == nil {
		return limit, nil
	}
	remaining := output.FirstScheduledAt.UTC().Add(*resolved.Deadline).Sub(now.UTC())
	if remaining <= 0 {
		remaining = time.Nanosecond
	}
	if !limit.enabled || remaining <= limit.duration {
		limit.duration = remaining
		limit.enabled = true
		limit.reasonCode = string(FailureBudgetExhausted)
	}
	return limit, nil
}

func generationOutputForActionMetadata(
	outputs []GenerationOutput,
	meta coordinatorActionRunMetadata,
) (GenerationOutput, bool) {
	for _, output := range outputs {
		if output.NodeID == meta.NodeID && output.ItemIndex == meta.ItemIndex {
			return output, true
		}
	}
	return GenerationOutput{}, false
}

func (r *CoordinatorRunner) resolveActionRun(
	ctx context.Context,
	taskRun task.Run,
) (coordinatorActionRunContext, error) {
	meta, err := parseCoordinatorActionRunMetadata(taskRun.Metadata)
	if err != nil {
		return coordinatorActionRunContext{}, err
	}
	loopRunID := RunID(strings.TrimSpace(taskRun.LoopRunID))
	if loopRunID == "" {
		return coordinatorActionRunContext{}, fmt.Errorf(
			"%w: action task run has no loop_run_id",
			ErrValidation,
		)
	}
	loopRun, err := r.store.GetLoopRunByID(ctx, loopRunID)
	if err != nil {
		return coordinatorActionRunContext{}, err
	}
	resolved, effective, err := r.resolvedLoopForAction(ctx, loopRun)
	if err != nil {
		return coordinatorActionRunContext{}, err
	}
	node, err := actionNodeForRun(resolved.Definition.Graph, meta.NodeID)
	if err != nil {
		return coordinatorActionRunContext{}, err
	}
	if dsl.ActionKind(node.Kind) == dsl.ActionGoal && meta.GoalSegmentEpoch < 1 {
		return coordinatorActionRunContext{}, fmt.Errorf(
			"%w: Goal action task metadata requires goal_segment_epoch",
			ErrValidation,
		)
	}
	return coordinatorActionRunContext{
		loopRun:   loopRun,
		resolved:  resolved,
		effective: effective,
		node:      node,
		meta:      meta,
	}, nil
}

func actionExecutionInput(
	taskRun task.Run,
	actor task.ActorContext,
	loopRun Run,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	node dsl.Node,
	meta coordinatorActionRunMetadata,
	outputs []GenerationOutput,
	history GenerationHistory,
) (ActionExecutionInput, error) {
	topology := newControlTopology(resolved.Definition.Graph)
	namespace, err := runtimeNamespaceWithHistory(
		loopRun,
		meta.Generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		history,
		node.ID,
		meta.ItemIndex,
	)
	if err != nil {
		return ActionExecutionInput{}, err
	}
	input := ActionExecutionInput{
		WorkspaceID:    loopRun.WorkspaceID,
		LoopRunID:      loopRun.ID,
		Generation:     meta.Generation,
		NodeID:         node.ID,
		ItemIndex:      meta.ItemIndex,
		Attempt:        meta.Attempt,
		CellEpoch:      meta.Epoch,
		RepairFailures: actionRepairFailures(history),
		Namespace:      namespace,
		Contract:       &resolved.Definition.Contract,
		ToolScope:      actionToolScope(loopRun, actor),
		Actor:          actor,
		CorrelationID:  strings.TrimSpace(taskRun.ID),
		RuntimeSelection: &ActionRuntimeSelection{
			Defaults:    effective.RuntimeDefaults,
			ConfigRules: cloneRuntimeRules(effective.RuntimeRules),
			RunRules:    cloneRuntimeRules(effective.RunRuntimeRules),
		},
		OriginSessionID:          loopRun.Origin.SessionID,
		OriginCreationProfileRef: loopRun.Origin.CreationProfileRef,
		OriginPolicySpecDigest:   loopRun.Origin.PolicySpecDigest,
		OriginCreationDigest:     loopRun.Origin.CreationDigest,
		GoalContextNudgeRatio:    new(loopRun.GoalContextNudgeRatio),
		GoalSegmentEpoch:         meta.GoalSegmentEpoch,
		NetworkParticipation:     new(loopRun.NetworkSpecSnapshot()),
	}
	if meta.ContinuationKind == deathResumeContinuationKind {
		input.DeathResume = &DeathResumeContext{
			SourceTaskRunID: meta.ResumeFromTaskRunID,
			SourceSessionID: meta.ResumeFromSessionID,
			Checkpoint:      meta.DeathCheckpoint.Clone(),
		}
	}
	return input, nil
}

func (r *CoordinatorRunner) resolvedLoopForAction(
	ctx context.Context,
	loopRun Run,
) (*ResolvedDefinition, EffectiveConfig, error) {
	resolved, err := r.resolvePinnedDefinition(ctx, loopRun)
	if err != nil {
		return nil, EffectiveConfig{}, err
	}
	resolved, err = controlExecutionResolved(resolved)
	if err != nil {
		return nil, EffectiveConfig{}, err
	}
	effective, err := pinnedEffectiveConfig(resolved)
	if err != nil {
		return nil, EffectiveConfig{}, err
	}
	return coordinatorResolvedWithEffectiveConfig(resolved, effective), effective, nil
}

func actionNodeForRun(graph dsl.Graph, nodeID string) (dsl.Node, error) {
	node, ok := graphNode(graph, dsl.NodeID(nodeID))
	if !ok {
		return dsl.Node{}, fmt.Errorf("%w: action node %q not found", ErrValidation, nodeID)
	}
	if node.Class != dsl.NodeClassAction {
		return dsl.Node{}, fmt.Errorf(
			"%w: node %q is %q, not %q",
			ErrValidation,
			node.ID,
			node.Class,
			dsl.NodeClassAction,
		)
	}
	return node, nil
}

func actionToolScope(run Run, actor task.ActorContext) tools.Scope {
	return tools.Scope{
		WorkspaceID: string(run.WorkspaceID),
		ActorKind:   string(actor.Actor.Kind.Normalize()),
	}
}

func actionRunResultPayload(output ActionOutput) (json.RawMessage, error) {
	if len(bytes.TrimSpace(output.Structured)) > 0 {
		return cloneRawMessage(output.Structured), nil
	}
	if output.Value != nil {
		data, err := json.Marshal(output.Value)
		if err != nil {
			return nil, fmt.Errorf("loop: marshal action output value: %w", err)
		}
		return data, nil
	}
	payload := map[string]any{}
	if strings.TrimSpace(output.Text) != "" {
		payload["text"] = output.Text
	}
	if strings.TrimSpace(output.SessionID) != "" {
		payload["session_id"] = output.SessionID
	}
	if strings.TrimSpace(string(output.ChildLoopRunID)) != "" {
		payload[metadataLoopRunIDKey] = string(output.ChildLoopRunID)
	}
	if strings.TrimSpace(output.Status) != "" {
		payload["status"] = output.Status
	}
	if len(payload) == 0 {
		payload["status"] = "completed"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal action output payload: %w", err)
	}
	return data, nil
}
