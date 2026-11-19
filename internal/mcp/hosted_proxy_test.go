package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/tools/builtin"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestApplyHostedToolsUsesDescriptorRawSchemas(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve input schemas and omit declared output schemas", func(t *testing.T) {
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
		if tool.OutputSchema != nil {
			t.Fatalf("OutputSchema = %#v, want omitted", tool.OutputSchema)
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
	t.Run("Should preserve result envelopes across output contract registration changes", func(t *testing.T) {
		t.Parallel()
		view := hostedToolView("compozy__hosted_echo")
		proxy := &hostedProxyClientStub{callResult: HostedCallResponse{Result: tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "plain answer"}},
		}}}
		server := newHostedProxyTestServer()
		client := connectHostedProxyClient(t, server)
		for _, schema := range []json.RawMessage{nil, json.RawMessage(`{"type":"object"}`), nil} {
			view.Descriptor.OutputSchema = schema
			applyHostedTools(server, proxy, "bind-1", []tools.ToolView{view})
			listed, err := client.ListTools(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(listed.Tools) != 1 || listed.Tools[0].OutputSchema != nil {
				t.Fatalf("tools/list = %#v", listed)
			}
			result, err := client.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: view.Descriptor.ID.String()})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError || len(result.Content) != 1 {
				t.Fatalf("result = %#v", result)
			}
			text, ok := result.Content[0].(*sdkmcp.TextContent)
			if !ok || text.Text != "plain answer" {
				t.Fatalf("content = %#v", result.Content)
			}
			if len(schema) == 0 {
				if result.StructuredContent != nil {
					t.Fatalf("schema-less result gained structured content: %#v", result.StructuredContent)
				}
			} else {
				payload, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				if !equalJSON(t, payload, []byte(`{"truncated":false,"preview":"plain answer"}`)) {
					t.Fatalf("structured envelope = %s", payload)
				}
			}
		}
	})
}

// Invariant: the proxy keeps dialing while the daemon's socket is not listening yet and binds once
// it is, but returns any answer from the daemon, and a dial failure that outlives the window, at once.
func TestBindHostedMCPRetriesOnlyWhileTheDaemonCannotBeDialed(t *testing.T) {
	t.Parallel()
	dialErr := &net.OpError{Op: "dial", Net: "unix", Err: errors.New("connect: no such file or directory")}
	request := HostedBindRequest{SessionID: "sess-1", Nonce: "nonce"}
	t.Run("Should bind after the daemon starts listening", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			client := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-1"})
			client.bindErrors = []error{fmt.Errorf("cli: request failed: %w", dialErr), dialErr}
			bind, err := bindHostedMCPWithDialRetry(t.Context(), client, request)
			if err != nil || bind.BindID != "bind-1" || client.bindCalls != 3 {
				t.Fatalf("bind = %#v, %v after %d calls; want bind-1 on the third call", bind, err, client.bindCalls)
			}
		})
	})
	t.Run("Should return a daemon refusal without retrying", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			client := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-1"})
			client.bindErrors = []error{ErrHostedNonceInvalid}
			_, err := bindHostedMCPWithDialRetry(t.Context(), client, request)
			if !errors.Is(err, ErrHostedNonceInvalid) || client.bindCalls != 1 {
				t.Fatalf("bind error = %v after %d calls; want the refusal on the first call", err, client.bindCalls)
			}
		})
	})
	t.Run("Should stop when the daemon never listens within the window", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			client := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-1"})
			attempts := int(hostedBindDialRetryWindow/hostedBindDialRetryInterval) + 2
			for range attempts {
				client.bindErrors = append(client.bindErrors, dialErr)
			}
			started := time.Now()
			_, err := bindHostedMCPWithDialRetry(t.Context(), client, request)
			if !isHostedDialError(err) || time.Since(started) != hostedBindDialRetryWindow {
				t.Fatalf("bind error = %v after %s; want the dial error at the window", err, time.Since(started))
			}
		})
	})
}

