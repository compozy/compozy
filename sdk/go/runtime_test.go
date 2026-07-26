package aghsdk_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	aghsdk "github.com/compozy/agh/sdk/go"
)

func TestStdioTransportBidirectionalCalls(t *testing.T) {
	t.Parallel()

	clientInput, serverOutput := io.Pipe()
	serverInput, clientOutput := io.Pipe()
	client := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
		Input:  clientInput,
		Output: clientOutput,
	})
	server := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
		Input:  serverInput,
		Output: serverOutput,
	})
	server.Handle(
		"echo",
		func(_ context.Context, params json.RawMessage, _ aghsdk.JSONRPCRequestEnvelope) (any, error) {
			var payload map[string]string
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			return map[string]string{"echo": payload["value"]}, nil
		},
	)
	server.Handle("fail", func(context.Context, json.RawMessage, aghsdk.JSONRPCRequestEnvelope) (any, error) {
		return nil, aghsdk.NewInvalidParamsError("forced failure", nil)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- client.Run(ctx) }()
	go func() { done <- server.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		closePipes(t, clientInput, clientOutput, serverInput, serverOutput)
		waitForTransportStops(t, done, 2)
	})

	var echo map[string]string
	if err := client.Call(ctx, "echo", map[string]string{"value": "alpha"}, &echo); err != nil {
		t.Fatalf("client.Call(echo) error = %v", err)
	}
	if echo["echo"] != "alpha" {
		t.Fatalf("echo response = %#v, want alpha", echo)
	}

	var failed map[string]string
	err := client.Call(ctx, "fail", map[string]string{"value": "alpha"}, &failed)
	if err == nil {
		t.Fatal("client.Call(fail) error = nil, want RPC error")
	}
	var rpcErr *aghsdk.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32602 {
		t.Fatalf("client.Call(fail) error = %v, want invalid params RPC error", err)
	}
}

func TestExtensionRuntimeBuiltInAndCustomMethods(t *testing.T) {
	t.Parallel()

	runtime := newRuntimeHarness(t)
	extension := aghsdk.NewExtension(
		aghsdk.ExtensionDefinition{
			Name:    "Memory Extension",
			Version: "0.1.0",
			Capabilities: aghsdk.CapabilitiesConfig{
				Provides: []string{"memory.backend"},
			},
			Actions: aghsdk.ActionsConfig{
				Requires: []aghsdk.HostAPIMethod{aghsdk.HostAPIMethodSessionsList},
			},
			Security: aghsdk.SecurityConfig{
				Capabilities: []string{"memory.read"},
			},
			SupportedHookEvents: []string{"session.started"},
		},
		aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
		aghsdk.WithSDKVersion("test-version"),
		aghsdk.WithStderr(io.Discard),
	)
	if err := extension.Handle("memory/store", func(
		context.Context,
		aghsdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]bool{"stored": true}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}
	if err := extension.Handle("memory/recall", func(
		context.Context,
		aghsdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]any{"entries": []any{}}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/recall) error = %v", err)
	}
	if err := extension.Handle("memory/forget", func(
		context.Context,
		aghsdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]bool{"forgotten": true}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/forget) error = %v", err)
	}
	if err := extension.Handle("health_check", func(
		context.Context,
		aghsdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return aghsdk.HealthCheckResult{Healthy: true, Message: "ok"}, nil
	}); err != nil {
		t.Fatalf("Handle(health_check) error = %v", err)
	}
	if err := extension.Handle("shutdown", func(
		context.Context,
		aghsdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return aghsdk.ShutdownResponse{Acknowledged: true}, nil
	}); err != nil {
		t.Fatalf("Handle(shutdown) error = %v", err)
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

	initialize := runtime.call(t, 1, "initialize", initializeParamsWithGrants(
		"Memory Extension",
		[]string{"memory.backend"},
		[]string{"sessions/list"},
		[]string{"memory.read"},
	))
	if initialize.Error != nil {
		t.Fatalf("initialize error = %#v", initialize.Error)
	}
	var initResult aghsdk.InitializeResponse
	decodeResult(t, initialize.Result, &initResult)
	if initResult.ExtensionInfo.SDKVersion != "test-version" {
		t.Fatalf("sdk version = %q, want test-version", initResult.ExtensionInfo.SDKVersion)
	}
	if !contains(initResult.SupportedHookEvents, "session.started") {
		t.Fatalf("supported hook events = %#v, want session.started", initResult.SupportedHookEvents)
	}

	store := runtime.call(t, 2, "memory/store", map[string]string{"key": "alpha"})
	if store.Error != nil {
		t.Fatalf("memory/store error = %#v", store.Error)
	}
	var stored map[string]bool
	decodeResult(t, store.Result, &stored)
	if !stored["stored"] {
		t.Fatalf("memory/store result = %#v, want stored", stored)
	}

	health := runtime.call(t, 3, "health_check", map[string]any{})
	if health.Error != nil {
		t.Fatalf("health_check error = %#v", health.Error)
	}
	var healthResult aghsdk.HealthCheckResult
	decodeResult(t, health.Result, &healthResult)
	if !healthResult.Healthy || healthResult.Message != "ok" {
		t.Fatalf("health result = %#v, want healthy ok", healthResult)
	}

	shutdown := runtime.call(t, 4, "shutdown", map[string]any{"reason": "test", "deadline_ms": 100})
	if shutdown.Error != nil {
		t.Fatalf("shutdown error = %#v", shutdown.Error)
	}
	var shutdownResult aghsdk.ShutdownResponse
	decodeResult(t, shutdown.Result, &shutdownResult)
	if !shutdownResult.Acknowledged {
		t.Fatalf("shutdown result = %#v, want acknowledged", shutdownResult)
	}

	blocked := runtime.call(t, 5, "memory/recall", map[string]any{})
	if blocked.Error == nil || blocked.Error.Code != -32004 {
		t.Fatalf("post-shutdown response error = %#v, want shutdown in progress", blocked.Error)
	}
}

