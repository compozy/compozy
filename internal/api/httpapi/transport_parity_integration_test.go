//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	apispec "github.com/compozy/agh/internal/api/spec"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	tomltree "github.com/pelletier/go-toml"
)

const (
	transportApprovalAgentName = "transport-approver"
	transportAutomationAgent   = "transport-automation-runner"
	transportFaultyAgent       = "transport-faulty-runner"
	transportOverrideProvider  = "qa-transport-override"
)

func TestHTTPTransportApprovalFlowUsesSharedRuntimeHarness(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "permission_env_fixture.json"),
			FixtureAgent: "approver",
			AgentName:    transportApprovalAgentName,
		}},
	})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
		AgentName:     transportApprovalAgentName,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session = waitForHTTPTransportSessionActive(t, ctx, runtimeHarness, session)

	var approvedRequestID string
	events, err := runtimeHarness.PromptSessionHTTPWithEvents(
		ctx,
		session.ID,
		"request permission",
		func(record e2etest.SSEEvent) error {
			payload, ok := e2etest.PermissionPayloadFromSSE(record)
			if !ok || payload.Decision != "" || approvedRequestID != "" {
				return nil
			}
			approvedRequestID = payload.RequestID
			return runtimeHarness.ApproveSessionPermission(ctx, session.ID, aghcontract.ApproveSessionRequest{
				RequestID: payload.RequestID,
				Decision:  "allow-always",
			})
		},
	)
	if err != nil {
		t.Fatalf("PromptSessionHTTPWithEvents() error = %v", err)
	}
	if approvedRequestID == "" {
		t.Fatal("approvedRequestID = empty, want pending permission request")
	}

	payloads := e2etest.PermissionPayloads(events)
	if got, want := len(payloads), 2; got != want {
		t.Fatalf("len(permission payloads) = %d, want %d; payloads=%#v", got, want, payloads)
	}
	if got, want := payloads[0].RequestID, approvedRequestID; got != want {
		t.Fatalf("payloads[0].RequestID = %q, want %q", got, want)
	}
	if got := payloads[len(payloads)-1].Decision; got != "allow-always" {
		t.Fatalf("final permission decision = %q, want %q", got, "allow-always")
	}
	if !e2etest.RecordsContainTextDelta(events, "allow-always") {
		t.Fatalf("prompt events = %#v, want allow-always text delta", events)
	}

	sessionResp := mustHTTPRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		runtimeHarness.HTTPURL(transportHarnessSessionPath(t, runtimeHarness, session.ID, "")),
		nil,
		nil,
	)
	if sessionResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(sessionResp.Body)
		_ = sessionResp.Body.Close()
		t.Fatalf("session status = %d, want %d; body=%s", sessionResp.StatusCode, http.StatusOK, string(body))
	}
	var detail struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, sessionResp, &detail)
	if got, want := detail.Session.ID, session.ID; got != want {
		t.Fatalf("detail.Session.ID = %q, want %q", got, want)
	}
}