func TestRunHostedProxyNotifiesToolListChangedForProjectionChanges(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{ErrHostedBindNotFound, ErrHostedBindRequired} {
		t.Run("Should stop projection retries after "+failure.Error(), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				client := newHostedProxyClientStub(HostedBindResponse{})
				client.streamErrors <- failure
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				streamHostedProjection(ctx, client, newHostedProxyTestServer(), HostedBindResponse{})
				if ctx.Err() != nil {
					t.Fatal("retired bind kept retrying until cancellation")
				}
			})
		})
	}
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

			proxyClient.streamErrors <- errors.New("private refresh failure")
			select {
			case <-proxyClient.streamClosed:
			case <-time.After(2 * time.Second):
				t.Fatal("refresh failure was not delivered")
			}
			result, err = session.CallTool(
				ctx,
				&sdkmcp.CallToolParams{
					Name:      "compozy__hosted_echo",
					Arguments: map[string]any{"message": "during reconnect"},
				},
			)
			if err != nil || result.IsError {
				t.Fatalf("proxy stopped serving during reconnect: %v", err)
			}
			proxyClient.projections <- HostedProjectionResponse{Tools: []tools.ToolView{initial}, Digest: "recovered"}
			select {
			case <-proxyClient.applied:
			case <-time.After(2 * HostedProjectionFallbackRefresh):
				t.Fatal("proxy did not reconnect its projection stream")
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
				"protocolVersion": protocolVersionJune2025,
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
		if got, want := initialized.Result.ProtocolVersion, protocolVersionJune2025; got != want {
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

	t.Run("Should keep default artifact pages below the duplicated transport budget", func(t *testing.T) {
		t.Parallel()
		store, err := tools.OpenFilesystemToolArtifactStore(t.Context(), t.TempDir(), tools.ToolArtifactRetention{
			MaxCount: 10, MaxBytes: 1 << 20, MaxAge: time.Hour,
		})
		if err != nil {
			t.Fatal(err)
		}
		scope := workspaceaccess.ScopeTarget("workspace-a", "")
		ref, err := store.Put(t.Context(), scope, []byte(strings.Repeat("x", 128<<10)))
		if err != nil {
			t.Fatal(err)
		}
		page, err := store.ReadPage(t.Context(), scope, ref.URI, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != 32<<10 || page.NextOffset != int64(len(decoded)) || page.EOF {
			t.Fatalf("default page = %#v, want a complete 32 KiB first page", page)
		}
		payload, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		result, err := hostedToolResult(tools.ToolResult{Structured: payload, Preview: "Artifact page"})
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if len(envelope) >= 128<<10 || result.StructuredContent == nil || len(result.Content) != 2 {
			t.Fatalf("hosted artifact envelope = %d bytes, want duplicated payload below 128 KiB", len(envelope))
		}
		maximum, err := store.ReadPage(t.Context(), scope, ref.URI, 0, tools.MaxToolArtifactPageBytes)
		if err != nil || maximum.Bytes != 64<<10 {
			t.Fatalf("explicit maximum page = %d bytes, %v; want compatible 64 KiB read", maximum.Bytes, err)
		}
	})

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
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("hostedToolDescription() = %q, want substring %q", got, want)
			}
		}
		// Discovery remains complete in the dedicated search metadata and registry.
		hint := hostedToolMeta(view.Descriptor)["anthropic/searchHint"].(string)
		for _, term := range []string{"compozy__bootstrap", "catalog", "echo", "send text back"} {
			if !strings.Contains(hint, term) {
				t.Fatalf("search metadata lost %q", term)
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

	t.Run("Should reduce hosted descriptions without changing schemas or identities", func(t *testing.T) {
		t.Parallel()
		var before, after, eagerBefore, eagerAfter int
		for _, descriptor := range builtin.NativeDescriptors() {
			tool := hostedMCPTool(descriptor)
			if tool.Name != descriptor.ID.String() ||
				!reflect.DeepEqual(tool.InputSchema, cloneRaw(descriptor.InputSchema)) {
				t.Fatalf("tool %s changed its identity or input contract", descriptor.ID)
			}
			// The base renderer repeated discovery metadata and invocation boilerplate.
			legacy := strings.TrimSpace(descriptor.Presentation().DisplayTitle)
			if description := strings.TrimSpace(descriptor.Description); description != "" {
				legacy = strings.TrimSpace(legacy + "\n\n" + description)
			}
			legacy = strings.TrimSpace(legacy + "\n\nCompozy canonical tool ID: " + descriptor.ID.String())
			if len(descriptor.Toolsets) > 0 {
				var names []string
				for _, id := range descriptor.Toolsets {
					names = append(names, id.String())
				}
				legacy += "\n\nCompozy toolsets: " + strings.Join(names, ", ")
			}
			if len(descriptor.Tags) > 0 {
				legacy += "\n\nTags: " + strings.Join(cleanedValues(descriptor.Tags), ", ")
			}
			if len(descriptor.SearchHints) > 0 {
				legacy += "\n\nSearch hints: " + strings.Join(cleanedValues(descriptor.SearchHints), ", ")
			}
			legacy += "\n\nCall the harness-returned tool reference."
			for _, removed := range []string{"\n\nCompozy toolsets:", "\n\nTags:", "\n\nSearch hints:", "Call the harness-returned tool reference."} {
				if strings.Contains(tool.Description, removed) {
					t.Fatalf("tool %s repeated discovery metadata %q", descriptor.ID, removed)
				}
			}
			before += len(legacy)
			after += len(tool.Description)
			if tool.Meta["anthropic/alwaysLoad"] == true {
				eagerBefore += len(legacy)
				eagerAfter += len(tool.Description)
			}
		}
		t.Logf(
			"native descriptions before=%d after=%d; eager before=%d after=%d bytes",
			before,
			after,
			eagerBefore,
			eagerAfter,
		)
		if after >= before || eagerAfter >= eagerBefore {
			t.Fatal("hosted descriptions did not shrink")
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
	return connectHostedProxyClientWithOptions(
		t,
		server,
		&sdkmcp.ClientOptions{Capabilities: &sdkmcp.ClientCapabilities{}},
	)
}

func connectHostedProxyClientWithOptions(
	t *testing.T, server *sdkmcp.Server, opts *sdkmcp.ClientOptions,
) *sdkmcp.ClientSession {
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
		opts,
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
	beforeServed func(context.Context, HostedServedRequest)
	servedErr    error
	served       []HostedServedRequest
	streamErrors chan error
	streamClosed chan struct{}
	mu           sync.Mutex
	bind         HostedBindResponse
	bindErrors   []error
	bindCalls    int
	callResult   HostedCallResponse
	block        func(HostedCallRequest) (HostedCallResponse, error)
	calls        []HostedCallRequest
	releases     []HostedReleaseRequest
	projections  chan HostedProjectionResponse
	applied      chan struct{}
}

var _ HostedProxyClient = (*hostedProxyClientStub)(nil)

func newHostedProxyClientStub(bind HostedBindResponse) *hostedProxyClientStub {
	return &hostedProxyClientStub{
		streamErrors: make(chan error, 1),
		streamClosed: make(chan struct{}, 1),
		bind:         bind,
		projections:  make(chan HostedProjectionResponse, 4),
		applied:      make(chan struct{}, 4),
	}
}

func (c *hostedProxyClientStub) BindHostedMCP(context.Context, HostedBindRequest) (HostedBindResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindCalls++
	if len(c.bindErrors) > 0 {
		err := c.bindErrors[0]
		c.bindErrors = c.bindErrors[1:]
		return HostedBindResponse{}, err
	}
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
		case err := <-c.streamErrors:
			c.streamClosed <- struct{}{}
			return err
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

func TestHostedResultViews(t *testing.T) {
	t.Parallel()
	t.Run("Should retain the view selector without advertising output schemas", func(t *testing.T) {
		t.Parallel()
		descriptor := hostedResultViewDescriptor(t)
		tool := hostedMCPTool(descriptor)
		input, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if !equalJSON(t, input, descriptor.InputSchema) || !strings.Contains(string(input), `"result_view"`) {
			t.Fatalf("advertised input lost selector: %s", input)
		}
		if tool.OutputSchema != nil {
			t.Fatalf("OutputSchema = %#v, want omitted", tool.OutputSchema)
		}
	})

	for _, mode := range []string{"truncated", "typed error", "partial error"} {
		t.Run("Should round trip "+mode+" through MCP and real dispatch", func(t *testing.T) {
			t.Parallel()
			descriptor := hostedResultViewDescriptor(t)
			registry := hostedRuntimeRegistry(
				t,
				tools.PolicyInputs{SystemPermissionMode: tools.PermissionModeApproveAll},
				nil,
				tools.NativeTool{
					Descriptor: descriptor,
					Call: func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
						if mode == "partial error" {
							return tools.ToolResult{}, tools.NewToolError(tools.ErrorCodeResultPersistenceFailed, descriptor.ID,
								"could not retain result", tools.ErrToolResultPersistence, tools.ReasonResultPersistenceFailed).
								WithPartialResult(tools.ToolResult{Content: []tools.ToolContent{{Type: "text", Text: "bounded partial"}}})
						}
						if mode == "typed error" {
							return tools.ToolResult{}, tools.NewToolError(
								tools.ErrorCodeInvalidInput,
								descriptor.ID,
								"invalid query",
								tools.ErrToolInvalidInput,
								tools.ReasonSchemaInvalid,
							)
						}
						return tools.ToolResult{
							Truncated: true,
							Preview:   "bounded preview",
							Content:   []tools.ToolContent{{Type: "text", Text: "bounded preview"}},
							Artifacts: []tools.ArtifactRef{{URI: "compozy://tool-artifacts/art_test", Bytes: 9000}},
							Metadata:  map[string]json.RawMessage{"truncated_from_bytes": json.RawMessage(`9000`)},
						}, nil
					},
				},
			)
			scope := tools.Scope{SessionID: "sess-views", WorkspaceID: "ws-views", AgentName: "general"}
			views, err := registry.SessionProjection(t.Context(), scope)
			if err != nil {
				t.Fatal(err)
			}
			proxy := &hostedProxyClientStub{block: func(req HostedCallRequest) (HostedCallResponse, error) {
				result, err := registry.Call(
					t.Context(),
					scope,
					tools.CallRequest{ToolID: tools.ToolID(req.ToolName), Input: req.Input},
				)
				return HostedCallResponse{Result: result}, err
			}}
			server := newHostedProxyTestServer()
			applyHostedTools(server, proxy, "bind-views", views)
			client := connectHostedProxyClient(t, server)
			for _, view := range []string{"compact", "full"} {
				result, err := client.CallTool(
					t.Context(),
					&sdkmcp.CallToolParams{
						Name:      descriptor.ID.String(),
						Arguments: map[string]any{"result_view": view},
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				payload, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "partial error":
					if !result.IsError || result.StructuredContent != nil || len(result.Content) != 2 {
						t.Fatalf("partial result = %#v", result)
					}
					for i, want := range []string{"bounded partial", "result_persistence_failed: could not retain result"} {
						text, ok := result.Content[i].(*sdkmcp.TextContent)
						if !ok || text.Text != want {
							t.Fatalf("partial content[%d] = %#v, want %s", i, result.Content[i], want)
						}
					}
				case "typed error":
					var typed contract.ToolErrorResponse
					if err := json.Unmarshal(payload, &typed); err != nil {
						t.Fatal(err)
					}
					if !result.IsError || typed.Error.Code != tools.ErrorCodeInvalidInput ||
						typed.Error.Message != "invalid query" {
						t.Fatalf("typed error = %s, isError=%t", payload, result.IsError)
					}
					if len(result.Content) == 0 {
						t.Fatal("missing result text")
					}
					text, ok := result.Content[0].(*sdkmcp.TextContent)
					if !ok || text.Text != "schema_invalid: invalid query" {
						t.Fatalf("error text = %#v", result.Content)
					}
				case "truncated":
					var envelope hostedResultEnvelope
					if err := json.Unmarshal(payload, &envelope); err != nil {
						t.Fatal(err)
					}
					if result.IsError || !envelope.Truncated || envelope.Preview != "bounded preview" ||
						envelope.ReadTool != tools.ToolIDToolArtifactRead ||
						envelope.TruncatedFromBytes == nil ||
						*envelope.TruncatedFromBytes != 9000 ||
						len(
							envelope.Artifacts,
						) != 1 || envelope.Artifacts[0].URI != "compozy://tool-artifacts/art_test" || envelope.Artifacts[0].Bytes != 9000 {
						t.Fatalf("continuation = %s", payload)
					}
					if len(result.Content) == 0 {
						t.Fatal("missing result text")
					}
					text, ok := result.Content[0].(*sdkmcp.TextContent)
					if !ok || text.Text != "bounded preview" || len(result.Content) != 2 {
						t.Fatalf("continuation text = %#v", result.Content)
					}
					artifactText, ok := result.Content[1].(*sdkmcp.TextContent)
					if !ok ||
						!equalJSON(
							t,
							[]byte(artifactText.Text),
							[]byte(
								`{"artifacts":[{"uri":"compozy://tool-artifacts/art_test","bytes":9000}],"read_tool":"compozy__tool_artifact_read"}`,
							),
						) {
						t.Fatalf("artifact text = %#v", result.Content[1])
					}
				}
			}
		})
	}

	t.Run("Should round trip both views through MCP and real dispatch", func(t *testing.T) {
		t.Parallel()
		descriptor := hostedResultViewDescriptor(t)
		registry := hostedRuntimeRegistry(t, tools.PolicyInputs{
			SystemPermissionMode: tools.PermissionModeApproveAll,
		}, nil, tools.NativeTool{
			Descriptor: descriptor,
			Call: func(_ context.Context, _ tools.Scope, req tools.CallRequest) (tools.ToolResult, error) {
				if string(req.Input) != "{}" {
					t.Errorf("handler input retained selector: %s", req.Input)
				}
				payload := `{"record":{"id":"r1"}}`
				if req.View == tools.ResultViewFull {
					payload = `{"record":{"id":"r1","detail":"complete"}}`
				}
				return tools.ToolResult{Structured: json.RawMessage(payload)}, nil
			},
		})
		scope := tools.Scope{SessionID: "sess-views", WorkspaceID: "ws-views", AgentName: "general"}
		views, err := registry.SessionProjection(t.Context(), scope)
		if err != nil {
			t.Fatal(err)
		}
		proxy := &hostedProxyClientStub{block: func(req HostedCallRequest) (HostedCallResponse, error) {
			result, err := registry.Call(t.Context(), scope, tools.CallRequest{
				ToolID: tools.ToolID(req.ToolName), ToolCallID: req.ToolCallID, Input: req.Input,
			})
			return HostedCallResponse{Result: result}, err
		}}
		server := newHostedProxyTestServer()
		applyHostedTools(server, proxy, "bind-views", views)
		client := connectHostedProxyClient(t, server)
		listed, err := client.ListTools(t.Context(), nil)
		if err != nil || len(listed.Tools) != 1 {
			t.Fatalf("ListTools=%#v, %v", listed, err)
		}
		if listed.Tools[0].OutputSchema != nil {
			t.Fatal("tools/list advertised outputSchema")
		}
		for _, view := range []string{"", "compact", "full"} {
			arguments := map[string]any{}
			if view != "" {
				arguments[tools.ResultViewInputKey] = view
			}
			result, err := client.CallTool(t.Context(), &sdkmcp.CallToolParams{
				Name: descriptor.ID.String(), Arguments: arguments,
			})
			if err != nil || result.IsError {
				t.Fatalf("CallTool(%q)=%#v, %v", view, result, err)
			}
			payload, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}

			want := `{"record":{"id":"r1"}}`
			if view == "full" {
				want = `{"record":{"id":"r1","detail":"complete"}}`
			}
			if len(result.Content) != 1 {
				t.Fatalf("content = %#v", result.Content)
			}
			text, ok := result.Content[0].(*sdkmcp.TextContent)
			if !ok || !equalJSON(t, []byte(text.Text), []byte(want)) {
				t.Fatalf("text = %#v, want %s", result.Content, want)
			}
			if !equalJSON(t, payload, []byte(want)) {
				t.Fatalf("%q result=%s, want %s", view, payload, want)
			}
		}
	})
}

func hostedResultViewDescriptor(t *testing.T) tools.Descriptor {
	t.Helper()
	descriptor := hostedToolView("compozy__hosted_views").Descriptor
	descriptor.Source = tools.BuiltinSource()
	descriptor.InputSchema = json.RawMessage(`{"type":"object","additionalProperties":false,` +
		`"properties":{"result_view":` + tools.ResultViewSelectorSchema + `}}`)
	compact := json.RawMessage(`{"type":"object","required":["record"],"additionalProperties":false,` +
		`"properties":{"record":{"$ref":"#/$defs/record"}},` +
		`"$defs":{"record":{"type":"object","required":["id"],"additionalProperties":false,` +
		`"properties":{"id":{"type":"string"}}}}}`)
	full := json.RawMessage(`{"type":"object","required":["record"],"additionalProperties":false,` +
		`"properties":{"record":{"$ref":"#/$defs/record"}},` +
		`"$defs":{"record":{"type":"object","required":["id","detail"],"additionalProperties":false,` +
		`"properties":{"id":{"type":"string"},"detail":{"type":"string"}}}}}`)
	descriptor.OutputSchema = compact
	descriptor.SetResultViews(&tools.ResultViews{
		Compact: tools.ResultViewContract{Schema: compact}, Full: tools.ResultViewContract{Schema: full},
	})
	return descriptor
}

func TestHostedResultViewCatalogSize(t *testing.T) {
	t.Parallel()
	t.Run("Should report the full-policy general session tools list size", func(t *testing.T) {
		t.Parallel()
		descriptors := builtin.NativeDescriptors()
		nativeTools := make([]tools.NativeTool, 0, len(descriptors))
		for _, descriptor := range descriptors {
			nativeTools = append(nativeTools, tools.NativeTool{
				Descriptor: descriptor,
				Call: func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
					return tools.ToolResult{}, nil
				},
			})
		}
		// A general session has an operator approval channel; compozy__mcp_allow always asks
		// the operator, so without one it is withheld from the projection.
		registry := hostedRuntimeRegistry(t, tools.PolicyInputs{
			SystemPermissionMode: tools.PermissionModeApproveAll,
			ApprovalAvailable:    true,
		}, nil, nativeTools...)
		views, err := registry.SessionProjection(t.Context(), tools.Scope{
			SessionID: "sess-full-policy", WorkspaceID: "ws-full-policy", AgentName: "general",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != len(descriptors) {
			t.Fatalf("full policy projected %d of %d tools", len(views), len(descriptors))
		}
		server := newHostedProxyTestServer()
		applyHostedTools(server, &hostedProxyClientStub{}, "bind-full-policy", views)
		listed, err := connectHostedProxyClient(t, server).ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(listed)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(after), `"outputSchema"`) {
			t.Fatal("tools/list contains outputSchema")
		}
		if len(listed.Tools) != len(views) || listed.NextCursor != "" {
			t.Fatalf(
				"incomplete tools/list: %d of %d tools, cursor %q",
				len(listed.Tools),
				len(views),
				listed.NextCursor,
			)
		}
		expected := make(map[string]tools.Descriptor, len(views))
		for _, view := range views {
			expected[view.Descriptor.ID.String()] = view.Descriptor
		}
		const preFeatureBaseline = 744687
		withViews := 0
		modelTools := make([]map[string]any, 0, len(listed.Tools))
		for _, tool := range listed.Tools {
			if tool.OutputSchema != nil {
				t.Fatalf("%s advertises outputSchema", tool.Name)
			}
			input, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			descriptor, found := expected[tool.Name]
			if !found {
				t.Fatalf("unexpected or duplicate tool %s", tool.Name)
			}
			delete(expected, tool.Name)
			if !equalJSON(t, input, descriptor.InputSchema) {
				t.Fatalf("%s input schema changed: %s", tool.Name, input)
			}
			if descriptor.ResultViews() != nil {
				var schema struct {
					Properties map[string]json.RawMessage `json:"properties"`
				}
				if err := json.Unmarshal(input, &schema); err != nil {
					t.Fatal(err)
				}
				if !equalJSON(t, schema.Properties[tools.ResultViewInputKey], []byte(tools.ResultViewSelectorSchema)) {
					t.Fatalf("%s lost its result_view selector: %s", tool.Name, input)
				}
				withViews++
			}
			modelTools = append(
				modelTools,
				map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema},
			)
		}
		model, err := json.Marshal(modelTools)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("tools/list: tools=%d views=%d pre-feature=%d after=%d delta=%d model-facing=%d bytes",
			len(listed.Tools), withViews, preFeatureBaseline, len(after), len(after)-preFeatureBaseline, len(model))
		if len(after) > preFeatureBaseline {
			t.Fatalf("tools/list grew beyond pre-feature baseline: %d > %d", len(after), preFeatureBaseline)
		}
	})
}

func TestHostedExternalOutputContracts(t *testing.T) {
	t.Parallel()
	for _, backend := range []tools.BackendKind{tools.BackendMCP, tools.BackendExtensionHost} {
		t.Run("Should retain external output schemas for "+string(backend), func(t *testing.T) {
			t.Parallel()
			view := hostedToolView("compozy__external_echo")
			view.Descriptor.Backend.Kind = backend
			proxy := &hostedProxyClientStub{}
			server := newHostedProxyTestServer()
			client := connectHostedProxyClient(t, server)
			for _, field := range []string{"first", "second"} {
				schema := json.RawMessage(
					`{"type":"object","required":["` + field + `"],"properties":{"` + field + `":{"$ref":"#/$defs/value"}},"additionalProperties":false,"$defs":{"value":{"type":"string"}}}`,
				)
				view.Descriptor.OutputSchema = schema
				applyHostedTools(server, proxy, "bind-1", []tools.ToolView{view})
				listed, err := client.ListTools(t.Context(), nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(listed.Tools) != 1 || listed.Tools[0].OutputSchema == nil {
					t.Fatalf("missing external schema: %#v", listed)
				}
				raw, err := json.Marshal(listed.Tools[0].OutputSchema)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(raw), `"`+field+`"`) ||
					!strings.Contains(string(raw), "#/anyOf/0/$defs/value") {
					t.Fatalf("stale or unrebased schema: %s", raw)
				}
				for _, result := range []tools.ToolResult{
					{Structured: json.RawMessage(`{"` + field + `":"ok"}`)},
					{Content: []tools.ToolContent{{Type: "text", Text: "plain answer"}}},
					{Truncated: true, Preview: "bounded"},
				} {
					proxy.callResult = HostedCallResponse{Result: result}
					response, err := client.CallTool(
						t.Context(),
						&sdkmcp.CallToolParams{Name: view.Descriptor.ID.String()},
					)
					if err != nil {
						t.Fatal(err)
					}
					if response.IsError || response.StructuredContent == nil {
						t.Fatalf("invalid external result: %#v", response)
					}
				}
			}
		})
	}
}

func (c *hostedProxyClientStub) ReportHostedMCPServed(ctx context.Context, req HostedServedRequest) error {
	if c.beforeServed != nil {
		c.beforeServed(ctx, req)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.served = append(c.served, req)
	return c.servedErr
}

// Invariant: tools/list acknowledges the complete projected revision behind that response.
// Owner: bind-scoped hosted proxy; canonical suite: hosted proxy tests (UT-039/UT-113).
func TestHostedProxyServedCatalogReceipt(t *testing.T) {
	t.Parallel()
	t.Run("Should serve tools when receipt persistence fails and report the applied projection", func(t *testing.T) {
		t.Parallel()
		client := newHostedProxyClientStub(HostedBindResponse{})
		client.servedErr = errors.New("retired bind")
		server := newHostedProxyTestServer()
		catalog := new(hostedProxyCatalog)
		server.AddReceivingMiddleware(catalog.middleware(client, "bind-revision"))
		view := hostedToolView("mcp__echo__ping")
		view.Descriptor.Source = tools.SourceRef{
			Kind:          tools.SourceMCP,
			Owner:         "echo",
			RawServerName: "echo",
			RawToolName:   "ping",
		}
		catalog.apply(
			server,
			client,
			"bind-revision",
			HostedProjectionResponse{Tools: []tools.ToolView{view}, Catalog: tools.NewAdvertisedCatalog(nil)},
		)
		session := connectHostedProxyClient(t, server)
		result, err := session.ListTools(t.Context(), nil)
		if err != nil || len(result.Tools) != 1 || result.Tools[0].Name != view.Descriptor.ID.String() {
			t.Fatalf("list=%+v err=%v", result, err)
		}
		client.mu.Lock()
		defer client.mu.Unlock()
		if len(client.served) != 1 ||
			client.served[0].Digest != tools.NewAdvertisedCatalog([]tools.ToolView{view}).Digest {
			t.Fatalf("receipt=%+v", client.served)
		}
	})

	t.Run("Should report only a successfully served catalog revision", func(t *testing.T) {
		t.Parallel()
		client := newHostedProxyClientStub(HostedBindResponse{})
		server := newHostedProxyTestServer()
		catalog := new(hostedProxyCatalog)
		server.AddReceivingMiddleware(catalog.middleware(client, "bind-revision"))
		view := hostedToolView("mcp__echo__ping")
		view.Descriptor.Source = tools.SourceRef{
			Kind:          tools.SourceMCP,
			Owner:         "echo",
			RawServerName: "echo",
			RawToolName:   "ping",
		}
		first := HostedProjectionResponse{
			Tools:   []tools.ToolView{view},
			Catalog: tools.NewAdvertisedCatalog([]tools.ToolView{view}),
		}
		catalog.apply(server, client, "bind-revision", first)
		session := connectHostedProxyClient(t, server)
		if _, err := session.ListTools(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		// Health withdraws execution temporarily, while the entitled revision stays the same.
		catalog.apply(
			server,
			client,
			"bind-revision",
			HostedProjectionResponse{Tools: first.Tools, Catalog: first.Catalog},
		)
		if _, err := session.ListTools(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		removed := tools.NewAdvertisedCatalog(nil)
		catalog.apply(server, client, "bind-revision", HostedProjectionResponse{Catalog: removed})
		client.mu.Lock()
		if len(client.served) != 2 || client.served[0].Digest != first.Catalog.Digest ||
			client.served[1].Digest != first.Catalog.Digest {
			t.Fatal("reported an unserved or health-only revision")
		}
		client.mu.Unlock()
		if _, err := session.ListTools(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		client.mu.Lock()
		defer client.mu.Unlock()
		if len(client.served) != 3 || client.served[2].Digest != removed.Digest ||
			client.served[2].BindID != "bind-revision" {
			t.Fatalf("served receipts = %+v", client.served)
		}
	})

	t.Run("Should honor discovery deadlines while ordered receipts are blocked", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		first, second := hostedBudgetTestView("small", "first"), hostedBudgetTestView("small", "second")
		digest := func(view *tools.ToolView) string {
			t.Helper()
			return tools.AdvertisedCatalogDigest([]tools.ToolView{*view})
		}
		firstDigest, secondDigest := digest(&first), digest(&second)
		entered, release := make(chan struct{}), make(chan struct{})
		secondReceipt := make(chan struct{})
		releaseReceipt := sync.OnceFunc(func() { close(release) })
		client := &hostedProxyClientStub{beforeServed: func(ctx context.Context, req HostedServedRequest) {
			switch req.Digest {
			case firstDigest:
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
			case secondDigest:
				close(secondReceipt)
			}
		}}
		catalog := newHostedProxyCatalog(HostedProxyOptions{
			ToolListThreshold: 1, DiscoveryRefreshTimeout: 50 * time.Millisecond,
		})
		server := newHostedProxyTestServer()
		middleware := catalog.middleware(client, "bind-ordered")
		secondStarted := make(chan struct{})
		var listRequests atomic.Uint32
		server.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
			handle := middleware(next)
			return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
				if method == mcpToolsListMethod && listRequests.Add(1) == 2 {
					close(secondStarted)
				}
				return handle(ctx, method, req)
			}
		})
		catalog.apply(server, client, "bind-ordered", HostedProjectionResponse{Tools: []tools.ToolView{first}})
		session := connectHostedProxyClient(t, server)
		defer releaseReceipt()
		list := func() <-chan hostedListUpdate {
			t.Helper()
			result := make(chan hostedListUpdate, 1)
			go func() {
				list, err := session.ListTools(ctx, nil)
				result <- hostedListUpdate{result: list, err: err}
			}()
			return result
		}
		firstList := list()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("first list did not start reporting its receipt")
		}
		applied := make(chan struct{})
		go func() {
			catalog.apply(server, client, "bind-ordered", HostedProjectionResponse{Tools: []tools.ToolView{second}})
			close(applied)
		}()
		select {
		case <-applied:
		case <-ctx.Done():
			t.Fatal("the blocked receipt held the catalog lock")
		}
		secondList := list()
		select {
		case <-secondStarted:
		case <-ctx.Done():
			t.Fatal("second list did not reach the middleware")
		}
		unready := make(chan int, 1)
		go func() {
			unready <- catalog.awaitDiscovery(t.Context(), []tools.ToolID{"mcp__large__unknown"})
		}()
		select {
		case <-secondReceipt:
			t.Fatal("newer receipt overtook the blocked receipt")
		case pending := <-unready:
			if pending != 1 {
				t.Fatalf("pending = %d, want the unresolved name", pending)
			}
		case <-ctx.Done():
			t.Fatal("the blocked receipt prevented the discovery deadline from expiring")
		}
		releaseReceipt()
		for _, want := range []struct {
			outcome <-chan hostedListUpdate
			name    string
		}{{firstList, first.Descriptor.ID.String()}, {secondList, second.Descriptor.ID.String()}} {
			update := <-want.outcome
			if update.err != nil || !slices.Equal(sdkToolNames(update.result.Tools), []string{want.name}) {
				t.Fatalf("list = %+v, %v, want only %s", update.result, update.err, want.name)
			}
		}
		client.mu.Lock()
		defer client.mu.Unlock()
		if want := []HostedServedRequest{
			{BindID: "bind-ordered", Digest: firstDigest},
			{BindID: "bind-ordered", Digest: secondDigest},
		}; !slices.Equal(client.served, want) {
			t.Fatalf("receipts = %+v, want %+v", client.served, want)
		}
	})
}

func TestHostedProxyBudgetedToolList(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the complete default list payload", func(t *testing.T) {
		t.Parallel()
		views := []tools.ToolView{
			hostedToolView(tools.ToolIDToolSearch), hostedToolView(tools.ToolIDToolInfo),
			hostedToolView("compozy__hosted_echo"),
			hostedBudgetTestView("large", "alpha"), hostedBudgetTestView("large", "beta"),
		}
		want, err := os.ReadFile("testdata/hosted_tools_list_09565e71d.json")
		if err != nil {
			t.Fatal(err)
		}
		server := newHostedProxyTestServer()
		proxy := &hostedProxyClientStub{}
		catalog := new(hostedProxyCatalog)
		catalog.apply(server, proxy, "bind-default", HostedProjectionResponse{Tools: views})
		server.AddReceivingMiddleware(
			catalog.middleware(proxy, "bind-default"),
			privateToolListCacheMiddleware(),
		)
		got := hostedListBytes(t, server)
		if !bytes.Equal(got, bytes.TrimSuffix(want, []byte("\n"))) {
			t.Fatalf("default tools/list payload changed\ngot: %s\nwant: %s", got, want)
		}
	})

	t.Run("Should retain the full list without a callable search tool", func(t *testing.T) {
		t.Parallel()
		server := newHostedProxyTestServer()
		views := []tools.ToolView{hostedBudgetTestView("large", "alpha"), hostedBudgetTestView("large", "beta")}
		catalog := newHostedProxyCatalog(HostedProxyOptions{ToolListThreshold: 1})
		catalog.apply(server, &hostedProxyClientStub{}, "bind-full", HostedProjectionResponse{Tools: views})
		if got := len(listHostedProxyTools(t, server)); got != len(views) {
			t.Fatalf("tools/list count = %d, want %d", got, len(views))
		}
	})

	t.Run("Should bound the directory for many large servers", func(t *testing.T) {
		t.Parallel()
		server := newHostedProxyTestServer()
		views := []tools.ToolView{hostedToolView(tools.ToolIDToolSearch)}
		for i := range hostedDirectoryMaxServers + 2 {
			name := fmt.Sprintf("wide%02d%s", i, strings.Repeat("x", 100))
			views = append(views, hostedBudgetTestView(name, "alpha"), hostedBudgetTestView(name, "beta"))
		}
		catalog := newHostedProxyCatalog(HostedProxyOptions{ToolListThreshold: 1})
		catalog.apply(server, &hostedProxyClientStub{}, "bind-directory", HostedProjectionResponse{Tools: views})
		listed := listHostedProxyTools(t, server)
		desc := listed[tools.ToolIDToolSearch.String()].Description
		if len(listed) != 1 || strings.Count(desc, "(2 tools)") != hostedDirectoryMaxServers ||
			!strings.Contains(desc, "2 more servers") || strings.Contains(desc, strings.Repeat("x", 80)) {
			t.Fatalf("unexpected directory: %q", desc)
		}
	})
}

// The bind advertises discovered callable tools by real name and acknowledges the complete projection.
func TestHostedProxyBudgetedDiscovery(t *testing.T) {
	t.Parallel()
	t.Run("Should activate scoped results, announce them, and withdraw removed tools", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		search, info, listName := tools.ToolIDToolSearch.String(), tools.ToolIDToolInfo.String(), tools.ToolIDToolList.String()
		initial := []tools.ToolView{
			hostedToolView(tools.ToolIDToolSearch), hostedToolView(tools.ToolIDToolInfo),
			hostedToolView(tools.ToolIDToolList),
			hostedBudgetTestView("small", "alpha"), hostedBudgetTestView("small", "beta"),
		}
		deniedView := hostedBudgetTestView("large", "denied")
		deniedView.Decision.Callable = false
		large := []tools.ToolView{
			hostedBudgetTestView("large", "search_match"),
			hostedBudgetTestView("large", "info_match"),
			hostedBudgetTestView("large", "list_match"),
			deniedView,
		}
		full := append(slices.Clone(initial), large...)
		first, second := large[0].Descriptor.ID, large[1].Descriptor.ID
		third, denied := large[2].Descriptor.ID, large[3].Descriptor.ID
		const outside = "mcp__outside__echo"
		proxy := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-budget", Tools: full, Digest: "initial"})
		proxy.block = func(req HostedCallRequest) (HostedCallResponse, error) {
			switch req.ToolName {
			case search:
				return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{first, denied, outside}}, nil
			case info:
				return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{second}}, nil
			case listName:
				return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{third}}, nil
			default:
				return HostedCallResponse{Result: tools.ToolResult{Content: []tools.ToolContent{
					{Type: "text", Text: req.ToolName},
				}}}, nil
			}
		}
		updates, options := hostedListUpdates()
		session := startHostedBudgetProxy(t, proxy, HostedProxyOptions{ToolListThreshold: 2}, options)
		list, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		wantNames := []string{info, listName, search, "mcp__small__alpha", "mcp__small__beta"}
		if got := sdkToolNames(list.Tools); !slices.Equal(got, wantNames) {
			t.Fatalf("initial list = %v, want %v", got, wantNames)
		}
		if !slices.ContainsFunc(list.Tools, func(tool *sdkmcp.Tool) bool {
			return tool.Name == search && strings.Contains(tool.Description, `"large" (4 tools)`)
		}) {
			t.Fatal("the search tool did not advertise the large server")
		}
		requireHostedBudgetReceipt(t, proxy, full)
		call := func(name string) {
			t.Helper()
			result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name})
			if err != nil || result == nil || result.IsError {
				t.Fatalf("call %s = %+v, error = %v", name, result, err)
			}
		}
		requireUpdate := func(want []string, activated string) {
			t.Helper()
			select {
			case update := <-updates:
				if update.err != nil {
					t.Fatal(update.err)
				}
				if got := sdkToolNames(update.result.Tools); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
					t.Fatalf("notified list = %v, want %v", got, want)
				}
				if activated != "" && !slices.ContainsFunc(update.result.Tools, func(tool *sdkmcp.Tool) bool {
					_, alwaysLoad := tool.Meta["anthropic/alwaysLoad"]
					return tool.Name == activated && !alwaysLoad
				}) {
					t.Fatalf("activated tool %s did not preserve normal schema deferral", activated)
				}
			case <-ctx.Done():
				t.Fatal("no tools/list_changed notification")
			}
		}
		call(search)
		withFirst := append(slices.Clone(wantNames), first.String())
		requireUpdate(withFirst, first.String())
		call(first.String())
		if got := proxy.lastCall(t).ToolName; got != first.String() {
			t.Fatalf("forwarded name = %q, want %q", got, first)
		}
		for _, name := range []string{denied.String(), outside} {
			result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name})
			if err == nil && (result == nil || !result.IsError) {
				t.Fatalf("call %s = %+v, want rejection", name, result)
			}
			if got := proxy.lastCall(t).ToolName; got != first.String() {
				t.Fatalf("unlisted %s reached the daemon", name)
			}
		}

		call(info)
		withBoth := append(slices.Clone(withFirst), second.String())
		requireUpdate(withBoth, second.String())
		call(second.String())
		call(listName)
		withAll := append(slices.Clone(withBoth), third.String())
		requireUpdate(withAll, third.String())
		call(third.String())
		requireHostedBudgetReceipt(t, proxy, full)

		refreshed := append(slices.Clone(full[:len(full)-1]), hostedBudgetTestView("large", "replacement"))
		proxy.emitProjection(t, HostedProjectionResponse{Tools: refreshed, Digest: "hidden-tool-changed"})
		requireUpdate(withAll, first.String())
		requireHostedBudgetReceipt(t, proxy, refreshed)

		proxy.emitProjection(t, HostedProjectionResponse{Tools: initial, Digest: "large-server-removed"})
		requireUpdate(wantNames, "")
		requireHostedBudgetReceipt(t, proxy, initial)
	})
}

