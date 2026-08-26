package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
)

const (
	errorRoutedOutputRefPrefix = "error_routed:"
	failureAbsorbedOutputRef   = "failure_absorbed"
)

type nodeLifecycleResult struct {
	attempt  NodeAttempt
	events   []GenerationLifecycleEventIntent
	controls []NodeControlMutation
	handled  bool
	live     bool
}

func (r *CoordinatorRunner) applyNodeLifecyclePrecedence(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	outputs []GenerationOutput,
) ([]GenerationOutput, []NodeAttempt, []NodeControlMutation, []GenerationLifecycleEventIntent, bool, error) {
	attempts, err := r.listNodeAttempts(ctx, run)
	if err != nil {
		return nil, nil, nil, nil, false, err
	}
	nodeControls, err := r.listNodeControls(ctx, run)
	if err != nil {
		return nil, nil, nil, nil, false, err
	}
	controlByNode := nodeControlsByID(nodeControls)
	advanced := cloneGenerationOutputs(outputs)
	newAttempts := make([]NodeAttempt, 0)
	events := make([]GenerationLifecycleEventIntent, 0)
	controls := make([]NodeControlMutation, 0)
	graph := resolved.Definition.Graph
	topology := newResolvedControlTopology(resolved)
	history := GenerationHistory{}
	if r.targetHealth != nil {
		history, err = r.readGenerationHistory(ctx, run, generation)
		if err != nil {
			return nil, nil, nil, nil, false, err
		}
	}
	live := false
	for index := range advanced {
		if advanced[index].Status == generationOutputRetrying {
			live = true
			continue
		}
		result, applyErr := r.applyTerminalNodeLifecycle(
			ctx, run, generation, effective, graph, topology, advanced, index, attempts, history, controlByNode,
		)
		if applyErr != nil {
			return nil, nil, nil, nil, false, applyErr
		}
		if !result.handled {
			continue
		}
		newAttempts = append(newAttempts, result.attempt)
		events = append(events, result.events...)
		controls = append(controls, result.controls...)
		attempts = append(attempts, result.attempt)
		live = live || result.live
	}
	return advanced, newAttempts, controls, events, live, nil
}

func (r *CoordinatorRunner) applyTerminalNodeLifecycle(
	ctx context.Context,
	run Run,
	generation int,
	effective EffectiveConfig,
	graph dsl.Graph,
	topology controlTopology,
	advanced []GenerationOutput,
	index int,
	attempts []NodeAttempt,
	history GenerationHistory,
	controlByNode map[NodeID]NodeControl,
) (nodeLifecycleResult, error) {
	output := advanced[index]
	node, found := lifecycleActionNode(graph, output, generation, attempts)
	if !found {
		return nodeLifecycleResult{}, nil
	}
	defer r.discardTargetProbe(output.TaskRunID)
	targetInput := ActionExecutionInput{
		WorkspaceID: run.WorkspaceID,
		ToolScope:   tools.Scope{ProfileID: strings.TrimSpace(run.ProfileID)},
	}
	if r.targetHealth != nil {
		namespace, err := runtimeNamespaceWithHistory(
			run,
			generation,
			graph,
			topology,
			advanced,
			history,
			node.ID,
			output.ItemIndex,
		)
		if err != nil {
			return nodeLifecycleResult{}, err
		}
		targetInput.Namespace = namespace
	}
	taskRun, err := r.taskRuns.GetTaskRun(ctx, output.TaskRunID)
	if err != nil {
		return nodeLifecycleResult{}, fmt.Errorf(
			"loop: read task run %q for node lifecycle: %w",
			output.TaskRunID,
			err,
		)
	}
	attempt := nodeAttemptForTaskRun(run, generation, output, taskRun, r.now().UTC())
	if output.Status == generationOutputSucceeded {
		attempt.Disposition = AttemptSucceeded
		if errorPolicy := nodeErrorPolicy(node); errorPolicy != nil && errorPolicy.Route != "" {
			skipErrorRouteOnSuccess(graph, topology, node, output, &advanced)
		}
		events, err := r.accountActionTarget(ctx, run, node, targetInput, taskRun, nil, attempt.Disposition)
		if err != nil {
			return nodeLifecycleResult{}, err
		}
		return nodeLifecycleResult{attempt: attempt, events: events, handled: true}, nil
	}
	return r.applyFailedNodeLifecycle(
		ctx, run, effective, graph, topology, advanced, index, attempts,
		controlByNode, node, targetInput, taskRun, attempt,
	)
}

