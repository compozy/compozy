package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type writeCloserBuffer struct {
	bytes.Buffer
	closed bool
}

func (b *writeCloserBuffer) Close() error {
	b.closed = true
	return nil
}

type channelTransport struct {
	incoming   <-chan Message
	outgoing   chan<- Message
	localDone  chan struct{}
	remoteDone <-chan struct{}
	closeOnce  sync.Once
}

func newChannelTransportPair() (*channelTransport, *channelTransport) {
	leftToRight := make(chan Message, 16)
	rightToLeft := make(chan Message, 16)
	leftDone := make(chan struct{})
	rightDone := make(chan struct{})

	left := &channelTransport{
		incoming:   rightToLeft,
		outgoing:   leftToRight,
		localDone:  leftDone,
		remoteDone: rightDone,
	}
	right := &channelTransport{
		incoming:   leftToRight,
		outgoing:   rightToLeft,
		localDone:  rightDone,
		remoteDone: leftDone,
	}
	return left, right
}

func (t *channelTransport) ReadMessage() (Message, error) {
	select {
	case <-t.localDone:
		return Message{}, io.EOF
	case message := <-t.incoming:
		return message, nil
	}
}

func (t *channelTransport) WriteMessage(message Message) error {
	select {
	case <-t.localDone:
		return io.EOF
	case <-t.remoteDone:
		return io.EOF
	case t.outgoing <- message:
		return nil
	}
}

func (t *channelTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.localDone)
	})
	return nil
}

