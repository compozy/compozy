package loop_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/tools/builtin"
)

func TestLinterShouldRejectDeletedNoProgressParameters(t *testing.T) {
	t.Parallel()

	definition, err := dsl.Parse([]byte(`
apiVersion: compozy.loop/v1
kind: Loop
contract:
  no_progress:
    window: 2
    hash_fields: [artifact]
graph:
  nodes: []
  edges: []
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	errList := loop.NewLinter().Lint(definition)
	requireLintCodes(t, errList, loop.CodeUnknownParameter, loop.CodeNonTerminatingStructure)
	requireLintMessageContains(t, errList, loop.CodeUnknownParameter, "contract.no_progress.hash_fields")
}

// Invariant: every declared input has a closed shape and a valid author-owned default.
// The canonical linter suite owns Loop input declaration diagnostics.
func TestLinterShouldValidateInputDeclarations(t *testing.T) {
	t.Parallel()

	valid := validDefinition()
	valid.Inputs = map[string]dsl.Input{
		"environment": {
			Type: dsl.InputTypeString, Enum: []string{"dev", "prod"}, Default: "dev",
		},
		"reviewer": {Type: dsl.InputTypeAgent, Default: "reviewer"},
		"runtime": {
			Type: dsl.InputTypeRuntime, Default: map[string]any{"model": "gpt-5"},
		},
		"skill": {
			Type: dsl.InputTypeRef, Ref: &dsl.InputRef{Kind: dsl.InputRefKindSkill},
		},
		"tasks": {
			Type: dsl.InputTypeRef, Ref: &dsl.InputRef{Kind: dsl.InputRefKindSkill}, Required: true,
		},
	}
	if _, err := loop.NewCompiler().Compile(valid); err != nil {
		t.Fatalf("Compile(valid inputs) error = %v", err)
	}

	for _, testCase := range []struct {
		name   string
		path   string
		code   string
		mutate func(*dsl.Definition)
	}{
		{
			name: "Should require an input type", path: "inputs.environment.type",
			code: loop.CodeInputTypeRequired,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["environment"]
				input.Type = ""
				definition.Inputs["environment"] = input
			},
		},
		{
			name: "Should reject an unknown input field", path: "inputs.environment.kind",
			code: loop.CodeInputFieldUnknown,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["environment"]
				input.Extra = map[string]any{"kind": "string"}
				definition.Inputs["environment"] = input
			},
		},
		{
			name: "Should require ref kind", path: "inputs.skill.ref",
			code: loop.CodeInputRefRequired,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["skill"]
				input.Ref = nil
				definition.Inputs["skill"] = input
			},
		},
		{
			name: "Should reject an unknown ref kind", path: "inputs.skill.ref.kind",
			code: loop.CodeInputRefKindInvalid,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["skill"]
				input.Ref = &dsl.InputRef{Kind: "recipe"}
				definition.Inputs["skill"] = input
			},
		},
		{
			name: "Should reject enum outside string inputs", path: "inputs.environment.enum",
			code: loop.CodeInputEnumInvalid,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["environment"]
				input.Type = dsl.InputTypeAgent
				definition.Inputs["environment"] = input
			},
		},
		{
			name: "Should reject a default outside the enum", path: "inputs.environment.default",
			code: loop.CodeInputDefaultInvalid,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["environment"]
				input.Default = "staging"
				definition.Inputs["environment"] = input
			},
		},
		{
			name: "Should reject an unknown runtime field", path: "inputs.runtime.default",
			code: loop.CodeInputDefaultInvalid,
			mutate: func(definition *dsl.Definition) {
				input := definition.Inputs["runtime"]
				input.Default = map[string]any{"turbo_mode": "fast"}
				definition.Inputs["runtime"] = input
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			definition := valid
			definition.Inputs = maps.Clone(valid.Inputs)
			testCase.mutate(&definition)
			requireLintPathDiagnostic(
				t,
				loop.NewLinter().Lint(definition),
				testCase.path,
				testCase.code,
			)
		})
	}
}

// Invariant: ask controls declare a valid answer shape and only route expiry forward.
// The canonical linter suite owns authoring diagnostics and output-schema derivation.
func TestLinterShouldValidateAskControls(t *testing.T) {
	t.Parallel()

	valid := dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "ask-lint"},
		Contract: dsl.Contract{
			Goal: "Select an environment", DefinitionOfDone: "Selection recorded", IterationCap: 2,
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "select", Class: dsl.NodeClassControl, Kind: string(dsl.ControlAsk),
					Params: dsl.NodeParams{
						"prompt": "Which environment?",
						"expect": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"environment": map[string]any{
									"type": "string", "x-compozy-kind": "workspace",
								},
							},
						},
						"responders": map[string]any{"agents": "allow"},
						"expires":    map[string]any{"after": "1h", "route": "expired"},
					},
				},
				{ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{
						"environment": map[string]any{"template": "{{ .nodes.select.output.environment }}"},
					}}},
				{ID: "expired", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"expired": map[string]any{"value": true}}}},
			},
			Edges: []dsl.Edge{{From: "select", To: "publish"}, {From: "select", To: "expired"}},
		},
	}
	if _, err := loop.NewCompiler().Compile(valid); err != nil {
		t.Fatalf("Compile(valid ask) error = %v", err)
	}

	for _, tt := range []struct {
		name string
		edit func(*dsl.Definition)
		code string
	}{
		{
			name: "Should require an answer shape",
			edit: func(definition *dsl.Definition) { delete(definition.Graph.Nodes[0].Params, "expect") },
			code: loop.CodeAskExpectRequired,
		},
		{
			name: "Should reject a zero expiry duration",
			edit: func(definition *dsl.Definition) {
				definition.Graph.Nodes[0].Params["expires"] = map[string]any{"after": "0s", "route": "expired"}
			},
			code: loop.CodeDurationInvalid,
		},
		{
			name: "Should reject a non-forward expiry route",
			edit: func(definition *dsl.Definition) {
				definition.Graph.Nodes[0].Params["expires"] = map[string]any{"after": "1h", "route": "missing"}
			},
			code: loop.CodeErrorRouteBackward,
		},
		{
			name: "Should reject an unknown responder policy",
			edit: func(definition *dsl.Definition) {
				definition.Graph.Nodes[0].Params["responders"] = map[string]any{"agents": "sometimes"}
			},
			code: loop.CodeResponderPolicyInvalid,
		},
		{
			name: "Should reject an unknown entity annotation",
			edit: func(definition *dsl.Definition) {
				definition.Graph.Nodes[0].Params["expect"] = map[string]any{
					"type": "object",
					"properties": map[string]any{
						"environment": map[string]any{
							"type": "string", "x-compozy-kind": "recipe",
						},
					},
				}
			},
			code: loop.CodeRequestEntityKindInvalid,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			definition := valid
			definition.Graph.Nodes = append([]dsl.Node(nil), valid.Graph.Nodes...)
			definition.Graph.Nodes[0].Params = maps.Clone(valid.Graph.Nodes[0].Params)
			tt.edit(&definition)
			requireLintDiagnostic(t, loop.NewLinter().Lint(definition), tt.code, loop.SeverityError)
		})
	}
}

// Invariant: action reviews declare a closed decision set, forward reject route, and an output shape for respond.
// The canonical linter suite owns review grammar and its compiled CEL condition.
func TestLinterShouldValidateActionReviews(t *testing.T) {
	t.Parallel()

	definition := validDefinition()
	agent := requireNode(t, &definition, "agent")
	agent.Review = &dsl.ReviewSpec{
		Decisions: []dsl.ReviewDecision{
			dsl.ReviewDecisionApprove, dsl.ReviewDecisionEdit,
			dsl.ReviewDecisionReject, dsl.ReviewDecisionRespond,
		},
		When:   `inputs.tasks != null`,
		Prompt: "Review {{ .inputs.tasks }}",
	}
	if _, err := loop.NewCompiler().Compile(definition); err != nil {
		t.Fatalf("Compile(valid review) error = %v", err)
	}

	t.Run("Should require a declared output shape for respond", func(t *testing.T) {
		t.Parallel()

		invalid := validDefinition()
		node := requireNode(t, &invalid, "agent")
		delete(node.Params, "output_schema")
		node.Review = &dsl.ReviewSpec{Decisions: []dsl.ReviewDecision{dsl.ReviewDecisionRespond}}
		requireLintDiagnostic(
			t,
			loop.NewLinter().Lint(invalid),
			loop.CodeReviewRespondSchemaRequired,
			loop.SeverityError,
		)
	})

	t.Run("Should reject a non-forward rejection route", func(t *testing.T) {
		t.Parallel()

		invalid := validDefinition()
		node := requireNode(t, &invalid, "agent")
		node.Review = &dsl.ReviewSpec{OnReject: &dsl.RejectPolicy{Route: "missing"}}
		requireLintDiagnostic(t, loop.NewLinter().Lint(invalid), loop.CodeErrorRouteBackward, loop.SeverityError)
	})

	t.Run("Should reject an unknown entity kind in an external tool edit schema", func(t *testing.T) {
		t.Parallel()

		invalid := validDefinition()
		node := requireNode(t, &invalid, "agent")
		node.Kind = "compozy__custom_publish"
		node.Review = &dsl.ReviewSpec{Decisions: []dsl.ReviewDecision{dsl.ReviewDecisionEdit}}
		linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{
			node.Kind: {
				ToolID: node.Kind,
				InputSchema: mustJSON(t, map[string]any{
					"type": "object",
					"properties": map[string]any{
						"reviewer": map[string]any{
							"type": "string", "x-compozy-kind": "recipe",
						},
					},
				}),
			},
		}))
		requireLintDiagnostic(
			t,
			linter.Lint(invalid),
			loop.CodeRequestEntityKindInvalid,
			loop.SeverityError,
		)
	})
}

func TestLinterShouldValidatePredicateErrorPolicies(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		mutate func(*dsl.Definition)
	}{
		{
			name: "Should reject an unknown predicate error policy",
			mutate: func(definition *dsl.Definition) {
				node := requireNode(t, definition, "agent")
				node.Condition = "true"
				node.OnEvalError = "retry"
			},
		},
		{
			name: "Should reject a policy without a condition or filter",
			mutate: func(definition *dsl.Definition) {
				requireNode(t, definition, "agent").OnEvalError = dsl.EvalErrorExit
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			definition := validDefinition()
			tt.mutate(&definition)
			requireLintCodes(t, loop.NewLinter().Lint(definition), loop.CodeEvalErrorPolicyInvalid)
		})
	}
}

func TestLinterShouldValidateExclusiveRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should compile ordered routes and record every condition", func(t *testing.T) {
		t.Parallel()

		definition := routeDefinitionForTest()
		resolved, err := loop.NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		for _, key := range []string{"nodes.router.routes.0.when", "nodes.router.routes.1.when"} {
			if resolved.Conditions[key] == nil {
				t.Fatalf("resolved condition %q = nil", key)
			}
		}
	})

	t.Run("Should accept one conditional route plus default", func(t *testing.T) {
		t.Parallel()

		definition := routeDefinitionForTest()
		router := requireNode(t, &definition, "router")
		router.Routes = router.Routes[:1]
		router.Default = "review"
		definition.Graph.Edges = definition.Graph.Edges[:2]
		definition.Graph.Nodes = definition.Graph.Nodes[:3]
		requireLintCodes(t, loop.NewLinter().Lint(definition))
	})

	for _, tt := range []struct {
		name     string
		mutate   func(*dsl.Definition)
		wantCode string
		message  string
	}{
		{
			name: "Should require a default destination",
			mutate: func(definition *dsl.Definition) {
				router := requireNode(t, definition, "router")
				router.Default = ""
				router.Routes = append(router.Routes, dsl.RouteSpec{When: "false", To: "fallback"})
			},
			wantCode: loop.CodeRouteDefaultMissing,
			message:  "router",
		},
		{
			name: "Should reject a backward or unknown destination",
			mutate: func(definition *dsl.Definition) {
				requireNode(t, definition, "router").Routes[0].To = "missing"
			},
			wantCode: loop.CodeRouteTargetInvalid,
			message:  "missing",
		},
		{
			name: "Should reject duplicate destinations",
			mutate: func(definition *dsl.Definition) {
				requireNode(t, definition, "router").Default = "quick"
			},
			wantCode: loop.CodeRouteTargetInvalid,
			message:  "duplicates",
		},
		{
			name: "Should reject an unknown condition reference",
			mutate: func(definition *dsl.Definition) {
				requireNode(t, definition, "router").Routes[0].When = "nodes.missing.output.x == 1"
			},
			wantCode: refs.CodeUnknownReference,
			message:  "available",
		},
		{
			name: "Should reject an exit policy because route conditions fail closed",
			mutate: func(definition *dsl.Definition) {
				requireNode(t, definition, "router").OnEvalError = dsl.EvalErrorExit
			},
			wantCode: loop.CodeEvalErrorPolicyInvalid,
			message:  "fail-closed",
		},
		{
			name: "Should reject conflicting gate route objects",
			mutate: func(definition *dsl.Definition) {
				router := requireNode(t, definition, "router")
				router.Kind = string(dsl.ControlGate)
				router.Routes = nil
				router.Default = ""
				router.Criteria = []dsl.GateCriterion{{ID: "verify", Type: dsl.CriterionCommand, Check: "true"}}
				router.OnResult = map[string]any{
					"fail": map[string]any{"route": "quick", "action": "revise"},
				}
			},
			wantCode: loop.CodeRouteMappingInvalid,
			message:  "exactly one",
		},
		{
			name: "Should reject the removed branch action",
			mutate: func(definition *dsl.Definition) {
				router := requireNode(t, definition, "router")
				router.Kind = string(dsl.ControlGate)
				router.Routes = nil
				router.Default = ""
				router.Criteria = []dsl.GateCriterion{{ID: "verify", Type: dsl.CriterionCommand, Check: "true"}}
				router.OnResult = map[string]any{"fail": "branch"}
			},
			wantCode: loop.CodeRouteActionRemoved,
			message:  "{route: node_id}",
		},
		{
			name: "Should reject an approval object route that bypasses the pending decision",
			mutate: func(definition *dsl.Definition) {
				router := requireNode(t, definition, "router")
				router.Kind = string(dsl.ControlGate)
				router.Routes = nil
				router.Default = ""
				router.Criteria = []dsl.GateCriterion{{
					ID: "operator", Type: dsl.CriterionHuman, Prompt: "Approve release?",
				}}
				router.OnResult = map[string]any{
					"approval": map[string]any{"route": "quick"},
				}
			},
			wantCode: loop.CodeRouteMappingInvalid,
			message:  "cannot bypass a pending approval",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			definition := routeDefinitionForTest()
			tt.mutate(&definition)
			errors := loop.NewLinter().Lint(definition)
			requireLintCodes(t, errors, tt.wantCode)
			requireLintMessageContains(t, errors, tt.wantCode, tt.message)
		})
	}
}

func TestMetricShouldParseAndLintGateCriteria(t *testing.T) {
	t.Parallel()

	t.Run("Should parse a metric direction with an unset minimum delta", func(t *testing.T) {
		t.Parallel()

		definition, err := dsl.Parse([]byte(`
apiVersion: compozy.loop/v1
kind: Loop
contract:
  verification:
    - id: quality
      type: agent-judge
      metric:
        direction: maximize
`))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		metric := definition.Contract.Verification[0].Metric
		if metric == nil {
			t.Fatal("metric = nil, want parsed metric")
		}
		if metric.Direction != dsl.MetricMaximize {
			t.Fatalf("metric direction = %q, want %q", metric.Direction, dsl.MetricMaximize)
		}
		if metric.MinDelta != nil {
			t.Fatalf("metric min_delta = %v, want nil", *metric.MinDelta)
		}
	})

	t.Run("Should preserve an explicit zero minimum delta", func(t *testing.T) {
		t.Parallel()

		definition, err := dsl.Parse([]byte(`
apiVersion: compozy.loop/v1
kind: Loop
contract:
  verification:
    - id: quality
      type: command
      metric:
        direction: minimize
        min_delta: 0
`))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		metric := definition.Contract.Verification[0].Metric
		if metric == nil || metric.MinDelta == nil {
			t.Fatal("metric min_delta = nil, want explicit zero")
		}
		if *metric.MinDelta != 0 {
			t.Fatalf("metric min_delta = %v, want 0", *metric.MinDelta)
		}
	})

	for _, rawDelta := range []string{".nan", ".inf"} {
		t.Run("Should reject a parsed non-finite minimum delta "+rawDelta, func(t *testing.T) {
			t.Parallel()

			definition, err := dsl.Parse([]byte(`
apiVersion: compozy.loop/v1
kind: Loop
contract:
  verification:
    - id: quality
      type: command
      metric:
        direction: maximize
        min_delta: ` + rawDelta))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			metric := definition.Contract.Verification[0].Metric
			if metric == nil || metric.MinDelta == nil {
				t.Fatal("metric min_delta = nil, want parsed non-finite value")
			}
			valid := validDefinition()
			appendMetricGate(&valid, dsl.CriterionCommand, metric)
			requireLintCodes(t, loop.NewLinter().Lint(valid), loop.CodeMetricMinDeltaInvalid)
		})
	}

	tests := []struct {
		name      string
		mutate    func(*dsl.Definition)
		wantCode  string
		wantInMsg string
	}{
		{
			name: "Should reject a metric on a human criterion",
			mutate: func(def *dsl.Definition) {
				appendMetricGate(def, dsl.CriterionHuman, &dsl.MetricSpec{Direction: dsl.MetricMaximize})
			},
			wantCode: loop.CodeMetricMachineCriterionRequired,
		},
		{
			name: "Should reject multiple metrics across contract and graph criteria",
			mutate: func(def *dsl.Definition) {
				def.Contract.Verification = []dsl.GateCriterion{{
					ID:     "contract_quality",
					Type:   dsl.CriterionCommand,
					Check:  "make verify",
					Metric: &dsl.MetricSpec{Direction: dsl.MetricMaximize},
				}}
				appendMetricGate(def, dsl.CriterionCommand, &dsl.MetricSpec{Direction: dsl.MetricMaximize})
			},
			wantCode: loop.CodeMetricSingle,
		},
		{
			name: "Should reject a metric without a direction",
			mutate: func(def *dsl.Definition) {
				appendMetricGate(def, dsl.CriterionCommand, &dsl.MetricSpec{MinDelta: new(0.1)})
			},
			wantCode: loop.CodeMetricDirectionRequired,
		},
		{
			name: "Should reject an unknown metric direction",
			mutate: func(def *dsl.Definition) {
				appendMetricGate(def, dsl.CriterionExtension, &dsl.MetricSpec{Direction: "sideways"})
			},
			wantCode:  loop.CodeMetricDirectionInvalid,
			wantInMsg: "sideways",
		},
		{
			name: "Should reject a negative metric minimum delta",
			mutate: func(def *dsl.Definition) {
				appendMetricGate(def, dsl.CriterionCommand, &dsl.MetricSpec{
					Direction: dsl.MetricMaximize,
					MinDelta:  new(-0.1),
				})
			},
			wantCode: loop.CodeMetricMinDeltaInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			definition := validDefinition()
			tt.mutate(&definition)
			errs := loop.NewLinter().Lint(definition)
			requireLintCodes(t, errs, tt.wantCode)
			if tt.wantInMsg != "" {
				requireLintMessageContains(t, errs, tt.wantCode, tt.wantInMsg)
			}
		})
	}

	t.Run("Should exclude Goal turn judges from the definition metric limit", func(t *testing.T) {
		t.Parallel()

		definition := validGoalDefinition()
		requireNode(t, &definition, "converge").Params["judge"] = []any{
			map[string]any{
				"id":     "turn_one",
				"type":   "command",
				"check":  "make verify",
				"metric": map[string]any{"direction": "maximize"},
			},
			map[string]any{
				"id":     "turn_two",
				"type":   "command",
				"check":  "make test",
				"metric": map[string]any{"direction": "minimize"},
			},
		}
		requireLintCodes(t, loop.NewLinter().Lint(definition))
	})
}

func TestLinterShouldValidateGenerationHistoryReferences(t *testing.T) {
	t.Parallel()

	t.Run("Should accept previous and best history references", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		appendGate(&definition, dsl.Node{
			ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
			Criteria: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "make verify"}},
		})
		definition.Contract.StopWhen = dsl.StopWhenSpec{Expr: "previous.generation >= 1 && best.score >= 0"}
		requireNode(t, &definition, "agent").Params["prompt"] =
			"{{ with .previous }}Repair {{ with .verdicts.quality }}{{ .blocking_issues }}{{ end }} " +
				"from {{ .nodes.agent.output.summary }}{{ end }}"
		requireLintCodes(t, loop.NewLinter().Lint(definition))
	})

	t.Run("Should reject the lossy singular previous verdict reference", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		requireNode(t, &definition, "agent").Params["prompt"] = "{{ .previous.verdict.blocking_issues }}"
		requireLintCodes(t, loop.NewLinter().Lint(definition), refs.CodeUnknownReference)
	})

	for _, reference := range []struct {
		name        string
		path        string
		asCondition bool
	}{
		{
			name: "Should reject a child below previous node status",
			path: "previous.nodes.agent.status.code",
		},
		{
			name:        "Should reject a child below previous verdict outcome",
			path:        "previous.verdicts.quality.outcome.code",
			asCondition: true,
		},
	} {
		t.Run(reference.name, func(t *testing.T) {
			t.Parallel()

			definition := validDefinition()
			appendGate(&definition, dsl.Node{
				ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
				Criteria: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "make verify"}},
			})
			if reference.asCondition {
				definition.Contract.StopWhen = dsl.StopWhenSpec{Expr: reference.path + " != null"}
			} else {
				requireNode(t, &definition, "agent").Params["prompt"] = "{{ ." + reference.path + " }}"
			}
			requireLintCodes(t, loop.NewLinter().Lint(definition), refs.CodeUnresolvablePath)
		})
	}
}

func TestLinterShouldValidateWaitEventFilterReferences(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unavailable node output references in wait event filters", func(t *testing.T) {
		t.Parallel()

		definition := singleNodeDefinition(dsl.Node{
			ID: "wait_for_task", Class: dsl.NodeClassControl, Kind: string(dsl.ControlWait),
			Params: dsl.NodeParams{"event": map[string]any{
				"kind":   "task.status_changed",
				"filter": `event.task_id == nodes.producer.output.task_id`,
			}},
		})
		definition.Graph.Nodes = append(definition.Graph.Nodes, dsl.Node{
			ID: "producer", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{"agent": "codex", "prompt": "Produce a task id"},
		})
		definition.Graph.Edges = append(definition.Graph.Edges, dsl.Edge{From: "wait_for_task", To: "producer"})
		errs := loop.NewLinter().Lint(definition)
		requireLintDiagnostic(t, errs, refs.CodeUnresolvablePath, loop.SeverityWarning)
	})
}

func TestLinterShouldRejectStructuralAndReferenceInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*dsl.Definition)
		wantCodes []string
	}{
		{
			name: "Should accept valid finite graph",
		},
		{
			name: "Should accept a direct reference to a declared runtime input",
			mutate: func(def *dsl.Definition) {
				def.Inputs["worker_runtime"] = dsl.Input{Type: dsl.InputTypeRuntime, Required: true}
				requireNode(t, def, "agent").Params["runtime"] = "{{ .inputs.worker_runtime }}"
			},
		},
		{
			name: "Should reject graph cycles",
			mutate: func(def *dsl.Definition) {
				def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "load"})
			},
			wantCodes: []string{loop.CodeCycle},
		},
		{
			name: "Should reject unreachable nodes",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
					ID:    "orphan",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{
							"value": map[string]any{"value": "literal"},
						},
					},
				})
			},
			wantCodes: []string{loop.CodeUnreachableNode},
		},
		{
			name: "Should reject zero-edge nodes that are unreachable from an explicit source",
			mutate: func(def *dsl.Definition) {
				def.Graph.Edges = nil
			},
			wantCodes: []string{loop.CodeUnreachableNode},
		},
		{
			name: "Should reject empty non terminating graph",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = nil
				def.Graph.Edges = nil
			},
			wantCodes: []string{loop.CodeNonTerminatingStructure},
		},
		{
			name: "Should reject unbounded fan out",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.MaxFanOut = 0
			},
			wantCodes: []string{loop.CodeFanOutUnbounded},
		},
		{
			name: "Should reject fan out collection literals without references",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.Collection = "nodes.load.output.items"
			},
			wantCodes: []string{loop.CodeFanOutUnbounded},
		},
		{
			name: "Should accept item and index in the fan out filter",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "fan").Filter = `item.title != "" && index >= 0`
			},
		},
		{
			name: "Should accept the fan out iteration aliases in its filter",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.BindAs = "task_item"
				node.IndexAs = "task_index"
				node.Filter = `task_item.title != "" && task_index >= 0`
				node.OnEvalError = dsl.EvalErrorExit
			},
		},
		{
			name: "Should reject an undeclared iteration alias in the fan out filter",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "fan").Filter = `undeclared_item.title != ""`
			},
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should require explicit acceptable missing coverage for best effort",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.Strategy = &dsl.StrategySpec{
					Kind: dsl.StrategyBestEffort,
					Threshold: &dsl.StrategyThreshold{
						Kind: dsl.ThresholdPercent, Percent: 66,
					},
				}
			},
			wantCodes: []string{loop.CodeStrategyCoverageUndeclared},
		},
		{
			name: "Should reject invalid strategy thresholds",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.Strategy = &dsl.StrategySpec{
					Kind: dsl.StrategyBestEffort,
					Threshold: &dsl.StrategyThreshold{
						Kind: dsl.ThresholdCount, Count: 0,
					},
					Missing: dsl.MissingAcceptable,
				}
			},
			wantCodes: []string{loop.CodeStrategyThresholdInvalid},
		},
		{
			name: "Should reject reserved iteration names",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "fan").BindAs = "previous"
			},
			wantCodes: []string{loop.CodeIterationNameConflict},
		},
		{
			name: "Should reject duplicate iteration names",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.BindAs = "entry"
				node.IndexAs = "entry"
			},
			wantCodes: []string{loop.CodeIterationNameConflict},
		},
		{
			name: "Should reject iteration names shadowed by nested fan outs",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "fan").BindAs = "entry"
				def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
					ID: "inner", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut),
					Collection: "{{ json .item.children }}", MaxFanOut: 4, BindAs: "entry",
				})
				def.Graph.Edges = []dsl.Edge{
					{From: "load", To: "fan"}, {From: "fan", To: "inner"}, {From: "inner", To: "agent"},
				}
			},
			wantCodes: []string{loop.CodeIterationNameConflict},
		},
		{
			name: "Should reject iteration aliases outside their fan out body",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "fan").BindAs = "file"
				def.Graph.Nodes = append(def.Graph.Nodes,
					dsl.Node{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
					dsl.Node{ID: "after", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
						Params: dsl.NodeParams{"agent": "codex", "prompt": "Use {{ .file }}"}},
				)
				def.Graph.Edges = []dsl.Edge{
					{From: "load", To: "fan"}, {From: "fan", To: "agent"},
					{From: "agent", To: "collect"}, {From: "collect", To: "after"},
				}
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject gate max revisions over ceiling",
			mutate: func(def *dsl.Definition) {
				appendGate(def, dsl.Node{
					ID:            "review_gate",
					Class:         dsl.NodeClassControl,
					Kind:          string(dsl.ControlGate),
					MaxRevisions:  loop.LoopMaxGateRevisions + 1,
					VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
					Criteria: []dsl.GateCriterion{
						{
							ID:     "judge",
							Type:   dsl.CriterionAgentJudge,
							Agent:  "critic",
							Rubric: "Approve only correct work",
						},
					},
				})
			},
			wantCodes: []string{loop.CodeGateMaxRevisionsCeilingExceeded},
		},
		{
			name: "Should reject revise until clean gate without judge",
			mutate: func(def *dsl.Definition) {
				appendGate(def, dsl.Node{
					ID:            "review_gate",
					Class:         dsl.NodeClassControl,
					Kind:          string(dsl.ControlGate),
					MaxRevisions:  1,
					VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
					Criteria: []dsl.GateCriterion{
						{
							ID:     "unit",
							Type:   dsl.CriterionCommand,
							Check:  "make test",
							Expect: "exit 0",
						},
					},
				})
			},
			wantCodes: []string{loop.CodeVerdictPolicyRequiresJudge},
		},
		{
			name: "Should reject node ids outside snake case",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.ID = "Bad-ID"
			},
			wantCodes: []string{loop.CodeNodeIDInvalid},
		},
		{
			name: "Should reject unknown terminal states",
			mutate: func(def *dsl.Definition) {
				def.Contract.TerminalStates = append(def.Contract.TerminalStates, "terminated")
			},
			wantCodes: []string{loop.CodeUnknownTerminalState},
		},
		{
			name: "Should reject source input refs not declared as inputs",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "load")
				node.InputRef = "missing"
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},

		{
			name: "Should reject event range harvest on reserved action kinds",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Harvest = &dsl.HarvestSpec{Kind: "event_range"}
			},
			wantCodes: []string{loop.CodeInvalidHarvest},
		},

		{
			name: "Should reject unknown harvest kinds on open actions",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "compozy__task_read"
				node.Harvest = &dsl.HarvestSpec{Kind: "event_rang"}
			},
			wantCodes: []string{loop.CodeInvalidHarvest},
		},
		{
			name: "Should reject unresolved open action ToolIDs",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "compozy__missing_tool"
			},
			wantCodes: []string{loop.CodeUnknownActionKind},
		},
		{
			name: "Should reject unknown template references",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Params["prompt"] = "{{ .inputs.missing }}"
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject unresolvable template paths",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Params["prompt"] = "{{ .nodes.load.output.missing }}"
			},
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject unquoted runtime values in command checks",
			mutate: func(def *dsl.Definition) {
				def.Contract.Verification = []dsl.GateCriterion{{
					ID: "verify", Type: dsl.CriterionCommand,
					Check: "test -d artifacts/{{ .inputs.tasks }}",
				}}
			},
			wantCodes: []string{refs.CodeUnsafeCommandInterpolation},
		},
		{
			name: "Should reject item outside fanout",
			mutate: func(def *dsl.Definition) {
				agent := requireNode(t, def, "agent")
				agent.Params["prompt"] = "{{ .item.title }}"
				def.Graph.Nodes = []dsl.Node{*requireNode(t, def, "load"), *agent}
				def.Graph.Edges = []dsl.Edge{{From: "load", To: "agent"}}
			},
			wantCodes: []string{refs.CodeItemOutsideFanout},
		},
		{
			name: "Should reject non bool CEL conditions",
			mutate: func(def *dsl.Definition) {
				appendGate(def, dsl.Node{
					ID:        "decision",
					Class:     dsl.NodeClassControl,
					Kind:      string(dsl.ControlBranch),
					Condition: "1 + 1",
				})
			},
			wantCodes: []string{refs.CodeConditionNotBool},
		},
		{
			name: "Should accept trigger input mapping payload references for trigger starts",
			mutate: func(def *dsl.Definition) {
				def.Start = []dsl.StartBinding{{
					Kind: dsl.StartTrigger,
					InputMapping: map[string]string{
						"tasks": "{{ .trigger.payload.tasks }}",
					},
				}}
			},
		},
		{
			name: "Should reject trigger input mapping payload references for manual starts",
			mutate: func(def *dsl.Definition) {
				def.Start = []dsl.StartBinding{{
					Kind: dsl.StartManual,
					InputMapping: map[string]string{
						"tasks": "{{ .trigger.payload.tasks }}",
					},
				}}
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject start input mapping keys outside declared inputs",
			mutate: func(def *dsl.Definition) {
				def.Start = []dsl.StartBinding{{
					Kind: dsl.StartTrigger,
					InputMapping: map[string]string{
						"missing": "{{ .trigger.payload.tasks }}",
					},
				}}
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject contract verification template references that do not resolve",
			mutate: func(def *dsl.Definition) {
				def.Contract.Verification = []dsl.GateCriterion{{
					ID:     "dod",
					Type:   dsl.CriterionAgentJudge,
					Agent:  "critic",
					Rubric: "Judge {{ .inputs.missing }}",
				}}
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should restrict contract narrative templates to declared inputs",
			mutate: func(def *dsl.Definition) {
				def.Contract.Goal = "Ship {{ .nodes.agent.output.summary }}"
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject item references after collect barrier",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = append(def.Graph.Nodes,
					dsl.Node{
						ID:    "collect",
						Class: dsl.NodeClassControl,
						Kind:  string(dsl.ControlCollect),
					},
					dsl.Node{
						ID:    "after_collect",
						Class: dsl.NodeClassAction,
						Kind:  string(dsl.ActionRunAgent),
						Params: dsl.NodeParams{
							"agent":  "codex",
							"prompt": "Outside {{ .item.title }}",
							"output_schema": map[string]any{
								"summary": "string",
							},
						},
					},
				)
				def.Graph.Edges = append(def.Graph.Edges,
					dsl.Edge{From: "agent", To: "collect"},
					dsl.Edge{From: "collect", To: "after_collect"},
				)
			},
			wantCodes: []string{refs.CodeItemOutsideFanout},
		},
		{
			name: "Should accept CEL member functions over node outputs",
			mutate: func(def *dsl.Definition) {
				def.Contract.StopWhen = dsl.StopWhenSpec{Expr: `nodes.agent.output.summary.contains("done")`}
			},
		},
		{
			name: "Should accept CEL indexed node output fields",
			mutate: func(def *dsl.Definition) {
				agent := requireNode(t, def, "agent")
				agent.Params["output_schema"] = map[string]any{
					"issues": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{"type": "string"},
							},
						},
					},
				}
				def.Contract.StopWhen = dsl.StopWhenSpec{Expr: `nodes.agent.output.issues[0].id == "issue-1"`}
			},
		},
		{
			name: "Should reject CEL indexed node output fields that do not resolve",
			mutate: func(def *dsl.Definition) {
				agent := requireNode(t, def, "agent")
				agent.Params["output_schema"] = map[string]any{
					"issues": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{"type": "string"},
							},
						},
					},
				}
				def.Contract.StopWhen = dsl.StopWhenSpec{Expr: `nodes.agent.output.issues[0].missing == "issue-1"`}
			},
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject transform from paths that do not resolve",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
					ID:    "shape",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{
							"summary": map[string]any{
								"from": "nodes.agent.output.missing",
							},
						},
					},
				})
				def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "shape"})
			},
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should accept transform value literals that look like templates",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
					ID:    "shape",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{
							"literal": map[string]any{
								"value": "{{ .inputs.missing }} stays literal",
							},
						},
					},
				})
				def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "shape"})
			},
		},
		{
			name: "Should validate nested params named output schema outside declared schemas",
			mutate: func(def *dsl.Definition) {
				def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
					ID:    "shape",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{
							"output_schema": map[string]any{
								"template": "{{ .inputs.missing }}",
							},
						},
					},
				})
				def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "shape"})
			},
			wantCodes: []string{refs.CodeUnknownReference},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := validDefinition()
			if tt.mutate != nil {
				tt.mutate(&def)
			}

			linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{}))
			errs := linter.Lint(def)
			requireLintCodes(t, errs, tt.wantCodes...)
		})
	}
}

func TestLinterShouldAcceptLargeAuthorFanOutBounds(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a large positive max fan-out and parallel window", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		requireNode(t, &def, "fan").MaxFanOut = 500
		requireNode(t, &def, "fan").MaxParallel = 128
		appendGate(&def, dsl.Node{
			ID:            "review_gate",
			Class:         dsl.NodeClassControl,
			Kind:          string(dsl.ControlGate),
			MaxRevisions:  loop.LoopMaxGateRevisions,
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{
				{
					ID:     "judge",
					Type:   dsl.CriterionAgentJudge,
					Agent:  "critic",
					Rubric: "Approve only correct work",
				},
			},
		})

		linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{}))
		requireLintCodes(t, linter.Lint(def))
	})
}

func TestLinterShouldWarnForUnknownOutputSchemaReferences(t *testing.T) {
	t.Parallel()

	t.Run("Should warn when output schema references are unavailable", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		agent := requireNode(t, &def, "agent")
		delete(agent.Params, "output_schema")
		def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
			ID:    "after_agent",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent":  "codex",
				"prompt": "Use {{ .nodes.agent.output.summary }}",
				"output_schema": map[string]any{
					"done": "boolean",
				},
			},
		})
		def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "after_agent"})

		linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{}))
		requireLintWarning(t, linter.Lint(def), refs.CodeUnresolvablePath)
	})
}

func TestLinterShouldRejectClosedEnumAndReservedSchemaViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		def       dsl.Definition
		wantCodes []string
	}{
		{
			name: "Should reject unknown node classes",
			def: singleNodeDefinition(dsl.Node{
				ID:    "mystery",
				Class: "custom",
				Kind:  "anything",
			}),
			wantCodes: []string{refs.CodeUnknownReference},
		},
		{
			name: "Should reject unknown control kinds",
			def: singleNodeDefinition(dsl.Node{
				ID:    "control",
				Class: dsl.NodeClassControl,
				Kind:  "parallel",
			}),
			wantCodes: []string{loop.CodeUnknownControlKind},
		},
		{
			name: "Should reject synthetic budget gate node id",
			def: singleNodeDefinition(dsl.Node{
				ID:    loop.BudgetGateID,
				Class: dsl.NodeClassControl,
				Kind:  string(dsl.ControlGate),
			}),
			wantCodes: []string{loop.CodeNodeIDInvalid},
		},
		{
			name: "Should reject synthetic contract gate node id",
			def: singleNodeDefinition(dsl.Node{
				ID:    "contract",
				Class: dsl.NodeClassControl,
				Kind:  string(dsl.ControlGate),
			}),
			wantCodes: []string{loop.CodeNodeIDInvalid},
		},
		{
			name: "Should reject unknown source kinds",
			def: singleNodeDefinition(dsl.Node{
				ID:    "source",
				Class: dsl.NodeClassSource,
				Kind:  "mailbox",
			}),
			wantCodes: []string{loop.CodeUnknownSourceKind},
		},
		{
			name: "Should reject unresolved open action ToolIDs",
			def: singleNodeDefinition(dsl.Node{
				ID:    "bad_tool",
				Class: dsl.NodeClassAction,
				Kind:  "BadTool",
			}),
			wantCodes: []string{loop.CodeUnknownActionKind},
		},
		{
			name: "Should reject run agent missing required params",
			def: singleNodeDefinition(dsl.Node{
				ID:     "agent",
				Class:  dsl.NodeClassAction,
				Kind:   string(dsl.ActionRunAgent),
				Params: dsl.NodeParams{},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject run loop missing loop and invalid mode",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"mode": "later",
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject unknown child loop config override fields",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"iteration_caps": 4,
					},
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject operator-owned child loop lifecycle policy",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"lifecycle": map[string]any{},
					},
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject operator-owned child loop request expiry",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"request_expire_after": "30m",
					},
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject wrong child loop config override types",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"iteration_cap": "four",
					},
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject unknown referenced child loop config override fields",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"iteration_caps": "{{ .inputs.items }}",
					},
				},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should accept literal JSON child loop config overrides",
			def: singleNodeDefinition(dsl.Node{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "implement-tasks",
					"config_overrides": map[string]any{
						"enabled_checks_json": map[string]any{
							"quality": map[string]any{"enabled": true},
						},
					},
				},
			}),
		},
		{
			name: "Should accept typed references in child loop config overrides",
			def:  runLoopTemplateConfigDefinition(),
		},
		{
			name: "Should reject transform without map",
			def: singleNodeDefinition(dsl.Node{
				ID:     "transform",
				Class:  dsl.NodeClassAction,
				Kind:   string(dsl.ActionTransform),
				Params: dsl.NodeParams{},
			}),
			wantCodes: []string{refs.CodeUnresolvablePath},
		},
		{
			name: "Should reject environment on transform nodes",
			def: singleNodeDefinition(dsl.Node{
				ID:    "transform_environment",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionTransform),
				Params: dsl.NodeParams{
					"map":         map[string]any{"ok": map[string]any{"value": true}},
					"environment": map[string]any{"mode": "root"},
				},
			}),
			wantCodes: []string{loop.CodeEnvironmentUnsupported},
		},
		{
			name: "Should reject environment on control nodes",
			def: singleNodeDefinition(dsl.Node{
				ID:          "fan_environment",
				Class:       dsl.NodeClassControl,
				Kind:        string(dsl.ControlFanOut),
				Collection:  "{{ .inputs.items }}",
				MaxFanOut:   1,
				MaxParallel: 1,
				Params: dsl.NodeParams{
					"environment": map[string]any{"mode": "root"},
				},
			}),
			wantCodes: []string{loop.CodeEnvironmentUnsupported},
		},
		{
			name: "Should reject environment on source nodes",
			def: singleNodeDefinition(dsl.Node{
				ID:      "source_environment",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
				Parse:   dsl.FileParseText,
				Params: dsl.NodeParams{
					"environment": map[string]any{"mode": "root"},
				},
			}),
			wantCodes: []string{loop.CodeEnvironmentUnsupported},
		},
		{
			name: "Should reject empty sub loops",
			def: singleNodeDefinition(dsl.Node{
				ID:    "sub",
				Class: dsl.NodeClassControl,
				Kind:  string(dsl.ControlSubLoop),
				Body:  &dsl.Graph{},
			}),
			wantCodes: []string{loop.CodeNonTerminatingStructure},
		},
		{
			name: "Should reject invalid sub loop body structures",
			def: singleNodeDefinition(dsl.Node{
				ID:    "sub",
				Class: dsl.NodeClassControl,
				Kind:  string(dsl.ControlSubLoop),
				Body: &dsl.Graph{
					Nodes: []dsl.Node{
						{
							ID:       "nested_load",
							Class:    dsl.NodeClassSource,
							Kind:     string(dsl.SourceInput),
							InputRef: "items",
						},
						{
							ID:    "nested_agent",
							Class: dsl.NodeClassAction,
							Kind:  string(dsl.ActionRunAgent),
							Params: dsl.NodeParams{
								"agent":  "codex",
								"prompt": "Summarize {{ .nodes.nested_load.output }}",
								"output_schema": map[string]any{
									"summary": "string",
								},
							},
						},
					},
					Edges: []dsl.Edge{
						{From: "nested_load", To: "nested_agent"},
						{From: "nested_agent", To: "nested_load"},
					},
				},
			}),
			wantCodes: []string{loop.CodeCycle},
		},
		{
			name: "Should reject invalid file import source schema",
			def: singleNodeDefinition(dsl.Node{
				ID:      "files",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
				Parse:   "yaml",
			}),
			wantCodes: []string{loop.CodeFileImportParseRequired},
		},
		{
			name: "Should accept watch source closed enum",
			def: singleNodeDefinition(dsl.Node{
				ID:        "watch",
				Class:     dsl.NodeClassSource,
				Kind:      string(dsl.SourceWatchSource),
				WatchSpec: map[string]any{"kind": "reviews"},
			}),
		},
		{
			name: "Should accept watch-events closed enum",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "task.status_changed",
			}})),
		},
		{
			name: "Should reject watch source without kind",
			def: singleNodeDefinition(dsl.Node{
				ID:    "watch",
				Class: dsl.NodeClassSource,
				Kind:  string(dsl.SourceWatchSource),
			}),
			wantCodes: []string{loop.CodeWatchKindRequired},
		},
		{
			name: "Should accept JSON file import source parse",
			def: singleNodeDefinition(dsl.Node{
				ID:      "files",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
				Parse:   dsl.FileParseJSON,
			}),
		},
		{
			name: "Should accept text file import source parse",
			def: singleNodeDefinition(dsl.Node{
				ID:      "files",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
				Parse:   dsl.FileParseText,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{}))
			requireLintCodes(t, linter.Lint(tt.def), tt.wantCodes...)
		})
	}
}

func TestLinterShouldValidateWatchEventsSourceNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		def           dsl.Definition
		wantCodes     []string
		messageCode   string
		messageSubstr string
	}{
		{
			name: "Should accept supported watch-events subscriptions",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{
				{
					Kind:   "task.status_changed",
					Filter: `event.task_id == inputs.items && event.payload.to_status in ["blocked", "completed"]`,
				},
				{
					Kind:   "task.run.completed",
					Filter: `event.task_id == inputs.items`,
				},
			})),
		},
		{
			name: "Should accept event post-record only with a session constraint",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind:   "event.post_record",
				Filter: `event.session_id == "sess-watch" && event.payload.record_type == "agent_message"`,
			}})),
		},
		{
			name: "Should reject event post-record without a session constraint",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind:   "event.post_record",
				Filter: `event.payload.record_type == "agent_message"`,
			}})),
			wantCodes:     []string{loop.CodeWatchEventsFilterTooBroad},
			messageCode:   loop.CodeWatchEventsFilterTooBroad,
			messageSubstr: "session_id",
		},
		{
			name: "Should reject event post-record with a non-conjunctive session constraint",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind:   "event.post_record",
				Filter: `event.session_id == "sess-watch" || event.payload.record_type == "agent_message"`,
			}})),
			wantCodes: []string{loop.CodeWatchEventsFilterTooBroad},
		},
		{
			name:      "Should require at least one watch-events subscription",
			def:       singleNodeDefinition(watchEventsNodeForTest(nil)),
			wantCodes: []string{loop.CodeWatchEventsSubscriptionRequired},
		},
		{
			name: "Should reject subscription kinds outside the hook catalog",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "not.a.hook",
			}})),
			wantCodes: []string{loop.CodeWatchEventsKindUnknown},
		},
		{
			name: "Should reject pre-fire catalog kinds outside the supported registry",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "automation.job.pre_fire",
			}})),
			wantCodes:     []string{loop.CodeWatchEventsKindUnsupported},
			messageCode:   loop.CodeWatchEventsKindUnsupported,
			messageSubstr: "task.status_changed",
		},
		{
			name: "Should reject automation post-fire kinds without durable anchors",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "automation.job.post_fire",
			}})),
			wantCodes:     []string{loop.CodeWatchEventsKindUnsupported},
			messageCode:   loop.CodeWatchEventsKindUnsupported,
			messageSubstr: "automation.run.completed",
		},

		{
			name: "Should reject coordinator pre-spawn because it is pre-state",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "coordinator.pre_spawn",
			}})),
			wantCodes: []string{loop.CodeWatchEventsKindUnsupported},
		},
		{
			name: "Should reject event pre-record because it is pre-state",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "event.pre_record",
			}})),
			wantCodes: []string{loop.CodeWatchEventsKindUnsupported},
		},
		{
			name: "Should reject invalid watch-events CEL filters",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind:   "task.status_changed",
				Filter: "(((",
			}})),
			wantCodes: []string{loop.CodeWatchEventsFilterInvalid},
		},
		{
			name: "Should reject watch blocks on watch-events nodes",
			def: func() dsl.Definition {
				node := watchEventsNodeForTest([]dsl.EventSubscription{{Kind: "task.status_changed"}})
				node.WatchSpec = map[string]any{"kind": "legacy"}
				return singleNodeDefinition(node)
			}(),
			wantCodes: []string{loop.CodeWatchEventsShapeInvalid},
		},
		{
			name: "Should reject events blocks on other source kinds",
			def: singleNodeDefinition(dsl.Node{
				ID:       "items",
				Class:    dsl.NodeClassSource,
				Kind:     string(dsl.SourceInput),
				InputRef: "items",
				Events:   []dsl.EventSubscription{{Kind: "task.status_changed"}},
			}),
			wantCodes: []string{loop.CodeWatchEventsShapeInvalid},
		},
		{
			name: "Should reject produces shapes that contradict the fixed envelope",
			def: func() dsl.Definition {
				node := watchEventsNodeForTest([]dsl.EventSubscription{{Kind: "task.status_changed"}})
				node.Produces = dsl.Schema{
					"events": map[string]any{"type": "string"},
				}
				return singleNodeDefinition(node)
			}(),
			wantCodes: []string{loop.CodeWatchEventsShapeInvalid},
		},
		{
			name: "Should reject cursor produces shapes that contradict the fixed envelope",
			def: func() dsl.Definition {
				node := watchEventsNodeForTest([]dsl.EventSubscription{{Kind: "task.status_changed"}})
				node.Produces = dsl.Schema{
					"events":  map[string]any{"type": "array"},
					"cursors": "string",
				}
				return singleNodeDefinition(node)
			}(),
			wantCodes: []string{loop.CodeWatchEventsShapeInvalid},
		},
		{
			name: "Should accept compatible watch-events produces shapes",
			def: func() dsl.Definition {
				node := watchEventsNodeForTest([]dsl.EventSubscription{{Kind: "task.status_changed"}})
				node.Produces = dsl.Schema{
					"events":  []any{map[string]any{"kind": "string"}},
					"cursors": "map",
				}
				return singleNodeDefinition(node)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{})).Lint(tt.def)
			requireLintCodes(t, errs, tt.wantCodes...)
			if tt.messageCode != "" {
				requireLintMessageContains(t, errs, tt.messageCode, tt.messageSubstr)
			}
		})
	}
}

func TestLinterShouldRejectFileImportParseOutsideJSONAndText(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		parse dsl.FileParseKind
	}{
		{name: "Should reject empty parse", parse: ""},
		{name: "Should reject legacy task parse", parse: "md" + "_tasks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			def := singleNodeDefinition(dsl.Node{
				ID:      "files",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
				Parse:   tc.parse,
			})
			errs := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{})).Lint(def)
			requireLintCodes(t, errs, loop.CodeFileImportParseRequired)
			requireLintMessageContains(t, errs, loop.CodeFileImportParseRequired, "json|text")
		})
	}
}

func TestLinterShouldValidateGoalContractsWithoutMutation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition func() dsl.Definition
		mutate     func(*dsl.Definition)
		wantCodes  []string
	}{
		{name: "Should accept a minimal goal with compiler-owned defaults", definition: validGoalDefinition},
		{
			name:       "Should reject a missing objective",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["objective"] = " "
			},
			wantCodes: []string{loop.CodeGoalObjectiveRequired},
		},
		{
			name:       "Should reject a missing judge",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["judge"] = []any{}
			},
			wantCodes: []string{loop.CodeGoalJudgeRequired},
		},
		{
			name:       "Should reject a human goal judge",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["judge"] = []any{
					map[string]any{"id": "operator", "type": "human"},
				}
			},
			wantCodes: []string{loop.CodeGoalHumanJudgeUnsupported},
		},
		{
			name:       "Should reject a command judge without check",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["judge"] = []any{
					map[string]any{"id": "verify", "type": "command", "command": "make verify"},
				}
			},
			wantCodes: []string{loop.CodeGoalJudgeRequired},
		},
		{
			name:       "Should reject a missing positive max turns",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["max_turns"] = 0
			},
			wantCodes: []string{loop.CodeGoalMaxTurnsRequired},
		},
		{
			name:       "Should reject output status without blocked",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["output_schema"] = map[string]any{
					"status": map[string]any{"type": "string", "enum": []any{"complete"}},
				}
			},
			wantCodes: []string{loop.CodeGoalOutputStatusMissingBlocked},
		},
		{
			name:       "Should reject output status without complete",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["output_schema"] = map[string]any{
					"status": map[string]any{"type": "string", "enum": []any{"completed", "blocked"}},
				}
			},
			wantCodes: []string{loop.CodeGoalOutputStatusMissingComplete},
		},
		{
			name:       "Should reject an invalid exhaustion policy",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["on_exhausted"] = "blocked"
			},
			wantCodes: []string{loop.CodeGoalOnExhaustedInvalid},
		},
		{
			name:       "Should reject ambiguous session forms",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Session = &dsl.SessionSpec{Isolated: true, Mode: "continuous"}
			},
			wantCodes: []string{loop.CodeSessionSpecAmbiguous},
		},
		{
			name:       "Should reject continuous goal parallel fan out",
			definition: validFanoutGoalDefinition,
			wantCodes:  []string{loop.CodeContinuousForbidsParallel},
		},
		{
			name:       "Should reject fresh session retry outside continuous mode",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "converge")
				node.Session = &dsl.SessionSpec{Isolated: true}
				node.Retry = &dsl.RetrySpec{MaxAttempts: 2, OnFailure: "fresh_session"}
			},
			wantCodes: []string{loop.CodeRetryFreshSessionRequiresContinuous},
		},
		{
			name:       "Should reject fresh session retry with one total attempt",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "converge")
				node.Session = &dsl.SessionSpec{Mode: "continuous"}
				node.Retry = &dsl.RetrySpec{MaxAttempts: 1, OnFailure: "fresh_session"}
			},
			wantCodes: []string{loop.CodeRetryFreshSessionRequiresContinuous},
		},
		{
			name:       "Should reject retired retry max",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "converge")
				node.Retry = &dsl.RetrySpec{
					MaxAttempts: 2,
					OnFailure:   "fresh_session",
					Extra:       map[string]any{"max": 2},
				}
			},
			wantCodes: []string{loop.CodeRetryMaxUnsupported},
		},
		{
			name:       "Should reject retired cwd with the environment migration",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["cwd"] = "packages/api"
			},
			wantCodes: []string{loop.CodeEnvironmentCWDRemoved},
		},
		{
			name:       "Should reject an incomplete worktree environment",
			definition: validGoalDefinition,
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "converge").Params["environment"] = map[string]any{
					"mode": "worktree",
				}
			},
			wantCodes: []string{loop.CodeEnvironmentInvalid},
		},
		{
			name:       "Should reject explicit continuous handle reuse",
			definition: explicitReusedGoalHandleDefinition,
			wantCodes:  []string{loop.CodeContinuousHandleReused},
		},
		{
			name:       "Should reject implicit and explicit continuous handle reuse",
			definition: implicitReusedGoalHandleDefinition,
			wantCodes:  []string{loop.CodeContinuousHandleReused},
		},
		{
			name: "Should accept supported judge kinds and explicit isolated session",
			definition: func() dsl.Definition {
				def := validGoalDefinition()
				node := requireNode(t, &def, "converge")
				node.Session = &dsl.SessionSpec{Isolated: true}
				node.Params["judge"] = []any{
					map[string]any{"id": "rubric", "type": "agent-judge", "rubric": "Be strict"},
					map[string]any{"id": "policy", "type": "extension", "tool": "ext__policy__check"},
				}
				node.Params["on_exhausted"] = "escalate"
				return def
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := tt.definition()
			if tt.mutate != nil {
				tt.mutate(&def)
			}
			before, err := dsl.Serialize(def)
			if err != nil {
				t.Fatalf("Serialize(before) error = %v", err)
			}
			errs := loop.NewLinter().Lint(def)
			requireLintCodes(t, errs, tt.wantCodes...)
			after, err := dsl.Serialize(def)
			if err != nil {
				t.Fatalf("Serialize(after) error = %v", err)
			}
			if !bytes.Equal(after, before) {
				t.Fatalf("Lint() mutated definition\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestLinterShouldPreserveReferenceDiagnosticDetails(t *testing.T) {
	t.Parallel()
	t.Run("Should report the deepest valid path and available fields", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		agent := requireNode(t, &def, "agent")
		agent.Params["prompt"] = "Summarize {{ .nodes.load.output.items.summary }}"
		diagnostics := loop.NewLinter().Lint(def)
		requireLintDiagnostic(t, diagnostics, refs.CodeUnresolvablePath, loop.SeverityError)
		requireLintMessageContains(t, diagnostics, refs.CodeUnresolvablePath, "available fields: title")
		requireLintMessageContains(t, diagnostics, refs.CodeUnresolvablePath, "nodes.load.output.items")
	})
}

func TestLinterShouldValidateLifecycleGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*dsl.Definition)
		linter   *loop.DefinitionLinter
		wantCode string
		severity loop.LintSeverity
	}{
		{
			name: "Should reject a non-forward error route",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "agent").OnError = &dsl.ErrorPolicy{Route: "load"}
			},
			wantCode: loop.CodeErrorRouteBackward,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject conflicting error dispositions",
			mutate: func(def *dsl.Definition) {
				agent := requireNode(t, def, "agent")
				agent.OnError = &dsl.ErrorPolicy{Route: "fallback", AllowFail: true}
				appendLifecycleFallback(def)
			},
			wantCode: loop.CodeErrorRouteConflict,
			severity: loop.SeverityError,
		},
		{
			name: "Should warn about a dead error route",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "load").OnError = &dsl.ErrorPolicy{Route: "fan"}
			},
			wantCode: loop.CodeErrorRouteDead,
			severity: loop.SeverityWarning,
		},
		{
			name: "Should reject generic lifecycle fields on a Goal",
			mutate: func(def *dsl.Definition) {
				*def = validGoalDefinition()
				goal := requireNode(t, def, "converge")
				goal.NodeLifecycleState = &dsl.NodeLifecycleState{Deadline: "1m"}
			},
			wantCode: loop.CodeRetryOnGoalNode,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject timeout beyond deadline",
			mutate: func(def *dsl.Definition) {
				agent := requireNode(t, def, "agent")
				agent.Timeout = "2m"
				agent.Deadline = "1m"
			},
			wantCode: loop.CodeTimeoutExceedsDeadline,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject an invalid duration",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "agent").Timeout = "soon"
			},
			wantCode: loop.CodeDurationInvalid,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject a result contract outside the output schema",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "agent").ResultContract = &dsl.ResultContract{FailureField: "error"}
			},
			wantCode: loop.CodeResultContractInvalid,
			severity: loop.SeverityError,
		},
		{
			name: "Should warn about an unknown effect tool",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "agent").OnSuccess = []dsl.EffectSpec{{Tool: "missing__tool"}}
			},
			linter:   loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{})),
			wantCode: loop.CodeEffectToolUnknown,
			severity: loop.SeverityWarning,
		},
		{
			name: "Should reject a wait without one discriminator",
			mutate: func(def *dsl.Definition) {
				appendLifecycleWait(def, dsl.NodeParams{})
			},
			wantCode: loop.CodeWaitShapeInvalid,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject a wait event outside the hook catalog before execution",
			mutate: func(def *dsl.Definition) {
				appendLifecycleWait(def, dsl.NodeParams{
					"event": map[string]any{"kind": "qa.acknowledged"},
				})
			},
			wantCode: loop.CodeWatchEventsKindUnknown,
			severity: loop.SeverityError,
		},
		{
			name: "Should warn when wait expiry has no path",
			mutate: func(def *dsl.Definition) {
				appendLifecycleWait(def, dsl.NodeParams{
					"for":     "1m",
					"expires": map[string]any{"after": "2m"},
				})
			},
			wantCode: loop.CodeWaitExpiryWithoutPath,
			severity: loop.SeverityWarning,
		},
		{
			name: "Should reject gate expiry without an after duration",
			mutate: func(def *dsl.Definition) {
				gate := dsl.Node{
					ID: "approval", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
					Criteria: []dsl.GateCriterion{{
						ID: "check", Type: dsl.CriterionCommand, Check: "true", Expect: "exit_zero",
					}},
					VerdictPolicy: dsl.VerdictPolicyFixedPasses,
					Expires:       &dsl.WaitExpiry{Route: "load"},
				}
				def.Graph.Nodes = append(def.Graph.Nodes, gate)
				def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "approval", To: "load"})
			},
			wantCode: loop.CodeWaitShapeInvalid,
			severity: loop.SeverityError,
		},
		{
			name: "Should reject a watch source without stable identity",
			mutate: func(def *dsl.Definition) {
				*def = singleNodeDefinition(dsl.Node{
					ID:        "watch",
					Class:     dsl.NodeClassSource,
					Kind:      string(dsl.SourceWatchSource),
					WatchSpec: map[string]any{"kind": "unstable"},
				})
			},
			linter:   loop.NewLinter(loop.WithToolSchemaSource(identityToolSchemas{})),
			wantCode: loop.CodeWatchIdentityRequired,
			severity: loop.SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := validDefinition()
			def.Normalize()
			tt.mutate(&def)
			linter := tt.linter
			if linter == nil {
				linter = loop.NewLinter()
			}
			requireLintDiagnostic(t, linter.Lint(def), tt.wantCode, tt.severity)
		})
	}

	t.Run("Should accept zero annotations and a fully annotated definition without mutation", func(t *testing.T) {
		t.Parallel()

		zero := validDefinition()
		requireLintCodes(t, loop.NewLinter().Lint(zero))

		annotated := validDefinition()
		annotated.Normalize()
		agent := requireNode(t, &annotated, "agent")
		agent.Timeout = "30s"
		agent.Deadline = "2m"
		agent.Retry = &dsl.RetrySpec{
			MaxAttempts:  2,
			Backoff:      &dsl.BackoffSpec{Base: "1s", Max: "10s"},
			NonRetryable: []string{string(loop.FailurePayloadDeclared)},
		}
		agent.ResultContract = &dsl.ResultContract{FailureField: "summary"}
		agent.OnError = &dsl.ErrorPolicy{
			AllowFail: true,
			Effects:   []dsl.EffectSpec{{Tool: "known__effect", With: map[string]any{"body": "failed"}}},
		}
		agent.OnRetry = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "agent_retrying"}}}
		agent.OnSuccess = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "agent_succeeded"}}}
		annotated.Contract.OnDone = []dsl.EffectSpec{{Tool: "known__effect"}}
		before, err := dsl.Serialize(annotated)
		if err != nil {
			t.Fatalf("Serialize(before) error = %v", err)
		}
		linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{
			"known__effect": {ToolID: "known__effect"},
		}))
		requireLintCodes(t, linter.Lint(annotated))
		after, err := dsl.Serialize(annotated)
		if err != nil {
			t.Fatalf("Serialize(after) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("normalize/lint mutated lifecycle grammar\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should reject both effect kinds and neither effect kind", func(t *testing.T) {
		t.Parallel()

		for name, effect := range map[string]dsl.EffectSpec{
			"both":    {Emit: &dsl.EmitSpec{Kind: "event"}, Tool: "known__effect"},
			"neither": {},
		} {
			t.Run("Should reject "+name, func(t *testing.T) {
				t.Parallel()
				def := validDefinition()
				def.Normalize()
				requireNode(t, &def, "agent").OnSuccess = []dsl.EffectSpec{effect}
				requireLintDiagnostic(t, loop.NewLinter().Lint(def), loop.CodeEffectShapeInvalid, loop.SeverityError)
			})
		}
	})

	t.Run("Should enforce wait XOR and expiry after", func(t *testing.T) {
		t.Parallel()

		for name, params := range map[string]dsl.NodeParams{
			"multiple discriminators": {"for": "1m", "until": "{{ .inputs.deadline }}"},
			"expiry without after":    {"for": "1m", "expires": map[string]any{"route": "after_wait"}},
		} {
			t.Run("Should reject "+name, func(t *testing.T) {
				t.Parallel()
				def := validDefinition()
				appendLifecycleWait(&def, params)
				requireLintDiagnostic(t, loop.NewLinter().Lint(def), loop.CodeWaitShapeInvalid, loop.SeverityError)
			})
		}
	})

	t.Run("Should allow parent close only on run-loop with a closed value", func(t *testing.T) {
		t.Parallel()

		valid := singleNodeDefinition(dsl.Node{
			ID:                 "child",
			Class:              dsl.NodeClassAction,
			Kind:               string(dsl.ActionRunLoop),
			Params:             dsl.NodeParams{"loop": "child-loop", "mode": string(dsl.RunLoopAwait)},
			NodeLifecycleState: &dsl.NodeLifecycleState{OnParentClose: dsl.ParentCloseTerminate},
		})
		requireLintCodes(t, loop.NewLinter().Lint(valid))

		invalidKind := validDefinition()
		invalidKind.Normalize()
		requireNode(t, &invalidKind, "agent").OnParentClose = dsl.ParentCloseTerminate
		requireLintDiagnostic(t, loop.NewLinter().Lint(invalidKind), loop.CodeParentCloseInvalid, loop.SeverityError)

		invalidValue := valid
		invalidValue.Normalize()
		invalidValue.Graph.Nodes[0].OnParentClose = "detach"
		requireLintDiagnostic(t, loop.NewLinter().Lint(invalidValue), loop.CodeParentCloseInvalid, loop.SeverityError)
	})
}

func appendLifecycleFallback(def *dsl.Definition) {
	def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
		ID:    "fallback",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{"map": map[string]any{
			"summary": "{{ .nodes.agent.output.summary }}",
		}},
	})
	def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "fallback"})
}

func appendLifecycleWait(def *dsl.Definition, params dsl.NodeParams) {
	def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
		ID:     "wait",
		Class:  dsl.NodeClassControl,
		Kind:   string(dsl.ControlWait),
		Params: params,
	})
	def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: "wait"})
}

func requireLintDiagnostic(t *testing.T, diagnostics []loop.LintError, code string, severity loop.LintSeverity) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Severity == severity && diagnostic.Message != "" {
			return
		}
	}
	t.Fatalf("Lint() diagnostics = %#v, want %s %q", diagnostics, severity, code)
}

func requireLintPathDiagnostic(t *testing.T, diagnostics []loop.LintError, path string, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Path == path && diagnostic.Code == code && diagnostic.Severity == loop.SeverityError {
			return
		}
	}
	t.Fatalf("Lint() diagnostics = %#v, want error %q at %q", diagnostics, code, path)
}

type identityToolSchemas struct{}

func (identityToolSchemas) Snapshot(string) (loop.ToolSchemaSnapshot, bool) {
	return loop.ToolSchemaSnapshot{}, false
}

func (identityToolSchemas) SupportsStableWatchIdentity(string) bool {
	return false
}

func validDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion:  dsl.APIVersion,
		Kind:        dsl.KindLoop,
		Concurrency: dsl.ConcurrencyForbid,
		Meta: dsl.Meta{
			Name:    "valid-loop",
			Version: 1,
		},
		Inputs: map[string]dsl.Input{
			"tasks": {
				Type: dsl.InputTypeRef, Required: true,
				Ref: &dsl.InputRef{Kind: dsl.InputRefKindSkill},
			},
		},
		Contract: dsl.Contract{
			Goal:             "Complete task list",
			DefinitionOfDone: "All tasks have summaries",
			Verification:     []dsl.GateCriterion{},
			ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalStates: []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
			},
			IterationCap: 3,
			NoProgress:   dsl.NoProgress{Window: 2},
			Budget: dsl.Budget{
				Tokens:       1000,
				WallClockSec: 60,
				OnExceeded:   dsl.BudgetExceededHalt,
			},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:       "load",
					Class:    dsl.NodeClassSource,
					Kind:     string(dsl.SourceInput),
					InputRef: "tasks",
					Produces: dsl.Schema{
						"items": []any{map[string]any{"title": "string"}},
					},
				},
				{
					ID:          "fan",
					Class:       dsl.NodeClassControl,
					Kind:        string(dsl.ControlFanOut),
					Collection:  "{{ .nodes.load.output.items }}",
					MaxFanOut:   4,
					MaxParallel: 2,
				},
				{
					ID:    "agent",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{
						"agent":  "codex",
						"prompt": "Summarize {{ .item.title }}",
						"output_schema": map[string]any{
							"summary": "string",
						},
					},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "fan"},
				{From: "fan", To: "agent"},
			},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartManual}},
		},
	}
}

func routeDefinitionForTest() dsl.Definition {
	definition := singleNodeDefinition(dsl.Node{
		ID:      "router",
		Class:   dsl.NodeClassControl,
		Kind:    string(dsl.ControlRoute),
		Routes:  []dsl.RouteSpec{{When: "true", To: "quick"}, {When: "false", To: "review"}},
		Default: "fallback",
	})
	for _, id := range []dsl.NodeID{"quick", "review", "fallback"} {
		definition.Graph.Nodes = append(definition.Graph.Nodes, dsl.Node{
			ID: id, Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{"agent": "codex", "prompt": "Run " + string(id)},
		})
		definition.Graph.Edges = append(definition.Graph.Edges, dsl.Edge{From: "router", To: id})
	}
	return definition
}

func validGoalDefinition() dsl.Definition {
	return singleNodeDefinition(validGoalNode("converge", ""))
}

func validGoalNode(id dsl.NodeID, handle string) dsl.Node {
	node := dsl.Node{
		ID:    id,
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionGoal),
		Params: dsl.NodeParams{
			"agent":     "codex",
			"objective": "Make verification pass",
			"judge": []any{
				map[string]any{"id": "verify", "type": "command", "check": "make verify"},
			},
			"max_turns": 10,
			"output_schema": map[string]any{
				"status": map[string]any{
					"type": "string",
					"enum": []any{"complete", "blocked"},
				},
			},
		},
	}
	if handle != "" {
		node.Session = &dsl.SessionSpec{Mode: "continuous", Handle: handle}
	}
	return node
}

func validFanoutGoalDefinition() dsl.Definition {
	def := validDefinition()
	goal := validGoalNode("agent", "")
	def.Graph.Nodes[2] = goal
	return def
}

func explicitReusedGoalHandleDefinition() dsl.Definition {
	def := validGoalDefinition()
	first := &def.Graph.Nodes[0]
	first.Session = &dsl.SessionSpec{Mode: "continuous", Handle: "shared"}
	second := validGoalNode("finish", "shared")
	def.Graph.Nodes = append(def.Graph.Nodes, second)
	def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: first.ID, To: second.ID})
	return def
}

func implicitReusedGoalHandleDefinition() dsl.Definition {
	def := singleNodeDefinition(validGoalNode("shared", ""))
	second := validGoalNode("finish", "shared")
	def.Graph.Nodes = append(def.Graph.Nodes, second)
	def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "shared", To: "finish"})
	return def
}

// Invariant: an awaited child result declared by produces is exactly what downstream templates can read.
// The Loop linter owns the authoring rejection before an expensive child run starts.
func TestLinterShouldValidateRunLoopTerminalOutputShape(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name      string
		mode      dsl.RunLoopMode
		inputs    map[string]any
		produces  dsl.Schema
		wantCodes []string
	}{
		{name: "Should accept a declared awaited child result", produces: dsl.Schema{"loop_run_id": "string", "status": "string"}},
		{name: "Should accept a literal immutable worktree candidate", inputs: map[string]any{dsl.ReviewedWorktreeInput: `{"worktree_id":"wt-reviewed","include_paths":["src/change.go"],"fingerprint":"` + strings.Repeat("a", 64) + `"}`}},
		{name: "Should reject a mutable worktree candidate without a fingerprint", inputs: map[string]any{dsl.ReviewedWorktreeInput: `{"worktree_id":"wt-reviewed","include_paths":["src/change.go"]}`}, wantCodes: []string{refs.CodeUnresolvablePath}},
		{name: "Should accept explicit string types", produces: dsl.Schema{"loop_run_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}},
		{name: "Should reject a constrained awaited status", produces: dsl.Schema{"loop_run_id": "string", "status": map[string]any{"type": "string", "enum": []any{"done"}}}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should reject a constrained awaited run ID", produces: dsl.Schema{"loop_run_id": map[string]any{"type": "string", "pattern": "^fixed$"}, "status": "string"}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should reject an unsupported awaited child field", produces: dsl.Schema{"loop_run_id": "string", "outputs": "object"}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should reject a non-string child status", produces: dsl.Schema{"status": "number"}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should reject an incomplete awaited child result", produces: dsl.Schema{"loop_run_id": "string"}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should accept a declared detached child result", mode: dsl.RunLoopDetach, produces: dsl.Schema{"loop_run_id": "string"}},
		{name: "Should reject a constrained detached run ID", mode: dsl.RunLoopDetach, produces: dsl.Schema{"loop_run_id": map[string]any{"type": "string", "minLength": 1000}}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
		{name: "Should reject status in a detached child result", mode: dsl.RunLoopDetach, produces: dsl.Schema{"status": "string"}, wantCodes: []string{loop.CodeRunLoopOutputShapeInvalid}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			def := singleNodeDefinition(dsl.Node{
				ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
				Params:   dsl.NodeParams{"loop": "nested", "mode": string(testCase.mode), "inputs": testCase.inputs},
				Produces: testCase.produces,
			})
			requireLintCodes(t, loop.NewLinter().Lint(def), testCase.wantCodes...)
		})
	}
	t.Run("Should reject object fields from an undeclared awaited child result", func(t *testing.T) {
		t.Parallel()
		def := singleNodeDefinition(dsl.Node{
			ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			Params: dsl.NodeParams{"loop": "nested", "mode": string(dsl.RunLoopAwait)},
		})
		def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
			ID: "inspect", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{"agent": "qa", "prompt": "{{ .nodes.child.output.status }}"},
		})
		def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "child", To: "inspect"})
		requireLintCodes(t, loop.NewLinter().Lint(def), refs.CodeUnresolvablePath)
	})
}

func singleNodeDefinition(node dsl.Node) dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "single-node"},
		Inputs: map[string]dsl.Input{
			"items": {Type: dsl.InputTypeRef, Ref: &dsl.InputRef{Kind: dsl.InputRefKindSkill}},
		},
		Contract: dsl.Contract{
			Goal:             "Validate",
			DefinitionOfDone: "No lint errors",
			ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalStates: []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
			},
			IterationCap: 1,
			NoProgress:   dsl.NoProgress{Window: 1},
			Budget: dsl.Budget{
				Tokens:       100,
				WallClockSec: 10,
				OnExceeded:   dsl.BudgetExceededHalt,
			},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{node},
			Edges: []dsl.Edge{},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartManual}},
		},
	}
}

func runLoopTemplateConfigDefinition() dsl.Definition {
	definition := singleNodeDefinition(dsl.Node{
		ID:    "routing",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Produces: dsl.Schema{
			"remaining_tokens": "number",
			"runtime_rules": []any{map[string]any{
				"match": map[string]any{"id": "string"},
				"runtime": map[string]any{
					"provider": "string",
					"model":    "string",
				},
			}},
		},
		Params: dsl.NodeParams{
			"map": map[string]any{
				"remaining_tokens": map[string]any{"value": 250000},
				"runtime_rules":    map[string]any{"value": []any{}},
			},
		},
	})
	definition.Graph.Nodes = append(definition.Graph.Nodes, dsl.Node{
		ID:    "child",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionRunLoop),
		Params: dsl.NodeParams{
			"loop": "implement-tasks",
			"config_overrides": map[string]any{
				"budget_tokens": "{{ .nodes.routing.output.remaining_tokens }}",
				"runtime_rules": "{{ .nodes.routing.output.runtime_rules }}",
			},
		},
	})
	definition.Graph.Edges = append(definition.Graph.Edges, dsl.Edge{From: "routing", To: "child"})
	return definition
}

func watchEventsNodeForTest(events []dsl.EventSubscription) dsl.Node {
	return dsl.Node{
		ID:     "task_activity",
		Class:  dsl.NodeClassSource,
		Kind:   string(dsl.SourceWatchEvents),
		Events: events,
	}
}

func appendGate(def *dsl.Definition, node dsl.Node) {
	def.Graph.Nodes = append(def.Graph.Nodes, node)
	def.Graph.Edges = append(def.Graph.Edges, dsl.Edge{From: "agent", To: node.ID})
}

func appendMetricGate(def *dsl.Definition, criterionType dsl.CriterionType, metric *dsl.MetricSpec) {
	criterion := dsl.GateCriterion{ID: "metric", Type: criterionType, Metric: metric}
	switch criterionType {
	case dsl.CriterionCommand:
		criterion.Check = "make verify"
	case dsl.CriterionAgentJudge:
		criterion.Rubric = "Check the result"
	case dsl.CriterionExtension:
		criterion.Tool = "ext__quality__score"
	}
	appendGate(def, dsl.Node{
		ID:       "metric_gate",
		Class:    dsl.NodeClassControl,
		Kind:     string(dsl.ControlGate),
		Criteria: []dsl.GateCriterion{criterion},
	})
}

func requireNode(t *testing.T, def *dsl.Definition, id dsl.NodeID) *dsl.Node {
	t.Helper()
	for idx := range def.Graph.Nodes {
		if def.Graph.Nodes[idx].ID == id {
			return &def.Graph.Nodes[idx]
		}
	}
	t.Fatalf("node %q not found", id)
	return nil
}

func requireLintCodes(t *testing.T, errs []loop.LintError, wantCodes ...string) {
	t.Helper()
	if len(wantCodes) == 0 {
		if len(errs) > 0 {
			t.Fatalf("Lint() errors = %#v, want none", errs)
		}
		return
	}

	got := make([]string, 0, len(errs))
	for _, err := range errs {
		if err.Message == "" {
			t.Fatalf("Lint() emitted empty message for code %q", err.Code)
		}
		if err.Severity != loop.SeverityError {
			t.Fatalf(
				"Lint() severity for code %q = %q, want %q",
				err.Code,
				err.Severity,
				loop.SeverityError,
			)
		}
		got = append(got, err.Code)
	}
	for _, want := range wantCodes {
		if !slices.Contains(got, want) {
			t.Fatalf("Lint() codes = %#v, want code %q; errors=%#v", got, want, errs)
		}
	}
}

func requireLintMessageContains(t *testing.T, errs []loop.LintError, code string, want string) {
	t.Helper()
	for _, err := range errs {
		if err.Code == code && strings.Contains(err.Message, want) {
			return
		}
	}
	t.Fatalf("Lint() errors = %#v, want code %q message containing %q", errs, code, want)
}

func requireLintWarning(t *testing.T, errs []loop.LintError, code string) {
	t.Helper()
	if len(errs) == 0 {
		t.Fatalf("Lint() errors = nil, want warning code %s", code)
	}
	for _, err := range errs {
		if err.Code == code && err.Severity == loop.SeverityWarning {
			return
		}
		if err.Severity == loop.SeverityError {
			t.Fatalf("Lint() emitted blocking error while looking for warning: %#v", errs)
		}
	}
	t.Fatalf("Lint() diagnostics = %#v, want warning code %s", errs, code)
}

type fakeToolSchemas map[string]loop.ToolSchemaSnapshot

func (s fakeToolSchemas) Snapshot(toolID string) (loop.ToolSchemaSnapshot, bool) {
	snapshot, ok := s[toolID]
	return snapshot, ok
}

func mustJSON(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

// Invariant: references and produces are checked against each node's literal view.
func TestLinterResultViewReferences(t *testing.T) {
	t.Parallel()
	t.Run("Should compose tool declarations with result views", testToolProducesReferences)
	const compact = `{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}`
	const full = `{"type":"object","properties":{"id":{"type":"string"},"details":{"type":"object","properties":{"name":{"type":"string"}},"additionalProperties":false},"scalar":{"type":"string"}},"additionalProperties":false}`
	for _, tc := range []struct {
		name          string
		view          any
		compact       string
		full          string
		path          string
		produces      bool
		override      dsl.Schema
		want          string
		warning       bool
		prefix        string
		consumerError bool
	}{
		{name: "Should reject annotated scalar schema metadata", view: "full", full: `{"type":"string","$comment":"scalar"}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject annotated closed-object metadata", view: "full", full: `{"type":"object","additionalProperties":false,"$comment":"empty"}`, path: "additionalProperties", want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata with extension annotations", view: "full", full: `{"type":"string","x-compozy-free-form":true}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata with a schema reference", view: "full", full: `{"type":"string","$ref":"#/$defs/value","$defs":{"value":{"type":"string"}}}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata with pattern properties", view: "full", full: `{"type":"object","additionalProperties":false,"patternProperties":{"^x":{"type":"string"}}}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata with conditional schemas", view: "full", full: `{"type":"string","else":{}}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject undeclared paths despite an open tool declaration from current", prefix: "", view: nil, path: "details.name", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite a closed tool declaration from current", prefix: "", view: nil, path: "type", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "additionalProperties": false}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite an open tool declaration from current", prefix: "", view: nil, path: "type", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should preserve a narrowing tool declaration from current", prefix: "", view: "full", path: "details.name", override: dsl.Schema{"id": "string"}, want: refs.CodeUnresolvablePath},
		{name: "Should accept declared compact fields with a tool declaration from current", prefix: "", view: nil, path: "id", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}, want: ""},
		{name: "Should reject undeclared paths despite an open tool declaration from previous.", prefix: "previous.", view: nil, path: "details.name", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite a closed tool declaration from previous.", prefix: "previous.", view: nil, path: "type", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "additionalProperties": false}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite an open tool declaration from previous.", prefix: "previous.", view: nil, path: "type", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should preserve a narrowing tool declaration from previous.", prefix: "previous.", view: "full", path: "details.name", override: dsl.Schema{"id": "string"}, want: refs.CodeUnresolvablePath},
		{name: "Should accept declared compact fields with a tool declaration from previous.", prefix: "previous.", view: nil, path: "id", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}, want: ""},
		{name: "Should reject undeclared paths despite an open tool declaration from best.", prefix: "best.", view: nil, path: "details.name", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite a closed tool declaration from best.", prefix: "best.", view: nil, path: "type", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "additionalProperties": false}, want: refs.CodeUnresolvablePath},
		{name: "Should reject metadata despite an open tool declaration from best.", prefix: "best.", view: nil, path: "type", override: dsl.Schema{"type": "object"}, want: refs.CodeUnresolvablePath},
		{name: "Should preserve a narrowing tool declaration from best.", prefix: "best.", view: "full", path: "details.name", override: dsl.Schema{"id": "string"}, want: refs.CodeUnresolvablePath},
		{name: "Should accept declared compact fields with a tool declaration from best.", prefix: "best.", view: nil, path: "id", override: dsl.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}, want: ""},
		{name: "Should bound a declared moved field at its consumer from current", prefix: "", path: "details.name", override: dsl.Schema{"details": map[string]any{"name": "string"}}, want: loop.CodeResultViewFieldRequiresFull, consumerError: true},
		{name: "Should bound declared keyword data at its consumer from current", prefix: "", path: "type", override: dsl.Schema{"type": "string", "id": "string"}, want: refs.CodeUnresolvablePath, consumerError: true},
		{name: "Should bound a declared moved field at its consumer from previous.", prefix: "previous.", path: "details.name", override: dsl.Schema{"details": map[string]any{"name": "string"}}, want: loop.CodeResultViewFieldRequiresFull, consumerError: true},
		{name: "Should bound declared keyword data at its consumer from previous.", prefix: "previous.", path: "type", override: dsl.Schema{"type": "string", "id": "string"}, want: refs.CodeUnresolvablePath, consumerError: true},
		{name: "Should bound a declared moved field at its consumer from best.", prefix: "best.", path: "details.name", override: dsl.Schema{"details": map[string]any{"name": "string"}}, want: loop.CodeResultViewFieldRequiresFull, consumerError: true},
		{name: "Should bound declared keyword data at its consumer from best.", prefix: "best.", path: "type", override: dsl.Schema{"type": "string", "id": "string"}, want: refs.CodeUnresolvablePath, consumerError: true},
		{name: "Should accept compact fields", path: "id"},
		{name: "Should reject schema keywords on a closed compact node", path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should reject schema keywords on a closed full node", view: "full", path: "properties.id", want: refs.CodeUnresolvablePath},
		{name: "Should reject keywords on a typeless closed object", view: "full", full: `{"additionalProperties":false}`, path: "additionalProperties", want: refs.CodeUnresolvablePath},
		{name: "Should accept explicitly declared keyword-named fields", view: "full", full: `{"type":"object","properties":{"type":{"type":"string"}},"additionalProperties":false}`, path: "type"},
		{name: "Should reject schema keywords on scalar full nodes", view: "full", full: `{"type":"string"}`, path: "type", want: refs.CodeUnresolvablePath},
		{name: "Should warn for keyword-named fields on an open full node", view: "full", full: `{"type":"object"}`, path: "type", warning: true},
		{name: "Should preserve produces restrictions on compact references", path: "details.name", override: dsl.Schema{"id": "string"}, want: refs.CodeUnresolvablePath},
		{name: "Should reject moved fields from previous generations", prefix: "previous.", path: "details.name", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should warn for open full fields from the best generation", prefix: "best.", view: "full", full: `{"type":"object"}`, path: "details.name", warning: true},
		{name: "Should require full for a moved field", path: "details.name", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should require full with an empty compact schema", compact: `{}`, path: "details.name", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should accept a declared full field", view: "full", path: "details.name"},
		{name: "Should accept open full fields with a warning", view: "full", full: `{"type":"object"}`, path: "details.name", warning: true},
		{name: "Should accept typeless open full fields with a warning", view: "full", full: `{}`, path: "details.name", warning: true},
		{name: "Should accept object union full fields with a warning", view: "full", full: `{"type":["object","null"]}`, path: "details.name", warning: true},
		{name: "Should require full for open full fields", full: `{"type":"object"}`, path: "details.name", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should reject unknown closed compact fields", path: "details.typo", want: refs.CodeUnresolvablePath},
		{name: "Should reject unknown closed full fields", view: "full", path: "details.typo", want: refs.CodeUnresolvablePath},
		{name: "Should reject children of a scalar full field", view: "full", path: "scalar.typo", want: refs.CodeUnresolvablePath},
		{name: "Should reject children of a scalar root", view: "full", full: `{"type":"string"}`, path: "typo", want: refs.CodeUnresolvablePath},
		{name: "Should reject children of an untyped array", view: "full", full: `{"type":"array"}`, path: "typo", want: refs.CodeUnresolvablePath},
		{name: "Should check produced shorthand array fields against compact", compact: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","additionalProperties":false}}},"additionalProperties":false}`, full: `{"type":"object"}`, path: "items.details", produces: true, override: dsl.Schema{"items": []any{map[string]any{"details": "string"}}}, want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should reject produced shorthand array fields absent from full", compact: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","additionalProperties":false}}},"additionalProperties":false}`, full: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","additionalProperties":false}}},"additionalProperties":false}`, path: "items.details", produces: true, override: dsl.Schema{"items": []any{map[string]any{"details": "string"}}}, want: refs.CodeUnresolvablePath},
		{name: "Should require full for produced moved fields", path: "details.name", produces: true, want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should reject produced fields absent from both views", path: "details.typo", produces: true, want: refs.CodeUnresolvablePath},
		{name: "Should reject produced fields absent from full", view: "full", path: "details.typo", produces: true, want: refs.CodeUnresolvablePath},
		{name: "Should accept produced open full fields with a warning", view: "full", full: `{"type":"object"}`, path: "details.name", produces: true, warning: true},
		{name: "Should reject a templated selector", view: "{{ .inputs.view }}", path: "id", want: loop.CodeResultViewNotLiteral},
		{name: "Should reject a non-string selector", view: true, path: "id", want: "schema_invalid"},
		{name: "Should reject an unknown selector", view: "summary", path: "id", want: "schema_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fullSchema := tc.full
			if fullSchema == "" {
				fullSchema = full
			}
			compactSchema := tc.compact
			if compactSchema == "" {
				compactSchema = compact
			}
			snapshot := loop.ToolSchemaSnapshot{
				ToolID:                 "compozy__fixture",
				OutputSchema:           json.RawMessage(compactSchema),
				FullOutputSchema:       json.RawMessage(fullSchema),
				FullOutputSchemaDigest: "full",
			}
			inspect := dsl.Node{
				ID:     "inspect",
				Class:  dsl.NodeClassAction,
				Kind:   snapshot.ToolID,
				Params: dsl.NodeParams{},
			}
			if tc.view != nil {
				inspect.Params["result_view"] = tc.view
			}
			if tc.produces {
				var value any = "string"
				parts := strings.Split(tc.path, ".")
				for _, part := range slices.Backward(parts) {
					value = map[string]any{part: value}
				}
				inspect.Produces = dsl.Schema(value.(map[string]any))
			}
			if tc.override != nil {
				inspect.Produces = tc.override
			}
			def := singleNodeDefinition(inspect)
			def.Inputs = map[string]dsl.Input{"view": {Type: dsl.InputTypeString}}
			if !tc.produces {
				def.Graph.Nodes = append(
					def.Graph.Nodes,
					dsl.Node{
						ID:    "report",
						Class: dsl.NodeClassAction,
						Kind:  "run-agent",
						Params: dsl.NodeParams{
							"agent":  "codex",
							"prompt": "Read {{ ." + tc.prefix + "nodes.inspect.output." + tc.path + " }}",
						},
					},
				)
				def.Graph.Edges = []dsl.Edge{{From: "inspect", To: "report"}}
			}
			diagnostics := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{snapshot.ToolID: snapshot})).
				Lint(def)
			var codes []string
			warned := false
			consumerRejected := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == loop.SeverityWarning {
					warned = warned || strings.Contains(diagnostic.Message, "cannot be proven")
					continue
				}
				codes = append(codes, diagnostic.Code)
				if diagnostic.NodeID == "report" && diagnostic.Code == tc.want {
					consumerRejected = true
				}
				if diagnostic.Code == loop.CodeResultViewFieldRequiresFull &&
					(!strings.Contains(diagnostic.Path, "nodes.inspect.output."+tc.path) || !strings.Contains(diagnostic.Message, snapshot.ToolID) || !strings.Contains(diagnostic.Message, "params of node \"inspect\"")) {
					t.Fatalf("missing source metadata: %#v", diagnostic)
				}
			}
			if tc.want == "" && len(codes) > 0 || tc.want != "" && !slices.Contains(codes, tc.want) {
				t.Fatalf("diagnostics = %#v, want %q", diagnostics, tc.want)
			}
			if tc.want == refs.CodeUnresolvablePath && slices.Contains(codes, loop.CodeResultViewFieldRequiresFull) {
				t.Fatal(diagnostics)
			}
			if tc.consumerError && !consumerRejected {
				t.Fatalf("consumer escaped selected view: %#v", diagnostics)
			}
			if tc.warning && !warned {
				t.Fatalf("missing open schema warning: %#v", diagnostics)
			}
		})
	}
	t.Run("Should reject selectors on native single-result tools", func(t *testing.T) {
		t.Parallel()
		node := dsl.Node{
			ID:     "inspect",
			Class:  dsl.NodeClassAction,
			Kind:   "compozy__fixture",
			Params: dsl.NodeParams{"result_view": "full"},
		}
		diagnostics := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{node.Kind: {ToolID: node.Kind, OutputSchema: json.RawMessage(compact)}})).
			Lint(singleNodeDefinition(node))
		requireLintMessageContains(
			t,
			diagnostics,
			"schema_invalid",
			`tool "compozy__fixture" returns a single result and does not accept "result_view"`,
		)
	})
	t.Run("Should select views independently for two nodes on one tool", func(t *testing.T) {
		t.Parallel()
		snapshot := loop.ToolSchemaSnapshot{
			ToolID:                 "compozy__fixture",
			OutputSchema:           json.RawMessage(compact),
			FullOutputSchema:       json.RawMessage(full),
			FullOutputSchemaDigest: "full",
		}
		def := singleNodeDefinition(
			dsl.Node{
				ID:     "full",
				Class:  dsl.NodeClassAction,
				Kind:   snapshot.ToolID,
				Params: dsl.NodeParams{"result_view": "full"},
			},
		)
		def.Graph.Nodes = append(
			def.Graph.Nodes,
			dsl.Node{ID: "compact", Class: dsl.NodeClassAction, Kind: snapshot.ToolID},
			dsl.Node{
				ID:    "report",
				Class: dsl.NodeClassAction,
				Kind:  "run-agent",
				Params: dsl.NodeParams{
					"agent":  "codex",
					"prompt": "{{ .nodes.full.output.details.name }} {{ .nodes.compact.output.details.name }}",
				},
			},
		)
		def.Graph.Edges = []dsl.Edge{{From: "full", To: "compact"}, {From: "compact", To: "report"}}
		diagnostics := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{snapshot.ToolID: snapshot})).Lint(def)
		count := 0
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == loop.SeverityError {
				count++
				if diagnostic.Code != loop.CodeResultViewFieldRequiresFull ||
					diagnostic.Path != "nodes.compact.output.details.name" {
					t.Fatal(diagnostic)
				}
			}
		}
		if count != 1 {
			t.Fatalf("diagnostics = %#v", diagnostics)
		}
	})
}

