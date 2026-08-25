package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func appendCoordinatorArtifactsForOutputs(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	gatesEnabled bool,
	outputs []GenerationOutput,
) error {
	if err := appendCoordinatorTasksForOutputs(
		plan,
		run,
		generation,
		graph,
		topology,
		gatesEnabled,
		outputs,
	); err != nil {
		return err
	}
	appendCoordinatorDependenciesForOutputs(
		plan,
		run,
		generation,
		graph,
		topology,
		gatesEnabled,
		outputs,
	)
	return nil
}

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
	planned := coordinatorPlannedNodeTaskIDs(plan.NodeRuns)
	for _, output := range sortedGenerationOutputs(outputs) {
		if GenerationOutputStatusParked(output.Status) {
			continue
		}
		if _, parked := requiredParkedProducerOutput(graph, topology, outputs, output); parked {
			continue
		}
		node, ok := graphNode(graph, dsl.NodeID(output.NodeID))
		if !ok {
			continue
		}
		if isCoordinatorOwnedNodeWithGates(node, gatesEnabled) {
			continue
		}
		taskID := coordinatorNodeTaskID(run.ID, generation, node.ID, output.ItemIndex)
		if _, ok := planned[taskID]; !ok {
			continue
		}
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
) {
	existing := make(map[string]struct{}, len(plan.Dependencies))
	for _, dependency := range plan.Dependencies {
		existing[dependencyKey(dependency.TaskID, dependency.DependsOnTaskID)] = struct{}{}
	}
	planned := coordinatorPlannedNodeTaskIDs(plan.NodeRuns)
	outputMap := generationOutputMap(outputs)
	for _, output := range sortedGenerationOutputs(outputs) {
		if GenerationOutputStatusParked(output.Status) {
			continue
		}
		if _, parked := requiredParkedProducerOutput(graph, topology, outputs, output); parked {
			continue
		}
		nodeID := dsl.NodeID(output.NodeID)
		node, ok := graphNode(graph, nodeID)
		if !ok || isCoordinatorOwnedNodeWithGates(node, gatesEnabled) {
			continue
		}
		taskID := coordinatorNodeTaskID(run.ID, generation, nodeID, output.ItemIndex)
		if _, ok := planned[taskID]; !ok {
			continue
		}
		for _, dependency := range topology.dependencies[nodeID] {
			dependencyNode, ok := graphNode(graph, dependency)
			if !ok || isCoordinatorOwnedNodeWithGates(dependencyNode, gatesEnabled) {
				continue
			}
			if isAuthoredErrorRouteDependency(dependencyNode, nodeID) {
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
				if dependencyOutput.NodeID == "" || GenerationOutputStatusParked(dependencyOutput.Status) {
					continue
				}
				if !generationOutputOwnsTaskInGeneration(
					run,
					generation,
					dependencyNode,
					dependencyOutput,
				) {
					continue
				}
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
}

func coordinatorPlannedNodeTaskIDs(runs []task.EnqueueSpec) map[string]struct{} {
	taskIDs := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		taskID := strings.TrimSpace(run.TaskID)
		if taskID != "" {
			taskIDs[taskID] = struct{}{}
		}
	}
	return taskIDs
}

func generationOutputOwnsTaskInGeneration(
	run Run,
	generation int,
	node dsl.Node,
	output GenerationOutput,
) bool {
	taskRunID := strings.TrimSpace(output.TaskRunID)
	if taskRunID == "" || outputMarksSkippedRoute(output) {
		return false
	}
	if dsl.ActionKind(node.Kind) == dsl.ActionGoal {
		baseRunID := coordinatorNodeRunID(run.ID, generation, node.ID, output.ItemIndex)
		return strings.HasPrefix(taskRunID, baseRunID+":goal-segment:")
	}
	return taskRunID == NodeCellAttemptRunID(
		run.ID,
		generation,
		string(node.ID),
		output.ItemIndex,
		output.Attempt,
	)
}

func isAuthoredErrorRouteDependency(source dsl.Node, target dsl.NodeID) bool {
	policy := nodeErrorPolicy(source)
	return policy != nil && policy.Route == target
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
	metadata, err := coordinatorNodeMetadataWithFanOutItem(
		run,
		generation,
		node,
		output.ItemIndex,
		output.Attempt,
		output.Epoch,
		item,
		hasItem,
	)
	if err != nil {
		return nil, err
	}
	if node.Review == nil || !OutputRefLooksContentAddressed(output.OutputRef) {
		return metadata, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(metadata, &payload); err != nil {
		return nil, fmt.Errorf("loop: decode reviewed node metadata for %s: %w", node.ID, err)
	}
	payload["reviewed_params_ref"] = output.OutputRef
	metadata, err = json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal reviewed node metadata for %s: %w", node.ID, err)
	}
	return metadata, nil
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
