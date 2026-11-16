package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const maxTerminalMessageDependencies = 8

// noReadyNodesTerminal fails a generation that has unsettled nodes but nothing runnable and
// names the first stuck node with the dependencies that never settled.
func noReadyNodesTerminal(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
) *task.CoordinatorTerminal {
	terminal := &task.CoordinatorTerminal{
		Status:     string(StatusFailed),
		Cause:      string(TransitionCauseContract),
		ReasonCode: noReadyNodesReasonCode,
		Message:    "no node can run and the graph has not finished",
	}
	nodeID, blocking, ok := firstStuckNode(graph, topology, outputs)
	if !ok {
		return terminal
	}
	terminal.NodeID = string(nodeID)
	terminal.Message = fmt.Sprintf("node %q can never run", nodeID)
	if len(blocking) > 0 {
		terminal.Message += ": dependencies never settled: " + joinNodeIDs(blocking)
	}
	return terminal
}

func firstStuckNode(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
) (dsl.NodeID, []string, bool) {
	outputMap := generationOutputMap(outputs)
	for _, output := range sortedGenerationOutputs(outputs) {
		if output.Status != generationOutputPending {
			continue
		}
		nodeID := dsl.NodeID(output.NodeID)
		if _, found := graphNode(graph, nodeID); !found {
			continue
		}
		blocking := make([]string, 0)
		for _, dependency := range topology.dependencies[nodeID] {
			dependencyOutput, ok := dependencyOutputForNode(topology, outputMap, nodeID, dependency, output.ItemIndex)
			if ok && generationOutputSatisfiesDependency(dependencyOutput.Status) {
				continue
			}
			state := "missing"
			if ok {
				state = dependencyOutput.Status
			}
			blocking = append(blocking, fmt.Sprintf("%s (%s)", dependency, state))
		}
		if len(blocking) > 0 {
			return nodeID, blocking, true
		}
	}
	return "", nil, false
}

func failedOutputMessage(nodeID string, failure ClassifiedFailure) string {
	message := fmt.Sprintf("node %q failed", nodeID)
	if cause := strings.TrimSpace(failure.Cause); cause != "" {
		message += ": " + cause
	}
	return message
}

func joinNodeIDs(values []string) string {
	if len(values) > maxTerminalMessageDependencies {
		extra := len(values) - maxTerminalMessageDependencies
		values = append(values[:maxTerminalMessageDependencies:maxTerminalMessageDependencies],
			fmt.Sprintf("and %d more", extra))
	}
	return strings.Join(values, ", ")
}