func TestHostedProxyBudgetedDiscoveryEligibility(t *testing.T) {
	t.Parallel()
	search := hostedToolView(tools.ToolIDToolSearch)

	t.Run("Should accept an already delivered name during an unrelated refresh", func(t *testing.T) {
		t.Parallel()
		catalog := newHostedProxyCatalog(HostedProxyOptions{ToolListThreshold: 1})
		server, proxy := newHostedProxyTestServer(), &hostedProxyClientStub{}
		old := hostedBudgetTestView("small", "old")
		initial := []tools.ToolView{search, old}
		catalog.apply(server, proxy, "bind", HostedProjectionResponse{Tools: initial})
		catalog.delivery.delivered = catalog.delivery.revision
		newTool := hostedBudgetTestView("other", "new")
		catalog.apply(server, proxy, "bind", HostedProjectionResponse{Tools: append(initial, newTool)})

		waiting := catalog.activate(server, proxy, "bind", []tools.ToolID{old.Descriptor.ID, newTool.Descriptor.ID})
		if !slices.Equal(waiting, []tools.ToolID{newTool.Descriptor.ID}) {
			t.Fatalf("waiting for %v, want only %s", waiting, newTool.Descriptor.ID)
		}
		if count, _ := catalog.unready([]tools.ToolID{old.Descriptor.ID, newTool.Descriptor.ID}); count != 1 {
			t.Fatalf("unready count = %d, want only the new name", count)
		}
	})

	t.Run("Should skip a known tool that is forbidden in the bind", func(t *testing.T) {
		t.Parallel()
		catalog := newHostedProxyCatalog(HostedProxyOptions{ToolListThreshold: 1})
		server, proxy := newHostedProxyTestServer(), &hostedProxyClientStub{}
		denied := hostedBudgetTestView("large", "denied")
		denied.Decision.Callable = false
		catalog.apply(server, proxy, "bind", HostedProjectionResponse{Tools: []tools.ToolView{
			search, hostedBudgetTestView("large", "other"), denied,
		}})

		pending := catalog.discover(t.Context(), server, proxy, "bind", []tools.ToolID{denied.Descriptor.ID})
		if pending != 0 {
			t.Fatalf("known forbidden name reported %d pending refreshes", pending)
		}
	})

	for _, name := range []string{"Should keep an unavailable tool deferred", "Should drop an unavailable late discovery"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			catalog := newHostedProxyCatalog(HostedProxyOptions{ToolListThreshold: 1})
			server, proxy := newHostedProxyTestServer(), &hostedProxyClientStub{}
			unavailable := hostedBudgetTestView("large", "unavailable")
			unavailable.Availability.Available = false
			unavailable.Availability.Executable = false
			stable := hostedBudgetTestView("small", "stable")
			initial := []tools.ToolView{search, hostedBudgetTestView("large", "other"), stable}
			catalog.apply(server, proxy, "bind", HostedProjectionResponse{Tools: initial})
			id := unavailable.Descriptor.ID
			late := strings.Contains(name, "late discovery")
			if late {
				catalog.activate(server, proxy, "bind", []tools.ToolID{id})
			}
			initial[2].Availability.Available = false
			initial[2].Availability.Executable = false
			catalog.apply(server, proxy, "bind", HostedProjectionResponse{Tools: append(initial, unavailable)})
			if !late {
				catalog.activate(server, proxy, "bind", []tools.ToolID{id})
			}

			list := listHostedProxyTools(t, server)
			if _, listed := list[id.String()]; listed {
				t.Fatalf("listed unavailable deferred tool %s", id)
			}
			if _, listed := list[stable.Descriptor.ID.String()]; !listed {
				t.Fatalf("withdrew previously listed tool %s", stable.Descriptor.ID)
			}
		})
	}
}