func TestStdIOTransportAndHelperErrors(t *testing.T) {
	t.Parallel()

	t.Run("round trip and close", func(t *testing.T) {
		t.Parallel()

		writer := &writeCloserBuffer{}
		transport := NewStdIOTransport(io.NopCloser(strings.NewReader("\n{\"id\":1,\"method\":\"ping\"}\n")), writer)

		message, err := transport.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		if message.Method != "ping" {
			t.Fatalf("message.Method = %q, want ping", message.Method)
		}

		if err := transport.WriteMessage(
			Message{ID: MustJSON("1"), Result: MustJSON(map[string]string{"status": "ok"})},
		); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
		if !strings.Contains(writer.String(), "\"jsonrpc\":\"2.0\"") {
			t.Fatalf("WriteMessage() output = %q, want jsonrpc envelope", writer.String())
		}
		if !strings.HasSuffix(writer.String(), "\n") {
			t.Fatalf("WriteMessage() output = %q, want trailing newline", writer.String())
		}

		if err := transport.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if !writer.closed {
			t.Fatal("writer was not closed")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		t.Parallel()

		transport := NewStdIOTransport(strings.NewReader("{bad-json}\n"), io.Discard)
		err := expectRPCError(t, transport.ReadMessage)
		if err.Code != -32700 {
			t.Fatalf("parse error code = %d, want -32700", err.Code)
		}
	})

	t.Run("helper constructors", func(t *testing.T) {
		t.Parallel()

		parseErr := newParseError(map[string]any{"error": "bad"})
		if parseErr.Code != -32700 {
			t.Fatalf("newParseError().Code = %d, want -32700", parseErr.Code)
		}

		invalidRequest := newInvalidRequestError(map[string]any{"reason": "bad"})
		if invalidRequest.Code != -32600 {
			t.Fatalf("newInvalidRequestError().Code = %d, want -32600", invalidRequest.Code)
		}

		methodErr := newMethodNotFoundError("ping")
		if !strings.Contains(methodErr.Error(), "Method not found") {
			t.Fatalf("Error() = %q, want method not found message", methodErr.Error())
		}

		internalErr := newInternalError(map[string]any{"error": "boom"})
		if internalErr.Code != -32603 {
			t.Fatalf("newInternalError().Code = %d, want -32603", internalErr.Code)
		}

		var data map[string]any
		if err := methodErr.DecodeData(&data); err != nil {
			t.Fatalf("DecodeData() error = %v", err)
		}
		if data["method"] != "ping" {
			t.Fatalf("DecodeData() method = %#v, want ping", data["method"])
		}

		noData := &Error{Code: -1, Message: "no data"}
		if !errors.Is(noData.DecodeData(&data), io.EOF) {
			t.Fatalf("DecodeData(no data) error = %v, want io.EOF", noData.DecodeData(&data))
		}
	})

	t.Run("context and rpc conversion helpers", func(t *testing.T) {
		t.Parallel()

		var nilCtx context.Context
		if contextError(nilCtx) != nil {
			t.Fatal("contextError(nil) should return nil")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if !errors.Is(contextError(ctx), context.Canceled) {
			t.Fatalf("contextError(canceled) = %v, want context.Canceled", contextError(ctx))
		}

		requestErr := &Error{Code: -32000, Message: "boom"}
		if got := toRPCError(requestErr); got != requestErr {
			t.Fatal("toRPCError(requestErr) should preserve the original pointer")
		}

		converted := toRPCError(errors.New("boom"))
		if converted.Code != -32603 {
			t.Fatalf("toRPCError(generic).Code = %d, want -32603", converted.Code)
		}

		var extracted *Error
		if !errorAs(requestErr, &extracted) || extracted != requestErr {
			t.Fatal("errorAs() did not extract the request error")
		}
		if errorAs(errors.New("boom"), &extracted) {
			t.Fatal("errorAs() should reject non-RPC errors")
		}

		open := make(chan struct{})
		if isClosed(open) {
			t.Fatal("isClosed(open) = true, want false")
		}
		close(open)
		if !isClosed(open) {
			t.Fatal("isClosed(closed) = false, want true")
		}
	})
}

func TestExtensionStartBranchesAndOverrides(t *testing.T) {
	t.Parallel()

	t.Run("requires identity", func(t *testing.T) {
		t.Parallel()

		if err := New("", "1.0.0").Start(context.Background()); err == nil {
			t.Fatal("Start() without name error = nil, want error")
		}
		if err := New("sdk-ext", "").Start(context.Background()); err == nil {
			t.Fatal("Start() without version error = nil, want error")
		}
	})

	t.Run("double start and sdk version override", func(t *testing.T) {
		t.Parallel()

		extSide, hostSide := newChannelTransportPair()
		ext := New("sdk-ext", "1.0.0").WithTransport(extSide).WithSDKVersion("sdk-custom")

		errCh := runExtension(t, ext)
		response := initializeExtension(t, hostSide)
		if response.ExtensionInfo.SDKVersion != "sdk-custom" {
			t.Fatalf("sdk_version = %q, want sdk-custom", response.ExtensionInfo.SDKVersion)
		}

		if err := ext.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "already started") {
			t.Fatalf("second Start() error = %v, want already started", err)
		}

		sendRequestExpectResult(
			t,
			hostSide,
			"2",
			"shutdown",
			ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
			&ShutdownResponse{},
		)
		waitForTerminalError(t, errCh, nil)
	})

	t.Run("health check override", func(t *testing.T) {
		t.Parallel()

		extSide, hostSide := newChannelTransportPair()
		ext := New(
			"sdk-ext",
			"1.0.0",
		).WithTransport(extSide).
			OnHealthCheck(func(_ context.Context, _ HealthCheckRequest) (HealthCheckResponse, error) {
				return HealthCheckResponse{Healthy: false, Message: "degraded"}, nil
			})

		errCh := runExtension(t, ext)
		initializeExtension(t, hostSide)

		var response HealthCheckResponse
		sendRequestExpectResult(t, hostSide, "2", "health_check", HealthCheckRequest{}, &response)
		if response.Healthy {
			t.Fatalf("health response = %#v, want unhealthy override", response)
		}
		if _, ok := response.Details["active_requests"]; !ok {
			t.Fatal("health response missing active_requests detail")
		}

		sendRequestExpectResult(
			t,
			hostSide,
			"3",
			"shutdown",
			ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
			&ShutdownResponse{},
		)
		waitForTerminalError(t, errCh, nil)
	})

	t.Run("shutdown handler error terminates start", func(t *testing.T) {
		t.Parallel()

		extSide, hostSide := newChannelTransportPair()
		ext := New("sdk-ext", "1.0.0").WithTransport(extSide).OnShutdown(func(context.Context, ShutdownRequest) error {
			return errors.New("boom")
		})

		errCh := runExtension(t, ext)
		initializeExtension(t, hostSide)

		err := sendRequestExpectError(
			t,
			hostSide,
			"2",
			"shutdown",
			ShutdownRequest{Reason: "run_failed", DeadlineMS: 1000},
		)
		if err.Code != -32603 {
			t.Fatalf("shutdown error code = %d, want -32603", err.Code)
		}
		waitForTerminalError(t, errCh, func(err error) bool {
			return err != nil && strings.Contains(err.Error(), "boom")
		})
	})
}

func TestExtensionDirectRequestBranches(t *testing.T) {
	t.Parallel()

	t.Run("handle unknown request", func(t *testing.T) {
		t.Parallel()

		left, right := newChannelTransportPair()
		ext := New("sdk-ext", "1.0.0").WithTransport(left)
		ext.initialized = true
		ext.acceptedCapabilities = map[Capability]struct{}{}

		ext.handleRequest(Message{ID: MustJSON("7"), Method: "unknown"})

		response, err := right.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		if response.Error == nil || response.Error.Code != -32601 {
			t.Fatalf("response.Error = %#v, want method not found", response.Error)
		}
	})

	t.Run("handle request while draining", func(t *testing.T) {
		t.Parallel()

		left, right := newChannelTransportPair()
		ext := New("sdk-ext", "1.0.0").WithTransport(left)
		ext.initialized = true
		ext.draining = true

		ext.handleRequest(Message{ID: MustJSON("8"), Method: "health_check"})

		response, err := right.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		if response.Error == nil || response.Error.Code != -32004 {
			t.Fatalf("response.Error = %#v, want shutdown in progress", response.Error)
		}
	})

	t.Run("host call while draining", func(t *testing.T) {
		t.Parallel()

		ext := New("sdk-ext", "1.0.0")
		ext.initialized = true
		ext.draining = true
		ext.acceptedCapabilities = map[Capability]struct{}{CapabilityTasksRead: {}}

		err := ext.call(context.Background(), "host.tasks.list", TaskListRequest{Workflow: "demo"}, &[]Task{})
		requestErr := assertRPCErrorCode(t, err)
		if requestErr.Code != -32004 {
			t.Fatalf("call error code = %d, want -32004", requestErr.Code)
		}
	})
}

func TestHandleFetchReviewsDispatchesRegisteredReviewProvider(t *testing.T) {
	t.Parallel()

	extSide, hostSide := newChannelTransportPair()
	ext := New("sdk-ext", "1.0.0").
		WithTransport(extSide).
		WithCapabilities(CapabilityProvidersRegister).
		RegisterReviewProvider("sdk-review", ReviewProvider{
			FetchReviewsFunc: func(
				_ context.Context,
				ctx ReviewProviderContext,
				req FetchRequest,
			) ([]ReviewItem, error) {
				if ctx.Provider != "sdk-review" {
					t.Fatalf("provider context = %q, want sdk-review", ctx.Provider)
				}
				if req.PR != "123" || !req.IncludeNitpicks {
					t.Fatalf("request = %#v, want propagated fetch request", req)
				}
				return []ReviewItem{{
					Title:       "issue",
					Body:        "from provider",
					ProviderRef: "thread-1",
				}}, nil
			},
		})

	errCh := runExtension(t, ext)
	var response InitializeResponse
	sendRequestExpectResult(t, hostSide, "1", "initialize", InitializeRequest{
		ProtocolVersion:           ProtocolVersion,
		SupportedProtocolVersions: []string{ProtocolVersion},
		CompozyVersion:            "dev",
		Extension: InitializeRequestIdentity{
			Name:    "sdk-ext",
			Version: "1.0.0",
			Source:  "workspace",
		},
		GrantedCapabilities: []Capability{CapabilityProvidersRegister},
		Runtime: InitializeRuntime{
			RunID:                 "run-001",
			WorkspaceRoot:         ".",
			InvokingCommand:       "start",
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  5000,
			HealthCheckIntervalMS: 1000,
		},
	}, &response)
	if got := response.RegisteredReviewProviders; len(got) != 1 || got[0] != "sdk-review" {
		t.Fatalf("registered_review_providers = %#v, want [sdk-review]", got)
	}

	var items []ReviewItem
	sendRequestExpectResult(t, hostSide, "2", "fetch_reviews", map[string]any{
		"provider":         "sdk-review",
		"pr":               "123",
		"include_nitpicks": true,
	}, &items)
	if len(items) != 1 || items[0].Title != "issue" {
		t.Fatalf("fetch_reviews result = %#v, want bridged review item", items)
	}

	sendRequestExpectResult(
		t,
		hostSide,
		"3",
		"shutdown",
		ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
		&ShutdownResponse{},
	)
	waitForTerminalError(t, errCh, nil)
}

func TestHandleResolveIssuesDispatchesRegisteredReviewProvider(t *testing.T) {
	t.Parallel()

	extSide, hostSide := newChannelTransportPair()
	seen := make(chan ResolveIssuesRequest, 1)
	ext := New("sdk-ext", "1.0.0").
		WithTransport(extSide).
		WithCapabilities(CapabilityProvidersRegister).
		RegisterReviewProvider("sdk-review", ReviewProvider{
			ResolveIssuesFunc: func(
				_ context.Context,
				ctx ReviewProviderContext,
				req ResolveIssuesRequest,
			) error {
				if ctx.Provider != "sdk-review" {
					t.Fatalf("provider context = %q, want sdk-review", ctx.Provider)
				}
				seen <- req
				return nil
			},
		})

	errCh := runExtension(t, ext)
	sendRequestExpectResult(t, hostSide, "1", "initialize", InitializeRequest{
		ProtocolVersion:           ProtocolVersion,
		SupportedProtocolVersions: []string{ProtocolVersion},
		CompozyVersion:            "dev",
		Extension: InitializeRequestIdentity{
			Name:    "sdk-ext",
			Version: "1.0.0",
			Source:  "workspace",
		},
		GrantedCapabilities: []Capability{CapabilityProvidersRegister},
		Runtime: InitializeRuntime{
			RunID:                 "run-001",
			WorkspaceRoot:         ".",
			InvokingCommand:       "start",
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  5000,
			HealthCheckIntervalMS: 1000,
		},
	}, &InitializeResponse{})

	sendRequestExpectResult(t, hostSide, "2", "resolve_issues", map[string]any{
		"provider": "sdk-review",
		"pr":       "123",
		"issues": []map[string]any{{
			"file_path":    "issue_001.md",
			"provider_ref": "thread-1",
		}},
	}, &map[string]any{})

	select {
	case req := <-seen:
		if req.PR != "123" || len(req.Issues) != 1 || req.Issues[0].FilePath != "issue_001.md" {
			t.Fatalf("resolve issues request = %#v, want propagated payload", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resolve issues request")
	}

	sendRequestExpectResult(
		t,
		hostSide,
		"3",
		"shutdown",
		ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
		&ShutdownResponse{},
	)
	waitForTerminalError(t, errCh, nil)
}

func TestHandleFetchReviewsRequiresProvidersRegisterCapability(t *testing.T) {
	t.Parallel()

	extSide, hostSide := newChannelTransportPair()
	ext := New("sdk-ext", "1.0.0").
		WithTransport(extSide).
		RegisterReviewProvider("sdk-review", ReviewProvider{})

	errCh := runExtension(t, ext)
	initializeExtension(t, hostSide)

	requestErr := sendRequestExpectError(t, hostSide, "2", "fetch_reviews", map[string]any{
		"provider": "sdk-review",
		"pr":       "123",
	})
	if requestErr.Code != -32001 {
		t.Fatalf("fetch_reviews error code = %d, want -32001", requestErr.Code)
	}

	var data struct {
		Method   string       `json:"method"`
		Required []Capability `json:"required"`
	}
	if err := json.Unmarshal(requestErr.Data, &data); err != nil {
		t.Fatalf("unmarshal error data: %v", err)
	}
	if data.Method != "fetch_reviews" {
		t.Fatalf("error method = %q, want fetch_reviews", data.Method)
	}
	if len(data.Required) != 1 || data.Required[0] != CapabilityProvidersRegister {
		t.Fatalf("required capabilities = %#v, want [providers.register]", data.Required)
	}

	sendRequestExpectResult(
		t,
		hostSide,
		"3",
		"shutdown",
		ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
		&ShutdownResponse{},
	)
	waitForTerminalError(t, errCh, nil)
}

func runExtension(t *testing.T, ext *Extension) <-chan error {
	t.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ext.Start(context.Background())
	}()
	return errCh
}

func initializeExtension(t *testing.T, transport *channelTransport) InitializeResponse {
	t.Helper()

	var response InitializeResponse
	sendRequestExpectResult(t, transport, "1", "initialize", InitializeRequest{
		ProtocolVersion:           ProtocolVersion,
		SupportedProtocolVersions: []string{ProtocolVersion},
		CompozyVersion:            "dev",
		Extension: InitializeRequestIdentity{
			Name:    "sdk-ext",
			Version: "1.0.0",
			Source:  "workspace",
		},
		Runtime: InitializeRuntime{
			RunID:                 "run-001",
			WorkspaceRoot:         ".",
			InvokingCommand:       "start",
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  5000,
			HealthCheckIntervalMS: 1000,
		},
	}, &response)
	return response
}

func sendRequestExpectResult(
	t *testing.T,
	transport *channelTransport,
	id string,
	method string,
	params any,
	target any,
) {
	t.Helper()

	if err := transport.WriteMessage(Message{
		ID:     MustJSON(id),
		Method: method,
		Params: MustJSON(params),
	}); err != nil {
		t.Fatalf("WriteMessage(%s) error = %v", method, err)
	}

	response, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(%s) error = %v", method, err)
	}
	if response.Error != nil {
		t.Fatalf("%s response error = %v", method, response.Error)
	}
	if err := unmarshalJSON(response.Result, target); err != nil {
		t.Fatalf("unmarshal %s result: %v", method, err)
	}
}

func sendRequestExpectError(
	t *testing.T,
	transport *channelTransport,
	id string,
	method string,
	params any,
) *Error {
	t.Helper()

	if err := transport.WriteMessage(Message{
		ID:     MustJSON(id),
		Method: method,
		Params: MustJSON(params),
	}); err != nil {
		t.Fatalf("WriteMessage(%s) error = %v", method, err)
	}

	response, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(%s) error = %v", method, err)
	}
	if response.Error == nil {
		t.Fatalf("%s response error = nil, want error", method)
	}
	return response.Error
}

func expectRPCError(t *testing.T, fn func() (Message, error)) *Error {
	t.Helper()

	_, err := fn()
	return assertRPCErrorCode(t, err)
}

func assertRPCErrorCode(t *testing.T, err error) *Error {
	t.Helper()

	var requestErr *Error
	if !errors.As(err, &requestErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	return requestErr
}

func waitForTerminalError(t *testing.T, errCh <-chan error, match func(error) bool) {
	t.Helper()

	select {
	case err := <-errCh:
		switch {
		case match == nil && err != nil:
			t.Fatalf("terminal error = %v, want nil", err)
		case match != nil && !match(err):
			t.Fatalf("terminal error = %v, did not match expectation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal error")
	}
}
