package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) refreshRecoveredChildOutputs(
	ctx context.Context, parent Run, generation int, graph dsl.Graph,
	topology controlTopology, outputs []GenerationOutput,
) ([]GenerationOutput, error) {
	refreshedOutputs := outputs[:0]
	for _, output := range outputs {
		refreshed, err := r.refreshRecoveredChildOutput(ctx, parent, generation, graph, topology, outputs, output)
		if err != nil {
			return nil, err
		}
		refreshedOutputs = append(refreshedOutputs, refreshed)
	}
	return refreshedOutputs, nil
}

func (r *CoordinatorRunner) refreshRecoveredChildOutput(
	ctx context.Context, parent Run, generation int, graph dsl.Graph,
	topology controlTopology, outputs []GenerationOutput, output GenerationOutput,
) (GenerationOutput, error) {
	if output.Status != generationOutputFailed || output.ChildLoopRunID == "" ||
		!recoveredChildFailure(output) {
		return output, nil
	}
	node, found := graphNode(graph, dsl.NodeID(output.NodeID))
	if !found || node.Class != dsl.NodeClassAction || node.Kind != string(dsl.ActionRunLoop) {
		return output, nil
	}
	if strings.TrimSpace(output.ChildLoopRunID) != output.ChildLoopRunID {
		return invalidAwaitedChildIdentityOutput(output), nil
	}
	child, err := r.store.GetLoopRunByID(ctx, RunID(output.ChildLoopRunID))
	if err != nil {
		return GenerationOutput{}, err
	}
	if child.WorkspaceID != parent.WorkspaceID || child.ParentLoopRunID != parent.ID {
		return invalidAwaitedChildBoundaryOutput(output, child.ID), nil
	}
	if child.Historical || (child.Status != StatusDone && child.Status != StatusNoOp) {
		return output, nil
	}
	if !recoveredChildDependenciesSucceeded(topology, outputs, node.ID, output.ItemIndex) {
		return output, nil
	}
	current, err := r.recoveredChildCandidateMatches(
		ctx, parent, generation, graph, topology, outputs, node, output, child,
	)
	if err != nil || !current {
		return output, err
	}

	valid, err := r.recoveredChildReceiptMatches(ctx, parent, node, output)
	if err != nil || !valid {
		return output, err
	}

	refreshed, _, _, err := r.refreshAwaitingChildOutput(ctx, parent, graph, output)
	if err != nil {
		return GenerationOutput{}, err
	}
	return refreshed, nil
}

func (r *CoordinatorRunner) recoveredChildCandidateMatches(
	ctx context.Context, parent Run, generation int, graph dsl.Graph,
	topology controlTopology, outputs []GenerationOutput, node dsl.Node, output GenerationOutput, child Run,
) (bool, error) {
	// A terminal child is reusable only while the authored candidate still matches its inputs.
	history, err := r.readGenerationHistory(ctx, parent, generation)
	if err != nil {
		return false, err
	}
	namespace, err := runtimeNamespaceWithHistory(
		parent,
		generation,
		graph,
		topology,
		outputs,
		history,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return false, err
	}
	params, err := renderNodeParams(node, namespace)
	if err != nil {
		return false, err
	}
	var spec dsl.RunLoopParams
	if err := dsl.NodeParams(params).Decode(&spec); err != nil {
		return false, err
	}
	if spec.Mode == dsl.RunLoopDetach || spec.Loop != child.LoopName {
		return false, nil
	}
	requested, err := json.Marshal(spec.Inputs)
	if err != nil {
		return false, err
	}
	persisted, err := json.Marshal(child.Inputs)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(requested, persisted) && (len(spec.Inputs) > 0 || len(child.Inputs) > 0) {
		return false, nil
	}
	current, err := r.recoveredCandidateMatches(ctx, parent, child.Inputs)
	if err != nil {
		return false, err
	}
	if !current {
		return false, nil
	}
	return true, nil
}

func (r *CoordinatorRunner) recoveredChildReceiptMatches(
	ctx context.Context, parent Run, node dsl.Node, output GenerationOutput,
) (bool, error) {
	if output.TaskRunID == "" {
		return false, nil
	}
	taskRun, err := r.taskRuns.GetTaskRun(ctx, output.TaskRunID)
	if err != nil {
		return false, err
	}
	if taskRun.Status.Normalize() != task.TaskRunStatusCompleted || taskRun.LoopRunID != string(parent.ID) ||
		taskRun.TaskID != coordinatorNodeTaskID(parent.ID, output.Generation, node.ID, output.ItemIndex) {
		return false, nil
	}
	var result completedRunLoopResult
	if json.Unmarshal(taskRun.ResultValue(), &result) != nil || result.LoopRunID != output.ChildLoopRunID ||
		result.Status != generationOutputAwaitingChild {
		return false, nil
	}
	return true, nil
}

func recoveredChildDependenciesSucceeded(
	topology controlTopology,
	outputs []GenerationOutput,
	nodeID dsl.NodeID,
	itemIndex int,
) bool {
	visited := map[dsl.NodeID]bool{}
	pending := append([]dsl.NodeID(nil), topology.dependencies[nodeID]...)
	for len(pending) > 0 {
		dependency := pending[0]
		pending = pending[1:]
		if visited[dependency] {
			continue
		}
		visited[dependency] = true
		matched := false
		for _, output := range outputs {
			if output.NodeID != string(dependency) {
				continue
			}
			if topology.sameFanOutBody(nodeID, dependency) && output.ItemIndex != itemIndex {
				continue
			}
			matched = true
			if output.Status != generationOutputSucceeded {
				return false
			}
		}
		if !matched {
			return false
		}
		pending = append(pending, topology.dependencies[dependency]...)
	}
	return true
}

func recoveredChildFailure(output GenerationOutput) bool {
	payload := generationOutputRuntimePayload(output)
	if strings.HasPrefix(payload, "child_loop_status:") {
		return true
	}
	// The settled failure wire may omit its optional diagnostic target. The output cell and
	// completed canonical task receipt must still identify the same parent-linked child.
	var failure ActionFailure
	return json.Unmarshal([]byte(payload), &failure) == nil && failure.Kind == actionFailureKind &&
		strings.HasPrefix(failure.Code, "child_loop_status:") &&
		(failure.Target == "" || failure.Target == output.ChildLoopRunID)
}
