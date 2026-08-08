package compozysdk_test

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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	compozysdk "github.com/compozy/compozy/sdk/go"
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
			options compozysdk.ToolOptions
		}{
			{
				name:    "Should Reject Empty Handler",
				handler: " ",
				options: validToolOptions(),
			},
			{
				name:    "Should Reject Missing Input Schema",
				handler: "search",
				options: compozysdk.ToolOptions{ReadOnly: true},
			},
			{
				name:    "Should Reject Non Object Schema",
				handler: "search",
				options: compozysdk.ToolOptions{ReadOnly: true, InputSchema: []any{"bad"}},
			},
			{
				name:    "Should Reject Invalid Explicit ID",
				handler: "search",
				options: compozysdk.ToolOptions{
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
					compozysdk.ToolRequest[json.RawMessage],
				) (compozysdk.ToolResult, error) {
					return compozysdk.EmptyResult(), nil
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
		options := compozysdk.ToolOptions{
			ReadOnly:     true,
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			OutputSchema: []byte(`{"type":"object"}`),
		}
		if err := extension.Tool("search", options, rawOKHandler); err != nil {
			t.Fatalf("Tool() raw schema registration error = %v", err)
		}
	})

	t.Run("Should Project Typed Registration Into Describe Payload", func(t *testing.T) {
		t.Parallel()

		type searchInput struct {
			Query string `json:"query"`
		}
		inputSchema := json.RawMessage(`{
  "type": "object",
  "required": ["query"],
  "properties": {"query": {"type": "string"}}
}`)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{
				Name:    "describe-fixture",
				Version: "0.1.0",
				Resources: compozysdk.DescribeResources{
					Skills:     []string{" skills/zeta ", "skills/alpha", "skills/zeta"},
					Loops:      []string{" loops/zeta ", "loops/alpha", "loops/zeta"},
					Agents:     []string{" agents/zeta ", "agents/alpha", "agents/zeta"},
					Automation: []string{" automation/zeta ", "automation/alpha", "automation/zeta"},
					Layouts:    []string{" layouts/zeta ", "layouts/alpha", "layouts/zeta"},
				},
				Subprocess: compozysdk.DescribeSubprocess{Command: "./bin"},
			},
			compozysdk.WithStderr(io.Discard),
		)
		if err := compozysdk.Tool[searchInput](
			extension,
			"search",
			compozysdk.ToolOptions{
				Description:         "Search extension data",
				ReadOnly:            true,
				RequiresInteraction: true,
				InputSchema:         inputSchema,
				Command: &compozysdk.ExtensionCommandSpec{
					Verb: "review/search", Summary: "Search reviews",
					Flags: map[string]string{"query": "query"},
				},
			},
			func(context.Context, compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
				return compozysdk.EmptyResult(), nil
			},
		); err != nil {
			t.Fatalf("Tool() error = %v", err)
		}
		if err := extension.CommandGroup("review", "Review commands"); err != nil {
			t.Fatalf("CommandGroup() error = %v", err)
		}

		payload, err := extension.Describe()
		if err != nil {
			t.Fatalf("Describe() error = %v", err)
		}
		if len(payload.Tools) != 1 {
			t.Fatalf("len(Describe().Tools) = %d, want 1", len(payload.Tools))
		}
		digest, err := compozysdk.SchemaDigest(inputSchema)
		if err != nil {
			t.Fatalf("SchemaDigest() error = %v", err)
		}
		tool := payload.Tools[0]
		if tool.Handler != "search" || tool.Description != "Search extension data" {
			t.Fatalf("Describe().Tools[0] = %#v, want registered metadata", tool)
		}
		if tool.InputSchemaDigest != digest {
			t.Fatalf("InputSchemaDigest = %q, want %q", tool.InputSchemaDigest, digest)
		}
		if !jsonEqual(tool.InputSchema, inputSchema) {
			t.Fatalf("InputSchema = %s, want %s", tool.InputSchema, inputSchema)
		}
		if !tool.RequiresInteraction || tool.Command == nil || tool.Command.Verb != "review/search" ||
			tool.Command.Flags["query"] != "query" {
			t.Fatalf("Describe().Tools[0] command metadata = %#v", tool)
		}
		if len(payload.CommandGroups) != 1 || payload.CommandGroups[0].Path != "review" ||
			payload.CommandGroups[0].Summary != "Review commands" {
			t.Fatalf("Describe().CommandGroups = %#v", payload.CommandGroups)
		}
		wantResources := compozysdk.DescribeResources{
			Skills:     []string{"skills/alpha", "skills/zeta"},
			Loops:      []string{"loops/alpha", "loops/zeta"},
			Agents:     []string{"agents/alpha", "agents/zeta"},
			Automation: []string{"automation/alpha", "automation/zeta"},
			Layouts:    []string{"layouts/alpha", "layouts/zeta"},
		}
		if !reflect.DeepEqual(payload.Resources, wantResources) {
			t.Fatalf("Describe().Resources = %#v, want %#v", payload.Resources, wantResources)
		}
	})

	t.Run("Should Reserve Tool Provider Methods", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		if err := extension.Handle("provide_tools", func(
			context.Context,
			compozysdk.ExtensionContext,
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

			canonical, err := compozysdk.CanonicalJSON(fixture.Schema)
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			if got := string(canonical); got != fixture.Canonical {
				t.Fatalf("CanonicalJSON() = %q, want %q", got, fixture.Canonical)
			}

			digest, err := compozysdk.SchemaDigest(fixture.Schema)
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
		host := compozysdk.NewHostAPI(transport, func() bool { return true })
		err := host.Request(
			context.Background(),
			compozysdk.HostAPIMethodNetworkSend,
			map[string]any{"claim_token": "compozy_claim_secret"},
			&json.RawMessage{},
		)
		if err == nil {
			t.Fatal("HostAPI.Request() error = nil, want sensitive value rejection")
		}
		if transport.calls != 0 {
			t.Fatalf("transport calls = %d, want 0", transport.calls)
		}
	})

	t.Run("Should Reject Uppercase Raw Claim Token Under Benign Key", func(t *testing.T) {
		t.Parallel()

		transport := &recordingTransport{}
		host := compozysdk.NewHostAPI(transport, func() bool { return true })
		err := host.Request(
			context.Background(),
			compozysdk.HostAPIMethodNetworkSend,
			map[string]any{"note": "COMPOZY_CLAIM_EXTENSION_SECRET"},
			&json.RawMessage{},
		)
		if err == nil {
			t.Fatal("HostAPI.Request() error = nil, want raw claim token rejection")
		}
		if transport.calls != 0 {
			t.Fatalf("transport calls = %d, want 0", transport.calls)
		}
	})

	t.Run("Should Reject Calls Before Initialize", func(t *testing.T) {
		t.Parallel()

		transport := &recordingTransport{}
		host := compozysdk.NewHostAPI(transport, func() bool { return false })
		err := host.Request(context.Background(), compozysdk.HostAPIMethodSessionsList, nil, &json.RawMessage{})
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
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Go Tool", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		type searchInput struct {
			Query string `json:"query"`
		}
		workspaceSeen := make(chan *compozysdk.ExtensionToolWorkspaceScope, 1)
		if err := compozysdk.Tool[searchInput](
			extension,
			"search",
			validToolOptions(),
			func(_ context.Context, req compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
				workspaceSeen <- req.TrustedWorkspace
				return compozysdk.TextResult("result:" + req.Input.Query), nil
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
		var initResult compozysdk.InitializeResponse
		decodeResult(t, initialize.Result, &initResult)
		if !contains(initResult.ImplementedMethods, "provide_tools") ||
			!contains(initResult.ImplementedMethods, "tools/call") {
			t.Fatalf("implemented methods = %#v, want tool provider methods", initResult.ImplementedMethods)
		}

		provideTools := runtime.call(t, 2, "provide_tools", map[string]any{})
		if provideTools.Error != nil {
			t.Fatalf("provide_tools error = %#v", provideTools.Error)
		}
		var provided compozysdk.ExtensionProvideToolsResponse
		decodeResult(t, provideTools.Result, &provided)
		if len(provided.Tools) != 1 {
			t.Fatalf("provided tools = %d, want 1", len(provided.Tools))
		}
		if got, want := provided.Tools[0].ID, compozysdk.ToolID("ext__go_tool__search"); got != want {
			t.Fatalf("provided tool id = %q, want %q", got, want)
		}

		call := runtime.call(t, 3, "tools/call", map[string]any{
			"tool_id":       "ext__go_tool__search",
			"handler":       "search",
			"invocation_id": "invocation-1",
			"trusted_workspace": map[string]any{
				"id":   "workspace-1",
				"root": "/workspace/one",
			},
			"input": map[string]any{"query": "alpha"},
		})
		if call.Error != nil {
			t.Fatalf("tools/call error = %#v", call.Error)
		}
		var callResult compozysdk.ExtensionToolCallResponse
		decodeResult(t, call.Result, &callResult)
		if len(callResult.Result.Content) != 1 || callResult.Result.Content[0].Text != "result:alpha" {
			t.Fatalf("tool result = %#v, want result:alpha", callResult.Result)
		}
		workspace := <-workspaceSeen
		if workspace == nil || workspace.ID != "workspace-1" || workspace.Root != "/workspace/one" {
			t.Fatalf("trusted workspace = %#v, want daemon-authenticated scope", workspace)
		}
	})

	t.Run("Should Redact Sensitive Input From Handler Errors", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Go Tool", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		type sensitiveInput struct {
			Secret string `json:"secret"`
		}
		options := validToolOptions()
		options.SensitiveInputFields = []string{"secret"}
		if err := compozysdk.Tool[sensitiveInput](
			extension,
			"search",
			options,
			func(_ context.Context, req compozysdk.ToolRequest[sensitiveInput]) (compozysdk.ToolResult, error) {
				return compozysdk.ToolResult{}, fmt.Errorf("bad secret %s", req.Input.Secret)
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
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Go Watch", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		type watchSpec struct {
			Kind  string `json:"kind"`
			Query string `json:"query"`
		}
		if err := compozysdk.WatchSource[watchSpec](
			extension,
			"reviews",
			compozysdk.WatchSourceOptions{},
			func(_ context.Context, req compozysdk.WatchSourceRequest[watchSpec]) (compozysdk.WatchPollResponse, error) {
				if req.ExpectedStateDigest != "sha256:previous" {
					t.Fatalf("ExpectedStateDigest = %q, want sha256:previous", req.ExpectedStateDigest)
				}
				return compozysdk.WatchPollResponse{
					Ready:       req.Spec.Query == "open",
					EventKey:    "reviews:r1",
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
		var initResult compozysdk.InitializeResponse
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
		var pollResult compozysdk.WatchPollResponse
		decodeResult(t, call.Result, &pollResult)
		if !pollResult.Ready || pollResult.EventKey != "reviews:r1" || pollResult.StateDigest != "sha256:next" {
			t.Fatalf("watch poll result = %#v, want ready sha256:next", pollResult)
		}
		if got, want := string(pollResult.Payload), `{"review":"r1"}`; got != want {
			t.Fatalf("watch poll payload = %s, want %s", got, want)
		}
	})
}

func TestStdioRuntimeProvidesConnectivityProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should serve every connectivity provider method over stdio", func(t *testing.T) {
		t.Parallel()
		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Go Connectivity", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		if err := compozysdk.ConnectivityProvider(extension, validConnectivityHandlers()); err != nil {
			t.Fatalf("ConnectivityProvider() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
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
		params := initializeParams("Go Connectivity")
		params["capabilities"].(map[string]any)["provides"] = []string{"connectivity.provider"}
		params["methods"].(map[string]any)["extension_services"] = []string{
			"connectivity/establish", "connectivity/status", "connectivity/teardown",
		}
		initialize := runtime.call(t, 1, "initialize", params)
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		var initResult compozysdk.InitializeResponse
		decodeResult(t, initialize.Result, &initResult)
		for _, method := range []string{"connectivity/establish", "connectivity/status", "connectivity/teardown"} {
			if !contains(initResult.ImplementedMethods, method) {
				t.Fatalf("implemented methods = %#v, want %s", initResult.ImplementedMethods, method)
			}
		}
		deadline := time.Now().Add(time.Second).UTC()
		establish := runtime.call(t, 2, "connectivity/establish", map[string]any{
			"tier": "private", "forward_target": "127.0.0.1:43210",
			"challenge_path": "/.well-known/compozy/gateway-challenge/sdk",
			"deadline":       deadline.Format(time.RFC3339Nano),
		})
		if establish.Error != nil {
			t.Fatalf("connectivity/establish error = %#v", establish.Error)
		}
		var reachability compozysdk.ConnectivityReachability
		decodeResult(t, establish.Result, &reachability)
		if reachability.Tier != "private" || reachability.Health != "healthy" || len(reachability.Endpoints) != 1 {
			t.Fatalf("connectivity reachability = %#v", reachability)
		}
		status := runtime.call(t, 3, "connectivity/status", map[string]any{"tier": "private"})
		if status.Error != nil {
			t.Fatalf("connectivity/status error = %#v", status.Error)
		}
		var statusReachability compozysdk.ConnectivityReachability
		decodeResult(t, status.Result, &statusReachability)
		if statusReachability.Tier != "private" || statusReachability.Health != "healthy" || len(statusReachability.Endpoints) != 1 {
			t.Fatalf("connectivity status = %#v", statusReachability)
		}
		teardown := runtime.call(t, 4, "connectivity/teardown", map[string]any{
			"tier": "private", "deadline": deadline.Format(time.RFC3339Nano),
		})
		if teardown.Error != nil {
			t.Fatalf("connectivity/teardown error = %#v", teardown.Error)
		}
		var stopped compozysdk.ConnectivityTeardownResponse
		decodeResult(t, teardown.Result, &stopped)
		if !stopped.Stopped {
			t.Fatal("connectivity teardown stopped = false")
		}
		malformed := runtime.call(t, 5, "connectivity/status", map[string]any{"tier": 42})
		if malformed.Error == nil {
			t.Fatal("connectivity/status malformed error = nil")
		}
		unimplemented := runtime.call(t, 6, "connectivity/not-implemented", map[string]any{})
		if unimplemented.Error == nil {
			t.Fatal("unimplemented connectivity method error = nil")
		}
	})
}

func TestConnectivityProviderRegistrationIsAtomic(t *testing.T) {
	t.Parallel()

	t.Run("Should leave every unreserved method available after a collision", func(t *testing.T) {
		t.Parallel()
		extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{Name: "collision", Version: "0.1.0"})
		if err := extension.Handle(compozysdk.ExtensionServiceMethodConnectivityStatus, noOpExtensionHandler); err != nil {
			t.Fatalf("Handle(status) error = %v", err)
		}
		if err := compozysdk.ConnectivityProvider(extension, validConnectivityHandlers()); err == nil {
			t.Fatal("ConnectivityProvider() error = nil, want reserved-method collision")
		}
		for _, method := range []string{
			compozysdk.ExtensionServiceMethodConnectivityEstablish,
			compozysdk.ExtensionServiceMethodConnectivityTeardown,
		} {
			if err := extension.Handle(method, noOpExtensionHandler); err != nil {
				t.Fatalf("Handle(%s) after failed registration error = %v", method, err)
			}
		}
	})

	t.Run("Should reject duplicate and post-initialize registration", func(t *testing.T) {
		t.Parallel()
		extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{Name: "duplicate", Version: "0.1.0"})
		if err := compozysdk.ConnectivityProvider(extension, validConnectivityHandlers()); err != nil {
			t.Fatalf("ConnectivityProvider(first) error = %v", err)
		}
		if err := compozysdk.ConnectivityProvider(extension, validConnectivityHandlers()); err == nil {
			t.Fatal("ConnectivityProvider(second) error = nil")
		}

		runtime := newRuntimeHarness(t)
		late := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "late", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
		)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- late.Run(ctx) }()
		params := initializeParams("Late Connectivity")
		if response := runtime.call(t, 1, "initialize", params); response.Error != nil {
			t.Fatalf("initialize error = %#v", response.Error)
		}
		if err := compozysdk.ConnectivityProvider(late, validConnectivityHandlers()); err == nil {
			t.Fatal("ConnectivityProvider(after initialize) error = nil")
		}
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
}