func TestHTTPTransportSessionProviderLifecycle(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportAutomationAgent,
		}},
	})

	registration, ok := runtimeHarness.MockAgentRegistration(transportAutomationAgent)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) not found", transportAutomationAgent)
	}
	provider := registration.Provider

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Run("Should round-trip the provider through create and read", func(t *testing.T) {
		var created aghcontract.SessionResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/sessions", aghcontract.CreateSessionRequest{
			AgentName:     transportAutomationAgent,
			Provider:      provider,
			WorkspacePath: runtimeHarness.WorkspaceRoot,
		}, &created); err != nil {
			t.Fatalf("HTTP create session error = %v", err)
		}
		if created.Session.ID == "" {
			t.Fatal("HTTP create session id = empty, want non-empty")
		}
		if created.Session.Provider != provider {
			t.Fatalf("HTTP create provider = %q, want %q", created.Session.Provider, provider)
		}
		created.Session = waitForHTTPTransportSessionActive(t, ctx, runtimeHarness, created.Session)

		var detail aghcontract.SessionResponse
		if err := runtimeHarness.HTTPJSON(
			ctx,
			http.MethodGet,
			transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, ""),
			nil,
			&detail,
		); err != nil {
			t.Fatalf("HTTP get session error = %v", err)
		}
		if detail.Session.Provider != created.Session.Provider {
			t.Fatalf(
				"HTTP detail provider = %q, want create provider %q",
				detail.Session.Provider,
				created.Session.Provider,
			)
		}
	})

	t.Run("Should return conflict when attaching a stopped session", func(t *testing.T) {
		writeTransportProviderOverrideConfig(
			t,
			runtimeHarness.WorkspaceRoot,
			transportOverrideProvider,
			registration.Command,
			true,
		)

		var created aghcontract.SessionResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/sessions", aghcontract.CreateSessionRequest{
			AgentName:     transportAutomationAgent,
			Provider:      transportOverrideProvider,
			WorkspacePath: runtimeHarness.WorkspaceRoot,
		}, &created); err != nil {
			t.Fatalf("HTTP create session error = %v", err)
		}
		created.Session = waitForHTTPTransportSessionActive(t, ctx, runtimeHarness, created.Session)

		stopResp := mustHTTPRequest(
			t,
			runtimeHarness.HTTPClient,
			http.MethodPost,
			runtimeHarness.HTTPURL(transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, "/stop")),
			nil,
			nil,
		)
		body, readErr := io.ReadAll(stopResp.Body)
		closeErr := stopResp.Body.Close()
		if readErr != nil {
			t.Fatalf("read HTTP stop response body error = %v", readErr)
		}
		if closeErr != nil {
			t.Fatalf("close HTTP stop response body error = %v", closeErr)
		}
		if stopResp.StatusCode != http.StatusNoContent {
			t.Fatalf(
				"HTTP stop session status = %d, want %d; body=%s",
				stopResp.StatusCode,
				http.StatusNoContent,
				string(body),
			)
		}

		writeTransportProviderOverrideConfig(
			t,
			runtimeHarness.WorkspaceRoot,
			transportOverrideProvider,
			registration.Command,
			false,
		)

		resumeResp := mustHTTPRequest(
			t,
			runtimeHarness.HTTPClient,
			http.MethodPost,
			runtimeHarness.HTTPURL(
				transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, "/attach"),
			),
			nil,
			nil,
		)
		resumeBody, err := io.ReadAll(resumeResp.Body)
		resumeCloseErr := resumeResp.Body.Close()
		if err != nil {
			t.Fatalf("read HTTP resume body error = %v", err)
		}
		if resumeCloseErr != nil {
			t.Fatalf("close HTTP resume body error = %v", resumeCloseErr)
		}
		if resumeResp.StatusCode != http.StatusConflict {
			t.Fatalf(
				"HTTP attach status = %d, want %d; body=%s",
				resumeResp.StatusCode,
				http.StatusConflict,
				string(resumeBody),
			)
		}

		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(resumeBody, &payload); err != nil {
			t.Fatalf("Unmarshal(resume body) error = %v; body=%s", err, string(resumeBody))
		}

		if !strings.Contains(payload.Error, created.Session.ID) {
			t.Fatalf("HTTP attach error = %s, want session id %q", payload.Error, created.Session.ID)
		}
		if !strings.Contains(payload.Error, "not attachable") {
			t.Fatalf("HTTP attach error = %s, want attachability context", payload.Error)
		}
	})
}

func TestHTTPTransportWebhookIngressUsesSharedRuntimeHarness(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent: transportAutomationAgent,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportAutomationAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	trigger, endpoint := seedTransportWebhookTrigger(t, ctx, runtimeHarness)
	payload := []byte(`{"payload":"deploy","branch":"main"}`)
	delivery, err := runtimeHarness.DeliverGlobalWebhook(
		ctx,
		endpoint,
		"shared-secret",
		payload,
		"delivery-http-transport",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("DeliverGlobalWebhook() error = %v", err)
	}
	if got, want := len(delivery.Runs), 1; got != want {
		t.Fatalf("len(delivery.Runs) = %d, want %d; delivery=%#v", got, want, delivery)
	}

	httpRun := waitForHTTPAutomationRun(t, ctx, runtimeHarness, delivery.Runs[0].ID)
	if err := e2etest.ValidateWebhookRunProjection(delivery, httpRun); err != nil {
		t.Fatalf("ValidateWebhookRunProjection() error = %v", err)
	}
	if got, want := httpRun.TriggerID, trigger.ID; got != want {
		t.Fatalf("httpRun.TriggerID = %q, want %q", got, want)
	}
	if httpRun.SessionID == "" {
		t.Fatalf("httpRun.SessionID = %q, want non-empty session linkage", httpRun.SessionID)
	}
}

