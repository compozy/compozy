package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	extensionpkg "github.com/compozy/agh/internal/extension"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/resources"
	mcpclient "github.com/mark3labs/mcp-go/client"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestHostAPIProjectionDecisions(t *testing.T) {
	t.Parallel()

	t.Run("Should cover every canonical Host API method explicitly", func(t *testing.T) {
		t.Parallel()

		if missing := HostAPIProjectionCoverage(); len(missing) != 0 {
			t.Fatalf("HostAPIProjectionCoverage() = %v, want empty", missing)
		}
		if got, want := len(hostAPIProjectionDecisions), len(extensionprotocol.AllHostAPIMethods()); got != want {
			t.Fatalf("len(hostAPIProjectionDecisions) = %d, want exact registry size %d", got, want)
		}
		for method, decision := range hostAPIProjectionDecisions {
			if decision.Publish && decision.Binding == workspaceBindingNone {
				t.Fatalf("decision[%q] publishes without a workspace binding", method)
			}
			if !decision.Publish && strings.TrimSpace(decision.Reason) == "" {
				t.Fatalf("decision[%q] exclusion has no reason", method)
			}
		}
	})

	t.Run("Should keep projected names outside the native tool namespace and reversible", func(t *testing.T) {
		t.Parallel()

		for _, method := range ProjectedHostAPIMethods() {
			name := hostAPIToolName(extensionprotocol.HostAPIMethod(method))
			if strings.HasPrefix(name, "agh__") {
				t.Fatalf("hostAPIToolName(%q) = %q, want non-native namespace", method, name)
			}
			got, ok := hostAPIMethodFromToolName(name)
			if !ok || string(got) != method {
				t.Fatalf("hostAPIMethodFromToolName(%q) = %q, %v; want %q, true", name, got, ok, method)
			}
		}
	})

	t.Run("Should derive one schema-backed tool for every published method", func(t *testing.T) {
		t.Parallel()

		tools, err := projectedHostAPITools()
		if err != nil {
			t.Fatalf("projectedHostAPITools() error = %v", err)
		}
		if got, want := len(tools), len(ProjectedHostAPIMethods()); got != want {
			t.Fatalf("len(projectedHostAPITools()) = %d, want %d", got, want)
		}
		for _, tool := range tools {
			if len(tool.RawInputSchema) == 0 || !json.Valid(tool.RawInputSchema) {
				t.Fatalf("tool %q schema = %s, want valid JSON", tool.Name, tool.RawInputSchema)
			}
		}
	})
}

