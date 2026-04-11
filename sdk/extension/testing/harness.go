package exttesting

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/sdk/extension"
)

// HostCall records one Host API request emitted by the extension.
type HostCall struct {
	Method string
	Params json.RawMessage
}

// HostHandler serves one Host API method inside the test harness.
type HostHandler func(context.Context, json.RawMessage) (any, error)

// HarnessOptions configures the host-side initialize request defaults.
type HarnessOptions struct {
	ProtocolVersion           string
	SupportedProtocolVersions []string
	CompozyVersion            string
	Source                    string
	GrantedCapabilities       []extension.Capability
	Runtime                   extension.InitializeRuntime
}

// TestHarness simulates the Compozy host for in-process SDK tests.
type TestHarness struct {
	options HarnessOptions

	extensionSide *MockTransport
	hostSide      *MockTransport
	hostPeer      *rpcPeer

	mu       sync.Mutex
	handlers map[string]HostHandler
	calls    []HostCall
}

// NewTestHarness constructs a new in-process host harness.
func NewTestHarness(options HarnessOptions) *TestHarness {
	extensionSide, hostSide := NewMockTransportPair()
	harness := &TestHarness{
		options:       normalizeHarnessOptions(options),
		extensionSide: extensionSide,
		hostSide:      hostSide,
		handlers:      make(map[string]HostHandler),
	}
	harness.hostPeer = newRPCPeer(hostSide, harness.handleHostRequest)
	harness.hostPeer.start()
	return harness
}

// Transport returns the extension-side transport to pass into the SDK.
func (h *TestHarness) Transport() extension.Transport {
	if h == nil {
		return nil
	}
	return h.extensionSide
}

// Run starts the extension against the harness transport.
func (h *TestHarness) Run(ctx context.Context, ext *extension.Extension) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		if ext == nil {
			errCh <- fmt.Errorf("run test harness: missing extension")
			return
		}
		errCh <- ext.WithTransport(h.Transport()).Start(ctx)
	}()
	return errCh
}

