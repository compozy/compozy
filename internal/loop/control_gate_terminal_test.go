package loop

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

type failingCommandRunner struct{}

func (failingCommandRunner) RunCommand(context.Context, gate.CommandRequest) (gate.CommandResult, error) {
	return gate.CommandResult{ExitCode: 1}, nil
}

func TestGateRouteTerminalShouldReconcileHaltFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should honor an authored blocked status", func(t *testing.T) {
		t.Parallel()

		terminal := gateRouteTerminal(
			"phase_gate",
			gate.RouteDecision{Action: gate.RouteHalt, TerminalStatus: "blocked", ReasonCode: "gate_blocked"},
			`{"outcome":"rejected"}`,
		)
		if terminal.Status != string(StatusBlocked) {
			t.Fatalf("terminal.Status = %q, want blocked", terminal.Status)
		}
		if terminal.NodeID != "phase_gate" || terminal.GateID != "phase_gate" {
			t.Fatalf("terminal node/gate = %q/%q, want phase_gate", terminal.NodeID, terminal.GateID)
		}
		if terminal.ReasonCode != "gate_blocked" {
			t.Fatalf("terminal.ReasonCode = %q, want gate_blocked", terminal.ReasonCode)
		}
	})

	t.Run("Should fall back to failed for halt like gate routing", func(t *testing.T) {
		t.Parallel()

		terminal := gateRouteTerminal("phase_gate", gate.RouteDecision{Action: gate.RouteHalt}, "")
		if terminal.Status != string(StatusFailed) {
			t.Fatalf("terminal.Status = %q, want failed", terminal.Status)
		}
	})
}

func TestEvaluateGateNodeShouldSettleAuthoredBlocked(t *testing.T) {
	t.Parallel()

	definition := dsl.Definition{
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "load", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"ready": map[string]any{"value": true}}},
				},
				{
					ID: "phase_gate", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
					Criteria: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "true"}},
					OnResult: map[string]any{
						"fail": map[string]any{"action": "halt", "terminal_status": "blocked"},
					},
				},
				{
					ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"published": map[string]any{"value": true}}},
				},
				{
					ID: "repair", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"repaired": map[string]any{"value": true}}},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "phase_gate"},
				{From: "phase_gate", To: "publish"},
				{From: "phase_gate", To: "repair"},
			},
		},
		Contract: dsl.Contract{
			ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalStates: []dsl.TerminalState{dsl.TerminalBlocked, dsl.TerminalFailed},
			},
		},
	}
	resolved := compileCoordinatorControlDefinition(t, definition)
	topology := newControlTopology(resolved.Definition.Graph)
	outputs := []GenerationOutput{
		{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: `{}`},
		{Generation: 1, NodeID: "phase_gate", Status: generationOutputPending},
		{Generation: 1, NodeID: "publish", Status: generationOutputPending},
		{Generation: 1, NodeID: "repair", Status: generationOutputPending},
	}
	result, terminal, err := evaluateGateNode(
		t.Context(),
		Run{ID: "run-authored-blocked", WorkspaceID: "ws-1"},
		1,
		resolved,
		topology,
		EffectiveConfig{},
		gate.NewEvaluator(gate.WithCommandRunner(failingCommandRunner{})),
		nil,
		nil,
		nil,
		GenerationHistory{},
		outputs[1],
		resolved.Definition.Graph.Nodes[1],
		&outputs,
		&gateEvaluationCollector{},
		time.Now,
	)
	if err != nil {
		t.Fatalf("evaluateGateNode() error = %v", err)
	}
	if terminal == nil {
		t.Fatal("terminal = nil, want blocked")
	}
	if terminal.Status != string(StatusBlocked) {
		t.Fatalf("terminal.Status = %q, want blocked", terminal.Status)
	}
	if terminal.NodeID != "phase_gate" {
		t.Fatalf("terminal.NodeID = %q, want phase_gate", terminal.NodeID)
	}
	if result.Status != generationOutputSucceeded {
		t.Fatalf("gate output status = %q, want succeeded", result.Status)
	}
}

