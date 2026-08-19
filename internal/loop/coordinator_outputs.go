package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) refreshGenerationOutputs(
	ctx context.Context,
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	existing []GenerationOutput,
) ([]GenerationOutput, *GenerationOutput, *task.CoordinatorTerminal, bool, []task.CoordinatorStopSpec, error) {
	outputs := generationOutputMap(existing)
	for key, output := range outputs {
		if output.Attempt == 0 {
			output.Attempt = 1
		}
		if output.ExpectedEpoch == nil {
			expectedEpoch := output.Epoch
			output.ExpectedEpoch = &expectedEpoch
		}
		outputs[key] = output
	}
	for _, node := range graph.Nodes {
		if _, inFanOut := topology.inFanOutBody(node.ID); inFanOut {
			continue
		}
		key := generationOutputKey{nodeID: string(node.ID), itemIndex: 0}
		output, ok := outputs[key]
		if !ok {
			output = GenerationOutput{
				Generation: generation,
				NodeID:     string(node.ID),
				ItemIndex:  0,
				Status:     generationOutputPending,
				Attempt:    1,
			}
		}
		output.Generation = generation
		outputs[key] = output
	}
	live := false
	loopStops := make([]task.CoordinatorStopSpec, 0)
	controlTerminals := make([]goalControlCandidate, 0, 1)
	for key, output := range outputs {
		refreshed, isLive, stops, controlTerminal, err := r.refreshGenerationOutputFromTaskRun(
			ctx,
			run,
			graph,
			output,
		)
		if err != nil {
			return nil, nil, nil, false, nil, err
		}
		outputs[key] = refreshed
		loopStops = append(loopStops, stops...)
		if isLive {
			live = true
		}
		if controlTerminal != nil {
			controlTerminals = append(controlTerminals, goalControlCandidate{
				key:      key,
				terminal: *controlTerminal,
			})
		}
	}
	if live {
		keepDeferredGoalTerminalsPending(outputs, controlTerminals)
	}
	ordered := make([]GenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		ordered = append(ordered, output)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].NodeID == ordered[j].NodeID {
			return ordered[i].ItemIndex < ordered[j].ItemIndex
		}
		return ordered[i].NodeID < ordered[j].NodeID
	})
	failed := selectFailedOutput(ordered)
	return ordered, failed, selectGoalControlTerminal(controlTerminals), live, loopStops, nil
}

func keepDeferredGoalTerminalsPending(
	outputs map[generationOutputKey]GenerationOutput,
	candidates []goalControlCandidate,
) {
	for _, candidate := range candidates {
		if candidate.terminal.Status != string(StatusBlocked) &&
			candidate.terminal.Status != string(StatusExhausted) {
			continue
		}
		output := outputs[candidate.key]
		output.Status = generationOutputControlPending
		setGenerationOutputRef(&output, "")
		outputs[candidate.key] = output
	}
}

func selectFailedOutput(outputs []GenerationOutput) *GenerationOutput {
	var first *GenerationOutput
	for idx := range outputs {
		output := outputs[idx]
		if output.Status != generationOutputFailed {
			continue
		}
		if classifyGenerationOutputFailure(output, task.Run{}).Class == FailureTargetUnavailable {
			selected := output
			return &selected
		}
		if first == nil {
			selected := output
			first = &selected
		}
	}
	return first
}

func (r *CoordinatorRunner) refreshGenerationOutputFromTaskRun(
	ctx context.Context,
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, *task.CoordinatorTerminal, error) {
	switch output.Status {
	case generationOutputRetrying:
		return r.refreshRetryingGenerationOutput(graph, output)
	case generationOutputAwaitingChild:
		refreshed, live, stops, err := r.refreshAwaitingChildOutput(ctx, parent, graph, output)
		return refreshed, live, stops, nil, err
	}
	if generationOutputSettledWithoutTaskRun(output) {
		return output, false, nil, nil, nil
	}
	taskRunID := strings.TrimSpace(output.TaskRunID)
	if taskRunID == "" {
		if output.Status == generationOutputControlPending || output.Status == generationOutputAwaitingGoal {
			return GenerationOutput{}, false, nil, nil, invalidActionControlError("task_run_id", "")
		}
		return output, output.Status == generationOutputRunning ||
			output.Status == generationOutputEnqueued, nil, nil, nil
	}
	run, err := r.taskRuns.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return GenerationOutput{}, false, nil, nil, err
	}
	refreshed, live, stops, terminal, err := refreshGenerationOutputFromTaskStatus(parent, graph, output, run)
	if err != nil || refreshed.Status != generationOutputAwaitingChild {
		return refreshed, live, stops, terminal, err
	}
	refreshed, childLive, childStops, err := r.refreshAwaitingChildOutput(ctx, parent, graph, refreshed)
	return refreshed, live || childLive, append(stops, childStops...), terminal, err
}

