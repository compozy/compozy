package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const branchFalseOutputRef = "branch:false"
const branchSkippedOutputRef = "branch_skipped"
const branchTrueOutputRef = "branch:true"

func evaluateBranchNode(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	history GenerationHistory,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	diagnostics *gateEvaluationCollector,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	key := fmt.Sprintf("nodes.%s.condition", node.ID)
	condition := resolved.Conditions[key]
	if condition == nil {
		return GenerationOutput{}, nil, fmt.Errorf(
			"%w: compiled branch condition %q is missing",
			ErrValidation,
			key,
		)
	}
	namespace, err := runtimeNamespaceWithHistory(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		*outputs,
		history,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	evaluated, err := evaluatePredicate(key, condition, namespace, PredicateRouting, node.OnEvalError)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: evaluate branch %s: %w", node.ID, err)
	}
	diagnostics.recordPredicate(evaluated.Diagnostics...)
	if evaluated.Disposition != nil {
		if evaluated.Disposition.Policy == PredicateErrorExit {
			output.Status = generationOutputSucceeded
			return output, predicateExitTerminal(evaluated.Disposition.Diagnostic), nil
		}
		failed, failureErr := applyPredicateFailureDisposition(
			output,
			node,
			evaluated.Disposition.Failure,
			resolved.Definition.Graph,
			topology,
			outputs,
		)
		return failed, nil, failureErr
	}
	result := evaluated.Value
	output.Status = generationOutputSucceeded
	if !result {
		setGenerationOutputRef(&output, branchFalseOutputRef)
		skipBranchDependents(resolved.Definition.Graph, topology, node.ID, output, outputs)
		return output, nil, nil
	}
	setGenerationOutputRef(&output, branchTrueOutputRef)
	return output, nil, nil
}

func skipBranchDependents(
	graph dsl.Graph,
	topology controlTopology,
	branchID dsl.NodeID,
	branchOutput GenerationOutput,
	outputs *[]GenerationOutput,
) {
	indexes := generationOutputIndexMap(*outputs)
	outputMap := generationOutputMap(*outputs)
	itemIndex := branchOutput.ItemIndex
	outputMap[generationOutputKey{nodeID: string(branchID), itemIndex: itemIndex}] = branchOutput
	queue := append([]dsl.NodeID(nil), topology.dependents[branchID]...)
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		node, ok := graphNode(graph, nodeID)
		if !ok {
			continue
		}
		if !allBranchPathDependenciesSkipped(topology, outputMap, nodeID, branchID, itemIndex) {
			continue
		}
		key := generationOutputKey{
			nodeID:    string(nodeID),
			itemIndex: itemIndexForSkippedNode(topology, nodeID, branchID, itemIndex),
		}
		if idx, exists := indexes[key]; exists {
			if (*outputs)[idx].Status == generationOutputPending {
				(*outputs)[idx].Status = generationOutputSucceeded
				setGenerationOutputRef(&(*outputs)[idx], branchSkippedOutputRef)
				outputMap[key] = (*outputs)[idx]
				queue = append(queue, topology.dependents[nodeID]...)
				if isControlKind(node, dsl.ControlFanOut) {
					for bodyID := range topology.fanOutScopes[nodeID].body {
						queue = append(queue, bodyID)
					}
					for collectID := range topology.fanOutScopes[nodeID].collect {
						queue = append(queue, collectID)
					}
				}
			}
		} else {
			skippedOutput := GenerationOutput{
				NodeID:    string(nodeID),
				ItemIndex: key.itemIndex,
				Status:    generationOutputSucceeded,
			}
			setGenerationOutputRef(&skippedOutput, branchSkippedOutputRef)
			*outputs = append(*outputs, skippedOutput)
			indexes[key] = len(*outputs) - 1
			outputMap[key] = skippedOutput
			queue = append(queue, topology.dependents[nodeID]...)
			if isControlKind(node, dsl.ControlFanOut) {
				for bodyID := range topology.fanOutScopes[nodeID].body {
					queue = append(queue, bodyID)
				}
				for collectID := range topology.fanOutScopes[nodeID].collect {
					queue = append(queue, collectID)
				}
			}
		}
	}
}

func allBranchPathDependenciesSkipped(
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
	branchID dsl.NodeID,
	itemIndex int,
) bool {
	dependencies := topology.dependencies[nodeID]
	if len(dependencies) == 0 {
		return false
	}
	for _, dependency := range dependencies {
		dependencyOutput, ok := dependencyOutputForNode(topology, outputs, nodeID, dependency, itemIndex)
		if !ok {
			fanOutID, inFanOutBody := topology.inFanOutBody(dependency)
			if inFanOutBody {
				fanOutOutput, fanOutOK := dependencyOutputForNode(
					topology,
					outputs,
					nodeID,
					fanOutID,
					itemIndex,
				)
				if fanOutOK && fanOutOutput.OutputRef == branchSkippedOutputRef {
					continue
				}
			}
			return false
		}
		if dependency == branchID && dependencyOutput.OutputRef == branchFalseOutputRef {
			continue
		}
		if dependencyOutput.OutputRef == branchSkippedOutputRef {
			continue
		}
		return false
	}
	return true
}

func itemIndexForSkippedNode(
	topology controlTopology,
	nodeID dsl.NodeID,
	branchID dsl.NodeID,
	itemIndex int,
) int {
	if topology.sameFanOutBody(nodeID, branchID) {
		return itemIndex
	}
	return 0
}
