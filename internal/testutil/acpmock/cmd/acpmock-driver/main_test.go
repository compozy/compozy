package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func TestNetworkEnvironmentNames(t *testing.T) {
	t.Run("Should return only populated Network environment names", func(t *testing.T) {
		t.Setenv("AGH_SESSION_CHANNEL", "builders")
		t.Setenv("AGH_PEER_ID", "coder.session-1")

		got := networkEnvironmentNames()
		want := []string{"AGH_SESSION_CHANNEL", "AGH_PEER_ID"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("networkEnvironmentNames() = %#v, want %#v", got, want)
		}

		if err := os.Unsetenv("AGH_SESSION_CHANNEL"); err != nil {
			t.Fatalf("Unsetenv(AGH_SESSION_CHANNEL) error = %v", err)
		}
		if err := os.Unsetenv("AGH_PEER_ID"); err != nil {
			t.Fatalf("Unsetenv(AGH_PEER_ID) error = %v", err)
		}
		if got := networkEnvironmentNames(); len(got) != 0 {
			t.Fatalf("networkEnvironmentNames() = %#v, want empty", got)
		}
	})
}

func TestPromptResponseUsagePreservesExplicitZeroTotal(t *testing.T) {
	t.Parallel()

	fixture, err := acpmock.ParseFixture([]byte(`{
		"version": 2,
		"agents": [{
			"name": "usage-agent",
			"provider": "claude",
			"turns": [{
				"match": {"turn_source": "user", "user_text": "measure"},
				"steps": [{"kind": "assistant", "text": "measured"}],
				"usage": {"input_tokens": 10, "output_tokens": 2, "total_tokens": 0}
			}]
		}]
	}`))
	if err != nil {
		t.Fatalf("ParseFixture() error = %v", err)
	}

	usage := promptResponseUsage(fixture.Agents[0].Turns[0])
	if usage == nil || usage.TotalTokens != 0 {
		t.Fatalf("promptResponseUsage().TotalTokens = %v, want explicit zero", usage)
	}
}

func TestExtractPromptTextPreservesAugmentedPromptDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve augmented prompt diagnostics", func(t *testing.T) {
		t.Parallel()

		prompt := "Session instructions\n\n" +
			"User request:\n\n" +
			"<agh-situation-context>{}</agh-situation-context>\n\n" +
			"<turn-recall>\n" +
			"Relevant durable memory for this turn:\n" +
			"- Auth [workspace]\n" +
			"</turn-recall>\n\n" +
			"<user-message>\n" +
			"hello alpha\n" +
			"</user-message>"
		blocks := []acpsdk.ContentBlock{
			acpsdk.TextBlock("ignored"),
			acpsdk.TextBlock(prompt),
		}

		if got, want := extractPromptText(blocks), prompt; got != want {
			t.Fatalf("extractPromptText() = %q, want %q", got, want)
		}
	})
}

