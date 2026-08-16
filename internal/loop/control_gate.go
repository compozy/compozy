package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
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
	controls NodeControlReader,
	runtimeCatalog WorkspaceRuntimeCatalog,
	history GenerationHistory,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	evaluations *gateEvaluationCollector,
	evaluatedAt time.Time,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	if err := requireGateEvaluator(evaluator, node.ID); err != nil {
		return GenerationOutput{}, nil, err
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
	if err := validateJudgeGateRuntimes(
		ctx,
		runtimeCatalog,
		run.WorkspaceID,
		effective.RuntimeDefaults.Judge,
		runtimeGate.Criteria,
	); err != nil {
		return GenerationOutput{}, nil, err
	}
	humanDecisions, err := loadGateDecisions(ctx, decisions, run, generation, dsl.NodeID(runtimeGate.ID))
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	control, err := loadGateRevisionControl(ctx, controls, run, dsl.NodeID(runtimeGate.ID))
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	gateInput, err := runtimeGateInput(
		run,
		control.GateRevisions[output.ItemIndex],
		resolved,
		effective,
		gate.PlacementInBody,
		humanDecisions,
	)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	verdict, err := evaluator.Evaluate(ctx, runtimeGate, gateInput)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	verdict, err = gate.SanitizeVerdict(verdict)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	if evaluations != nil {
		evaluations.recordWithControl(runtimeGate, output.ItemIndex, verdict, control, evaluatedAt)
	}
	updated, terminal, err := gateOutputFromVerdict(output, node.ID, verdict)
	if err != nil || verdict.Route.Target == "" {
		return updated, terminal, err
	}
	selected := dsl.NodeID(verdict.Route.Target)
	if !containsNodeID(topology.dependents[node.ID], selected) {
		return GenerationOutput{}, nil, fmt.Errorf(
			"%w: gate %q selected non-forward route %q",
			ErrValidation,
			node.ID,
			selected,
		)
	}
	skipUnselectedRoutePaths(resolved.Definition.Graph, topology, node.ID, selected, updated, outputs)
	if evaluations != nil {
		evaluations.recordRoute(routeDecision{
			NodeID: node.ID, ItemIndex: output.ItemIndex, Target: selected,
			Cause: "gate_verdict:" + string(verdict.Outcome),
		})
	}
	return updated, terminal, nil
}

func requireGateEvaluator(evaluator gate.GateEvaluator, nodeID dsl.NodeID) error {
	if evaluator == nil {
		return fmt.Errorf(
			"%w: gate node %q requires a coordinator gate evaluator",
			ErrValidation,
			nodeID,
		)
	}
	return nil
}

func validateJudgeGateRuntimes(
	ctx context.Context,
	factory WorkspaceRuntimeCatalog,
	workspaceID WorkspaceID,
	defaults RuntimeSpec,
	criteria []dsl.GateCriterion,
) error {
	if factory == nil {
		return nil
	}
	catalog, err := factory.ForWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("resolve judge runtime catalog: %w", err)
	}
	if catalog == nil {
		return fmt.Errorf("%w: judge runtime catalog returned nil", ErrActionDependencyMissing)
	}
	for _, criterion := range criteria {
		if criterion.Type != dsl.CriterionAgentJudge {
			continue
		}
		resolved := ResolveJudgeRuntime(defaults, criterion.Runtime)
		if _, err := ValidateResolvedRuntime(ctx, catalog, "", resolved); err != nil {
			return err
		}
	}
	return nil
}

func runtimeGateInput(
	run Run,
	revision int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	placement gate.Placement,
	humanDecisions map[string]gate.HumanDecision,
) (gate.GateInput, error) {
	if humanDecisions == nil {
		humanDecisions = map[string]gate.HumanDecision{}
	}
	contract, err := MaterializeContract(resolved.Definition.Contract, run.Inputs)
	if err != nil {
		return gate.GateInput{}, err
	}
	return gate.GateInput{
		LoopRunID:            string(run.ID),
		Placement:            placement,
		Contract:             &contract,
		Revision:             max(0, revision),
		BestScore:            cloneFloat64(run.BestScore),
		HumanDecisions:       humanDecisions,
		JudgeRuntime:         effective.RuntimeDefaults.Judge,
		NetworkParticipation: new(run.NetworkSpecSnapshot()),
		ToolScope: tools.Scope{
			WorkspaceID: string(run.WorkspaceID),
			ActorKind:   startLoopMetaKey,
		},
	}, nil
}

func loadGateRevisionControl(
	ctx context.Context,
	reader NodeControlReader,
	run Run,
	gateID dsl.NodeID,
) (NodeControl, error) {
	if reader == nil {
		return NodeControl{}, nil
	}
	controls, err := reader.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return NodeControl{}, fmt.Errorf("loop: list gate revision controls: %w", err)
	}
	for _, control := range controls {
		if control.NodeID == NodeID(gateID) {
			return control, nil
		}
	}
	return NodeControl{}, nil
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
	setGenerationOutputRef(&output, ref)
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
	setGenerationOutputRef(&output, ref)
	switch verdict.Route.Action {
	case gate.RouteContinue, gate.RouteDone:
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
	if criterion.Check, err = renderCommandGateString(prefix+".check", criterion.Check, namespace); err != nil {
		return dsl.GateCriterion{}, err
	}
	if criterion.Contains, err = renderGateString(prefix+".contains", criterion.Contains, namespace); err != nil {
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
	renderedInputs, err := renderActionParam(prefix+".inputs", criterion.Inputs, namespace)
	if err != nil {
		return dsl.GateCriterion{}, fmt.Errorf("render gate %s.inputs: %w", prefix, err)
	}
	if renderedInputs != nil {
		inputs, ok := renderedInputs.(map[string]any)
		if !ok {
			return dsl.GateCriterion{}, fmt.Errorf("%w: gate %s.inputs must be an object", ErrValidation, prefix)
		}
		criterion.Inputs = inputs
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

func renderCommandGateString(name string, raw string, namespace map[string]any) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, nil
	}
	rendered, err := refs.RenderCommandTemplateString(name, raw, namespace)
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
