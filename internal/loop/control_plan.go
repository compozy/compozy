package loop

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

const branchFalseOutputRef = "branch:false"
const branchSkippedOutputRef = "branch_skipped"
const branchTrueOutputRef = "branch:true"
const subLoopEnteredOutputRef = "sub_loop_entered"

type controlEvalContext struct {
	ctx                context.Context
	run                Run
	generation         int
	resolved           *ResolvedDefinition
	topology           controlTopology
	effective          EffectiveConfig
	gateEvaluator      gate.GateEvaluator
	gateDecisions      GateDecisionReader
	fanOutWidth        int
	watchRuntime       coordinatorWatchRuntime
	watchEventsRuntime coordinatorWatchEventsRuntime
}

func buildInitialControlAwareCoordinatorPlan(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	gateEvaluator gate.GateEvaluator,
	gateDecisions GateDecisionReader,
	fanOutWidth int,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	topology := newControlTopology(graph)
	outputs := initialGenerationOutputs(graph, topology, generation)
	plan, err := newInitialControlCoordinatorPlan(
		run,
		generation,
		graph,
		topology,
		gateEvaluator != nil,
		outputs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	outputBlobs := []GenerationOutputBlob{}
	terminal, err := advanceControlNodes(
		&controlEvalContext{
			ctx:                ctx,
			run:                run,
			generation:         generation,
			resolved:           resolved,
			topology:           topology,
			effective:          effective,
			gateEvaluator:      gateEvaluator,
			gateDecisions:      gateDecisions,
			fanOutWidth:        fanOutWidth,
			watchRuntime:       watchRuntime,
			watchEventsRuntime: watchEventsRuntime,
		},
		&plan,
		&outputs,
		&outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.Snapshot.Payload = generationSnapshotPayload(outputs, outputBlobs)
	if terminal != nil {
		plan.Terminal = terminal
	}
	if terminal != nil || plan.Yield {
		return plan, nil
	}
	return finishInitialControlPlan(
		&plan,
		run,
		generation,
		resolved,
		topology,
		gateEvaluator,
		outputs,
		outputBlobs,
	)
}

func initialGenerationOutputs(
	graph dsl.Graph,
	topology controlTopology,
	generation int,
) []GenerationOutput {
	outputs := make([]GenerationOutput, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if _, inFanOut := topology.inFanOutBody(node.ID); inFanOut {
			continue
		}
		outputs = append(outputs, GenerationOutput{
			Generation: generation,
			NodeID:     string(node.ID),
			ItemIndex:  0,
			Status:     generationOutputPending,
		})
	}
	return outputs
}

func advanceControlNodes(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	outputs *[]GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (*task.CoordinatorTerminal, error) {
	for {
		changed := false
		indexes := generationOutputIndexMap(*outputs)
		for _, output := range sortedGenerationOutputs(*outputs) {
			if output.Status != generationOutputPending {
				continue
			}
			node, ok := graphNode(eval.resolved.Definition.Graph, dsl.NodeID(output.NodeID))
			if !ok || !isCoordinatorOwnedNodeWithGates(node, eval.gateEvaluator != nil) {
				continue
			}
			if !dependenciesSucceededForOutput(eval.resolved.Definition.Graph, eval.topology, *outputs, output) {
				continue
			}
			updated, terminal, err := evaluateControlNode(
				eval,
				plan,
				output,
				node,
				outputs,
				outputBlobs,
			)
			if err != nil {
				return nil, err
			}
			key := generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}
			if idx, ok := indexes[key]; ok {
				(*outputs)[idx] = updated
			}
			if terminal != nil {
				return terminal, nil
			}
			if plan.Yield {
				return nil, nil
			}
			changed = true
		}
		if !changed {
			return nil, nil
		}
	}
}

func evaluateControlNode(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	if evaluated, terminal, handled, err := evaluateSourceControlNode(
		eval.ctx,
		plan,
		eval.run,
		eval.generation,
		eval.resolved,
		eval.topology,
		eval.watchRuntime,
		eval.watchEventsRuntime,
		output,
		node,
		*outputs,
		outputBlobs,
	); handled {
		return evaluated, terminal, err
	}
	switch dsl.ControlKind(node.Kind) {
	case dsl.ControlFanOut:
		return evaluateFanOutNode(
			plan,
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			eval.gateEvaluator != nil,
			eval.fanOutWidth,
			output,
			node,
			outputs,
		)
	case dsl.ControlCollect:
		output.Status = generationOutputSucceeded
		return output, nil, nil
	case dsl.ControlBranch:
		return evaluateBranchNode(
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			output,
			node,
			outputs,
		)
	case dsl.ControlGate:
		return evaluateGateNode(
			eval.ctx,
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			eval.effective,
			eval.gateEvaluator,
			eval.gateDecisions,
			output,
			node,
			*outputs,
		)
	case dsl.ControlSubLoop:
		output.Status = generationOutputSucceeded
		output.OutputRef = subLoopEnteredOutputRef
		return output, nil, nil
	default:
		return output, nil, nil
	}
}

func evaluateFanOutNode(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	gatesEnabled bool,
	fanOutWidth int,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	namespace, err := runtimeNamespace(run, generation, resolved.Definition.Graph, topology, *outputs, node.ID, 0)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	items, err := resolveFanOutCollection(resolved, node, namespace)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	materialization, terminal := buildFanOutMaterialization(node, items, fanOutWidth)
	if terminal != nil {
		return output, terminal, nil
	}
	ref, err := fanOutMaterializationRef(materialization)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputSucceeded
	output.OutputRef = ref
	if err := materializeFanOutBody(
		plan,
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		gatesEnabled,
		node.ID,
		materialization,
		outputs,
	); err != nil {
		return GenerationOutput{}, nil, err
	}
	return output, nil, nil
}

func materializeFanOutBody(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	gatesEnabled bool,
	fanOutID dsl.NodeID,
	materialization fanOutMaterialization,
	outputs *[]GenerationOutput,
) error {
	scope := topology.fanOutScopes[fanOutID]
	indexes := generationOutputIndexMap(*outputs)
	for itemIndex := 0; itemIndex < materialization.Branches; itemIndex++ {
		for _, node := range graph.Nodes {
			if _, ok := scope.body[node.ID]; !ok {
				continue
			}
			key := generationOutputKey{nodeID: string(node.ID), itemIndex: itemIndex}
			if _, exists := indexes[key]; exists {
				continue
			}
			*outputs = append(*outputs, GenerationOutput{
				Generation: generation,
				NodeID:     string(node.ID),
				ItemIndex:  itemIndex,
				Status:     generationOutputPending,
			})
		}
	}
	if err := appendCoordinatorTasksForOutputs(
		plan,
		run,
		generation,
		graph,
		topology,
		gatesEnabled,
		*outputs,
	); err != nil {
		return err
	}
	return appendCoordinatorDependenciesForOutputs(plan, run, generation, graph, topology, gatesEnabled, *outputs)
}

func evaluateBranchNode(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
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
	namespace, err := runtimeNamespace(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		*outputs,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	value, _, err := condition.Program.Eval(namespace)
	if err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: evaluate branch %s: %w", node.ID, err)
	}
	result, ok := value.Value().(bool)
	if !ok {
		return GenerationOutput{}, nil, fmt.Errorf(
			"%w: branch %s condition returned %T",
			ErrValidation,
			node.ID,
			value.Value(),
		)
	}
	output.Status = generationOutputSucceeded
	if !result {
		output.OutputRef = branchFalseOutputRef
		skipBranchDependents(resolved.Definition.Graph, topology, node.ID, output, outputs)
		return output, nil, nil
	}
	output.OutputRef = branchTrueOutputRef
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
		if !ok || isControlKind(node, dsl.ControlCollect) {
			continue
		}
		if !allBranchPathDependenciesSkipped(topology, outputMap, nodeID, branchID, itemIndex) {
			continue
		}
		key := generationOutputKey{
			nodeID:    string(nodeID),
			itemIndex: itemIndexForSkippedNode(topology, nodeID, branchID, itemIndex),
		}
		if idx, exists := indexes[key]; exists && (*outputs)[idx].Status == generationOutputPending {
			(*outputs)[idx].Status = generationOutputSucceeded
			(*outputs)[idx].OutputRef = branchSkippedOutputRef
			outputMap[key] = (*outputs)[idx]
			queue = append(queue, topology.dependents[nodeID]...)
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

func fanOutCeilingTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusExhausted),
		Cause:      string(TransitionCauseContract),
		ReasonCode: "fan_out_width_exceeded",
	}
}

func noReadyNodesTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusFailed),
		Cause:      string(TransitionCauseContract),
		ReasonCode: "no_ready_nodes",
	}
}