func TestHostAPIBinding(t *testing.T) {
	t.Parallel()

	t.Run("Should force canonical workspace identity fields", func(t *testing.T) {
		t.Parallel()

		bound, err := bindHostAPIParams(
			json.RawMessage(`{"session_id":"sess-1"}`),
			workspaceBindingID,
			"ws-1",
			"/workspace",
		)
		if err != nil {
			t.Fatalf("bindHostAPIParams() error = %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(bound, &payload); err != nil {
			t.Fatalf("json.Unmarshal(bound) error = %v", err)
		}
		if got := payload["workspace_id"]; got != "ws-1" {
			t.Fatalf("workspace_id = %#v, want ws-1", got)
		}
	})

	t.Run("Should reject caller workspace conflicts", func(t *testing.T) {
		t.Parallel()

		_, err := bindHostAPIParams(
			json.RawMessage(`{"workspace_id":"ws-2"}`),
			workspaceBindingID,
			"ws-1",
			"/workspace",
		)
		if err == nil || !strings.Contains(err.Error(), "conflicts") {
			t.Fatalf("bindHostAPIParams() error = %v, want conflict", err)
		}
	})

	t.Run("Should force task creation into workspace scope", func(t *testing.T) {
		t.Parallel()

		bound, err := bindHostAPIParams(
			json.RawMessage(`{"title":"Ship"}`),
			workspaceBindingTask,
			"ws-1",
			"/workspace",
		)
		if err != nil {
			t.Fatalf("bindHostAPIParams() error = %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(bound, &payload); err != nil {
			t.Fatalf("json.Unmarshal(bound) error = %v", err)
		}
		if payload["scope"] != "workspace" || payload["workspace"] != "/workspace" {
			t.Fatalf("bound task = %#v, want workspace scope/root", payload)
		}
	})

	t.Run("Should bind every resource snapshot record to the workspace", func(t *testing.T) {
		t.Parallel()

		bound, err := bindHostAPIParams(
			json.RawMessage(`{"records":[{"kind":"tool","id":"a","spec":{}}]}`),
			workspaceBindingResource,
			"ws-1",
			"/workspace",
		)
		if err != nil {
			t.Fatalf("bindHostAPIParams() error = %v", err)
		}
		if !strings.Contains(string(bound), `"kind":"workspace"`) ||
			!strings.Contains(string(bound), `"id":"ws-1"`) {
			t.Fatalf("bound resource payload = %s, want workspace scope", bound)
		}
	})
}

func TestHostAPIHTTPAuth(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a missing bearer token deterministically", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		response := httptest.NewRecorder()
		bearerTokenHandler("0123456789abcdef", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler called without authentication")
		})).ServeHTTP(response, request)
		if got := response.Code; got != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", got, http.StatusUnauthorized)
		}
		if got := response.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
		}
		if got, want := response.Body.String(), "Unauthorized\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("Should accept the exact bearer token", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		request.Header.Set("Authorization", "Bearer 0123456789abcdef")
		response := httptest.NewRecorder()
		bearerTokenHandler("0123456789abcdef", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(response, request)
		if got := response.Code; got != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", got, http.StatusNoContent)
		}
		if got := response.Body.String(); got != "" {
			t.Fatalf("body = %q, want empty", got)
		}
	})

	t.Run("Should reject a raw token without the bearer scheme", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		request.Header.Set("Authorization", "0123456789abcdef")
		response := httptest.NewRecorder()
		bearerTokenHandler("0123456789abcdef", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler called without bearer scheme")
		})).ServeHTTP(response, request)
		if got := response.Code; got != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", got, http.StatusUnauthorized)
		}
		if got := response.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
		}
		if got, want := response.Body.String(), "Unauthorized\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("Should reject HTTP startup without a token", func(t *testing.T) {
		t.Parallel()

		err := ServeHTTP(
			t.Context(),
			&recordingHostAPIInvoker{},
			"workspace",
			"127.0.0.1:0",
			"",
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "token is required") {
			t.Fatalf("ServeHTTP() error = %v, want token required", err)
		}
	})

	t.Run("Should reject a non-loopback listener", func(t *testing.T) {
		t.Parallel()

		err := ServeHTTP(
			t.Context(),
			&recordingHostAPIInvoker{},
			"workspace",
			"0.0.0.0:2123",
			"0123456789abcdef",
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "must be loopback") {
			t.Fatalf("ServeHTTP() error = %v, want loopback validation", err)
		}
	})
}