func TestHostedProxyBudgetedDirectoryRefresh(t *testing.T) {
	t.Parallel()
	t.Run("Should remove a fully activated server in the same notification", func(t *testing.T) {
		t.Parallel()
		alpha, beta := hostedBudgetTestView("drained", "alpha"), hostedBudgetTestView("drained", "beta")
		search := tools.ToolIDToolSearch.String()
		proxy := newHostedProxyClientStub(HostedBindResponse{BindID: "bind", Digest: "initial", Tools: []tools.ToolView{
			hostedToolView(tools.ToolIDToolSearch), alpha, beta,
			hostedBudgetTestView("remaining", "alpha"), hostedBudgetTestView("remaining", "beta"),
		}})
		proxy.block = func(HostedCallRequest) (HostedCallResponse, error) {
			return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{alpha.Descriptor.ID, beta.Descriptor.ID}}, nil
		}
		updates, opts := hostedListUpdates()
		session := startHostedBudgetProxy(t, proxy, HostedProxyOptions{ToolListThreshold: 1}, opts)
		result, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: search})
		if err != nil || result.IsError || hostedHasPendingDiscovery(result) {
			t.Fatalf("discovery result = %+v, %v", result, err)
		}
		select {
		case update := <-updates:
			if update.err != nil {
				t.Fatal(update.err)
			}
			names := sdkToolNames(update.result.Tools)
			if !slices.Equal(names, []string{search, alpha.Descriptor.ID.String(), beta.Descriptor.ID.String()}) {
				t.Fatalf("refreshed names = %v", names)
			}
			for _, tool := range update.result.Tools {
				if tool.Name == search && (strings.Contains(tool.Description, `"drained"`) ||
					!strings.Contains(tool.Description, `"remaining" (2 tools)`)) {
					t.Fatalf("refreshed directory = %q", tool.Description)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no list change for activation")
		}
	})
}

