//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentidentity"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDaemonE2EFixtureBackedMockAgentLaunchesThroughNormalAgentDefinition(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    "mock-alpha",
		}},
	})

	registration, ok := harness.MockAgentRegistration("mock-alpha")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-alpha) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := createFixtureBackedSession(t, ctx, harness, "mock-alpha", "launch-alpha")
	stream, err := harness.PromptSession(ctx, session.ID, "hello alpha")
	if err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}
	if len(stream) == 0 {
		t.Fatal("PromptSession() stream = empty, want mock agent updates")
	}

	transcriptResp, err := harness.SessionTranscript(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionTranscript() error = %v", err)
	}
	gotTranscript := joinTranscriptContent(sessionTranscriptMessages(transcriptResp))
	if !strings.Contains(gotTranscript, "alpha says hi") || !strings.Contains(gotTranscript, "bridge-alpha") {
		t.Fatalf("transcript = %q, want alpha assistant and bridge content", gotTranscript)
	}

	if err := harness.CaptureSessionTranscript(ctx, session.ID); err != nil {
		t.Fatalf("CaptureSessionTranscript() error = %v", err)
	}
	if err := harness.CaptureSessionEvents(ctx, session.ID); err != nil {
		t.Fatalf("CaptureSessionEvents() error = %v", err)
	}
	if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics() error = %v", err)
	}

	providerCallsPath, ok := harness.Artifacts.ArtifactPath(e2etest.ArtifactKindProviderCalls)
	if !ok {
		t.Fatal("ArtifactPath(provider_calls) = missing, want present")
	}
	providerCalls, err := os.ReadFile(providerCallsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", providerCallsPath, err)
	}
	if !strings.Contains(string(providerCalls), "alpha-hello") {
		t.Fatalf("provider_calls artifact = %s, want alpha diagnostics", string(providerCalls))
	}
}

func TestDaemonE2EProviderReasoningNegotiatesThroughAdvertisedACPOptions(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "reasoning_negotiation_fixture.json")
	agents := []struct {
		name         string
		fixtureAgent string
		provider     string
	}{
		{name: "reasoning-claude-max", provider: "claude"},
		{name: "reasoning-claude-agent-default", fixtureAgent: "reasoning-claude-max", provider: "claude"},
		{name: "reasoning-claude-concurrent", fixtureAgent: "reasoning-claude-max", provider: "claude"},
		{name: "reasoning-codex-max", provider: "codex"},
		{name: "reasoning-codex-unavailable", fixtureAgent: "reasoning-codex-max", provider: "codex"},
		{name: "reasoning-codex-unsupported", fixtureAgent: "reasoning-codex-max", provider: "codex"},
		{name: "reasoning-codex-none", provider: "codex"},
		{name: "reasoning-codex-missing", provider: "codex"},
	}
	specs := make([]e2etest.MockAgentSpec, 0, len(agents))
	for _, agent := range agents {
		fixtureAgent := agent.fixtureAgent
		if fixtureAgent == "" {
			fixtureAgent = agent.name
		}
		specs = append(specs, e2etest.MockAgentSpec{
			FixturePath:  fixturePath,
			FixtureAgent: fixtureAgent,
			AgentName:    agent.name,
			ProviderName: agent.provider,
		})
	}

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			for _, providerName := range []string{"claude", "codex"} {
				provider := cfg.Providers[providerName]
				provider.AuthMode = compozyconfig.ProviderAuthModeNone
				provider.NoneSecurity = compozyconfig.ProviderNoneSecurityLocalTransport
				cfg.Providers[providerName] = provider
			}
		}},
		MockAgents: specs,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// not parallel: subtests share one runtime harness and per-agent diagnostics files.

	t.Run("Should resolve the AGENT reasoning default before the first prompt", func(t *testing.T) {
		registration, ok := harness.MockAgentRegistration("reasoning-claude-agent-default")
		if !ok {
			t.Fatal("MockAgentRegistration(reasoning-claude-agent-default) = missing")
		}
		sessionPayload, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
			AgentName:     "reasoning-claude-agent-default",
			WorkspacePath: harness.WorkspaceRoot,
		})
		if err != nil {
			t.Fatalf("CreateSession(agent default) error = %v", err)
		}
		sessionPayload = waitForReasoningSessionActive(t, ctx, harness, sessionPayload)
		if _, err := harness.PromptSession(ctx, sessionPayload.ID, "claude max"); err != nil {
			t.Fatalf("PromptSession(agent default) error = %v", err)
		}
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(agent default) error = %v", err)
		}
		assertReasoningProtocolSequence(
			t,
			acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForCompozySession(records, sessionPayload.ID)),
			"sonnet",
			"effort",
			"max",
		)
	})

	positiveCases := []struct {
		name              string
		agentName         string
		runtime           *compozycontract.PromptRuntimeSelectionPayload
		transportModel    string
		effort            compozycontract.ReasoningEffort
		prompt            string
		reasoningOptionID string
	}{
		{
			name:      "Should apply Claude max after the model and before the first prompt",
			agentName: "reasoning-claude-max",
			runtime: &compozycontract.PromptRuntimeSelectionPayload{
				Provider:        "claude",
				Model:           "claude-sonnet-5",
				ReasoningEffort: "max",
			},
			transportModel:    "sonnet",
			effort:            "max",
			prompt:            "claude max",
			reasoningOptionID: "effort",
		},
		{
			name:      "Should apply Codex max after the model and before the first prompt",
			agentName: "reasoning-codex-max",
			runtime: &compozycontract.PromptRuntimeSelectionPayload{
				Provider:        "codex",
				Model:           "gpt-5.6-sol",
				ReasoningEffort: "max",
			},
			transportModel:    "gpt-5.6-sol",
			effort:            "max",
			prompt:            "codex max",
			reasoningOptionID: "reasoning_effort",
		},
		{
			name:      "Should send explicit Codex none instead of treating it as provider default",
			agentName: "reasoning-codex-none",
			runtime: &compozycontract.PromptRuntimeSelectionPayload{
				Provider:        "codex",
				Model:           "gpt-5.6-sol",
				ReasoningEffort: "none",
			},
			transportModel:    "gpt-5.6-sol",
			effort:            "none",
			prompt:            "codex none",
			reasoningOptionID: "reasoning_effort",
		},
	}
	for _, tt := range positiveCases {
		t.Run(tt.name, func(t *testing.T) {
			registration, ok := harness.MockAgentRegistration(tt.agentName)
			if !ok {
				t.Fatalf("MockAgentRegistration(%q) = missing", tt.agentName)
			}
			sessionPayload, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
				AgentName:     tt.agentName,
				WorkspacePath: harness.WorkspaceRoot,
			})
			if err != nil {
				t.Fatalf("CreateSession(%q) error = %v", tt.agentName, err)
			}
			sessionPayload = waitForReasoningSessionActive(t, ctx, harness, sessionPayload)
			if _, err := harness.PromptSessionWithRuntime(ctx, sessionPayload.ID, tt.prompt, tt.runtime); err != nil {
				t.Fatalf("PromptSession(%q) error = %v", tt.agentName, err)
			}
			records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
			if err != nil {
				t.Fatalf("ReadDiagnostics(%q) error = %v", tt.agentName, err)
			}
			assertReasoningProtocolSequence(
				t,
				acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForCompozySession(records, sessionPayload.ID)),
				tt.transportModel,
				tt.reasoningOptionID,
				string(tt.effort),
			)
		})
	}

	t.Run("Should isolate concurrent sessions when process-local ACP session IDs collide", func(t *testing.T) {
		registration, ok := harness.MockAgentRegistration("reasoning-claude-concurrent")
		if !ok {
			t.Fatal("MockAgentRegistration(reasoning-claude-concurrent) = missing")
		}

		sessions := make([]compozycontract.SessionPayload, 0, 2)
		for index := 0; index < 2; index++ {
			sessionPayload, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
				AgentName:     "reasoning-claude-concurrent",
				WorkspacePath: harness.WorkspaceRoot,
			})
			if err != nil {
				t.Fatalf("CreateSession(concurrent %d) error = %v", index, err)
			}
			sessions = append(sessions, sessionPayload)
		}
		for index, sessionPayload := range sessions {
			sessions[index] = waitForReasoningSessionActive(t, ctx, harness, sessionPayload)
		}
		if sessions[0].ID == sessions[1].ID {
			t.Fatalf("concurrent Compozy session IDs = %q, want distinct owners", sessions[0].ID)
		}

		promptResults := make(chan error, len(sessions))
		for _, sessionPayload := range sessions {
			sessionID := sessionPayload.ID
			go func() {
				_, err := harness.PromptSessionWithRuntime(
					ctx,
					sessionID,
					"claude max",
					&compozycontract.PromptRuntimeSelectionPayload{
						Provider: "claude", Model: "claude-sonnet-5", ReasoningEffort: "max",
					},
				)
				if err != nil {
					err = fmt.Errorf("PromptSession(%q): %w", sessionID, err)
				}
				promptResults <- err
			}()
		}
		var promptErr error
		for range sessions {
			promptErr = errors.Join(promptErr, <-promptResults)
		}
		if promptErr != nil {
			t.Fatal(promptErr)
		}

		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(concurrent) error = %v", err)
		}
		for index, record := range records {
			if strings.TrimSpace(record.CompozySessionID) == "" {
				t.Fatalf("diagnostics[%d] = %#v, want a daemon-owned Compozy session ID", index, record)
			}
		}

		owned := make([][]acpmock.DiagnosticsRecord, 0, len(sessions))
		for _, sessionPayload := range sessions {
			sessionRecords := acpmock.DiagnosticsForCompozySession(records, sessionPayload.ID)
			for index, record := range sessionRecords {
				if record.CompozySessionID != sessionPayload.ID {
					t.Fatalf("owned diagnostics[%d] = %#v, want owner %q", index, record, sessionPayload.ID)
				}
			}
			protocol := acpmock.ProtocolDiagnostics(sessionRecords)
			assertReasoningProtocolSequence(t, protocol, "sonnet", "effort", "max")
			owned = append(owned, protocol)
		}
		if got := owned[0][0].SessionID; got == "" || got != owned[1][0].SessionID {
			t.Fatalf(
				"process-local ACP session IDs = (%q, %q), want one non-empty collision",
				got,
				owned[1][0].SessionID,
			)
		}
	})

	t.Run("Should persist reasoning_option_missing before the first prompt", func(t *testing.T) {
		registration, ok := harness.MockAgentRegistration("reasoning-codex-missing")
		if !ok {
			t.Fatal("MockAgentRegistration(reasoning-codex-missing) = missing")
		}
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, compozycontract.CreateSessionRequest{
			AgentName:     "reasoning-codex-missing",
			WorkspacePath: harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateActive {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 active", status, accepted)
		}
		_, promptErr := harness.PromptSessionWithRuntime(
			ctx,
			accepted.ID,
			"codex missing reasoning option",
			&compozycontract.PromptRuntimeSelectionPayload{
				Provider: "codex", Model: "gpt-5.6-sol", ReasoningEffort: "max",
			},
		)
		requirePromptDispatchIndeterminate(t, promptErr, `reasoning effort "max" is unavailable`)
		requireIndeterminateRuntimeFailure(
			t,
			ctx,
			harness,
			accepted.ID,
			`reasoning effort "max" is unavailable`,
		)
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(reasoning-codex-missing) error = %v", err)
		}
		protocol := acpmock.ProtocolDiagnostics(records)
		if len(protocol) != 1 ||
			protocol[0].ProtocolMethod != acpsdk.AgentMethodSessionSetConfigOption ||
			protocol[0].ConfigOptionID != "model" ||
			protocol[0].ConfigOptionValue != "gpt-5.6-sol" {
			t.Fatalf("protocol diagnostics = %#v, want only model set_config_option", protocol)
		}
		if promptRecords := acpmock.PromptDiagnostics(records); len(promptRecords) != 0 {
			t.Fatalf("prompt diagnostics = %#v, want no prompt after negotiation failure", promptRecords)
		}
	})

	t.Run("Should persist model_unavailable before the first prompt", func(t *testing.T) {
		registration, ok := harness.MockAgentRegistration("reasoning-codex-unavailable")
		if !ok {
			t.Fatal("MockAgentRegistration(reasoning-codex-unavailable) = missing")
		}
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, compozycontract.CreateSessionRequest{
			AgentName:     "reasoning-codex-unavailable",
			WorkspacePath: harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateActive {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 active", status, accepted)
		}
		_, promptErr := harness.PromptSessionWithRuntime(
			ctx,
			accepted.ID,
			"codex unavailable model",
			&compozycontract.PromptRuntimeSelectionPayload{
				Provider: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "max",
			},
		)
		requirePromptDispatchIndeterminate(t, promptErr, `model "gpt-5.6-terra" is unavailable`)
		requireIndeterminateRuntimeFailure(
			t,
			ctx,
			harness,
			accepted.ID,
			`model "gpt-5.6-terra" is unavailable`,
		)
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(reasoning-codex-unavailable) error = %v", err)
		}
		if protocol := acpmock.ProtocolDiagnostics(records); len(protocol) != 0 {
			t.Fatalf("protocol diagnostics = %#v, want no config RPC for unavailable model", protocol)
		}
		if promptRecords := acpmock.PromptDiagnostics(records); len(promptRecords) != 0 {
			t.Fatalf("prompt diagnostics = %#v, want no prompt after unavailable model", promptRecords)
		}
	})

	t.Run("Should persist reasoning_effort_unsupported before the first prompt", func(t *testing.T) {
		registration, ok := harness.MockAgentRegistration("reasoning-codex-unsupported")
		if !ok {
			t.Fatal("MockAgentRegistration(reasoning-codex-unsupported) = missing")
		}
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, compozycontract.CreateSessionRequest{
			AgentName:     "reasoning-codex-unsupported",
			WorkspacePath: harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateActive {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 active", status, accepted)
		}
		_, promptErr := harness.PromptSessionWithRuntime(
			ctx,
			accepted.ID,
			"codex unsupported reasoning effort",
			&compozycontract.PromptRuntimeSelectionPayload{
				Provider: "codex", Model: "gpt-5.6-sol", ReasoningEffort: "minimal",
			},
		)
		requirePromptDispatchIndeterminate(t, promptErr, `reasoning effort "minimal" is unavailable`)
		requireIndeterminateRuntimeFailure(
			t,
			ctx,
			harness,
			accepted.ID,
			`reasoning effort "minimal" is unavailable`,
		)
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(reasoning-codex-unsupported) error = %v", err)
		}
		protocol := acpmock.ProtocolDiagnostics(records)
		if len(protocol) != 1 ||
			protocol[0].ProtocolMethod != acpsdk.AgentMethodSessionSetConfigOption ||
			protocol[0].ConfigOptionID != "model" ||
			protocol[0].ConfigOptionValue != "gpt-5.6-sol" {
			t.Fatalf("protocol diagnostics = %#v, want only model set_config_option", protocol)
		}
		if promptRecords := acpmock.PromptDiagnostics(records); len(promptRecords) != 0 {
			t.Fatalf("prompt diagnostics = %#v, want no prompt after unsupported effort", promptRecords)
		}
	})
}

