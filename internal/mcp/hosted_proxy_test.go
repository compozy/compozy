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
	"github.com/compozy/compozy/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestApplyHostedToolsUsesDescriptorRawSchemas(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve input schemas and widen output schemas with hosted result envelopes", func(t *testing.T) {
		t.Parallel()

		inputSchema := json.RawMessage(
			`{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}`,
		)
		outputSchema := json.RawMessage(`{"type":"object","required":["grants"],` +
			`"properties":{"grants":{"type":"array","items":{"$ref":"#/$defs/grant"}}},` +
			`"additionalProperties":false,` +
			`"$defs":{"grant":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}},` +
			`"additionalProperties":false}}}`)
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
		advertised := compileHostedOutputSchema(t, tool)
		for _, testCase := range []struct {
			name    string
			payload string
			valid   bool
		}{
			{name: "accept a declared payload", payload: `{"grants":[{"id":"grant-1"}]}`, valid: true},
			{name: "reject a payload violating a referenced definition", payload: `{"grants":[{"id":7}]}`},
			{name: "reject a payload with an undeclared field", payload: `{"grants":[],"extra":true}`},
			{
				name: "accept the result continuation",
				payload: `{"truncated":true,"truncated_from_bytes":530000,"preview":"{\"grants\":[",` +
					`"artifacts":[{"uri":"compozy://tool-artifacts/art_1","bytes":530000}],` +
					`"read_tool":"compozy__tool_artifact_read"}`,
				valid: true,
			},
			{
				name:    "accept a typed tool error",
				payload: `{"error":{"code":"tool_denied","message":"denied"}}`,
				valid:   true,
			},
			{name: "reject an untyped error object", payload: `{"error":{}}`},
		} {
			t.Run("Should "+testCase.name, func(t *testing.T) {
				t.Parallel()

				err := validateHostedStructuredContent(t, advertised, json.RawMessage(testCase.payload))
				if testCase.valid && err != nil {
					t.Fatalf("advertised output schema rejected %s: %v", testCase.payload, err)
				}
				if !testCase.valid && err == nil {
					t.Fatalf("advertised output schema accepted %s", testCase.payload)
				}
			})
		}
		declaration, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatalf("json.Marshal(OutputSchema) error = %v", err)
		}
		var root map[string]any
		if err := json.Unmarshal(declaration, &root); err != nil {
			t.Fatalf("json.Unmarshal(OutputSchema) error = %v", err)
		}
		if root["type"] != "object" {
			t.Fatalf("OutputSchema root type = %#v, want object as MCP requires", root["type"])
		}
	})

	t.Run("Should keep declared local references resolvable inside the widened output schema", func(t *testing.T) {
		t.Parallel()

		propertyRef := `"properties":{"value":{"type":"string"},"copy":{"$ref":"#/properties/value"}},` +
			`"additionalProperties":false`
		for _, testCase := range []struct {
			name    string
			schema  string
			valid   string
			invalid string
		}{
			{
				name:    "a property pointer",
				schema:  `{"type":"object",` + propertyRef + `}`,
				valid:   `{"value":"a","copy":"b"}`,
				invalid: `{"copy":7}`,
			},
			{
				name: "a recursive root reference",
				schema: `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},` +
					`"children":{"type":"array","items":{"$ref":"#"}}},"additionalProperties":false}`,
				valid:   `{"name":"root","children":[{"name":"leaf","children":[]}]}`,
				invalid: `{"name":"root","children":[{"children":[]}]}`,
			},
			{
				name: "a nested embedded schema resource",
				schema: `{"type":"object","required":["nested"],"properties":{"nested":` +
					`{"$id":"https://compozy.test/nested.json","type":"object",` + propertyRef + `}}}`,
				valid:   `{"nested":{"value":"a","copy":"b"}}`,
				invalid: `{"nested":{"copy":7}}`,
			},
			{
				name:    "an embedded schema resource",
				schema:  `{"$id":"https://compozy.test/output.json","type":"object",` + propertyRef + `}`,
				valid:   `{"value":"a","copy":"b"}`,
				invalid: `{"copy":7}`,
			},
		} {
			t.Run("Should resolve "+testCase.name, func(t *testing.T) {
				t.Parallel()

				view := hostedToolView("compozy__hosted_echo")
				view.Descriptor.OutputSchema = json.RawMessage(testCase.schema)
				tool := hostedMCPTool(view.Descriptor)
				advertised := compileHostedOutputSchema(t, &tool)
				if err := validateHostedStructuredContent(t, advertised, json.RawMessage(testCase.valid)); err != nil {
					t.Fatalf("advertised output schema rejected declared payload %s: %v", testCase.valid, err)
				}
				if err := validateHostedStructuredContent(
					t,
					advertised,
					json.RawMessage(testCase.invalid),
				); err == nil {
					t.Fatalf("advertised output schema accepted invalid payload %s", testCase.invalid)
				}
				if err := validateHostedStructuredContent(
					t,
					advertised,
					json.RawMessage(`{"truncated":true,"read_tool":"compozy__tool_artifact_read"}`),
				); err != nil {
					t.Fatalf("advertised output schema rejected the result continuation: %v", err)
				}
			})
		}
	})

	t.Run("Should omit an absent output schema from the MCP declaration", func(t *testing.T) {
		t.Parallel()

		view := hostedToolView("compozy__hosted_echo")
		view.Descriptor.OutputSchema = nil
		tool := hostedMCPTool(view.Descriptor)

		payload, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("json.Marshal(Tool) error = %v", err)
		}
		var declaration map[string]any
		if err := json.Unmarshal(payload, &declaration); err != nil {
			t.Fatalf("json.Unmarshal(Tool) error = %v", err)
		}
		if _, present := declaration["outputSchema"]; present {
			t.Fatalf("tool declaration = %s, want outputSchema omitted", payload)
		}
	})
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

	t.Run("Should keep the wait path loaded", func(t *testing.T) {
		t.Parallel()

		mcpServer := newHostedProxyTestServer()
		applyHostedTools(mcpServer, &hostedProxyClientStub{}, "bind-1", []tools.ToolView{
			hostedToolView(tools.ToolIDTerminalExec),
			hostedToolView(tools.ToolIDMonitorCreate),
		})

		registered := listHostedProxyTools(t, mcpServer)
		for _, id := range []tools.ToolID{tools.ToolIDTerminalExec, tools.ToolIDMonitorCreate} {
			tool, ok := registered[id.String()]
			if !ok {
				t.Fatalf("registered tools = %#v, want %s", registered, id)
			}
			if got := tool.Meta["anthropic/alwaysLoad"]; got != true {
				t.Fatalf("%s alwaysLoad = %#v, want true", id, got)
			}
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

func TestRunHostedProxyNotifiesToolListChangedForProjectionChanges(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should notify and refresh the list after late discovery without repeating unchanged updates",
		func(t *testing.T) {
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

			changed := make(chan struct{}, 16)
			mcpClient := sdkmcp.NewClient(
				&sdkmcp.Implementation{Name: "hosted-proxy-test", Version: "1.0.0"},
				&sdkmcp.ClientOptions{
					Capabilities:           &sdkmcp.ClientCapabilities{},
					ToolListChangedHandler: func(context.Context, *sdkmcp.ToolListChangedRequest) { changed <- struct{}{} },
				},
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

			if caps := session.InitializeResult().Capabilities.Tools; caps == nil || !caps.ListChanged {
				t.Fatal("want listChanged advertised")
			}
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

			proxyClient.emitProjection(
				t,
				HostedProjectionResponse{Tools: []tools.ToolView{initial, updated}, Digest: "added"},
			)
			select {
			case <-changed:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for additive discovery notification")
			}
			list, err = session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools(added): %v", err)
			}
			if got, want := sdkToolNames(
				list.Tools,
			), []string{
				"compozy__hosted_echo",
				"compozy__hosted_other",
			}; !slices.Equal(
				got,
				want,
			) {
				t.Fatalf("additive tools=%#v, want %#v", got, want)
			}
			proxyClient.emitProjection(t, HostedProjectionResponse{
				Tools:  []tools.ToolView{updated},
				Digest: "updated",
			})

			select {
			case <-changed:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for tools/list_changed")
			}
			list, err = session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools(updated) error = %v", err)
			}
			if got, want := sdkToolNames(list.Tools), []string{"compozy__hosted_other"}; !slices.Equal(got, want) {
				t.Fatalf("updated tools = %#v, want %#v", got, want)
			}

			result, err = session.CallTool(
				ctx,
				&sdkmcp.CallToolParams{Name: "compozy__hosted_other", Arguments: map[string]any{"message": "late"}},
			)
			if err != nil || result == nil || result.IsError {
				t.Fatalf("CallTool(late) result = %#v, error = %v", result, err)
			}

			if len(result.Content) != 1 {
				t.Fatalf("late content = %#v, want one text block", result.Content)
			}
			lateText, isText := result.Content[0].(*sdkmcp.TextContent)
			if !isText || lateText.Text != "called" {
				t.Fatalf("late content = %#v, want called", result.Content)
			}
			if !equalJSON(t, proxyClient.lastCall(t).Input, json.RawMessage(`{"message":"late"}`)) {
				t.Fatalf("late forwarded input = %s", proxyClient.lastCall(t).Input)
			}
			lateCall := proxyClient.lastCall(t)
			if lateCall.BindID != "bind-1" || lateCall.ToolName != "compozy__hosted_other" {
				t.Fatalf("late call = %#v", lateCall)
			}
			if got, want := list.CacheScope, "private"; got != want {
				t.Fatalf("tools/list cache scope = %q, want %q", got, want)
			}
			if list.TTLMs <= 0 {
				t.Fatalf("tools/list TTL = %d, want positive", list.TTLMs)
			}

			// Drain any coalesced addition/removal notifications before replaying a fallback refresh.
			select {
			case <-changed:
			case <-time.After(100 * time.Millisecond):
			}
			for range 3 {
				proxyClient.emitProjection(
					t,
					HostedProjectionResponse{Tools: []tools.ToolView{updated}, Digest: "updated"},
				)
			}
			proxyClient.emitProjection(
				t,
				HostedProjectionResponse{Tools: []tools.ToolView{updated}, Digest: "same-tools-new-digest"},
			)
			select {
			case <-changed:
				t.Fatal("unchanged tools emitted tools/list_changed")
			case <-time.After(100 * time.Millisecond):
			}
			proxyClient.emitProjection(t, HostedProjectionResponse{Digest: "removed"})
			select {
			case <-changed:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for tool removal notification")
			}
			list, err = session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools(removed): %v", err)
			}
			if len(list.Tools) != 0 {
				t.Fatalf("removed tools = %#v, want none", sdkToolNames(list.Tools))
			}
			cancel()
			if err := session.Close(); err != nil {
				t.Fatalf("session.Close() error = %v", err)
			}
			select {
			case runErr := <-errCh:
				if !isExpectedStdioServerTermination(runErr) && !errors.Is(runErr, io.ErrClosedPipe) {
					t.Fatalf("RunHostedProxy() error = %v", runErr)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for hosted proxy shutdown")
			}
			if got := proxyClient.releaseCount(); got != 1 {
				t.Fatalf("ReleaseHostedMCP calls = %d, want 1", got)
			}
		},
	)
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
		decode := func(value any) error {
			t.Helper()
			timer := time.AfterFunc(2*time.Second, func() {
				if err := clientReader.CloseWithError(errors.New("MCP response timed out")); err != nil {
					t.Errorf("close timed-out reader: %v", err)
				}
			})
			defer timer.Stop()
			return decoder.Decode(value)
		}
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
				Capabilities    struct {
					Tools struct {
						ListChanged bool `json:"listChanged"`
					} `json:"tools"`
				} `json:"capabilities"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		}
		if err := decode(&initialized); err != nil {
			t.Fatalf("decode initialize response error = %v", err)
		}
		if len(initialized.Error) > 0 && string(initialized.Error) != "null" {
			t.Fatalf("initialize response error = %s", initialized.Error)
		}
		if got, want := initialized.Result.ProtocolVersion, protocolVersionProviderLegacy; got != want {
			t.Fatalf("initialize protocol version = %q, want %q", got, want)
		}

		if !initialized.Result.Capabilities.Tools.ListChanged {
			t.Fatal("provider protocol must advertise listChanged")
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
		if err := decode(&listed); err != nil {
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

		proxyClient.emitProjection(
			t,
			HostedProjectionResponse{Tools: []tools.ToolView{hostedToolView("mcp__late__echo")}, Digest: "late"},
		)
		var notification struct {
			Method string `json:"method"`
		}
		if err := decode(&notification); err != nil {
			t.Fatalf("decode change notification: %v", err)
		}
		if notification.Method != "notifications/tools/list_changed" {
			t.Fatalf("notification = %q", notification.Method)
		}
		if err := encoder.Encode(
			map[string]any{"jsonrpc": "2.0", "id": 3, "method": mcpToolsListMethod, "params": map[string]any{}},
		); err != nil {
			t.Fatal(err)
		}
		if err := decode(&listed); err != nil {
			t.Fatal(err)
		}

		if len(listed.Error) > 0 && string(listed.Error) != "null" {
			t.Fatalf("late tools/list error = %s", listed.Error)
		}
		if len(listed.Result.Tools) != 1 || listed.Result.Tools[0].Name != "mcp__late__echo" {
			t.Fatalf("late tools = %#v", listed.Result.Tools)
		}
		if err := encoder.Encode(
			map[string]any{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  "tools/call",
				"params":  map[string]any{"name": "mcp__late__echo", "arguments": map[string]any{"message": "late"}},
			},
		); err != nil {
			t.Fatal(err)
		}
		var called struct {
			ID     json.Number            `json:"id"`
			Result *sdkmcp.CallToolResult `json:"result"`
			Error  json.RawMessage        `json:"error"`
		}
		if err := decode(&called); err != nil {
			t.Fatalf("decode late call: %v", err)
		}
		if string(called.ID) != "4" {
			t.Fatalf("late response id = %q, want 4", called.ID)
		}
		if called.Result == nil || called.Result.IsError || (len(called.Error) > 0 && string(called.Error) != "null") {
			t.Fatalf("late call response = %#v", called)
		}
		lateCall := proxyClient.lastCall(t)
		if !equalJSON(t, lateCall.Input, json.RawMessage(`{"message":"late"}`)) {
			t.Fatalf("late provider forwarded input = %s", lateCall.Input)
		}
		if lateCall.BindID != "bind-provider" || lateCall.ToolName != "mcp__late__echo" {
			t.Fatalf("late provider call = %#v", lateCall)
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

// TestHostedProxyHelpers verifies hosted result projection, fallback content, error classification, and exact JSON deduplication.
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

		identical, err := hostedToolResult(tools.ToolResult{
			Structured: json.RawMessage(`{"ok":true,"value":42}`),
			Preview:    "  {\"value\":42, \"ok\":true}\n",
		})
		if err != nil || identical == nil || len(identical.Content) != 1 {
			t.Fatalf("hostedToolResult(identical preview) = %#v, %v; want one text result", identical, err)
		}

		precise, err := hostedToolResult(tools.ToolResult{
			Structured: json.RawMessage(`{"n":9007199254740992}`),
			Preview:    `{"n":9007199254740993}`,
		})
		if err != nil || precise == nil || len(precise.Content) != 2 {
			t.Fatalf(
				"hostedToolResult(adjacent large integers) = %#v, %v; want distinct preview and exact result",
				precise,
				err,
			)
		}
		exact, ok := precise.Content[1].(*sdkmcp.TextContent)
		if !ok || exact.Text != `{"n":9007199254740992}` {
			t.Fatalf("exact structured text = %#v, want original integer", precise.Content[1])
		}

		structured, err := hostedToolResult(tools.ToolResult{
			Structured: json.RawMessage(`{"ok":true}`),
			Preview:    "structured fallback",
		})
		if err != nil || structured == nil || structured.IsError {
			t.Fatalf("hostedToolResult(structured) = %#v, %v; want structured result", structured, err)
		}
		var textResult any
		previewPresent := false
		for _, block := range structured.Content {
			content, ok := block.(*sdkmcp.TextContent)
			if !ok {
				continue
			}
			previewPresent = previewPresent || content.Text == "structured fallback"
			if json.Valid([]byte(content.Text)) {
				if err := json.Unmarshal([]byte(content.Text), &textResult); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !previewPresent || !reflect.DeepEqual(textResult, structured.StructuredContent) {
			t.Fatalf(
				"text content = %#v, want preview and complete structured result %#v",
				structured.Content,
				structured.StructuredContent,
			)
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
		terminalEnvelope := contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
			Code: "service_unavailable", Message: "terminal service unavailable",
			Details: &contract.TerminalErrorDetails{Path: "/workspace", Platform: "windows"},
		}}
		gotStructured := hostedToolErrorStructuredContent(hostedTerminalEnvelopeError{response: terminalEnvelope})
		if !reflect.DeepEqual(gotStructured, terminalEnvelope) {
			t.Fatalf("hostedToolErrorStructuredContent(terminal) = %#v, want %#v", gotStructured, terminalEnvelope)
		}

		missingPathResponse := contract.ToolErrorResponse{Error: contract.ToolErrorPayload{
			Code:        tools.ErrorCodeNotFound,
			ToolID:      tools.ToolIDConfigGet,
			Message:     "config path not found",
			ReasonCodes: []tools.ReasonCode{tools.ReasonConfigPathNotFound},
		}}
		got := hostedToolErrorMessage(hostedToolResponseError{response: missingPathResponse})
		if want := "config_path_not_found: config path not found"; got != want {
			t.Fatalf("hostedToolErrorMessage(missing config path) = %q, want %q", got, want)
		}

		invalidSkillResponse := contract.ToolErrorResponse{Error: contract.ToolErrorPayload{
			Code:        tools.ErrorCodeInvalidInput,
			ToolID:      tools.ToolIDSkillView,
			Message:     "skill definition is invalid",
			ReasonCodes: []tools.ReasonCode{tools.ReasonSkillDefinitionInvalid},
			Details: map[string]json.RawMessage{
				contract.ToolOperatorCauseDetailKey: json.RawMessage(
					`"/workspace/.agents/skills/broken/SKILL.md: yaml: line 2"`,
				),
				contract.ToolOperatorRecoveryDetailKey: json.RawMessage(
					`"Fix the SKILL.md YAML frontmatter and retry skill_view."`,
				),
			},
		}}
		invalidSkillErr := hostedToolResponseError{response: invalidSkillResponse}
		if got := hostedToolErrorMessage(
			invalidSkillErr,
		); got != "skill_definition_invalid: skill definition is invalid"+
			"\nCause: /workspace/.agents/skills/broken/SKILL.md: yaml: line 2"+
			"\nRecovery: Fix the SKILL.md YAML frontmatter and retry skill_view." {
			t.Fatalf("hostedToolErrorMessage(invalid skill) = %q, want reason with safe cause and recovery", got)
		}
		deniedSpawnErr := hostedToolResponseError{response: contract.ToolErrorResponse{Error: contract.ToolErrorPayload{
			Code:        tools.ErrorCodeDenied,
			ToolID:      tools.ToolIDSessionSpawn,
			Message:     "tool invocation denied",
			ReasonCodes: []tools.ReasonCode{tools.ReasonPolicyDenied},
			Details: map[string]json.RawMessage{
				contract.ToolOperatorCauseDetailKey: json.RawMessage(
					`"session: spawn permission denied: workspace_paths value \"/wt\" widens parent permissions"`,
				),
				"internal_trace": json.RawMessage(`"must stay hidden"`),
			},
		}}}
		if got := hostedToolErrorMessage(deniedSpawnErr); !strings.HasPrefix(got,
			"policy_denied: tool invocation denied\nCause: session: spawn permission denied: workspace_paths") ||
			strings.Contains(got, "must stay hidden") || strings.Contains(got, "Recovery:") {
			t.Fatalf("hostedToolErrorMessage(denied spawn) = %q, want only the safe refused-field cause", got)
		}
		deniedStructured, ok := hostedToolErrorStructuredContent(deniedSpawnErr).(contract.ToolErrorResponse)
		if !ok {
			t.Fatalf("hostedToolErrorStructuredContent(denied spawn) = %T, want tool error response", deniedStructured)
		}
		if _, kept := deniedStructured.Error.Details[contract.ToolOperatorCauseDetailKey]; !kept {
			t.Fatalf("structured denied spawn details = %#v, want operator cause", deniedStructured.Error.Details)
		}
		if _, leaked := deniedStructured.Error.Details["internal_trace"]; leaked {
			t.Fatalf(
				"structured denied spawn details = %#v, want internal_trace filtered",
				deniedStructured.Error.Details,
			)
		}
		if got := hostedToolErrorStructuredContent(invalidSkillErr); !reflect.DeepEqual(got, invalidSkillResponse) {
			t.Fatalf("hostedToolErrorStructuredContent(invalid skill) = %#v, want %#v", got, invalidSkillResponse)
		}
	})
}

type hostedToolResponseError struct {
	response contract.ToolErrorResponse
}

type hostedTerminalEnvelopeError struct {
	response contract.TerminalErrorResponse
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

func (e hostedTerminalEnvelopeError) Error() string {
	return "terminal transport error"
}

func (e hostedTerminalEnvelopeError) TerminalErrorEnvelope() contract.TerminalErrorResponse {
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
	clientSession := connectHostedProxyClient(t, server)
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

func connectHostedProxyClient(t *testing.T, server *sdkmcp.Server) *sdkmcp.ClientSession {
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
	return clientSession
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
	block       func(HostedCallRequest) (HostedCallResponse, error)
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
	c.calls = append(c.calls, req)
	block := c.block
	c.mu.Unlock()
	if block != nil {
		return block(req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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

func TestHostedSubagentAnnotations(t *testing.T) {
	t.Run("Should retain actionable invalid request errors through the hosted transport", func(t *testing.T) {
		t.Parallel()
		failure := tools.NewToolError(
			tools.ErrorCodeInvalidRequest,
			tools.ToolIDSubagentStatus,
			"subagent_id is required.",
			tools.ErrToolInvalidInput,
			tools.ReasonSchemaInvalid,
		)
		response := contract.ToolErrorResponse{
			Error: contract.ToolErrorPayload{
				Code:        tools.ErrorCodeInvalidRequest,
				Message:     failure.Message,
				ReasonCodes: failure.ReasonCodes,
			},
		}
		for _, err := range []error{failure, hostedToolResponseError{response: response}} {
			if got := hostedToolErrorMessage(err); got != "invalid_request: subagent_id is required." {
				t.Fatal(got)
			}
		}
	})
	t.Run("Should preserve read and acknowledgement semantics over hosted MCP", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			id                   tools.ToolID
			readOnly, idempotent bool
		}{
			{tools.ToolIDSubagentCapabilities, true, true}, {tools.ToolIDSubagentStatus, false, true}, {tools.ToolIDSubagentDelegate, false, false}, {tools.ToolIDSubagentCancel, false, false}, {"compozy__other", false, true},
		} {
			projected := hostedMCPTool(tools.Descriptor{
				ID: tc.id, ReadOnly: tc.readOnly,
				ToolExecutionMetadata: tools.NewToolExecutionMetadata(tc.idempotent, "", 0),
			})
			if projected.Annotations.ReadOnlyHint != tc.readOnly ||
				projected.Annotations.IdempotentHint != tc.idempotent {
				t.Fatalf("annotations = %#v", projected.Annotations)
			}
		}
	})
}

// Invariant: the hosted fallback identity comes from the JSON-RPC request, preserving provider tool IDs and retries.
// Owner: hosted MCP transport; canonical hosted proxy suite.
func TestHostedRequestIdentity(t *testing.T) {
	t.Run("Should return per-request invalid params and keep the hosted connection usable", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
		server := newHostedProxyTestServer()
		session, err := server.Connect(ctx, hostedIdentityTransport{Transport: serverTransport}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := session.Close(); err != nil {
				t.Error(err)
			}
		})
		conn, err := clientTransport.Connect(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		})
		call := func(raw string) *jsonrpc.Response {
			t.Helper()
			message, err := jsonrpc.DecodeMessage([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.Write(ctx, message); err != nil {
				t.Fatal(err)
			}
			message, err = conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			response, ok := message.(*jsonrpc.Response)
			if !ok {
				t.Fatalf("response = %T", message)
			}
			return response
		}
		response := call(
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"test"}}}`,
		)
		if response.Error != nil {
			t.Fatal(response.Error)
		}
		for _, params := range []string{`[]`, `"invalid"`, `42`, `{"name":"echo","_meta":[]}`, `{"name":"echo","_meta":"invalid"}`} {
			response = call(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":` + params + `}`)
			rpcErr, ok := errors.AsType[*jsonrpc.Error](response.Error)
			if !ok || rpcErr.Code != jsonrpc.CodeInvalidParams || response.ID.Raw() != int64(2) {
				t.Fatalf("params %s: response = %#v, error = %v", params, response, response.Error)
			}
		}
		response = call(`{"jsonrpc":"2.0","id":3,"method":"ping","params":{}}`)
		if response.Error != nil || response.ID.Raw() != int64(3) {
			t.Fatalf("ping after malformed calls = %#v", response)
		}
	})

	t.Run("Should preserve stable request identity without trusting caller metadata", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{
			`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"compozy__subagent_delegate","arguments":{}}}`,
			`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"compozy__subagent_delegate","arguments":{},"_meta":{"toolCallId":"provider-call","compozyHostedRequestId":"spoof"}}}`,
		} {
			transport := hostedIdentityTransport{
				Transport: &sdkmcp.IOTransport{
					Reader: io.NopCloser(strings.NewReader(raw + "\n")),
					Writer: nopWriteCloser{Writer: io.Discard},
				},
			}
			conn, err := transport.Connect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			message, err := conn.Read(t.Context())
			closeErr := conn.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			request, ok := message.(*jsonrpc.Request)
			if !ok {
				t.Fatalf("unexpected message %T", message)
			}
			params := new(sdkmcp.CallToolParamsRaw)
			if err := json.Unmarshal(request.Params, params); err != nil {
				t.Fatal(err)
			}
			req := &sdkmcp.CallToolRequest{Params: params}
			if hostedRequestIdentity(req) != "42" {
				t.Fatal(hostedRequestIdentity(req))
			}
			if strings.Contains(raw, "provider-call") && hostedToolCallID(req) != "provider-call" {
				t.Fatal(hostedToolCallID(req))
			}
		}
	})
}
