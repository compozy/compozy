package acpmock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestLoadFixtureParsesMultipleAgentsAndScenarioPrimitives(t *testing.T) {
	t.Parallel()

	t.Run("Should maps named agents and bridge responses", func(t *testing.T) {
		t.Parallel()

		fixture, err := LoadFixture(filepath.Join("testdata", "multi_agent_fixture.json"))
		if err != nil {
			t.Fatalf("LoadFixture() error = %v", err)
		}

		if got, want := len(fixture.Agents), 2; got != want {
			t.Fatalf("len(fixture.Agents) = %d, want %d", got, want)
		}

		alpha, err := fixture.Agent("alpha")
		if err != nil {
			t.Fatalf("fixture.Agent(alpha) error = %v", err)
		}
		turn, err := alpha.SelectTurn("hello alpha", acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser})
		if err != nil {
			t.Fatalf("alpha.SelectTurn() error = %v", err)
		}
		if got, want := turn.Name, "alpha-hello"; got != want {
			t.Fatalf("turn.Name = %q, want %q", got, want)
		}
		if got, want := turn.Steps[0].Kind, StepKindAssistant; got != want {
			t.Fatalf("turn.Steps[0].Kind = %q, want %q", got, want)
		}
		if got, want := turn.Steps[1].Kind, StepKindBridgeContent; got != want {
			t.Fatalf("turn.Steps[1].Kind = %q, want %q", got, want)
		}
	})

	t.Run("Should maps permission and sandbox primitives", func(t *testing.T) {
		t.Parallel()

		fixture, err := LoadFixture(filepath.Join("testdata", "permission_env_fixture.json"))
		if err != nil {
			t.Fatalf("LoadFixture() error = %v", err)
		}

		approver, err := fixture.Agent("approver")
		if err != nil {
			t.Fatalf("fixture.Agent(approver) error = %v", err)
		}
		permissionTurn, err := approver.SelectTurn(
			"request permission",
			acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		)
		if err != nil {
			t.Fatalf("approver.SelectTurn() error = %v", err)
		}
		if got, want := permissionTurn.Steps[0].Kind, StepKindPermission; got != want {
			t.Fatalf("permission step kind = %q, want %q", got, want)
		}
		if got, want := permissionTurn.Steps[0].ExpectDecision, "allow-always"; got != want {
			t.Fatalf("permission ExpectDecision = %q, want %q", got, want)
		}

		runner, err := fixture.Agent("runner")
		if err != nil {
			t.Fatalf("fixture.Agent(runner) error = %v", err)
		}
		sandboxTurn, err := runner.SelectTurn(
			"run sandbox",
			acp.PromptMeta{TurnSource: acp.PromptTurnSourceNetwork},
		)
		if err != nil {
			t.Fatalf("runner.SelectTurn() error = %v", err)
		}
		if got, want := sandboxTurn.Steps[0].Kind, StepKindSandbox; got != want {
			t.Fatalf("sandbox step kind = %q, want %q", got, want)
		}
		if got, want := sandboxTurn.Steps[0].Command, "agh"; got != want {
			t.Fatalf("sandbox step command = %q, want %q", got, want)
		}
	})

	t.Run("Should select a judge turn by exact structured metadata", func(t *testing.T) {
		t.Parallel()

		fixture, err := ParseFixture(
			[]byte(
				`{"version":2,"agents":[{"name":"judge","provider":"claude","turns":[` +
					`{"name":"turn-two","match":{"turn_source":"user","judge":{"role":"agent-judge","attempt":2}},` +
					`"steps":[{"kind":"assistant","text":"revise"}]}]}]}`,
			),
		)
		if err != nil {
			t.Fatalf("ParseFixture(judge metadata) error = %v", err)
		}
		judge, err := fixture.Agent("judge")
		if err != nil {
			t.Fatalf("fixture.Agent(judge) error = %v", err)
		}
		turn, err := judge.SelectTurn(
			"rendered rubric is not routing authority",
			acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser, Judge: &acp.PromptJudgeMeta{
				Role: acp.PromptJudgeRoleAgent, Attempt: 2, CorrelationID: "goal-judge:stable",
				GateID: "goal:goal-judge", CriterionID: "review",
			}},
		)
		if err != nil {
			t.Fatalf("judge.SelectTurn(judge metadata) error = %v", err)
		}
		if got, want := turn.Name, "turn-two"; got != want {
			t.Fatalf("turn.Name = %q, want %q", got, want)
		}
		if _, err := judge.SelectTurn(
			"same rendered rubric",
			acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser, Judge: &acp.PromptJudgeMeta{
				Role: acp.PromptJudgeRoleAgent, Attempt: 3, CorrelationID: "goal-judge:stable",
				GateID: "goal:goal-judge", CriterionID: "review",
			}},
		); err == nil || !strings.Contains(err.Error(), "no turn matched") {
			t.Fatalf("judge.SelectTurn(non-matching attempt) error = %v, want no-match error", err)
		}
	})

	t.Run("Should select a Goal turn by exact structured metadata", func(t *testing.T) {
		t.Parallel()

		fixture, err := ParseFixture([]byte(
			`{"version":2,"agents":[{"name":"judge","provider":"claude","turns":[` +
				`{"name":"second-work","match":{"turn_source":"synthetic","goal":{"kind":"goal-work","turn":2}},` +
				`"steps":[{"kind":"assistant","text":"pass"}]}]}]}`,
		))
		if err != nil {
			t.Fatalf("ParseFixture(Goal metadata) error = %v", err)
		}
		judge, err := fixture.Agent("judge")
		if err != nil {
			t.Fatalf("fixture.Agent(judge) error = %v", err)
		}
		turnValue := 2
		turn, err := judge.SelectTurn(
			"work prompt",
			acp.PromptMeta{TurnSource: acp.PromptTurnSourceSynthetic, Synthetic: &acp.PromptSyntheticMeta{
				Reason: "goal_work",
				Goal: &acp.GoalPromptMeta{
					Kind: acp.GoalPromptKindWork, RunID: "run-1", NodeID: "goal", Generation: 1,
					ItemIndex: 0, Turn: &turnValue, PromptAttempt: 0, PromptID: "prompt-2",
				},
			}},
		)
		if err != nil {
			t.Fatalf("judge.SelectTurn(Goal metadata) error = %v", err)
		}
		if turn.Name != "second-work" {
			t.Fatalf("turn.Name = %q, want second-work", turn.Name)
		}
	})
}