func waitForReasoningSessionActive(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	accepted compozycontract.SessionPayload,
) compozycontract.SessionPayload {
	t.Helper()
	current := accepted
	waitForRuntimeCondition(t, "reasoning session startup", 10*time.Second, func() bool {
		resolved, err := harness.GetSession(ctx, accepted.ID)
		if err != nil {
			return false
		}
		current = resolved
		return current.State == session.StateActive || current.State == session.StateStopped
	})
	if current.State != session.StateActive {
		t.Fatalf("reasoning session startup = %#v, want active", current)
	}
	return current
}

func requireIndeterminateRuntimeFailure(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	wantSummary string,
) compozycontract.SessionPayload {
	t.Helper()
	current, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", sessionID, err)
	}
	if current.State != session.StateActive || current.Runtime.Status != session.RuntimeStatusUnbound ||
		!strings.Contains(current.Runtime.Failure, wantSummary) {
		t.Fatalf(
			"reasoning negotiation session = %#v, want active/unbound runtime failure containing %q",
			current,
			wantSummary,
		)
	}
	return current
}

func requirePromptDispatchIndeterminate(t testing.TB, err error, wantCause string) {
	t.Helper()
	if err == nil {
		t.Fatal("PromptSessionWithRuntime() error = nil, want indeterminate dispatch")
	}
	message := strings.ReplaceAll(err.Error(), `\"`, `"`)
	for _, want := range []string{"prompt session status 409", "prompt_dispatch_indeterminate", wantCause} {
		if !strings.Contains(message, want) {
			t.Fatalf("PromptSessionWithRuntime() error = %q, want %q", message, want)
		}
	}
}

func assertReasoningProtocolSequence(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
	model string,
	reasoningOptionID string,
	effort string,
) {
	t.Helper()

	if len(records) != 3 {
		t.Fatalf("protocol diagnostics = %#v, want model, reasoning, prompt", records)
	}
	want := []acpmock.DiagnosticsRecord{
		{
			ProtocolMethod:    acpsdk.AgentMethodSessionSetConfigOption,
			ConfigOptionID:    "model",
			ConfigOptionValue: model,
		},
		{
			ProtocolMethod:    acpsdk.AgentMethodSessionSetConfigOption,
			ConfigOptionID:    reasoningOptionID,
			ConfigOptionValue: effort,
		},
		{ProtocolMethod: acpsdk.AgentMethodSessionPrompt},
	}
	for index := range want {
		if records[index].ProtocolMethod != want[index].ProtocolMethod ||
			records[index].ConfigOptionID != want[index].ConfigOptionID ||
			records[index].ConfigOptionValue != want[index].ConfigOptionValue {
			t.Fatalf("protocol diagnostics[%d] = %#v, want %#v", index, records[index], want[index])
		}
	}
}

