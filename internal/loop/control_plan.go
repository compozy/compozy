package loop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

const subLoopEnteredOutputRef = "sub_loop_entered"

func buildInitialControlAwareCoordinatorPlan(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	gateEvaluator gate.GateEvaluator,
	gateDecisions GateDecisionReader,
	nodeControls NodeControlReader,
	runtimeCatalog WorkspaceRuntimeCatalog,
	fanOutWidth int,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
	history GenerationHistory,
	scheduledAt time.Time,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	topology := newResolvedControlTopology(resolved)
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
	gateEvaluations := &gateEvaluationCollector{}
	terminal, err := advanceControlNodes(
		newInitialControlEvalContext(
			ctx, run, generation, resolved, topology, effective, gateEvaluator, gateDecisions,
			nodeControls, runtimeCatalog, fanOutWidth, watchRuntime, watchEventsRuntime,
			gateEvaluations, history, scheduledAt,
		),
		&plan,
		&outputs,
		&outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	payload, err := generationSnapshotPayloadPreservingIntents(plan.Snapshot.Payload, outputs, outputBlobs)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.Snapshot.Payload = payload
	if err := applyGateEvaluationIntents(&plan, run, generation, gateEvaluations); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
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
		scheduledAt,
	)
}

func newInitialControlEvalContext(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	effective EffectiveConfig,
	gateEvaluator gate.GateEvaluator,
	gateDecisions GateDecisionReader,
	nodeControls NodeControlReader,
	runtimeCatalog WorkspaceRuntimeCatalog,
	fanOutWidth int,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
	gateEvaluations *gateEvaluationCollector,
	history GenerationHistory,
	scheduledAt time.Time,
) *controlEvalContext {
	return &controlEvalContext{
		ctx: ctx, run: run, generation: generation, resolved: resolved, topology: topology,
		effective: effective, gateEvaluator: gateEvaluator, gateDecisions: gateDecisions,
		nodeControls: nodeControls, runtimeCatalog: runtimeCatalog, fanOutWidth: fanOutWidth,
		watchRuntime: watchRuntime, watchEventsRuntime: watchEventsRuntime,
		gateEvaluations: gateEvaluations, history: history, now: scheduledAt.UTC(),
	}
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
			Attempt:    1,
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
	var approvalTerminal *task.CoordinatorTerminal
	for {
		changed, err := advanceFanOutWindows(
			eval.resolved.Definition.Graph, eval.topology, eval.generation, outputs,
		)
		if err != nil {
			return nil, err
		}
		indexes := generationOutputIndexMap(*outputs)
		for _, candidate := range sortedGenerationOutputs(*outputs) {
			key := generationOutputKey{nodeID: candidate.NodeID, itemIndex: candidate.ItemIndex}
			idx, ok := indexes[key]
			if !ok {
				continue
			}
			output := (*outputs)[idx]
			if output.Status != generationOutputPending {
				continue
			}
			node, ok := graphNode(eval.resolved.Definition.Graph, dsl.NodeID(output.NodeID))
			if !ok {
				continue
			}
			isReview := node.Class == dsl.NodeClassAction && node.Review != nil &&
				strings.TrimSpace(output.OutputRef) == ""
			if !isReview && !isCoordinatorOwnedNodeWithGates(node, eval.gateEvaluator != nil) {
				continue
			}
			if !dependenciesSucceededForOutput(eval.resolved.Definition.Graph, eval.topology, *outputs, output) {
				continue
			}
			var updated GenerationOutput
			var terminal *task.CoordinatorTerminal
			var err error
			if isReview {
				updated, err = evaluateActionReview(eval, plan, output, node, *outputs)
			} else {
				updated, terminal, err = evaluateControlNode(
					eval, plan, output, node, outputs, outputBlobs,
				)
			}
			if err != nil {
				if !errors.Is(err, ErrActionMaterialization) {
					return nil, err
				}
				updated = materializationFailedOutput(output)
			}
			(*outputs)[idx] = updated
			if terminal != nil {
				if terminal.Status != string(StatusNeedsApproval) {
					return terminal, nil
				}
				if approvalTerminal == nil {
					approvalTerminal = terminal
				}
			}
			if plan.Yield {
				return approvalTerminal, nil
			}
			changed = true
		}
		if !changed {
			return approvalTerminal, nil
		}
	}
}

