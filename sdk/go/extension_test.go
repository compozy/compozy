package aghsdk_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aghsdk "github.com/compozy/agh/sdk/go"
)

type digestFixture struct {
	Name      string          `json:"name"`
	Schema    json.RawMessage `json:"schema"`
	Canonical string          `json:"canonical"`
	SHA256    string          `json:"sha256"`
}

func TestToolRegistrationValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should Reject Invalid Tool Definitions", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name    string
			handler string
			options aghsdk.ToolOptions
		}{
			{
				name:    "Should Reject Empty Handler",
				handler: " ",
				options: validToolOptions(),
			},
			{
				name:    "Should Reject Missing Input Schema",
				handler: "search",
				options: aghsdk.ToolOptions{ReadOnly: true},
			},
			{
				name:    "Should Reject Non Object Schema",
				handler: "search",
				options: aghsdk.ToolOptions{ReadOnly: true, InputSchema: []any{"bad"}},
			},
			{
				name:    "Should Reject Invalid Explicit ID",
				handler: "search",
				options: aghsdk.ToolOptions{
					ID:          "Ext.Bad",
					ReadOnly:    true,
					InputSchema: map[string]any{"type": "object"},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				extension := newTestExtension()
				err := extension.Tool(tc.handler, tc.options, func(
					context.Context,
					aghsdk.ToolRequest[json.RawMessage],
				) (aghsdk.ToolResult, error) {
					return aghsdk.EmptyResult(), nil
				})
				if err == nil {
					t.Fatal("Tool() error = nil, want validation error")
				}
			})
		}
	})

	t.Run("Should Reject Duplicate Handlers", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		if err := extension.Tool("search", validToolOptions(), rawOKHandler); err != nil {
			t.Fatalf("Tool() first registration error = %v", err)
		}
		if err := extension.Tool("search", validToolOptions(), rawOKHandler); err == nil {
			t.Fatal("Tool() duplicate error = nil, want error")
		}
	})

	t.Run("Should Reject Duplicate Explicit Tool IDs", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		options := validToolOptions()
		options.ID = "ext__duplicate__tool"
		if err := extension.Tool("search", options, rawOKHandler); err != nil {
			t.Fatalf("Tool() first registration error = %v", err)
		}
		if err := extension.Tool("lookup", options, rawOKHandler); err == nil {
			t.Fatal("Tool() duplicate id error = nil, want error")
		}
	})

	t.Run("Should Accept Raw JSON Schemas", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		options := aghsdk.ToolOptions{
			ReadOnly:     true,
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			OutputSchema: []byte(`{"type":"object"}`),
		}
		if err := extension.Tool("search", options, rawOKHandler); err != nil {
			t.Fatalf("Tool() raw schema registration error = %v", err)
		}
	})

	t.Run("Should Reserve Tool Provider Methods", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		if err := extension.Handle("provide_tools", func(
			context.Context,
			aghsdk.ExtensionContext,
			json.RawMessage,
		) (any, error) {
			return nil, nil
		}); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		if err := extension.Tool("search", validToolOptions(), rawOKHandler); err == nil {
			t.Fatal("Tool() error = nil, want reserved method error")
		}
	})
}

func TestSchemaDigestFixturesMatchDaemonAndTypeScript(t *testing.T) {
	t.Parallel()

	fixtures := readDigestFixtures(t)
	for _, fixture := range fixtures {
		t.Run("Should Match Fixture "+fixture.Name, func(t *testing.T) {
			t.Parallel()

			canonical, err := aghsdk.CanonicalJSON(fixture.Schema)
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			if got := string(canonical); got != fixture.Canonical {
				t.Fatalf("CanonicalJSON() = %q, want %q", got, fixture.Canonical)
			}

			digest, err := aghsdk.SchemaDigest(fixture.Schema)
			if err != nil {
				t.Fatalf("SchemaDigest() error = %v", err)
			}
			if digest != fixture.SHA256 {
				t.Fatalf("SchemaDigest() = %q, want %q", digest, fixture.SHA256)
			}
		})
	}
}