func hostedBudgetTestView(server, name string) tools.ToolView {
	view := hostedToolView(tools.ToolID("mcp__" + server + "__" + name))
	view.Descriptor.Source = tools.SourceRef{
		Kind:          tools.SourceMCP,
		Owner:         server,
		RawServerName: server,
		RawToolName:   name,
	}
	view.Descriptor.Backend = tools.BackendRef{Kind: tools.BackendMCP, MCPServer: server, MCPTool: name}
	view.Decision = tools.EffectiveToolDecision{Callable: true, VisibleToSession: true}
	return view
}

func hostedListBytes(t *testing.T, server *sdkmcp.Server) []byte {
	t.Helper()
	list, err := connectHostedProxyClient(t, server).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireHostedBudgetReceipt(t *testing.T, client *hostedProxyClientStub, views []tools.ToolView) {
	t.Helper()
	client.mu.Lock()
	defer client.mu.Unlock()
	want := tools.AdvertisedCatalogDigest(views)
	if len(client.served) == 0 || client.served[len(client.served)-1].Digest != want {
		t.Fatalf("served receipts = %+v, want latest digest %s", client.served, want)
	}
}

type hostedListUpdate struct {
	result *sdkmcp.ListToolsResult
	err    error
}

func hostedListUpdates() (<-chan hostedListUpdate, *sdkmcp.ClientOptions) {
	updates := make(chan hostedListUpdate, 8)
	return updates, &sdkmcp.ClientOptions{
		Capabilities: &sdkmcp.ClientCapabilities{},
		ToolListChangedHandler: func(ctx context.Context, req *sdkmcp.ToolListChangedRequest) {
			result, err := req.Session.ListTools(ctx, nil)
			updates <- hostedListUpdate{result: result, err: err}
		},
	}
}

func startHostedBudgetProxy(
	t *testing.T, proxy *hostedProxyClientStub, proxyOpts HostedProxyOptions, clientOpts *sdkmcp.ClientOptions,
) *sdkmcp.ClientSession {
	t.Helper()
	input, toProxy := io.Pipe()
	fromProxy, output := io.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	exited := make(chan error, 1)
	proxyOpts.SessionID, proxyOpts.Nonce = "sess-budget", "nonce-budget"
	proxyOpts.Stdin, proxyOpts.Stdout = input, output
	go func() {
		exited <- RunHostedProxy(ctx, proxy, proxyOpts)
	}()
	t.Cleanup(func() {
		cancel()
		if err := toProxy.Close(); err != nil {
			t.Error(err)
		}
		if err := fromProxy.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			t.Error("hosted proxy did not exit")
		}
	})
	return connectHostedBudgetTransport(t, fromProxy, toProxy, clientOpts)
}

