package aghsdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	transportErrorKey = "error"
)

// DefaultMaxMessageBytes is the default JSON-RPC line size limit.
const DefaultMaxMessageBytes = 10 * 1024 * 1024

// JSONRPCVersion is the JSON-RPC protocol version used by the subprocess runtime.
const JSONRPCVersion = "2.0"

// JSONRPCRequestEnvelope is the public request metadata visible to handlers.
type JSONRPCRequestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// TransportHandler handles one inbound JSON-RPC request.
type TransportHandler func(context.Context, json.RawMessage, JSONRPCRequestEnvelope) (any, error)

// Transport is the bidirectional JSON-RPC runtime used by Extension.
type Transport interface {
	Handle(method string, handler TransportHandler)
	Call(ctx context.Context, method string, params any, result any) error
	Run(ctx context.Context) error
	Close() error
}

type pendingCall struct {
	result chan json.RawMessage
	err    chan error
}

type requestFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  any             `json:"params,omitempty"`
}

type responseFrame struct {
	JSONRPC string              `json:"jsonrpc"`
	ID      json.RawMessage     `json:"id"`
	Result  json.RawMessage     `json:"result,omitempty"`
	Error   *JSONRPCErrorObject `json:"error,omitempty"`
}

// StdioTransport is a newline-delimited JSON-RPC 2.0 transport over stdio.
type StdioTransport struct {
	input           io.Reader
	output          io.Writer
	maxMessageBytes int

	mu       sync.Mutex
	handlers map[string]TransportHandler
	pending  map[string]pendingCall
	nextID   int64
	started  bool
	closed   bool
	err      error
	done     chan struct{}
	cancel   context.CancelFunc
	once     sync.Once
	writeMu  sync.Mutex
}

// StdioTransportOptions configures a StdioTransport.
type StdioTransportOptions struct {
	Input           io.Reader
	Output          io.Writer
	MaxMessageBytes int
}

// NewStdioTransport creates a stdio JSON-RPC transport.
func NewStdioTransport(options StdioTransportOptions) *StdioTransport {
	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}
	maxBytes := options.MaxMessageBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxMessageBytes
	}
	return &StdioTransport{
		input:           input,
		output:          output,
		maxMessageBytes: maxBytes,
		handlers:        make(map[string]TransportHandler),
		pending:         make(map[string]pendingCall),
		done:            make(chan struct{}),
	}
}

// Handle registers or replaces an inbound method handler.
func (t *StdioTransport) Handle(method string, handler TransportHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[strings.TrimSpace(method)] = handler
}

// Run starts the transport read loop and blocks until close or context cancellation.
func (t *StdioTransport) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("transport: context is required")
	}
	if err := t.start(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		t.fail(ctx.Err())
		return ctx.Err()
	case <-t.done:
		t.mu.Lock()
		err := t.err
		t.mu.Unlock()
		if err == nil || errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}

// Close closes the transport and rejects pending calls.
func (t *StdioTransport) Close() error {
	t.fail(errors.New("transport closed"))
	return nil
}

// Call sends one JSON-RPC request and decodes its response into result.
func (t *StdioTransport) Call(ctx context.Context, method string, params any, result any) error {
	if ctx == nil {
		return errors.New("transport: context is required")
	}
	cleanMethod := strings.TrimSpace(method)
	if cleanMethod == "" {
		return NewInvalidParamsError("method is required", nil)
	}
	if err := t.start(context.Background()); err != nil {
		return err
	}

	t.mu.Lock()
	if t.closed {
		err := t.err
		t.mu.Unlock()
		if err == nil {
			err = errors.New("transport closed")
		}
		return err
	}
	t.nextID++
	idBytes, err := json.Marshal(t.nextID)
	if err != nil {
		t.mu.Unlock()
		return wrapTransportError("transport: marshal request id", err)
	}
	id := json.RawMessage(idBytes)
	key := pendingKey(id)
	pending := pendingCall{result: make(chan json.RawMessage, 1), err: make(chan error, 1)}
	t.pending[key] = pending
	t.mu.Unlock()

	frame := requestFrame{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  cleanMethod,
		Params:  params,
	}
	if err := t.writeFrame(frame); err != nil {
		t.removePending(key)
		return err
	}

	select {
	case raw := <-pending.result:
		if result == nil || len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, result); err != nil {
			return wrapTransportError("transport: decode response", err)
		}
		return nil
	case err := <-pending.err:
		return err
	case <-ctx.Done():
		t.removePending(key)
		return ctx.Err()
	case <-t.done:
		t.removePending(key)
		t.mu.Lock()
		err := t.err
		t.mu.Unlock()
		if err == nil {
			err = errors.New("transport closed")
		}
		return err
	}
}

func (t *StdioTransport) start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		if t.err != nil {
			return t.err
		}
		return errors.New("transport closed")
	}
	if t.started {
		return nil
	}
	if ctx == nil {
		return errors.New("transport: context is required")
	}
	loopCtx, cancel := context.WithCancel(ctx)
	started := false
	defer func() {
		if !started {
			cancel()
		}
	}()
	t.started = true
	t.cancel = cancel
	go t.readLoop(loopCtx)
	started = true
	return nil
}