func TestDaemonE2EMockAgentsRemainIsolated(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "multi_agent_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  fixturePath,
				FixtureAgent: "alpha",
				AgentName:    "mock-alpha",
			},
			{
				FixturePath:  fixturePath,
				FixtureAgent: "beta",
				AgentName:    "mock-beta",
			},
		},
	})

	alphaReg, ok := harness.MockAgentRegistration("mock-alpha")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-alpha) = missing, want present")
	}
	betaReg, ok := harness.MockAgentRegistration("mock-beta")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-beta) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	alphaSession := createFixtureBackedSession(t, ctx, harness, "mock-alpha", "alpha-session")
	if _, err := harness.PromptSession(ctx, alphaSession.ID, "hello alpha"); err != nil {
		t.Fatalf("PromptSession(alpha) error = %v", err)
	}
	betaSession := createFixtureBackedSession(t, ctx, harness, "mock-beta", "beta-session")
	if _, err := harness.PromptSession(ctx, betaSession.ID, "hello beta"); err != nil {
		t.Fatalf("PromptSession(beta) error = %v", err)
	}

	alphaTranscript, err := harness.SessionTranscript(ctx, alphaSession.ID)
	if err != nil {
		t.Fatalf("SessionTranscript(alpha) error = %v", err)
	}
	betaTranscript, err := harness.SessionTranscript(ctx, betaSession.ID)
	if err != nil {
		t.Fatalf("SessionTranscript(beta) error = %v", err)
	}

	alphaContent := joinTranscriptContent(sessionTranscriptMessages(alphaTranscript))
	betaContent := joinTranscriptContent(sessionTranscriptMessages(betaTranscript))
	if !strings.Contains(alphaContent, "alpha says hi") || strings.Contains(alphaContent, "beta only") {
		t.Fatalf("alpha transcript = %q, want only alpha content", alphaContent)
	}
	if !strings.Contains(betaContent, "beta only") || strings.Contains(betaContent, "alpha says hi") {
		t.Fatalf("beta transcript = %q, want only beta content", betaContent)
	}

	alphaDiagnostics, err := acpmock.ReadDiagnostics(alphaReg.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(alpha) error = %v", err)
	}
	betaDiagnostics, err := acpmock.ReadDiagnostics(betaReg.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(beta) error = %v", err)
	}
	alphaPromptDiagnostics := acpmock.PromptDiagnostics(alphaDiagnostics)
	betaPromptDiagnostics := acpmock.PromptDiagnostics(betaDiagnostics)
	if len(alphaPromptDiagnostics) != 1 || alphaPromptDiagnostics[0].AgentName != "alpha" {
		t.Fatalf("alpha diagnostics = %#v, want one alpha record", alphaDiagnostics)
	}
	if len(betaPromptDiagnostics) != 1 || betaPromptDiagnostics[0].AgentName != "beta" {
		t.Fatalf("beta diagnostics = %#v, want one beta record", betaDiagnostics)
	}
}

func TestDaemonE2EToolPermissionFixtureEventsSurface(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "tool_permission_fixture.json")

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  fixturePath,
				FixtureAgent: "golden",
				AgentName:    "mock-golden",
			},
		},
	})

	registration, ok := harness.MockAgentRegistration("mock-golden")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-golden) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := createFixtureBackedSession(t, ctx, harness, "mock-golden", "golden-session")
	httpStream, err := harness.PromptSessionHTTPWithEvents(
		ctx,
		session.ID,
		"exercise golden",
		func(event e2etest.SSEEvent) error {
			requestID, ok := permissionRequestIDFromSSE(event)
			if !ok {
				return nil
			}
			return harness.ApproveSessionPermission(ctx, session.ID, compozycontract.ApproveSessionRequest{
				RequestID: requestID,
				Decision:  "allow-always",
			})
		},
	)
	if err != nil {
		t.Fatalf("PromptSessionHTTPWithEvents() error = %v", err)
	}
	if !streamContainsPermission(httpStream) {
		t.Fatalf("HTTP stream = %#v, want permission SSE event", httpStream)
	}

	eventsResp, err := harness.SessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	events := decodeAgentEvents(t, eventsResp.Events)
	if !containsAgentEvent(events, compozycontract.AgentEventPayload{
		Type:       "tool_call",
		Title:      "Inspect fixture",
		ToolCallID: "tool-1",
	}) {
		t.Fatalf("events = %#v, want tool_call event", events)
	}
	if !containsAgentEvent(events, compozycontract.AgentEventPayload{
		Type:       "tool_result",
		Title:      "Inspect fixture",
		ToolCallID: "tool-1",
	}) {
		t.Fatalf("events = %#v, want tool_result event", events)
	}
	if !containsAgentEvent(events, compozycontract.AgentEventPayload{
		Type:     "permission",
		Resource: "danger.txt",
	}) {
		t.Fatalf("events = %#v, want permission event", events)
	}
	if !containsAgentEvent(events, compozycontract.AgentEventPayload{
		Type:     "permission",
		Resource: "danger.txt",
		Decision: "allow-always",
	}) {
		t.Fatalf("events = %#v, want approved permission event", events)
	}
	if !containsAgentEvent(events, compozycontract.AgentEventPayload{
		Type: "agent_message",
		Text: "allow-always",
	}) {
		t.Fatalf("events = %#v, want allow-always assistant message", events)
	}

	if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics() error = %v", err)
	}
}