func TestMockAgentInitializeAdvertisesFixtureLoadCapability(t *testing.T) {
	t.Parallel()

	loadUnsupported := false
	tests := []struct {
		name    string
		fixture acpmock.AgentFixture
		want    bool
	}{
		{
			name:    "Should preserve load support for existing fixtures",
			fixture: acpmock.AgentFixture{Name: "default-load"},
			want:    true,
		},
		{
			name: "Should advertise missing load support when the fixture disables it",
			fixture: acpmock.AgentFixture{
				Name:        "no-load",
				LoadSession: &loadUnsupported,
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			agent := &mockAgent{agent: tc.fixture}
			response, err := agent.Initialize(context.Background(), acpsdk.InitializeRequest{})
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			if got := response.AgentCapabilities.LoadSession; got != tc.want {
				t.Fatalf("Initialize().AgentCapabilities.LoadSession = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractPromptTextPreservesAugmentedPromptWithoutNestedMessageMarker(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve augmented prompt without nested message marker", func(t *testing.T) {
		t.Parallel()

		prompt := "Session instructions\n\n" +
			"User request:\n\n" +
			"<agh-situation-context>{}</agh-situation-context>\n\n" +
			"hello alpha"
		blocks := []acpsdk.ContentBlock{
			acpsdk.TextBlock(prompt),
		}

		if got, want := extractPromptText(blocks), prompt; got != want {
			t.Fatalf("extractPromptText() = %q, want %q", got, want)
		}
	})
}

func TestMockAgentSelectTurnDoesNotCountUnmatchedPrompts(t *testing.T) {
	t.Parallel()

	t.Run("Should not count unmatched prompts", func(t *testing.T) {
		t.Parallel()

		agent := &mockAgent{
			agent: acpmock.AgentFixture{
				Name: "alpha",
				Turns: []acpmock.TurnFixture{
					{
						Name: "first",
						Match: acpmock.TurnMatch{
							TurnSource: acp.PromptTurnSourceUser,
							UserText:   "first prompt",
						},
					},
					{
						Name: "second",
						Match: acpmock.TurnMatch{
							TurnSource: acp.PromptTurnSourceUser,
							UserText:   "second prompt",
						},
					},
				},
			},
			sessions: map[string]*sessionState{},
		}
		meta := acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser}

		first, promptIndex, err := agent.selectTurn("acp-session-1", "first prompt", meta)
		if err != nil {
			t.Fatalf("selectTurn(first) error = %v", err)
		}
		if first.Name != "first" || promptIndex != 1 {
			t.Fatalf("selectTurn(first) = (%q, %d), want (first, 1)", first.Name, promptIndex)
		}

		_, promptIndex, err = agent.selectTurn("acp-session-1", "extractor internal prompt", meta)
		if err == nil || !strings.Contains(err.Error(), "no turn matched") {
			t.Fatalf("selectTurn(unmatched) error = %v, want no-match error", err)
		}
		if promptIndex != 2 {
			t.Fatalf("selectTurn(unmatched) prompt index = %d, want 2", promptIndex)
		}

		second, promptIndex, err := agent.selectTurn("acp-session-1", "second prompt", meta)
		if err != nil {
			t.Fatalf("selectTurn(second) error = %v", err)
		}
		if second.Name != "second" || promptIndex != 2 {
			t.Fatalf("selectTurn(second) = (%q, %d), want (second, 2)", second.Name, promptIndex)
		}
	})
}

func TestStopReasonMapsCompleteACPVocabulary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want acpsdk.StopReason
	}{
		{name: "Should default an empty reason to end turn", want: acpsdk.StopReasonEndTurn},
		{name: "Should map end turn", raw: string(acpsdk.StopReasonEndTurn), want: acpsdk.StopReasonEndTurn},
		{name: "Should map max tokens", raw: string(acpsdk.StopReasonMaxTokens), want: acpsdk.StopReasonMaxTokens},
		{
			name: "Should map max turn requests", raw: string(acpsdk.StopReasonMaxTurnRequests),
			want: acpsdk.StopReasonMaxTurnRequests,
		},
		{name: "Should map refusal", raw: string(acpsdk.StopReasonRefusal), want: acpsdk.StopReasonRefusal},
		{
			name: "Should map canceled ACP spelling",
			raw:  string(acpsdk.StopReasonCancelled),
			want: acpsdk.StopReasonCancelled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := stopReason(tt.raw); got != tt.want {
				t.Fatalf("stopReason(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMockAgentSessionConfigOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should update current select values", func(t *testing.T) {
		t.Parallel()

		diagnosticsPath := filepath.Join(t.TempDir(), "config-options.jsonl")
		agent := &mockAgent{
			agent:           acpmock.AgentFixture{Name: "config-options"},
			diagnosticsPath: diagnosticsPath,
			configTemplate: sessionConfigOptionsFromFixture([]acpmock.SessionConfigOptionFixture{
				{
					ID:      "model",
					Name:    "Model",
					Current: "qa-browser-model",
					Values: []acpmock.SessionConfigOptionValueFixture{
						{Value: "qa-browser-model", Label: "QA Browser Model"},
						{Value: "qa-browser-model-alt", Label: "QA Browser Model Alt"},
					},
				},
			}),
			sessions: map[string]*sessionState{},
		}
		session, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{})
		if err != nil {
			t.Fatalf("NewSession() error = %v", err)
		}

		response, err := agent.SetSessionConfigOption(
			context.Background(),
			acpsdk.SetSessionConfigOptionRequest{
				ValueId: &acpsdk.SetSessionConfigOptionValueId{
					SessionId: session.SessionId,
					ConfigId:  acpsdk.SessionConfigId("model"),
					Value:     acpsdk.SessionConfigValueId("qa-browser-model-alt"),
				},
			},
		)
		if err != nil {
			t.Fatalf("SetSessionConfigOption() error = %v", err)
		}
		if got, want := response.ConfigOptions[0].Select.CurrentValue, acpsdk.SessionConfigValueId(
			"qa-browser-model-alt",
		); got != want {
			t.Fatalf("CurrentValue = %q, want %q", got, want)
		}
		records, err := acpmock.ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics() error = %v", err)
		}
		protocol := acpmock.ProtocolDiagnostics(records)
		if len(protocol) != 1 ||
			protocol[0].ProtocolMethod != acpsdk.AgentMethodSessionSetConfigOption ||
			protocol[0].ConfigOptionID != "model" ||
			protocol[0].ConfigOptionValue != "qa-browser-model-alt" {
			t.Fatalf("protocol diagnostics = %#v, want model set_config_option record", protocol)
		}

		_, err = agent.SetSessionConfigOption(
			context.Background(),
			acpsdk.SetSessionConfigOptionRequest{
				ValueId: &acpsdk.SetSessionConfigOptionValueId{
					SessionId: session.SessionId,
					ConfigId:  acpsdk.SessionConfigId("model"),
					Value:     acpsdk.SessionConfigValueId("missing-model"),
				},
			},
		)
		if err == nil || !strings.Contains(err.Error(), "is not available") {
			t.Fatalf("SetSessionConfigOption(missing) error = %v, want unavailable value", err)
		}
	})

	t.Run("Should keep config options scoped to each session", func(t *testing.T) {
		t.Parallel()

		agent := &mockAgent{
			configTemplate: sessionConfigOptionsFromFixture([]acpmock.SessionConfigOptionFixture{
				{
					ID:      "model",
					Name:    "Model",
					Current: "qa-browser-model",
					Values: []acpmock.SessionConfigOptionValueFixture{
						{Value: "qa-browser-model", Label: "QA Browser Model"},
						{Value: "qa-browser-model-alt", Label: "QA Browser Model Alt"},
					},
				},
			}),
			sessions: map[string]*sessionState{},
		}

		first, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{})
		if err != nil {
			t.Fatalf("NewSession(first) error = %v", err)
		}
		second, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{})
		if err != nil {
			t.Fatalf("NewSession(second) error = %v", err)
		}

		response, err := agent.SetSessionConfigOption(
			context.Background(),
			acpsdk.SetSessionConfigOptionRequest{
				ValueId: &acpsdk.SetSessionConfigOptionValueId{
					SessionId: first.SessionId,
					ConfigId:  acpsdk.SessionConfigId("model"),
					Value:     acpsdk.SessionConfigValueId("qa-browser-model-alt"),
				},
			},
		)
		if err != nil {
			t.Fatalf("SetSessionConfigOption(first) error = %v", err)
		}
		if got, want := response.ConfigOptions[0].Select.CurrentValue, acpsdk.SessionConfigValueId(
			"qa-browser-model-alt",
		); got != want {
			t.Fatalf("first CurrentValue = %q, want %q", got, want)
		}

		resumedFirst, err := agent.ResumeSession(
			context.Background(),
			acpsdk.ResumeSessionRequest{SessionId: first.SessionId},
		)
		if err != nil {
			t.Fatalf("ResumeSession(first) error = %v", err)
		}
		if got, want := resumedFirst.ConfigOptions[0].Select.CurrentValue, acpsdk.SessionConfigValueId(
			"qa-browser-model-alt",
		); got != want {
			t.Fatalf("resumed first CurrentValue = %q, want %q", got, want)
		}

		loadedSecond, err := agent.LoadSession(
			context.Background(),
			acpsdk.LoadSessionRequest{SessionId: second.SessionId},
		)
		if err != nil {
			t.Fatalf("LoadSession(second) error = %v", err)
		}
		if got, want := loadedSecond.ConfigOptions[0].Select.CurrentValue, acpsdk.SessionConfigValueId(
			"qa-browser-model",
		); got != want {
			t.Fatalf("loaded second CurrentValue = %q, want %q", got, want)
		}
	})
}

func TestMockAgentLoadSessionValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject empty session id without panicking", func(t *testing.T) {
		t.Parallel()

		agent := &mockAgent{
			sessions: map[string]*sessionState{},
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("LoadSession(empty) panic = %v, want validation error", recovered)
			}
		}()

		_, err := agent.LoadSession(context.Background(), acpsdk.LoadSessionRequest{})
		if err == nil || !strings.Contains(err.Error(), "session id is required") {
			t.Fatalf("LoadSession(empty) error = %v, want session id validation", err)
		}
	})
}

func TestMockAgentSandboxTerminalCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should release terminal with detached context after wait cancellation", func(t *testing.T) {
		t.Parallel()

		conn := &recordingSandboxConnection{}
		agent := &mockAgent{conn: conn}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result := agent.runSandboxCommand(ctx, acpsdk.SessionId("sess-1"), acpmock.Step{
			Command: "/bin/sh",
			Args:    []string{"-c", "sleep 30"},
		})

		if !conn.releaseCalled {
			t.Fatal("ReleaseTerminal() was not called after terminal creation")
		}
		if conn.releaseContextErr != nil {
			t.Fatalf("ReleaseTerminal() context error = %v, want detached cleanup context", conn.releaseContextErr)
		}
		if !strings.Contains(result.ObservedError, "context canceled") {
			t.Fatalf("ObservedError = %q, want wait cancellation surfaced", result.ObservedError)
		}
	})
}

