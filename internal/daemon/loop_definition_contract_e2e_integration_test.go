//go:build integration

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/loop/dsl"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

// Invariant: graph authoring fields accepted by the DSL survive validation,
// publication, API reads, and the independently persisted workspace document.
func TestDaemonE2ELoopDefinitionContractPreservesGraphAuthoring(t *testing.T) {
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	definition := graphAuthoringContractDefinition()
	loopPath := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loops/" + url.PathEscape(definition.Meta.Name)

	var validation contract.LoopValidationResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		loopPath+"/validate",
		contract.ValidateLoopRequest{Definition: definition},
		&validation,
	); err != nil {
		t.Fatalf("HTTP validate Loop error = %v", err)
	}
	if !validation.Valid || len(validation.Errors) != 0 {
		t.Fatalf("HTTP validate Loop response = %#v, want valid", validation)
	}

	createLoopViaHTTP(t, ctx, harness, definition)
	var inspected contract.LoopResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, loopPath, nil, &inspected); err != nil {
		t.Fatalf("HTTP inspect Loop error = %v", err)
	}
	assertGraphAuthoringContract(t, dsl.Definition(inspected.Loop.Definition))

	filePath := filepath.Join(
		harness.WorkspaceRoot,
		compozyconfig.DirName,
		compozyconfig.LoopsDirName,
		definition.Meta.Name,
		"loop.yaml",
	)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read persisted Loop definition error = %v", err)
	}
	persisted, err := dsl.Parse(data)
	if err != nil {
		t.Fatalf("parse persisted Loop definition error = %v", err)
	}
	assertGraphAuthoringContract(t, persisted)
}

func graphAuthoringContractDefinition() contract.LoopDefinitionDocument {
	return contract.LoopDefinitionDocument{
		APIVersion:  dsl.APIVersion,
		Kind:        dsl.KindLoop,
		Concurrency: dsl.ConcurrencyForbid,
		Meta: dsl.Meta{
			Name:        "graph-authoring-contract-e2e",
			Description: "Prove graph authoring survives every public persistence seam.",
			Version:     1,
		},
		Inputs: map[string]dsl.Input{
			"tasks": {
				Type: dsl.InputTypeRef, Required: true,
				Ref: &dsl.InputRef{Kind: dsl.InputRefKindSkill},
			},
		},
		Contract: dsl.Contract{
			Goal:             "Process every task after an operator review.",
			DefinitionOfDone: "Every admitted task is processed.",
			IterationCap:     1,
			NoProgress:       dsl.NoProgress{Window: 2},
			Budget: dsl.Budget{
				Tokens:       1_000,
				WallClockSec: 60,
				OnExceeded:   dsl.BudgetExceededHalt,
			},
			ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalStates: []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
			},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:       "load",
					Class:    dsl.NodeClassSource,
					Kind:     string(dsl.SourceInput),
					InputRef: "tasks",
					Produces: dsl.Schema{"items": []any{map[string]any{"title": "string"}}},
				},
				{
					ID:          "fan",
					Class:       dsl.NodeClassControl,
					Kind:        string(dsl.ControlFanOut),
					Collection:  "{{ .nodes.load.output.items }}",
					MaxFanOut:   4,
					MaxParallel: 2,
					BindAs:      "task",
					IndexAs:     "task_index",
					Strategy: &dsl.StrategySpec{
						Kind:      dsl.StrategyBestEffort,
						Threshold: &dsl.StrategyThreshold{Kind: dsl.ThresholdPercent, Percent: 66},
						Missing:   dsl.MissingAcceptable,
					},
				},
				{
					ID:    "apply",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"title": map[string]any{"template": "{{ .task.title }}"}},
					},
					Review: &dsl.ReviewSpec{
						Decisions: []dsl.ReviewDecision{dsl.ReviewDecisionApprove, dsl.ReviewDecisionReject},
						Prompt:    "Review {{ .task.title }} at index {{ .task_index }}.",
					},
				},
			},
			Edges: []dsl.Edge{{From: "load", To: "fan"}, {From: "fan", To: "apply"}},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartHTTP}},
		},
	}
}

func assertGraphAuthoringContract(t testing.TB, definition dsl.Definition) {
	t.Helper()

	if len(definition.Graph.Nodes) != 3 {
		t.Fatalf("persisted graph nodes = %d, want 3", len(definition.Graph.Nodes))
	}
	fan := definition.Graph.Nodes[1]
	if fan.Strategy == nil || fan.Strategy.Kind != dsl.StrategyBestEffort ||
		fan.Strategy.Threshold == nil || fan.Strategy.Threshold.Percent != 66 ||
		fan.Strategy.Missing != dsl.MissingAcceptable {
		t.Fatalf("persisted fan strategy = %#v, want best_effort at 66%%", fan.Strategy)
	}
	if fan.BindAs != "task" || fan.IndexAs != "task_index" {
		t.Fatalf("persisted fan aliases = %q/%q, want task/task_index", fan.BindAs, fan.IndexAs)
	}
	review := definition.Graph.Nodes[2].Review
	if review == nil || review.Prompt == "" || len(review.Decisions) != 2 {
		t.Fatalf("persisted review = %#v, want prompt and two decisions", review)
	}
}
