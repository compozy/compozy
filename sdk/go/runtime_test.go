package compozysdk_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

func TestStdioTransportBidirectionalCalls(t *testing.T) {
	t.Parallel()

	clientInput, serverOutput := io.Pipe()
	serverInput, clientOutput := io.Pipe()
	client := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
		Input:  clientInput,
		Output: clientOutput,
	})
	server := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
		Input:  serverInput,
		Output: serverOutput,
	})
	server.Handle(
		"echo",
		func(_ context.Context, params json.RawMessage, _ compozysdk.JSONRPCRequestEnvelope) (any, error) {
			var payload map[string]string
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			return map[string]string{"echo": payload["value"]}, nil
		},
	)
	server.Handle("fail", func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
		return nil, compozysdk.NewInvalidParamsError("forced failure", nil)
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
	rpcErr, rpcErrMatched := errors.AsType[*compozysdk.RPCError](err)
	if !rpcErrMatched || rpcErr.Code != -32602 {
		t.Fatalf("client.Call(fail) error = %v, want invalid params RPC error", err)
	}
}

func TestExtensionRuntimeBuiltInAndCustomMethods(t *testing.T) {
	t.Parallel()
	t.Run("Should serve built-in and custom extension methods", testExtensionRuntimeBuiltInAndCustomMethods)
}

func testExtensionRuntimeBuiltInAndCustomMethods(t *testing.T) {
	t.Parallel()

	runtime := newRuntimeHarness(t)
	extension := compozysdk.NewExtension(
		compozysdk.ExtensionDefinition{
			Name:    "Memory Extension",
			Version: "0.1.0",
			Capabilities: compozysdk.CapabilitiesConfig{
				Provides: []string{"memory.backend"},
			},
			Permissions: compozysdk.PermissionsConfig{
				Requires: []compozysdk.HostAPIMethod{compozysdk.HostAPIMethodSessionsList},
			},
			SupportedHookEvents: []string{"session.started"},
		},
		compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
		compozysdk.WithSDKVersion("test-version"),
		compozysdk.WithStderr(io.Discard),
	)
	if err := extension.Handle("memory/store", func(
		context.Context,
		compozysdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]bool{"stored": true}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}
	if err := extension.Handle("memory/recall", func(
		context.Context,
		compozysdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]any{"entries": []any{}}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/recall) error = %v", err)
	}
	if err := extension.Handle("memory/forget", func(
		context.Context,
		compozysdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return map[string]bool{"forgotten": true}, nil
	}); err != nil {
		t.Fatalf("Handle(memory/forget) error = %v", err)
	}
	if err := extension.Handle("health_check", func(
		context.Context,
		compozysdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return compozysdk.HealthCheckResult{Healthy: true, Message: "ok"}, nil
	}); err != nil {
		t.Fatalf("Handle(health_check) error = %v", err)
	}
	if err := extension.Handle("shutdown", func(
		context.Context,
		compozysdk.ExtensionContext,
		json.RawMessage,
	) (any, error) {
		return compozysdk.ShutdownResponse{Acknowledged: true}, nil
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
	))
	if initialize.Error != nil {
		t.Fatalf("initialize error = %#v", initialize.Error)
	}
	var initResult compozysdk.InitializeResponse
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
	var healthResult compozysdk.HealthCheckResult
	decodeResult(t, health.Result, &healthResult)
	if !healthResult.Healthy || healthResult.Message != "ok" {
		t.Fatalf("health result = %#v, want healthy ok", healthResult)
	}

	shutdown := runtime.call(t, 4, "shutdown", map[string]any{"reason": "test", "deadline_ms": 100})
	if shutdown.Error != nil {
		t.Fatalf("shutdown error = %#v", shutdown.Error)
	}
	var shutdownResult compozysdk.ShutdownResponse
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
	t.Run("Should complete host API raw-request and result helper contracts", testHostAPIRawRequestAndResultHelpers)
}