// Initialize performs the host-initiated initialize handshake.
func (h *TestHarness) Initialize(
	ctx context.Context,
	identity extension.InitializeRequestIdentity,
) (*extension.InitializeResponse, error) {
	if h == nil {
		return nil, fmt.Errorf("initialize test harness: missing harness")
	}

	request := extension.InitializeRequest{
		ProtocolVersion:           h.options.ProtocolVersion,
		SupportedProtocolVersions: append([]string(nil), h.options.SupportedProtocolVersions...),
		CompozyVersion:            h.options.CompozyVersion,
		Extension:                 identity,
		GrantedCapabilities:       append([]extension.Capability(nil), h.options.GrantedCapabilities...),
		Runtime:                   h.options.Runtime,
	}

	var response extension.InitializeResponse
	if err := h.hostPeer.call(ctx, "initialize", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// DispatchHook issues one execute_hook request against the running extension.
func (h *TestHarness) DispatchHook(
	ctx context.Context,
	invocationID string,
	hook extension.HookInfo,
	payload any,
) (*extension.ExecuteHookResponse, error) {
	if h == nil {
		return nil, fmt.Errorf("dispatch hook: missing harness")
	}

	request := extension.ExecuteHookRequest{
		InvocationID: strings.TrimSpace(invocationID),
		Hook:         hook,
		Payload:      extension.MustJSON(payload),
	}

	var response extension.ExecuteHookResponse
	if err := h.hostPeer.call(ctx, "execute_hook", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// SendEvent issues one on_event request against the running extension.
func (h *TestHarness) SendEvent(ctx context.Context, event events.Event) error {
	if h == nil {
		return fmt.Errorf("send event: missing harness")
	}
	return h.hostPeer.call(ctx, "on_event", extension.OnEventRequest{Event: event}, &map[string]any{})
}

// HealthCheck issues one health_check request against the running extension.
func (h *TestHarness) HealthCheck(ctx context.Context) (*extension.HealthCheckResponse, error) {
	if h == nil {
		return nil, fmt.Errorf("health check: missing harness")
	}

	var response extension.HealthCheckResponse
	if err := h.hostPeer.call(ctx, "health_check", extension.HealthCheckRequest{}, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// Shutdown issues one shutdown request against the running extension.
func (h *TestHarness) Shutdown(
	ctx context.Context,
	req extension.ShutdownRequest,
) (*extension.ShutdownResponse, error) {
	if h == nil {
		return nil, fmt.Errorf("shutdown: missing harness")
	}

	var response extension.ShutdownResponse
	if err := h.hostPeer.call(ctx, "shutdown", req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// HandleHostMethod registers a Host API method handler.
func (h *TestHarness) HandleHostMethod(method string, handler HostHandler) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers[strings.TrimSpace(method)] = handler
}

// HostCalls returns the Host API calls emitted by the extension so far.
func (h *TestHarness) HostCalls() []HostCall {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	calls := make([]HostCall, len(h.calls))
	copy(calls, h.calls)
	return calls
}

func (h *TestHarness) handleHostRequest(
	ctx context.Context,
	message extension.Message,
) (any, *extension.Error) {
	h.mu.Lock()
	h.calls = append(h.calls, HostCall{
		Method: strings.TrimSpace(message.Method),
		Params: append(json.RawMessage(nil), message.Params...),
	})
	handler := h.handlers[strings.TrimSpace(message.Method)]
	h.mu.Unlock()

	if handler == nil {
		return nil, methodNotFoundError(message.Method)
	}

	result, err := handler(ctx, message.Params)
	if err != nil {
		return nil, toRequestError(err)
	}
	return result, nil
}

func normalizeHarnessOptions(options HarnessOptions) HarnessOptions {
	if strings.TrimSpace(options.ProtocolVersion) == "" {
		options.ProtocolVersion = extension.ProtocolVersion
	}
	if len(options.SupportedProtocolVersions) == 0 {
		options.SupportedProtocolVersions = []string{extension.ProtocolVersion}
	}
	if strings.TrimSpace(options.CompozyVersion) == "" {
		options.CompozyVersion = "dev"
	}
	if strings.TrimSpace(options.Source) == "" {
		options.Source = "workspace"
	}
	if options.Runtime.RunID == "" {
		options.Runtime.RunID = "run-test"
	}
	if options.Runtime.WorkspaceRoot == "" {
		options.Runtime.WorkspaceRoot = "."
	}
	if options.Runtime.InvokingCommand == "" {
		options.Runtime.InvokingCommand = "start"
	}
	if options.Runtime.ShutdownTimeoutMS == 0 {
		options.Runtime.ShutdownTimeoutMS = 1000
	}
	if options.Runtime.DefaultHookTimeoutMS == 0 {
		options.Runtime.DefaultHookTimeoutMS = 5000
	}
	return options
}

type rpcPeer struct {
	transport *MockTransport
	handler   func(context.Context, extension.Message) (any, *extension.Error)

	requestID atomic.Uint64

	pendingMu sync.Mutex
	pending   map[string]chan peerResult

	startOnce sync.Once
}

type peerResult struct {
	message extension.Message
	err     error
}

func newRPCPeer(
	transport *MockTransport,
	handler func(context.Context, extension.Message) (any, *extension.Error),
) *rpcPeer {
	return &rpcPeer{
		transport: transport,
		handler:   handler,
		pending:   make(map[string]chan peerResult),
	}
}

func (p *rpcPeer) start() {
	p.startOnce.Do(func() {
		go p.readLoop()
	})
}

func (p *rpcPeer) readLoop() {
	for {
		message, err := p.transport.ReadMessage()
		if err != nil {
			p.failPending(err)
			return
		}

		if strings.TrimSpace(message.Method) == "" {
			p.resolvePending(message)
			continue
		}

		go p.handleIncoming(message)
	}
}

func (p *rpcPeer) call(ctx context.Context, method string, params any, result any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	requestID := strconv.FormatUint(p.requestID.Add(1), 10)
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return err
	}

	responseCh := make(chan peerResult, 1)
	p.pendingMu.Lock()
	p.pending[requestID] = responseCh
	p.pendingMu.Unlock()
	defer p.removePending(requestID)

	if err := p.transport.WriteMessage(extension.Message{
		ID:     json.RawMessage(requestID),
		Method: strings.TrimSpace(method),
		Params: paramsRaw,
	}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case response := <-responseCh:
		if response.err != nil {
			return response.err
		}
		if response.message.Error != nil {
			return response.message.Error
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(response.message.Result, result)
	}
}

func (p *rpcPeer) handleIncoming(message extension.Message) {
	if p.handler == nil {
		if err := p.transport.WriteMessage(extension.Message{
			ID:    message.ID,
			Error: methodNotFoundError(strings.TrimSpace(message.Method)),
		}); err != nil {
			p.failPending(err)
		}
		return
	}

	result, requestErr := p.handler(context.Background(), message)
	if requestErr != nil {
		if err := p.transport.WriteMessage(extension.Message{ID: message.ID, Error: requestErr}); err != nil {
			p.failPending(err)
		}
		return
	}

	raw, err := json.Marshal(result)
	if err != nil {
		if writeErr := p.transport.WriteMessage(extension.Message{
			ID:    message.ID,
			Error: internalError(err),
		}); writeErr != nil {
			p.failPending(writeErr)
		}
		return
	}
	if err := p.transport.WriteMessage(extension.Message{ID: message.ID, Result: raw}); err != nil {
		p.failPending(err)
	}
}

func (p *rpcPeer) resolvePending(message extension.Message) {
	p.pendingMu.Lock()
	ch := p.pending[string(message.ID)]
	p.pendingMu.Unlock()
	if ch == nil {
		return
	}
	ch <- peerResult{message: message}
}

func (p *rpcPeer) failPending(err error) {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()

	for key, ch := range p.pending {
		ch <- peerResult{err: err}
		delete(p.pending, key)
	}
}

func (p *rpcPeer) removePending(id string) {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	delete(p.pending, id)
}

func toRequestError(err error) *extension.Error {
	if err == nil {
		return nil
	}
	requestErr, ok := err.(*extension.Error)
	if ok {
		return requestErr
	}
	return internalError(err)
}

func methodNotFoundError(method string) *extension.Error {
	return &extension.Error{
		Code:    -32601,
		Message: "Method not found",
		Data:    mustJSON(map[string]any{"method": strings.TrimSpace(method)}),
	}
}

func internalError(err error) *extension.Error {
	return &extension.Error{
		Code:    -32603,
		Message: "Internal error",
		Data:    mustJSON(map[string]any{"error": err.Error()}),
	}
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

// SortedHostCalls returns the recorded Host API methods in lexical order.
func SortedHostCalls(calls []HostCall) []string {
	methods := make([]string, 0, len(calls))
	for _, call := range calls {
		methods = append(methods, call.Method)
	}
	sort.Strings(methods)
	return methods
}