func TestHostAPIRejectsSensitiveParams(t *testing.T) {
	t.Parallel()

	t.Run("Should Reject Raw Sensitive Values Before Transport", func(t *testing.T) {
		t.Parallel()

		transport := &recordingTransport{}
		host := aghsdk.NewHostAPI(transport, func() bool { return true })
		err := host.Request(
			context.Background(),
			aghsdk.HostAPIMethodNetworkSend,
			map[string]any{"claim_token": "agh_claim_secret"},
			&json.RawMessage{},
		)
		if err == nil {
			t.Fatal("HostAPI.Request() error = nil, want sensitive value rejection")
		}
		if transport.calls != 0 {
			t.Fatalf("transport calls = %d, want 0", transport.calls)
		}
	})

	t.Run("Should Reject Calls Before Initialize", func(t *testing.T) {
		t.Parallel()

		transport := &recordingTransport{}
		host := aghsdk.NewHostAPI(transport, func() bool { return false })
		err := host.Request(context.Background(), aghsdk.HostAPIMethodSessionsList, nil, &json.RawMessage{})
		if err == nil {
			t.Fatal("HostAPI.Request() error = nil, want not initialized")
		}
		if transport.calls != 0 {
			t.Fatalf("transport calls = %d, want 0", transport.calls)
		}
	})
}

func TestStdioRuntimeProvidesAndCallsTools(t *testing.T) {
	t.Parallel()

	t.Run("Should Serve Initialize ProvideTools And ToolsCall", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{Name: "Go Tool", Version: "0.1.0"},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
		type searchInput struct {
			Query string `json:"query"`
		}
		if err := aghsdk.Tool[searchInput](
			extension,
			"search",
			validToolOptions(),
			func(_ context.Context, req aghsdk.ToolRequest[searchInput]) (aghsdk.ToolResult, error) {
				return aghsdk.TextResult("result:" + req.Input.Query), nil
			},
		); err != nil {
			t.Fatalf("Tool() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- extension.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("close input error = %v", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("extension runtime did not stop")
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Go Tool"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		var initResult aghsdk.InitializeResponse
		decodeResult(t, initialize.Result, &initResult)
		if !contains(initResult.ImplementedMethods, "provide_tools") ||
			!contains(initResult.ImplementedMethods, "tools/call") {
			t.Fatalf("implemented methods = %#v, want tool provider methods", initResult.ImplementedMethods)
		}

		provideTools := runtime.call(t, 2, "provide_tools", map[string]any{})
		if provideTools.Error != nil {
			t.Fatalf("provide_tools error = %#v", provideTools.Error)
		}
		var provided aghsdk.ExtensionProvideToolsResponse
		decodeResult(t, provideTools.Result, &provided)
		if len(provided.Tools) != 1 {
			t.Fatalf("provided tools = %d, want 1", len(provided.Tools))
		}
		if got, want := provided.Tools[0].ID, aghsdk.ToolID("ext__go_tool__search"); got != want {
			t.Fatalf("provided tool id = %q, want %q", got, want)
		}

		call := runtime.call(t, 3, "tools/call", map[string]any{
			"tool_id": "ext__go_tool__search",
			"handler": "search",
			"input":   map[string]any{"query": "alpha"},
		})
		if call.Error != nil {
			t.Fatalf("tools/call error = %#v", call.Error)
		}
		var callResult aghsdk.ExtensionToolCallResponse
		decodeResult(t, call.Result, &callResult)
		if len(callResult.Result.Content) != 1 || callResult.Result.Content[0].Text != "result:alpha" {
			t.Fatalf("tool result = %#v, want result:alpha", callResult.Result)
		}
	})

	t.Run("Should Redact Sensitive Input From Handler Errors", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{Name: "Go Tool", Version: "0.1.0"},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
		type sensitiveInput struct {
			Secret string `json:"secret"`
		}
		options := validToolOptions()
		options.SensitiveInputFields = []string{"secret"}
		if err := aghsdk.Tool[sensitiveInput](
			extension,
			"search",
			options,
			func(_ context.Context, req aghsdk.ToolRequest[sensitiveInput]) (aghsdk.ToolResult, error) {
				return aghsdk.ToolResult{}, fmt.Errorf("bad secret %s", req.Input.Secret)
			},
		); err != nil {
			t.Fatalf("Tool() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- extension.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("close input error = %v", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("extension runtime did not stop")
			}
		})

		if response := runtime.call(t, 1, "initialize", initializeParams("Go Tool")); response.Error != nil {
			t.Fatalf("initialize error = %#v", response.Error)
		}
		call := runtime.call(t, 2, "tools/call", map[string]any{
			"tool_id": "ext__go_tool__search",
			"handler": "search",
			"input":   map[string]any{"query": "alpha", "secret": "top-secret"},
		})
		if call.Error == nil {
			t.Fatal("tools/call error = nil, want ToolExecutionError")
		}
		encoded, err := json.Marshal(call.Error)
		if err != nil {
			t.Fatalf("json.Marshal(error) error = %v", err)
		}
		if bytes.Contains(encoded, []byte("top-secret")) {
			t.Fatalf("error payload leaked sensitive input: %s", string(encoded))
		}
		if !bytes.Contains(encoded, []byte("[REDACTED]")) {
			t.Fatalf("error payload = %s, want redaction marker", string(encoded))
		}
	})
}

