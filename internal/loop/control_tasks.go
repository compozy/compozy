package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func appendCoordinatorTasksForOutputs(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	gatesEnabled bool,
	outputs []GenerationOutput,
) error {
	existing := make(map[string]struct{}, len(plan.NodeTasks))
	for _, spec := range plan.NodeTasks {
		existing[spec.TaskID] = struct{}{}
	}
	for _, output := range sortedGenerationOutputs(outputs) {
		node, ok := graphNode(graph, dsl.NodeID(output.NodeID))
		if !ok {
			continue
		}
		if isCoordinatorOwnedNodeWithGates(node, gatesEnabled) {
			continue
		}
		taskID := coordinatorNodeTaskID(run.ID, generation, node.ID, output.ItemIndex)
		if _, ok := existing[taskID]; ok {
			continue
		}
		metadata, err := coordinatorNodeMetadataForOutput(run, generation, node, topology, outputs, output)
		if err != nil {
			return err
		}
		plan.NodeTasks = append(plan.NodeTasks, task.CoordinatorTaskSpec{
			TaskID: taskID,
			Title:  fmt.Sprintf("Loop %s node %s", run.LoopName, node.ID),
			Description: fmt.Sprintf(
				"Generation %d node %s for loop run %s.",
				generation,
				node.ID,
				run.ID,
			),
			Metadata: metadata,
		})
		existing[taskID] = struct{}{}
	}
	return nil
}

func appendCoordinatorDependenciesForOutputs(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	gatesEnabled bool,
	outputs []GenerationOutput,
) error {
	existing := make(map[string]struct{}, len(plan.Dependencies))
	for _, dependency := range plan.Dependencies {
		existing[dependencyKey(dependency.TaskID, dependency.DependsOnTaskID)] = struct{}{}
	}
	outputMap := generationOutputMap(outputs)
	for _, output := range sortedGenerationOutputs(outputs) {
		nodeID := dsl.NodeID(output.NodeID)
		node, ok := graphNode(graph, nodeID)
		if !ok || isCoordinatorOwnedNodeWithGates(node, gatesEnabled) {
			continue
		}
		for _, dependency := range topology.dependencies[nodeID] {
			dependencyNode, ok := graphNode(graph, dependency)
			if !ok || isCoordinatorOwnedNodeWithGates(dependencyNode, gatesEnabled) {
				continue
			}
			dependencyOutputs := dependencyOutputsForTaskDependency(
				graph,
				topology,
				outputMap,
				nodeID,
				dependency,
				output.ItemIndex,
			)
			for _, dependencyOutput := range dependencyOutputs {
				if dependencyOutput.NodeID == "" {
					continue
				}
				taskID := coordinatorNodeTaskID(run.ID, generation, nodeID, output.ItemIndex)
				dependsOnTaskID := coordinatorNodeTaskID(
					run.ID,
					generation,
					dsl.NodeID(dependencyOutput.NodeID),
					dependencyOutput.ItemIndex,
				)
				key := dependencyKey(taskID, dependsOnTaskID)
				if _, ok := existing[key]; ok {
					continue
				}
				plan.Dependencies = append(plan.Dependencies, task.CoordinatorDependencySpec{
					TaskID:          taskID,
					DependsOnTaskID: dependsOnTaskID,
					Kind:            task.DependencyKindBlocks,
				})
				existing[key] = struct{}{}
			}
		}
	}
	return nil
}

func dependencyOutputsForTaskDependency(
	graph dsl.Graph,
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
	dependency dsl.NodeID,
	itemIndex int,
) []GenerationOutput {
	if fanOutID, ok := topology.isCollectForFanOut(nodeID); ok {
		if _, inScope := topology.fanOutScopes[fanOutID].body[dependency]; inScope {
			branchCount := maxExistingItemIndex(graph, topology, outputs, dependency) + 1
			dependencies := make([]GenerationOutput, 0, branchCount)
			for idx := range branchCount {
				if output, ok := outputs[generationOutputKey{nodeID: string(dependency), itemIndex: idx}]; ok {
					dependencies = append(dependencies, output)
				}
			}
			return dependencies
		}
	}
	if topology.sameFanOutBody(nodeID, dependency) {
		if output, ok := outputs[generationOutputKey{nodeID: string(dependency), itemIndex: itemIndex}]; ok {
			return []GenerationOutput{output}
		}
		return nil
	}
	if output, ok := outputs[generationOutputKey{nodeID: string(dependency), itemIndex: 0}]; ok {
		return []GenerationOutput{output}
	}
	return nil
}

func maxExistingItemIndex(
	graph dsl.Graph,
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
) int {
	maxIndex := -1
	for key := range outputs {
		if key.nodeID != string(nodeID) {
			continue
		}
		if !graphNodeExists(graph, dsl.NodeID(key.nodeID)) {
			continue
		}
		if _, ok := topology.inFanOutBody(dsl.NodeID(key.nodeID)); !ok {
			continue
		}
		maxIndex = max(maxIndex, key.itemIndex)
	}
	return maxIndex
}

func dependencyKey(taskID string, dependsOnTaskID string) string {
	return taskID + "\x00" + dependsOnTaskID
}

func coordinatorNodeMetadataForOutput(
	run Run,
	generation int,
	node dsl.Node,
	topology controlTopology,
	outputs []GenerationOutput,
	output GenerationOutput,
) (json.RawMessage, error) {
	item, hasItem, err := fanOutItemForOutput(topology, outputs, node.ID, output.ItemIndex)
	if err != nil {
		return nil, err
	}
	return coordinatorNodeMetadataWithFanOutItem(run, generation, node, output.ItemIndex, item, hasItem)
}

func fanOutItemForOutput(
	topology controlTopology,
	outputs []GenerationOutput,
	nodeID dsl.NodeID,
	itemIndex int,
) (any, bool, error) {
	fanOutID, ok := topology.inFanOutBody(nodeID)
	if !ok {
		return nil, false, nil
	}
	return fanOutItem(outputs, fanOutID, itemIndex)
}