func (r *CoordinatorRunner) refreshRetryingGenerationOutput(
	graph dsl.Graph,
	output GenerationOutput,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, *task.CoordinatorTerminal, error) {
	if output.NextAttemptAt == nil {
		return GenerationOutput{}, false, nil, nil, fmt.Errorf(
			"%w: retrying output %s/%d has no next_attempt_at",
			ErrValidation,
			output.NodeID,
			output.ItemIndex,
		)
	}
	now := r.now().UTC()
	if now.Before(output.NextAttemptAt.UTC()) {
		return output, true, nil, nil, nil
	}
	node, found := graphNode(graph, dsl.NodeID(output.NodeID))
	if found && nodeDeadlineExceededAt(node, output, now) {
		failure := NewActionFailure(
			string(FailureBudgetExhausted),
			"node deadline elapsed before its retry became runnable",
			"Review the node deadline and retry policy before re-running the loop.",
		)
		output.Status = generationOutputFailed
		output.NextAttemptAt = nil
		if failureRef, ok := ActionFailureOutputRef(failure); ok {
			setGenerationOutputRef(&output, failureRef)
		}
		return output, false, nil, nil, nil
	}
	output.Status = generationOutputPending
	output.Attempt++
	output.NextAttemptAt = nil
	output.TaskRunID = ""
	setGenerationOutputRef(&output, "")
	return output, false, nil, nil, nil
}

func generationOutputSettledWithoutTaskRun(output GenerationOutput) bool {
	if output.Status == generationOutputSucceeded {
		return output.FirstScheduledAt == nil || strings.TrimSpace(output.TaskRunID) == "" ||
			outputRefRepresentsAbsentValue(output.OutputRef)
	}
	switch output.Status {
	case generationOutputFailed, generationOutputCanceled, generationOutputQuarantined,
		generationOutputWaiting, generationOutputPaused, generationOutputPending:
		return true
	default:
		return false
	}
}

func refreshGenerationOutputFromTaskStatus(
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
	run task.Run,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, *task.CoordinatorTerminal, error) {
	switch run.Status.Normalize() {
	case task.TaskRunStatusCompleted:
		return refreshCompletedTaskRunOutput(parent, graph, output, run)
	case task.TaskRunStatusFailed, task.TaskRunStatusCanceled:
		output.Status = generationOutputFailed
		if output.OutputRef == "" {
			reasonCode := failureReasonCode(run.Error)
			if reasonCode != "" {
				failure := NewActionFailure(reasonCode, reasonCode, "")
				if failureRef, ok := ActionFailureOutputRef(failure); ok {
					setGenerationOutputRef(&output, failureRef)
				}
			}
		}
		return output, false, nil, nil, nil
	case task.TaskRunStatusQueued:
		output.Status = generationOutputEnqueued
		return output, true, nil, nil, nil
	case task.TaskRunStatusClaimed, task.TaskRunStatusStarting, task.TaskRunStatusRunning:
		output.Status = generationOutputRunning
		return output, true, nil, nil, nil
	default:
		output.Status = generationOutputRunning
		return output, true, nil, nil, nil
	}
}

func refreshCompletedTaskRunOutput(
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
	run task.Run,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, *task.CoordinatorTerminal, error) {
	if output.Status == generationOutputControlPending || output.Status == generationOutputAwaitingGoal {
		refreshed, terminal, err := resolveGoalActionControl(parent, output, run)
		return refreshed, false, nil, terminal, err
	}
	payload := run.Result
	runtimeRef := generationOutputRuntimePayload(output)
	if len(payload) == 0 && runtimeRef != "" && !OutputRefLooksContentAddressed(runtimeRef) {
		payload = json.RawMessage(runtimeRef)
	}
	node, found := graphNode(graph, dsl.NodeID(output.NodeID))
	if !found {
		return invalidCompletedActionOwnerOutput(output), false, nil, nil, nil
	}
	if failure := completedRunAgentOutputFailure(node, payload); failure != nil {
		output.Status = generationOutputFailed
		if failureRef, ok := ActionFailureOutputRef(*failure); ok {
			setGenerationOutputRef(&output, failureRef)
		}
		return output, false, nil, nil, nil
	}
	inspection := InspectPayloadFailure(payload, nodeResultContract(node))
	if inspection.Failure != nil {
		output.Status = generationOutputFailed
		if failureRef, ok := ActionFailureOutputRef(*inspection.Failure); ok {
			setGenerationOutputRef(&output, failureRef)
		}
		return output, false, nil, nil, nil
	}
	if refreshed, handled := refreshCompletedRunLoopOutput(node, output, payload); handled {
		return refreshed, false, nil, nil, nil
	}
	output.Status = generationOutputSucceeded
	return output, false, nil, nil, nil
}

