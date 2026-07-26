package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
)

func evaluateGateNode(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	effective EffectiveConfig,
	evaluator gate.GateEvaluator,
	decisions GateDecisionReader,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	if evaluator == nil {
		return GenerationOutput{}, nil, fmt.Errorf(
			"%w: gate node %q requires a coordinator gate evaluator",
			ErrValidation,
			node.ID,
		)
	}
	namespace, err := runtimeNamespace(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	runtimeGate, err := renderGateNode(node, namespace)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	runtimeGate, empty, err := applyEffectiveGateConfig(runtimeGate, effective)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	if empty {
		return approveGateOutput(output)
	}
	humanDecisions, err := loadGateDecisions(ctx, decisions, run, generation, dsl.NodeID(runtimeGate.ID))
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	verdict, err := evaluator.Evaluate(ctx, runtimeGate, runtimeGateInput(
		run,
		generation,
		resolved,
		effective,
		namespace,
		gate.PlacementInBody,
		humanDecisions,
	))
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	return gateOutputFromVerdict(output, node.ID, verdict)
}

func runtimeGateInput(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	namespace map[string]any,
	placement gate.Placement,
	humanDecisions map[string]gate.HumanDecision,
) gate.GateInput {
	if humanDecisions == nil {
		humanDecisions = map[string]gate.HumanDecision{}
	}
	return gate.GateInput{
		LoopRunID:            string(run.ID),
		Placement:            placement,
		Contract:             new(resolved.Definition.Contract),
		TemplateData:         namespace,
		Revision:             max(0, generation-1),
		HumanDecisions:       humanDecisions,
		JudgeModel:           effective.ModelDefaults.Judge,
		NetworkParticipation: new(run.NetworkSpecSnapshot()),
		ToolScope: tools.Scope{
			WorkspaceID: string(run.WorkspaceID),
			ActorKind:   startLoopMetaKey,
		},
	}
}

func loadGateDecisions(
	ctx context.Context,
	decisions GateDecisionReader,
	run Run,
	generation int,
	gateID dsl.NodeID,
) (map[string]gate.HumanDecision, error) {
	if decisions == nil {
		return map[string]gate.HumanDecision{}, nil
	}
	return decisions.ListLoopGateDecisions(ctx, run.WorkspaceID, run.ID, generation, gateID)
}

func approveGateOutput(output GenerationOutput) (GenerationOutput, *task.CoordinatorTerminal, error) {
	ref, err := gateApprovedOutputRef()
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputSucceeded
	output.OutputRef = ref
	return output, nil, nil
}

func gateOutputFromVerdict(
	output GenerationOutput,
	nodeID dsl.NodeID,
	verdict gate.Verdict,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	ref, err := gateVerdictOutputRef(verdict)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.OutputRef = ref
	switch verdict.Route.Action {
	case gate.RouteContinue, gate.RouteDone, gate.RouteBranch:
		output.Status = generationOutputSucceeded
		return output, nil, nil
	case gate.RouteRevise, gate.RouteNextGeneration:
		output.Status = generationOutputFailed
		return output, nil, nil
	case gate.RouteEscalate, gate.RouteHalt:
		output.Status = generationOutputSucceeded
		return output, gateRouteTerminal(nodeID, verdict.Route, ref), nil
	default:
		return GenerationOutput{}, nil, fmt.Errorf(
			"%w: gate node %q returned unsupported route action %q",
			ErrValidation,
			nodeID,
			verdict.Route.Action,
		)
	}
}

func renderGateNode(node dsl.Node, namespace map[string]any) (gate.Gate, error) {
	runtimeGate := gate.GateFromNode(node)
	for index := range runtimeGate.Criteria {
		criterion, err := renderGateCriterion(node.ID, runtimeGate.Criteria[index], namespace)
		if err != nil {
			return gate.Gate{}, err
		}
		runtimeGate.Criteria[index] = criterion
	}
	return runtimeGate, nil
}

func renderGateCriterion(
	nodeID dsl.NodeID,
	criterion dsl.GateCriterion,
	namespace map[string]any,
) (dsl.GateCriterion, error) {
	var err error
	prefix := fmt.Sprintf("nodes.%s.criteria.%s", nodeID, criterion.ID)
	if criterion.Check, err = renderGateString(prefix+".check", criterion.Check, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	if criterion.Agent, err = renderGateString(prefix+".agent", criterion.Agent, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	if criterion.Rubric, err = renderGateString(prefix+".rubric", criterion.Rubric, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	if criterion.Prompt, err = renderGateString(prefix+".prompt", criterion.Prompt, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	if criterion.Tool, err = renderGateString(prefix+".tool", criterion.Tool, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	return criterion, nil
}

func renderGateString(name string, raw string, namespace map[string]any) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, nil
	}
	rendered, err := refs.RenderTemplateString(name, raw, namespace)
	if err != nil {
		return "", fmt.Errorf("render gate %s: %w", name, err)
	}
	return rendered, nil
}

func gateVerdictOutputRef(verdict gate.Verdict) (string, error) {
	data, err := json.Marshal(verdict)
	if err != nil {
		return "", fmt.Errorf("marshal gate verdict output: %w", err)
	}
	return string(data), nil
}

func gateRouteTerminal(gateID dsl.NodeID, route gate.RouteDecision, details string) *task.CoordinatorTerminal {
	status := strings.TrimSpace(route.TerminalStatus)
	if status == "" {
		switch route.Action {
		case gate.RouteEscalate:
			status = string(StatusNeedsApproval)
		case gate.RouteHalt:
			status = string(StatusBlocked)
		default:
			status = string(StatusFailed)
		}
	}
	reasonCode := strings.TrimSpace(route.ReasonCode)
	if reasonCode == "" {
		reasonCode = "gate_" + string(route.Action)
	}
	return &task.CoordinatorTerminal{
		Status:     status,
		Cause:      string(TransitionCauseGateRejected),
		ReasonCode: reasonCode,
		GateID:     string(gateID),
		Details:    json.RawMessage(details),
	}
}