func TestRegisterRendersValidatedAgentDefinition(t *testing.T) {
	t.Parallel()

	homePaths := mockHomePaths(t)
	diagnosticsPath := filepath.Join(t.TempDir(), "diag", "alpha.jsonl")

	registration, err := Register(homePaths, RegisterOptions{
		FixturePath:     filepath.Join("testdata", "multi_agent_fixture.json"),
		FixtureAgent:    "alpha",
		AgentName:       " mock-alpha ",
		DriverPath:      "/tmp/mock driver/acpmock-driver",
		DiagnosticsPath: diagnosticsPath,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got, want := registration.AgentName, "mock-alpha"; got != want {
		t.Fatalf("registration.AgentName = %q, want %q", got, want)
	}
	if got, want := registration.AgentDefPath, filepath.Join(
		homePaths.AgentsDir,
		"mock-alpha",
		"AGENT.md",
	); got != want {
		t.Fatalf("registration.AgentDefPath = %q, want %q", got, want)
	}

	loaded, err := aghconfig.LoadAgentDefFile(registration.AgentDefPath)
	if err != nil {
		t.Fatalf("LoadAgentDefFile(%q) error = %v", registration.AgentDefPath, err)
	}
	if got, want := loaded.Name, "mock-alpha"; got != want {
		t.Fatalf("loaded.Name = %q, want %q", got, want)
	}
	if got, want := loaded.Provider, "claude"; got != want {
		t.Fatalf("loaded.Provider = %q, want %q", got, want)
	}
	if got, want := loaded.Command, registration.Command; got != want {
		t.Fatalf("loaded.Command = %q, want %q", got, want)
	}

	cfg := aghconfig.DefaultWithHome(homePaths)
	resolved, err := cfg.ResolveAgent(loaded)
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}
	if got, want := resolved.Provider, "claude"; got != want {
		t.Fatalf("resolved.Provider = %q, want %q", got, want)
	}
	if got, want := resolved.Command, registration.Command; got != want {
		t.Fatalf("resolved.Command = %q, want %q", got, want)
	}
}

func TestReadDiagnosticsParsesJSONLines(t *testing.T) {
	t.Parallel()

	t.Run("Should decode AGH session ownership alongside ACP diagnostics", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "diag.jsonl")
		want := []DiagnosticsRecord{
			{
				AGHSessionID: "agh-session-a",
				AgentName:    "alpha",
				SessionID:    "acp-session-1",
				PromptIndex:  1,
				Prompt:       "hello",
				PromptMeta:   acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
				TurnName:     "alpha-hello",
				Match: TurnMatch{
					TurnSource: acp.PromptTurnSourceUser,
					UserText:   "hello",
				},
			},
			{
				AGHSessionID: "agh-session-b",
				AgentName:    "beta",
				SessionID:    "acp-session-1",
				PromptIndex:  2,
				Prompt:       "hello beta",
				PromptMeta: acp.PromptMeta{
					TurnSource: acp.PromptTurnSourceNetwork,
					Network: &acp.PromptNetworkMeta{
						MessageID:   "msg-2",
						Kind:        "say",
						Surface:     "direct",
						DirectID:    "direct_0123456789abcdef0123456789abcdef",
						WorkID:      "work_patch_42",
						ReplyTo:     "msg-root",
						TraceID:     "trace_ops_patch_42",
						CausationID: "msg-root",
						Trust:       "untrusted",
					},
				},
				TurnName: "beta-hello",
				Match: TurnMatch{
					TurnSource: acp.PromptTurnSourceNetwork,
					Network: &TurnMatchNetwork{
						MessageID:   "msg-2",
						Kind:        "say",
						Surface:     "direct",
						DirectID:    "direct_0123456789abcdef0123456789abcdef",
						WorkID:      "work_patch_42",
						ReplyTo:     "msg-root",
						TraceID:     "trace_ops_patch_42",
						CausationID: "msg-root",
						Trust:       "untrusted",
					},
				},
			},
			{
				AGHSessionID:      "agh-session-a",
				AgentName:         "gamma",
				SessionID:         "acp-session-1",
				ProtocolMethod:    "session/set_config_option",
				ConfigOptionID:    "reasoning_effort",
				ConfigOptionValue: "max",
			},
		}

		encoded := []byte("\n")
		for index, record := range want {
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatalf("json.Marshal(record %d) error = %v", index, err)
			}
			encoded = append(encoded, data...)
			encoded = append(encoded, '\n')
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", path, err)
		}

		got, err := ReadDiagnostics(path)
		if err != nil {
			t.Fatalf("ReadDiagnostics() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ReadDiagnostics() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should select one AGH session in append order before protocol projection", func(t *testing.T) {
		t.Parallel()

		records := []DiagnosticsRecord{
			{AGHSessionID: "agh-session-a", SessionID: "acp-session-1", ProtocolMethod: "model"},
			{AGHSessionID: "agh-session-b", SessionID: "acp-session-1", ProtocolMethod: "foreign"},
			{AGHSessionID: " agh-session-a ", SessionID: "acp-session-1", ProtocolMethod: "prompt"},
			{AGHSessionID: "agh-session-a", SessionID: "acp-session-1", PromptIndex: 1},
			{
				AGHSessionID:   "agh-session-a",
				SessionID:      "acp-session-1",
				LifecycleEvent: "session_new",
				PromptIndex:    1,
			},
		}
		owned := DiagnosticsForAGHSession(records, " agh-session-a ")
		if got, want := len(owned), 4; got != want {
			t.Fatalf("DiagnosticsForAGHSession() = %#v, want four records", owned)
		}
		if owned[0].ProtocolMethod != "model" || owned[1].ProtocolMethod != "prompt" {
			t.Fatalf("DiagnosticsForAGHSession() = %#v, want model then prompt", owned)
		}
		protocol := ProtocolDiagnostics(owned)
		if !reflect.DeepEqual(protocol, owned[:2]) {
			t.Fatalf("ProtocolDiagnostics(owned) = %#v, want %#v", protocol, owned[:2])
		}
		prompt := PromptDiagnostics(owned)
		if len(prompt) != 1 || prompt[0].PromptIndex != 1 || prompt[0].LifecycleEvent != "" {
			t.Fatalf("PromptDiagnostics(owned) = %#v, want one prompt record", prompt)
		}
	})

	t.Run("Should fail closed for empty or unknown AGH session owners", func(t *testing.T) {
		t.Parallel()

		records := []DiagnosticsRecord{
			{AGHSessionID: "", SessionID: "agh-session-a"},
			{AGHSessionID: "agh-session-a", SessionID: "acp-session-1"},
		}
		tests := []struct {
			name  string
			owner string
		}{
			{name: "Should return an allocated empty result for an empty owner"},
			{name: "Should return an allocated empty result for a whitespace owner", owner: "   "},
			{name: "Should return an allocated empty result for an unknown owner", owner: "agh-session-missing"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := DiagnosticsForAGHSession(records, tt.owner)
				if got == nil {
					t.Fatal("DiagnosticsForAGHSession() = nil, want allocated empty result")
				}
				if len(got) != 0 {
					t.Fatalf("DiagnosticsForAGHSession() = %#v, want no records", got)
				}
			})
		}
	})

	t.Run("Should reject a diagnostics record above the scanner limit", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "oversized.jsonl")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 10*1024*1024+1)), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", path, err)
		}
		if _, err := ReadDiagnostics(path); err == nil || !strings.Contains(err.Error(), "scan diagnostics") {
			t.Fatalf("ReadDiagnostics(oversized) error = %v, want scanner limit error", err)
		}
	})
}

func TestLoadFixtureAndParseFixtureValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should load fixture requires path", func(t *testing.T) {
		t.Parallel()

		if _, err := LoadFixture(" "); err == nil || !strings.Contains(err.Error(), "fixture path is required") {
			t.Fatalf("LoadFixture(empty) error = %v, want required-path error", err)
		}
	})

	t.Run("Should load fixture reports read error", func(t *testing.T) {
		t.Parallel()

		if _, err := LoadFixture(filepath.Join(t.TempDir(), "missing.json")); err == nil ||
			!strings.Contains(err.Error(), "read fixture") {
			t.Fatalf("LoadFixture(missing) error = %v, want read error", err)
		}
	})

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "Should reject invalid version",
			raw:  `{"version":1,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
			want: "fixture version 1",
		},
		{
			name: "Should reject duplicate agent",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"assistant","text":"hi"}]}]},{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hello"},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
			want: "duplicate agent",
		},
		{
			name: "Should reject legacy matcher fields",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"equals":"a"},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
			want: "unknown field",
		},
		{
			name: "Should reject invalid stop reason",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"stop_reason":"bad","steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
			want: "stop_reason",
		},
		{
			name: "Should reject negative scripted usage",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"usage":{"input_tokens":-1,"output_tokens":2},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
			want: "input_tokens must be >= 0",
		},
		{
			name: "Should reject scripted usage whose total exceeds int capacity",
			raw: fmt.Sprintf(
				`{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"usage":{"input_tokens":%d,"output_tokens":1},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
				math.MaxInt,
			),
			want: "total tokens exceed int capacity",
		},
		{
			name: "Should reject invalid permission decision",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"permission","tool_call_id":"perm-1","tool_kind":"edit","expect_decision":"maybe"}]}]}]}`,
			want: "expect_decision",
		},
		{
			name: "Should reject sandbox cwd that is not absolute",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"sandbox_exec","command":"agh","cwd":"relative"}]}]}]}`,
			want: "cwd must be absolute",
		},
		{
			name: "Should reject driver control without payload",
			raw:  `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"driver_control"}]}]}]}`,
			want: "driver_control is required",
		},
		{
			name: "Should reject driver delay that exceeds duration capacity",
			raw: fmt.Sprintf(
				`{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"driver_control","driver_control":{"action":"delay","delay_ms":%d}}]}]}]}`,
				math.MaxInt64/int64(time.Millisecond)+1,
			),
			want: "delay_ms exceeds duration capacity",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseFixture([]byte(tc.raw)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseFixture(%s) error = %v, want substring %q", tc.name, err, tc.want)
			}
		})
	}
}

