package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestApplyHostedToolsUsesDescriptorRawSchemas(t *testing.T) {
	t.Parallel()

	inputSchema := json.RawMessage(
		`{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}`,
	)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
	view := hostedToolView("compozy__hosted_echo")
	view.Descriptor.InputSchema = inputSchema
	view.Descriptor.OutputSchema = outputSchema

	mcpServer := newHostedProxyTestServer()
	applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{view})

	registered := listHostedProxyTools(t, mcpServer)
	tool, ok := registered["compozy__hosted_echo"]
	if !ok {
		t.Fatalf("registered tools = %#v, want compozy__hosted_echo", registered)
	}
	gotInput, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("json.Marshal(InputSchema) error = %v", err)
	}
	if !equalJSON(t, gotInput, inputSchema) {
		t.Fatalf("InputSchema = %s, want schema-equivalent %s", gotInput, inputSchema)
	}
	gotOutput, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		t.Fatalf("json.Marshal(OutputSchema) error = %v", err)
	}
	if !equalJSON(t, gotOutput, outputSchema) {
		t.Fatalf("OutputSchema = %s, want schema-equivalent %s", gotOutput, outputSchema)
	}
}

func TestApplyHostedToolsAddsAnthropicMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should set searchHint and omit alwaysLoad for non-search tools", func(t *testing.T) {
		t.Parallel()

		echo := hostedToolView("compozy__hosted_echo")
		echo.Descriptor.SearchHints = []string{"echo messages"}
		mcpServer := newHostedProxyTestServer()
		applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{echo})

		registered := listHostedProxyTools(t, mcpServer)
		echoTool, ok := registered["compozy__hosted_echo"]
		if !ok {
			t.Fatalf("registered tools = %#v, want compozy__hosted_echo", registered)
		}
		echoHint := requireHostedToolMetaString(t, echoTool.Meta, "anthropic/searchHint")
		if !strings.Contains(echoHint, "compozy__hosted_echo") || !strings.Contains(echoHint, "echo messages") {
			t.Fatalf("echo search hint = %q, want canonical ID and descriptor hint", echoHint)
		}
		if _, ok := echoTool.Meta["anthropic/alwaysLoad"]; ok {
			t.Fatalf("echo metadata = %#v, want no alwaysLoad hint", echoTool.Meta)
		}
	})

	t.Run("Should set searchHint and alwaysLoad for tool_search", func(t *testing.T) {
		t.Parallel()

		search := hostedToolView(tools.ToolIDToolSearch)
		search.Descriptor.SearchHints = []string{"find Compozy native tools"}
		mcpServer := newHostedProxyTestServer()
		applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{search})

		registered := listHostedProxyTools(t, mcpServer)
		searchTool, ok := registered[tools.ToolIDToolSearch.String()]
		if !ok {
			t.Fatalf("registered tools = %#v, want %s", registered, tools.ToolIDToolSearch)
		}
		searchHint := requireHostedToolMetaString(t, searchTool.Meta, "anthropic/searchHint")
		if !strings.Contains(searchHint, tools.ToolIDToolSearch.String()) ||
			!strings.Contains(searchHint, "find Compozy native tools") {
			t.Fatalf("tool_search hint = %q, want canonical ID and descriptor hint", searchHint)
		}
		if got := searchTool.Meta["anthropic/alwaysLoad"]; got != true {
			t.Fatalf("tool_search alwaysLoad = %#v, want true", got)
		}
	})
}

func TestReconcileHostedTools(t *testing.T) {
	t.Parallel()

	t.Run("Should retain an unchanged descriptor without re-registering the tool", func(t *testing.T) {
		t.Parallel()

		view := hostedToolView("compozy__hosted_echo")
		registry := &countingHostedToolRegistry{}
		client := &hostedProxyClientStub{}
		first := reconcileHostedTools(registry, client, "bind-1", nil, []tools.ToolView{view})
		second := reconcileHostedTools(registry, client, "bind-1", first, []tools.ToolView{view})

		if got, want := registry.addCalls, 1; got != want {
			t.Fatalf("AddTool calls = %d, want %d after unchanged projection", got, want)
		}
		if got := registry.removeCalls; len(got) != 0 {
			t.Fatalf("RemoveTools calls = %#v, want none after unchanged projection", got)
		}
		if !reflect.DeepEqual(second, first) {
			t.Fatalf("retained fingerprints = %#v, want %#v", second, first)
		}
	})
}