func lifecycleActionNode(
	graph dsl.Graph,
	output GenerationOutput,
	generation int,
	attempts []NodeAttempt,
) (dsl.Node, bool) {
	node, found := graphNode(graph, dsl.NodeID(output.NodeID))
	terminal := output.Status == generationOutputSucceeded || output.Status == generationOutputFailed
	if !found || !terminal || output.TaskRunID == "" || node.Class != dsl.NodeClassAction {
		return dsl.Node{}, false
	}
	if output.FirstScheduledAt == nil {
		return dsl.Node{}, false
	}
	return node, !nodeAttemptRecorded(attempts, generation, output)
}

func (r *CoordinatorRunner) applyFailedNodeLifecycle(
	ctx context.Context,
	run Run,
	effective EffectiveConfig,
	graph dsl.Graph,
	topology controlTopology,
	advanced []GenerationOutput,
	index int,
	attempts []NodeAttempt,
	controlByNode map[NodeID]NodeControl,
	node dsl.Node,
	targetInput ActionExecutionInput,
	taskRun task.Run,
	attempt NodeAttempt,
) (nodeLifecycleResult, error) {
	output := advanced[index]
	failure := classifyGenerationOutputFailure(output, taskRun)
	if failure.Target == "" {
		target, err := r.actionTargetFor(taskRun.ID, run, node, targetInput)
		if err != nil {
			return nodeLifecycleResult{}, err
		}
		failure.Target = target
	}
	pause, err := evaluateAutopause(node, failure, output.Attempt, effective)
	if err != nil {
		return nodeLifecycleResult{}, err
	}
	if pause.matched {
		result, applied := applyMatchedAutopause(
			pause, failure, controlByNode, output, attempt, advanced, index, r.now().UTC(),
		)
		if applied {
			return result, nil
		}
	}
	retryPlan, err := planNodeRetry(&nodeRetryPlanInput{
		node:        node,
		effective:   effective,
		output:      output,
		failure:     failure,
		attempts:    attemptsForOutput(attempts, attempt.Generation, output),
		now:         r.now().UTC(),
		randFloat64: r.retryRand,
	})
	if err != nil {
		return nodeLifecycleResult{}, err
	}
	failure = retryPlan.failure
	applyFailureToAttempt(&attempt, failure)
	if retryPlan.retry {
		attempt.Disposition = AttemptRetried
		attempt.NextAttemptAt = cloneTimePointer(retryPlan.nextAttemptAt)
		output.Status = generationOutputRetrying
		output.NextAttemptAt = cloneTimePointer(retryPlan.nextAttemptAt)
		output.Epoch++
		if err := validateRetrySchedule(output); err != nil {
			return nodeLifecycleResult{}, err
		}
		advanced[index] = output
		events, err := r.accountActionTarget(
			ctx, run, node, targetInput, taskRun, &failure, attempt.Disposition,
		)
		if err != nil {
			return nodeLifecycleResult{}, err
		}
		return nodeLifecycleResult{attempt: attempt, events: events, handled: true, live: true}, nil
	}
	attempt.Disposition = applyNodeFailureDisposition(graph, topology, node, output, advanced, index, failure)
	events, err := r.accountActionTarget(
		ctx, run, node, targetInput, taskRun, &failure, attempt.Disposition,
	)
	if err != nil {
		return nodeLifecycleResult{}, err
	}
	return nodeLifecycleResult{attempt: attempt, events: events, handled: true}, nil
}