func TestHostAPIRawRequestAndResultHelpers(t *testing.T) {
	t.Parallel()

	transport := &recordingTransport{rawResult: json.RawMessage(`{"ok":true}`)}
	host := aghsdk.NewHostAPI(transport, func() bool { return true })
	raw, err := host.RawRequest(
		context.Background(),
		aghsdk.HostAPIMethodObserveHealth,
		map[string]string{"scope": "unit"},
	)
	if err != nil {
		t.Fatalf("HostAPI.RawRequest() error = %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("HostAPI.RawRequest() = %s, want ok response", string(raw))
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1", transport.calls)
	}

	empty := aghsdk.EmptyResult()
	if empty.Truncated || empty.Bytes != 0 {
		t.Fatalf("EmptyResult() = %#v, want zero non-truncated result", empty)
	}
	structured, err := aghsdk.StructuredResult(map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("StructuredResult() error = %v", err)
	}
	if string(structured.Structured) != `{"ok":true}` {
		t.Fatalf("StructuredResult() = %s, want JSON payload", string(structured.Structured))
	}
	if _, err := aghsdk.StructuredResult(map[string]any{"bad": make(chan struct{})}); err == nil {
		t.Fatal("StructuredResult() error = nil, want marshal error")
	}
}

func TestValidationAndDigestErrorBranches(t *testing.T) {
	t.Parallel()

	invalidIDs := []aghsdk.ToolID{"", "A", "a_", "a___b", "a__"}
	for _, id := range invalidIDs {
		t.Run("Should Reject ToolID "+string(id), func(t *testing.T) {
			t.Parallel()

			if err := id.Validate(); err == nil {
				t.Fatalf("ToolID(%q).Validate() error = nil, want error", id)
			}
		})
	}

	if _, err := aghsdk.CanonicalJSON(json.RawMessage(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("CanonicalJSON() error = nil, want multiple value error")
	}
	if _, err := aghsdk.CanonicalJSON(json.RawMessage(`{"a":NaN}`)); err == nil {
		t.Fatal("CanonicalJSON() error = nil, want invalid JSON error")
	}
	if _, err := aghsdk.SchemaDigest(json.RawMessage(`[]`)); err == nil {
		t.Fatal("SchemaDigest([]) error = nil, want object error")
	}
	if canonical, err := aghsdk.CanonicalJSON(json.RawMessage(`{"n":1.20e+3}`)); err != nil {
		t.Fatalf("CanonicalJSON(number) error = %v", err)
	} else if string(canonical) != `{"n":1200}` {
		t.Fatalf("CanonicalJSON(number) = %s, want canonical exponent", string(canonical))
	}

	if err := (&aghsdk.RPCError{Message: "direct"}).Error(); err != "direct" {
		t.Fatalf("RPCError.Error() = %q, want direct", err)
	}
	var nilRPC *aghsdk.RPCError
	if nilRPC.Error() != "" {
		t.Fatalf("nil RPCError.Error() = %q, want empty", nilRPC.Error())
	}
	_ = aghsdk.NewInvalidRequestError("bad")
	_ = aghsdk.NewMethodNotFoundError("missing")
	_ = aghsdk.NewInternalError("internal")
	_ = aghsdk.NewCapabilityDeniedError(map[string]any{"field": "provides"})
}

func TestExtensionConvenienceAndFailureBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should Report Implemented Methods And Descriptors", func(t *testing.T) {
		t.Parallel()

		extension := newTestExtension()
		if got := extension.GetImplementedMethods(); !contains(got, "health_check") || !contains(got, "shutdown") {
			t.Fatalf("GetImplementedMethods() = %#v, want built-ins", got)
		}
		if err := extension.Tool("search", validToolOptions(), rawOKHandler); err != nil {
			t.Fatalf("Tool() error = %v", err)
		}
		if got := extension.GetImplementedMethods(); !contains(got, "provide_tools") || !contains(got, "tools/call") {
			t.Fatalf("GetImplementedMethods() = %#v, want tool methods", got)
		}
		descriptors := extension.GetToolDescriptors()
		if len(descriptors) != 1 || descriptors[0].Handler != "search" {
			t.Fatalf("GetToolDescriptors() = %#v, want search descriptor", descriptors)
		}
	})

	t.Run("Should Validate Run Definition Before Transport", func(t *testing.T) {
		t.Parallel()

		extension := aghsdk.NewExtension(aghsdk.ExtensionDefinition{}, aghsdk.WithTransport(&recordingTransport{}))
		if err := extension.Run(context.Background()); err == nil {
			t.Fatal("Run() error = nil, want definition validation error")
		}
	})

	t.Run("Should Reject Nil Generic Tool Inputs", func(t *testing.T) {
		t.Parallel()

		if err := aghsdk.Tool[map[string]any](nil, "search", validToolOptions(), nil); err == nil {
			t.Fatal("Tool(nil extension) error = nil, want error")
		}
		extension := newTestExtension()
		if err := aghsdk.Tool[map[string]any](extension, "search", validToolOptions(), nil); err == nil {
			t.Fatal("Tool(nil function) error = nil, want error")
		}
	})

	t.Run("Should Reject Invalid Initialize Grants", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{
				Name:    "Grant Extension",
				Version: "0.1.0",
				Actions: aghsdk.ActionsConfig{
					Requires: []aghsdk.HostAPIMethod{aghsdk.HostAPIMethodSessionsList},
				},
			},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
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

		response := runtime.call(t, 1, "initialize", initializeParams("Grant Extension"))
		if response.Error == nil || response.Error.Code != -32001 {
			t.Fatalf("initialize error = %#v, want capability denied", response.Error)
		}
	})
}

func TestTransportAndReadyCallbackBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should Run OnReady Host Call After Initialize", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{
				Name:    "Ready Extension",
				Version: "0.1.0",
				Actions: aghsdk.ActionsConfig{
					Requires: []aghsdk.HostAPIMethod{aghsdk.HostAPIMethodSessionsList},
				},
			},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
		extension.OnReady(func(ctx context.Context, host *aghsdk.HostAPI, _ aghsdk.ExtensionSession) error {
			_, err := host.RawRequest(ctx, aghsdk.HostAPIMethodSessionsList, map[string]any{"limit": 1})
			return err
		})
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

		writeRequest(t, runtime.daemonWriter, 1, "initialize", initializeParamsWithGrants(
			"Ready Extension",
			nil,
			[]string{"sessions/list"},
			nil,
		))
		reader := runtime.daemonReader
		seenInitialize := false
		seenHostCall := false
		for !seenInitialize || !seenHostCall {
			message := readMessage(t, reader)
			if method, _ := message["method"].(string); method == "sessions/list" {
				seenHostCall = true
				writeResponse(t, runtime.daemonWriter, message["id"], []map[string]string{{"id": "sess-1"}})
				continue
			}
			if id, ok := message["id"].(float64); ok && int(id) == 1 {
				if _, ok := message["result"]; !ok {
					t.Fatalf("initialize message = %#v, want result", message)
				}
				seenInitialize = true
				continue
			}
			t.Fatalf("unexpected message: %#v", message)
		}
	})

	t.Run("Should Close Transport And Reject Calls", func(t *testing.T) {
		t.Parallel()

		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  strings.NewReader(""),
			Output: &bytes.Buffer{},
		})
		if err := transport.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := transport.Call(context.Background(), "echo", nil, &json.RawMessage{}); err == nil {
			t.Fatal("Call() error = nil, want closed transport error")
		}
	})

	t.Run("Should Fail On Invalid JSON", func(t *testing.T) {
		t.Parallel()

		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  strings.NewReader("{bad}\n"),
			Output: &bytes.Buffer{},
		})
		err := transport.Run(context.Background())
		if err == nil {
			t.Fatal("Run() error = nil, want parse error")
		}
	})
}