func testHostAPIRawRequestAndResultHelpers(t *testing.T) {
	t.Parallel()

	transport := &recordingTransport{rawResult: json.RawMessage(`{"ok":true}`)}
	host := compozysdk.NewHostAPI(transport, func() bool { return true })
	raw, err := host.RawRequest(
		context.Background(),
		compozysdk.HostAPIMethodObserveHealth,
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

	empty := compozysdk.EmptyResult()
	if empty.Truncated || empty.Bytes != 0 {
		t.Fatalf("EmptyResult() = %#v, want zero non-truncated result", empty)
	}
	structured, err := compozysdk.StructuredResult(map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("StructuredResult() error = %v", err)
	}
	if string(structured.Structured) != `{"ok":true}` {
		t.Fatalf("StructuredResult() = %s, want JSON payload", string(structured.Structured))
	}
	if _, err := compozysdk.StructuredResult(map[string]any{"bad": make(chan struct{})}); err == nil {
		t.Fatal("StructuredResult() error = nil, want marshal error")
	}
}

func TestValidationAndDigestErrorBranches(t *testing.T) {
	t.Parallel()

	invalidIDs := []compozysdk.ToolID{"", "A", "a_", "a___b", "a__"}
	for _, id := range invalidIDs {
		t.Run("Should Reject ToolID "+string(id), func(t *testing.T) {
			t.Parallel()

			if err := id.Validate(); err == nil {
				t.Fatalf("ToolID(%q).Validate() error = nil, want error", id)
			}
		})
	}

	if _, err := compozysdk.CanonicalJSON(json.RawMessage(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("CanonicalJSON() error = nil, want multiple value error")
	}
	if _, err := compozysdk.CanonicalJSON(json.RawMessage(`{"a":NaN}`)); err == nil {
		t.Fatal("CanonicalJSON() error = nil, want invalid JSON error")
	}
	if _, err := compozysdk.SchemaDigest(json.RawMessage(`[]`)); err == nil {
		t.Fatal("SchemaDigest([]) error = nil, want object error")
	}
	if canonical, err := compozysdk.CanonicalJSON(json.RawMessage(`{"n":1.20e+3}`)); err != nil {
		t.Fatalf("CanonicalJSON(number) error = %v", err)
	} else if string(canonical) != `{"n":1200}` {
		t.Fatalf("CanonicalJSON(number) = %s, want canonical exponent", string(canonical))
	}

	if err := (&compozysdk.RPCError{Message: "direct"}).Error(); err != "direct" {
		t.Fatalf("RPCError.Error() = %q, want direct", err)
	}
	var nilRPC *compozysdk.RPCError
	if nilRPC.Error() != "" {
		t.Fatalf("nil RPCError.Error() = %q, want empty", nilRPC.Error())
	}
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

		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{},
			compozysdk.WithTransport(&recordingTransport{}),
		)
		if err := extension.Run(context.Background()); err == nil {
			t.Fatal("Run() error = nil, want definition validation error")
		}
	})

	t.Run("Should Reject Nil Generic Tool Inputs", func(t *testing.T) {
		t.Parallel()

		if err := compozysdk.Tool[map[string]any](nil, "search", validToolOptions(), nil); err == nil {
			t.Fatal("Tool(nil extension) error = nil, want error")
		}
		extension := newTestExtension()
		if err := compozysdk.Tool[map[string]any](extension, "search", validToolOptions(), nil); err == nil {
			t.Fatal("Tool(nil function) error = nil, want error")
		}
	})

	t.Run("Should Reject Invalid Initialize Grants", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{
				Name:    "Grant Extension",
				Version: "0.1.0",
				Permissions: compozysdk.PermissionsConfig{
					Requires: []compozysdk.HostAPIMethod{compozysdk.HostAPIMethodSessionsList},
				},
			},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
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
		var denied struct {
			Field    string   `json:"field"`
			Required []string `json:"required"`
			Granted  []string `json:"granted"`
		}
		decodeResult(t, response.Error.Data, &denied)
		if denied.Field != "permissions" || !contains(denied.Required, "sessions/list") {
			t.Fatalf("capability denied data = %#v, want missing sessions/list grant", denied)
		}
	})
}

func TestTransportAndReadyCallbackBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should Run OnReady Host Call After Initialize", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{
				Name:    "Ready Extension",
				Version: "0.1.0",
				Permissions: compozysdk.PermissionsConfig{
					Requires: []compozysdk.HostAPIMethod{compozysdk.HostAPIMethodSessionsList},
				},
			},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		extension.OnReady(func(ctx context.Context, host *compozysdk.HostAPI, _ compozysdk.ExtensionSession) error {
			_, err := host.RawRequest(ctx, compozysdk.HostAPIMethodSessionsList, map[string]any{"limit": 1})
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

	t.Run("Should Cancel And Join OnReady Callbacks When Runtime Closes", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Lifecycle Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		started := make(chan struct{})
		stopped := make(chan struct{})
		lateStarted := make(chan struct{})
		lateStopped := make(chan struct{})
		release := make(chan struct{})
		readyCallback := func(started chan<- struct{}, stopped chan<- struct{}) func(
			context.Context,
			*compozysdk.HostAPI,
			compozysdk.ExtensionSession,
		) error {
			return func(ctx context.Context, _ *compozysdk.HostAPI, _ compozysdk.ExtensionSession) error {
				close(started)
				select {
				case <-ctx.Done():
					close(stopped)
					return ctx.Err()
				case <-release:
					return nil
				}
			}
		}
		extension.OnReady(readyCallback(started, stopped))

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			close(release)
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Lifecycle Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not start")
		}
		extension.OnReady(readyCallback(lateStarted, lateStopped))
		select {
		case <-lateStarted:
		case <-time.After(time.Second):
			t.Fatal("late onReady callback did not start")
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if err != nil {
				t.Fatalf("extension Run() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
		select {
		case <-stopped:
		default:
			t.Fatal("onReady callback remained active after runtime closed")
		}
		select {
		case <-lateStopped:
		default:
			t.Fatal("late onReady callback remained active after runtime closed")
		}
	})

	t.Run("Should Preserve NonCancellation OnReady Error Joined With Cancellation", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Joined Error Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		sentinelErr := errors.New("ready callback sentinel failure")
		started := make(chan struct{})
		extension.OnReady(func(ctx context.Context, _ *compozysdk.HostAPI, _ compozysdk.ExtensionSession) error {
			close(started)
			<-ctx.Done()
			return errors.Join(ctx.Err(), sentinelErr)
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Joined Error Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not start")
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if !errors.Is(err, sentinelErr) {
				t.Fatalf("extension Run() error = %v, want sentinel callback failure", err)
			}
			if errors.Is(err, context.Canceled) {
				t.Fatalf("extension Run() error = %v, want cancellation branch suppressed", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Surface Shutdown Timeout For Uncooperative OnReady Callback", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Timeout Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		lateCallbackErr := errors.New("ready callback failed after the drain deadline")
		started := make(chan struct{})
		stopped := make(chan struct{})
		release := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			close(started)
			<-release
			close(stopped)
			return lateCallbackErr
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		callbackStarted := false
		inputClosed := false
		released := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !released {
				close(release)
				released = true
				if callbackStarted {
					select {
					case <-stopped:
					case <-time.After(time.Second):
						t.Error("uncooperative onReady callback did not stop during cleanup")
					}
				}
			}
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		params := initializeParams("Ready Timeout Extension")
		params["runtime"].(map[string]any)["shutdown_timeout_ms"] = 10
		initialize := runtime.call(t, 1, "initialize", params)
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not start")
		}
		callbackStarted = true

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("extension Run() error = %v, want shutdown deadline exceeded", err)
			}
			if errors.Is(err, lateCallbackErr) {
				t.Fatalf(
					"extension Run() error = %v, must not report a callback failure "+
						"that completed after Run returned",
					err,
				)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not observe shutdown timeout")
		}

		close(release)
		released = true
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("uncooperative onReady callback did not stop after release")
		}
	})

	t.Run("Should Use Shutdown Request Deadline Instead Of Initialized Runtime Timeout", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Request Deadline Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		started := make(chan struct{})
		stopped := make(chan struct{})
		release := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			close(started)
			<-release
			close(stopped)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		callbackStarted := false
		inputClosed := false
		released := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !released {
				close(release)
				released = true
				if callbackStarted {
					select {
					case <-stopped:
					case <-time.After(time.Second):
						t.Error("uncooperative onReady callback did not stop during cleanup")
					}
				}
			}
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		params := initializeParams("Ready Request Deadline Extension")
		params["runtime"].(map[string]any)["shutdown_timeout_ms"] = 30_000
		initialize := runtime.call(t, 1, "initialize", params)
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not start")
		}
		callbackStarted = true

		shutdown := runtime.call(t, 2, "shutdown", map[string]any{"reason": "test", "deadline_ms": 10})
		if shutdown.Error != nil {
			t.Fatalf("shutdown error = %#v", shutdown.Error)
		}
		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("extension Run() error = %v, want shutdown request deadline exceeded", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime waited for initialized runtime timeout instead of shutdown request deadline")
		}

		close(release)
		released = true
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("uncooperative onReady callback did not stop after release")
		}
	})

	t.Run("Should Reject Shutdown Deadline That Exceeds Go Duration", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Deadline Validation Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Deadline Validation Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		shutdown := runtime.call(
			t,
			2,
			"shutdown",
			map[string]any{"reason": "test", "deadline_ms": int64(math.MaxInt64)},
		)
		if shutdown.Error == nil || shutdown.Error.Code != -32602 {
			t.Fatalf("shutdown error = %#v, want invalid params", shutdown.Error)
		}
		health := runtime.call(t, 3, "health_check", map[string]any{})
		if health.Error != nil {
			t.Fatalf("health_check after rejected shutdown error = %#v, want ready runtime", health.Error)
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if err != nil {
				t.Fatalf("extension Run() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Return OnReady Callback Failures From Runtime", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Failure Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		callbackErr := errors.New("ready callback failed")
		completed := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			defer close(completed)
			return callbackErr
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Failure Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-completed:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not complete")
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if !errors.Is(err, callbackErr) {
				t.Fatalf("extension Run() error = %v, want callback error", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Return Recovered OnReady Panic From Runtime", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Panic Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		completed := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			defer close(completed)
			panic("ready callback panic")
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Panic Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-completed:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not complete")
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if err == nil || !strings.Contains(err.Error(), "onReady callback panic: ready callback panic") {
				t.Fatalf("extension Run() error = %v, want recovered callback panic", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Return OnReady Goexit From Runtime", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Goexit Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		completed := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			defer close(completed)
			goruntime.Goexit()
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Goexit Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		select {
		case <-completed:
		case <-time.After(time.Second):
			t.Fatal("onReady callback did not complete")
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if err == nil || !strings.Contains(err.Error(), "onReady callback exited without returning") {
				t.Fatalf("extension Run() error = %v, want Goexit callback failure", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Ignore OnReady Registration After Shutdown", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := compozysdk.NewExtension(
			compozysdk.ExtensionDefinition{Name: "Ready Shutdown Extension", Version: "0.1.0"},
			compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			compozysdk.WithStderr(io.Discard),
		)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- extension.Run(ctx) }()
		inputClosed := false
		runReturned := false
		t.Cleanup(func() {
			cancel()
			if !inputClosed {
				if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("close input error = %v", err)
				}
			}
			if !runReturned {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("extension runtime did not stop")
				}
			}
		})

		initialize := runtime.call(t, 1, "initialize", initializeParams("Ready Shutdown Extension"))
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		shutdown := runtime.call(t, 2, "shutdown", map[string]any{"reason": "test", "deadline_ms": 100})
		if shutdown.Error != nil {
			t.Fatalf("shutdown error = %#v", shutdown.Error)
		}
		callbackRan := make(chan struct{})
		extension.OnReady(func(context.Context, *compozysdk.HostAPI, compozysdk.ExtensionSession) error {
			close(callbackRan)
			return nil
		})
		select {
		case <-callbackRan:
			t.Fatal("onReady callback ran after shutdown")
		case <-time.After(time.Second):
		}

		if err := runtime.closeInput(); err != nil {
			t.Fatalf("close input error = %v", err)
		}
		inputClosed = true
		select {
		case err := <-done:
			runReturned = true
			if err != nil {
				t.Fatalf("extension Run() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("extension runtime did not stop")
		}
	})

	t.Run("Should Close Transport And Reject Calls", func(t *testing.T) {
		t.Parallel()

		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
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

		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
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
	t.Run("Should reject invalid runtime requests", testRuntimeErrorBranches)
}

func testRuntimeErrorBranches(t *testing.T) {
	t.Parallel()

	runtime := newRuntimeHarness(t)
	extension := compozysdk.NewExtension(
		compozysdk.ExtensionDefinition{Name: "Error Extension", Version: "0.1.0"},
		compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
		compozysdk.WithStderr(io.Discard),
	)
	type searchInput struct {
		Query string `json:"query"`
	}
	if err := compozysdk.Tool[searchInput](
		extension,
		"search",
		validToolOptions(),
		func(context.Context, compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
			return compozysdk.TextResult("ok"), nil
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

		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
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
		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
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

		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
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
		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
			Input:  inputReader,
			Output: outputWriter,
		})
		transport.Handle(
			"bad/result",
			func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
				return map[string]any{"bad": make(chan struct{})}, nil
			},
		)
		transport.Handle(
			"nil/result",
			func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
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

	t.Run("Should Redact Claim Tokens From Error Responses", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
			Input:  inputReader,
			Output: outputWriter,
		})
		transport.Handle(
			"error/common",
			func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
				return nil, errors.New("handler failed with compozy_claim_common-secret")
			},
		)
		transport.Handle(
			"error/rpc",
			func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
				return nil, &compozysdk.RPCError{
					Code:    -32010,
					Message: "direct compozy_claim_rpc-message",
					Data: json.RawMessage(
						`{"safe":"value","safe_number":9007199254740993,"nested":["compozy_claim_rpc-data"],"claim_token":"compozy_claim_rpc-token","compozy_claim_rpc-key":"discard"}`,
					),
				}
			},
		)
		transport.Handle(
			"error/marshal",
			func(context.Context, json.RawMessage, compozysdk.JSONRPCRequestEnvelope) (any, error) {
				return claimTokenMarshalFailure{}, nil
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

		reader := bufio.NewReader(outputReader)
		readError := func(t *testing.T) *compozysdk.JSONRPCErrorObject {
			t.Helper()

			line, err := reader.ReadBytes('\n')
			if err != nil {
				t.Fatalf("read error response: %v", err)
			}
			if bytes.Contains(line, []byte("compozy_claim_")) {
				t.Fatalf("error response leaked claim token material: %s", line)
			}
			var response struct {
				Error *compozysdk.JSONRPCErrorObject `json:"error"`
			}
			if err := json.Unmarshal(line, &response); err != nil {
				t.Fatalf("json.Unmarshal(error response %s) error = %v", string(line), err)
			}
			if response.Error == nil {
				t.Fatalf("error response = %s, want error object", line)
			}
			return response.Error
		}

		writeRequest(t, inputWriter, 1, "error/common", map[string]any{})
		common := readError(t)
		if common.Code != -32603 || common.Message != "Internal error" {
			t.Fatalf("common error = %#v, want internal error", common)
		}

		writeRequest(t, inputWriter, 2, "error/rpc", map[string]any{})
		rpc := readError(t)
		if rpc.Code != -32010 || rpc.Message != "direct [REDACTED]" {
			t.Fatalf("RPC error = %#v, want stable code and redacted message", rpc)
		}
		if !bytes.Contains(rpc.Data, []byte(`"safe":"value"`)) ||
			!bytes.Contains(rpc.Data, []byte(`"safe_number":9007199254740993`)) {
			t.Fatalf("RPC error data = %s, want safe data preserved", rpc.Data)
		}

		writeRequest(t, inputWriter, 3, "error/marshal", map[string]any{})
		marshalFailure := readError(t)
		if marshalFailure.Code != -32603 || marshalFailure.Message != "Internal error" {
			t.Fatalf("marshal failure error = %#v, want internal error", marshalFailure)
		}
	})

	t.Run("Should Redact Claim Tokens From Received Error Responses", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		transport := compozysdk.NewStdioTransport(compozysdk.StdioTransportOptions{
			Input:  inputReader,
			Output: outputWriter,
		})
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

		callDone := make(chan error, 1)
		go func() {
			callDone <- transport.Call(ctx, "remote/error", map[string]any{}, nil)
		}()
		request := readMessage(t, bufio.NewReader(outputReader))
		response := map[string]any{
			"jsonrpc": "2.0",
			"id":      request["id"],
			"error": map[string]any{
				"code":    -32010,
				"message": "remote compozy_claim_inbound-message",
				"data": map[string]any{
					"safe":        "preserved",
					"nested":      []string{"compozy_claim_inbound-data"},
					"claim_token": "compozy_claim_inbound-token",
				},
			},
		}
		if err := json.NewEncoder(inputWriter).Encode(response); err != nil {
			t.Fatalf("encode received error response: %v", err)
		}

		var callErr error
		select {
		case callErr = <-callDone:
		case <-time.After(time.Second):
			t.Fatal("Call() did not receive error response")
		}

		rpcErr, rpcErrMatched := errors.AsType[*compozysdk.RPCError](callErr)
		if !rpcErrMatched {
			t.Fatalf("Call() error = %v, want *RPCError", callErr)
		}
		if rpcErr.Code != -32010 || rpcErr.Message != "remote [REDACTED]" {
			t.Fatalf("received RPC error = %#v, want stable code and redacted message", rpcErr)
		}
		if bytes.Contains(rpcErr.Data, []byte("compozy_claim_")) ||
			bytes.Contains(rpcErr.Data, []byte(`"claim_token"`)) {
			t.Fatalf("received RPC error data leaked claim token material: %s", rpcErr.Data)
		}
		if !bytes.Contains(rpcErr.Data, []byte(`"safe":"preserved"`)) {
			t.Fatalf("received RPC error data = %s, want safe data preserved", rpcErr.Data)
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
			name: "Should Reject Shutdown Timeout That Exceeds Go Duration",
			mutate: func(params map[string]any) {
				params["runtime"].(map[string]any)["shutdown_timeout_ms"] = int64(math.MaxInt64)
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
			extension := compozysdk.NewExtension(
				compozysdk.ExtensionDefinition{Name: "Init Extension", Version: "0.1.0"},
				compozysdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
				compozysdk.WithStderr(io.Discard),
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
	permissions []string,
) map[string]any {
	params := initializeParams(name)
	capabilities := params["capabilities"].(map[string]any)
	capabilities["provides"] = provides
	capabilities["granted_permissions"] = permissions
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

type claimTokenMarshalFailure struct{}

func (claimTokenMarshalFailure) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failed with compozy_claim_marshal-secret")
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
