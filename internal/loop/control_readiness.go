package loop

import (
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
)

func dependenciesSucceededForOutput(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	output GenerationOutput,
) bool {
	nodeID := dsl.NodeID(output.NodeID)
	if fanOutID, ok := topology.isCollectForFanOut(nodeID); ok {
		return collectDependenciesSucceeded(graph, topology, outputs, fanOutID, output)
	}
	outputMap := generationOutputMap(outputs)
	for _, dependency := range topology.dependencies[nodeID] {
		dependencyOutput, ok := dependencyOutputForNode(topology, outputMap, nodeID, dependency, output.ItemIndex)
		if !ok || dependencyOutput.Status != generationOutputSucceeded {
			return false
		}
	}
	return true
}

func dependencyOutputForNode(
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
	dependency dsl.NodeID,
	itemIndex int,
) (GenerationOutput, bool) {
	if topology.sameFanOutBody(nodeID, dependency) {
		output, ok := outputs[generationOutputKey{nodeID: string(dependency), itemIndex: itemIndex}]
		return output, ok
	}
	output, ok := outputs[generationOutputKey{nodeID: string(dependency), itemIndex: 0}]
	return output, ok
}

func collectDependenciesSucceeded(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	collect GenerationOutput,
) bool {
	outputMap := generationOutputMap(outputs)
	for _, dependency := range topology.dependencies[dsl.NodeID(collect.NodeID)] {
		if _, inScope := topology.fanOutScopes[fanOutID].body[dependency]; !inScope {
			output, ok := outputMap[generationOutputKey{nodeID: string(dependency), itemIndex: 0}]
			if !ok || output.Status != generationOutputSucceeded {
				return false
			}
			continue
		}
		if !fanOutBodyNodeSucceededForAllBranches(graph, outputs, fanOutID, dependency) {
			return false
		}
	}
	return true
}

func fanOutBodyNodeSucceededForAllBranches(
	graph dsl.Graph,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	nodeID dsl.NodeID,
) bool {
	branchCount, ok := fanOutBranchCount(outputs, fanOutID)
	if !ok {
		return false
	}
	outputMap := generationOutputMap(outputs)
	for itemIndex := range branchCount {
		output, ok := outputMap[generationOutputKey{nodeID: string(nodeID), itemIndex: itemIndex}]
		if !ok || output.Status != generationOutputSucceeded {
			return false
		}
	}
	return graphNodeExists(graph, nodeID)
}

func fanOutBranchCount(outputs []GenerationOutput, fanOutID dsl.NodeID) (int, bool) {
	for _, output := range outputs {
		if output.NodeID != string(fanOutID) || output.ItemIndex != 0 {
			continue
		}
		materialization, ok, err := parseFanOutMaterialization(output.OutputRef)
		if err != nil || !ok {
			return 0, false
		}
		return materialization.Branches, true
	}
	return 0, false
}

func fanOutSlotAvailable(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	output GenerationOutput,
) bool {
	fanOutID, inScope := topology.inFanOutBody(dsl.NodeID(output.NodeID))
	if !inScope {
		return true
	}
	if branchStarted(topology, outputs, fanOutID, output.ItemIndex) {
		return true
	}
	limit := fanOutMaxParallel(outputs, fanOutID)
	if limit <= 0 {
		limit = 1
	}
	active := 0
	branchCount, ok := fanOutBranchCount(outputs, fanOutID)
	if !ok {
		return false
	}
	for itemIndex := range branchCount {
		if branchStarted(topology, outputs, fanOutID, itemIndex) &&
			!branchComplete(graph, topology, outputs, fanOutID, itemIndex) {
			active++
		}
	}
	return active < limit
}

func branchStarted(
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	itemIndex int,
) bool {
	scope := topology.fanOutScopes[fanOutID]
	for _, output := range outputs {
		if output.ItemIndex != itemIndex {
			continue
		}
		if _, ok := scope.body[dsl.NodeID(output.NodeID)]; !ok {
			continue
		}
		if output.Status != generationOutputPending {
			return true
		}
	}
	return false
}