func TestParseFixtureAcceptsScriptedUsage(t *testing.T) {
	t.Parallel()

	t.Run("Should accept scripted input and output token usage", func(t *testing.T) {
		t.Parallel()

		fixture, err := ParseFixture([]byte(
			`{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{` +
				`"match":{"turn_source":"user","user_text":"hi"},` +
				`"usage":{"input_tokens":13,"output_tokens":5},` +
				`"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
		))
		if err != nil {
			t.Fatalf("ParseFixture(scripted usage) error = %v", err)
		}
		agent, err := fixture.Agent("alpha")
		if err != nil {
			t.Fatalf("Fixture.Agent(alpha) error = %v", err)
		}
		if agent.Turns[0].Usage == nil || agent.Turns[0].Usage.InputTokens != 13 ||
			agent.Turns[0].Usage.OutputTokens != 5 {
			t.Fatalf("scripted usage = %#v, want input=13 output=5", agent.Turns[0].Usage)
		}
	})
}

func TestParseFixtureAcceptsACPStopReasonVocabulary(t *testing.T) {
	t.Parallel()

	reasons := []acp.PromptStopReason{
		acp.PromptStopReasonEndTurn,
		acp.PromptStopReasonMaxTokens,
		acp.PromptStopReasonMaxTurnRequests,
		acp.PromptStopReasonRefusal,
		acp.PromptStopReasonCancelled,
	}
	for _, reason := range reasons {
		t.Run("Should accept "+string(reason), func(t *testing.T) {
			t.Parallel()

			raw := `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{` +
				`"match":{"turn_source":"user","user_text":"hi"},"stop_reason":"` + string(reason) +
				`","steps":[{"kind":"assistant","text":"hi"}]}]}]}`
			if _, err := ParseFixture([]byte(raw)); err != nil {
				t.Fatalf("ParseFixture(stop_reason=%q) error = %v", reason, err)
			}
		})
	}
}

func TestParseFixtureAcceptsTurnUsage(t *testing.T) {
	t.Parallel()

	t.Run("Should parse deterministic prompt-response token usage on a turn", func(t *testing.T) {
		t.Parallel()

		raw := `{"version":2,"agents":[{"name":"alpha","provider":"claude","turns":[{` +
			`"match":{"turn_source":"user","user_text":"hi"},"stop_reason":"end_turn",` +
			`"steps":[{"kind":"assistant","text":"hi"}],` +
			`"usage":{"input_tokens":1200,"output_tokens":340,"total_tokens":1540}}]}]}`
		fixture, err := ParseFixture([]byte(raw))
		if err != nil {
			t.Fatalf("ParseFixture(usage) error = %v", err)
		}
		alpha, err := fixture.Agent("alpha")
		if err != nil {
			t.Fatalf("fixture.Agent(alpha) error = %v", err)
		}
		usage := alpha.Turns[0].Usage
		if usage == nil {
			t.Fatalf("Turns[0].Usage = nil, want parsed prompt-response usage")
		}
		if usage.InputTokens != 1200 || usage.OutputTokens != 340 ||
			usage.TotalTokens == nil || *usage.TotalTokens != 1540 {
			t.Fatalf("Turns[0].Usage = %+v, want {InputTokens:1200 OutputTokens:340 TotalTokens:1540}", *usage)
		}
	})
}

func TestFixtureLookupAndHelperErrors(t *testing.T) {
	t.Parallel()

	fixture, err := LoadFixture(filepath.Join("testdata", "multi_agent_fixture.json"))
	if err != nil {
		t.Fatalf("LoadFixture() error = %v", err)
	}

	if _, err := fixture.Agent(""); err == nil || !strings.Contains(err.Error(), "fixture agent name is required") {
		t.Fatalf("fixture.Agent(empty) error = %v, want required-name error", err)
	}
	if _, err := fixture.Agent("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("fixture.Agent(missing) error = %v, want not-found error", err)
	}
	trimmedFixture, err := ParseFixture(
		[]byte(
			`{"version":2,"agents":[{"name":" alpha ","provider":"claude","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`,
		),
	)
	if err != nil {
		t.Fatalf("ParseFixture(trimmed name) error = %v", err)
	}
	trimmedAgent, err := trimmedFixture.Agent("alpha")
	if err != nil {
		t.Fatalf("trimmedFixture.Agent(alpha) error = %v", err)
	}
	if got, want := trimmedAgent.Name, "alpha"; got != want {
		t.Fatalf("trimmedAgent.Name = %q, want %q", got, want)
	}
	alpha, err := fixture.Agent("alpha")
	if err != nil {
		t.Fatalf("fixture.Agent(alpha) error = %v", err)
	}
	if _, err := alpha.SelectTurn(
		"different prompt",
		acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	); err == nil ||
		!strings.Contains(err.Error(), "no turn matched") {
		t.Fatalf("alpha.SelectTurn(missing) error = %v, want no-match error", err)
	}

	augmentedPrompt := strings.Join([]string{
		"<agh-situation-context>",
		`{"self":{"session_id":"sess_123","agent_name":"alpha"}}`,
		"</agh-situation-context>",
		"",
		"<current-available-skills>",
		`  <skill name="agh">Operate AGH runtime surfaces.</skill>`,
		"</current-available-skills>",
		"",
		"The <current-available-skills> block above is the authoritative current skill state for this turn.",
		"If it differs from any earlier <available-skills> startup snapshot, trust the current block.",
		"Use `agh__skill_view` to load full instructions for any skill.",
		"Use `agh__skill_view` to read a specific skill resource file when the skill references one.",
		currentSkillsCatalogFinalLine,
		"",
		"<turn-recall>",
		"Relevant durable memory for this turn:",
		"- Global [user]",
		"  Snippet: remember the harness",
		"Use recalled memory only when it remains consistent with the current repository and runtime state.",
		"</turn-recall>",
		"",
		"<user-message>",
		"hello alpha",
		"</user-message>",
	}, "\n")
	turn, err := alpha.SelectTurn(
		augmentedPrompt,
		acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	)
	if err != nil {
		t.Fatalf("alpha.SelectTurn(augmented prompt) error = %v", err)
	}
	if got, want := turn.Name, "alpha-hello"; got != want {
		t.Fatalf("augmented turn.Name = %q, want %q", got, want)
	}

	startupAugmentedPrompt := strings.Join([]string{
		"Session instructions (treat as system guidance for this conversation):",
		"",
		"You are alpha.",
		"",
		"User request:",
		"",
		"<agh-situation-context>",
		`{"self":{"session_id":"sess_123","agent_name":"alpha"}}`,
		"</agh-situation-context>",
		"",
		"<current-available-skills>",
		`  <skill name="agh">Operate AGH runtime surfaces.</skill>`,
		"</current-available-skills>",
		"",
		"The <current-available-skills> block above is the authoritative current skill state for this turn.",
		"If it differs from any earlier <available-skills> startup snapshot, trust the current block.",
		"Use `agh__skill_view` to load full instructions for any skill.",
		"Use `agh__skill_view` to read a specific skill resource file when the skill references one.",
		"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
		"",
		"hello alpha",
	}, "\n")
	turn, err = alpha.SelectTurn(
		startupAugmentedPrompt,
		acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	)
	if err != nil {
		t.Fatalf("alpha.SelectTurn(startup augmented prompt) error = %v", err)
	}
	if got, want := turn.Name, "alpha-hello"; got != want {
		t.Fatalf("startup augmented turn.Name = %q, want %q", got, want)
	}

	memoryAugmentedPrompt := strings.Join([]string{
		"<turn-recall>",
		"Relevant durable memory for this turn:",
		"- Auth [workspace]",
		"</turn-recall>",
		"",
		"<user-message>",
		"hello alpha",
		"</user-message>",
	}, "\n")
	turn, err = alpha.SelectTurn(
		memoryAugmentedPrompt,
		acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	)
	if err != nil {
		t.Fatalf("alpha.SelectTurn(memory augmented prompt) error = %v", err)
	}
	if got, want := turn.Name, "alpha-hello"; got != want {
		t.Fatalf("memory augmented turn.Name = %q, want %q", got, want)
	}

	networkFixture, err := LoadFixture(filepath.Join("testdata", "network_collaboration_fixture.json"))
	if err != nil {
		t.Fatalf("LoadFixture(network) error = %v", err)
	}
	ops, err := networkFixture.Agent("ops-coordinator")
	if err != nil {
		t.Fatalf("fixture.Agent(ops-coordinator) error = %v", err)
	}
	turn, err = ops.SelectTurn("", acp.PromptMeta{
		TurnSource: acp.PromptTurnSourceNetwork,
		Network: &acp.PromptNetworkMeta{
			MessageID:   "msg_direct_01",
			Kind:        "say",
			Channel:     "builders",
			Surface:     "direct",
			From:        "patch-worker.sess",
			WorkID:      "work_patch_42",
			ReplyTo:     "msg_say_01",
			TraceID:     "trace_ops_patch_42",
			To:          "ops-coordinator.sess",
			CausationID: "msg_say_01",
			Trust:       "untrusted",
		},
	})
	if err != nil {
		t.Fatalf("ops.SelectTurn(network) error = %v", err)
	}
	if got, want := turn.Name, "accept-direct-request"; got != want {
		t.Fatalf("network turn.Name = %q, want %q", got, want)
	}

	capabilityCurator, err := networkFixture.Agent("capability-curator")
	if err != nil {
		t.Fatalf("fixture.Agent(capability-curator) error = %v", err)
	}
	turn, err = capabilityCurator.SelectTurn("", acp.PromptMeta{
		TurnSource: acp.PromptTurnSourceNetwork,
		Network: &acp.PromptNetworkMeta{
			MessageID: "msg_capability_say_01",
			Kind:      "say",
			Channel:   "capabilities",
			Surface:   "thread",
			ThreadID:  "thread_capabilities_main",
			From:      "release-bot.sess",
			To:        "capability-curator.sess",
			Trust:     "untrusted",
		},
	})
	if err != nil {
		t.Fatalf("capabilityCurator.SelectTurn(network) error = %v", err)
	}
	if got, want := turn.Name, "observe-capability-request"; got != want {
		t.Fatalf("capability curator turn.Name = %q, want %q", got, want)
	}
}

func TestTurnMatchNetworkRequiresExactConversationMetadata(t *testing.T) {
	t.Parallel()

	directMatcher := TurnMatchNetwork{
		MessageID:   "msg_direct_01",
		Kind:        "say",
		Channel:     "builders",
		Surface:     "direct",
		DirectID:    "direct_0123456789abcdef0123456789abcdef",
		From:        "patch-worker.sess",
		To:          "ops-coordinator.sess",
		WorkID:      "work_patch_42",
		ReplyTo:     "msg_say_01",
		TraceID:     "trace_ops_patch_42",
		CausationID: "msg_say_01",
		Trust:       "untrusted",
	}
	directMeta := acp.PromptNetworkMeta{
		MessageID:   "msg_direct_01",
		Kind:        "say",
		Channel:     "builders",
		Surface:     "direct",
		DirectID:    "direct_0123456789abcdef0123456789abcdef",
		From:        "patch-worker.sess",
		To:          "ops-coordinator.sess",
		WorkID:      "work_patch_42",
		ReplyTo:     "msg_say_01",
		TraceID:     "trace_ops_patch_42",
		CausationID: "msg_say_01",
		Trust:       "untrusted",
	}
	if !directMatcher.matches(directMeta) {
		t.Fatal("direct matcher did not match exact final conversation metadata")
	}

	directCases := []struct {
		name string
		edit func(*acp.PromptNetworkMeta)
	}{
		{name: "surface", edit: func(meta *acp.PromptNetworkMeta) { meta.Surface = "thread" }},
		{name: "direct id", edit: func(meta *acp.PromptNetworkMeta) { meta.DirectID = "direct_wrong" }},
		{name: "work id", edit: func(meta *acp.PromptNetworkMeta) { meta.WorkID = "work_wrong" }},
		{name: "reply to", edit: func(meta *acp.PromptNetworkMeta) { meta.ReplyTo = "msg_wrong" }},
		{name: "trace id", edit: func(meta *acp.PromptNetworkMeta) { meta.TraceID = "trace_wrong" }},
		{name: "causation id", edit: func(meta *acp.PromptNetworkMeta) { meta.CausationID = "msg_wrong" }},
		{name: "trust", edit: func(meta *acp.PromptNetworkMeta) { meta.Trust = "verified" }},
	}
	for _, tc := range directCases {
		t.Run("Should direct rejects wrong "+tc.name, func(t *testing.T) {
			t.Parallel()

			meta := directMeta
			tc.edit(&meta)
			if directMatcher.matches(meta) {
				t.Fatalf("direct matcher matched wrong %s metadata: %#v", tc.name, meta)
			}
		})
	}

	threadMatcher := TurnMatchNetwork{
		MessageID: "msg_say_01",
		Kind:      "say",
		Channel:   "builders",
		Surface:   "thread",
		ThreadID:  "thread_builders_main",
		TraceID:   "trace_ops_patch_42",
		Trust:     "untrusted",
	}
	threadMeta := acp.PromptNetworkMeta{
		MessageID: "msg_say_01",
		Kind:      "say",
		Channel:   "builders",
		Surface:   "thread",
		ThreadID:  "thread_builders_main",
		TraceID:   "trace_ops_patch_42",
		Trust:     "untrusted",
	}
	if !threadMatcher.matches(threadMeta) {
		t.Fatal("thread matcher did not match exact final conversation metadata")
	}

	threadCases := []struct {
		name string
		edit func(*acp.PromptNetworkMeta)
	}{
		{name: "surface", edit: func(meta *acp.PromptNetworkMeta) { meta.Surface = "direct" }},
		{name: "thread id", edit: func(meta *acp.PromptNetworkMeta) { meta.ThreadID = "thread_wrong" }},
		{name: "trace id", edit: func(meta *acp.PromptNetworkMeta) { meta.TraceID = "trace_wrong" }},
		{name: "trust", edit: func(meta *acp.PromptNetworkMeta) { meta.Trust = "verified" }},
	}
	for _, tc := range threadCases {
		t.Run("Should thread rejects wrong "+tc.name, func(t *testing.T) {
			t.Parallel()

			meta := threadMeta
			tc.edit(&meta)
			if threadMatcher.matches(meta) {
				t.Fatalf("thread matcher matched wrong %s metadata: %#v", tc.name, meta)
			}
		})
	}
}

func TestCanonicalUserTextStripsPromptAugmentationLayers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name: "Should preserve plain user prompt",
			prompt: `
hello alpha
`,
			want: "hello alpha",
		},
		{
			name: "Should strip layered situation skills and durable memory wrappers",
			prompt: strings.Join([]string{
				"<agh-situation-context>",
				`{"self":{"session_id":"sess_123","agent_name":"alpha"}}`,
				"</agh-situation-context>",
				"",
				"<current-available-skills>",
				`<skill name="qa-marker">Marker.</skill>`,
				"</current-available-skills>",
				"",
				"The <current-available-skills> block above is the authoritative current skill state for this turn.",
				"If it differs from any earlier <available-skills> startup snapshot, trust the current block.",
				"Use `agh__skill_view` to load full instructions for any skill.",
				"Use `agh__skill_view` to read a specific skill resource file when the skill references one.",
				"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
				"",
				"<turn-recall>",
				"Relevant durable memory for this turn:",
				"",
				"- project: keep search visibility sentinel visible",
				"</turn-recall>",
				"",
				"<user-message>",
				"hello alpha",
				"</user-message>",
			}, "\n"),
			want: "hello alpha",
		},
		{
			name: "Should strip current skill instructions from session-prefixed prompt",
			prompt: strings.Join([]string{
				"<current-available-skills>",
				`<skill name="qa-marker">Marker.</skill>`,
				"</current-available-skills>",
				"",
				"The <current-available-skills> block above is the authoritative current skill state for this turn.",
				"If it differs from any earlier <available-skills> startup snapshot, trust the current block.",
				"Resolve canonical `agh__skill_view` through the active harness, then call the returned tool reference to load full instructions for any skill.",
				"Use the returned tool reference for canonical `agh__skill_view` to read a specific skill resource file when the skill references one.",
				"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
				"",
				"hello alpha",
			}, "\n"),
			want: "hello alpha",
		},
		{
			name: "Should stop at malformed AGH wrapper",
			prompt: strings.Join([]string{
				"<current-available-skills>",
				`<skill name="qa-marker">Marker.</skill>`,
				"",
				"hello alpha",
			}, "\n"),
			want: strings.Join([]string{
				"<current-available-skills>",
				`<skill name="qa-marker">Marker.</skill>`,
				"",
				"hello alpha",
			}, "\n"),
		},
		{
			name: "Should strip self closing legacy available skills block",
			prompt: strings.Join([]string{
				"<available-skills />",
				"",
				"hello alpha",
			}, "\n"),
			want: "hello alpha",
		},
		{
			name: "Should strip bridge inbound envelope",
			prompt: strings.Join([]string{
				"Inbound bridge message",
				"Platform message ID: 322",
				"Received at: 2026-05-05T23:58:35Z",
				"Sender: Alice Example @alice id=888",
				"Peer ID: 777",
				"Thread ID: 654",
				"",
				"Provide a follow-up runtime bridge summary",
			}, "\n"),
			want: "Provide a follow-up runtime bridge summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := canonicalUserText(tt.prompt); got != tt.want {
				t.Fatalf("canonicalUserText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDriverPathHonorsExplicitAndEnvOverrides(t *testing.T) {
	if got, err := resolveDriverPath("/tmp/custom-driver"); err != nil || got != "/tmp/custom-driver" {
		t.Fatalf("resolveDriverPath(override) = %q, %v, want override path", got, err)
	}

	t.Setenv(driverBinaryEnvVar, "/env/acpmock-driver")
	if got, err := resolveDriverPath(""); err != nil || got != "/env/acpmock-driver" {
		t.Fatalf("resolveDriverPath(env) = %q, %v, want /env/acpmock-driver", got, err)
	}
}

func TestRegistrationHelperOverridesAndDiagnosticsErrors(t *testing.T) {
	t.Run("Should default diagnostics path uses logs directory", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		got, err := resolveDiagnosticsPath(homePaths, "alpha", "")
		if err != nil {
			t.Fatalf("resolveDiagnosticsPath() error = %v", err)
		}
		want := filepath.Join(homePaths.LogsDir, "acpmock", "alpha.jsonl")
		if got != want {
			t.Fatalf("resolveDiagnosticsPath() = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve diagnostics path rejects unsafe agent names", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		if _, err := resolveDiagnosticsPath(homePaths, "../alpha", ""); err == nil ||
			!strings.Contains(err.Error(), "invalid agent name") {
			t.Fatalf("resolveDiagnosticsPath(unsafe) error = %v, want invalid-agent-name error", err)
		}
	})

	t.Run("Should resolve diagnostics path requires logs directory without override", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		homePaths.LogsDir = " "
		if _, err := resolveDiagnosticsPath(homePaths, "alpha", ""); err == nil ||
			!strings.Contains(err.Error(), "logs directory is required") {
			t.Fatalf("resolveDiagnosticsPath(blank logs dir) error = %v, want logs-dir validation", err)
		}
	})

	t.Run("Should render agent def uses default prompt", func(t *testing.T) {
		t.Parallel()

		content := renderAgentDef(
			"mock-alpha",
			AgentFixture{Provider: "claude", ReasoningEffort: "max"},
			"node driver.js",
			"claude",
		)
		if !strings.Contains(content, "You are mock-alpha.") {
			t.Fatalf("renderAgentDef() = %q, want default prompt", content)
		}
		if !strings.Contains(content, "reasoning_effort: max") {
			t.Fatalf("renderAgentDef() = %q, want reasoning effort", content)
		}
	})

	t.Run("Should read diagnostics reports invalid json", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "bad.jsonl")
		if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", path, err)
		}
		if _, err := ReadDiagnostics(path); err == nil || !strings.Contains(err.Error(), "decode diagnostics line 1") {
			t.Fatalf("ReadDiagnostics(invalid) error = %v, want decode error", err)
		}
	})

	t.Run("Should register requires agents dir", func(t *testing.T) {
		t.Parallel()

		if _, err := Register(aghconfig.HomePaths{}, RegisterOptions{}); err == nil ||
			!strings.Contains(err.Error(), "agents directory is required") {
			t.Fatalf("Register(empty home) error = %v, want agents-dir validation", err)
		}
	})

	t.Run("Should register reports missing fixture agent", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		_, err := Register(homePaths, RegisterOptions{
			FixturePath:  filepath.Join("testdata", "multi_agent_fixture.json"),
			FixtureAgent: "missing",
			DriverPath:   "/tmp/mock-driver",
		})
		if err == nil || !strings.Contains(err.Error(), "lookup fixture agent") {
			t.Fatalf("Register(missing fixture agent) error = %v, want lookup context", err)
		}
	})

	t.Run("Should register rejects unsafe runtime agent names", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		_, err := Register(homePaths, RegisterOptions{
			FixturePath:  filepath.Join("testdata", "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    "../escape",
			DriverPath:   "/tmp/mock-driver",
		})
		if err == nil || !strings.Contains(err.Error(), "validate runtime agent name") {
			t.Fatalf("Register(unsafe runtime agent name) error = %v, want runtime-agent validation", err)
		}
	})

	t.Run("Should register wraps diagnostics path failures", func(t *testing.T) {
		t.Parallel()

		homePaths := mockHomePaths(t)
		homePaths.LogsDir = ""
		_, err := Register(homePaths, RegisterOptions{
			FixturePath:  filepath.Join("testdata", "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    "alpha",
			DriverPath:   "/tmp/mock-driver",
		})
		if err == nil || !strings.Contains(err.Error(), "resolve diagnostics path") {
			t.Fatalf("Register(blank logs dir) error = %v, want diagnostics-path context", err)
		}
	})
}

func TestBuildDriverBinaryHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	repoRoot, err := repoRootFromCaller()
	if err != nil {
		t.Fatalf("repoRootFromCaller() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = buildDriverBinary(ctx, repoRoot, filepath.Join(t.TempDir(), driverBinaryName()))
	if err == nil {
		t.Fatal("buildDriverBinary(canceled) error = nil, want context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("buildDriverBinary(canceled) error = %v, want context.Canceled", err)
	}
}

func TestDefaultDriverPathSharesConcurrentBuildResult(t *testing.T) {
	driverBinaryMu.Lock()
	cached := driverBinaryPath
	driverBinaryPath = ""
	driverBinaryMu.Unlock()
	t.Cleanup(func() {
		if strings.TrimSpace(cached) == "" {
			return
		}
		driverBinaryMu.Lock()
		driverBinaryPath = cached
		driverBinaryMu.Unlock()
	})

	const callers = 4
	type result struct {
		path string
		err  error
	}

	results := make(chan result, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			path, err := DefaultDriverPath()
			results <- result{path: path, err: err}
		})
	}
	wg.Wait()
	close(results)

	var builtPath string
	for item := range results {
		if item.err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", item.err)
		}
		if builtPath == "" {
			builtPath = item.path
			continue
		}
		if item.path != builtPath {
			t.Fatalf("DefaultDriverPath() path = %q, want shared cached path %q", item.path, builtPath)
		}
	}

	if builtPath == "" {
		t.Fatal("DefaultDriverPath() returned empty path for all callers")
	}
	if _, err := os.Stat(builtPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", builtPath, err)
	}
}

func TestValidationAndDriverHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should validation helpers reject invalid values", func(t *testing.T) {
		t.Parallel()

		if err := validateToolKind("tool_kind", "bogus"); err == nil {
			t.Fatal("validateToolKind(invalid) error = nil, want non-nil")
		}
		if err := validateToolStatus("status", "bogus"); err == nil {
			t.Fatal("validateToolStatus(invalid) error = nil, want non-nil")
		}
		if err := validatePermissionDecision("decision", "bogus"); err == nil {
			t.Fatal("validatePermissionDecision(invalid) error = nil, want non-nil")
		}
		if (TurnMatch{Judge: &TurnMatchJudge{Attempt: -1}}).Validate("match") == nil {
			t.Fatal("TurnMatch.Validate(negative judge attempt) error = nil, want non-nil")
		}
		if (TurnMatch{TurnSource: acp.PromptTurnSourceUser}).Validate("match") != nil {
			t.Fatal("TurnMatch.Validate(user selector) error != nil, want nil")
		}
		if (TurnMatch{TurnSource: acp.PromptTurnSourceSynthetic}).Validate("match") != nil {
			t.Fatal("TurnMatch.Validate(synthetic selector) error != nil, want nil")
		}
		if (TurnMatchNetwork{}).Validate("match.network") == nil {
			t.Fatal("TurnMatchNetwork.Validate(empty) error = nil, want non-nil")
		}
		if (DriverControlStep{Action: DriverControlWriteRawJSONRPC}).Validate("driver_control") == nil {
			t.Fatal("DriverControlStep.Validate(missing raw_jsonrpc) error = nil, want non-nil")
		}
		if (DriverControlStep{Action: DriverControlDisconnect, DelayMS: -1}).Validate("driver_control") == nil {
			t.Fatal("DriverControlStep.Validate(negative delay) error = nil, want non-nil")
		}
		if (DriverControlStep{Action: DriverControlBlockUntilCancel, Async: true}).Validate("driver_control") == nil {
			t.Fatal("DriverControlStep.Validate(async block_until_cancel) error = nil, want non-nil")
		}
		err := (DriverControlStep{Action: DriverControlDelay}).Validate("driver_control")
		if err == nil || !strings.Contains(err.Error(), "delay_ms must be > 0 for delay") {
			t.Fatalf("DriverControlStep.Validate(delay without delay_ms) error = %v, want delay_ms diagnostic", err)
		}
		err = (DriverControlStep{Action: DriverControlDelay, DelayMS: 1, Async: true}).Validate("driver_control")
		if err == nil || !strings.Contains(err.Error(), "async is invalid for delay") {
			t.Fatalf("DriverControlStep.Validate(async delay) error = %v, want async diagnostic", err)
		}
		if err := (DriverControlStep{Action: DriverControlDelay, DelayMS: 1}).Validate("driver_control"); err != nil {
			t.Fatalf("DriverControlStep.Validate(delay) error = %v", err)
		}
		if (TurnFixture{}).Validate("turn") == nil {
			t.Fatal("TurnFixture.Validate(no steps) error = nil, want non-nil")
		}
		if hasTextPayload("", nil) {
			t.Fatal("hasTextPayload(empty) = true, want false")
		}
	})

	t.Run("Should driver helpers resolve defaults and omit empty diagnostics", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		if got, err := resolveDriverPath(""); err != nil || got != driverPath {
			t.Fatalf("resolveDriverPath(default) = %q, %v, want %q", got, err, driverPath)
		}

		command := BuildCommand("/tmp/driver", "/tmp/fixture.json", "alpha", "")
		if strings.Contains(command, "--diagnostics") {
			t.Fatalf("BuildCommand() = %q, want no diagnostics flag", command)
		}
	})

	t.Run("Should read diagnostics reports missing file", func(t *testing.T) {
		t.Parallel()

		if _, err := ReadDiagnostics(filepath.Join(t.TempDir(), "missing.jsonl")); err == nil ||
			!strings.Contains(err.Error(), "open diagnostics") {
			t.Fatalf("ReadDiagnostics(missing) error = %v, want open error", err)
		}
	})
}

func mockHomePaths(t testing.TB) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}