func applyMatchedAutopause(
	pause autopauseDecision,
	failure ClassifiedFailure,
	controlByNode map[NodeID]NodeControl,
	output GenerationOutput,
	attempt NodeAttempt,
	advanced []GenerationOutput,
	index int,
	at time.Time,
) (nodeLifecycleResult, bool) {
	control, controlled := controlByNode[NodeID(output.NodeID)]
	if control.Paused {
		return nodeLifecycleResult{}, false
	}
	applyFailureToAttempt(&attempt, failure)
	attempt.Disposition = AttemptEscalated
	output.Status = generationOutputPaused
	output.Attempt++
	output.NextAttemptAt = nil
	output.Epoch++
	advanced[index] = output
	return nodeLifecycleResult{
		attempt: attempt,
		controls: []NodeControlMutation{{
			Kind: NodeControlMutationPause, NodeID: NodeID(output.NodeID),
			PauseActorKind: autopauseActorKind, PauseActorID: autopauseActorID,
			PauseReason: pause.reason, PauseRuleID: pause.ruleID, At: at,
			ExpectExisting: controlled, ExpectedRevision: control.Revision,
		}},
		events: []GenerationLifecycleEventIntent{{
			Kind: GenerationLifecycleEventNodePaused, NodeID: output.NodeID,
			ItemIndex: output.ItemIndex, Attempt: attempt.Attempt,
			ActorKind: autopauseActorKind, ActorID: autopauseActorID,
			RuleID: pause.ruleID, Reason: pause.reason,
		}},
		handled: true, live: true,
	}, true
}

func applyNodeFailureDisposition(
	graph dsl.Graph,
	topology controlTopology,
	node dsl.Node,
	output GenerationOutput,
	advanced []GenerationOutput,
	index int,
	failure ClassifiedFailure,
) AttemptDisposition {
	errorPolicy := nodeErrorPolicy(node)
	switch {
	case errorPolicy != nil && errorPolicy.Route != "":
		output.Status = generationOutputSucceeded
		setGenerationOutputRef(&output, errorRoutedOutputRefPrefix+string(errorPolicy.Route))
		output.NextAttemptAt = nil
		output.Epoch++
		advanced[index] = output
		skipErrorRouteSuccessPath(graph, topology, node, output, &advanced)
		return AttemptRouted
	case errorPolicy != nil && errorPolicy.AllowFail:
		output.Status = generationOutputSucceeded
		setGenerationOutputRef(&output, failureAbsorbedOutputRef)
		output.NextAttemptAt = nil
		output.Epoch++
		advanced[index] = output
		return AttemptAbsorbed
	default:
		if failureRef, ok := ActionFailureOutputRef(NewActionFailure(failure.Code, failure.Cause, failure.Hint)); ok {
			setGenerationOutputRef(&output, failureRef)
		}
		advanced[index] = output
		return AttemptEscalated
	}
}

func nodeErrorPolicy(node dsl.Node) *dsl.ErrorPolicy {
	if node.NodeLifecycleState == nil {
		return nil
	}
	return node.OnError
}

func (r *CoordinatorRunner) listNodeAttempts(ctx context.Context, run Run) ([]NodeAttempt, error) {
	if r.attempts == nil {
		return nil, nil
	}
	attempts, err := r.attempts.ListNodeAttempts(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("loop: list node attempts: %w", err)
	}
	return attempts, nil
}

func nodeAttemptRecorded(attempts []NodeAttempt, generation int, output GenerationOutput) bool {
	for _, attempt := range attempts {
		if attempt.Generation == generation && attempt.NodeID == NodeID(output.NodeID) &&
			attempt.ItemIndex == output.ItemIndex && attempt.Attempt == output.Attempt {
			return true
		}
	}
	return false
}

func attemptsForOutput(
	attempts []NodeAttempt,
	generation int,
	output GenerationOutput,
) []NodeAttempt {
	filtered := make([]NodeAttempt, 0)
	for _, attempt := range attempts {
		if attempt.Generation == generation && attempt.NodeID == NodeID(output.NodeID) &&
			attempt.ItemIndex == output.ItemIndex {
			filtered = append(filtered, attempt)
		}
	}
	return filtered
}