type recordingSandboxConnection struct {
	releaseCalled     bool
	releaseContextErr error
}

func (c *recordingSandboxConnection) SessionUpdate(
	context.Context,
	acpsdk.SessionNotification,
) error {
	return nil
}

func (c *recordingSandboxConnection) RequestPermission(
	context.Context,
	acpsdk.RequestPermissionRequest,
) (acpsdk.RequestPermissionResponse, error) {
	return acpsdk.RequestPermissionResponse{}, nil
}

func (c *recordingSandboxConnection) CreateTerminal(
	context.Context,
	acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{TerminalId: "term-cancel"}, nil
}

func (c *recordingSandboxConnection) WaitForTerminalExit(
	ctx context.Context,
	_ acpsdk.WaitForTerminalExitRequest,
) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, ctx.Err()
}

func (c *recordingSandboxConnection) TerminalOutput(
	context.Context,
	acpsdk.TerminalOutputRequest,
) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, nil
}

func (c *recordingSandboxConnection) ReleaseTerminal(
	ctx context.Context,
	_ acpsdk.ReleaseTerminalRequest,
) (acpsdk.ReleaseTerminalResponse, error) {
	c.releaseCalled = true
	c.releaseContextErr = ctx.Err()
	if c.releaseContextErr != nil {
		return acpsdk.ReleaseTerminalResponse{}, c.releaseContextErr
	}
	return acpsdk.ReleaseTerminalResponse{}, nil
}
