package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

const (
	mockAgentsMessageKey = "message"
)

// MockAgentSpec is the narrow-waist contract for fixture-backed mock agents.
// Runtime and browser E2E helpers should register mock agents through this type
// instead of calling acpmock.Register directly.
type MockAgentSpec struct {
	FixturePath     string
	FixtureAgent    string
	AgentName       string
	ProviderName    string
	DiagnosticsPath string
}

// RegisterMockAgent writes one temporary fixture-backed AGENT.md into the isolated AGH home.
func (h *RuntimeHarness) RegisterMockAgent(t testing.TB, spec MockAgentSpec) acpmock.Registration {
	t.Helper()

	registration, err := registerMockAgent(h.HomePaths, h.Artifacts, spec)
	if err != nil {
		t.Fatalf("RegisterMockAgent(%q) error = %v", spec.FixturePath, err)
	}
	if h.MockAgents == nil {
		h.MockAgents = make(map[string]acpmock.Registration)
	}
	h.MockAgents[registration.AgentName] = registration
	return registration
}

// MockAgentRegistration returns one previously registered mock-agent definition.
func (h *RuntimeHarness) MockAgentRegistration(agentName string) (acpmock.Registration, bool) {
	if h == nil || len(h.MockAgents) == 0 {
		return acpmock.Registration{}, false
	}
	registration, ok := h.MockAgents[agentName]
	return registration, ok
}

// CaptureMockAgentDiagnostics stores parsed mock-agent diagnostics into the shared artifact model.
func (h *RuntimeHarness) CaptureMockAgentDiagnostics(registrations ...acpmock.Registration) error {
	if len(registrations) == 0 {
		return nil
	}

	snapshot := make(map[string][]acpmock.DiagnosticsRecord, len(registrations))
	for _, registration := range registrations {
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			return err
		}
		snapshot[registration.AgentName] = records
	}
	return h.CaptureProviderCallsJSON(map[string]any{
		"mock_agents": snapshot,
	})
}

// PromptSessionHTTP sends one prompt through the public HTTP API and drains the SSE stream.
func (h *RuntimeHarness) PromptSessionHTTP(
	ctx context.Context,
	sessionID string,
	message string,
) ([]SSEEvent, error) {
	return h.PromptSessionHTTPWithEvents(ctx, sessionID, message, nil)
}

