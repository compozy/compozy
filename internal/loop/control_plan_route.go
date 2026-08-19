package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const routeNotTakenOutputRefPrefix = "route_not_taken:"

type routeDecision struct {
	NodeID      dsl.NodeID
	ItemIndex   int
	Target      dsl.NodeID
	Cause       string
	MatchedWhen string
	Default     bool
}

func (d routeDecision) normalized() routeDecision {
	d.NodeID = dsl.NodeID(strings.TrimSpace(string(d.NodeID)))
	d.Target = dsl.NodeID(strings.TrimSpace(string(d.Target)))
	d.Cause = strings.TrimSpace(d.Cause)
	d.MatchedWhen = strings.TrimSpace(d.MatchedWhen)
	return d
}

func evaluateRouteNode(
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
	decision := routeDecision{
		NodeID: node.ID, ItemIndex: output.ItemIndex, Target: node.Default,
		Cause: routeDefaultCause, Default: true,
	}
	for index, route := range node.Routes {
		key := fmt.Sprintf("nodes.%s.routes.%d.when", node.ID, index)
		condition := resolved.Conditions[key]
		if condition == nil {
			return GenerationOutput{}, nil, fmt.Errorf(
				"%w: compiled route condition %q is missing",
				ErrValidation,
				key,
			)
		}
		evaluated, evalErr := evaluatePredicate(
			key,
			condition,
			namespace,
			PredicateRouting,
			dsl.EvalErrorFail,
		)
		if evalErr != nil {
			return GenerationOutput{}, nil, fmt.Errorf("loop: evaluate route %s: %w", node.ID, evalErr)
		}
		diagnostics.recordPredicate(evaluated.Diagnostics...)
		if evaluated.Disposition != nil {
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
		if evaluated.Value {
			decision.Target = route.To
			decision.Cause = "matched_when"
			decision.MatchedWhen = route.When
			decision.Default = false
			break
		}
	}
	updated, err := applyRouteSelection(
		resolved.Definition.Graph,
		topology,
		output,
		node.ID,
		decision.Target,
		outputs,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	diagnostics.recordRoute(decision)
	return updated, nil, nil
}

func applyRouteSelection(
	graph dsl.Graph,
	topology controlTopology,
	output GenerationOutput,
	sourceID dsl.NodeID,
	selected dsl.NodeID,
	outputs *[]GenerationOutput,
) (GenerationOutput, error) {
	if !containsNodeID(topology.dependents[sourceID], selected) {
		return GenerationOutput{}, fmt.Errorf(
			"%w: route source %q selected non-forward target %q",
			ErrValidation,
			sourceID,
			selected,
		)
	}
	output.Status = generationOutputSucceeded
	setGenerationOutputRef(&output, "route:"+string(selected))
	skipUnselectedRoutePaths(graph, topology, sourceID, selected, output, outputs)
	return output, nil
}

func skipUnselectedRoutePaths(
	graph dsl.Graph,
	topology controlTopology,
	sourceID dsl.NodeID,
	selected dsl.NodeID,
	sourceOutput GenerationOutput,
	outputs *[]GenerationOutput,
) {
	indexes := generationOutputIndexMap(*outputs)
	outputMap := generationOutputMap(*outputs)
	outputMap[generationOutputKey{nodeID: string(sourceID), itemIndex: sourceOutput.ItemIndex}] = sourceOutput
	type queueEntry struct {
		nodeID       dsl.NodeID
		directTarget bool
	}
	queue := make([]queueEntry, 0, len(topology.dependents[sourceID]))
	for _, target := range topology.dependents[sourceID] {
		if target != selected {
			queue = append(queue, queueEntry{nodeID: target, directTarget: true})
		}
	}
	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]
		node, found := graphNode(graph, entry.nodeID)
		if !found || !routePathCanBeSkipped(
			topology,
			outputMap,
			entry.nodeID,
			sourceID,
			sourceOutput.ItemIndex,
			entry.directTarget,
		) {
			continue
		}
		key := generationOutputKey{
			nodeID: string(entry.nodeID),
			itemIndex: itemIndexForSkippedNode(
				topology,
				entry.nodeID,
				sourceID,
				sourceOutput.ItemIndex,
			),
		}
		index, exists := indexes[key]
		if !exists || (*outputs)[index].Status != generationOutputPending {
			continue
		}
		(*outputs)[index].Status = generationOutputSucceeded
		setGenerationOutputRef(&(*outputs)[index], routeNotTakenOutputRef(sourceID))
		outputMap[key] = (*outputs)[index]
		for _, dependent := range topology.dependents[entry.nodeID] {
			queue = append(queue, queueEntry{nodeID: dependent})
		}
		if isControlKind(node, dsl.ControlFanOut) {
			for collectID := range topology.fanOutScopes[entry.nodeID].collect {
				queue = append(queue, queueEntry{nodeID: collectID})
			}
		}
	}
}

func routePathCanBeSkipped(
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	nodeID dsl.NodeID,
	sourceID dsl.NodeID,
	itemIndex int,
	directTarget bool,
) bool {
	dependencies := topology.dependencies[nodeID]
	if len(dependencies) == 0 {
		return false
	}
	for _, dependency := range dependencies {
		if directTarget && dependency == sourceID {
			continue
		}
		dependencyOutput, ok := dependencyOutputForNode(topology, outputs, nodeID, dependency, itemIndex)
		if !ok || !outputRefMarksSkippedRoute(dependencyOutput.OutputRef) {
			return false
		}
	}
	return true
}

func routeNotTakenOutputRef(sourceID dsl.NodeID) string {
	return routeNotTakenOutputRefPrefix + string(sourceID)
}

func isRouteNotTakenOutputRef(outputRef string) bool {
	return strings.HasPrefix(strings.TrimSpace(outputRef), routeNotTakenOutputRefPrefix)
}

func outputRefMarksSkippedRoute(outputRef string) bool {
	return strings.TrimSpace(outputRef) == branchSkippedOutputRef || isRouteNotTakenOutputRef(outputRef)
}
