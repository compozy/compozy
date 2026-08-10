package gate

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
)

type commandRunnerFunc func(context.Context, CommandRequest) (CommandResult, error)

func (f commandRunnerFunc) RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error) {
	return f(ctx, req)
}

type judgeRunnerFunc func(context.Context, JudgeRequest) (JudgeResponse, error)

func (f judgeRunnerFunc) Judge(ctx context.Context, req JudgeRequest) (JudgeResponse, error) {
	return f(ctx, req)
}

type toolCallerFunc func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error)

func (f toolCallerFunc) Call(ctx context.Context, scope tools.Scope, req tools.CallRequest) (tools.ToolResult, error) {
	return f(ctx, scope, req)
}

func TestEvaluatorEvaluateCriteriaMapping(t *testing.T) {
	t.Parallel()

	t.Run("Should map command exit code into an approved criterion", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(_ context.Context, req CommandRequest) (CommandResult, error) {
				if req.Command != "make test" {
					t.Fatalf("CommandRequest.Command = %q, want make test", req.Command)
				}
				return CommandResult{ExitCode: 0, Stdout: "ok"}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "verify_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:    "unit_tests",
				Type:  dsl.CriterionCommand,
				Check: "make test",
			}},
		}, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Route.Action != RouteContinue {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteContinue)
		}
		if got, want := verdict.Criteria[0].Stdout, "ok"; got != want {
			t.Fatalf("Criterion.Stdout = %q, want %q", got, want)
		}
	})

	t.Run("Should map malformed judge JSON to revise without marking Broken", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{Raw: "not-json"}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "judge_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{{
				ID:     "rubric",
				Type:   dsl.CriterionAgentJudge,
				Rubric: "Check the output",
			}},
		}, GateInput{Placement: PlacementInBody, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		if verdict.Broken {
			t.Fatal("Verdict.Broken = true, want false")
		}
		requireIssueID(t, verdict.BlockingIssues, JudgeMalformedOutputIssueID)
		if verdict.Route.Action != RouteRevise {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteRevise)
		}
	})

	t.Run("Should reject judge output outside the exact single-object contract", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			raw  string
		}{
			{
				name: "Should reject prose around an otherwise valid verdict",
				raw:  `Result: {"verdict":"pass","evidence":{"checked":true}}`,
			},
			{
				name: "Should reject a fenced verdict",
				raw:  "```json\n{\"verdict\":\"pass\",\"evidence\":{\"checked\":true}}\n```",
			},
			{
				name: "Should reject multiple JSON objects",
				raw:  `{"verdict":"pass","evidence":{"checked":true}} {"verdict":"revise"}`,
			},
			{
				name: "Should reject fields outside the verdict schema",
				raw:  `{"verdict":"pass","evidence":{"checked":true},"commentary":"done"}`,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				result := ParseJudgeVerdict("judge", dsl.CriterionAgentJudge, tc.raw)
				if result.Outcome != VerdictOutcomeRejected || result.Broken {
					t.Fatalf("ParseJudgeVerdict() = %#v, want non-broken rejection", result)
				}
				requireIssueID(t, result.BlockingIssues, JudgeMalformedOutputIssueID)
			})
		}
	})

	t.Run("Should route agent judge through criterion model before effective default", func(t *testing.T) {
		t.Parallel()

		var runtimes []dsl.RuntimeSpec
		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(_ context.Context, req JudgeRequest) (JudgeResponse, error) {
				runtimes = append(runtimes, req.Runtime)
				return JudgeResponse{
					Raw: `{"verdict":"approved","evidence":{"checked":["model"]}}`,
				}, nil
			},
		)))
		_, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "judge_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{
				{
					ID:      "explicit",
					Type:    dsl.CriterionAgentJudge,
					Rubric:  "Check explicit model",
					Runtime: dsl.RuntimeSpec{Model: "criterion-model"},
				},
				{
					ID:     "defaulted",
					Type:   dsl.CriterionAgentJudge,
					Rubric: "Check default model",
				},
			},
		}, GateInput{
			Placement: PlacementInBody,
			Contract:  new(validContract()),
			JudgeRuntime: dsl.RuntimeSpec{
				Provider: "codex", Model: "default-judge-model", Reasoning: "high",
			},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if len(runtimes) != 2 {
			t.Fatalf("judge calls = %d, want 2", len(runtimes))
		}
		if runtimes[0].Provider != "codex" || runtimes[0].Model != "criterion-model" ||
			runtimes[0].Reasoning != "high" {
			t.Fatalf("explicit runtime = %#v, want field-merged criterion runtime", runtimes[0])
		}
		if runtimes[1].Provider != "codex" || runtimes[1].Model != "default-judge-model" ||
			runtimes[1].Reasoning != "high" {
			t.Fatalf("default runtime = %#v, want judge defaults", runtimes[1])
		}
	})

	t.Run("Should map judge transport failure to Broken for fail-open routing", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{}, errors.New("agent unavailable")
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "judge_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{{
				ID:     "rubric",
				Type:   dsl.CriterionAgentJudge,
				Rubric: "Check the output",
			}},
		}, GateInput{Placement: PlacementDefinitionOfDone, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !verdict.Broken {
			t.Fatal("Verdict.Broken = false, want true")
		}
		if verdict.Route.Action != RouteNextGeneration {
			t.Fatalf("Route.Action = %q, want fail-open %q", verdict.Route.Action, RouteNextGeneration)
		}
		if verdict.Route.Action == RouteDone || verdict.Route.Action == RouteContinue {
			t.Fatalf("Route.Action = %q, want no false done or DoD continue for broken judge", verdict.Route.Action)
		}
	})

	t.Run("Should derive complete judge correlation without a tool call owner", func(t *testing.T) {
		t.Parallel()

		var request JudgeRequest
		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(_ context.Context, req JudgeRequest) (JudgeResponse, error) {
				request = req
				return JudgeResponse{
					Raw: `{"verdict":"approved","evidence":{"checked":["identity"]}}`,
				}, nil
			},
		)))
		_, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "quality_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID: "quality", Type: dsl.CriterionAgentJudge, Rubric: "Check the output",
			}},
		}, GateInput{
			LoopRunID: "looprun-1", Placement: PlacementDefinitionOfDone,
			Revision: 2, Contract: new(validContract()),
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if request.CorrelationID != "loop-judge:looprun-1:quality_gate:quality:3" {
			t.Fatalf("JudgeRequest.CorrelationID = %q, want deterministic loop judge identity", request.CorrelationID)
		}
	})

	t.Run("Should map human approval decisions through ActorContext", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator().Evaluate(context.Background(), Gate{
			ID:            "human_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{{
				ID:   "owner_approval",
				Type: dsl.CriterionHuman,
			}},
		}, GateInput{
			Placement: PlacementDefinitionOfDone,
			HumanDecisions: map[string]HumanDecision{
				"owner_approval": {
					Decision: HumanDecisionApprove,
					Actor:    validHumanActor(t),
					Note:     "ship",
				},
			},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Route.Action != RouteDone {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteDone)
		}
	})

	t.Run("Should map extension structured output into a verdict", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithToolCaller(toolCallerFunc(
			func(_ context.Context, _ tools.Scope, req tools.CallRequest) (tools.ToolResult, error) {
				if req.ToolID != "ext__quality__gate" {
					t.Fatalf("CallRequest.ToolID = %q, want ext__quality__gate", req.ToolID)
				}
				return tools.ToolResult{
					Structured: []byte(
						`{"verdict":"revise","blocking_issues":[{"id":"coverage_gap","note":"missing branch"}]}`,
					),
				}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "extension_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:     "extension_quality",
				Type:   dsl.CriterionExtension,
				Tool:   "ext__quality__gate",
				Inputs: map[string]any{"path": "internal/loop"},
			}},
		}, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		requireIssueID(t, verdict.BlockingIssues, "coverage_gap")
	})
}