func nodeAttemptForTaskRun(
	run Run,
	generation int,
	output GenerationOutput,
	taskRun task.Run,
	now time.Time,
) NodeAttempt {
	startedAt := taskRun.StartedAt
	if startedAt.IsZero() {
		startedAt = taskRun.ClaimedAt
	}
	if startedAt.IsZero() {
		startedAt = taskRun.QueuedAt
	}
	if startedAt.IsZero() && output.FirstScheduledAt != nil {
		startedAt = *output.FirstScheduledAt
	}
	if startedAt.IsZero() {
		startedAt = now
	}
	endedAt := taskRun.EndedAt
	if endedAt.IsZero() {
		endedAt = now
	}
	return NodeAttempt{
		LoopRunID:  run.ID,
		Generation: generation,
		NodeID:     NodeID(output.NodeID),
		ItemIndex:  output.ItemIndex,
		Attempt:    output.Attempt,
		StartedAt:  startedAt.UTC(),
		EndedAt:    new(endedAt.UTC()),
	}
}

func applyFailureToAttempt(attempt *NodeAttempt, failure ClassifiedFailure) {
	if attempt == nil {
		return
	}
	failureClass := failure.Class
	attempt.FailureClass = &failureClass
	attempt.FailureCode = failure.Code
	attempt.Cause = failure.Cause
	attempt.Hint = failure.Hint
	attempt.Target = failure.Target
}

func classifyGenerationOutputFailure(output GenerationOutput, taskRun task.Run) ClassifiedFailure {
	runtimeRef := generationOutputRuntimePayload(output)
	failure := actionFailureFromOutputRef(runtimeRef)
	if failure.Code == "" {
		failure = NewActionFailure(
			string(FailurePayloadDeclared),
			firstNonEmpty(
				strings.TrimSpace(runtimeRef),
				strings.TrimSpace(taskRun.Error),
				"node execution failed",
			),
			"",
		)
	}
	evidence := FailureEvidence{
		Code:   failure.Code,
		Cause:  failure.Cause,
		Hint:   failure.Recovery,
		Target: failure.Target,
	}
	if targetUnavailableFailureCode(failure.Code) {
		evidence.TargetUnavailable = true
		return ClassifyNodeFailure(evidence)
	}
	switch failure.Code {
	case string(FailureCancellation), string(tools.ErrorCodeCanceled), childLoopStatusRef(StatusCanceled):
		evidence.CancelRequested = true
	case string(ReasonCodeActionTimeout), childLoopTimeoutReason:
		evidence.AttemptTimedOut = true
	case string(FailureBudgetExhausted):
		evidence.BudgetExhausted = true
	case string(tools.ErrorCodeUnavailable), string(tools.ErrorCodeBackendFailed),
		string(tools.ErrorCodeTimedOut), childLoopStatusRef(StatusFailed):
		evidence.Transport = true
	case string(ReasonCodeUnknownActionKind), string(ReasonCodeActionDependencyMissing),
		string(ReasonCodeInvalidOutput), string(ReasonCodeActionContractStale),
		string(ReasonCodeActionMaterializationFailed):
		evidence.Authoring = true
	default:
		evidence.PayloadFailure = &failure
	}
	return ClassifyNodeFailure(evidence)
}

func actionFailureFromOutputRef(outputRef string) ActionFailure {
	var failure ActionFailure
	if err := json.Unmarshal([]byte(strings.TrimSpace(outputRef)), &failure); err != nil {
		return ActionFailure{}
	}
	failure.Kind = strings.TrimSpace(failure.Kind)
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Cause = strings.TrimSpace(failure.Cause)
	failure.Recovery = strings.TrimSpace(failure.Recovery)
	failure.Target = strings.TrimSpace(failure.Target)
	if failure.Kind != actionFailureKind || failure.Code == "" || failure.Cause == "" {
		return ActionFailure{}
	}
	return failure
}

func targetUnavailableFailureCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "dependency_missing", "credential_missing", "resource_unreachable",
		string(FailureTargetUnavailable):
		return true
	default:
		return false
	}
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
