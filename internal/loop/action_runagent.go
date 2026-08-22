package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// RunAgentActionExecutor binds an ACP session and prompts a profile-backed agent.
type RunAgentActionExecutor struct {
	binder ActionSessionBinder
}

type runAgentSessionBindInput struct {
	node          dsl.Node
	execution     ActionExecutionInput
	spec          dsl.RunAgentParams
	contractBlock string
}

// Execute renders the work-order prompt, binds a session, and validates structured output.
func (e *RunAgentActionExecutor) Execute(
	ctx context.Context,
	node dsl.Node,
	in ActionExecutionInput,
) (ActionRawResult, error) {
	if e == nil || e.binder == nil {
		return ActionRawResult{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "session_binder"},
		)
	}
	params, err := actionParamsExcept(node, in, map[string]struct{}{
		outputSchemaParamKey: {},
	})
	if err != nil {
		return ActionRawResult{}, err
	}
	var spec dsl.RunAgentParams
	if err := dsl.NodeParams(params).Decode(&spec); err != nil {
		return ActionRawResult{}, fmt.Errorf("decode run-agent params: %w", err)
	}
	spec.Prompt, err = runAgentPromptWithRetryFeedback(spec.Prompt, in.Attempt, in.RetryFailure)
	if err != nil {
		return ActionRawResult{}, err
	}
	spec.Prompt, err = runAgentPromptWithRepairFeedback(spec.Prompt, in.Generation, in.RepairFailures)
	if err != nil {
		return ActionRawResult{}, err
	}
	spec.Prompt, err = runAgentPromptWithDeathResume(spec.Prompt, in.DeathResume)
	if err != nil {
		return ActionRawResult{}, err
	}
	spec.Prompt, err = runAgentPromptWithOutputContract(spec.Prompt, spec.OutputSchema)
	if err != nil {
		return ActionRawResult{}, err
	}
	runCtx, cancelRun, err := actionContextWithNodeTimeout(ctx, node.Timeout)
	if err != nil {
		return ActionRawResult{}, err
	}
	defer cancelRun()
	_, cancelOnDeadline := runCtx.Deadline()
	contract := dsl.Contract{}
	if in.Contract != nil {
		contract = *in.Contract
	}
	contractBlock := RenderContractBlock(contract)
	binding, resolvedRuntime, err := e.bindRunAgentSession(runCtx, node, in, spec, contractBlock)
	if err != nil {
		return ActionRawResult{}, err
	}
	first, err := e.promptActionSession(runCtx, binding, spec.Prompt, 0, cancelOnDeadline)
	if err != nil {
		return ActionRawResult{}, err
	}
	tokensUsed := first.TokensUsed
	structured, err := validateRunAgentStructured(spec.OutputSchema, first)
	if err == nil {
		first.TokensUsed = tokensUsed
		return rawFromPromptResult(binding, first, structured, resolvedRuntime), nil
	}
	if !errors.Is(err, ErrActionInvalidOutput) {
		return ActionRawResult{}, err
	}
	retryPrompt, retryErr := schemaRetryPrompt(spec.Prompt, spec.OutputSchema, err)
	if retryErr != nil {
		return ActionRawResult{}, retryErr
	}
	second, retryPromptErr := e.promptActionSession(runCtx, binding, retryPrompt, tokensUsed, cancelOnDeadline)
	if retryPromptErr != nil {
		return ActionRawResult{}, retryPromptErr
	}
	tokensUsed += second.TokensUsed
	structured, retryValidationErr := validateRunAgentStructured(spec.OutputSchema, second)
	if retryValidationErr != nil {
		return ActionRawResult{}, retryValidationErr
	}
	second.TokensUsed = tokensUsed
	return rawFromPromptResult(binding, second, structured, resolvedRuntime), nil
}

func (e *RunAgentActionExecutor) bindRunAgentSession(
	ctx context.Context,
	node dsl.Node,
	in ActionExecutionInput,
	spec dsl.RunAgentParams,
	contractBlock string,
) (ActionSessionBinding, ResolvedRuntime, error) {
	bindInput := runAgentSessionBindInput{
		node: node, execution: in, spec: spec, contractBlock: contractBlock,
	}
	resolvedRuntime, err := resolveRunAgentRuntime(ctx, &bindInput)
	if err != nil {
		return ActionSessionBinding{}, ResolvedRuntime{}, err
	}
	request, err := runAgentSessionBindRequest(&bindInput, resolvedRuntime)
	if err != nil {
		return ActionSessionBinding{}, ResolvedRuntime{}, err
	}
	binding, err := e.binder.BindActionSession(ctx, request)
	if err != nil {
		return ActionSessionBinding{}, ResolvedRuntime{}, fmt.Errorf("bind run-agent session: %w", err)
	}
	resolvedRuntime = appliedResolvedRuntime(
		resolvedRuntime,
		binding.AppliedRuntime,
		binding.SpeedResolution,
	)
	runtimeSelection := in.RuntimeSelectionOrZero()
	if runtimeSelection.Recorder != nil {
		if err := runtimeSelection.Recorder.RecordAppliedRuntime(
			ctx,
			in.WorkspaceID,
			in.LoopRunID,
			in.Generation,
			in.NodeID,
			in.ItemIndex,
			resolvedRuntime,
		); err != nil {
			return ActionSessionBinding{}, ResolvedRuntime{}, fmt.Errorf("persist applied runtime: %w", err)
		}
	}
	ReportActionSessionBound(ctx, binding.SessionID)
	return binding, resolvedRuntime, nil
}