func TestEvaluatorEvaluateCriteriaBundle(t *testing.T) {
	t.Parallel()

	t.Run("Should evaluate criteria bundle with per-criterion results", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(
			WithCommandRunner(commandRunnerFunc(func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0}, nil
			})),
			WithJudgeRunner(judgeRunnerFunc(func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{
					Raw: `{"verdict":"revise","blocking_issues":[{"id":"missing_tests","note":"add coverage"},{"id":"doc_gap","note":"document route"}],"evidence":{"checked":["tests"]}}`,
				}, nil
			})),
		)
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "bundle_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{
				{ID: "unit_tests", Type: dsl.CriterionCommand, Check: "go test ./internal/loop/gate"},
				{ID: "judge", Type: dsl.CriterionAgentJudge, Rubric: "Review the implementation"},
			},
		}, GateInput{Placement: PlacementDefinitionOfDone, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		if got, want := len(verdict.Criteria), 2; got != want {
			t.Fatalf("len(Criteria) = %d, want %d", got, want)
		}
		if !verdict.Criteria[0].Passed {
			t.Fatal("command criterion Passed = false, want true")
		}
		requireIssueID(t, verdict.BlockingIssues, "missing_tests")
		requireIssueID(t, verdict.BlockingIssues, "doc_gap")
		if !slices.Equal(verdict.BlockingIssueSignature, []string{"doc_gap", "missing_tests"}) {
			t.Fatalf("BlockingIssueSignature = %#v, want sorted blocker ids", verdict.BlockingIssueSignature)
		}
		if len(verdict.Payload) == 0 {
			t.Fatal("Verdict.Payload is empty, want aggregate evidence")
		}
		if verdict.Route.Action != RouteNextGeneration {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteNextGeneration)
		}
	})
}

func TestEvaluatorCommandExpectations(t *testing.T) {
	t.Parallel()

	t.Run("Should pass authored exit zero command expectation", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(_ context.Context, req CommandRequest) (CommandResult, error) {
				if got, want := req.Expect, commandExpectExitZero; got != want {
					t.Fatalf("CommandRequest.Expect = %q, want %q", got, want)
				}
				return CommandResult{ExitCode: 0}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(
			context.Background(),
			commandGate("cmd", "verify", "exit 0"),
			GateInput{Placement: PlacementInBody},
		)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
	})

	t.Run("Should pass exit_nonzero command expectation", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 2, Stderr: "lint failed"}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(
			context.Background(),
			commandGate("cmd", "verify", "exit_nonzero"),
			GateInput{Placement: PlacementInBody},
		)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
	})

	t.Run("Should pass stdout_contains command expectation", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(_ context.Context, req CommandRequest) (CommandResult, error) {
				if req.Contains != "ready" {
					t.Fatalf("CommandRequest.Contains = %q, want ready", req.Contains)
				}
				return CommandResult{ExitCode: 0, Stdout: "system ready"}, nil
			},
		)))
		gate := commandGate("cmd", "verify", "stdout_contains")
		gate.Criteria[0].Contains = "ready"
		verdict, err := evaluator.Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
	})

	t.Run("Should reject stdout_contains command expectation without contains", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0, Stdout: "system ready"}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(
			context.Background(),
			commandGate("cmd", "verify", "stdout_contains"),
			GateInput{Placement: PlacementInBody},
		)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		if got, want := verdict.Criteria[0].Warnings[0].Code, "command_contains_missing"; got != want {
			t.Fatalf("warning code = %q, want %q", got, want)
		}
	})

	t.Run("Should reject unsupported command expectation", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(
			context.Background(),
			commandGate("cmd", "verify", "unsupported"),
			GateInput{Placement: PlacementInBody},
		)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		requireIssueID(t, verdict.BlockingIssues, blockerCommandExpectation)
	})
}

func TestEvaluatorFailOpenStreakRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should fail open until broken judge streak reaches limit", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(
			WithBrokenJudgeStreakLimit(3),
			WithJudgeRunner(judgeRunnerFunc(func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{}, errors.New("transport down")
			})),
		)
		gate := Gate{
			ID:            "dod",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{{
				ID:     "judge",
				Type:   dsl.CriterionAgentJudge,
				Rubric: "Check",
			}},
		}

		inBody, err := evaluator.Evaluate(context.Background(), gate, GateInput{
			Placement:         PlacementInBody,
			Contract:          new(validContract()),
			BrokenJudgeStreak: 0,
		})
		if err != nil {
			t.Fatalf("Evaluate() in-body error = %v", err)
		}
		if inBody.Route.Action != RouteContinue {
			t.Fatalf("in-body Route.Action = %q, want %q", inBody.Route.Action, RouteContinue)
		}

		first, err := evaluator.Evaluate(context.Background(), gate, GateInput{
			Placement:         PlacementDefinitionOfDone,
			Contract:          new(validContract()),
			BrokenJudgeStreak: 0,
		})
		if err != nil {
			t.Fatalf("Evaluate() first error = %v", err)
		}
		if first.Route.Action != RouteNextGeneration {
			t.Fatalf("first Route.Action = %q, want %q", first.Route.Action, RouteNextGeneration)
		}
		if first.NextBrokenJudgeStreak != 1 {
			t.Fatalf("first NextBrokenJudgeStreak = %d, want 1", first.NextBrokenJudgeStreak)
		}

		third, err := evaluator.Evaluate(context.Background(), gate, GateInput{
			Placement:         PlacementDefinitionOfDone,
			Contract:          new(validContract()),
			BrokenJudgeStreak: 2,
		})
		if err != nil {
			t.Fatalf("Evaluate() third error = %v", err)
		}
		if third.Route.Action != RouteHalt {
			t.Fatalf("third Route.Action = %q, want %q", third.Route.Action, RouteHalt)
		}
		if third.Route.TerminalStatus != terminalStalled {
			t.Fatalf("third TerminalStatus = %q, want %q", third.Route.TerminalStatus, terminalStalled)
		}
	})

	t.Run("Should fail open before max revisions exhaustion for broken judge transport", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(
			WithJudgeRunner(judgeRunnerFunc(func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{}, errors.New("transport down")
			})),
		)
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "dod",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			MaxRevisions:  1,
			Criteria: []dsl.GateCriterion{{
				ID:     "judge",
				Type:   dsl.CriterionAgentJudge,
				Rubric: "Check",
			}},
		}, GateInput{
			Placement:         PlacementDefinitionOfDone,
			Contract:          new(validContract()),
			Revision:          1,
			BrokenJudgeStreak: 0,
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteNextGeneration {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteNextGeneration)
		}
		if verdict.Route.TerminalStatus != "" {
			t.Fatalf("TerminalStatus = %q, want empty", verdict.Route.TerminalStatus)
		}
	})
}