func TestHTTPTransportPromptFailureProjectionUsesSharedRuntimeHarness(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "driver_fault_fixture.json"),
			FixtureAgent: "faulty",
			AgentName:    transportFaultyAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
		AgentName:     transportFaultyAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session = waitForHTTPTransportSessionActive(t, ctx, runtimeHarness, session)

	stream, err := runtimeHarness.PromptSessionHTTP(ctx, session.ID, "trigger invalid frame")
	if err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}
	if !e2etest.RecordsContainTextDelta(stream, "partial before invalid frame") {
		t.Fatalf("HTTP prompt stream = %#v, want partial assistant delta", stream)
	}
	if !httpSSEContainsEvent(stream, "error") {
		t.Fatalf("HTTP prompt stream = %#v, want error event", stream)
	}

	var eventsResp aghcontract.SessionEventsResponse
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodGet,
		transportHarnessSessionPath(t, runtimeHarness, session.ID, "/events"),
		nil,
		&eventsResp,
	); err != nil {
		t.Fatalf("HTTP session events error = %v", err)
	}
	if !httpSessionEventsContainType(eventsResp.Events, "error") {
		t.Fatalf("HTTP session events = %#v, want error projection", eventsResp.Events)
	}
}

func waitForHTTPTransportSessionActive(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	accepted aghcontract.SessionPayload,
) aghcontract.SessionPayload {
	t.Helper()
	active, err := harness.WaitForSessionActive(ctx, accepted.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v; accepted=%#v", accepted.ID, err, accepted)
	}
	return active
}

func transportHarnessSessionPath(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	suffix string,
) string {
	t.Helper()

	workspaceID := strings.TrimSpace(harness.WorkspaceID)
	if workspaceID == "" {
		t.Fatal("runtime harness WorkspaceID = empty")
	}
	return "/api/workspaces/" + url.PathEscape(workspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + suffix
}

func TestHTTPTransportExtensionParityMatchesUDS(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	httpListResp := mustHTTPRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		runtimeHarness.HTTPURL("/api/extensions"),
		nil,
		nil,
	)
	if httpListResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpListResp.Body)
		_ = httpListResp.Body.Close()
		t.Fatalf(
			"HTTP list extensions status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var httpList aghcontract.ExtensionsResponse
	decodeHTTPJSON(t, httpListResp, &httpList)

	var udsList aghcontract.ExtensionsResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, "/api/extensions", nil, &udsList); err != nil {
		t.Fatalf("UDS list extensions error = %v", err)
	}
	sortExtensionsByName(httpList.Extensions)
	sortExtensionsByName(udsList.Extensions)
	if !extensionsSemanticallyEqual(httpList.Extensions, udsList.Extensions) {
		t.Fatalf("HTTP extensions = %#v, want UDS parity %#v", httpList.Extensions, udsList.Extensions)
	}
	if len(httpList.Extensions) == 0 {
		return
	}

	extensionName := httpList.Extensions[0].Name

	httpStatusResp := mustHTTPRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		runtimeHarness.HTTPURL("/api/extensions/"+url.PathEscape(extensionName)),
		nil,
		nil,
	)
	if httpStatusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpStatusResp.Body)
		_ = httpStatusResp.Body.Close()
		t.Fatalf("HTTP extension status = %d, want %d; body=%s", httpStatusResp.StatusCode, http.StatusOK, string(body))
	}
	var httpStatus aghcontract.ExtensionResponse
	decodeHTTPJSON(t, httpStatusResp, &httpStatus)

	var udsStatus aghcontract.ExtensionResponse
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(extensionName),
		nil,
		&udsStatus,
	); err != nil {
		t.Fatalf("UDS extension status error = %v", err)
	}
	if !extensionSemanticallyEqual(httpStatus.Extension, udsStatus.Extension) {
		t.Fatalf("HTTP extension = %#v, want UDS parity %#v", httpStatus.Extension, udsStatus.Extension)
	}
}