func connectHostedBudgetTransport(
	t *testing.T, reader io.ReadCloser, writer io.WriteCloser, options *sdkmcp.ClientOptions,
) *sdkmcp.ClientSession {
	t.Helper()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "budget-client", Version: "test"}, options)
	session, err := client.Connect(t.Context(), &sdkmcp.IOTransport{Reader: reader, Writer: writer}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	return session
}

// An early discovery survives until the same bind installs the matching projection.
func TestHostedProxyBudgetedPendingDiscovery(t *testing.T) {
	t.Parallel()
	t.Run("Should activate a discovered name when its projection arrives later", func(t *testing.T) {
		t.Parallel()
		search := tools.ToolIDToolSearch.String()
		late := hostedBudgetTestView("large", "late")
		lateID := late.Descriptor.ID
		views := []tools.ToolView{
			hostedToolView(tools.ToolIDToolSearch),
			hostedBudgetTestView("large", "alpha"), hostedBudgetTestView("large", "beta"),
		}
		proxy := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-late", Tools: views, Digest: "initial"})
		proxy.block = func(req HostedCallRequest) (HostedCallResponse, error) {
			if req.ToolName == search {
				return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{lateID}}, nil
			}
			return HostedCallResponse{}, nil
		}
		updates, opts := hostedListUpdates()
		session := startHostedBudgetProxy(t, proxy, HostedProxyOptions{
			ToolListThreshold: 1, DiscoveryRefreshTimeout: 20 * time.Millisecond,
		}, opts)
		result, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: search})
		if err != nil || !hostedHasPendingDiscovery(result) {
			t.Fatalf("discovery before projection = %#v, %v, want pending notice", result, err)
		}
		proxy.emitProjection(t, HostedProjectionResponse{Tools: append(views, late), Digest: "late"})
		select {
		case update := <-updates:
			if update.err != nil || !slices.Contains(sdkToolNames(update.result.Tools), lateID.String()) {
				t.Fatalf("updated list = %#v, %v, want %s", update.result, update.err, lateID)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("late activation did not notify the client")
		}
		called, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: lateID.String()})
		if err != nil || called.IsError || proxy.lastCall(t).ToolName != lateID.String() {
			t.Fatalf("call after projection = %#v, %v", called, err)
		}
	})

	t.Run("Should expire queued discoveries and bound the number retained", func(t *testing.T) {
		t.Parallel()
		const ttl = 5 * time.Second
		catalog := newHostedProxyCatalog(HostedProxyOptions{PendingDiscoveryTTL: ttl})
		now := time.Unix(1, 0)
		for i := range hostedPendingDiscoveryLimit + 1 {
			catalog.queuePending(tools.ToolID(fmt.Sprintf("mcp__large__tool_%d", i)), now)
		}
		if len(catalog.pending) != hostedPendingDiscoveryLimit {
			t.Fatalf("pending size = %d, want %d", len(catalog.pending), hostedPendingDiscoveryLimit)
		}
		catalog.prunePending(now.Add(ttl - time.Nanosecond))
		if len(catalog.pending) != hostedPendingDiscoveryLimit {
			t.Fatalf("pending size before expiration = %d, want %d", len(catalog.pending), hostedPendingDiscoveryLimit)
		}
		catalog.prunePending(now.Add(ttl))
		if len(catalog.pending) != 0 {
			t.Fatalf("pending size after expiration = %d", len(catalog.pending))
		}
	})

	t.Run("Should use the configured discovery refresh deadline", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			const timeout = 20 * time.Millisecond
			catalog := newHostedProxyCatalog(HostedProxyOptions{DiscoveryRefreshTimeout: timeout})
			started := time.Now()
			pending := catalog.awaitDiscovery(t.Context(), []tools.ToolID{"mcp__large__unknown"})
			if elapsed := time.Since(started); pending != 1 || elapsed != timeout {
				t.Fatalf("pending = %d after %s, want one unresolved name after %s", pending, elapsed, timeout)
			}
		})
	})
}