// PromptSessionHTTPWithEvents sends one prompt through the public HTTP API and
// lets callers react to streamed SSE records before the prompt completes.
func (h *RuntimeHarness) PromptSessionHTTPWithEvents(
	ctx context.Context,
	sessionID string,
	message string,
	onEvent func(SSEEvent) error,
) ([]SSEEvent, error) {
	body := map[string]string{mockAgentsMessageKey: message}
	path, err := h.sessionScopedAPIPath(sessionID, "/prompt")
	if err != nil {
		return nil, err
	}
	response, err := doRequest(
		ctx,
		h.HTTPClient,
		h.HTTPURL(path),
		http.MethodPost,
		body,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read HTTP prompt failure response: %w", readErr)
		}
		return nil, fmt.Errorf("HTTP prompt session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	return readSSERecordsWithCallback(response.Body, 0, onEvent)
}

// PromptSessionHTTPUntil sends one prompt through the public HTTP API and
// returns as soon as the streamed SSE records satisfy predicate.
func (h *RuntimeHarness) PromptSessionHTTPUntil(
	ctx context.Context,
	sessionID string,
	message string,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	if err := validateSSEPredicate(predicate); err != nil {
		return nil, err
	}
	body := map[string]string{mockAgentsMessageKey: message}
	path, err := h.sessionScopedAPIPath(sessionID, "/prompt")
	if err != nil {
		return nil, err
	}
	response, err := doRequest(
		ctx,
		h.HTTPClient,
		h.HTTPURL(path),
		http.MethodPost,
		body,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read HTTP prompt failure response: %w", readErr)
		}
		return nil, fmt.Errorf("HTTP prompt session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	return readSSERecordsUntil(response.Body, predicate)
}

// StreamSessionHTTPUntil opens the public HTTP session event stream and returns
// as soon as streamed SSE records satisfy predicate.
func (h *RuntimeHarness) StreamSessionHTTPUntil(
	ctx context.Context,
	sessionID string,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	return h.streamSessionHTTPUntil(ctx, sessionID, "", predicate)
}

// StreamSessionRawHTTPUntil opens the public HTTP session event stream in raw
// frame mode and returns as soon as streamed SSE records satisfy predicate.
func (h *RuntimeHarness) StreamSessionRawHTTPUntil(
	ctx context.Context,
	sessionID string,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	return h.streamSessionHTTPUntil(
		ctx,
		sessionID,
		fmt.Sprintf("frames=%s", aghcontract.SessionStreamFrameRaw),
		predicate,
	)
}

func (h *RuntimeHarness) streamSessionHTTPUntil(
	ctx context.Context,
	sessionID string,
	rawQuery string,
	predicate func(SSEEvent) bool,
) ([]SSEEvent, error) {
	if err := validateSSEPredicate(predicate); err != nil {
		return nil, err
	}
	path, err := h.sessionScopedAPIPath(sessionID, "/stream")
	if err != nil {
		return nil, err
	}
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	response, err := doRequest(
		ctx,
		h.HTTPClient,
		h.HTTPURL(path),
		http.MethodGet,
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read HTTP session stream failure response: %w", readErr)
		}
		return nil, fmt.Errorf("HTTP session stream status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}

	return readSSERecordsUntil(response.Body, predicate)
}

// ApproveSessionPermission resolves one live permission request through the public HTTP surface.
func (h *RuntimeHarness) ApproveSessionPermission(
	ctx context.Context,
	sessionID string,
	request aghcontract.ApproveSessionRequest,
) error {
	path, err := h.sessionScopedAPIPath(sessionID, "/approve")
	if err != nil {
		return err
	}
	response, err := doRequest(
		ctx,
		h.HTTPClient,
		h.HTTPURL(path),
		http.MethodPost,
		request,
	)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return fmt.Errorf("read HTTP approve failure response: %w", readErr)
		}
		return fmt.Errorf("HTTP approve session status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
	}
	return nil
}

func registerMockAgents(
	t testing.TB,
	homePaths aghconfig.HomePaths,
	artifacts *ArtifactCollector,
	specs []MockAgentSpec,
) map[string]acpmock.Registration {
	t.Helper()

	if len(specs) == 0 {
		return nil
	}

	registrations := make(map[string]acpmock.Registration, len(specs))
	for _, spec := range specs {
		registration, err := registerMockAgent(homePaths, artifacts, spec)
		if err != nil {
			t.Fatalf("register mock agent %q error = %v", spec.FixturePath, err)
		}
		registrations[registration.AgentName] = registration
	}
	return registrations
}

func registerMockAgent(
	homePaths aghconfig.HomePaths,
	artifacts *ArtifactCollector,
	spec MockAgentSpec,
) (acpmock.Registration, error) {
	diagnosticsPath, err := mockAgentDiagnosticsPath(artifacts, spec)
	if err != nil {
		return acpmock.Registration{}, err
	}

	providerName := strings.TrimSpace(spec.ProviderName)
	if providerName == "" {
		providerName = acpmock.ProviderName
	}
	return acpmock.Register(homePaths, acpmock.RegisterOptions{
		FixturePath:     spec.FixturePath,
		FixtureAgent:    spec.FixtureAgent,
		AgentName:       spec.AgentName,
		ProviderName:    providerName,
		DiagnosticsPath: diagnosticsPath,
	})
}

func ensureMockAgentProviderConfig(t testing.TB, cfg *aghconfig.Config) {
	t.Helper()

	if cfg == nil {
		return
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]aghconfig.ProviderConfig{}
	}
	if _, ok := cfg.Providers[acpmock.ProviderName]; ok {
		return
	}
	driverPath, err := acpmock.DefaultDriverPath()
	if err != nil {
		t.Fatalf("acpmock.DefaultDriverPath() error = %v", err)
	}
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driverPath)
}

func mockAgentDiagnosticsPath(
	artifacts *ArtifactCollector,
	spec MockAgentSpec,
) (string, error) {
	diagnosticsPath := spec.DiagnosticsPath
	if diagnosticsPath == "" {
		if artifacts == nil {
			return "", errors.New("mock agent diagnostics path requires artifacts when no override is set")
		}
		agentName := spec.AgentName
		if agentName == "" {
			agentName = spec.FixtureAgent
		}
		diagnosticsPath = filepath.Join(artifacts.RootDir(), "mock_agents", agentName+".jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(diagnosticsPath), 0o755); err != nil {
		return "", fmt.Errorf("os.MkdirAll(%q) error = %w", filepath.Dir(diagnosticsPath), err)
	}
	return diagnosticsPath, nil
}