func TestLinterOutputShapeRuntimeAgreement(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve inferred non-tool references", testInferredNonToolReferences)
	t.Run("Should report full paths for list shorthand warnings", func(t *testing.T) {
		t.Parallel()
		for _, prompt := range []string{"{{ .nodes.origin.output.issues.title }}", "{{ range .nodes.origin.output.issues }}{{ .title }}{{ end }}"} {
			node := dsl.Node{
				ID:       "origin",
				Class:    dsl.NodeClassAction,
				Kind:     "transform",
				Produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}},
				Params: dsl.NodeParams{
					"map": map[string]any{"issues": map[string]any{"value": []any{map[string]any{"title": "issue"}}}},
				},
			}
			def := singleNodeDefinition(node)
			def.Graph.Nodes = append(
				def.Graph.Nodes,
				dsl.Node{
					ID:     "read",
					Class:  dsl.NodeClassAction,
					Kind:   "run-agent",
					Params: dsl.NodeParams{"agent": "codex", "prompt": prompt},
				},
			)
			def.Graph.Edges = []dsl.Edge{{From: "origin", To: "read"}}
			warned := false
			for _, d := range loop.NewLinter().Lint(def) {
				if d.Severity == loop.SeverityError {
					t.Fatal(d)
				}
				if d.Severity == loop.SeverityWarning && strings.Contains(d.Message, "cannot be proven") {
					warned = true
					if !strings.Contains(d.Message, "nodes.origin.output.issues.title") {
						t.Fatalf("relative warning: %#v", d)
					}
				}
			}
			if !warned {
				t.Fatal("missing list warning")
			}
		}
	})

	t.Run("Should preserve baseline acceptance for generated keyword fields", func(t *testing.T) {
		t.Parallel()
		keys := []string{"type", "title", "properties", "items", "id"}
		for mask := 1; mask < 1<<len(keys); mask++ {
			schema := dsl.Schema{}
			prompt := ""
			for index, key := range keys {
				if mask&(1<<index) != 0 {
					schema[key] = "string"
					prompt += "{{ .nodes.origin.output." + key + " }} "
				}
			}
			baseline := refs.Namespace{
				Nodes: map[string]refs.NodeSchema{"origin": {Output: refs.Schema(schema), HasOutput: true}},
			}
			for key := range schema {
				if err := baseline.ValidatePath([]string{"nodes", "origin", "output", key}); err != nil {
					t.Fatalf("baseline field %s: %v", key, err)
				}
			}
			for _, source := range []string{"produces", "output_schema", "expect", "tool-single", "tool-compact", "tool-full"} {
				producer := dsl.Node{
					ID:     "origin",
					Class:  dsl.NodeClassAction,
					Kind:   "run-agent",
					Params: dsl.NodeParams{"agent": "codex", "prompt": "Return"},
				}
				switch source {
				case "produces":
					producer.Produces = schema
				case "output_schema":
					producer.Params["output_schema"] = map[string]any(schema)
				case "expect":
					producer.Class = dsl.NodeClassControl
					producer.Kind = "ask"
					producer.Params = dsl.NodeParams{"prompt": "Return", "expect": map[string]any(schema)}
				default:
					producer.Kind = "ext__fixture"
					producer.Params = dsl.NodeParams{}
					producer.Produces = schema
				}
				snapshot := loop.ToolSchemaSnapshot{
					ToolID:       "ext__fixture",
					OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
				}
				if source == "tool-compact" || source == "tool-full" {
					properties := map[string]any{}
					for _, key := range keys {
						properties[key] = map[string]any{"type": "string"}
					}
					snapshot.OutputSchema = mustJSON(
						t,
						map[string]any{"type": "object", "properties": properties, "additionalProperties": false},
					)
					snapshot.FullOutputSchema = json.RawMessage(`{"type":"object"}`)
					snapshot.FullOutputSchemaDigest = "full"
					if source == "tool-full" {
						producer.Params["result_view"] = "full"
					}
				}
				def := singleNodeDefinition(producer)
				def.Graph.Nodes = append(
					def.Graph.Nodes,
					dsl.Node{
						ID:     "read",
						Class:  dsl.NodeClassAction,
						Kind:   "run-agent",
						Params: dsl.NodeParams{"agent": "codex", "prompt": prompt},
					},
				)
				def.Graph.Edges = []dsl.Edge{{From: "origin", To: "read"}}
				for _, diagnostic := range loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{snapshot.ToolID: snapshot})).Lint(def) {
					if diagnostic.Severity == loop.SeverityError {
						t.Fatalf("source=%s schema=%#v lost baseline acceptance: %#v", source, schema, diagnostic)
					}
				}
			}
		}
	})

	for _, tc := range []struct {
		name, source, schema, payload string
		valid, invalid, unproven      []string
		warning                       bool
	}{
		{name: "Should retain a keyword-only transform", source: "map", payload: `{"type":"bug"}`, valid: []string{"type"}, invalid: []string{"missing"}},
		{name: "Should retain all keyword-named transform fields", source: "map", payload: `{"type":"bug","title":"title","description":"detail"}`, valid: []string{"type", "title", "description"}},
		{name: "Should retain a properties-named transform field", source: "map", payload: `{"properties":"value","name":"name"}`, valid: []string{"properties", "name"}},
		{name: "Should normalize a properties shorthand field", schema: `{"properties":"object","name":"string"}`, payload: `{"properties":{},"name":"name"}`, valid: []string{"properties", "name"}, invalid: []string{"missing"}},
		{name: "Should normalize nested properties shorthand", schema: `{"properties":{"a":"string"},"id":"string"}`, payload: `{"properties":{"a":"a"},"id":"id"}`, valid: []string{"properties", "properties.a", "id", "a"}, unproven: []string{"a"}},
		{name: "Should normalize an array-named shorthand field", schema: `{"type":"array","items":"string","name":"string"}`, payload: `{"type":[],"items":"items","name":"name"}`, valid: []string{"type", "items", "name"}},
		{name: "Should allow an explicit open object with a warning", schema: `{"type":"object"}`, payload: `{"type":"value"}`, valid: []string{"type"}, warning: true},
		{name: "Should warn for children of an explicit string", schema: `{"type":"string"}`, payload: `"value"`, valid: []string{"type"}, unproven: []string{"type"}},
		{name: "Should warn for children of an annotated explicit string", schema: `{"type":"string","description":"string"}`, payload: `"value"`, valid: []string{"type", "description"}, unproven: []string{"type", "description"}},
		{name: "Should warn for metadata of an authored closed object", schema: `{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}`, payload: `{"id":"id"}`, valid: []string{"id", "type", "properties", "additionalProperties"}, unproven: []string{"type", "properties", "additionalProperties"}, invalid: []string{"missing"}},
		{name: "Should warn for metadata of a nested authored string", schema: `{"user":{"type":"string"},"id":"string"}`, payload: `{"user":"value","id":"id"}`, valid: []string{"user", "id", "user.type"}, unproven: []string{"user.type"}},
		{name: "Should warn for authored array metadata", schema: `{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}}`, payload: `[{"id":"id"}]`, valid: []string{"type", "items.type"}, unproven: []string{"type", "items.type"}},
		{name: "Should preserve keyword collisions in authored properties", schema: `{"type":"object","properties":{"type":{"type":"string"}},"additionalProperties":false}`, payload: `{"type":"value"}`, valid: []string{"type", "type.type"}, unproven: []string{"type.type"}},
		{name: "Should retain Goal authored compatibility", source: "goal-output_schema", schema: `{"status":{"type":"string","enum":["complete","blocked"]},"user":{"type":"string"}}`, payload: `{"status":"complete","user":"value"}`, valid: []string{"status", "user", "user.type"}, unproven: []string{"user.type"}},
	} {
		sources := []string{tc.source}
		if tc.source == "" {
			sources = []string{"produces", "output_schema", "expect"}
		}
		for _, source := range sources {
			t.Run(tc.name+" from "+source, func(t *testing.T) {
				t.Parallel()
				node := dsl.Node{
					ID:     "shape",
					Class:  dsl.NodeClassAction,
					Kind:   "run-agent",
					Params: dsl.NodeParams{"agent": "codex", "prompt": "Return output"},
				}
				var schema dsl.Schema
				if source == "map" {
					var values map[string]any
					if err := json.Unmarshal([]byte(tc.payload), &values); err != nil {
						t.Fatal(err)
					}
					fields := map[string]any{}
					for key, value := range values {
						fields[key] = map[string]any{"value": value}
					}
					node.Kind = "transform"
					node.Params = dsl.NodeParams{"map": fields}
					raw, err := (loop.TransformActionExecutor{}).Execute(t.Context(), node, loop.ActionExecutionInput{})
					if err != nil {
						t.Fatal(err)
					}
					expected, err := json.Marshal(values)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(raw.Structured, expected) {
						t.Fatalf("runtime output=%s want %s", raw.Structured, expected)
					}
				} else {
					if err := json.Unmarshal([]byte(tc.schema), &schema); err != nil {
						t.Fatal(err)
					}
					if _, err := loop.ValidateActionStructured(
						schema,
						loop.ActionPromptResult{Structured: json.RawMessage(tc.payload)},
					); err != nil {
						t.Fatalf("runtime rejected payload: %v", err)
					}
					if strings.HasPrefix(tc.payload, `"`) {
						if _, err := loop.ValidateActionStructured(
							schema,
							loop.ActionPromptResult{Structured: json.RawMessage(`{"type":"value"}`)},
						); err == nil {
							t.Fatal("runtime accepted an object for a string schema")
						}
					}
					switch source {
					case "produces":
						node.Produces = schema
					case "output_schema":
						node.Params[source] = map[string]any(schema)
					case "goal-output_schema":
						node.Kind = "goal"
						node.Params = dsl.NodeParams{
							"agent":         "codex",
							"objective":     "Return output",
							"max_turns":     1,
							"judge":         []any{map[string]any{"id": "verify", "type": "command", "check": "true"}},
							"output_schema": map[string]any(schema),
						}
					case "expect":
						node.Class = dsl.NodeClassControl
						node.Kind = "ask"
						node.Params = dsl.NodeParams{"prompt": "Return output", "expect": map[string]any(schema)}
					}
				}
				for _, field := range append(slices.Clone(tc.valid), tc.invalid...) {
					if slices.Contains(tc.unproven, field) {
						var output any
						if err := json.Unmarshal([]byte(tc.payload), &output); err != nil {
							t.Fatal(err)
						}
						namespace := map[string]any{"nodes": map[string]any{"shape": map[string]any{"output": output}}}
						expression := "{{ .nodes.shape.output." + field + " }}"
						if rendered, err := refs.RenderTemplateString(
							"authored-output",
							expression,
							namespace,
						); err == nil ||
							rendered != "" {
							t.Fatalf("runtime rendered=%q err=%v, want failed evaluation", rendered, err)
						}
						for _, mapping := range []map[string]any{{"template": expression}, {"from": "nodes.shape.output." + field}} {
							consumer := dsl.Node{
								ID:     "consume",
								Class:  dsl.NodeClassAction,
								Kind:   "transform",
								Params: dsl.NodeParams{"map": map[string]any{"value": mapping}},
							}
							if _, err := (loop.TransformActionExecutor{}).Execute(
								t.Context(),
								consumer,
								loop.ActionExecutionInput{Namespace: namespace},
							); !errors.Is(
								err,
								loop.ErrActionMaterialization,
							) {
								t.Fatalf("runtime consumer error=%v", err)
							}
						}
						if field == "a" {
							values := output.(map[string]any)
							values["a"] = "root"
							extra, err := json.Marshal(values)
							if err != nil {
								t.Fatal(err)
							}
							if _, err := loop.ValidateActionStructured(
								schema,
								loop.ActionPromptResult{Structured: extra},
							); err != nil {
								t.Fatalf("additional root field rejected: %v", err)
							}
							if rendered, err := refs.RenderTemplateString(
								"authored-output",
								expression,
								namespace,
							); err != nil ||
								rendered != "root" {
								t.Fatalf("runtime root a=%q err=%v", rendered, err)
							}
						}
					}
					for _, prefix := range []string{"", "previous.", "best."} {
						def := singleNodeDefinition(node)
						def.Graph.Nodes = append(
							def.Graph.Nodes,
							dsl.Node{
								ID:    "read",
								Class: dsl.NodeClassAction,
								Kind:  "run-agent",
								Params: dsl.NodeParams{
									"agent":  "codex",
									"prompt": "{{ ." + prefix + "nodes.shape.output." + field + " }}",
								},
							},
						)
						def.Graph.Edges = []dsl.Edge{{From: "shape", To: "read"}}
						diagnostics := loop.NewLinter().Lint(def)
						rejected, warned := false, false
						for _, d := range diagnostics {
							if d.Severity == loop.SeverityError {
								if d.Code != refs.CodeUnresolvablePath {
									t.Fatalf("unexpected diagnostic: %#v", d)
								}
								rejected = true
								if field == "a" && !strings.Contains(d.Message, "available fields: id, properties") {
									t.Errorf("lost declared field names: %#v", d)
								}
							}
							if d.Severity == loop.SeverityWarning && strings.Contains(d.Message, "cannot be proven") {
								warned = true
							}
						}
						t.Logf(
							"source=%s prefix=%s schema=%s field=%s rejected=%t warning=%t",
							source,
							prefix,
							tc.schema,
							field,
							rejected,
							warned,
						)
						if rejected != slices.Contains(tc.invalid, field) {
							t.Errorf("field=%s diagnostics=%#v", field, diagnostics)
						}
						if (tc.warning || slices.Contains(tc.unproven, field)) && !warned {
							t.Errorf("field=%s missing open-object warning: %#v", field, diagnostics)
						}
					}
				}
			})
		}
	}
}