func nodeDeadlineExceededAt(node dsl.Node, output GenerationOutput, now time.Time) bool {
	deadlineSpec := ""
	if node.NodeLifecycleState != nil {
		deadlineSpec = node.Deadline
	}
	if output.FirstScheduledAt == nil || strings.TrimSpace(deadlineSpec) == "" {
		return false
	}
	deadline, err := time.ParseDuration(strings.TrimSpace(deadlineSpec))
	if err != nil || deadline <= 0 {
		return false
	}
	return !now.UTC().Before(output.FirstScheduledAt.UTC().Add(deadline))
}

func nodeResultContract(node dsl.Node) *dsl.ResultContract {
	if node.NodeLifecycleState == nil {
		return nil
	}
	return node.ResultContract
}

func (r *CoordinatorRunner) refreshAwaitingChildOutput(
	ctx context.Context,
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, error) {
	childRunID := output.ChildLoopRunID
	if childRunID == "" {
		output.Status = generationOutputFailed
		setGenerationOutputRef(&output, "child_loop_missing")
		return output, false, nil, nil
	}
	if strings.TrimSpace(childRunID) != childRunID {
		return invalidAwaitedChildIdentityOutput(output), false, nil, nil
	}
	child, err := r.store.GetLoopRunByID(ctx, RunID(childRunID))
	if err != nil {
		return GenerationOutput{}, false, nil, err
	}
	if child.WorkspaceID != parent.WorkspaceID || child.ParentLoopRunID != parent.ID {
		return invalidAwaitedChildBoundaryOutput(output, child.ID), false, nil, nil
	}
	if child.Status.Terminal() {
		switch child.Status {
		case StatusDone, StatusNoOp:
			setGenerationOutputRef(&output, childLoopStatusRef(child.Status))
			output.Status = generationOutputSucceeded
		default:
			setGenerationOutputRef(&output, childLoopFailureRef(child))
			output.Status = generationOutputFailed
		}
		return output, false, nil, nil
	}
	timedOut, err := r.awaitingChildTimedOut(parent, graph, output, child)
	if err != nil {
		return GenerationOutput{}, false, nil, err
	}
	if !timedOut {
		return output, true, nil, nil
	}
	output.Status = generationOutputFailed
	setGenerationOutputRef(&output, childLoopTimeoutReason)
	return output, false, []task.CoordinatorStopSpec{{
		LoopRunID:  childRunID,
		ReasonCode: childLoopTimeoutReason,
	}}, nil
}

func (r *CoordinatorRunner) awaitingChildTimedOut(
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
	child Run,
) (bool, error) {
	node, ok := graphNode(graph, dsl.NodeID(output.NodeID))
	if !ok {
		return false, nil
	}
	timeoutSpec := strings.TrimSpace(node.Timeout)
	if timeoutSpec == "" {
		return false, nil
	}
	timeout, err := time.ParseDuration(timeoutSpec)
	if err != nil {
		return false, fmt.Errorf(
			"%w: node %q timeout is invalid: %w",
			ErrValidation,
			output.NodeID,
			err,
		)
	}
	if timeout <= 0 {
		return false, nil
	}
	startedAt := child.CreatedAt
	if startedAt.IsZero() {
		startedAt = parent.LastProgressAt
	}
	if startedAt.IsZero() {
		return false, nil
	}
	return !r.now().UTC().Before(startedAt.UTC().Add(timeout)), nil
}

func childLoopStatusRef(status Status) string {
	return "child_loop_status:" + string(status)
}

func childLoopFailureRef(child Run) string {
	failure := NewActionFailure(
		childLoopStatusRef(child.Status),
		"child loop closed with status "+string(child.Status),
		"Inspect the child Loop run before choosing a route or retry.",
	)
	failure.Target = string(child.ID)
	ref, ok := ActionFailureOutputRef(failure)
	if !ok {
		return childLoopStatusRef(child.Status)
	}
	return ref
}

type generationOutputKey struct {
	nodeID    string
	itemIndex int
}

func generationOutputMap(outputs []GenerationOutput) map[generationOutputKey]GenerationOutput {
	mapped := make(map[generationOutputKey]GenerationOutput, len(outputs))
	for _, output := range outputs {
		key := generationOutputKey{
			nodeID:    strings.TrimSpace(output.NodeID),
			itemIndex: output.ItemIndex,
		}
		mapped[key] = output
	}
	return mapped
}

func generationOutputIndexMap(outputs []GenerationOutput) map[generationOutputKey]int {
	indexes := make(map[generationOutputKey]int, len(outputs))
	for idx := range outputs {
		key := generationOutputKey{
			nodeID:    strings.TrimSpace(outputs[idx].NodeID),
			itemIndex: outputs[idx].ItemIndex,
		}
		indexes[key] = idx
	}
	return indexes
}

func graphNode(graph dsl.Graph, nodeID dsl.NodeID) (dsl.Node, bool) {
	for _, node := range graph.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return dsl.Node{}, false
}