func TestEvaluatorBrokenJudgeWithRealFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should keep non-broken rejection retryable when judge transport fails", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(
			WithCommandRunner(commandRunnerFunc(func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1}, nil
			})),
			WithJudgeRunner(judgeRunnerFunc(func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{}, errors.New("judge transport down")
			})),
		)
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "mixed_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{
				{ID: "cmd", Type: dsl.CriterionCommand, Check: "verify"},
				{ID: "judge", Type: dsl.CriterionAgentJudge, Rubric: "Check"},
			},
		}, GateInput{Placement: PlacementDefinitionOfDone, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		if verdict.Broken {
			t.Fatal("Verdict.Broken = true, want false when a real non-broken failure exists")
		}
		if verdict.Route.Action != RouteNextGeneration {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteNextGeneration)
		}
	})
}

func TestEvaluatorHumanDecisionRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should route request_changes to revision", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator().Evaluate(context.Background(), humanGate(), GateInput{
			Placement: PlacementInBody,
			HumanDecisions: map[string]HumanDecision{
				"owner_approval": {
					Decision: HumanDecisionRequestChanges,
					Actor:    validHumanActor(t),
					Note:     "missing acceptance test",
				},
			},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		requireIssueID(t, verdict.BlockingIssues, blockerHumanRequestedChanges)
		if verdict.Route.Action != RouteRevise {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteRevise)
		}
	})

	t.Run("Should route reject to terminal blocked", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator().Evaluate(context.Background(), humanGate(), GateInput{
			Placement: PlacementDefinitionOfDone,
			HumanDecisions: map[string]HumanDecision{
				"owner_approval": {
					Decision: HumanDecisionReject,
					Actor:    validHumanActor(t),
					Note:     "not acceptable",
				},
			},
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeBlocked)
		requireIssueID(t, verdict.BlockingIssues, blockerHumanRejected)
		if verdict.Route.TerminalStatus != terminalBlocked {
			t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, terminalBlocked)
		}
	})

	t.Run("Should route pending human decision to approval escalation", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator().Evaluate(context.Background(), humanGate(), GateInput{
			Placement: PlacementDefinitionOfDone,
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeAwaitingApproval)
		requireIssueID(t, verdict.BlockingIssues, blockerHumanPending)
		if verdict.Route.Action != RouteEscalate {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteEscalate)
		}
		if verdict.Route.TerminalStatus != statusNeedsApproval {
			t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, statusNeedsApproval)
		}
		if got, want := verdict.Warnings[0].Code, "human_decision_pending"; got != want {
			t.Fatalf("warning code = %q, want %q", got, want)
		}
	})

	t.Run("Should not route pending human decision through fail override", func(t *testing.T) {
		t.Parallel()

		gate := humanGate()
		gate.OnResult = map[string]any{"fail": "halt"}
		for _, placement := range []Placement{PlacementInBody, PlacementDefinitionOfDone} {
			t.Run("Should escalate "+string(placement), func(t *testing.T) {
				t.Parallel()

				verdict, err := NewEvaluator().Evaluate(context.Background(), gate, GateInput{Placement: placement})
				if err != nil {
					t.Fatalf("Evaluate() error = %v", err)
				}
				requireOutcome(t, verdict, VerdictOutcomeAwaitingApproval)
				if verdict.Route.Action != RouteEscalate {
					t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteEscalate)
				}
				if verdict.Route.TerminalStatus != statusNeedsApproval {
					t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, statusNeedsApproval)
				}
			})
		}
	})

	t.Run("Should clamp pending human approval override to park route", func(t *testing.T) {
		t.Parallel()

		gate := humanGate()
		gate.OnResult = map[string]any{"approval": "done"}
		verdict, err := NewEvaluator().Evaluate(
			context.Background(),
			gate,
			GateInput{Placement: PlacementDefinitionOfDone},
		)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeAwaitingApproval)
		if verdict.Route.Action != RouteEscalate {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteEscalate)
		}
		if verdict.Route.TerminalStatus != statusNeedsApproval {
			t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, statusNeedsApproval)
		}
	})

	t.Run("Should default approval escalate override to needs approval status", func(t *testing.T) {
		t.Parallel()

		gate := humanGate()
		gate.OnResult = map[string]any{"approval": "escalate"}
		verdict, err := NewEvaluator().Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteEscalate {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteEscalate)
		}
		if verdict.Route.TerminalStatus != statusNeedsApproval {
			t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, statusNeedsApproval)
		}
	})
}

func TestEvaluatorPlacementRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should route in-body and definition-of-done placements differently", func(t *testing.T) {
		t.Parallel()

		failingCommand := commandRunnerFunc(func(context.Context, CommandRequest) (CommandResult, error) {
			return CommandResult{ExitCode: 1}, nil
		})
		passingCommand := commandRunnerFunc(func(context.Context, CommandRequest) (CommandResult, error) {
			return CommandResult{ExitCode: 0}, nil
		})
		gate := Gate{
			ID:            "route_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:    "cmd",
				Type:  dsl.CriterionCommand,
				Check: "verify",
			}},
		}

		inBodyRejected, err := NewEvaluator(WithCommandRunner(failingCommand)).
			Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() in-body rejected error = %v", err)
		}
		if inBodyRejected.Route.Action != RouteRevise {
			t.Fatalf("in-body rejected Action = %q, want %q", inBodyRejected.Route.Action, RouteRevise)
		}

		dodRejected, err := NewEvaluator(WithCommandRunner(failingCommand)).
			Evaluate(context.Background(), gate, GateInput{Placement: PlacementDefinitionOfDone})
		if err != nil {
			t.Fatalf("Evaluate() DoD rejected error = %v", err)
		}
		if dodRejected.Route.Action != RouteNextGeneration {
			t.Fatalf("DoD rejected Action = %q, want %q", dodRejected.Route.Action, RouteNextGeneration)
		}

		dodApproved, err := NewEvaluator(WithCommandRunner(passingCommand)).
			Evaluate(context.Background(), gate, GateInput{Placement: PlacementDefinitionOfDone})
		if err != nil {
			t.Fatalf("Evaluate() DoD approved error = %v", err)
		}
		if dodApproved.Route.Action != RouteDone {
			t.Fatalf("DoD approved Action = %q, want %q", dodApproved.Route.Action, RouteDone)
		}
	})
}

func TestEvaluatorRouteOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should route an in-body rejection to the next generation", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("cmd", "verify", "")
		gate.OnResult = map[string]any{"fail": "next_generation"}
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1}, nil
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteNextGeneration {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteNextGeneration)
		}
	})

	t.Run("Should map DoD continue override to done", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("cmd", "verify", "")
		gate.OnResult = map[string]any{"pass": "continue"}
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0}, nil
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementDefinitionOfDone})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteDone {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteDone)
		}
	})

	t.Run("Should route in-body failed override to branch", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("cmd", "verify", "")
		gate.OnResult = map[string]any{
			"fail": map[string]any{"route": "branch", "branch": "repair", "reason_code": "custom_repair"},
		}
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1}, nil
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteBranch {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteBranch)
		}
		if verdict.Route.Branch != "repair" {
			t.Fatalf("Route.Branch = %q, want repair", verdict.Route.Branch)
		}
		if verdict.Route.ReasonCode != "custom_repair" {
			t.Fatalf("Route.ReasonCode = %q, want custom_repair", verdict.Route.ReasonCode)
		}
	})

	t.Run("Should ignore invalid override and use default route", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("cmd", "verify", "")
		gate.OnResult = map[string]any{"fail": "done"}
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1}, nil
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteRevise {
			t.Fatalf("Route.Action = %q, want default %q", verdict.Route.Action, RouteRevise)
		}
	})

	t.Run("Should not route execution error through fail override", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("cmd", "verify", "")
		gate.OnResult = map[string]any{"fail": "revise"}
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{}, errors.New("runner unavailable")
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeError)
		if verdict.Route.Action != RouteHalt {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteHalt)
		}
	})
}

func TestEvaluatorMaxRevisions(t *testing.T) {
	t.Parallel()

	t.Run("Should route exhausted revisions to terminal halt", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "max_revisions_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			MaxRevisions:  2,
			Criteria: []dsl.GateCriterion{{
				ID:    "cmd",
				Type:  dsl.CriterionCommand,
				Check: "verify",
			}},
		}, GateInput{Placement: PlacementDefinitionOfDone, Revision: 2})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Route.Action != RouteHalt {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteHalt)
		}
		if verdict.Route.TerminalStatus != terminalExhausted {
			t.Fatalf("TerminalStatus = %q, want %q", verdict.Route.TerminalStatus, terminalExhausted)
		}
	})

	t.Run("Should reject max revisions above the ceiling", func(t *testing.T) {
		t.Parallel()

		_, err := NewEvaluator().Evaluate(context.Background(), Gate{
			ID:            "invalid_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			MaxRevisions:  dsl.GateMaxRevisionsCeiling + 1,
			Criteria: []dsl.GateCriterion{{
				ID:    "cmd",
				Type:  dsl.CriterionCommand,
				Check: "verify",
			}},
		}, GateInput{Placement: PlacementInBody})
		if !errors.Is(err, ErrMaxRevisionsCeiling) {
			t.Fatalf("Evaluate() error = %v, want ErrMaxRevisionsCeiling", err)
		}
	})
}

func TestEvaluatorExtensionOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Should map extension exit code output to rejection", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithToolCaller(toolCallerFunc(
			func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
				return tools.ToolResult{Structured: []byte(`{"exit_code":2,"stdout":"out","stderr":"err"}`)}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "extension_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:    "external_check",
				Type:  dsl.CriterionExtension,
				Check: "ext__quality__gate",
			}},
		}, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		if verdict.Criteria[0].ExitCode == nil || *verdict.Criteria[0].ExitCode != 2 {
			t.Fatalf("ExitCode = %#v, want 2", verdict.Criteria[0].ExitCode)
		}
		requireIssueID(t, verdict.BlockingIssues, blockerExtensionFailed)
	})

	t.Run("Should map extension missing structured output to invalid output", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithToolCaller(toolCallerFunc(
			func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
				return tools.ToolResult{}, nil
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), extensionGate(), GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeInvalidOutput)
		requireIssueID(t, verdict.BlockingIssues, blockerExtensionInvalid)
	})

	t.Run("Should map extension tool call failure to error", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithToolCaller(toolCallerFunc(
			func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
				return tools.ToolResult{}, errors.New("tool unavailable")
			},
		)))
		verdict, err := evaluator.Evaluate(context.Background(), extensionGate(), GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeError)
		requireIssueID(t, verdict.BlockingIssues, blockerExtensionFailed)
	})
}

func TestEvaluatorMetricScoreContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should emit a finite command metric score from exact stdout JSON", func(t *testing.T) {
		t.Parallel()

		gate := commandGate("score", "verify", "exit_zero")
		gate.Criteria[0].Metric = metricSpec(dsl.MetricMaximize, nil)
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0, Stdout: `{"score":0.72}`}, nil
			},
		))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Criteria[0].Score == nil || *verdict.Criteria[0].Score != 0.72 {
			t.Fatalf("Criterion.Score = %#v, want 0.72", verdict.Criteria[0].Score)
		}
	})

	t.Run("Should reject command metric output without a numeric score", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			stdout string
		}{
			{name: "Should reject missing metric score", stdout: `{}`},
			{name: "Should reject non-numeric metric score", stdout: `{"score":"high"}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gate := commandGate("score", "verify", "exit_zero")
				gate.Criteria[0].Metric = metricSpec(dsl.MetricMaximize, nil)
				verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
					func(context.Context, CommandRequest) (CommandResult, error) {
						return CommandResult{ExitCode: 0, Stdout: tc.stdout}, nil
					},
				))).Evaluate(context.Background(), gate, GateInput{Placement: PlacementInBody})
				if err != nil {
					t.Fatalf("Evaluate() error = %v", err)
				}
				requireOutcome(t, verdict, VerdictOutcomeInvalidOutput)
				requireIssueID(t, verdict.BlockingIssues, "metric_score_invalid")
			})
		}
	})

	t.Run("Should report a failed command expectation before parsing metric output", func(t *testing.T) {
		t.Parallel()

		runtimeGate := commandGate("score", "verify", "exit_zero")
		runtimeGate.Criteria[0].Metric = metricSpec(dsl.MetricMaximize, nil)
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 1, Stdout: "not-json"}, nil
			},
		))).Evaluate(t.Context(), runtimeGate, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireIssueID(t, verdict.BlockingIssues, blockerCommandExpectation)
		if slices.ContainsFunc(verdict.BlockingIssues, func(issue BlockingIssue) bool {
			return issue.ID == MetricScoreInvalidIssueID
		}) {
			t.Fatalf("BlockingIssues = %#v, metric parsing must not mask command failure", verdict.BlockingIssues)
		}
	})

	t.Run("Should require and surface judge metric scores", func(t *testing.T) {
		t.Parallel()

		criterion := dsl.GateCriterion{
			ID: "judge", Type: dsl.CriterionAgentJudge, Rubric: "Check", Metric: metricSpec(dsl.MetricMaximize, nil),
		}
		verdict, err := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{Raw: `{"verdict":"pass","evidence":{"checked":true},"score":0.91}`}, nil
			},
		))).Evaluate(context.Background(), Gate{
			ID: "judge_gate", VerdictPolicy: dsl.VerdictPolicyFixedPasses, Criteria: []dsl.GateCriterion{criterion},
		}, GateInput{Placement: PlacementInBody, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Criteria[0].Score == nil || *verdict.Criteria[0].Score != 0.91 {
			t.Fatalf("Criterion.Score = %#v, want 0.91", verdict.Criteria[0].Score)
		}
	})

	t.Run("Should reject a judge metric response without a score", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{Raw: `{"verdict":"pass","evidence":{"checked":true}}`}, nil
			},
		))).Evaluate(context.Background(), Gate{
			ID: "judge_gate", VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:     "judge",
				Type:   dsl.CriterionAgentJudge,
				Rubric: "Check",
				Metric: metricSpec(dsl.MetricMaximize, nil),
			}},
		}, GateInput{Placement: PlacementInBody, Contract: new(validContract())})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeInvalidOutput)
		requireIssueID(t, verdict.BlockingIssues, "metric_score_invalid")
	})

	t.Run("Should require and surface extension metric scores", func(t *testing.T) {
		t.Parallel()

		verdict, err := NewEvaluator(WithToolCaller(toolCallerFunc(
			func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
				return tools.ToolResult{Structured: []byte(`{"verdict":"pass","score":0.83}`)}, nil
			},
		))).Evaluate(context.Background(), Gate{
			ID: "extension_gate", VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:     "extension",
				Type:   dsl.CriterionExtension,
				Tool:   "ext__quality__gate",
				Metric: metricSpec(dsl.MetricMaximize, nil),
			}},
		}, GateInput{Placement: PlacementInBody})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Criteria[0].Score == nil || *verdict.Criteria[0].Score != 0.83 {
			t.Fatalf("Criterion.Score = %#v, want 0.83", verdict.Criteria[0].Score)
		}
	})

	t.Run("Should leave non-metric criterion scores unset", func(t *testing.T) {
		t.Parallel()

		bestScore := 0.8
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0, Stdout: `{"score":0.72}`}, nil
			},
		))).Evaluate(context.Background(), commandGate("plain", "verify", "exit_zero"), GateInput{
			Placement: PlacementInBody,
			BestScore: &bestScore,
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if verdict.Criteria[0].Score != nil {
			t.Fatalf("Criterion.Score = %#v, want nil for non-metric criterion", verdict.Criteria[0].Score)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
	})

	t.Run("Should reject a metric score that does not improve the best baseline", func(t *testing.T) {
		t.Parallel()

		bestScore := 0.8
		runtimeGate := commandGate("score", "verify", "exit_zero")
		runtimeGate.Criteria[0].Metric = metricSpec(dsl.MetricMaximize, nil)
		verdict, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0, Stdout: `{"score":0.72}`}, nil
			},
		))).Evaluate(context.Background(), runtimeGate, GateInput{
			Placement: PlacementInBody,
			BestScore: &bestScore,
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeRejected)
		requireIssueID(t, verdict.BlockingIssues, "metric_regression")
		if got, want := verdict.Route.Action, RouteRevise; got != want {
			t.Fatalf("Route.Action = %q, want %q", got, want)
		}
	})
}

func TestBestUpdateForVerdict(t *testing.T) {
	t.Parallel()

	t.Run("Should apply directional strict metric improvements", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			direction dsl.MetricDirection
			current   float64
			candidate float64
			minDelta  float64
			want      bool
		}{
			{
				name:      "Should improve maximize score",
				direction: dsl.MetricMaximize,
				current:   0.8,
				candidate: 0.9,
				minDelta:  0,
				want:      true,
			},
			{
				name:      "Should improve minimize score",
				direction: dsl.MetricMinimize,
				current:   120,
				candidate: 100,
				minDelta:  10,
				want:      true,
			},
			{
				name:      "Should reject equal score",
				direction: dsl.MetricMaximize,
				current:   0.8,
				candidate: 0.8,
				minDelta:  0,
				want:      false,
			},
			{
				name:      "Should reject insufficient maximize delta",
				direction: dsl.MetricMaximize,
				current:   0.8,
				candidate: 0.85,
				minDelta:  0.1,
				want:      false,
			},
			{
				name:      "Should accept an exact authored decimal delta",
				direction: dsl.MetricMaximize,
				current:   0.2,
				candidate: 0.3,
				minDelta:  0.1,
				want:      true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				metric := metricSpec(tc.direction, new(tc.minDelta))
				gate := Gate{ID: "metric_gate", Criteria: []dsl.GateCriterion{{ID: "metric", Metric: metric}}}
				verdict := approvedMetricVerdict("metric", tc.candidate)
				update, err := BestUpdateForVerdict(
					gate,
					verdict,
					2,
					&BestUpdateIntent{Generation: 1, Score: tc.current},
				)
				if err != nil {
					t.Fatalf("BestUpdateForVerdict() error = %v", err)
				}
				if tc.want && (update == nil || update.Generation != 2 || update.Score != tc.candidate) {
					t.Fatalf("BestUpdateForVerdict() = %#v, want generation 2 score %v", update, tc.candidate)
				}
				if !tc.want && update != nil {
					t.Fatalf("BestUpdateForVerdict() = %#v, want nil", update)
				}
			})
		}
	})

	t.Run("Should reject an unsupported metric direction", func(t *testing.T) {
		t.Parallel()

		unsupported := dsl.MetricDirection("sideways")
		_, err := BestUpdateForVerdict(
			Gate{ID: "metric_gate", Criteria: []dsl.GateCriterion{{
				ID: "metric", Metric: metricSpec(unsupported, nil),
			}}},
			approvedMetricVerdict("metric", 0.9),
			2,
			&BestUpdateIntent{Generation: 1, Score: 0.8},
		)
		if err == nil || !strings.Contains(err.Error(), `unsupported metric direction "sideways"`) {
			t.Fatalf("BestUpdateForVerdict() error = %v, want unsupported direction", err)
		}
	})

	t.Run("Should only create a first best from an approved finite metric verdict", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			outcome VerdictOutcome
			score   float64
			wantErr bool
			want    bool
		}{
			{
				name:    "Should create first best from approved score",
				outcome: VerdictOutcomeApproved,
				score:   0.7,
				want:    true,
			},
			{
				name:    "Should not create first best from rejected score",
				outcome: VerdictOutcomeRejected,
				score:   0.7,
				want:    false,
			},
			{
				name:    "Should reject non-finite metric score",
				outcome: VerdictOutcomeApproved,
				score:   math.Inf(1),
				wantErr: true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gate := Gate{
					ID:       "metric_gate",
					Criteria: []dsl.GateCriterion{{ID: "metric", Metric: metricSpec(dsl.MetricMaximize, nil)}},
				}
				verdict := approvedMetricVerdict("metric", tc.score)
				verdict.Outcome = tc.outcome
				update, err := BestUpdateForVerdict(gate, verdict, 1, nil)
				if tc.wantErr && err == nil {
					t.Fatal("BestUpdateForVerdict() error = nil, want non-nil")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("BestUpdateForVerdict() error = %v", err)
				}
				if tc.want && update == nil {
					t.Fatal("BestUpdateForVerdict() = nil, want initial best")
				}
				if !tc.want && update != nil {
					t.Fatalf("BestUpdateForVerdict() = %#v, want nil", update)
				}
			})
		}
	})
}

func TestNewVerdictIntent(t *testing.T) {
	t.Parallel()

	t.Run("Should project one sanitized verdict record", func(t *testing.T) {
		t.Parallel()

		score := 0.9
		rank := 0
		claimSecret := "compozy_claim_sensitive_123456"
		oauthSecret := "authorization_code=oauth-code-sensitive-123456"
		diagnostics := claimSecret + " " + oauthSecret
		intent, err := NewVerdictIntent(" quality ", 0, Verdict{
			Outcome: VerdictOutcomeRejected,
			Criteria: []CriterionResult{{
				ID: "metric", Score: &score, Stderr: diagnostics,
			}},
			BlockingIssues: []BlockingIssue{{ID: "quality_gap", Note: diagnostics}},
		}, &rank)
		if err != nil {
			t.Fatalf("NewVerdictIntent() error = %v", err)
		}
		if intent.GateID != "quality" || intent.Score == nil || *intent.Score != score ||
			intent.RouteCauseRank == nil ||
			*intent.RouteCauseRank != rank {
			t.Fatalf("NewVerdictIntent() = %#v, want sanitized score and rank", intent)
		}
		rank++
		if *intent.RouteCauseRank != 0 {
			t.Fatalf(
				"NewVerdictIntent().RouteCauseRank = %d after caller mutation, want immutable 0",
				*intent.RouteCauseRank,
			)
		}
		if string(intent.BlockingIssues) == "" || string(intent.Criteria) == "" ||
			strings.Contains(string(intent.BlockingIssues), claimSecret) ||
			strings.Contains(string(intent.Criteria), claimSecret) ||
			strings.Contains(string(intent.BlockingIssues), oauthSecret) ||
			strings.Contains(string(intent.Criteria), oauthSecret) {
			t.Fatalf(
				"NewVerdictIntent() leaked or omitted diagnostics: blocking=%s criteria=%s",
				intent.BlockingIssues,
				intent.Criteria,
			)
		}
		var criteria []CriterionResult
		if err := json.Unmarshal(intent.Criteria, &criteria); err != nil {
			t.Fatalf("json.Unmarshal(criteria) error = %v", err)
		}
		if len(criteria) != 1 {
			t.Fatalf("sanitized criteria = %#v, want one criterion", criteria)
		}
	})

	t.Run("Should bound oversized sanitized diagnostics", func(t *testing.T) {
		t.Parallel()

		intent, err := NewVerdictIntent("quality", 0, Verdict{
			Outcome: VerdictOutcomeRejected,
			BlockingIssues: []BlockingIssue{{
				ID: "quality_gap", Note: strings.Repeat("detail ", maxVerdictDiagnosticBytes),
			}},
		}, nil)
		if err != nil {
			t.Fatalf("NewVerdictIntent() error = %v", err)
		}
		if len(intent.BlockingIssues) > maxVerdictDiagnosticBytes {
			t.Fatalf("len(BlockingIssues) = %d, want at most %d", len(intent.BlockingIssues), maxVerdictDiagnosticBytes)
		}
		if !json.Valid(intent.BlockingIssues) || !json.Valid(intent.Criteria) {
			t.Fatalf(
				"NewVerdictIntent() emitted invalid JSON: blocking=%s criteria=%s",
				intent.BlockingIssues,
				intent.Criteria,
			)
		}
	})

	t.Run("Should preserve diagnostic root shape, array fields, and exact keys on overflow", func(t *testing.T) {
		t.Parallel()

		items := make([]any, 0, maxVerdictDiagnosticBytes)
		for index := range maxVerdictDiagnosticBytes {
			items = append(items, map[string]any{"detail": index})
		}
		raw, err := sanitizeVerdictDiagnostics(map[string]any{"criteria": items})
		if err != nil {
			t.Fatalf("sanitizeVerdictDiagnostics() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(overflow diagnostics) error = %v", err)
		}
		value, ok := decoded["criteria"].([]any)
		if !ok || len(value) == 0 {
			t.Fatalf("overflow diagnostics = %#v, want array field and truncation value", decoded)
		}
		bounded, ok := boundDiagnosticValue(map[string]any{" criteria ": "value"}, 16).(map[string]any)
		if !ok || bounded[" criteria "] != "value" {
			t.Fatalf("boundDiagnosticValue() = %#v, want exact original map key", bounded)
		}
	})

	t.Run("Should classify every verdict intent validation failure", func(t *testing.T) {
		t.Parallel()

		nan := math.NaN()
		scoreOne := 0.5
		scoreTwo := 0.6
		negative := -1
		tests := []struct {
			name      string
			gateID    string
			itemIndex int
			verdict   Verdict
			rank      *int
		}{
			{name: "Should classify an empty gate id", itemIndex: 0},
			{name: "Should classify a negative item index", gateID: "quality", itemIndex: -1},
			{name: "Should classify a negative route rank", gateID: "quality", itemIndex: 0, rank: &negative},
			{
				name: "Should classify a non-finite score", gateID: "quality", itemIndex: 0,
				verdict: Verdict{Criteria: []CriterionResult{{Score: &nan}}},
			},
			{
				name: "Should classify multiple scores", gateID: "quality", itemIndex: 0,
				verdict: Verdict{Criteria: []CriterionResult{{Score: &scoreOne}, {Score: &scoreTwo}}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := NewVerdictIntent(tt.gateID, tt.itemIndex, tt.verdict, tt.rank)
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("NewVerdictIntent() error = %v, want ErrValidation", err)
				}
			})
		}
	})
}

func TestEvaluatorAgentJudgeRubricAndEvidence(t *testing.T) {
	t.Parallel()

	t.Run("Should append a materialized rubric to contract context", func(t *testing.T) {
		t.Parallel()

		rendered, err := RenderAgentJudgeRubric(dsl.GateCriterion{
			ID:     "judge",
			Type:   dsl.CriterionAgentJudge,
			Rubric: "Check summary against Ship the loop evaluator",
		}, validContract(), JudgeEvidence{})
		if err != nil {
			t.Fatalf("RenderAgentJudgeRubric() error = %v", err)
		}
		for _, want := range []string{
			"Goal: Ship the loop evaluator",
			"Definition of done: All gates route correctly",
			"Do not emit false done",
			"Stop when: verified",
			"Check summary against Ship the loop evaluator",
			"evidence",
		} {
			if !stringsContains(rendered, want) {
				t.Fatalf("rendered rubric missing %q:\n%s", want, rendered)
			}
		}
	})

	t.Run("Should require a finite score in every metric verdict", func(t *testing.T) {
		t.Parallel()

		rendered, err := RenderAgentJudgeRubric(dsl.GateCriterion{
			ID: "judge", Type: dsl.CriterionAgentJudge, Rubric: "Score the candidate",
			Metric: metricSpec(dsl.MetricMaximize, nil),
		}, validContract(), JudgeEvidence{})
		if err != nil {
			t.Fatalf("RenderAgentJudgeRubric() error = %v", err)
		}
		if !strings.Contains(rendered, "Every metric verdict must include a finite numeric score") {
			t.Fatalf("metric judge rubric = %q, want mandatory finite score instruction", rendered)
		}
	})

	t.Run("Should reject approved judge verdict without evidence without Broken", func(t *testing.T) {
		t.Parallel()

		result := ParseJudgeVerdict("judge", dsl.CriterionAgentJudge, `{"verdict":"approved","evidence":{}}`)
		if result.Outcome != VerdictOutcomeRejected {
			t.Fatalf("Outcome = %q, want %q", result.Outcome, VerdictOutcomeRejected)
		}
		if result.Broken {
			t.Fatal("Broken = true, want false")
		}
		requireIssueID(t, result.BlockingIssues, JudgeEvidenceRequiredIssueID)
	})

	t.Run("Should reject approved judge verdict with whitespace-only evidence object", func(t *testing.T) {
		t.Parallel()

		result := ParseJudgeVerdict("judge", dsl.CriterionAgentJudge, `{"verdict":"approved","evidence":{ }}`)
		if result.Outcome != VerdictOutcomeRejected {
			t.Fatalf("Outcome = %q, want %q", result.Outcome, VerdictOutcomeRejected)
		}
		if result.Broken {
			t.Fatal("Broken = true, want false")
		}
		requireIssueID(t, result.BlockingIssues, JudgeEvidenceRequiredIssueID)
	})

	t.Run("Should degrade out-of-contract judge verdict to rejected", func(t *testing.T) {
		t.Parallel()

		result := ParseJudgeVerdict(
			"judge",
			dsl.CriterionAgentJudge,
			`{"verdict":"blocked","blocking_issues":[{"id":"external_block","note":"blocked"}]}`,
		)
		if result.Outcome != VerdictOutcomeRejected {
			t.Fatalf("Outcome = %q, want %q", result.Outcome, VerdictOutcomeRejected)
		}
		if result.Broken {
			t.Fatal("Broken = true, want false")
		}
		requireIssueID(t, result.BlockingIssues, JudgeMalformedOutputIssueID)
	})

	t.Run("Should preserve reported zero and aggregate multiple judge token totals", func(t *testing.T) {
		t.Parallel()

		responses := []JudgeResponse{
			{
				Raw:        `{"verdict":"approved","evidence":{"criterion":"first"}}`,
				TokensUsed: 0, TokensReported: true,
			},
			{
				Raw:        `{"verdict":"approved","evidence":{"criterion":"second"}}`,
				TokensUsed: 7, TokensReported: true,
			},
		}
		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				response := responses[0]
				responses = responses[1:]
				return response, nil
			},
		)))
		var reports []int64
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID: "judge-usage", VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{
				{ID: "first", Type: dsl.CriterionAgentJudge, Rubric: "Check first"},
				{ID: "second", Type: dsl.CriterionAgentJudge, Rubric: "Check second"},
			},
		}, GateInput{
			Placement: PlacementInBody,
			Contract:  new(validContract()),
			JudgeUsageReporter: JudgeUsageReporterFunc(func(tokens int64) {
				reports = append(reports, tokens)
			}),
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if !slices.Equal(reports, []int64{7}) {
			t.Fatalf("judge usage reports = %#v, want aggregate 7", reports)
		}
	})

	t.Run("Should report a real zero for a zero-token judge", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithJudgeRunner(judgeRunnerFunc(
			func(context.Context, JudgeRequest) (JudgeResponse, error) {
				return JudgeResponse{
					Raw:            `{"verdict":"approved","evidence":{"criterion":"zero"}}`,
					TokensReported: true,
				}, nil
			},
		)))
		var reports []int64
		_, err := evaluator.Evaluate(context.Background(), Gate{
			ID: "judge-zero", VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID: "zero", Type: dsl.CriterionAgentJudge, Rubric: "Check zero",
			}},
		}, GateInput{
			Placement: PlacementInBody,
			Contract:  new(validContract()),
			JudgeUsageReporter: JudgeUsageReporterFunc(func(tokens int64) {
				reports = append(reports, tokens)
			}),
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if !slices.Equal(reports, []int64{0}) {
			t.Fatalf("judge usage reports = %#v, want reported zero", reports)
		}
	})
}

func TestEvaluatorAdaptersAndDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should clone gate node criteria and routes", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{
			ID:            "gate_node",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			MaxRevisions:  4,
			Criteria: []dsl.GateCriterion{{
				ID:    "cmd",
				Type:  dsl.CriterionCommand,
				Check: "verify",
				Inputs: map[string]any{
					"path":   "old",
					"nested": map[string]any{"mode": "strict"},
					"list":   []any{"alpha", map[string]any{"flag": "on"}},
				},
				Extra: map[string]any{
					"contains": "ok",
					"nested":   map[string]any{"kind": "stdout"},
				},
				Metric: metricSpec(dsl.MetricMaximize, new(0.1)),
			}},
			OnResult: map[string]any{
				"pass": map[string]any{"route": "branch", "branch": "ship"},
			},
		}
		gate := GateFromNode(node)
		node.Criteria[0].ID = "changed"
		node.Criteria[0].Inputs["path"] = "changed"
		node.Criteria[0].Inputs["nested"].(map[string]any)["mode"] = "changed"
		node.Criteria[0].Inputs["list"].([]any)[1].(map[string]any)["flag"] = "changed"
		node.Criteria[0].Extra["contains"] = "changed"
		node.Criteria[0].Extra["nested"].(map[string]any)["kind"] = "changed"
		*node.Criteria[0].Metric.MinDelta = 0.2
		node.OnResult["pass"].(map[string]any)["branch"] = "changed"
		if gate.ID != "gate_node" {
			t.Fatalf("Gate.ID = %q, want gate_node", gate.ID)
		}
		if gate.Criteria[0].ID != "cmd" {
			t.Fatalf("Gate.Criteria[0].ID = %q, want cmd", gate.Criteria[0].ID)
		}
		if gate.Criteria[0].Inputs["path"] != "old" {
			t.Fatalf("Gate.Criteria[0].Inputs[path] = %v, want old", gate.Criteria[0].Inputs["path"])
		}
		inputNested := gate.Criteria[0].Inputs["nested"].(map[string]any)
		if inputNested["mode"] != "strict" {
			t.Fatalf("Gate.Criteria[0].Inputs[nested][mode] = %v, want strict", inputNested["mode"])
		}
		inputList := gate.Criteria[0].Inputs["list"].([]any)
		inputListNested := inputList[1].(map[string]any)
		if inputListNested["flag"] != "on" {
			t.Fatalf("Gate.Criteria[0].Inputs[list][1][flag] = %v, want on", inputListNested["flag"])
		}
		if gate.Criteria[0].Extra["contains"] != "ok" {
			t.Fatalf("Gate.Criteria[0].Extra[contains] = %v, want ok", gate.Criteria[0].Extra["contains"])
		}
		extraNested := gate.Criteria[0].Extra["nested"].(map[string]any)
		if extraNested["kind"] != "stdout" {
			t.Fatalf("Gate.Criteria[0].Extra[nested][kind] = %v, want stdout", extraNested["kind"])
		}
		if gate.Criteria[0].Metric == nil || gate.Criteria[0].Metric.MinDelta == nil ||
			*gate.Criteria[0].Metric.MinDelta != 0.1 {
			t.Fatalf("Gate.Criteria[0].Metric = %#v, want independent metric declaration", gate.Criteria[0].Metric)
		}
		route := gate.OnResult["pass"].(map[string]any)
		if route["branch"] != "ship" {
			t.Fatalf("Gate.OnResult[pass].branch = %v, want ship", route["branch"])
		}
	})

	t.Run("Should build contract gate with revise policy", func(t *testing.T) {
		t.Parallel()

		contract := validContract()
		contract.Verification = []dsl.GateCriterion{{
			ID:     "judge",
			Type:   dsl.CriterionAgentJudge,
			Rubric: "Check",
		}}
		gate := GateFromContract("dod", contract, 5)
		if gate.ID != "dod" {
			t.Fatalf("Gate.ID = %q, want dod", gate.ID)
		}
		if gate.VerdictPolicy != dsl.VerdictPolicyReviseUntilClean {
			t.Fatalf("VerdictPolicy = %q, want revise_until_clean", gate.VerdictPolicy)
		}
		if gate.MaxRevisions != 5 {
			t.Fatalf("MaxRevisions = %d, want 5", gate.MaxRevisions)
		}
	})

	t.Run("Should build command-only contract gate with fixed passes policy", func(t *testing.T) {
		t.Parallel()

		contract := validContract()
		contract.Verification = []dsl.GateCriterion{{
			ID:    "cmd",
			Type:  dsl.CriterionCommand,
			Check: "verify",
		}}
		gate := GateFromContract("dod", contract, 5)
		if gate.VerdictPolicy != dsl.VerdictPolicyFixedPasses {
			t.Fatalf("VerdictPolicy = %q, want fixed_passes", gate.VerdictPolicy)
		}
	})

	t.Run("Should recover invalid evaluator option defaults", func(t *testing.T) {
		t.Parallel()

		evaluator := NewEvaluator(WithMaxRevisionsCeiling(-1), WithBrokenJudgeStreakLimit(-1))
		if evaluator.maxRevisionsCeiling != dsl.GateMaxRevisionsCeiling {
			t.Fatalf("maxRevisionsCeiling = %d, want %d", evaluator.maxRevisionsCeiling, dsl.GateMaxRevisionsCeiling)
		}
		if evaluator.brokenJudgeStreakLimit != DefaultBrokenJudgeStreakLimit {
			t.Fatalf(
				"brokenJudgeStreakLimit = %d, want %d",
				evaluator.brokenJudgeStreakLimit,
				DefaultBrokenJudgeStreakLimit,
			)
		}
	})
}

func TestEvaluatorVerdictPolicyValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject revise_until_clean without judge or human source", func(t *testing.T) {
		t.Parallel()

		_, err := NewEvaluator(WithCommandRunner(commandRunnerFunc(
			func(context.Context, CommandRequest) (CommandResult, error) {
				return CommandResult{ExitCode: 0}, nil
			},
		))).Evaluate(context.Background(), Gate{
			ID:            "policy_gate",
			VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
			Criteria: []dsl.GateCriterion{{
				ID:    "cmd",
				Type:  dsl.CriterionCommand,
				Check: "verify",
			}},
		}, GateInput{Placement: PlacementInBody})
		if !errors.Is(err, ErrVerdictPolicyRequiresJudge) {
			t.Fatalf("Evaluate() error = %v, want ErrVerdictPolicyRequiresJudge", err)
		}
	})
}

func commandGate(criterionID string, check string, expect string) Gate {
	return Gate{
		ID:            "command_gate",
		VerdictPolicy: dsl.VerdictPolicyFixedPasses,
		Criteria: []dsl.GateCriterion{{
			ID:     criterionID,
			Type:   dsl.CriterionCommand,
			Check:  check,
			Expect: expect,
		}},
	}
}

func humanGate() Gate {
	return Gate{
		ID:            "human_gate",
		VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
		Criteria: []dsl.GateCriterion{{
			ID:   "owner_approval",
			Type: dsl.CriterionHuman,
		}},
	}
}

func extensionGate() Gate {
	return Gate{
		ID:            "extension_gate",
		VerdictPolicy: dsl.VerdictPolicyFixedPasses,
		Criteria: []dsl.GateCriterion{{
			ID:   "external_check",
			Type: dsl.CriterionExtension,
			Tool: "ext__quality__gate",
		}},
	}
}

func metricSpec(direction dsl.MetricDirection, minDelta *float64) *dsl.MetricSpec {
	return &dsl.MetricSpec{Direction: direction, MinDelta: minDelta}
}

func approvedMetricVerdict(criterionID string, score float64) Verdict {
	return Verdict{
		Outcome: VerdictOutcomeApproved,
		Criteria: []CriterionResult{{
			ID: criterionID, Outcome: VerdictOutcomeApproved, Passed: true, Score: &score,
		}},
	}
}

func validContract() dsl.Contract {
	return dsl.Contract{
		Goal:             "Ship the loop evaluator",
		DefinitionOfDone: "All gates route correctly",
		Constraints:      []string{"Do not emit false done"},
		Boundaries:       []string{"No downstream coordinator rewrites"},
		StopWhen:         "verified",
	}
}

func validHumanActor(t *testing.T) task.ActorContext {
	t.Helper()

	actor, err := task.DeriveHumanActorContext("user-1", task.OriginKindCLI, "cli")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	return actor
}

func requireOutcome(t *testing.T, verdict Verdict, want VerdictOutcome) {
	t.Helper()

	if verdict.Outcome != want {
		t.Fatalf("Verdict.Outcome = %q, want %q; verdict = %#v", verdict.Outcome, want, verdict)
	}
}

func requireIssueID(t *testing.T, issues []BlockingIssue, want string) {
	t.Helper()

	for _, issue := range issues {
		if issue.ID == want {
			return
		}
	}
	t.Fatalf("issues = %#v, want id %q", issues, want)
}

func stringsContains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}