func TestDaemonE2EHostedMCPProjectsAndCallsNonBootstrapNativeTool(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should project and call a non-bootstrap native tool over hosted MCP stdio", func(t *testing.T) {
		t.Parallel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			EnableNetwork: true,
			MockAgents: []e2etest.MockAgentSpec{
				{
					FixturePath:  mockFixturePath(t, "hosted_native_tools_fixture.json"),
					FixtureAgent: "hosted-native",
					AgentName:    "mock-hosted-native",
				},
			},
		})

		registration, ok := harness.MockAgentRegistration("mock-hosted-native")
		if !ok {
			t.Fatal("MockAgentRegistration(mock-hosted-native) = missing, want present")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		channelID := "hostednative"
		if _, err := harness.CreateNetworkChannel(ctx, compozycontract.CreateNetworkChannelRequest{
			Channel:      channelID,
			WorkspaceID:  harness.WorkspaceID,
			Purpose:      "Hosted MCP native tool projection",
			FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
			AgentNames:   []string{"mock-hosted-native"},
		}); err != nil {
			t.Fatalf("CreateNetworkChannel(%q) error = %v", channelID, err)
		}
		session, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
			AgentName:            "mock-hosted-native",
			Name:                 "hosted-native-session",
			WorkspacePath:        harness.WorkspaceRoot,
			NetworkParticipation: daemonTestNamedParticipationRequest(channelID),
		})
		if err != nil {
			t.Fatalf("CreateSession(hosted-native Live) error = %v", err)
		}
		if got := resolvedParticipationChannelID(session.ResolvedNetworkParticipation); got != channelID {
			t.Fatalf(
				"hosted session ResolvedNetworkParticipation.ChannelID = %q, want %q",
				got,
				channelID,
			)
		}
		waitForRuntimeCondition(t, "hosted-native session visible", 5*time.Second, func() bool {
			current, getErr := harness.GetSession(ctx, session.ID)
			return getErr == nil && current.ID == session.ID
		})
		bindFixtureBackedSession(t, ctx, harness, session, "noop")
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(hosted-native) error = %v", err)
		}
		hostedServer := requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest)
		client := startHostedMCPClient(t, ctx, hostedServer)
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				t.Fatalf("Close(hosted MCP client) error = %v", closeErr)
			}
		}()

		list, err := client.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools(hosted MCP) error = %v", err)
		}
		networkToolID := toolspkg.ToolIDNetworkChannelCreate.String()
		if !sdkToolListContains(list.Tools, networkToolID) {
			t.Fatalf("hosted MCP tools = %#v, want non-bootstrap tool %s", sdkToolNames(list.Tools), networkToolID)
		}

		channelName := "hostednative-created"
		call := &sdkmcp.CallToolParams{Name: networkToolID, Arguments: map[string]any{
			"workspace": harness.WorkspaceID,
			"channel":   channelName,
			"purpose":   "Runtime E2E hosted native tool access",
		}}
		result, err := client.CallTool(ctx, call)
		if err != nil {
			t.Fatalf("CallTool(%s) error = %v", networkToolID, err)
		}
		if result == nil || result.IsError {
			t.Fatalf("CallTool(%s) result = %#v, want successful result", networkToolID, result)
		}
		structured, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("Marshal(CallTool structuredContent) error = %v", err)
		}
		if !strings.Contains(string(structured), channelName) {
			t.Fatalf("CallTool structuredContent = %s, want channel %q", structured, channelName)
		}

		channel, err := harness.NetworkChannel(ctx, channelName)
		if err != nil {
			t.Fatalf("NetworkChannel(%q) error = %v", channelName, err)
		}
		if channel.Channel != channelName || channel.Purpose != "Runtime E2E hosted native tool access" {
			t.Fatalf("NetworkChannel(%q) = %#v, want hosted native purpose", channelName, channel)
		}
		if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
			t.Fatalf("CaptureMockAgentDiagnostics() error = %v", err)
		}
		if err := harness.CaptureNetworkArtifacts(ctx, channelName); err != nil {
			t.Fatalf("CaptureNetworkArtifacts(%q) error = %v", channelName, err)
		}
	})

	t.Run("Should expose the real session projection to the managed provider protocol", func(t *testing.T) {
		t.Parallel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "hosted_native_tools_fixture.json"),
				FixtureAgent: "hosted-native",
				AgentName:    "mock-hosted-provider-legacy",
			}},
		})
		registration, ok := harness.MockAgentRegistration("mock-hosted-provider-legacy")
		if !ok {
			t.Fatal("MockAgentRegistration(mock-hosted-provider-legacy) = missing, want present")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		createBoundFixtureBackedSession(t, ctx, harness, "mock-hosted-provider-legacy", "hosted-provider-legacy")
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(hosted-provider-legacy) error = %v", err)
		}
		stdio := requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest)
		firstToolNames := listLegacyHostedMCPTools(
			t,
			ctx,
			stdio,
		)
		if !slices.Contains(firstToolNames, toolspkg.ToolIDTaskRunClaimNext.String()) {
			t.Fatalf(
				"first legacy hosted MCP tools = %#v, want %s",
				firstToolNames,
				toolspkg.ToolIDTaskRunClaimNext,
			)
		}
		restartedToolNames := listLegacyHostedMCPTools(t, ctx, stdio)
		if !slices.Contains(restartedToolNames, toolspkg.ToolIDTaskRunClaimNext.String()) {
			t.Fatalf(
				"restarted legacy hosted MCP tools = %#v, want %s",
				restartedToolNames,
				toolspkg.ToolIDTaskRunClaimNext,
			)
		}
	})

	t.Run("Should round trip provider model curation across CLI HTTP and hosted native tools", func(t *testing.T) {
		t.Parallel()

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "hosted_native_tools_fixture.json"),
				FixtureAgent: "hosted-native",
				AgentName:    "mock-provider-models",
			}},
		})

		registration, ok := harness.MockAgentRegistration("mock-provider-models")
		if !ok {
			t.Fatal("MockAgentRegistration(mock-provider-models) = missing, want present")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		createBoundFixtureBackedSession(t, ctx, harness, "mock-provider-models", "provider-models-session")
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(provider-models) error = %v", err)
		}
		client := startHostedMCPClient(
			t,
			ctx,
			requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest),
		)
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				t.Fatalf("Close(hosted provider-models MCP client) error = %v", closeErr)
			}
		}()

		tools, err := client.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools(hosted provider-models MCP) error = %v", err)
		}
		for _, toolID := range []toolspkg.ToolID{
			toolspkg.ToolIDProviderModelsList,
			toolspkg.ToolIDProviderModelsRefresh,
			toolspkg.ToolIDProviderModelsCurate,
		} {
			if !sdkToolListContains(tools.Tools, toolID.String()) {
				t.Fatalf("hosted MCP tools = %#v, want %s", sdkToolNames(tools.Tools), toolID)
			}
		}

		var refresh compozycontract.ProviderModelRefreshResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&refresh,
			"provider",
			"models",
			"refresh",
			"codex",
			"--source",
			"builtin",
			"--force",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI provider models refresh error = %v", err)
		}
		if len(refresh.Sources) != 1 || refresh.Sources[0].SourceID != "builtin" {
			t.Fatalf("CLI provider models refresh = %#v, want builtin status", refresh)
		}

		var hidden compozycontract.ProviderModelCurationResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&hidden,
			"provider",
			"models",
			"set",
			"codex",
			"gpt-5.6-sol",
			"--hidden=true",
			"--default-effort=max",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI provider models set error = %v", err)
		}
		if !hidden.Apply.Applied || !hidden.Model.Hidden || hidden.Model.DefaultReasoningEffort == nil ||
			*hidden.Model.DefaultReasoningEffort != compozycontract.ReasoningEffort("max") {
			t.Fatalf("CLI provider models set = %#v, want applied hidden max", hidden)
		}

		status, curatedHTTP := providerModelListHTTP(t, ctx, harness, "codex", "")
		if status != http.StatusOK {
			t.Fatalf("HTTP curated provider models status = %d, want %d", status, http.StatusOK)
		}
		if providerModelPayloadExists(curatedHTTP.Models, "gpt-5.6-sol") {
			t.Fatalf("HTTP curated provider models = %#v, want hidden Sol absent", curatedHTTP.Models)
		}
		status, allHTTP := providerModelListHTTP(t, ctx, harness, "codex", "all")
		if status != http.StatusOK {
			t.Fatalf("HTTP all provider models status = %d, want %d", status, http.StatusOK)
		}
		solHTTP := requireProviderModelPayload(t, allHTTP.Models, "gpt-5.6-sol")
		assertCanonicalHiddenSolPayload(t, solHTTP)

		var nativeAll compozycontract.ProviderModelListResponse
		callHostedMCPToolJSON(
			t,
			ctx,
			client,
			toolspkg.ToolIDProviderModelsList.String(),
			map[string]any{"provider_id": "codex", "view": "all"},
			&nativeAll,
		)
		solNative := requireProviderModelPayload(t, nativeAll.Models, "gpt-5.6-sol")
		if !reflect.DeepEqual(solNative, solHTTP) {
			t.Fatalf("native Sol = %#v, want HTTP payload %#v", solNative, solHTTP)
		}

		var unhidden compozycontract.ProviderModelCurationResponse
		callHostedMCPToolJSON(
			t,
			ctx,
			client,
			toolspkg.ToolIDProviderModelsCurate.String(),
			map[string]any{
				"provider_id": "codex",
				"model_id":    "gpt-5.6-sol",
				"hidden":      false,
			},
			&unhidden,
		)
		if !unhidden.Apply.Applied || unhidden.Model.Hidden || !unhidden.Model.Curated {
			t.Fatalf("native provider model curation = %#v, want visible curated Sol", unhidden)
		}

		var nativeCurated compozycontract.ProviderModelListResponse
		callHostedMCPToolJSON(
			t,
			ctx,
			client,
			toolspkg.ToolIDProviderModelsList.String(),
			map[string]any{"provider_id": "codex"},
			&nativeCurated,
		)
		solCuratedNative := requireProviderModelPayload(t, nativeCurated.Models, "gpt-5.6-sol")

		var cliCurated compozycontract.ProviderModelListResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&cliCurated,
			"provider",
			"models",
			"list",
			"codex",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI provider models list error = %v", err)
		}
		solCLI := requireProviderModelPayload(t, cliCurated.Models, "gpt-5.6-sol")
		if !reflect.DeepEqual(solCLI, solCuratedNative) {
			t.Fatalf("CLI Sol = %#v, want native payload %#v", solCLI, solCuratedNative)
		}
	})
}