func TestRuntimeErrorBranches(t *testing.T) {
	t.Parallel()

	runtime := newRuntimeHarness(t)
	extension := aghsdk.NewExtension(
		aghsdk.ExtensionDefinition{Name: "Error Extension", Version: "0.1.0"},
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
		func(context.Context, aghsdk.ToolRequest[searchInput]) (aghsdk.ToolResult, error) {
			return aghsdk.TextResult("ok"), nil
		},
	); err != nil {
		t.Fatalf("Tool() error = %v", err)
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

	beforeInit := runtime.call(t, 1, "tools/call", map[string]any{})
	if beforeInit.Error == nil || beforeInit.Error.Code != -32003 {
		t.Fatalf("before initialize error = %#v, want not initialized", beforeInit.Error)
	}
	if response := runtime.call(t, 2, "initialize", initializeParams("Error Extension")); response.Error != nil {
		t.Fatalf("initialize error = %#v", response.Error)
	}
	unknown := runtime.call(t, 3, "unknown/method", map[string]any{})
	if unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("unknown method error = %#v, want method not found", unknown.Error)
	}
	badShutdown := runtime.call(t, 4, "shutdown", map[string]any{"reason": "bad", "deadline_ms": 0})
	if badShutdown.Error == nil || badShutdown.Error.Code != -32602 {
		t.Fatalf("bad shutdown error = %#v, want invalid params", badShutdown.Error)
	}
	missingHandler := runtime.call(t, 5, "tools/call", map[string]any{
		"tool_id": "ext__error_extension__search",
		"handler": "missing",
		"input":   map[string]any{"query": "alpha"},
	})
	if missingHandler.Error == nil || missingHandler.Error.Code != -32601 {
		t.Fatalf("missing handler error = %#v, want method not found", missingHandler.Error)
	}
	mismatchedToolID := runtime.call(t, 6, "tools/call", map[string]any{
		"tool_id": "ext__error_extension__other",
		"handler": "search",
		"input":   map[string]any{"query": "alpha"},
	})
	if mismatchedToolID.Error == nil || mismatchedToolID.Error.Code != -32602 {
		t.Fatalf("mismatched tool id error = %#v, want invalid params", mismatchedToolID.Error)
	}
	invalidInput := runtime.call(t, 7, "tools/call", map[string]any{
		"tool_id": "ext__error_extension__search",
		"handler": "search",
		"input":   map[string]any{"query": 42},
	})
	if invalidInput.Error == nil || invalidInput.Error.Code != -32602 {
		t.Fatalf("invalid input error = %#v, want invalid params", invalidInput.Error)
	}
}

func TestTransportValidationBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should Reject Invalid Call Inputs", func(t *testing.T) {
		t.Parallel()

		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  strings.NewReader(""),
			Output: &bytes.Buffer{},
		})
		var nilContext context.Context
		if err := transport.Call(nilContext, "echo", nil, nil); err == nil {
			t.Fatal("Call(nil context) error = nil, want error")
		}
		if err := transport.Call(context.Background(), " ", nil, nil); err == nil {
			t.Fatal("Call(blank method) error = nil, want error")
		}
	})

	t.Run("Should Surface Write Failures", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  inputReader,
			Output: failingWriter{},
		})
		t.Cleanup(func() {
			if err := inputWriter.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("inputWriter.Close() error = %v", err)
			}
			if err := inputReader.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("inputReader.Close() error = %v", err)
			}
		})
		if err := transport.Call(context.Background(), "echo", map[string]any{}, &json.RawMessage{}); err == nil {
			t.Fatal("Call() error = nil, want write failure")
		}
	})

	t.Run("Should Fail On Invalid Envelope", func(t *testing.T) {
		t.Parallel()

		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  strings.NewReader("[]\n"),
			Output: &bytes.Buffer{},
		})
		err := transport.Run(context.Background())
		if err == nil {
			t.Fatal("Run() error = nil, want invalid request")
		}
	})

	t.Run("Should Convert Handler Marshal Failures To Error Responses", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  inputReader,
			Output: outputWriter,
		})
		transport.Handle(
			"bad/result",
			func(context.Context, json.RawMessage, aghsdk.JSONRPCRequestEnvelope) (any, error) {
				return map[string]any{"bad": make(chan struct{})}, nil
			},
		)
		transport.Handle(
			"nil/result",
			func(context.Context, json.RawMessage, aghsdk.JSONRPCRequestEnvelope) (any, error) {
				return nil, nil
			},
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- transport.Run(ctx) }()
		t.Cleanup(func() {
			cancel()
			closePipes(t, inputReader, inputWriter, outputReader, outputWriter)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("transport did not stop")
			}
		})

		writeRequest(t, inputWriter, 1, "bad/result", map[string]any{})
		message := readMessage(t, bufio.NewReader(outputReader))
		if _, ok := message["error"]; !ok {
			t.Fatalf("message = %#v, want error response", message)
		}
		writeRequest(t, inputWriter, 2, "nil/result", map[string]any{})
		nilMessage := readMessage(t, bufio.NewReader(outputReader))
		if got := nilMessage["result"]; got != nil {
			t.Fatalf("nil result message = %#v, want null result", nilMessage)
		}
	})
}

func TestInitializeValidationBranches(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "Should Reject Missing Protocol",
			mutate: func(params map[string]any) {
				delete(params, "protocol_version")
			},
		},
		{
			name: "Should Reject Empty Supported Versions",
			mutate: func(params map[string]any) {
				params["supported_protocol_versions"] = []string{}
			},
		},
		{
			name: "Should Reject Missing Extension Identity",
			mutate: func(params map[string]any) {
				params["extension"] = map[string]any{"name": "", "version": ""}
			},
		},
		{
			name: "Should Reject Invalid Runtime",
			mutate: func(params map[string]any) {
				params["runtime"].(map[string]any)["health_check_timeout_ms"] = 0
			},
		},
		{
			name: "Should Reject Unsupported Protocol",
			mutate: func(params map[string]any) {
				params["protocol_version"] = "2"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runtime := newRuntimeHarness(t)
			extension := aghsdk.NewExtension(
				aghsdk.ExtensionDefinition{Name: "Init Extension", Version: "0.1.0"},
				aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
				aghsdk.WithStderr(io.Discard),
			)
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

			params := initializeParams("Init Extension")
			tc.mutate(params)
			response := runtime.call(t, 1, "initialize", params)
			if response.Error == nil || response.Error.Code != -32602 {
				t.Fatalf("initialize response error = %#v, want invalid params", response.Error)
			}
		})
	}
}

func initializeParamsWithGrants(
	name string,
	provides []string,
	actions []string,
	security []string,
) map[string]any {
	params := initializeParams(name)
	capabilities := params["capabilities"].(map[string]any)
	capabilities["provides"] = provides
	capabilities["granted_actions"] = actions
	capabilities["granted_security"] = security
	extensionServices := []string{"memory/store", "memory/recall", "memory/forget"}
	if contains(provides, "tool.provider") {
		extensionServices = append(extensionServices, "provide_tools", "tools/call")
	}
	params["methods"].(map[string]any)["extension_services"] = extensionServices
	return params
}

func closePipes(t *testing.T, pipes ...interface{ Close() error }) {
	t.Helper()

	for _, pipe := range pipes {
		if err := pipe.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("pipe.Close() error = %v", err)
		}
	}
}

func waitForTransportStops(t *testing.T, done <-chan error, count int) {
	t.Helper()

	for range count {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("transport did not stop")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced write failure")
}

func writeRequest(t *testing.T, writer io.Writer, id int, method string, params any) {
	t.Helper()

	frame := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("json.Marshal(request) error = %v", err)
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		t.Fatalf("write request error = %v", err)
	}
}

func writeResponse(t *testing.T, writer io.Writer, id any, result any) {
	t.Helper()

	frame := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("json.Marshal(response) error = %v", err)
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		t.Fatalf("write response error = %v", err)
	}
}

func readMessage(t *testing.T, reader *bufio.Reader) map[string]any {
	t.Helper()

	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read message error = %v", err)
	}
	var message map[string]any
	if err := json.Unmarshal(line, &message); err != nil {
		t.Fatalf("json.Unmarshal(message %s) error = %v", string(line), err)
	}
	return message
}