func resolveRunAgentRuntime(
	ctx context.Context,
	input *runAgentSessionBindInput,
) (ResolvedRuntime, error) {
	runtimeSelection := input.execution.RuntimeSelectionOrZero()
	item, err := ItemRuntimeFromNamespace(input.execution.Namespace, input.node.Params, input.spec.Runtime)
	if err != nil {
		return ResolvedRuntime{}, err
	}
	resolved, err := ResolveItemRuntime(RuntimeLayers{
		Defaults:    runtimeSelection.Defaults.Worker,
		ConfigRules: runtimeSelection.ConfigRules,
		RunRules:    runtimeSelection.RunRules,
	}, item)
	if err != nil {
		return ResolvedRuntime{}, err
	}
	return ValidateResolvedRuntime(ctx, runtimeSelection.Catalog, item.TaskID, resolved)
}

func runAgentSessionBindRequest(
	input *runAgentSessionBindInput,
	resolvedRuntime ResolvedRuntime,
) (ActionSessionBindRequest, error) {
	in := input.execution
	handle := actionSessionHandle(input.node.Session)
	sharedKey := strings.TrimSpace(in.SessionSharedKey)
	if sharedKey == "" {
		sharedKey = actionSessionSharedKey(in.Generation, in.NodeID, in.ItemIndex, handle)
	}
	environment, err := ResolveActionEnvironment(input.spec.Environment, in.EnvironmentValue())
	if err != nil {
		return ActionSessionBindRequest{}, err
	}
	runtimeRequest := resolvedRuntime.Runtime
	return ActionSessionBindRequest{
		WorkspaceID: in.WorkspaceID, LoopRunID: in.LoopRunID, Generation: in.Generation,
		NodeID: in.NodeID, Agent: strings.TrimSpace(input.spec.Agent),
		Environment: cloneEnvironmentSpec(environment), Handle: handle, SharedKey: sharedKey,
		ItemIndex: in.ItemIndex, TargetBindingEpoch: in.CellEpoch + 1,
		ProvenanceParentSessionID: strings.TrimSpace(in.ProvenanceParentSessionID),
		CellFence: &ActionSessionCellFence{
			Epoch: in.CellEpoch, TaskRunID: strings.TrimSpace(in.CorrelationID),
		},
		Isolated:             input.node.Session != nil && input.node.Session.Isolated,
		Runtime:              &runtimeRequest,
		AllowedTools:         append([]string(nil), input.spec.AllowedTools...),
		MaxTurns:             input.spec.MaxTurns,
		ContractBlock:        input.contractBlock,
		NetworkParticipation: in.NetworkParticipation,
	}, nil
}

// Harvest returns the run-agent prompt result.
func (e *RunAgentActionExecutor) Harvest(_ context.Context, raw ActionRawResult, node dsl.Node) (ActionOutput, error) {
	if err := validateSyncHarvest(node); err != nil {
		return ActionOutput{}, err
	}
	return outputFromRaw(raw)
}

func (e *RunAgentActionExecutor) promptActionSession(
	ctx context.Context,
	binding ActionSessionBinding,
	message string,
	tokensUsedBase int64,
	cancelOnDeadline bool,
) (ActionPromptResult, error) {
	req := ActionPromptRequest{
		Message:       message,
		UsageReporter: cumulativeActionUsageReporter(ctx, tokensUsedBase),
	}
	deadline, ok := ctx.Deadline()
	if !cancelOnDeadline || !ok {
		return e.binder.PromptActionSession(ctx, binding, req)
	}
	timeout := max(time.Until(deadline), 0)
	timeoutCancel := make(chan error, 1)
	cancelSession := func() error {
		cancelCtx, cancelCancel := context.WithTimeout(context.WithoutCancel(ctx), actionCancelMaxWait)
		defer cancelCancel()
		return e.binder.CancelActionSession(cancelCtx, binding)
	}
	timer := time.AfterFunc(timeout, func() {
		timeoutCancel <- cancelSession()
	})
	result, err := e.binder.PromptActionSession(ctx, binding, req)
	if timer.Stop() {
		if ctx.Err() == nil {
			return result, err
		}
		timeoutCancel <- cancelSession()
	}
	var cancelErr error
	select {
	case cancelErr = <-timeoutCancel:
	case <-time.After(actionCancelMaxWait + actionCancelWaitGrace):
		cancelErr = fmt.Errorf("cancel run-agent session: %w", context.DeadlineExceeded)
	}
	return result, errors.Join(
		reasonError(
			ReasonCodeActionTimeout,
			ErrActionTimeout,
			map[string]string{watchEventsFieldSessionID: binding.SessionID},
		),
		err,
		cancelErr,
	)
}

func cumulativeActionUsageReporter(ctx context.Context, base int64) ActionUsageReporter {
	reporter := actionUsageReporterFromContext(ctx)
	if reporter == nil {
		return nil
	}
	return ActionUsageReporterFunc(func(tokensUsed int64) {
		if tokensUsed <= 0 {
			return
		}
		reporter.ReportActionTokensUsed(base + tokensUsed)
	})
}

func rawFromPromptResult(
	binding ActionSessionBinding,
	result ActionPromptResult,
	structured json.RawMessage,
	resolvedRuntime ResolvedRuntime,
) ActionRawResult {
	return ActionRawResult{
		Structured:      cloneRawMessage(structured),
		Text:            result.Text,
		SessionID:       binding.SessionID,
		EventStartSeq:   result.EventStartSeq,
		EventEndSeq:     result.EventEndSeq,
		TokensUsed:      result.TokensUsed,
		ResolvedRuntime: &resolvedRuntime,
	}
}