func TestRunHostedProxyUsesPrivateTTLCacheForProjectionChanges(t *testing.T) {
	t.Parallel()

	initial := hostedToolView("compozy__hosted_echo")
	updated := hostedToolView("compozy__hosted_other")
	proxyClient := newHostedProxyClientStub(HostedBindResponse{
		BindID: "bind-1",
		Scope:  tools.Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "codex"},
		Tools:  []tools.ToolView{initial},
		Digest: "initial",
	})
	proxyClient.callResult = HostedCallResponse{
		Result: tools.ToolResult{Content: []tools.ToolContent{{Type: "text", Text: "called"}}},
	}

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunHostedProxy(ctx, proxyClient, HostedProxyOptions{
			SessionID: "sess-1",
			Nonce:     "nonce-1",
			Stdin:     serverReader,
			Stdout:    serverWriter,
			Stderr:    io.Discard,
			Version:   "test",
		})
	}()

	mcpClient := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "hosted-proxy-test", Version: "1.0.0"},
		&sdkmcp.ClientOptions{Capabilities: &sdkmcp.ClientCapabilities{}},
	)
	session, err := mcpClient.Connect(
		ctx,
		&sdkmcp.IOTransport{
			Reader: io.NopCloser(clientReader),
			Writer: nopWriteCloser{Writer: clientWriter},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := session.Close(); closeErr != nil {
			t.Errorf("session.Close() error = %v", closeErr)
		}
	})

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(initial) error = %v", err)
	}
	if got, want := sdkToolNames(list.Tools), []string{"compozy__hosted_echo"}; !slices.Equal(got, want) {
		t.Fatalf("initial tools = %#v, want %#v", got, want)
	}

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "compozy__hosted_echo",
		Arguments: map[string]any{"message": "hello"},
		Meta: sdkmcp.Meta{
			"toolCallId":    "call-1",
			"approvalToken": "ignored",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool() result = %#v, want non-error result", result)
	}
	observedCall := proxyClient.lastCall(t)
	if observedCall.BindID != "bind-1" || observedCall.ToolName != "compozy__hosted_echo" ||
		observedCall.ToolCallID != "call-1" {
		t.Fatalf("hosted call request = %#v, want bind/tool/toolCallId propagated", observedCall)
	}

	proxyClient.emitProjection(t, HostedProjectionResponse{
		Tools:  []tools.ToolView{updated},
		Digest: "updated",
	})

	list, err = session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(cached) error = %v", err)
	}
	if got, want := sdkToolNames(list.Tools), []string{"compozy__hosted_echo"}; !slices.Equal(got, want) {
		t.Fatalf("cached tools = %#v, want %#v until private TTL expires", got, want)
	}
	if got, want := list.CacheScope, "private"; got != want {
		t.Fatalf("tools/list cache scope = %q, want %q", got, want)
	}
	if list.TTLMs <= 0 {
		t.Fatalf("tools/list TTL = %d, want positive", list.TTLMs)
	}

	cancel()
	if err := session.Close(); err != nil {
		t.Fatalf("session.Close() error = %v", err)
	}
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hosted proxy shutdown")
	}
	if got := proxyClient.releaseCount(); got != 1 {
		t.Fatalf("ReleaseHostedMCP calls = %d, want 1", got)
	}
}