func TestHostAPIFacadeCloseReleasesPrincipals(t *testing.T) {
	t.Parallel()
	t.Run("Should release principals and source-owned resources idempotently", func(t *testing.T) {
		t.Parallel()

		checker := &extensionpkg.CapabilityChecker{}
		sources := &recordingSourceSessionManager{}
		facade := NewHostAPIFacade(nil, checker, nil, sources)
		session, err := facade.session(t.Context(), "serve-1", "ws-1")
		if err != nil {
			t.Fatalf("facade.session() error = %v", err)
		}
		nextSession, err := facade.session(t.Context(), "serve-2", "ws-1")
		if err != nil {
			t.Fatalf("facade.session(next client) error = %v", err)
		}
		if session.actor.Source == nextSession.actor.Source ||
			session.actor.SessionNonce == nextSession.actor.SessionNonce {
			t.Fatalf("serve sessions share source lifecycle: first=%#v next=%#v", session.actor, nextSession.actor)
		}
		if err := checker.Check(session.principal, "session.read"); err != nil {
			t.Fatalf("CapabilityChecker.Check() before close error = %v", err)
		}

		if err := facade.CloseHostAPISession(t.Context(), "serve-1"); err != nil {
			t.Fatalf("CloseHostAPISession(first) error = %v", err)
		}
		if err := facade.CloseHostAPISession(t.Context(), "serve-1"); err != nil {
			t.Fatalf("CloseHostAPISession(second) error = %v", err)
		}

		if err := checker.Check(session.principal, "session.read"); err == nil {
			t.Fatal("CapabilityChecker.Check() after close error = nil, want principal removed")
		}
		if got, want := sources.lastReset(t), session.actor.Source; got != want {
			t.Fatalf("ResetSource source = %#v, want %#v", got, want)
		}
		if err := checker.Check(nextSession.principal, "session.read"); err != nil {
			t.Fatalf("next client principal after first close error = %v", err)
		}
		if err := facade.Close(t.Context()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		_, sessionErr := facade.session(t.Context(), "serve-2", "ws-1")
		if sessionErr == nil || !strings.Contains(sessionErr.Error(), "closed") {
			t.Fatalf("facade.session() after close error = %v, want closed", sessionErr)
		}
	})
}

func TestHostAPIMCPServerRoundTrip(t *testing.T) {
	t.Parallel()
	t.Run("Should relay projected tools with the stable serve-session identity", func(t *testing.T) {
		t.Parallel()

		invoker := &recordingHostAPIInvoker{result: json.RawMessage(`{"sessions":[]}`)}
		mcpServer, err := newHostAPIMCPServer(invoker, "serve-1", "workspace-a")
		if err != nil {
			t.Fatalf("newHostAPIMCPServer() error = %v", err)
		}
		client, err := mcpclient.NewInProcessClient(mcpServer)
		if err != nil {
			t.Fatalf("NewInProcessClient() error = %v", err)
		}
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Errorf("client.Close() error = %v", err)
			}
		})
		if err := client.Start(t.Context()); err != nil {
			t.Fatalf("client.Start() error = %v", err)
		}
		var initialize sdkmcp.InitializeRequest
		initialize.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
		initialize.Params.ClientInfo = sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}
		if _, err := client.Initialize(t.Context(), initialize); err != nil {
			t.Fatalf("client.Initialize() error = %v", err)
		}

		listed, err := client.ListTools(t.Context(), sdkmcp.ListToolsRequest{})
		if err != nil {
			t.Fatalf("client.ListTools() error = %v", err)
		}
		names := make([]string, 0, len(listed.Tools))
		for _, tool := range listed.Tools {
			names = append(names, tool.Name)
		}
		if !slices.Contains(names, "agh_host__sessions__list") || slices.Contains(names, "agh__sessions_list") {
			t.Fatalf("tool names = %v, want MCP Host API namespace only", names)
		}

		var call sdkmcp.CallToolRequest
		call.Params.Name = "agh_host__sessions__list"
		call.Params.Arguments = map[string]any{}
		result, err := client.CallTool(t.Context(), call)
		if err != nil {
			t.Fatalf("client.CallTool() error = %v", err)
		}
		if result == nil || result.IsError {
			t.Fatalf("client.CallTool() result = %#v, want success", result)
		}
		observed := invoker.lastCall(t)
		if observed.serveSessionID != "serve-1" || observed.workspace != "workspace-a" ||
			observed.method != "sessions/list" || string(observed.params) != "{}" {
			t.Fatalf("InvokeHostAPI call = %#v, want bound workspace and sessions/list", observed)
		}

		call.Params.Name = "agh_host__tasks__create"
		call.Params.Arguments = map[string]any{"title": "Ship"}
		if _, err := client.CallTool(t.Context(), call); err != nil {
			t.Fatalf("client.CallTool(tasks/create without bound fields) error = %v", err)
		}
		observed = invoker.lastCall(t)
		if observed.method != "tasks/create" {
			t.Fatalf("InvokeHostAPI method = %q, want tasks/create", observed.method)
		}
	})
}

type recordingHostAPIInvoker struct {
	mu     sync.Mutex
	result json.RawMessage
	calls  []recordedHostAPICall
}

type recordedHostAPICall struct {
	serveSessionID string
	workspace      string
	method         string
	params         json.RawMessage
}

func (i *recordingHostAPIInvoker) InvokeHostAPI(
	_ context.Context,
	serveSessionID string,
	workspace string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, recordedHostAPICall{
		serveSessionID: serveSessionID,
		workspace:      workspace,
		method:         method,
		params:         append(json.RawMessage(nil), params...),
	})
	if i.result == nil {
		return json.RawMessage(`{}`), nil
	}
	return append(json.RawMessage(nil), i.result...), nil
}

func (i *recordingHostAPIInvoker) CloseHostAPISession(context.Context, string) error { return nil }

func (i *recordingHostAPIInvoker) lastCall(t *testing.T) recordedHostAPICall {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.calls) == 0 {
		t.Fatal("InvokeHostAPI was not called")
	}
	return i.calls[len(i.calls)-1]
}

type recordingSourceSessionManager struct {
	mu          sync.Mutex
	activations []resources.ResourceSource
	resets      []resources.ResourceSource
}

func (m *recordingSourceSessionManager) ActivateSourceSession(
	_ context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
	_ string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activations = append(m.activations, source)
	return nil
}

func (m *recordingSourceSessionManager) ResetSource(
	_ context.Context,
	_ resources.MutationActor,
	source resources.ResourceSource,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resets = append(m.resets, source)
	return nil
}

func (m *recordingSourceSessionManager) lastReset(t *testing.T) resources.ResourceSource {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.resets) == 0 {
		t.Fatal("ResetSource was not called")
	}
	return m.resets[len(m.resets)-1]
}