func TestStdioRuntimeProvidesAndCallsWatchSource(t *testing.T) {
	t.Parallel()

	t.Run("Should Serve Initialize WatchSourceKinds And WatchPoll", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{Name: "Go Watch", Version: "0.1.0"},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
		type watchSpec struct {
			Kind  string `json:"kind"`
			Query string `json:"query"`
		}
		if err := aghsdk.WatchSource[watchSpec](
			extension,
			"reviews",
			aghsdk.WatchSourceOptions{},
			func(_ context.Context, req aghsdk.WatchSourceRequest[watchSpec]) (aghsdk.WatchPollResponse, error) {
				if req.ExpectedStateDigest != "sha256:previous" {
					t.Fatalf("ExpectedStateDigest = %q, want sha256:previous", req.ExpectedStateDigest)
				}
				return aghsdk.WatchPollResponse{
					Ready:       req.Spec.Query == "open",
					StateDigest: "sha256:next",
					Payload:     json.RawMessage(`{"review":"r1"}`),
				}, nil
			},
		); err != nil {
			t.Fatalf("WatchSource() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- extension.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("close input error = %v", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("extension runtime did not stop")
			}
		})

		params := initializeParams("Go Watch")
		params["capabilities"].(map[string]any)["provides"] = []string{"loop.watch_source"}
		params["methods"].(map[string]any)["extension_services"] = []string{"watch/poll"}
		initialize := runtime.call(t, 1, "initialize", params)
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		var initResult aghsdk.InitializeResponse
		decodeResult(t, initialize.Result, &initResult)
		if !contains(initResult.ImplementedMethods, "watch/poll") {
			t.Fatalf("implemented methods = %#v, want watch/poll", initResult.ImplementedMethods)
		}
		if got, want := initResult.WatchSourceKinds, []string{"reviews"}; !slices.Equal(got, want) {
			t.Fatalf("watch source kinds = %#v, want %#v", got, want)
		}

		call := runtime.call(t, 2, "watch/poll", map[string]any{
			"spec":                  map[string]any{"kind": "reviews", "query": "open"},
			"expected_state_digest": "sha256:previous",
		})
		if call.Error != nil {
			t.Fatalf("watch/poll error = %#v", call.Error)
		}
		var pollResult aghsdk.WatchPollResponse
		decodeResult(t, call.Result, &pollResult)
		if !pollResult.Ready || pollResult.StateDigest != "sha256:next" {
			t.Fatalf("watch poll result = %#v, want ready sha256:next", pollResult)
		}
		if got, want := string(pollResult.Payload), `{"review":"r1"}`; got != want {
			t.Fatalf("watch poll payload = %s, want %s", got, want)
		}
	})
}

func TestSDKHasNoDaemonInternalImports(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-deps", ".")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps . error = %v\n%s", err, string(output))
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		if strings.HasPrefix(line, "github.com/compozy/agh/internal/") {
			t.Fatalf("sdk/go imports daemon internal package %q", line)
		}
	}
}