func TestHTTPTransportTaskSurfaceMatchesDocumentedSpecOperations(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))

	got := make([]string, 0)
	for _, route := range engine.Routes() {
		if isDocumentedHTTPTaskRoute(route.Path) {
			got = append(got, route.Method+" "+route.Path)
		}
	}
	sort.Strings(got)

	want := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if !slices.Contains(operation.Transports, apispec.TransportHTTP) {
			continue
		}
		if !isDocumentedHTTPTaskRoute(operation.Path) {
			continue
		}
		want = append(want, operation.Method+" "+normalizeSpecRoutePath(operation.Path))
	}
	sort.Strings(want)

	if !slices.Equal(got, want) {
		t.Fatalf("HTTP task routes = %v, want documented task routes %v", got, want)
	}
}
func seedTransportWebhookTrigger(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) (aghcontract.TriggerPayload, string) {
	t.Helper()

	state, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Triggers: []aghcontract.CreateTriggerRequest{{
			Scope:              automationpkg.AutomationScopeGlobal,
			Name:               "deploy-review",
			AgentName:          transportAutomationAgent,
			Prompt:             `Review payload {{ index .Data "payload" }} for {{ index .Data "branch" }}`,
			Event:              "webhook",
			EndpointSlug:       "deploy-review",
			WebhookSecretValue: "shared-secret",
		}},
	})
	if err != nil {
		t.Fatalf("SeedAutomationFixtures() error = %v", err)
	}
	if got, want := len(state.Triggers), 1; got != want {
		t.Fatalf("len(state.Triggers) = %d, want %d", got, want)
	}

	endpoint, err := automationpkg.FormatWebhookEndpoint(
		state.Triggers[0].EndpointSlug,
		state.Triggers[0].WebhookID,
	)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}
	return state.Triggers[0], endpoint
}

func waitForHTTPAutomationRun(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) aghcontract.RunPayload {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastErr  error
		lastSeen string
	)
	for {
		var response aghcontract.RunResponse
		err := harness.HTTPJSON(waitCtx, http.MethodGet, "/api/automation/runs/"+url.PathEscape(runID), nil, &response)
		if err == nil && response.Run.ID == runID {
			return response.Run
		}
		lastErr = err
		lastSeen = response.Run.ID
		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"waitForHTTPAutomationRun(%q) timed out: %v; last error=%v last run=%q",
				runID,
				waitCtx.Err(),
				lastErr,
				lastSeen,
			)
		case <-ticker.C:
		}
	}
}

func sortExtensionsByName(values []aghcontract.ExtensionPayload) {
	slices.SortFunc(values, func(left, right aghcontract.ExtensionPayload) int {
		switch {
		case left.Name < right.Name:
			return -1
		case left.Name > right.Name:
			return 1
		default:
			return 0
		}
	})
}

func extensionsSemanticallyEqual(left, right []aghcontract.ExtensionPayload) bool {
	return slices.EqualFunc(left, right, extensionSemanticallyEqual)
}

func extensionSemanticallyEqual(left, right aghcontract.ExtensionPayload) bool {
	left = normalizeExtensionPayload(left)
	right = normalizeExtensionPayload(right)

	if left.Name != right.Name ||
		left.Version != right.Version ||
		left.Type != right.Type ||
		left.Source != right.Source ||
		left.Enabled != right.Enabled ||
		left.State != right.State ||
		left.PID != right.PID ||
		left.UptimeSeconds != right.UptimeSeconds ||
		left.Health != right.Health ||
		left.HealthMessage != right.HealthMessage ||
		left.LastError != right.LastError ||
		left.DaemonRunning != right.DaemonRunning {
		return false
	}
	if !slices.Equal(left.Capabilities, right.Capabilities) {
		return false
	}
	if !slices.Equal(left.Actions, right.Actions) {
		return false
	}
	return extensionBundlesSemanticallyEqual(left.Bundles, right.Bundles)
}

func normalizeExtensionPayload(value aghcontract.ExtensionPayload) aghcontract.ExtensionPayload {
	value.Capabilities = normalizeStrings(value.Capabilities)
	value.Actions = normalizeStrings(value.Actions)
	value.Bundles = normalizeExtensionBundles(value.Bundles)
	return value
}