func TestCoordinatorShouldSettleAuthoredBlockedTerminal(t *testing.T) {
	t.Parallel()

	definition := dsl.Definition{
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "load", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"ready": map[string]any{"value": true}}},
				},
				{
					ID: "phase_gate", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
					Criteria: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "true"}},
					OnResult: map[string]any{
						"fail": map[string]any{"action": "halt", "terminal_status": "blocked"},
					},
				},
				{
					ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"published": map[string]any{"value": true}}},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "phase_gate"},
				{From: "phase_gate", To: "publish"},
			},
		},
	}
	loopRun := Run{
		ID: "looprun-authored-blocked-runner", WorkspaceID: "ws-1", LoopName: "delivery",
		Status: StatusRunning, Generation: 1,
	}
	coordinatorRun := task.Run{
		ID: "run-authored-blocked-runner", TaskID: "task-authored-blocked-runner",
		RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusClaimed,
	}
	runner := newCoordinatorRunnerForTestWithDefinition(
		t,
		loopRun,
		coordinatorRun,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
			{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: `{}`},
			{Generation: 1, NodeID: "phase_gate", Status: generationOutputPending},
		}}},
		definition,
		WithCoordinatorGateEvaluator(gateEvaluatorFunc(
			func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
				verdict := testRouteVerdict(gate.RouteHalt)
				verdict.Route.TerminalStatus = "blocked"
				verdict.Route.ReasonCode = "gate_blocked"
				return verdict, nil
			},
		)),
	)

	plan, err := runner.Run(t.Context(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if plan.Terminal == nil {
		t.Fatal("Terminal = nil, want blocked")
	}
	if plan.Terminal.Status != string(StatusBlocked) {
		t.Fatalf("terminal status = %q, want blocked", plan.Terminal.Status)
	}
	if plan.Terminal.NodeID != "phase_gate" {
		t.Fatalf("terminal node_id = %q, want phase_gate", plan.Terminal.NodeID)
	}
}

func TestDefinitionOfDoneGateShouldSettleAuthoredBlocked(t *testing.T) {
	t.Parallel()

	contract := dsl.Contract{
		Verification: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "true"}},
		ContractLifecycleState: &dsl.ContractLifecycleState{
			TerminalStates: []dsl.TerminalState{dsl.TerminalBlocked, dsl.TerminalFailed},
			OnResult: map[string]any{
				"fail": map[string]any{"action": "halt", "terminal_status": "blocked"},
			},
		},
	}
	runtimeGate := gate.GateFromContract(definitionOfDoneGateID, contract, 0)
	verdict, err := gate.NewEvaluator(gate.WithCommandRunner(failingCommandRunner{})).Evaluate(
		t.Context(),
		runtimeGate,
		gate.GateInput{Placement: gate.PlacementDefinitionOfDone, Contract: &contract},
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	terminal, err := terminalFromDefinitionOfDoneVerdict(
		&task.CoordinatorTerminal{Status: string(StatusDone), Cause: string(TransitionCauseContract)},
		verdict,
	)
	if err != nil {
		t.Fatalf("terminalFromDefinitionOfDoneVerdict() error = %v", err)
	}
	if terminal == nil {
		t.Fatal("terminal = nil, want blocked")
	}
	if terminal.Status != string(StatusBlocked) {
		t.Fatalf("terminal.Status = %q, want blocked", terminal.Status)
	}
	if terminal.NodeID != definitionOfDoneGateID {
		t.Fatalf("terminal.NodeID = %q, want %q", terminal.NodeID, definitionOfDoneGateID)
	}
}

func TestDefinitionOfDoneGateShouldKeepNextGenerationByDefault(t *testing.T) {
	t.Parallel()

	contract := dsl.Contract{
		Verification: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "true"}},
	}
	runtimeGate := gate.GateFromContract(definitionOfDoneGateID, contract, 0)
	verdict, err := gate.NewEvaluator(gate.WithCommandRunner(failingCommandRunner{})).Evaluate(
		t.Context(),
		runtimeGate,
		gate.GateInput{Placement: gate.PlacementDefinitionOfDone, Contract: &contract},
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	terminal, err := terminalFromDefinitionOfDoneVerdict(
		&task.CoordinatorTerminal{Status: string(StatusDone), Cause: string(TransitionCauseContract)},
		verdict,
	)
	if err != nil {
		t.Fatalf("terminalFromDefinitionOfDoneVerdict() error = %v", err)
	}
	if terminal != nil {
		t.Fatalf("terminal = %#v, want nil for the default next generation", terminal)
	}
}