func TestRunHostedProxyProviderProtocolCompatibility(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the claim tool to the managed provider protocol", func(t *testing.T) {
		t.Parallel()

		claim := hostedToolView(tools.ToolIDTaskRunClaimNext)
		proxyClient := newHostedProxyClientStub(HostedBindResponse{
			BindID: "bind-provider",
			Scope: tools.Scope{
				SessionID:   "sess-provider",
				WorkspaceID: "ws-provider",
				AgentName:   "codex",
			},
			Tools:  []tools.ToolView{claim},
			Digest: "provider",
		})

		serverReader, clientWriter := io.Pipe()
		clientReader, serverWriter := io.Pipe()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errCh := make(chan error, 1)
		go func() {
			errCh <- RunHostedProxy(ctx, proxyClient, HostedProxyOptions{
				SessionID: "sess-provider",
				Nonce:     "nonce-provider",
				Stdin:     serverReader,
				Stdout:    serverWriter,
				Stderr:    io.Discard,
				Version:   "test",
			})
		}()

		encoder := json.NewEncoder(clientWriter)
		decoder := json.NewDecoder(clientReader)
		if err := encoder.Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": protocolVersionProviderLegacy,
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
			t.Fatalf("encode initialize request error = %v", err)
		}
		var initialized struct {
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		}
		if err := decoder.Decode(&initialized); err != nil {
			t.Fatalf("decode initialize response error = %v", err)
		}
		if len(initialized.Error) > 0 && string(initialized.Error) != "null" {
			t.Fatalf("initialize response error = %s", initialized.Error)
		}
		if got, want := initialized.Result.ProtocolVersion, protocolVersionProviderLegacy; got != want {
			t.Fatalf("initialize protocol version = %q, want %q", got, want)
		}

		if err := encoder.Encode(map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}); err != nil {
			t.Fatalf("encode initialized notification error = %v", err)
		}
		if err := encoder.Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  mcpToolsListMethod,
			"params":  map[string]any{},
		}); err != nil {
			t.Fatalf("encode tools/list request error = %v", err)
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
			t.Fatalf("decode tools/list response error = %v", err)
		}
		if len(listed.Error) > 0 && string(listed.Error) != "null" {
			t.Fatalf("tools/list response error = %s", listed.Error)
		}
		if got, want := listed.Result.Tools, []struct {
			Name string `json:"name"`
		}{{Name: tools.ToolIDTaskRunClaimNext.String()}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("tools/list tools = %#v, want %#v", got, want)
		}

		cancel()
		if err := clientWriter.Close(); err != nil {
			t.Errorf("client writer close error = %v", err)
		}
		if err := clientReader.Close(); err != nil {
			t.Errorf("client reader close error = %v", err)
		}
		select {
		case runErr := <-errCh:
			if !isExpectedStdioServerTermination(runErr) && !errors.Is(runErr, io.ErrClosedPipe) {
				t.Fatalf("RunHostedProxy() error = %v", runErr)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for hosted provider proxy shutdown")
		}
		if got := proxyClient.releaseCount(); got != 1 {
			t.Fatalf("ReleaseHostedMCP calls = %d, want 1", got)
		}
	})
}

func TestHostedProxyHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should build descriptions from title and descriptor text", func(t *testing.T) {
		t.Parallel()

		view := hostedToolView("compozy__hosted_echo")
		view.Descriptor.DisplayTitle = "Echo"
		view.Descriptor.Description = "Send text back"
		view.Descriptor.Toolsets = []tools.ToolsetID{tools.ToolsetIDBootstrap}
		view.Descriptor.Tags = []string{"catalog", "echo"}
		view.Descriptor.SearchHints = []string{"send text back"}
		got := hostedToolDescription(view.Descriptor)
		for _, want := range []string{
			"Echo\n\nSend text back",
			"Compozy canonical tool ID: compozy__hosted_echo",
			"Compozy toolsets: compozy__bootstrap",
			"Tags: catalog, echo",
			"Search hints: send text back",
			"Call the harness-returned tool reference.",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("hostedToolDescription() = %q, want substring %q", got, want)
			}
		}
		view.Descriptor.Description = ""
		if got := hostedToolDescription(view.Descriptor); !strings.Contains(got, "Echo") ||
			!strings.Contains(got, "Compozy canonical tool ID: compozy__hosted_echo") {
			t.Fatalf("hostedToolDescription(title only) = %q, want title and canonical ID", got)
		}
		view.Descriptor.DisplayTitle = ""
		view.Descriptor.Description = "fallback"
		if got := hostedToolDescription(view.Descriptor); !strings.Contains(got, "fallback") ||
			!strings.Contains(got, "Compozy canonical tool ID: compozy__hosted_echo") {
			t.Fatalf("hostedToolDescription(description only) = %q, want description and canonical ID", got)
		}
	})

	t.Run("Should normalize raw arguments", func(t *testing.T) {
		t.Parallel()

		raw, err := rawArguments(nil)
		if err != nil || string(raw) != `{}` {
			t.Fatalf("rawArguments(nil) = %s, %v; want {}", raw, err)
		}
		raw, err = rawArguments(json.RawMessage(`{"ok":true}`))
		if err != nil || string(raw) != `{"ok":true}` {
			t.Fatalf("rawArguments(raw) = %s, %v; want exact raw", raw, err)
		}
		raw, err = rawArguments(map[string]any{"message": "hello"})
		if err != nil || !json.Valid(raw) {
			t.Fatalf("rawArguments(map) = %s, %v; want valid JSON", raw, err)
		}
		if _, err = rawArguments(make(chan struct{})); err == nil {
			t.Fatal("rawArguments(unmarshalable) error = nil, want error")
		}
	})

	t.Run("Should read optional tool call metadata only from supported key", func(t *testing.T) {
		t.Parallel()

		if got := hostedToolCallID(&sdkmcp.CallToolRequest{}); got != "" {
			t.Fatalf("hostedToolCallID(no meta) = %q, want empty", got)
		}
		req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{}}
		req.Params.Meta = sdkmcp.Meta{"toolCallId": " call-1 "}
		if got, want := hostedToolCallID(req), "call-1"; got != want {
			t.Fatalf("hostedToolCallID() = %q, want %q", got, want)
		}
		req.Params.Meta = sdkmcp.Meta{"toolCallId": 42}
		if got := hostedToolCallID(req); got != "" {
			t.Fatalf("hostedToolCallID(non-string) = %q, want empty", got)
		}
	})

	t.Run("Should convert canonical results and errors", func(t *testing.T) {
		t.Parallel()

		structured, err := hostedToolResult(tools.ToolResult{
			Structured: json.RawMessage(`{"ok":true}`),
			Preview:    "structured fallback",
		})
		if err != nil || structured == nil || structured.IsError {
			t.Fatalf("hostedToolResult(structured) = %#v, %v; want structured result", structured, err)
		}

		text, err := hostedToolResult(tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "hello"}},
		})
		if err != nil || text == nil || text.IsError || len(text.Content) != 1 {
			t.Fatalf("hostedToolResult(text) = %#v, %v; want one text block", text, err)
		}

		fallback, err := hostedToolResult(tools.ToolResult{Preview: "preview"})
		if err != nil || fallback == nil || fallback.IsError {
			t.Fatalf("hostedToolResult(fallback) = %#v, %v; want fallback text result", fallback, err)
		}
		if got, want := hostedResultFallback(tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "a"}, {Type: "text", Text: "b"}},
		}), "a\nb"; got != want {
			t.Fatalf("hostedResultFallback(content) = %q, want %q", got, want)
		}
		if got, want := hostedResultFallback(tools.ToolResult{}), "{}"; got != want {
			t.Fatalf("hostedResultFallback(empty) = %q, want %q", got, want)
		}

		toolErr := tools.NewToolError(
			tools.ErrorCodeApprovalRequired,
			"compozy__hosted_echo",
			"approval timed out",
			tools.ErrToolApprovalRequired,
			tools.ReasonApprovalTimedOut,
		)
		if got := hostedToolErrorMessage(toolErr); !strings.Contains(got, string(tools.ReasonApprovalTimedOut)) {
			t.Fatalf("hostedToolErrorMessage(tool error) = %q, want reason", got)
		}
		codeOnlyErr := tools.NewToolError(
			tools.ErrorCodeBackendFailed,
			"compozy__hosted_echo",
			"backend failed",
			tools.ErrToolBackendFailed,
		)
		if got := hostedToolErrorMessage(codeOnlyErr); !strings.Contains(got, string(tools.ErrorCodeBackendFailed)) {
			t.Fatalf("hostedToolErrorMessage(code-only tool error) = %q, want code", got)
		}
		if got := hostedToolErrorMessage(errors.New("plain")); got != "plain" {
			t.Fatalf("hostedToolErrorMessage(plain) = %q, want plain", got)
		}
		transportErr := hostedToolResponseError{response: contract.ToolErrorResponse{
			Error: contract.ToolErrorPayload{
				Code:        tools.ErrorCodeDenied,
				Message:     "workspace denied: operator hint",
				ReasonCodes: []tools.ReasonCode{tools.ReasonWorkspaceAccessDenied},
			},
		}}
		want := "workspace_access_denied: workspace denied: operator hint"
		if got := hostedToolErrorMessage(transportErr); got != want {
			t.Fatalf("hostedToolErrorMessage(transport) = %q, want reason and public message", got)
		}

		missingPathErr := tools.NewToolError(
			tools.ErrorCodeNotFound,
			tools.ToolIDConfigGet,
			`config path "loops.inputs.extension-probe.preference" not found`,
			tools.ErrToolNotFound,
			tools.ReasonConfigPathNotFound,
		)
		status := core.StatusForToolError(missingPathErr)
		missingPathResponse := core.ToolErrorResponseForError(missingPathErr, status, true)
		got := hostedToolErrorMessage(hostedToolResponseError{response: missingPathResponse})
		if want := "config_path_not_found: config path not found"; got != want {
			t.Fatalf("hostedToolErrorMessage(missing config path) = %q, want %q", got, want)
		}
	})
}