func TestExternalConsumerBuildsAgainstPublicSDK(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}
	dir := t.TempDir()
	writeText(
		t,
		filepath.Join(dir, "go.mod"),
		"module example.com/agh-sdk-consumer\n\ngo 1.26.4\n\nrequire github.com/compozy/agh v0.0.0\n",
	)
	writeText(t, filepath.Join(dir, "main.go"), `package main

import (
	"context"

	aghsdk "github.com/compozy/agh/sdk/go"
)

type input struct {
	Query string `+"`json:\"query\"`"+`
}

func main() {
	extension := aghsdk.NewExtension(aghsdk.ExtensionDefinition{Name: "consumer", Version: "0.1.0"})
	if err := aghsdk.Tool[input](extension, "search", aghsdk.ToolOptions{
		ReadOnly: true,
		InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, aghsdk.ToolRequest[input]) (aghsdk.ToolResult, error) {
		return aghsdk.TextResult("ok"), nil
	}); err != nil {
		panic(err)
	}
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	edit := exec.CommandContext(
		ctx,
		"go",
		"mod",
		"edit",
		"-replace",
		"github.com/compozy/agh="+repoRoot,
	)
	edit.Dir = dir
	if output, err := edit.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit replace error = %v\n%s", err, string(output))
	}
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external consumer go test error = %v\n%s", err, string(output))
	}
}

func newTestExtension() *aghsdk.Extension {
	return aghsdk.NewExtension(
		aghsdk.ExtensionDefinition{Name: "test-extension", Version: "0.1.0"},
		aghsdk.WithStderr(io.Discard),
	)
}

func validToolOptions() aghsdk.ToolOptions {
	return aghsdk.ToolOptions{
		ReadOnly:    true,
		InputSchema: map[string]any{"type": "object"},
	}
}

func rawOKHandler(context.Context, aghsdk.ToolRequest[json.RawMessage]) (aghsdk.ToolResult, error) {
	return aghsdk.EmptyResult(), nil
}

func readDigestFixtures(t *testing.T) []digestFixture {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("test-fixtures", "digest", "cases.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(digest fixtures) error = %v", err)
	}
	var fixtures []digestFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("json.Unmarshal(digest fixtures) error = %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("digest fixtures are empty")
	}
	return fixtures
}

type recordingTransport struct {
	calls     int
	rawResult json.RawMessage
}

func (t *recordingTransport) Handle(string, aghsdk.TransportHandler) {}

func (t *recordingTransport) Call(_ context.Context, _ string, _ any, result any) error {
	t.calls++
	if result != nil && len(t.rawResult) > 0 {
		if err := json.Unmarshal(t.rawResult, result); err != nil {
			return err
		}
	}
	return nil
}

func (t *recordingTransport) Run(context.Context) error {
	return nil
}

func (t *recordingTransport) Close() error {
	return nil
}

type rpcResponse struct {
	JSONRPC string                     `json:"jsonrpc"`
	ID      int                        `json:"id"`
	Result  json.RawMessage            `json:"result,omitempty"`
	Error   *aghsdk.JSONRPCErrorObject `json:"error,omitempty"`
}

type runtimeHarness struct {
	extensionInput  *io.PipeReader
	daemonWriter    *io.PipeWriter
	daemonReader    *bufio.Reader
	extensionOutput *io.PipeWriter
}

func newRuntimeHarness(t *testing.T) *runtimeHarness {
	t.Helper()

	extensionInput, daemonWriter := io.Pipe()
	daemonReaderPipe, extensionOutput := io.Pipe()
	return &runtimeHarness{
		extensionInput:  extensionInput,
		daemonWriter:    daemonWriter,
		daemonReader:    bufio.NewReader(daemonReaderPipe),
		extensionOutput: extensionOutput,
	}
}

func (h *runtimeHarness) call(t *testing.T, id int, method string, params any) rpcResponse {
	t.Helper()

	frame := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", method, err)
	}
	if _, err := h.daemonWriter.Write(append(encoded, '\n')); err != nil {
		t.Fatalf("daemon write %s error = %v", method, err)
	}
	line, err := h.daemonReader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("daemon read %s response error = %v", method, err)
	}
	var response rpcResponse
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatalf("json.Unmarshal(%s response %s) error = %v", method, string(line), err)
	}
	if response.ID != id {
		t.Fatalf("response id = %d, want %d", response.ID, id)
	}
	return response
}

func (h *runtimeHarness) closeInput() error {
	return h.daemonWriter.Close()
}

func initializeParams(name string) map[string]any {
	return map[string]any{
		"protocol_version":            "1",
		"supported_protocol_versions": []string{"1"},
		"agh_version":                 "0.5.0",
		"session_nonce":               "nonce",
		"extension": map[string]any{
			"name":        name,
			"version":     "0.1.0",
			"source_tier": "user",
		},
		"capabilities": map[string]any{
			"provides":                []string{"tool.provider"},
			"granted_actions":         []string{},
			"granted_security":        []string{},
			"granted_resource_kinds":  []string{},
			"granted_resource_scopes": []string{},
		},
		"methods": map[string]any{
			"daemon_requests":    []string{"health_check", "shutdown"},
			"extension_services": []string{"provide_tools", "tools/call"},
		},
		"runtime": map[string]any{
			"health_check_interval_ms": 30000,
			"health_check_timeout_ms":  5000,
			"shutdown_timeout_ms":      10000,
			"default_hook_timeout_ms":  5000,
		},
	}
}

func decodeResult(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()

	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("json.Unmarshal(result %s) error = %v", string(raw), err)
	}
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func writeText(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}