func TestDaemonE2EWorkspaceAccessModeAndConsentMatrix(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixturePath := mockFixturePath(t, "workspace_access_fixture.json")
	agentNames := []string{
		"mock-cross-deny",
		"mock-cross-once",
		"mock-cross-session",
		"mock-cross-reject",
		"mock-cross-all",
	}
	mockAgents := make([]e2etest.MockAgentSpec, 0, len(agentNames))
	for _, agentName := range agentNames {
		fixtureAgent := "hosted-reads"
		switch agentName {
		case "mock-cross-deny":
			fixtureAgent = "hosted-deny"
		case "mock-cross-all":
			fixtureAgent = "hosted-all"
		}
		mockAgents = append(mockAgents, e2etest.MockAgentSpec{
			FixturePath:  fixturePath,
			FixtureAgent: fixtureAgent,
			AgentName:    agentName,
		})
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{MockAgents: mockAgents})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	sourceWorkspaceID := harness.WorkspaceID
	target, err := harness.ResolveWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveWorkspace(target) error = %v", err)
	}
	targetIdentity, err := workspacepkg.EnsureIdentity(ctx, target.RootDir)
	if err != nil {
		t.Fatalf("EnsureIdentity(target) error = %v", err)
	}
	harness.WorkspaceID = sourceWorkspaceID
	var (
		onceSession       compozycontract.SessionPayload
		sessionConsent    compozycontract.SessionPayload
		approveAllSession compozycontract.SessionPayload
	)

	t.Run("Should hard-deny deny-all without prompting", func(t *testing.T) {
		denySession, denyClient := newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-deny",
			"cross-deny",
		)
		denied := callWorkspaceAccessTool(t, ctx, denyClient, "cross-deny-1", target.ID)
		assertWorkspaceAccessDeniedResult(t, denied)
		assertSessionPermissionEventCount(t, ctx, harness, denySession.ID, 0)
		waitForRuntimeCondition(t, "deny-all workspace access audit", 5*time.Second, func() bool {
			return workspaceAccessAuditExists(
				t,
				ctx,
				harness,
				denySession.ID,
				targetIdentity.WorkspaceID,
				workspaceaccess.SeamTool,
				workspaceaccess.SourceDenied,
				"workspace.access_denied",
			)
		})
	})

	t.Run("Should consume allow-once consent only for one call", func(t *testing.T) {
		var onceClient *sdkmcp.ClientSession
		onceSession, onceClient = newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-once",
			"cross-once",
		)
		firstOnce := callWorkspaceAccessToolWithDecision(
			t,
			ctx,
			harness,
			onceSession.ID,
			onceClient,
			"cross-once-1",
			target.ID,
			"allow-once",
		)
		assertWorkspaceAccessAllowedResult(t, firstOnce)
		secondOnce := callWorkspaceAccessToolWithDecision(
			t,
			ctx,
			harness,
			onceSession.ID,
			onceClient,
			"cross-once-2",
			target.ID,
			"reject-once",
		)
		assertWorkspaceAccessDeniedResult(t, secondOnce)
		waitForRuntimeCondition(t, "allow-once workspace access audits", 5*time.Second, func() bool {
			return workspaceAccessAuditCount(
				t,
				ctx,
				harness,
				onceSession.ID,
				targetIdentity.WorkspaceID,
				workspaceaccess.SeamTool,
				workspaceaccess.SourceDenied,
				"workspace.access_denied",
			) == 2
		})
	})

	t.Run("Should reuse allow-session consent within one live session", func(t *testing.T) {
		var sessionClient *sdkmcp.ClientSession
		sessionConsent, sessionClient = newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-session",
			"cross-session",
		)
		allowedSession := callWorkspaceAccessToolWithDecision(
			t,
			ctx,
			harness,
			sessionConsent.ID,
			sessionClient,
			"cross-session-1",
			target.ID,
			"allow-always",
		)
		assertWorkspaceAccessAllowedResult(t, allowedSession)
		assertWorkspaceAccessAllowedResult(
			t,
			callWorkspaceAccessTool(t, ctx, sessionClient, "cross-session-2", target.ID),
		)
	})

	t.Run("Should keep rejection session-scoped and require consent in a new session", func(t *testing.T) {
		newSessionConsent, newSessionClient := newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-session",
			"cross-session-new",
		)
		newSessionDenied := callWorkspaceAccessToolWithDecision(
			t,
			ctx,
			harness,
			newSessionConsent.ID,
			newSessionClient,
			"cross-session-new-1",
			target.ID,
			"reject-once",
		)
		assertWorkspaceAccessDeniedResult(t, newSessionDenied)

		rejectSession, rejectClient := newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-reject",
			"cross-reject",
		)
		rejectedSession := callWorkspaceAccessToolWithDecision(
			t,
			ctx,
			harness,
			rejectSession.ID,
			rejectClient,
			"cross-reject-1",
			target.ID,
			"reject-always",
		)
		assertWorkspaceAccessDeniedResult(t, rejectedSession)
		assertWorkspaceAccessDeniedResult(
			t,
			callWorkspaceAccessTool(t, ctx, rejectClient, "cross-reject-2", target.ID),
		)
	})

	t.Run("Should allow approve-all without prompting", func(t *testing.T) {
		var approveAllClient *sdkmcp.ClientSession
		approveAllSession, approveAllClient = newWorkspaceAccessHostedSession(
			t,
			ctx,
			harness,
			"mock-cross-all",
			"cross-all",
		)
		assertWorkspaceAccessAllowedResult(
			t,
			callWorkspaceAccessTool(t, ctx, approveAllClient, "cross-all-1", target.ID),
		)
		assertSessionPermissionEventCount(t, ctx, harness, approveAllSession.ID, 0)
	})

	t.Run("Should preserve CLI and spawn workspace policy parity", func(t *testing.T) {
		assertWorkspaceAccessAgentCLIParity(t, ctx, harness, onceSession, approveAllSession, target.ID)
		assertWorkspaceAccessSpawnParity(t, ctx, harness, onceSession, approveAllSession, target.ID)
	})

	t.Run("Should reuse session consent at the task seam", func(t *testing.T) {
		var created compozycontract.TaskResponse
		if err := harness.UDSJSON(ctx, http.MethodPost, "/api/tasks", compozycontract.CreateTaskRequest{
			Scope:     taskpkg.ScopeWorkspace,
			Workspace: target.ID,
			Title:     "Cross seam consent reuse",
		}, &created); err != nil {
			t.Fatalf("create foreign task error = %v", err)
		}
		run := enqueueWakeTaskRunForWakeE2E(t, ctx, harness, created.Task.ID, "cross-seam-consent")
		_, claimStderr, err := harness.CLI.RunInDirWithEnv(
			ctx,
			harness.WorkspaceRoot,
			map[string]string{
				agentidentity.EnvSessionID: sessionConsent.ID,
				agentidentity.EnvAgent:     sessionConsent.AgentName,
			},
			"task",
			"next",
			"--run-id",
			run.ID,
			"--workspace",
			target.ID,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("cross-seam consent task claim error = %v; stderr=%s", err, claimStderr)
		}
		waitForRuntimeCondition(t, "task seam session consent audit", 5*time.Second, func() bool {
			return workspaceAccessAuditExists(
				t,
				ctx,
				harness,
				sessionConsent.ID,
				target.ID,
				workspaceaccess.SeamTask,
				workspaceaccess.SourceSessionConsent,
				"workspace.access_granted",
			)
		})
	})
}

func TestDaemonE2ETaskWakeCreatorDeliversSyntheticTurnAndSuppressesIneligibleWakes(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "task_wake_creator_fixture.json"),
			FixtureAgent: "wake-creator",
			AgentName:    "mock-wake-creator",
		}},
	})

	registration, ok := harness.MockAgentRegistration("mock-wake-creator")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-wake-creator) = missing, want present")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	creatorSession := createBoundFixtureBackedSession(t, ctx, harness, "mock-wake-creator", "wake-creator-session")
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(wake-creator) error = %v", err)
	}
	hostedServer := requireHostedMCPStdioServer(t, diagnostics, hostedMCPServerEarliest)
	client := startHostedMCPClient(t, ctx, hostedServer)
	clientClosed := false
	defer func() {
		if clientClosed {
			return
		}
		if closeErr := client.Close(); closeErr != nil {
			t.Logf("Close(hosted MCP client) after failed test error = %v", closeErr)
		}
	}()

	terminalTaskID := createHostedTaskForWakeE2E(
		t,
		ctx,
		client,
		"task-wake-e2e-terminal",
		"Wake terminal child",
	)
	optOutTaskID := createHostedTaskForWakeE2E(
		t,
		ctx,
		client,
		"task-wake-e2e-opt-out",
		"Wake opt-out child",
	)
	selfWakeTaskID := createHostedTaskForWakeE2E(
		t,
		ctx,
		client,
		"task-wake-e2e-self",
		"Wake self child",
	)
	if closeErr := client.Close(); closeErr != nil {
		t.Fatalf("Close(hosted MCP client) error = %v", closeErr)
	}
	clientClosed = true

	t.Run("Should deliver synthetic wake for eligible terminal task", func(t *testing.T) {
		terminalExecutor := createFixtureBackedSession(
			t,
			ctx,
			harness,
			"mock-wake-creator",
			"wake-terminal-executor",
		)
		terminalRun := completeWakeTaskRunViaSession(
			t,
			ctx,
			harness,
			terminalTaskID,
			terminalExecutor.ID,
			"terminal",
		)
		terminalWake := waitForSyntheticWakePrompt(
			t,
			registration.DiagnosticsPath,
			terminalTaskID,
			terminalRun.ID,
			5*time.Second,
		)
		if strings.TrimSpace(creatorSession.Runtime.ACPSessionID) == "" {
			t.Fatalf("creator session ACPSessionID is empty: %#v", creatorSession)
		}
		if got, want := terminalWake.SessionID, creatorSession.Runtime.ACPSessionID; got != want {
			t.Fatalf("terminal wake ACP session = %q, want creator ACP session %q", got, want)
		}
		terminalMeta := terminalWake.PromptMeta.Normalize()
		if terminalMeta.Synthetic == nil {
			t.Fatal("terminal wake synthetic metadata = nil")
		}
		if got, want := terminalMeta.Synthetic.Reason, string(taskpkg.WakeReasonTerminal); got != want {
			t.Fatalf("terminal wake reason = %q, want %q", got, want)
		}
		if strings.TrimSpace(terminalMeta.Synthetic.WakeEventID) == "" {
			t.Fatal("terminal wake WakeEventID is empty")
		}
		if !diagnosticStepsContainText(terminalWake.Steps, "wake observed") {
			t.Fatalf("terminal wake steps = %#v, want fixture assistant observation", terminalWake.Steps)
		}
	})

	t.Run("Should suppress wakes when wake creator is disabled", func(t *testing.T) {
		disableTaskWakeCreatorForWakeE2E(t, ctx, harness, optOutTaskID)
		optOutExecutor := createFixtureBackedSession(
			t,
			ctx,
			harness,
			"mock-wake-creator",
			"wake-opt-out-executor",
		)
		optOutRun := completeWakeTaskRunViaSession(
			t,
			ctx,
			harness,
			optOutTaskID,
			optOutExecutor.ID,
			"opt-out",
		)
		assertNoSyntheticWakePromptWithin(
			t,
			registration.DiagnosticsPath,
			optOutTaskID,
			optOutRun.ID,
			500*time.Millisecond,
		)
		assertTaskWakeSuppressedReason(t, ctx, harness, optOutTaskID, "wake_creator_disabled")
	})

	t.Run("Should suppress self wakes", func(t *testing.T) {
		selfWakeRun := completeWakeTaskRunViaSession(
			t,
			ctx,
			harness,
			selfWakeTaskID,
			creatorSession.ID,
			"self",
		)
		assertNoSyntheticWakePromptWithin(
			t,
			registration.DiagnosticsPath,
			selfWakeTaskID,
			selfWakeRun.ID,
			500*time.Millisecond,
		)
		assertTaskWakeSuppressedReason(t, ctx, harness, selfWakeTaskID, "self_wake")
	})

	if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics() error = %v", err)
	}
}

func newWorkspaceAccessHostedSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	agentName string,
	name string,
) (compozycontract.SessionPayload, *sdkmcp.ClientSession) {
	t.Helper()

	active := createBoundFixtureBackedSession(t, ctx, harness, agentName, name)
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) = missing", agentName)
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", agentName, err)
	}
	sessionDiagnostics := acpmock.DiagnosticsForCompozySession(diagnostics, active.ID)
	client := startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(t, sessionDiagnostics, hostedMCPServerLatest),
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close(hosted workspace-access MCP client) error = %v", err)
		}
	})
	return active, client
}