func testInferredNonToolReferences(t *testing.T) {
	t.Helper()
	t.Parallel()
	for _, kind := range []string{"direct-string", "direct-number", "direct-boolean", "direct-file", "direct-agent", "direct-ref", "direct-runtime", "source-input", "await-loop", "detach-loop"} {
		t.Run("Should retain "+kind+" reference acceptance", func(t *testing.T) {
			t.Parallel()
			input := dsl.Input{Type: dsl.InputTypeString}
			field := "type"
			switch kind {
			case "direct-number":
				input.Type = dsl.InputTypeNumber
			case "direct-boolean":
				input.Type = dsl.InputTypeBoolean
			case "direct-file":
				input.Type = dsl.InputTypeFile
			case "direct-agent":
				input.Type = dsl.InputTypeAgent
			case "direct-ref":
				input.Type = dsl.InputTypeRef
				input.Ref = &dsl.InputRef{Kind: dsl.InputRefKindSkill}
			case "direct-runtime":
				input.Type = dsl.InputTypeRuntime
				field = "provider.type"
			case "detach-loop":
				field = "loop_run_id"
			}
			for _, prefix := range []string{"", "previous.", "best."} {
				producer := dsl.Node{
					ID:       "origin",
					Class:    dsl.NodeClassSource,
					Kind:     string(dsl.SourceInput),
					InputRef: "subject",
				}
				if kind == "await-loop" || kind == "detach-loop" {
					mode := "await"
					if kind == "detach-loop" {
						mode = "detach"
					}
					producer = dsl.Node{
						ID:     "origin",
						Class:  dsl.NodeClassAction,
						Kind:   "run-loop",
						Params: dsl.NodeParams{"loop": "child", "mode": mode, "inputs": map[string]any{}},
					}
				}
				path := prefix + "nodes.origin.output." + field
				if strings.HasPrefix(kind, "direct-") {
					if prefix != "" {
						continue
					}
					path = "inputs.subject." + field
				}
				def := singleNodeDefinition(producer)
				def.Inputs = map[string]dsl.Input{"subject": input}
				def.Graph.Nodes = append(
					def.Graph.Nodes,
					dsl.Node{
						ID:     "read",
						Class:  dsl.NodeClassAction,
						Kind:   "run-agent",
						Params: dsl.NodeParams{"agent": "codex", "prompt": "{{ ." + path + " }}"},
					},
				)
				def.Graph.Edges = []dsl.Edge{{From: "origin", To: "read"}}
				diagnostics := loop.NewLinter().Lint(def)
				warned := false
				for _, diagnostic := range diagnostics {
					if diagnostic.Severity == loop.SeverityError {
						t.Errorf("path=%s unexpected error=%#v", path, diagnostic)
					}
					if diagnostic.Severity == loop.SeverityWarning &&
						strings.Contains(diagnostic.Message, "cannot be proven") {
						warned = true
					}
				}
				if kind != "detach-loop" && !warned {
					t.Errorf("path=%s missing warning: %#v", path, diagnostics)
				}
				if kind == "detach-loop" && warned {
					t.Errorf("proven handle warned: %#v", diagnostics)
				}
			}
		})
	}
}