func branchComplete(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	itemIndex int,
) bool {
	scope := topology.fanOutScopes[fanOutID]
	outputMap := generationOutputMap(outputs)
	for nodeID := range scope.body {
		if !graphNodeExists(graph, nodeID) {
			continue
		}
		output, ok := outputMap[generationOutputKey{nodeID: string(nodeID), itemIndex: itemIndex}]
		if !ok || output.Status != generationOutputSucceeded && output.Status != generationOutputFailed {
			return false
		}
	}
	return true
}

func fanOutMaxParallel(outputs []GenerationOutput, fanOutID dsl.NodeID) int {
	for _, output := range outputs {
		if output.NodeID != string(fanOutID) || output.ItemIndex != 0 {
			continue
		}
		materialization, ok, err := parseFanOutMaterialization(output.OutputRef)
		if err != nil || !ok {
			return 0
		}
		return materialization.MaxParallel
	}
	return 0
}

func appendReadyNodeRunsControlAware(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	gatesEnabled bool,
	outputs []GenerationOutput,
) error {
	indexes := generationOutputIndexMap(outputs)
	for _, output := range sortedGenerationOutputs(outputs) {
		if output.Status != generationOutputPending {
			continue
		}
		node, ok := graphNode(resolved.Definition.Graph, dsl.NodeID(output.NodeID))
		if !ok || isCoordinatorOwnedNodeWithGates(node, gatesEnabled) {
			continue
		}
		if !dependenciesSucceededForOutput(resolved.Definition.Graph, topology, outputs, output) {
			continue
		}
		if !fanOutSlotAvailable(resolved.Definition.Graph, topology, outputs, output) {
			continue
		}
		metadata, err := coordinatorNodeMetadataForOutput(run, generation, node, topology, outputs, output)
		if err != nil {
			return err
		}
		runID := coordinatorNodeRunID(run.ID, generation, node.ID, output.ItemIndex)
		idempotencyKey := coordinatorNodeIdempotencyKey(run.ID, generation, node.ID, output.ItemIndex)
		if dsl.ActionKind(node.Kind) == dsl.ActionGoal {
			runID = GoalSegmentRunID(run.ID, generation, node.ID, output.ItemIndex, initialGoalSegmentEpoch)
			idempotencyKey = GoalSegmentIdempotencyKey(
				run.ID,
				generation,
				node.ID,
				output.ItemIndex,
				initialGoalSegmentEpoch,
			)
		}
		output.Status = generationOutputEnqueued
		output.TaskRunID = runID
		key := generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}
		if idx, exists := indexes[key]; exists {
			outputs[idx] = output
		}
		plan.NodeRuns = append(plan.NodeRuns, task.EnqueueSpec{
			TaskID:                       coordinatorNodeTaskID(run.ID, generation, node.ID, output.ItemIndex),
			RunID:                        runID,
			RunKind:                      task.RunKindWorker,
			LoopRunID:                    string(run.ID),
			IdempotencyKey:               idempotencyKey,
			ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
			Metadata:                     metadata,
		})
	}
	return nil
}

func allGenerationOutputsSucceededControlAware(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
) bool {
	if len(graph.Nodes) == 0 {
		return false
	}
	outputMap := generationOutputMap(outputs)
	for _, node := range graph.Nodes {
		if _, inFanOut := topology.inFanOutBody(node.ID); inFanOut {
			branchCount, ok := fanOutBranchCount(outputs, topology.nodeFanOut[node.ID])
			if !ok {
				return false
			}
			for itemIndex := range branchCount {
				output, ok := outputMap[generationOutputKey{nodeID: string(node.ID), itemIndex: itemIndex}]
				if !ok || output.Status != generationOutputSucceeded {
					return false
				}
			}
			continue
		}
		output, ok := outputMap[generationOutputKey{nodeID: string(node.ID), itemIndex: 0}]
		if !ok || output.Status != generationOutputSucceeded {
			return false
		}
	}
	return true
}

func graphNodeExists(graph dsl.Graph, nodeID dsl.NodeID) bool {
	_, ok := graphNode(graph, nodeID)
	return ok
}