func callWorkspaceAccessTool(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	toolCallID string,
	targetWorkspaceID string,
) *sdkmcp.CallToolResult {
	t.Helper()
	return callHostedTool(
		t,
		ctx,
		client,
		toolspkg.ToolIDWorkspaceInfo,
		toolCallID,
		map[string]any{"workspace": strings.TrimSpace(targetWorkspaceID)},
	)
}

func callHostedTool(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	toolID toolspkg.ToolID,
	toolCallID string,
	arguments map[string]any,
) *sdkmcp.CallToolResult {
	t.Helper()

	call := &sdkmcp.CallToolParams{
		Meta:      sdkmcp.Meta{"toolCallId": toolCallID},
		Name:      toolID.String(),
		Arguments: arguments,
	}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolID, err)
	}
	return result
}

func callWorkspaceAccessToolWithDecision(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	client *sdkmcp.ClientSession,
	toolCallID string,
	targetWorkspaceID string,
	decision string,
) *sdkmcp.CallToolResult {
	t.Helper()

	type callOutcome struct {
		result *sdkmcp.CallToolResult
		err    error
	}
	outcomeCh := make(chan callOutcome, 1)
	go func() {
		call := &sdkmcp.CallToolParams{
			Meta:      sdkmcp.Meta{"toolCallId": toolCallID},
			Name:      toolspkg.ToolIDWorkspaceInfo.String(),
			Arguments: map[string]any{"workspace": strings.TrimSpace(targetWorkspaceID)},
		}
		result, err := client.CallTool(ctx, call)
		outcomeCh <- callOutcome{result: result, err: err}
	}()
	approved := false
	var lastApprovalErr error
	promptDeadline := time.NewTimer(5 * time.Second)
	defer promptDeadline.Stop()
	promptPoll := time.NewTicker(25 * time.Millisecond)
	defer promptPoll.Stop()
	for !approved {
		select {
		case outcome := <-outcomeCh:
			raw, marshalErr := json.Marshal(outcome.result)
			if marshalErr != nil {
				raw = []byte("<unavailable>")
			}
			t.Fatalf(
				"workspace access call returned before prompt: result=%s err=%v last_approval_error=%v",
				raw,
				outcome.err,
				lastApprovalErr,
			)
		case <-promptDeadline.C:
			t.Fatal("timed out waiting for workspace access permission event")
		case <-ctx.Done():
			t.Fatalf("workspace access permission context ended: %v", ctx.Err())
		case <-promptPoll.C:
		}
		lastApprovalErr = harness.ApproveSessionPermission(ctx, sessionID, compozycontract.ApproveSessionRequest{
			RequestID: toolCallID,
			Decision:  decision,
		})
		approved = lastApprovalErr == nil
	}
	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("CallTool(%s) error = %v", toolspkg.ToolIDWorkspaceInfo, outcome.err)
		}
		return outcome.result
	case <-ctx.Done():
		t.Fatalf("CallTool(%s) did not complete: %v", toolspkg.ToolIDWorkspaceInfo, ctx.Err())
		return nil
	}
}

func assertWorkspaceAccessAllowedResult(t testing.TB, result *sdkmcp.CallToolResult) {
	t.Helper()
	if result == nil || result.IsError {
		t.Fatalf("workspace access result = %#v, want success", result)
	}
}

func assertWorkspaceAccessDeniedResult(t testing.TB, result *sdkmcp.CallToolResult) {
	t.Helper()
	if result == nil || !result.IsError {
		t.Fatalf("workspace access result = %#v, want denied tool result", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("workspace access result content = %#v, want one error text item", result.Content)
	}
	content, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("workspace access result content[0] = %T, want *sdkmcp.TextContent", result.Content[0])
	}
	if !strings.Contains(content.Text, string(toolspkg.ReasonWorkspaceAccessDenied)) ||
		!strings.Contains(content.Text, "tool invocation denied") {
		t.Fatalf("workspace access result text = %q, want canonical denial", content.Text)
	}
	if strings.Contains(content.Text, workspaceaccess.DenialHint) {
		t.Fatalf("workspace access result text = %q, want internal denial hint redacted", content.Text)
	}
}

func assertSessionPermissionEventCount(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	want int,
) {
	t.Helper()
	if want > 0 {
		waitForRuntimeCondition(t, "session permission event count", 5*time.Second, func() bool {
			return sessionPermissionEventCount(t, ctx, harness, sessionID) == want
		})
		return
	}
	count := sessionPermissionEventCount(t, ctx, harness, sessionID)
	if count != want {
		t.Fatalf("session %q permission event count = %d, want %d", sessionID, count, want)
	}
}

func sessionPermissionEventCount(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) int {
	t.Helper()

	response, err := harness.SessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionEvents(%q) error = %v", sessionID, err)
	}
	count := 0
	for _, event := range decodeAgentEvents(t, response.Events) {
		if event.Type == "permission" {
			count++
		}
	}
	return count
}

func workspaceAccessAuditExists(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	targetWorkspaceID string,
	seam workspaceaccess.Seam,
	source workspaceaccess.DecisionSource,
	eventType string,
) bool {
	t.Helper()
	return workspaceAccessAuditCount(
		t,
		ctx,
		harness,
		sessionID,
		targetWorkspaceID,
		seam,
		source,
		eventType,
	) > 0
}

func workspaceAccessAuditCount(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	targetWorkspaceID string,
	seam workspaceaccess.Seam,
	source workspaceaccess.DecisionSource,
	eventType string,
) int {
	t.Helper()

	path := "/api/logs?workspace_id=" + url.QueryEscape(harness.WorkspaceID) +
		"&type=" + url.QueryEscape(eventType) + "&limit=100"
	var logs compozycontract.LogsListResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &logs); err != nil {
		t.Fatalf("UDSJSON(workspace access logs) error = %v", err)
	}
	count := 0
	for _, event := range logs.Events {
		if event.SessionID != sessionID || event.Type != eventType {
			continue
		}
		var payload workspaceAccessAuditPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("decode matching workspace access audit event %q: %v", event.Content, err)
		}
		if payload.TargetWorkspaceID == targetWorkspaceID && payload.Seam == seam && payload.Source == source {
			count++
		}
	}
	return count
}

func assertWorkspaceAccessAgentCLIParity(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	approveReads compozycontract.SessionPayload,
	approveAll compozycontract.SessionPayload,
	targetWorkspaceID string,
) {
	t.Helper()

	readEnv := map[string]string{
		agentidentity.EnvSessionID: approveReads.ID,
		agentidentity.EnvAgent:     approveReads.AgentName,
	}
	_, deniedStderr, err := harness.CLI.RunInDirWithEnv(
		ctx,
		harness.WorkspaceRoot,
		readEnv,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		targetWorkspaceID,
		"--title",
		"Cross-workspace CLI deny parity",
		"--as-agent",
		"-o",
		"json",
	)
	exitErr, exitErrMatched := errors.AsType[*exec.ExitError](err)
	if !exitErrMatched || exitErr.ExitCode() != agentidentity.ExitUnauthorized {
		t.Fatalf(
			"approve-reads CLI error = %v, want exit %d; stderr=%s",
			err,
			agentidentity.ExitUnauthorized,
			deniedStderr,
		)
	}
	if !strings.Contains(deniedStderr, workspaceaccess.DenialHint) {
		t.Fatalf("approve-reads CLI stderr = %q, want denial hint", deniedStderr)
	}

	allEnv := map[string]string{
		agentidentity.EnvSessionID: approveAll.ID,
		agentidentity.EnvAgent:     approveAll.AgentName,
	}
	stdout, stderr, err := harness.CLI.RunInDirWithEnv(
		ctx,
		harness.WorkspaceRoot,
		allEnv,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		targetWorkspaceID,
		"--title",
		"Cross-workspace CLI allow parity",
		"--as-agent",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("approve-all CLI error = %v; stderr=%s", err, stderr)
	}
	var payload struct {
		ResolutionSource string `json:"resolution_source"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(approve-all CLI) error = %v; stdout=%s", err, stdout)
	}
	if payload.ResolutionSource != "cross_workspace_attempt" {
		t.Fatalf("approve-all CLI resolution_source = %q, want cross_workspace_attempt", payload.ResolutionSource)
	}
}

func assertWorkspaceAccessSpawnParity(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	approveReads compozycontract.SessionPayload,
	approveAll compozycontract.SessionPayload,
	targetWorkspaceID string,
) {
	t.Helper()

	readEnv := map[string]string{
		agentidentity.EnvSessionID: approveReads.ID,
		agentidentity.EnvAgent:     approveReads.AgentName,
	}
	promptCountBefore := sessionPermissionEventCount(t, ctx, harness, approveReads.ID)
	_, deniedStderr, err := harness.CLI.RunInDirWithEnv(
		ctx,
		harness.WorkspaceRoot,
		readEnv,
		"spawn",
		"--agent",
		approveReads.AgentName,
		"--workspace",
		targetWorkspaceID,
		"--ttl-seconds",
		"60",
		"--idempotency-key",
		"cross-workspace-spawn-denied",
		"-o",
		"json",
	)
	if err == nil {
		t.Fatalf("approve-reads cross-workspace spawn error = nil, want denial")
	}
	exitErr, exitErrMatched := errors.AsType[*exec.ExitError](err)
	if !exitErrMatched || exitErr.ExitCode() != agentidentity.ExitUnauthorized {
		t.Fatalf(
			"approve-reads cross-workspace spawn error = %v, want exit %d; stderr=%s",
			err,
			agentidentity.ExitUnauthorized,
			deniedStderr,
		)
	}
	if !strings.Contains(deniedStderr, workspaceaccess.DenialHint) {
		t.Fatalf("approve-reads cross-workspace spawn stderr = %q, want denial hint", deniedStderr)
	}
	assertSessionPermissionEventCount(t, ctx, harness, approveReads.ID, promptCountBefore)

	allEnv := map[string]string{
		agentidentity.EnvSessionID: approveAll.ID,
		agentidentity.EnvAgent:     approveAll.AgentName,
	}
	stdout, stderr, err := harness.CLI.RunInDirWithEnv(
		ctx,
		harness.WorkspaceRoot,
		allEnv,
		"spawn",
		"--agent",
		approveAll.AgentName,
		"--workspace",
		targetWorkspaceID,
		"--ttl-seconds",
		"60",
		"--idempotency-key",
		"cross-workspace-spawn-allowed",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("approve-all cross-workspace spawn error = %v; stderr=%s", err, stderr)
	}
	var payload struct {
		Session struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
		} `json:"session"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(approve-all spawn output) error = %v; stdout=%s", err, stdout)
	}
	if payload.Session.ID == "" || payload.Session.WorkspaceID != targetWorkspaceID {
		t.Fatalf("approve-all spawn output = %#v, want child in workspace %q", payload, targetWorkspaceID)
	}
}