func validConnectivityHandlers() compozysdk.ConnectivityProviderHandlers {
	return compozysdk.ConnectivityProviderHandlers{
		Establish: func(
			_ context.Context,
			_ compozysdk.ExtensionContext,
			req compozysdk.ConnectivityEstablishRequest,
		) (compozysdk.ConnectivityReachability, error) {
			return connectivitySDKReachability(req.Tier), nil
		},
		Status: func(
			_ context.Context,
			_ compozysdk.ExtensionContext,
			req compozysdk.ConnectivityStatusRequest,
		) (compozysdk.ConnectivityReachability, error) {
			return connectivitySDKReachability(req.Tier), nil
		},
		Teardown: func(
			context.Context,
			compozysdk.ExtensionContext,
			compozysdk.ConnectivityTeardownRequest,
		) (compozysdk.ConnectivityTeardownResponse, error) {
			return compozysdk.ConnectivityTeardownResponse{Stopped: true}, nil
		},
	}
}

func noOpExtensionHandler(context.Context, compozysdk.ExtensionContext, json.RawMessage) (any, error) {
	return map[string]any{"ok": true}, nil
}

func connectivitySDKReachability(tier string) compozysdk.ConnectivityReachability {
	return compozysdk.ConnectivityReachability{
		Tier: tier,
		Endpoints: []compozysdk.ConnectivityAdvertisedEndpoint{{
			URL: "https://gateway.example.test", Scheme: "https", Stability: "stable",
		}},
		Health: "healthy",
	}
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
		if strings.HasPrefix(line, "github.com/compozy/compozy/internal/") {
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
		"module example.com/compozy-sdk-consumer\n\ngo 1.26.4\n\nrequire github.com/compozy/compozy/sdk/go v0.0.0\n",
	)
	writeText(t, filepath.Join(dir, "main.go"), `package main

import (
	"context"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

type input struct {
	Query string `+"`json:\"query\"`"+`
}

func main() {
	extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{Name: "consumer", Version: "0.1.0"})
	if err := compozysdk.Tool[input](extension, "search", compozysdk.ToolOptions{
		ReadOnly: true,
		InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, compozysdk.ToolRequest[input]) (compozysdk.ToolResult, error) {
		return compozysdk.TextResult("ok"), nil
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
		"github.com/compozy/compozy/sdk/go="+filepath.Join(repoRoot, "sdk", "go"),
	)
	edit.Dir = dir
	if output, err := edit.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit replace error = %v\n%s", err, string(output))
	}
	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = dir
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy error = %v\n%s", err, string(output))
	}
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external consumer go test error = %v\n%s", err, string(output))
	}
}

func newTestExtension() *compozysdk.Extension {
	return compozysdk.NewExtension(
		compozysdk.ExtensionDefinition{Name: "test-extension", Version: "0.1.0"},
		compozysdk.WithStderr(io.Discard),
	)
}

func validToolOptions() compozysdk.ToolOptions {
	return compozysdk.ToolOptions{
		ReadOnly:    true,
		InputSchema: map[string]any{"type": "object"},
	}
}

func rawOKHandler(context.Context, compozysdk.ToolRequest[json.RawMessage]) (compozysdk.ToolResult, error) {
	return compozysdk.EmptyResult(), nil
}

func jsonEqual(left, right []byte) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
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

func (t *recordingTransport) Handle(string, compozysdk.TransportHandler) {}

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
	JSONRPC string                         `json:"jsonrpc"`
	ID      int                            `json:"id"`
	Result  json.RawMessage                `json:"result,omitempty"`
	Error   *compozysdk.JSONRPCErrorObject `json:"error,omitempty"`
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
		"compozy_version":             "0.5.0",
		"session_nonce":               "nonce",
		"extension": map[string]any{
			"name":        name,
			"version":     "0.1.0",
			"source_tier": "user",
		},
		"capabilities": map[string]any{
			"provides":                []string{"tool.provider"},
			"granted_permissions":     []string{},
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