func (t *StdioTransport) readLoop(ctx context.Context) {
	reader := bufio.NewReaderSize(t.input, 64*1024)
	for {
		select {
		case <-ctx.Done():
			t.fail(ctx.Err())
			return
		default:
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if len(line) > 0 {
				t.processLine(ctx, line)
			}
			t.fail(err)
			return
		}
		if len(line) > t.maxMessageBytes+1 {
			t.fail(fmt.Errorf("message exceeds %d bytes", t.maxMessageBytes))
			return
		}
		t.processLine(ctx, line)
	}
}

func (t *StdioTransport) processLine(ctx context.Context, line []byte) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return
	}
	if len(trimmed) > t.maxMessageBytes {
		t.fail(fmt.Errorf("message exceeds %d bytes", t.maxMessageBytes))
		return
	}

	var envelope struct {
		JSONRPC string              `json:"jsonrpc"`
		ID      json.RawMessage     `json:"id,omitempty"`
		Method  string              `json:"method,omitempty"`
		Params  json.RawMessage     `json:"params,omitempty"`
		Result  json.RawMessage     `json:"result,omitempty"`
		Error   *JSONRPCErrorObject `json:"error,omitempty"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		t.fail(NewRPCError(-32700, "Parse error", map[string]any{transportErrorKey: err.Error()}))
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		t.fail(NewRPCError(-32700, "Parse error", map[string]any{transportErrorKey: err.Error()}))
		return
	}
	if envelope.JSONRPC != JSONRPCVersion {
		t.fail(NewInvalidRequestError("jsonrpc must be 2.0"))
		return
	}
	if envelope.Method != "" {
		if len(envelope.ID) == 0 || bytes.Equal(envelope.ID, []byte("null")) {
			return
		}
		request := JSONRPCRequestEnvelope{
			JSONRPC: envelope.JSONRPC,
			ID:      cloneRawMessage(envelope.ID),
			Method:  envelope.Method,
			Params:  cloneRawMessage(envelope.Params),
		}
		go t.dispatchRequest(ctx, request)
		return
	}
	if len(envelope.ID) > 0 {
		_, hasResult := fields["result"]
		_, hasError := fields[transportErrorKey]
		t.dispatchResponse(envelope.ID, envelope.Result, envelope.Error, hasResult, hasError)
		return
	}
	t.fail(NewInvalidRequestError("invalid json-rpc envelope"))
}

func (t *StdioTransport) dispatchRequest(ctx context.Context, request JSONRPCRequestEnvelope) {
	t.mu.Lock()
	handler := t.handlers[strings.TrimSpace(request.Method)]
	t.mu.Unlock()
	if handler == nil {
		t.sendError(request.ID, NewMethodNotFoundError(request.Method))
		return
	}
	result, err := handler(ctx, cloneRawMessage(request.Params), request)
	if err != nil {
		t.sendError(request.ID, ensureRPCError(err))
		return
	}
	t.sendResult(request.ID, result)
}

func (t *StdioTransport) dispatchResponse(
	id json.RawMessage,
	result json.RawMessage,
	rpcErr *JSONRPCErrorObject,
	hasResult bool,
	hasError bool,
) {
	key := pendingKey(id)
	t.mu.Lock()
	pending, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.mu.Unlock()
	if !ok {
		return
	}
	switch {
	case rpcErr != nil && hasResult:
		pending.err <- NewInvalidRequestError("response must not include both result and error")
		return
	case rpcErr != nil:
		pending.err <- rpcErrorFromObject(*rpcErr)
		return
	case hasError:
		pending.err <- NewInvalidRequestError("response error must be an object")
		return
	case !hasResult:
		pending.err <- NewInvalidRequestError("response must include result or error")
		return
	default:
		pending.result <- cloneRawMessage(result)
	}
}

func (t *StdioTransport) sendResult(id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		t.sendError(id, NewInternalError(err.Error()))
		return
	}
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	if err := t.writeFrame(responseFrame{
		JSONRPC: JSONRPCVersion,
		ID:      cloneRawMessage(id),
		Result:  raw,
	}); err != nil {
		t.fail(err)
	}
}

func (t *StdioTransport) sendError(id json.RawMessage, err *RPCError) {
	obj := err.object()
	if writeErr := t.writeFrame(responseFrame{
		JSONRPC: JSONRPCVersion,
		ID:      cloneRawMessage(id),
		Error:   &obj,
	}); writeErr != nil {
		t.fail(writeErr)
	}
}

func (t *StdioTransport) writeFrame(frame any) error {
	encoded, err := json.Marshal(frame)
	if err != nil {
		return wrapTransportError("transport: marshal frame", err)
	}
	if len(encoded) > t.maxMessageBytes {
		return fmt.Errorf("message exceeds %d bytes", t.maxMessageBytes)
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.output.Write(append(encoded, '\n')); err != nil {
		return wrapTransportError("transport: write frame", err)
	}
	return nil
}

func (t *StdioTransport) fail(err error) {
	t.once.Do(func() {
		t.mu.Lock()
		t.closed = true
		t.err = err
		cancel := t.cancel
		t.cancel = nil
		pending := t.pending
		t.pending = make(map[string]pendingCall)
		close(t.done)
		t.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		for _, call := range pending {
			call.err <- err
		}
	})
}

func (t *StdioTransport) removePending(key string) {
	t.mu.Lock()
	delete(t.pending, key)
	t.mu.Unlock()
}

func pendingKey(id json.RawMessage) string {
	return string(bytes.TrimSpace(id))
}