type hostedMCPServerSelection string

const (
	hostedMCPServerEarliest hostedMCPServerSelection = "earliest"
	hostedMCPServerLatest   hostedMCPServerSelection = "latest"
)

func requireHostedMCPStdioServer(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
	selection hostedMCPServerSelection,
) acpsdk.McpServerStdio {
	t.Helper()

	start, end, step := 0, len(records), 1
	if selection == hostedMCPServerLatest {
		start, end, step = len(records)-1, -1, -1
	} else if selection != hostedMCPServerEarliest {
		t.Fatalf("hosted MCP server selection = %q, want earliest or latest", selection)
	}
	for index := start; index != end; index += step {
		record := records[index]
		if record.LifecycleEvent != "session_new" {
			continue
		}
		for _, server := range record.MCPServers {
			if server.Stdio == nil || server.Stdio.Name != mcppkg.HostedServerName {
				continue
			}
			return *server.Stdio
		}
	}
	t.Fatalf(
		"diagnostics = %#v, want %s session_new %s stdio MCP server",
		records,
		selection,
		mcppkg.HostedServerName,
	)
	return acpsdk.McpServerStdio{}
}

func startHostedMCPClient(
	t testing.TB,
	ctx context.Context,
	stdio acpsdk.McpServerStdio,
) *sdkmcp.ClientSession {
	t.Helper()

	if strings.TrimSpace(stdio.Command) == "" {
		t.Fatalf("hosted MCP stdio server = %#v, want command", stdio)
	}
	command := exec.CommandContext(ctx, stdio.Command, stdio.Args...)
	command.Env = append(os.Environ(), hostedMCPStdioEnv(stdio)...)
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "compozy-hosted-e2e", Version: "1.0.0"},
		&sdkmcp.ClientOptions{Capabilities: &sdkmcp.ClientCapabilities{}},
	)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatalf("Connect(hosted MCP %q) error = %v", stdio.Command, err)
	}
	return session
}

func listLegacyHostedMCPTools(
	t testing.TB,
	ctx context.Context,
	stdio acpsdk.McpServerStdio,
) []string {
	t.Helper()

	if strings.TrimSpace(stdio.Command) == "" {
		t.Fatalf("hosted MCP stdio server = %#v, want command", stdio)
	}
	command := exec.CommandContext(ctx, stdio.Command, stdio.Args...)
	command.Env = append(os.Environ(), hostedMCPStdioEnv(stdio)...)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe(hosted MCP) error = %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe(hosted MCP) error = %v", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("Start(hosted MCP %q) error = %v", stdio.Command, err)
	}
	waited := false
	t.Cleanup(func() {
		if waited {
			return
		}
		if err := stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("Close(hosted MCP stdin) error = %v", err)
		}
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("Kill(hosted MCP process) error = %v", err)
		}
		if err := command.Wait(); err != nil && !strings.Contains(stderr.String(), "EOF") {
			t.Errorf("Wait(hosted MCP process) error = %v; stderr = %s", err, stderr.String())
		}
	})

	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(stdout)
	if err := encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"elicitation": map[string]any{
					"form": map[string]any{},
					"url":  map[string]any{},
				},
			},
			"clientInfo": map[string]string{
				"name":    "codex-mcp-client",
				"title":   "Codex",
				"version": "0.146.0",
			},
		},
	}); err != nil {
		t.Fatalf("Encode(hosted MCP initialize) error = %v", err)
	}
	var initialized struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := decoder.Decode(&initialized); err != nil {
		t.Fatalf("Decode(hosted MCP initialize) error = %v; stderr = %s", err, stderr.String())
	}
	if len(initialized.Error) > 0 && string(initialized.Error) != "null" {
		t.Fatalf("hosted MCP initialize error = %s", initialized.Error)
	}
	if initialized.Result.ProtocolVersion != "2025-06-18" {
		t.Fatalf(
			"hosted MCP protocol version = %q, want 2025-06-18",
			initialized.Result.ProtocolVersion,
		)
	}
	if err := encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); err != nil {
		t.Fatalf("Encode(hosted MCP initialized notification) error = %v", err)
	}
	if err := encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("Encode(hosted MCP tools/list) error = %v", err)
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := decoder.Decode(&listed); err != nil {
		t.Fatalf("Decode(hosted MCP tools/list) error = %v; stderr = %s", err, stderr.String())
	}
	if len(listed.Error) > 0 && string(listed.Error) != "null" {
		t.Fatalf("hosted MCP tools/list error = %s", listed.Error)
	}

	toolNames := make([]string, 0, len(listed.Result.Tools))
	for _, tool := range listed.Result.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("Close(hosted MCP stdin) error = %v", err)
	}
	if err := command.Wait(); err != nil && !strings.Contains(stderr.String(), "EOF") {
		t.Fatalf("Wait(hosted MCP process) error = %v; stderr = %s", err, stderr.String())
	}
	waited = true
	return toolNames
}

func hostedMCPStdioEnv(stdio acpsdk.McpServerStdio) []string {
	env := make([]string, 0, len(stdio.Env))
	for _, entry := range stdio.Env {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		env = append(env, name+"="+entry.Value)
	}
	return env
}

func sdkToolListContains(tools []*sdkmcp.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func sdkToolNames(tools []*sdkmcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func callHostedMCPToolJSON(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	toolID string,
	arguments map[string]any,
	destination any,
) {
	t.Helper()

	call := &sdkmcp.CallToolParams{Name: toolID, Arguments: arguments}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolID, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want successful result", toolID, result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(CallTool(%s) structured content) error = %v", toolID, err)
	}
	if err := json.Unmarshal(payload, destination); err != nil {
		t.Fatalf("Unmarshal(CallTool(%s) structured content) error = %v; payload=%s", toolID, err, payload)
	}
}

func providerModelPayloadExists(models []compozycontract.ProviderModelPayload, modelID string) bool {
	for _, model := range models {
		if model.ModelID == modelID {
			return true
		}
	}
	return false
}

func requireProviderModelPayload(
	t testing.TB,
	models []compozycontract.ProviderModelPayload,
	modelID string,
) compozycontract.ProviderModelPayload {
	t.Helper()
	for _, model := range models {
		if model.ModelID == modelID {
			return model
		}
	}
	t.Fatalf("provider model %q not found in %#v", modelID, models)
	return compozycontract.ProviderModelPayload{}
}

func assertCanonicalHiddenSolPayload(t testing.TB, model compozycontract.ProviderModelPayload) {
	t.Helper()

	wantEfforts := []compozycontract.ReasoningEffort{
		compozycontract.ReasoningEffort("none"),
		compozycontract.ReasoningEffort("low"),
		compozycontract.ReasoningEffort("medium"),
		compozycontract.ReasoningEffort("high"),
		compozycontract.ReasoningEffort("xhigh"),
		compozycontract.ReasoningEffort("max"),
	}
	if model.ProviderID != "codex" || model.ModelID != "gpt-5.6-sol" || model.Curated ||
		!model.Hidden || model.Deprecated || !model.Featured || model.ReleaseDate != "2026-06-26" ||
		model.ReasoningSource != compozycontract.ReasoningSource("catalog") {
		t.Fatalf("Sol identity/curation payload = %#v", model)
	}
	if model.ContextWindow == nil || *model.ContextWindow != 1_050_000 ||
		model.MaxOutputTokens == nil || *model.MaxOutputTokens != 128_000 ||
		model.SupportsTools == nil || !*model.SupportsTools ||
		model.SupportsReasoning == nil || !*model.SupportsReasoning {
		t.Fatalf("Sol capability payload = %#v", model)
	}
	if !reflect.DeepEqual(model.ReasoningEfforts, wantEfforts) ||
		model.DefaultReasoningEffort == nil || *model.DefaultReasoningEffort != compozycontract.ReasoningEffort("max") {
		t.Fatalf("Sol reasoning payload = %#v, want efforts %#v and default max", model, wantEfforts)
	}
	if model.Cost == nil || model.Cost.InputPerMillion == nil || *model.Cost.InputPerMillion != 5 ||
		model.Cost.OutputPerMillion == nil || *model.Cost.OutputPerMillion != 30 {
		t.Fatalf("Sol cost payload = %#v, want 5/30", model.Cost)
	}
}

