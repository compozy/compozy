package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func TestRuntimeHarnessRegisterMockAgentWritesFixtureBackedDefinition(t *testing.T) {
	t.Run("Should write an auth-free fixture-backed definition", func(t *testing.T) {
		t.Parallel()

		fixturePath, err := filepath.Abs(filepath.Join("..", "acpmock", "testdata", "multi_agent_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}

		homePaths := NewHomePaths(t)
		harness := &RuntimeHarness{
			HomePaths: homePaths,
			Artifacts: NewArtifactCollector(t),
		}

		registration := harness.RegisterMockAgent(t, MockAgentSpec{
			FixturePath:  fixturePath,
			FixtureAgent: "alpha",
			AgentName:    "mock-alpha",
		})

		loaded, err := aghconfig.LoadAgentDefFile(registration.AgentDefPath)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(%q) error = %v", registration.AgentDefPath, err)
		}
		if got, want := loaded.Name, "mock-alpha"; got != want {
			t.Fatalf("loaded.Name = %q, want %q", got, want)
		}
		if got, want := loaded.Provider, acpmock.ProviderName; got != want {
			t.Fatalf("loaded.Provider = %q, want %q", got, want)
		}
		if !strings.Contains(loaded.Command, "--agent alpha") {
			t.Fatalf("loaded.Command = %q, want --agent alpha", loaded.Command)
		}
		if !strings.Contains(loaded.Command, "--fixture") {
			t.Fatalf("loaded.Command = %q, want --fixture", loaded.Command)
		}
		if strings.Contains(loaded.Command, "driver/dist/index.js") {
			t.Fatalf("loaded.Command = %q, want no dist/index.js dependency", loaded.Command)
		}
	})

	t.Run("Should preserve a real provider configuration with the fixture command", func(t *testing.T) {
		t.Parallel()

		fixturePath, err := filepath.Abs(filepath.Join("..", "acpmock", "testdata", "multi_agent_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}
		homePaths := NewHomePaths(t)
		harness := &RuntimeHarness{HomePaths: homePaths, Artifacts: NewArtifactCollector(t)}
		registration := harness.RegisterMockAgent(t, MockAgentSpec{
			FixturePath:  fixturePath,
			FixtureAgent: "alpha",
			AgentName:    "mock-codex",
			ProviderName: "codex",
		})
		loaded, err := aghconfig.LoadAgentDefFile(registration.AgentDefPath)
		if err != nil {
			t.Fatalf("LoadAgentDefFile(%q) error = %v", registration.AgentDefPath, err)
		}
		cfg := aghconfig.DefaultWithHome(homePaths)
		resolved, err := cfg.ResolveAgent(loaded)
		if err != nil {
			t.Fatalf("ResolveAgent(mock-codex) error = %v", err)
		}
		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("resolved.Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Command, registration.Command; got != want {
			t.Fatalf("resolved.Command = %q, want fixture command %q", got, want)
		}
		if got, want := resolved.Reasoning.Apply, aghconfig.ReasoningApplyACPOption; got != want {
			t.Fatalf("resolved.Reasoning.Apply = %q, want %q", got, want)
		}
	})
}

func TestRuntimeHarnessMockAgentProviderConfig(t *testing.T) {
	t.Run("Should seed an auth-free mock provider for runtime layouts", func(t *testing.T) {
		t.Parallel()

		fixturePath, err := filepath.Abs(filepath.Join("..", "acpmock", "testdata", "multi_agent_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}

		homePaths := NewHomePaths(t)
		layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{
			HomePaths: homePaths,
			MockAgents: []MockAgentSpec{{
				FixturePath:  fixturePath,
				FixtureAgent: "alpha",
				AgentName:    "mock-alpha",
			}},
		})

		loadedConfig, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		agent, err := aghconfig.LoadAgentDef("mock-alpha", homePaths)
		if err != nil {
			t.Fatalf("LoadAgentDef(mock-alpha) error = %v", err)
		}
		resolved, err := loadedConfig.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent(mock-alpha) error = %v", err)
		}
		registration, ok := layout.MockAgents["mock-alpha"]
		if !ok {
			t.Fatal("layout.MockAgents[mock-alpha] = missing, want registered mock agent")
		}

		if got, want := resolved.Provider, acpmock.ProviderName; got != want {
			t.Fatalf("resolved.Provider = %q, want %q", got, want)
		}
		if got, want := resolved.AuthMode, aghconfig.ProviderAuthModeNone; got != want {
			t.Fatalf("resolved.AuthMode = %q, want %q", got, want)
		}
		if got, want := resolved.NoneSecurity, aghconfig.ProviderNoneSecurityLocalTransport; got != want {
			t.Fatalf("resolved.NoneSecurity = %q, want %q", got, want)
		}
		if got, want := loadedConfig.Providers[acpmock.ProviderName].Command, registration.DriverPath; got != want {
			t.Fatalf("mock provider command = %q, want registered driver path %q", got, want)
		}
		if got, want := resolved.Command, registration.Command; got != want {
			t.Fatalf("resolved.Command = %q, want %q", got, want)
		}
	})
}

func TestRuntimeHarnessCaptureMockAgentDiagnosticsStoresArtifact(t *testing.T) {
	t.Parallel()

	diagnosticsPath := filepath.Join(t.TempDir(), "alpha.jsonl")
	record := acpmock.DiagnosticsRecord{
		AgentName:   "alpha",
		SessionID:   "sess-1",
		PromptIndex: 1,
		Prompt:      "hello alpha",
		TurnName:    "alpha-hello",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal(record) error = %v", err)
	}
	if err := os.WriteFile(diagnosticsPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", diagnosticsPath, err)
	}

	harness := &RuntimeHarness{
		Artifacts: NewArtifactCollector(t),
	}
	if err := harness.CaptureMockAgentDiagnostics(acpmock.Registration{
		AgentName:       "alpha",
		DiagnosticsPath: diagnosticsPath,
	}); err != nil {
		t.Fatalf("CaptureMockAgentDiagnostics() error = %v", err)
	}

	artifactPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindProviderCalls)
	if !ok {
		t.Fatal("ArtifactPath(provider_calls) = missing, want present")
	}
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", artifactPath, err)
	}
	if !strings.Contains(string(content), "mock_agents") || !strings.Contains(string(content), "alpha-hello") {
		t.Fatalf("provider_calls artifact = %s, want mock-agent diagnostics", string(content))
	}
}

func TestRuntimeHarnessMockAgentRegistrationLookupAndBatchRegister(t *testing.T) {
	fixturePath, err := filepath.Abs(filepath.Join("..", "acpmock", "testdata", "multi_agent_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}

	harness := &RuntimeHarness{
		HomePaths: NewHomePaths(t),
		Artifacts: NewArtifactCollector(t),
	}
	registrations := registerMockAgents(t, harness.HomePaths, harness.Artifacts, []MockAgentSpec{{
		FixturePath:  fixturePath,
		FixtureAgent: "alpha",
		AgentName:    "mock-alpha",
	}})
	harness.MockAgents = registrations

	registration, ok := harness.MockAgentRegistration("mock-alpha")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-alpha) = missing, want present")
	}
	if got, want := registration.AgentName, "mock-alpha"; got != want {
		t.Fatalf("registration.AgentName = %q, want %q", got, want)
	}
	if _, ok := harness.MockAgentRegistration("missing"); ok {
		t.Fatal("MockAgentRegistration(missing) = present, want absent")
	}
}

func TestRuntimeHarnessPromptSessionHTTPAndApprovePermissionUsePublicSurface(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/prompt":
			w.Header().Set("Content-Type", "text/event-stream")
			if _, err := fmt.Fprint(
				w,
				"event: agent_message\n"+
					"data: {\"delta\":\"hello\"}\n\n"+
					"event: done\n"+
					"data: [DONE]\n\n",
			); err != nil {
				t.Errorf("write prompt stream error = %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/approve":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		WorkspaceID: "ws-1",
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
	}

	var seenEvents []SSEEvent
	events, err := harness.PromptSessionHTTPWithEvents(
		context.Background(),
		"sess-1",
		"hello http",
		func(event SSEEvent) error {
			seenEvents = append(seenEvents, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("PromptSessionHTTPWithEvents() error = %v", err)
	}
	if got, want := len(events), 2; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if got, want := len(seenEvents), 2; got != want {
		t.Fatalf("len(seenEvents) = %d, want %d", got, want)
	}

	simpleEvents, err := harness.PromptSessionHTTP(context.Background(), "sess-1", "hello wrapper")
	if err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}
	if got, want := len(simpleEvents), 2; got != want {
		t.Fatalf("len(simpleEvents) = %d, want %d", got, want)
	}

	if err := harness.ApproveSessionPermission(
		context.Background(),
		"sess-1",
		aghcontract.ApproveSessionRequest{},
	); err != nil {
		t.Fatalf("ApproveSessionPermission() error = %v", err)
	}
}

func TestRuntimeHarnessPromptSessionHTTPEscapesSessionIDs(t *testing.T) {
	t.Parallel()

	sessionID := "sess/ops worker"
	expectedPromptPath := "/api/workspaces/ws-1/sessions/" + url.PathEscape(sessionID) + "/prompt"
	expectedApprovePath := "/api/workspaces/ws-1/sessions/" + url.PathEscape(sessionID) + "/approve"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.EscapedPath() == expectedPromptPath:
			w.Header().Set("Content-Type", "text/event-stream")
			if _, err := fmt.Fprint(
				w,
				"event: agent_message\n"+
					"data: {\"delta\":\"hello\"}\n\n"+
					"event: done\n"+
					"data: [DONE]\n\n",
			); err != nil {
				t.Errorf("write escaped prompt stream error = %v", err)
			}
		case r.Method == http.MethodPost && r.URL.EscapedPath() == expectedApprovePath:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		WorkspaceID: "ws-1",
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
	}

	events, err := harness.PromptSessionHTTP(context.Background(), sessionID, "hello http")
	if err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}
	if got, want := len(events), 2; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}

	if err := harness.ApproveSessionPermission(
		context.Background(),
		sessionID,
		aghcontract.ApproveSessionRequest{},
	); err != nil {
		t.Fatalf("ApproveSessionPermission() error = %v", err)
	}
}

func TestRuntimeHarnessPromptSessionHTTPAndApprovePermissionReportFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/prompt":
			http.Error(w, "permission denied", http.StatusForbidden)
		case r.Method == http.MethodPost && r.URL.Path == "/api/workspaces/ws-1/sessions/sess-1/approve":
			http.Error(w, "approval failed", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		WorkspaceID: "ws-1",
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
	}

	if _, err := harness.PromptSessionHTTP(
		context.Background(),
		"sess-1",
		"hello http",
	); err == nil ||
		!strings.Contains(err.Error(), "HTTP prompt session status 403") {
		t.Fatalf("PromptSessionHTTP() error = %v, want 403 failure", err)
	}
	if err := harness.ApproveSessionPermission(
		context.Background(),
		"sess-1",
		aghcontract.ApproveSessionRequest{},
	); err == nil ||
		!strings.Contains(err.Error(), "HTTP approve session status 400") {
		t.Fatalf("ApproveSessionPermission() error = %v, want 400 failure", err)
	}
}