func hostedHasPendingDiscovery(result *sdkmcp.CallToolResult) bool {
	if result == nil || len(result.Content) == 0 {
		return false
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	return ok && strings.Contains(text.Text, "hosted MCP tool list is still refreshing")
}

// Serving a first page cannot acknowledge a refresh, and a timed-out discovery remains actionable.
func TestHostedProxyBudgetedListDelivery(t *testing.T) {
	t.Parallel()
	t.Run("Should report a delayed final page and allow the call after refresh", func(t *testing.T) {
		t.Parallel()
		search := tools.ToolIDToolSearch.String()
		echo := hostedBudgetTestView("large", "echo")
		echoID := echo.Descriptor.ID
		proxy := &hostedProxyClientStub{block: func(req HostedCallRequest) (HostedCallResponse, error) {
			if req.ToolName == search {
				return HostedCallResponse{DiscoveredToolIDs: []tools.ToolID{echoID}}, nil
			}
			return HostedCallResponse{}, nil
		}}
		catalog := newHostedProxyCatalog(HostedProxyOptions{
			ToolListThreshold: 1, DiscoveryRefreshTimeout: 100 * time.Millisecond,
		})
		server := sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: HostedServerName, Version: "test"},
			&sdkmcp.ServerOptions{
				PageSize:     1,
				Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{ListChanged: true}},
			},
		)
		server.AddReceivingMiddleware(catalog.middleware(proxy, "bind-pages"))
		catalog.apply(server, proxy, "bind-pages", HostedProjectionResponse{Tools: []tools.ToolView{
			hostedToolView(tools.ToolIDToolSearch), echo, hostedBudgetTestView("large", "other"),
		}})
		input, toProxy := io.Pipe()
		fromProxy, output := io.Pipe()
		transport := hostedIdentityTransport{
			Transport: &sdkmcp.IOTransport{
				Reader: input, Writer: nopWriteCloser{Writer: hostedListOutput{Writer: output, catalog: catalog}},
			},
			trackLists: true,
		}
		host, err := server.Connect(t.Context(), transport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := host.Close(); err != nil {
				t.Error(err)
			}
		})
		notices := make(chan struct{}, 4)
		session := connectHostedBudgetTransport(t, fromProxy, toProxy, &sdkmcp.ClientOptions{
			Capabilities:           &sdkmcp.ClientCapabilities{},
			ToolListChangedHandler: func(context.Context, *sdkmcp.ToolListChangedRequest) { notices <- struct{}{} },
		})
		if initial, err := session.ListTools(t.Context(), nil); err != nil || initial.NextCursor != "" {
			t.Fatalf("initial list = %+v, %v, want one page", initial, err)
		}
		type callOutcome struct {
			result *sdkmcp.CallToolResult
			err    error
		}
		discovery := make(chan callOutcome, 1)
		go func() {
			result, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: search})
			discovery <- callOutcome{result: result, err: err}
		}()
		select {
		case <-notices:
		case <-time.After(2 * time.Second):
			t.Fatal("expected activation notification")
		}
		first, err := session.ListTools(t.Context(), nil)
		if err != nil || first.NextCursor == "" || slices.Contains(sdkToolNames(first.Tools), echoID.String()) {
			t.Fatalf("first refreshed page = %+v, %v", first, err)
		}
		select {
		case outcome := <-discovery:
			if outcome.err != nil || !hostedHasPendingDiscovery(outcome.result) {
				t.Fatalf("discovery with missing last page = %#v, %v, want timeout notice", outcome.result, outcome.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("discovery did not return when refresh took too long")
		}
		last, err := session.ListTools(t.Context(), &sdkmcp.ListToolsParams{Cursor: first.NextCursor})
		if err != nil || last.NextCursor != "" || !slices.Contains(sdkToolNames(last.Tools), echoID.String()) {
			t.Fatalf("last refreshed page = %+v, %v", last, err)
		}
		result, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: search})
		if err != nil || hostedHasPendingDiscovery(result) {
			t.Fatalf("discovery after completed refresh = %#v, %v", result, err)
		}
		result, err = session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: echoID.String()})
		if err != nil || result.IsError || proxy.lastCall(t).ToolName != echoID.String() {
			t.Fatalf("activated real-name call = %#v, %v", result, err)
		}
	})

	t.Run("Should accept a current response after older undelivered pages filled the tracker", func(t *testing.T) {
		t.Parallel()
		delivery := &hostedListDelivery{revision: 1}
		record := func(id string) {
			t.Helper()
			delivery.record(&sdkmcp.ListToolsRequest{
				Extra: &sdkmcp.RequestExtra{Header: http.Header{hostedListRequestHeader: {id}}},
			}, &sdkmcp.ListToolsResult{})
		}
		for i := range hostedListPageLimit {
			record(fmt.Sprintf("n:%d", i))
		}
		delivery.revision++
		record("s:new")
		if !delivery.written("s:new") || delivery.delivered != delivery.revision {
			t.Fatalf("delivered revision = %d, want %d", delivery.delivered, delivery.revision)
		}
	})

	t.Run("Should write without waiting for the catalog lock when there are no outstanding pages", func(t *testing.T) {
		t.Parallel()
		catalog := new(hostedProxyCatalog)
		writer := hostedListOutput{Writer: io.Discard, catalog: catalog}
		catalog.mu.Lock()
		defer catalog.mu.Unlock()
		done := make(chan error, 1)
		go func() {
			_, err := writer.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`))
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("untracked stdout write waited for the catalog")
		}
	})
}
