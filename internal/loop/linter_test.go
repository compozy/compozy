package loop_test

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/network/participation"
)

func TestLinterShouldValidateDefinitionNetworkParticipation(t *testing.T) {
	t.Parallel()

	modeLocal := participation.ModeLocal
	modeLive := participation.ModeLive
	strategyNamed := participation.StrategyNamed
	channel := " builders "
	testCases := []struct {
		name        string
		request     *participation.Request
		wantCode    string
		wantMessage string
	}{
		{
			name:        "Should reject default Local participation for a Network node",
			wantCode:    loop.CodeLoopRequiresLive,
			wantMessage: "agent",
		},
		{
			name:        "Should reject explicit Local participation for a Network node",
			request:     &participation.Request{Mode: &modeLocal},
			wantCode:    loop.CodeLoopRequiresLive,
			wantMessage: "compozy__network_send",
		},
		{
			name: "Should accept and canonicalize valid Live participation during compile",
			request: &participation.Request{
				Mode:            &modeLive,
				ChannelStrategy: &strategyNamed,
				ChannelID:       &channel,
			},
		},
		{
			name: "Should reject an invalid Local participation envelope",
			request: &participation.Request{
				Mode:            &modeLocal,
				ChannelStrategy: &strategyNamed,
				ChannelID:       &channel,
			},
			wantCode:    loop.CodeNetworkParticipationInvalid,
			wantMessage: "local mode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			def := validDefinition()
			def.NetworkParticipation = tc.request
			def.Graph.Nodes[2].Kind = "compozy__network_send"
			linter := loop.NewLinter(loop.WithToolSchemaSource(fakeToolSchemas{
				"compozy__network_send": {ToolID: "compozy__network_send"},
			}))
			errs := linter.Lint(def)
			if tc.wantCode == "" {
				requireLintCodes(t, errs)
				return
			}
			requireLintCodes(t, errs, tc.wantCode)
			requireLintMessageContains(t, errs, tc.wantCode, tc.wantMessage)
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
		{
			name: "Should reject a non-finite metric minimum delta",
			mutate: func(def *dsl.Definition) {
				appendMetricGate(def, dsl.CriterionCommand, &dsl.MetricSpec{
					Direction: dsl.MetricMaximize,
					MinDelta:  new(math.NaN()),
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
		definition.Contract.StopWhen = "previous.generation >= 1 && best.score >= 0"
		requireNode(t, &definition, "agent").Params["prompt"] =
			"Repair {{ .previous.verdicts.quality.blocking_issues }} from {{ .best.nodes.agent.output.summary }}"
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
			if reference.asCondition {
				definition.Contract.StopWhen = reference.path + " != null"
			} else {
				requireNode(t, &definition, "agent").Params["prompt"] = "{{ ." + reference.path + " }}"
			}
			requireLintCodes(t, loop.NewLinter().Lint(definition), refs.CodeUnresolvablePath)
		})
	}
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
			name: "Should reject fan out over ceiling",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "fan")
				node.MaxFanOut = loop.LoopMaxFanoutWidth + 1
			},
			wantCodes: []string{loop.CodeFanOutCeilingExceeded},
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
			name: "Should reject deleted channel post action as unknown",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "channel-post"
			},
			wantCodes: []string{loop.CodeUnknownActionKind},
		},
		{
			name: "Should reject channel result harvest on reserved action kinds",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Harvest = &dsl.HarvestSpec{Kind: "channel_result", Window: "5m"}
			},
			wantCodes: []string{loop.CodeInvalidHarvest},
		},
		{
			name: "Should reject channel result harvest on non network send actions",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "compozy__task_read"
				node.Harvest = &dsl.HarvestSpec{Kind: "channel_result", Window: "5m"}
			},
			wantCodes: []string{loop.CodeInvalidHarvest},
		},
		{
			name: "Should reject channel result harvest without a positive window",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "compozy__network_send"
				node.Harvest = &dsl.HarvestSpec{Kind: "channel_result", Window: "0s"}
			},
			wantCodes: []string{loop.CodeInvalidHarvest},
		},
		{
			name: "Should reject unsupported channel result content rules",
			mutate: func(def *dsl.Definition) {
				node := requireNode(t, def, "agent")
				node.Kind = "compozy__network_send"
				node.Harvest = &dsl.HarvestSpec{
					Kind:        "channel_result",
					Window:      "5m",
					ContentRule: "jsonpath:decision",
				}
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
				def.Contract.StopWhen = `nodes.agent.output.summary.contains("done")`
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
				def.Contract.StopWhen = `nodes.agent.output.issues[0].id == "issue-1"`
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
				def.Contract.StopWhen = `nodes.agent.output.issues[0].missing == "issue-1"`
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

func TestLinterShouldAcceptCompileTimeCeilingsAtBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should accept configured ceilings exactly at the boundary", func(t *testing.T) {
		t.Parallel()

		def := validDefinition()
		requireNode(t, &def, "fan").MaxFanOut = loop.LoopMaxFanoutWidth
		requireNode(t, &def, "fan").MaxParallel = loop.LoopMaxFanoutWidth
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
			name: "Should reject fan out max parallel over ceiling",
			def: singleNodeDefinition(dsl.Node{
				ID:          "fan",
				Class:       dsl.NodeClassControl,
				Kind:        string(dsl.ControlFanOut),
				Collection:  "{{ .inputs.items }}",
				MaxFanOut:   1,
				MaxParallel: loop.LoopMaxFanoutWidth + 1,
			}),
			wantCodes: []string{loop.CodeFanOutCeilingExceeded},
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
								"prompt": "Summarize {{ .nodes.nested_load.output.type }}",
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
			name: "Should reject empty file import parse",
			def: singleNodeDefinition(dsl.Node{
				ID:      "files",
				Class:   dsl.NodeClassSource,
				Kind:    string(dsl.SourceFileImport),
				Pattern: "docs/*.md",
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
			name: "Should reject network peer lifecycle kinds without durable anchors",
			def: singleNodeDefinition(watchEventsNodeForTest([]dsl.EventSubscription{{
				Kind: "network.peer.joined",
			}})),
			wantCodes:     []string{loop.CodeWatchEventsKindUnsupported},
			messageCode:   loop.CodeWatchEventsKindUnsupported,
			messageSubstr: "network.message.persisted",
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
			name: "Should reject an effect without a kind",
			mutate: func(def *dsl.Definition) {
				requireNode(t, def, "agent").OnSuccess = []dsl.EffectSpec{{}}
			},
			wantCode: loop.CodeEffectShapeInvalid,
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
			NodeLifecycleState: &dsl.NodeLifecycleState{OnParentClose: dsl.ParentCloseCancel},
		})
		requireLintCodes(t, loop.NewLinter().Lint(valid))

		invalidKind := validDefinition()
		invalidKind.Normalize()
		requireNode(t, &invalidKind, "agent").OnParentClose = dsl.ParentCloseCancel
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
			"tasks": {Type: dsl.InputTypeRef, Required: true},
		},
		Contract: dsl.Contract{
			Goal:             "Complete task list",
			DefinitionOfDone: "All tasks have summaries",
			Verification:     []dsl.GateCriterion{},
			ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalStates: []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
			},
			IterationCap: 3,
			NoProgress: dsl.NoProgress{
				Window:     2,
				HashFields: []string{"nodes.agent.output.summary"},
			},
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
	def.Contract.NoProgress.HashFields = []string{"nodes.agent.output.status"}
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

func singleNodeDefinition(node dsl.Node) dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "single-node"},
		Inputs: map[string]dsl.Input{
			"items": {Type: dsl.InputTypeRef},
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