func testToolProducesReferences(t *testing.T) {
	t.Helper()
	t.Parallel()
	const closed = `{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}`
	const lists = `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}},"issues":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string"}},"additionalProperties":false}},"type":{"type":"string"},"properties":{"type":"string"},"id":{"type":"string"},"extra":{"type":"string"}},"additionalProperties":false}`
	for _, tc := range []struct {
		name, toolID, compact, full, path, want string
		produces                                dsl.Schema
		warning, fullView, rawReject            bool
	}{
		{name: "Should accept single-result list declarations", compact: closed, produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", warning: true},
		{name: "Should accept single-result keyword lists", compact: closed, produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}}, path: "items", warning: true},
		{name: "Should traverse single-result keyword lists", compact: closed, produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}}, path: "items.id", warning: true},
		{name: "Should accept single-result keyword fields", compact: closed, produces: dsl.Schema{"type": "string", "properties": "string", "id": "string"}, path: "type"},
		{name: "Should replace a closed single-result descriptor", compact: closed, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra"},
		{name: "Should replace a closed MCP descriptor", toolID: "mcp__fixture", compact: closed, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra"},
		{name: "Should replace a closed native single-result descriptor", toolID: "compozy__fixture", compact: closed, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra"},
		{name: "Should accept a declared extra field in compact", compact: lists, full: lists, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra"},
		{name: "Should require full for a declared extra field", compact: closed, full: lists, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should reject extra fields in a pinned closed full view", compact: closed, full: closed, fullView: true, produces: dsl.Schema{"id": "string", "extra": "string"}, path: "extra", want: refs.CodeUnresolvablePath},
		{name: "Should accept declarations without a descriptor schema", produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", warning: true},
		{name: "Should accept declarations over an open single-result descriptor", compact: `{"type":"object"}`, produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", warning: true},
		{name: "Should accept pinned compact list fields", compact: lists, full: lists, produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", warning: true},
		{name: "Should accept pinned compact keyword lists", compact: lists, full: lists, produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}}, path: "items", warning: true},
		{name: "Should traverse pinned compact keyword lists", compact: lists, full: lists, produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}}, path: "items.id", warning: true},
		{name: "Should require full for declared list fields", compact: closed, full: lists, produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", want: loop.CodeResultViewFieldRequiresFull},
		{name: "Should reject list fields absent from closed full", compact: closed, full: closed, produces: dsl.Schema{"issues": []any{map[string]any{"title": "string"}}}, path: "issues.title", want: refs.CodeUnresolvablePath},
		{name: "Should warn for descriptor-open keyword fields", fullView: true, compact: closed, full: `{"type":"object"}`, produces: dsl.Schema{"type": "string", "id": "string"}, path: "type", warning: true},
		{name: "Should reject paths absent from the raw declaration", rawReject: true, compact: closed, full: lists, produces: dsl.Schema{"type": "object"}, path: "issues.title", want: refs.CodeUnresolvablePath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, prefix := range []string{"", "previous.", "best."} {
				for _, form := range []string{"template", "from", "range"} {
					base, leaf, nested := strings.Cut(tc.path, ".")
					if form == "range" && !nested {
						continue
					}
					toolID := tc.toolID
					if toolID == "" {
						toolID = "ext__fixture"
					}
					snapshot := loop.ToolSchemaSnapshot{
						ToolID:           toolID,
						OutputSchema:     json.RawMessage(tc.compact),
						FullOutputSchema: json.RawMessage(tc.full),
					}
					if tc.full != "" {
						snapshot.FullOutputSchemaDigest = "full"
					}
					producer := dsl.Node{
						ID:       "inspect",
						Class:    dsl.NodeClassAction,
						Kind:     snapshot.ToolID,
						Produces: tc.produces,
						Params:   dsl.NodeParams{},
					}
					if tc.fullView {
						producer.Params["result_view"] = "full"
					}
					path := prefix + "nodes.inspect.output." + tc.path
					baseline := refs.Namespace{
						Nodes: map[string]refs.NodeSchema{
							"inspect": {Output: refs.Schema(tc.produces), HasOutput: true},
						},
					}
					if err := baseline.ValidatePath(strings.Split(path, ".")); (err != nil) != tc.rawReject {
						t.Fatalf("baseline declaration %s: %v", path, err)
					}
					consumer := dsl.Node{
						ID:     "report",
						Class:  dsl.NodeClassAction,
						Kind:   "run-agent",
						Params: dsl.NodeParams{"agent": "codex", "prompt": "{{ ." + path + " }}"},
					}
					if form == "from" {
						consumer.Kind = "transform"
						consumer.Params = dsl.NodeParams{"map": map[string]any{"value": map[string]any{"from": path}}}
					}
					if form == "range" {
						consumer.Params["prompt"] = "{{ range ." + prefix + "nodes.inspect.output." + base + " }}{{ ." + leaf + " }}{{ end }}"
					}
					def := singleNodeDefinition(producer)
					def.Graph.Nodes = append(def.Graph.Nodes, consumer)
					def.Graph.Edges = []dsl.Edge{{From: "inspect", To: "report"}}
					diagnostics := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{snapshot.ToolID: snapshot})).
						Lint(def)
					consumerError, warned := false, false
					for _, diagnostic := range diagnostics {
						if diagnostic.Severity == loop.SeverityError {
							if tc.want == "" || diagnostic.Code != tc.want {
								t.Fatalf("%s %s: %#v", path, form, diagnostics)
							}
							consumerError = consumerError || diagnostic.NodeID == "report"
						}
						warned = warned ||
							diagnostic.NodeID == "report" && diagnostic.Severity == loop.SeverityWarning &&
								strings.Contains(diagnostic.Message, "cannot be proven")
					}
					if consumerError != (tc.want != "") || tc.want == "" && warned != tc.warning {
						t.Fatalf("%s %s: %#v", path, form, diagnostics)
					}
				}
			}
		})
	}
}

