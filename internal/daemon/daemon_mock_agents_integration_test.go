//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	toolspkg "github.com/compozy/agh/internal/tools"
	mcpclient "github.com/mark3labs/mcp-go/client"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
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
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *aghconfig.Config) {
			for _, providerName := range []string{"claude", "codex"} {
				provider := cfg.Providers[providerName]
				provider.AuthMode = aghconfig.ProviderAuthModeNone
				provider.NoneSecurity = aghconfig.ProviderNoneSecurityLocalTransport
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
		sessionPayload, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
			AgentName:     "reasoning-claude-agent-default",
			WorkspacePath: harness.WorkspaceRoot,
		})
		if err != nil {
			t.Fatalf("CreateSession(agent default) error = %v", err)
		}
		sessionPayload = waitForReasoningSessionActive(t, ctx, harness, sessionPayload)
		if sessionPayload.Provider != "claude" || sessionPayload.Model != "claude-sonnet-5" ||
			sessionPayload.ReasoningEffort != aghcontract.ReasoningEffort("max") {
			t.Fatalf("session runtime = %#v, want claude/claude-sonnet-5/max", sessionPayload)
		}
		if _, err := harness.PromptSession(ctx, sessionPayload.ID, "claude max"); err != nil {
			t.Fatalf("PromptSession(agent default) error = %v", err)
		}
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(agent default) error = %v", err)
		}
		assertReasoningProtocolSequence(
			t,
			acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForAGHSession(records, sessionPayload.ID)),
			"sonnet",
			"effort",
			"max",
		)
	})

	positiveCases := []struct {
		name              string
		agentName         string
		provider          string
		model             string
		transportModel    string
		effort            aghcontract.ReasoningEffort
		prompt            string
		reasoningOptionID string
	}{
		{
			name:              "Should apply Claude max after the model and before the first prompt",
			agentName:         "reasoning-claude-max",
			provider:          "claude",
			model:             "claude-sonnet-5",
			transportModel:    "sonnet",
			effort:            "max",
			prompt:            "claude max",
			reasoningOptionID: "effort",
		},
		{
			name:              "Should apply Codex max after the model and before the first prompt",
			agentName:         "reasoning-codex-max",
			provider:          "codex",
			model:             "gpt-5.6-sol",
			transportModel:    "gpt-5.6-sol",
			effort:            "max",
			prompt:            "codex max",
			reasoningOptionID: "reasoning_effort",
		},
		{
			name:              "Should send explicit Codex none instead of treating it as provider default",
			agentName:         "reasoning-codex-none",
			provider:          "codex",
			model:             "gpt-5.6-sol",
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
			sessionPayload, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
				AgentName:       tt.agentName,
				Provider:        tt.provider,
				Model:           tt.model,
				ReasoningEffort: tt.effort,
				WorkspacePath:   harness.WorkspaceRoot,
			})
			if err != nil {
				t.Fatalf("CreateSession(%q) error = %v", tt.agentName, err)
			}
			sessionPayload = waitForReasoningSessionActive(t, ctx, harness, sessionPayload)
			if sessionPayload.Provider != tt.provider || sessionPayload.Model != tt.model ||
				sessionPayload.ReasoningEffort != tt.effort {
				t.Fatalf("session runtime = %#v, want %s/%s/%s", sessionPayload, tt.provider, tt.model, tt.effort)
			}
			if _, err := harness.PromptSession(ctx, sessionPayload.ID, tt.prompt); err != nil {
				t.Fatalf("PromptSession(%q) error = %v", tt.agentName, err)
			}
			records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
			if err != nil {
				t.Fatalf("ReadDiagnostics(%q) error = %v", tt.agentName, err)
			}
			assertReasoningProtocolSequence(
				t,
				acpmock.ProtocolDiagnostics(acpmock.DiagnosticsForAGHSession(records, sessionPayload.ID)),
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

		sessions := make([]aghcontract.SessionPayload, 0, 2)
		for index := 0; index < 2; index++ {
			sessionPayload, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
				AgentName:       "reasoning-claude-concurrent",
				Provider:        "claude",
				Model:           "claude-sonnet-5",
				ReasoningEffort: "max",
				WorkspacePath:   harness.WorkspaceRoot,
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
			t.Fatalf("concurrent AGH session IDs = %q, want distinct owners", sessions[0].ID)
		}

		promptResults := make(chan error, len(sessions))
		for _, sessionPayload := range sessions {
			sessionID := sessionPayload.ID
			go func() {
				_, err := harness.PromptSession(ctx, sessionID, "claude max")
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
			if strings.TrimSpace(record.AGHSessionID) == "" {
				t.Fatalf("diagnostics[%d] = %#v, want a daemon-owned AGH session ID", index, record)
			}
		}

		owned := make([][]acpmock.DiagnosticsRecord, 0, len(sessions))
		for _, sessionPayload := range sessions {
			sessionRecords := acpmock.DiagnosticsForAGHSession(records, sessionPayload.ID)
			for index, record := range sessionRecords {
				if record.AGHSessionID != sessionPayload.ID {
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
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, aghcontract.CreateSessionRequest{
			AgentName:       "reasoning-codex-missing",
			Provider:        "codex",
			Model:           "gpt-5.6-sol",
			ReasoningEffort: "max",
			WorkspacePath:   harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateStarting {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 starting", status, accepted)
		}
		waitForReasoningNegotiationFailure(
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
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, aghcontract.CreateSessionRequest{
			AgentName:       "reasoning-codex-unavailable",
			Provider:        "codex",
			Model:           "gpt-5.6-terra",
			ReasoningEffort: "max",
			WorkspacePath:   harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateStarting {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 starting", status, accepted)
		}
		waitForReasoningNegotiationFailure(
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
		status, accepted := createSessionHTTPAccepted(t, ctx, harness, aghcontract.CreateSessionRequest{
			AgentName:       "reasoning-codex-unsupported",
			Provider:        "codex",
			Model:           "gpt-5.6-sol",
			ReasoningEffort: "minimal",
			WorkspacePath:   harness.WorkspaceRoot,
		})
		if status != http.StatusCreated || accepted.State != session.StateStarting {
			t.Fatalf("HTTP create session = status:%d session:%#v, want 201 starting", status, accepted)
		}
		waitForReasoningNegotiationFailure(
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
	accepted aghcontract.SessionPayload,
) aghcontract.SessionPayload {
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

func waitForReasoningNegotiationFailure(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	wantSummary string,
) aghcontract.SessionPayload {
	t.Helper()
	var current aghcontract.SessionPayload
	waitForRuntimeCondition(t, "reasoning negotiation failure", 10*time.Second, func() bool {
		resolved, err := harness.GetSession(ctx, sessionID)
		if err != nil {
			return false
		}
		current = resolved
		return current.State == session.StateStopped
	})
	if current.Failure == nil || current.Failure.Kind != store.FailureProtocol ||
		!strings.Contains(current.Failure.Summary, wantSummary) {
		t.Fatalf(
			"reasoning negotiation failure = %#v, want kind %q containing %q",
			current.Failure,
			store.FailureProtocol,
			wantSummary,
		)
	}
	return current
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
			return harness.ApproveSessionPermission(ctx, session.ID, aghcontract.ApproveSessionRequest{
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
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{
		Type:       "tool_call",
		Title:      "Inspect fixture",
		ToolCallID: "tool-1",
	}) {
		t.Fatalf("events = %#v, want tool_call event", events)
	}
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{
		Type:       "tool_result",
		Title:      "Inspect fixture",
		ToolCallID: "tool-1",
	}) {
		t.Fatalf("events = %#v, want tool_result event", events)
	}
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{
		Type:     "permission",
		Resource: "danger.txt",
	}) {
		t.Fatalf("events = %#v, want permission event", events)
	}
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{
		Type:     "permission",
		Resource: "danger.txt",
		Decision: "allow-always",
	}) {
		t.Fatalf("events = %#v, want approved permission event", events)
	}
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{
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
		if _, err := harness.CreateNetworkChannel(ctx, aghcontract.CreateNetworkChannelRequest{
			Channel:      channelID,
			WorkspaceID:  harness.WorkspaceID,
			Purpose:      "Hosted MCP native tool projection",
			FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
			AgentNames:   []string{"mock-hosted-native"},
		}); err != nil {
			t.Fatalf("CreateNetworkChannel(%q) error = %v", channelID, err)
		}
		session, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
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
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(hosted-native) error = %v", err)
		}
		hostedServer := requireHostedMCPStdioServer(t, diagnostics)
		client := startHostedMCPClient(t, hostedServer)
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				t.Fatalf("Close(hosted MCP client) error = %v", closeErr)
			}
		}()

		var init sdkmcp.InitializeRequest
		init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
		init.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-hosted-e2e", Version: "1.0.0"}
		if _, err := client.Initialize(ctx, init); err != nil {
			t.Fatalf("Initialize(hosted MCP client) error = %v", err)
		}

		list, err := client.ListTools(ctx, sdkmcp.ListToolsRequest{})
		if err != nil {
			t.Fatalf("ListTools(hosted MCP) error = %v", err)
		}
		networkToolID := toolspkg.ToolIDNetworkChannelCreate.String()
		if !sdkToolListContains(list.Tools, networkToolID) {
			t.Fatalf("hosted MCP tools = %#v, want non-bootstrap tool %s", sdkToolNames(list.Tools), networkToolID)
		}

		channelName := "hostednative-created"
		var call sdkmcp.CallToolRequest
		call.Params.Name = networkToolID
		call.Params.Arguments = map[string]any{
			"workspace_id": harness.WorkspaceID,
			"channel":      channelName,
			"purpose":      "Runtime E2E hosted native tool access",
		}
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

		createFixtureBackedSession(t, ctx, harness, "mock-provider-models", "provider-models-session")
		diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(provider-models) error = %v", err)
		}
		client := startHostedMCPClient(t, requireHostedMCPStdioServer(t, diagnostics))
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				t.Fatalf("Close(hosted provider-models MCP client) error = %v", closeErr)
			}
		}()

		var init sdkmcp.InitializeRequest
		init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
		init.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-provider-models-e2e", Version: "1.0.0"}
		if _, err := client.Initialize(ctx, init); err != nil {
			t.Fatalf("Initialize(hosted provider-models MCP client) error = %v", err)
		}
		tools, err := client.ListTools(ctx, sdkmcp.ListToolsRequest{})
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

		var refresh aghcontract.ProviderModelRefreshResponse
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

		var hidden aghcontract.ProviderModelCurationResponse
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
			*hidden.Model.DefaultReasoningEffort != aghcontract.ReasoningEffort("max") {
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

		var nativeAll aghcontract.ProviderModelListResponse
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

		var unhidden aghcontract.ProviderModelCurationResponse
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

		var nativeCurated aghcontract.ProviderModelListResponse
		callHostedMCPToolJSON(
			t,
			ctx,
			client,
			toolspkg.ToolIDProviderModelsList.String(),
			map[string]any{"provider_id": "codex"},
			&nativeCurated,
		)
		solCuratedNative := requireProviderModelPayload(t, nativeCurated.Models, "gpt-5.6-sol")

		var cliCurated aghcontract.ProviderModelListResponse
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

	creatorSession := createFixtureBackedSession(t, ctx, harness, "mock-wake-creator", "wake-creator-session")
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(wake-creator) error = %v", err)
	}
	hostedServer := requireHostedMCPStdioServer(t, diagnostics)
	client := startHostedMCPClient(t, hostedServer)
	clientClosed := false
	defer func() {
		if clientClosed {
			return
		}
		if closeErr := client.Close(); closeErr != nil {
			t.Logf("Close(hosted MCP client) after failed test error = %v", closeErr)
		}
	}()

	var init sdkmcp.InitializeRequest
	init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-task-wake-e2e", Version: "1.0.0"}
	if _, err := client.Initialize(ctx, init); err != nil {
		t.Fatalf("Initialize(hosted MCP client) error = %v", err)
	}

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
		if strings.TrimSpace(creatorSession.ACPSessionID) == "" {
			t.Fatalf("creator session ACPSessionID is empty: %#v", creatorSession)
		}
		if got, want := terminalWake.SessionID, creatorSession.ACPSessionID; got != want {
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

func requireHostedMCPStdioServer(
	t testing.TB,
	records []acpmock.DiagnosticsRecord,
) acpsdk.McpServerStdio {
	t.Helper()

	for _, record := range records {
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
	t.Fatalf("diagnostics = %#v, want session_new %s stdio MCP server", records, mcppkg.HostedServerName)
	return acpsdk.McpServerStdio{}
}

func startHostedMCPClient(
	t testing.TB,
	stdio acpsdk.McpServerStdio,
) *mcpclient.Client {
	t.Helper()

	if strings.TrimSpace(stdio.Command) == "" {
		t.Fatalf("hosted MCP stdio server = %#v, want command", stdio)
	}
	client, err := mcpclient.NewStdioMCPClientWithOptions(
		stdio.Command,
		hostedMCPStdioEnv(stdio),
		append([]string(nil), stdio.Args...),
	)
	if err != nil {
		t.Fatalf("NewStdioMCPClientWithOptions(%q) error = %v", stdio.Command, err)
	}
	return client
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

func sdkToolListContains(tools []sdkmcp.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func sdkToolNames(tools []sdkmcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func callHostedMCPToolJSON(
	t testing.TB,
	ctx context.Context,
	client *mcpclient.Client,
	toolID string,
	arguments map[string]any,
	destination any,
) {
	t.Helper()

	var call sdkmcp.CallToolRequest
	call.Params.Name = toolID
	call.Params.Arguments = arguments
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

func providerModelPayloadExists(models []aghcontract.ProviderModelPayload, modelID string) bool {
	for _, model := range models {
		if model.ModelID == modelID {
			return true
		}
	}
	return false
}

func requireProviderModelPayload(
	t testing.TB,
	models []aghcontract.ProviderModelPayload,
	modelID string,
) aghcontract.ProviderModelPayload {
	t.Helper()
	for _, model := range models {
		if model.ModelID == modelID {
			return model
		}
	}
	t.Fatalf("provider model %q not found in %#v", modelID, models)
	return aghcontract.ProviderModelPayload{}
}

func assertCanonicalHiddenSolPayload(t testing.TB, model aghcontract.ProviderModelPayload) {
	t.Helper()

	wantEfforts := []aghcontract.ReasoningEffort{
		aghcontract.ReasoningEffort("none"),
		aghcontract.ReasoningEffort("low"),
		aghcontract.ReasoningEffort("medium"),
		aghcontract.ReasoningEffort("high"),
		aghcontract.ReasoningEffort("xhigh"),
		aghcontract.ReasoningEffort("max"),
	}
	if model.ProviderID != "codex" || model.ModelID != "gpt-5.6-sol" || model.Curated ||
		!model.Hidden || model.Deprecated || !model.Featured || model.ReleaseDate != "2026-06-26" ||
		model.ReasoningSource != aghcontract.ReasoningSource("catalog") {
		t.Fatalf("Sol identity/curation payload = %#v", model)
	}
	if model.ContextWindow == nil || *model.ContextWindow != 1_050_000 ||
		model.MaxOutputTokens == nil || *model.MaxOutputTokens != 128_000 ||
		model.SupportsTools == nil || !*model.SupportsTools ||
		model.SupportsReasoning == nil || !*model.SupportsReasoning {
		t.Fatalf("Sol capability payload = %#v", model)
	}
	if !reflect.DeepEqual(model.ReasoningEfforts, wantEfforts) ||
		model.DefaultReasoningEffort == nil || *model.DefaultReasoningEffort != aghcontract.ReasoningEffort("max") {
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
	client *mcpclient.Client,
	taskID string,
	title string,
) string {
	t.Helper()

	var call sdkmcp.CallToolRequest
	call.Params.Name = toolspkg.ToolIDTaskCreate.String()
	call.Params.Arguments = map[string]any{
		"id":    strings.TrimSpace(taskID),
		"scope": string(taskpkg.ScopeGlobal),
		"title": strings.TrimSpace(title),
	}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", call.Params.Name, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%s) result = %#v, want successful result", call.Params.Name, result)
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
) aghcontract.TaskRunPayload {
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
	var completedLease aghcontract.AgentTaskLeaseResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		sessionRecord,
		http.MethodPost,
		"/api/agent/tasks/"+url.PathEscape(claimed.ID)+"/complete",
		aghcontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
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
	return aghcontract.TaskRunPayload{}
}

func enqueueWakeTaskRunForWakeE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	idempotencyKey string,
) aghcontract.TaskRunPayload {
	t.Helper()

	var response aghcontract.TaskRunResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID)) + "/runs"
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		path,
		aghcontract.EnqueueTaskRunRequest{IdempotencyKey: strings.TrimSpace(idempotencyKey)},
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
) aghcontract.TaskRunPayload {
	t.Helper()

	var response aghcontract.TaskRunResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/attach-session"
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		path,
		aghcontract.AttachTaskRunSessionRequest{SessionID: strings.TrimSpace(sessionID)},
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

	var response aghcontract.TaskTimelineResponse
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
	if envelope.Type != "data-agh-permission" || envelope.Data.Decision != "" || envelope.Data.RequestID == "" {
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
	events []aghcontract.SessionEventPayload,
) []aghcontract.AgentEventPayload {
	t.Helper()

	decoded := make([]aghcontract.AgentEventPayload, 0, len(events))
	for _, event := range events {
		var payload aghcontract.AgentEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(session event %q) error = %v", event.ID, err)
		}
		decoded = append(decoded, payload)
	}
	return decoded
}

func containsAgentEvent(events []aghcontract.AgentEventPayload, want aghcontract.AgentEventPayload) bool {
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
