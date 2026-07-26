package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/tools"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestApplyHostedToolsUsesDescriptorRawSchemas(t *testing.T) {
	t.Parallel()

	inputSchema := json.RawMessage(
		`{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}`,
	)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
	view := hostedToolView("agh__hosted_echo")
	view.Descriptor.InputSchema = inputSchema
	view.Descriptor.OutputSchema = outputSchema

	mcpServer := server.NewMCPServer(HostedServerName, "test", server.WithToolCapabilities(true))
	applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{view})

	registered := mcpServer.ListTools()
	tool, ok := registered["agh__hosted_echo"]
	if !ok {
		t.Fatalf("registered tools = %#v, want agh__hosted_echo", registered)
	}
	if string(tool.Tool.RawInputSchema) != string(inputSchema) {
		t.Fatalf("RawInputSchema = %s, want exact descriptor schema %s", tool.Tool.RawInputSchema, inputSchema)
	}
	if string(tool.Tool.RawOutputSchema) != string(outputSchema) {
		t.Fatalf("RawOutputSchema = %s, want exact descriptor schema %s", tool.Tool.RawOutputSchema, outputSchema)
	}
}

func TestApplyHostedToolsAddsAnthropicMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should set searchHint and omit alwaysLoad for non-search tools", func(t *testing.T) {
		t.Parallel()

		echo := hostedToolView("agh__hosted_echo")
		echo.Descriptor.SearchHints = []string{"echo messages"}
		mcpServer := server.NewMCPServer(HostedServerName, "test", server.WithToolCapabilities(true))
		applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{echo})

		registered := mcpServer.ListTools()
		echoTool, ok := registered["agh__hosted_echo"]
		if !ok {
			t.Fatalf("registered tools = %#v, want agh__hosted_echo", registered)
		}
		echoHint := requireHostedToolMetaString(t, echoTool.Tool.Meta, "anthropic/searchHint")
		if !strings.Contains(echoHint, "agh__hosted_echo") || !strings.Contains(echoHint, "echo messages") {
			t.Fatalf("echo search hint = %q, want canonical ID and descriptor hint", echoHint)
		}
		if _, ok := echoTool.Tool.Meta.AdditionalFields["anthropic/alwaysLoad"]; ok {
			t.Fatalf("echo metadata = %#v, want no alwaysLoad hint", echoTool.Tool.Meta.AdditionalFields)
		}
	})

	t.Run("Should set searchHint and alwaysLoad for tool_search", func(t *testing.T) {
		t.Parallel()

		search := hostedToolView(tools.ToolIDToolSearch)
		search.Descriptor.SearchHints = []string{"find AGH native tools"}
		mcpServer := server.NewMCPServer(HostedServerName, "test", server.WithToolCapabilities(true))
		applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{search})

		registered := mcpServer.ListTools()
		searchTool, ok := registered[tools.ToolIDToolSearch.String()]
		if !ok {
			t.Fatalf("registered tools = %#v, want %s", registered, tools.ToolIDToolSearch)
		}
		searchHint := requireHostedToolMetaString(t, searchTool.Tool.Meta, "anthropic/searchHint")
		if !strings.Contains(searchHint, tools.ToolIDToolSearch.String()) ||
			!strings.Contains(searchHint, "find AGH native tools") {
			t.Fatalf("tool_search hint = %q, want canonical ID and descriptor hint", searchHint)
		}
		if got := searchTool.Tool.Meta.AdditionalFields["anthropic/alwaysLoad"]; got != true {
			t.Fatalf("tool_search alwaysLoad = %#v, want true", got)
		}
	})
}