type hostedToolResponseError struct {
	response contract.ToolErrorResponse
}

type countingHostedToolRegistry struct {
	addCalls    int
	removeCalls [][]string
}

func (r *countingHostedToolRegistry) AddTool(_ *sdkmcp.Tool, _ sdkmcp.ToolHandler) {
	r.addCalls++
}

func (r *countingHostedToolRegistry) RemoveTools(names ...string) {
	r.removeCalls = append(r.removeCalls, append([]string(nil), names...))
}

func (e hostedToolResponseError) Error() string {
	return "transport tool error"
}

func (e hostedToolResponseError) Response() contract.ToolErrorResponse {
	return e.response
}

func newHostedProxyTestServer() *sdkmcp.Server {
	return sdkmcp.NewServer(&sdkmcp.Implementation{Name: HostedServerName, Version: "test"}, &sdkmcp.ServerOptions{
		Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{}},
	})
}

func equalJSON(t *testing.T, left, right []byte) bool {
	t.Helper()
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		t.Fatalf("json.Unmarshal(left) error = %v", err)
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		t.Fatalf("json.Unmarshal(right) error = %v", err)
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func listHostedProxyTools(t *testing.T, server *sdkmcp.Server) map[string]*sdkmcp.Tool {
	t.Helper()
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := serverSession.Close(); closeErr != nil {
			t.Errorf("serverSession.Close() error = %v", closeErr)
		}
	})
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "hosted-proxy-test", Version: "test"},
		&sdkmcp.ClientOptions{Capabilities: &sdkmcp.ClientCapabilities{}},
	)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := clientSession.Close(); closeErr != nil {
			t.Errorf("clientSession.Close() error = %v", closeErr)
		}
	})
	listed, err := clientSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("clientSession.ListTools() error = %v", err)
	}
	result := make(map[string]*sdkmcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		result[tool.Name] = tool
	}
	return result
}

func sdkToolNames(tools []*sdkmcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func requireHostedToolMetaString(t *testing.T, meta sdkmcp.Meta, field string) string {
	t.Helper()

	if meta == nil {
		t.Fatalf("tool meta = nil, want %s", field)
	}
	got, ok := meta[field].(string)
	if !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("tool meta %s = %#v, want non-empty string", field, meta[field])
	}
	return got
}

type hostedProxyClientStub struct {
	mu          sync.Mutex
	bind        HostedBindResponse
	callResult  HostedCallResponse
	calls       []HostedCallRequest
	releases    []HostedReleaseRequest
	projections chan HostedProjectionResponse
	applied     chan struct{}
}

var _ HostedProxyClient = (*hostedProxyClientStub)(nil)

func newHostedProxyClientStub(bind HostedBindResponse) *hostedProxyClientStub {
	return &hostedProxyClientStub{
		bind:        bind,
		projections: make(chan HostedProjectionResponse, 4),
		applied:     make(chan struct{}, 4),
	}
}

func (c *hostedProxyClientStub) BindHostedMCP(context.Context, HostedBindRequest) (HostedBindResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bind, nil
}

func (c *hostedProxyClientStub) HostedMCPProjection(context.Context, string) (HostedProjectionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return HostedProjectionResponse{Tools: c.bind.Tools, Digest: c.bind.Digest}, nil
}

func (c *hostedProxyClientStub) StreamHostedMCPProjection(
	ctx context.Context,
	_ string,
	_ string,
	handler HostedProjectionHandler,
) error {
	for {
		select {
		case projection := <-c.projections:
			if err := handler(projection); err != nil {
				return err
			}
			c.applied <- struct{}{}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *hostedProxyClientStub) CallHostedMCP(
	_ context.Context,
	req HostedCallRequest,
) (HostedCallResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, req)
	return c.callResult, nil
}

func (c *hostedProxyClientStub) ReleaseHostedMCP(_ context.Context, req HostedReleaseRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases = append(c.releases, req)
	return nil
}

func (c *hostedProxyClientStub) emitProjection(t *testing.T, projection HostedProjectionResponse) {
	t.Helper()

	c.projections <- projection
	select {
	case <-c.applied:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hosted projection handler")
	}
}

func (c *hostedProxyClientStub) lastCall(t *testing.T) HostedCallRequest {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		t.Fatal("CallHostedMCP was not invoked")
	}
	return c.calls[len(c.calls)-1]
}

func (c *hostedProxyClientStub) releaseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.releases)
}