func createHostedTaskForWakeE2E(
	t testing.TB,
	ctx context.Context,
	client *sdkmcp.ClientSession,
	taskID string,
	title string,
) string {
	t.Helper()

	call := &sdkmcp.CallToolParams{Name: toolspkg.ToolIDTaskCreate.String(), Arguments: map[string]any{
		"id":    strings.TrimSpace(taskID),
		"scope": string(taskpkg.ScopeWorkspace),
		"title": strings.TrimSpace(title),
	}}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", call.Name, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want successful result", call.Name, result)
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(CallTool structuredContent) error = %v", err)
	}
	var payload struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(structured, &payload); err != nil {
		t.Fatalf("Unmarshal(task_create structuredContent) error = %v; content=%s", err, structured)
	}
	if got, want := payload.Task.ID, strings.TrimSpace(taskID); got != want {
		t.Fatalf("task_create id = %q, want %q; content=%s", got, want, structured)
	}
	return payload.Task.ID
}

func completeWakeTaskRunViaSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	sessionID string,
	idempotencyPrefix string,
) compozycontract.TaskRunPayload {
	t.Helper()

	run := enqueueWakeTaskRunForWakeE2E(t, ctx, harness, taskID, idempotencyPrefix+"-enqueue")
	sessionRecord, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%s) error = %v", sessionID, err)
	}
	claimed, err := harness.ClaimExactTaskRunForSession(ctx, run.ID, sessionRecord)
	if err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", run.ID, err)
	}
	var completedLease compozycontract.AgentTaskLeaseResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		sessionRecord,
		http.MethodPost,
		"/api/agent/tasks/"+url.PathEscape(claimed.ID)+"/complete",
		compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
		&completedLease,
	)
	if completedLease.Lease.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("agent complete lease = %#v, want completed", completedLease.Lease)
	}
	runs, err := harness.ListTaskRuns(ctx, taskID, nil)
	if err != nil {
		t.Fatalf("ListTaskRuns(%s) error = %v", taskID, err)
	}
	for _, completed := range runs {
		if completed.ID == claimed.ID {
			return completed
		}
	}
	t.Fatalf("ListTaskRuns(%s) missing completed run %s", taskID, claimed.ID)
	return compozycontract.TaskRunPayload{}
}

func enqueueWakeTaskRunForWakeE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	idempotencyKey string,
) compozycontract.TaskRunPayload {
	t.Helper()

	var response compozycontract.TaskRunResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID)) + "/runs"
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		path,
		compozycontract.EnqueueTaskRunRequest{IdempotencyKey: strings.TrimSpace(idempotencyKey)},
		&response,
	); err != nil {
		t.Fatalf("enqueue task run %q error = %v", taskID, err)
	}
	return response.Run
}

func attachWakeTaskRunSessionForWakeE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	sessionID string,
) compozycontract.TaskRunPayload {
	t.Helper()

	var response compozycontract.TaskRunResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/attach-session"
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		path,
		compozycontract.AttachTaskRunSessionRequest{SessionID: strings.TrimSpace(sessionID)},
		&response,
	); err != nil {
		t.Fatalf("attach task run session %q/%q error = %v", runID, sessionID, err)
	}
	return response.Run
}

func disableTaskWakeCreatorForWakeE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
) {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(ctx, harness.HomePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(%q) error = %v", harness.HomePaths.DatabaseFile, err)
	}
	defer func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			t.Fatalf("Close(global db) error = %v", closeErr)
		}
	}()
	if _, err := db.SetTaskWakeCreator(ctx, taskpkg.WakeCreatorMutation{
		TaskID:      strings.TrimSpace(taskID),
		WakeCreator: false,
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetTaskWakeCreator(%q) error = %v", taskID, err)
	}
}

func waitForSyntheticWakePrompt(
	t testing.TB,
	diagnosticsPath string,
	taskID string,
	runID string,
	timeout time.Duration,
) acpmock.DiagnosticsRecord {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var lastReadErr error
	for {
		records, err := acpmock.ReadDiagnostics(diagnosticsPath)
		if err != nil {
			lastReadErr = err
		} else {
			for _, record := range acpmock.PromptDiagnostics(records) {
				if wakePromptMatches(record, taskID, runID) {
					return record
				}
			}
		}
		select {
		case <-timer.C:
			t.Fatalf(
				"timed out waiting for synthetic wake prompt task=%q run=%q diagnostics=%q last_read_error=%v",
				taskID,
				runID,
				diagnosticsPath,
				lastReadErr,
			)
		case <-ticker.C:
		}
	}
}

func assertNoSyntheticWakePromptWithin(
	t testing.TB,
	diagnosticsPath string,
	taskID string,
	runID string,
	timeout time.Duration,
) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		records, err := acpmock.ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(%q) error = %v", diagnosticsPath, err)
		}
		for _, record := range acpmock.PromptDiagnostics(records) {
			if wakePromptMatches(record, taskID, runID) {
				t.Fatalf("unexpected synthetic wake prompt task=%q run=%q record=%#v", taskID, runID, record)
			}
		}
		select {
		case <-timer.C:
			return
		case <-ticker.C:
		}
	}
}

func wakePromptMatches(record acpmock.DiagnosticsRecord, taskID string, runID string) bool {
	meta := record.PromptMeta.Normalize()
	if meta.TurnSource != acp.PromptTurnSourceSynthetic || meta.Synthetic == nil {
		return false
	}
	if strings.TrimSpace(meta.Synthetic.TaskID) != strings.TrimSpace(taskID) {
		return false
	}
	if strings.TrimSpace(runID) != "" && strings.TrimSpace(meta.Synthetic.TaskRunID) != strings.TrimSpace(runID) {
		return false
	}
	return strings.TrimSpace(meta.Synthetic.WakeEventID) != ""
}

func diagnosticStepsContainText(steps []acpmock.DiagnosticsStep, text string) bool {
	for _, step := range steps {
		if strings.Contains(step.Text, text) {
			return true
		}
	}
	return false
}

func assertTaskWakeSuppressedReason(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	reason string,
) {
	t.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		if taskTimelineHasWakeSuppression(t, ctx, harness, taskID, reason) {
			return
		}
		select {
		case <-timer.C:
			t.Fatalf("timed out waiting for wake suppression task=%q reason=%q", taskID, reason)
		case <-ticker.C:
		}
	}
}

func taskTimelineHasWakeSuppression(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	reason string,
) bool {
	t.Helper()

	var response compozycontract.TaskTimelineResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID)) + "/timeline"
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("task timeline %q error = %v", taskID, err)
	}
	for _, item := range response.Timeline {
		if item.EventType != eventspkg.TaskWakeSuppressed {
			continue
		}
		var payload struct {
			SuppressionReason string `json:"suppression_reason"`
		}
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal(wake suppression payload) error = %v; payload=%s", err, item.Payload)
		}
		if payload.SuppressionReason == reason {
			return true
		}
	}
	return false
}

func permissionRequestIDFromSSE(event e2etest.SSEEvent) (string, bool) {
	if event.Event != "permission" || len(event.Data) == 0 {
		return "", false
	}

	var envelope struct {
		Type string `json:"type"`
		Data struct {
			RequestID string `json:"request_id"`
			Decision  string `json:"decision,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(event.Data, &envelope); err != nil {
		return "", false
	}
	if envelope.Type != "data-compozy-permission" || envelope.Data.Decision != "" || envelope.Data.RequestID == "" {
		return "", false
	}
	return envelope.Data.RequestID, true
}

func streamContainsPermission(events []e2etest.SSEEvent) bool {
	for _, event := range events {
		if event.Event == "permission" {
			return true
		}
	}
	return false
}

func decodeAgentEvents(
	t testing.TB,
	events []compozycontract.SessionEventPayload,
) []compozycontract.AgentEventPayload {
	t.Helper()

	decoded := make([]compozycontract.AgentEventPayload, 0, len(events))
	for _, event := range events {
		var payload compozycontract.AgentEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(session event %q) error = %v", event.ID, err)
		}
		decoded = append(decoded, payload)
	}
	return decoded
}

func containsAgentEvent(events []compozycontract.AgentEventPayload, want compozycontract.AgentEventPayload) bool {
	for _, event := range events {
		if want.Type != "" && event.Type != want.Type {
			continue
		}
		if want.Text != "" && event.Text != want.Text {
			continue
		}
		if want.Title != "" && event.Title != want.Title {
			continue
		}
		if want.ToolCallID != "" && event.ToolCallID != want.ToolCallID {
			continue
		}
		if want.Resource != "" && event.Resource != want.Resource {
			continue
		}
		if want.Decision != "" && event.Decision != want.Decision {
			continue
		}
		return true
	}
	return false
}
