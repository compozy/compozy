package loop

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
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
		output.OutputRef = ""
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
		if explicitDependencyBlocker(output.OutputRef) {
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
	case generationOutputAwaitingChild:
		refreshed, live, stops, err := r.refreshAwaitingChildOutput(ctx, parent, graph, output)
		return refreshed, live, stops, nil, err
	case generationOutputSucceeded, generationOutputFailed, generationOutputPending:
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
	switch run.Status.Normalize() {
	case task.TaskRunStatusCompleted:
		if output.Status == generationOutputControlPending || output.Status == generationOutputAwaitingGoal {
			refreshed, terminal, controlErr := resolveGoalActionControl(parent, output, run)
			return refreshed, false, nil, terminal, controlErr
		}
		output.Status = generationOutputSucceeded
		return output, false, nil, nil, nil
	case task.TaskRunStatusFailed, task.TaskRunStatusCanceled:
		output.Status = generationOutputFailed
		if output.OutputRef == "" {
			output.OutputRef = failureReasonCode(run.Error)
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

func (r *CoordinatorRunner) refreshAwaitingChildOutput(
	ctx context.Context,
	parent Run,
	graph dsl.Graph,
	output GenerationOutput,
) (GenerationOutput, bool, []task.CoordinatorStopSpec, error) {
	childRunID := strings.TrimSpace(output.ChildLoopRunID)
	if childRunID == "" {
		output.Status = generationOutputFailed
		output.OutputRef = "child_loop_missing"
		return output, false, nil, nil
	}
	child, err := r.store.GetLoopRunByID(ctx, RunID(childRunID))
	if err != nil {
		return GenerationOutput{}, false, nil, err
	}
	if child.Status.Terminal() {
		output.OutputRef = childLoopStatusRef(child.Status)
		switch child.Status {
		case StatusDone, StatusNoOp:
			output.Status = generationOutputSucceeded
		default:
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
	output.OutputRef = childLoopTimeoutReason
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