func TestLinterBuiltinComposedResultViews(t *testing.T) {
	t.Parallel()
	for _, descriptor := range builtin.NativeDescriptors() {
		path := ""
		var produces dsl.Schema
		switch descriptor.ID.String() {
		case "compozy__goal_get":
			path = "goal.run_id"
			produces = dsl.Schema{"goal": map[string]any{"run_id": "string"}}
		case "compozy__agent_list":
			path = "agents.name"
			produces = dsl.Schema{"agents": []any{map[string]any{"name": "string"}}}
		default:
			continue
		}
		views := descriptor.ResultViews()
		for _, view := range []string{"compact", "full"} {
			for _, declared := range []bool{false, true} {
				for _, prefix := range []string{"", "previous.", "best."} {
					t.Run(
						fmt.Sprintf("Should accept %s %s produces %v %s", descriptor.ID, view, declared, prefix),
						func(t *testing.T) {
							t.Parallel()
							snapshot := loop.ToolSchemaSnapshot{
								ToolID:                 descriptor.ID.String(),
								OutputSchema:           views.Compact.Schema,
								FullOutputSchema:       views.Full.Schema,
								FullOutputSchemaDigest: "full",
							}
							node := dsl.Node{
								ID:     "inspect",
								Class:  dsl.NodeClassAction,
								Kind:   snapshot.ToolID,
								Params: dsl.NodeParams{"result_view": view},
							}
							if declared {
								node.Produces = produces
							}
							def := singleNodeDefinition(node)
							def.Graph.Nodes = append(
								def.Graph.Nodes,
								dsl.Node{
									ID:    "report",
									Class: dsl.NodeClassAction,
									Kind:  "run-agent",
									Params: dsl.NodeParams{
										"agent":  "codex",
										"prompt": "{{ ." + prefix + "nodes.inspect.output." + path + " }}",
									},
								},
							)
							def.Graph.Edges = []dsl.Edge{{From: "inspect", To: "report"}}
							for _, d := range loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{snapshot.ToolID: snapshot})).Lint(def) {
								if d.Severity == loop.SeverityError {
									t.Error(d)
								}
							}
						},
					)
				}
			}
		}
	}
}