func normalizeExtensionBundles(
	values []aghcontract.ExtensionBundleSummaryPayload,
) []aghcontract.ExtensionBundleSummaryPayload {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]aghcontract.ExtensionBundleSummaryPayload, len(values))
	for idx, value := range values {
		value.Profiles = normalizeStrings(value.Profiles)
		normalized[idx] = value
	}
	return normalized
}

func extensionBundlesSemanticallyEqual(
	left,
	right []aghcontract.ExtensionBundleSummaryPayload,
) bool {
	return slices.EqualFunc(left, right, func(left, right aghcontract.ExtensionBundleSummaryPayload) bool {
		return left.Name == right.Name &&
			left.Description == right.Description &&
			slices.Equal(left.Profiles, right.Profiles)
	})
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func transportMockFixturePath(t testing.TB, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testutil", "acpmock", "testdata", name)
}

func writeTransportProviderOverrideConfig(
	t testing.TB,
	workspaceRoot string,
	providerName string,
	command string,
	includeProvider bool,
) {
	t.Helper()

	configDir := filepath.Join(workspaceRoot, aghconfig.DirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", configDir, err)
	}

	configPath := filepath.Join(configDir, aghconfig.ConfigName)
	tree, err := loadTransportProviderOverrideTree(configPath)
	if err != nil {
		t.Fatalf("load transport override config %q error = %v", configPath, err)
	}

	providerPath := []string{"providers", strings.TrimSpace(providerName)}
	if includeProvider {
		if tree.GetPath(providerPath) != nil {
			if err := tree.DeletePath(providerPath); err != nil {
				t.Fatalf("DeletePath(%v) error = %v", providerPath, err)
			}
		}
		tree.SetPath(append(providerPath, "command"), strings.TrimSpace(command))
		tree.SetPath(append(providerPath, "models", "default"), "transport-override-model")
		credentialSlot, err := tomltree.TreeFromMap(map[string]any{
			"name":       "api_key",
			"target_env": "TRANSPORT_OVERRIDE_API_KEY",
			"secret_ref": "env:TRANSPORT_OVERRIDE_API_KEY",
			"kind":       "api_key",
			"required":   false,
		})
		if err != nil {
			t.Fatalf("TreeFromMap(credential slot) error = %v", err)
		}
		tree.SetPath(append(providerPath, "credential_slots"), []*tomltree.Tree{credentialSlot})
	} else {
		if tree.GetPath(providerPath) != nil {
			if err := tree.DeletePath(providerPath); err != nil {
				t.Fatalf("DeletePath(%v) error = %v", providerPath, err)
			}
		}
		if providers, ok := tree.GetPath([]string{"providers"}).(*tomltree.Tree); ok && len(providers.Keys()) == 0 {
			if err := tree.Delete("providers"); err != nil {
				t.Fatalf("Delete(providers) error = %v", err)
			}
		}
	}

	rendered, err := tree.ToTomlString()
	if err != nil {
		t.Fatalf("ToTomlString(%q) error = %v", configPath, err)
	}
	if err := os.WriteFile(configPath, []byte(rendered), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", configPath, err)
	}
}

func loadTransportProviderOverrideTree(configPath string) (*tomltree.Tree, error) {
	contents, err := os.ReadFile(configPath)
	switch {
	case err == nil:
		if strings.TrimSpace(string(contents)) == "" {
			return tomltree.TreeFromMap(map[string]any{})
		}
		return tomltree.LoadBytes(contents)
	case errors.Is(err, os.ErrNotExist):
		return tomltree.TreeFromMap(map[string]any{})
	default:
		return nil, err
	}
}

func httpSSEContainsEvent(records []e2etest.SSEEvent, want string) bool {
	for _, record := range records {
		if record.Event == want {
			return true
		}
	}
	return false
}

type httpSessionEventContent struct {
	Type string `json:"type"`
}

func httpSessionEventsContainType(events []aghcontract.SessionEventPayload, want string) bool {
	for _, event := range events {
		var payload httpSessionEventContent
		if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
			continue
		}
		if payload.Type == want {
			return true
		}
	}
	return false
}

func isDocumentedHTTPTaskRoute(path string) bool {
	return strings.HasPrefix(path, "/api/tasks") ||
		strings.HasPrefix(path, "/api/task-runs") ||
		strings.HasPrefix(path, "/api/observe/tasks")
}

func normalizeSpecRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[i] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}