func materializationFailedOutput(output GenerationOutput) GenerationOutput {
	output.Status = generationOutputFailed
	failure := NewActionFailure(
		string(ReasonCodeActionMaterializationFailed),
		"an authored node value could not be materialized",
		"fix the node template or its referenced data",
	)
	if ref, ok := ActionFailureOutputRef(failure); ok {
		setGenerationOutputRef(&output, ref)
	}
	return output
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
		eval.gateEvaluations,
		eval.history,
		output,
		node,
		*outputs,
		outputBlobs,
	); handled {
		return evaluated, terminal, err
	}
	return evaluateControlNodeKind(eval, plan, output, node, outputs, outputBlobs)
}

func evaluateControlNodeKind(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	switch dsl.ControlKind(node.Kind) {
	case dsl.ControlFanOut:
		return evaluateFanOutNode(eval, output, node, outputs, outputBlobs)
	case dsl.ControlCollect:
		return evaluateCollectNode(eval, plan, output, node, outputs, outputBlobs)
	case dsl.ControlBranch:
		return evaluateBranchNode(
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			eval.history,
			output,
			node,
			outputs,
			eval.gateEvaluations,
		)
	case dsl.ControlRoute:
		return evaluateRouteNode(
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			eval.history,
			output,
			node,
			outputs,
			eval.gateEvaluations,
		)
	case dsl.ControlGate:
		gateOutput, terminal, err := evaluateGateNode(
			eval.ctx,
			eval.run,
			eval.generation,
			eval.resolved,
			eval.topology,
			eval.effective,
			eval.gateEvaluator,
			eval.gateDecisions,
			eval.nodeControls,
			eval.runtimeCatalog,
			eval.history,
			output,
			node,
			outputs,
			eval.gateEvaluations,
			eval.now,
		)
		if err != nil || terminal == nil || terminal.Status != string(StatusNeedsApproval) {
			return gateOutput, terminal, err
		}
		parked, err := parkGateApprovalWait(eval, plan, node, gateOutput)
		return parked, terminal, err
	case dsl.ControlWait:
		return evaluateWaitNode(eval, plan, output, node, *outputs, outputBlobs)
	case dsl.ControlAsk:
		return evaluateAskNode(eval, plan, output, node, *outputs)
	case dsl.ControlSubLoop:
		output.Status = generationOutputSucceeded
		setGenerationOutputRef(&output, subLoopEnteredOutputRef)
		return output, nil, nil
	default:
		return output, nil, nil
	}
}

func evaluateFanOutNode(
	eval *controlEvalContext,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	namespace, err := runtimeNamespaceWithHistory(
		eval.run,
		eval.generation,
		eval.resolved.Definition.Graph,
		eval.topology,
		*outputs,
		eval.history,
		node.ID,
		0,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	items, err := resolveFanOutCollection(eval.resolved, node, namespace)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	filtered, err := evaluateFanOutFilter(eval.resolved, node, namespace, items)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	eval.gateEvaluations.recordPredicate(filtered.Diagnostics...)
	if filtered.Disposition != nil {
		if filtered.Disposition.Policy == PredicateErrorExit {
			output.Status = generationOutputSucceeded
			return output, predicateExitTerminal(filtered.Disposition.Diagnostic), nil
		}
		failed, failureErr := applyPredicateFailureDisposition(
			output,
			node,
			filtered.Disposition.Failure,
			eval.resolved.Definition.Graph,
			eval.topology,
			outputs,
		)
		return failed, nil, failureErr
	}
	materialization, terminal := buildFanOutMaterialization(node, filtered.Candidates, eval.fanOutWidth)
	if terminal != nil {
		return output, terminal, nil
	}
	ref, err := fanOutMaterializationRef(materialization)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	storedRef, runtimePayload, err := generationOutputRefForPayload(
		json.RawMessage(ref),
		outputBlobs,
		eval.now,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputSucceeded
	setGenerationOutputRef(&output, storedRef)
	output.runtimePayload = runtimePayload
	materializeFanOutWindow(
		eval.resolved.Definition.Graph,
		eval.topology,
		eval.generation,
		node.ID,
		materialization,
		outputs,
	)
	return output, nil, nil
}

func fanOutBoundTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusExhausted),
		Cause:      string(TransitionCauseContract),
		ReasonCode: "fan_out_bound_exceeded",
	}
}