func TestRunHostedProxyListsCallsAndStreamsProjectionChanges(t *testing.T) {
	t.Parallel()

	initial := hostedToolView("agh__hosted_echo")
	updated := hostedToolView("agh__hosted_other")
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

	var transportLog bytes.Buffer
	transport := mcptransport.NewIO(clientReader, clientWriter, io.NopCloser(&transportLog))
	if err := transport.Start(ctx); err != nil {
		t.Fatalf("transport.Start() error = %v", err)
	}
	mcpClient := mcpclient.NewClient(transport)
	defer func() { _ = mcpClient.Close() }()

	var init sdkmcp.InitializeRequest
	init.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = sdkmcp.Implementation{Name: "hosted-proxy-test", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(ctx, init); err != nil {
		t.Fatalf("Initialize() error = %v; transport log: %s", err, transportLog.String())
	}

	list, err := mcpClient.ListTools(ctx, sdkmcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools(initial) error = %v", err)
	}
	if got, want := sdkToolNames(list.Tools), []string{"agh__hosted_echo"}; !slices.Equal(got, want) {
		t.Fatalf("initial tools = %#v, want %#v", got, want)
	}

	var call sdkmcp.CallToolRequest
	call.Params.Name = "agh__hosted_echo"
	call.Params.Arguments = map[string]any{"message": "hello"}
	call.Params.Meta = &sdkmcp.Meta{
		AdditionalFields: map[string]any{"toolCallId": "call-1", "approvalToken": "ignored"},
	}
	result, err := mcpClient.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool() result = %#v, want non-error result", result)
	}
	observedCall := proxyClient.lastCall(t)
	if observedCall.BindID != "bind-1" || observedCall.ToolName != "agh__hosted_echo" ||
		observedCall.ToolCallID != "call-1" {
		t.Fatalf("hosted call request = %#v, want bind/tool/toolCallId propagated", observedCall)
	}

	proxyClient.emitProjection(t, HostedProjectionResponse{
		Tools:  []tools.ToolView{updated},
		Digest: "updated",
	})

	list, err = mcpClient.ListTools(ctx, sdkmcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools(updated) error = %v", err)
	}
	if got, want := sdkToolNames(list.Tools), []string{"agh__hosted_other"}; !slices.Equal(got, want) {
		t.Fatalf("updated tools = %#v, want %#v", got, want)
	}

	cancel()
	_ = mcpClient.Close()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hosted proxy shutdown")
	}
	if got := proxyClient.releaseCount(); got != 1 {
		t.Fatalf("ReleaseHostedMCP calls = %d, want 1", got)
	}
}

func TestHostedProxyHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should build descriptions from title and descriptor text", func(t *testing.T) {
		t.Parallel()

		view := hostedToolView("agh__hosted_echo")
		view.Descriptor.DisplayTitle = "Echo"
		view.Descriptor.Description = "Send text back"
		view.Descriptor.Toolsets = []tools.ToolsetID{tools.ToolsetIDBootstrap}
		view.Descriptor.Tags = []string{"catalog", "echo"}
		view.Descriptor.SearchHints = []string{"send text back"}
		got := hostedToolDescription(view.Descriptor)
		for _, want := range []string{
			"Echo\n\nSend text back",
			"AGH canonical tool ID: agh__hosted_echo",
			"AGH toolsets: agh__bootstrap",
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
			!strings.Contains(got, "AGH canonical tool ID: agh__hosted_echo") {
			t.Fatalf("hostedToolDescription(title only) = %q, want title and canonical ID", got)
		}
		view.Descriptor.DisplayTitle = ""
		view.Descriptor.Description = "fallback"
		if got := hostedToolDescription(view.Descriptor); !strings.Contains(got, "fallback") ||
			!strings.Contains(got, "AGH canonical tool ID: agh__hosted_echo") {
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

		if got := hostedToolCallID(sdkmcp.CallToolRequest{}); got != "" {
			t.Fatalf("hostedToolCallID(no meta) = %q, want empty", got)
		}
		req := sdkmcp.CallToolRequest{}
		req.Params.Meta = &sdkmcp.Meta{AdditionalFields: map[string]any{"toolCallId": " call-1 "}}
		if got, want := hostedToolCallID(req), "call-1"; got != want {
			t.Fatalf("hostedToolCallID() = %q, want %q", got, want)
		}
		req.Params.Meta = &sdkmcp.Meta{AdditionalFields: map[string]any{"toolCallId": 42}}
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
			"agh__hosted_echo",
			"approval timed out",
			tools.ErrToolApprovalRequired,
			tools.ReasonApprovalTimedOut,
		)
		if got := hostedToolErrorMessage(toolErr); !strings.Contains(got, string(tools.ReasonApprovalTimedOut)) {
			t.Fatalf("hostedToolErrorMessage(tool error) = %q, want reason", got)
		}
		codeOnlyErr := tools.NewToolError(
			tools.ErrorCodeBackendFailed,
			"agh__hosted_echo",
			"backend failed",
			tools.ErrToolBackendFailed,
		)
		if got := hostedToolErrorMessage(codeOnlyErr); !strings.Contains(got, string(tools.ErrorCodeBackendFailed)) {
			t.Fatalf("hostedToolErrorMessage(code-only tool error) = %q, want code", got)
		}
		if got := hostedToolErrorMessage(errors.New("plain")); got != "plain" {
			t.Fatalf("hostedToolErrorMessage(plain) = %q, want plain", got)
		}
	})
}

func sdkToolNames(tools []sdkmcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func requireHostedToolMetaString(t *testing.T, meta *sdkmcp.Meta, field string) string {
	t.Helper()

	if meta == nil {
		t.Fatalf("tool meta = nil, want %s", field)
	}
	got, ok := meta.AdditionalFields[field].(string)
	if !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("tool meta %s = %#v, want non-empty string", field, meta.AdditionalFields[field])
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
