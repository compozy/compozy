package subprocess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	transportErrorKey  = "error"
	transportMethodKey = "method"
)

const (
	jsonRPCVersion = "2.0"

	codeParseError       = -32700
	codeInvalidRequest   = -32600
	codeMethodNotFound   = -32601
	codeInvalidParams    = -32602
	codeInternalError    = -32603
	codeCapabilityDenied = -32001
	codeNotInitialized   = -32003
	codeShutdownProgress = -32004
)

// ErrTransportClosedBeforeResponse reports a subprocess transport that closed
// while an RPC call was still waiting for its response.
var ErrTransportClosedBeforeResponse = errors.New("subprocess: transport closed before response")

// ResponseDecodeError reports that a JSON-RPC response arrived but its result
// could not be materialized into the caller's negotiated response type.
type ResponseDecodeError struct {
	Method string
	Err    error
}

// Error implements error.
func (e *ResponseDecodeError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("subprocess: decode %q response: %v", e.Method, e.Err)
}

// Unwrap exposes the underlying JSON decode failure.
func (e *ResponseDecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// HandlerFunc handles one inbound JSON-RPC request.
type HandlerFunc func(context.Context, json.RawMessage) (any, error)

// RPCError models a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("json-rpc error %d", e.Code)
	}
	return fmt.Sprintf("json-rpc error %d: %s", e.Code, e.Message)
}

// NewRPCError constructs a JSON-RPC error with optional structured data.
func NewRPCError(code int, message string, data any) *RPCError {
	err := &RPCError{
		Code:    code,
		Message: message,
	}
	if data == nil {
		return err
	}
	encoded, marshalErr := json.Marshal(data)
	if marshalErr == nil && string(encoded) != "null" {
		err.Data = encoded
	}
	return err
}

type transport struct {
	process         *Process
	maxMessageBytes int

	handlersMu sync.RWMutex
	handlers   map[string]HandlerFunc

	pendingMu sync.Mutex
	pending   map[string]chan callResult

	writeMu sync.Mutex

	readerDone chan struct{}
	seq        atomic.Int64
}

type callResult struct {
	result json.RawMessage
	err    *RPCError
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  any             `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type rpcID struct {
	raw json.RawMessage
	key string
}

func newTransport(process *Process, maxMessageBytes int) *transport {
	return &transport{
		process:         process,
		maxMessageBytes: maxMessageBytes,
		handlers:        make(map[string]HandlerFunc),
		pending:         make(map[string]chan callResult),
		readerDone:      make(chan struct{}),
	}
}

func (t *transport) start() {
	go t.readLoop()
}

func (t *transport) shutdown(waitErr error) {
	<-t.readerDone
	if waitErr == nil {
		waitErr = t.process.currentTransportError()
	}
	t.closePending(waitErr)
}

func (t *transport) handleMethod(method string, handler HandlerFunc) error {
	if strings.TrimSpace(method) == "" {
		return errors.New("subprocess: method is required")
	}
	if handler == nil {
		return errors.New("subprocess: handler is required")
	}

	t.handlersMu.Lock()
	defer t.handlersMu.Unlock()
	t.handlers[method] = handler
	return nil
}

func (t *transport) call(ctx context.Context, method string, params, result any) error {
	requestID := t.nextRequestID()
	responseCh := make(chan callResult, 1)

	t.pendingMu.Lock()
	t.pending[requestID.key] = responseCh
	t.pendingMu.Unlock()

	request := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      requestID.raw,
		Method:  method,
		Params:  params,
	}
	if err := t.writeJSON(request); err != nil {
		t.pendingMu.Lock()
		delete(t.pending, requestID.key)
		t.pendingMu.Unlock()
		return err
	}

	select {
	case response, ok := <-responseCh:
		if !ok {
			if transportErr := t.process.currentTransportError(); transportErr != nil {
				return transportErr
			}
			return ErrTransportClosedBeforeResponse
		}
		if response.err != nil {
			return response.err
		}
		if result == nil || len(response.result) == 0 || bytes.Equal(response.result, []byte("null")) {
			return nil
		}
		if err := json.Unmarshal(response.result, result); err != nil {
			return &ResponseDecodeError{Method: method, Err: err}
		}
		return nil
	case <-ctx.Done():
		t.pendingMu.Lock()
		delete(t.pending, requestID.key)
		t.pendingMu.Unlock()
		return fmt.Errorf("subprocess: call %q: %w", method, ctx.Err())
	case <-t.process.Done():
		return t.process.Wait()
	}
}

func (t *transport) nextRequestID() rpcID {
	id := t.seq.Add(1)
	raw := json.RawMessage(strconv.AppendInt(nil, id, 10))
	return rpcID{raw: raw, key: "n:" + strconv.FormatInt(id, 10)}
}

func (t *transport) writeJSON(message any) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("subprocess: encode message: %w", err)
	}
	if len(encoded) > t.maxMessageBytes {
		return fmt.Errorf("subprocess: message exceeds %d bytes", t.maxMessageBytes)
	}
	encoded = append(encoded, '\n')

	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.process.stdin.Write(encoded); err != nil {
		return fmt.Errorf("subprocess: write frame: %w", err)
	}
	return nil
}

func (t *transport) readLoop() {
	defer close(t.readerDone)

	scanner := bufio.NewScanner(t.process.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), t.maxMessageBytes+1)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var envelope rpcEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			t.failTransport(fmt.Errorf("subprocess: decode frame: %w", err))
			return
		}
		if envelope.JSONRPC != jsonRPCVersion {
			t.failTransport(fmt.Errorf("subprocess: unsupported jsonrpc version %q", envelope.JSONRPC))
			return
		}

		if strings.TrimSpace(envelope.Method) != "" {
			t.handleRequest(envelope)
			continue
		}
		t.handleResponse(envelope)
	}

	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			t.failTransport(fmt.Errorf("subprocess: message exceeds %d bytes", t.maxMessageBytes))
			return
		}
		if !errors.Is(err, io.EOF) {
			t.failTransport(fmt.Errorf("subprocess: read frame: %w", err))
		}
	}
}

func (t *transport) handleResponse(envelope rpcEnvelope) {
	id, err := parseRPCID(envelope.ID)
	if err != nil {
		t.failTransport(fmt.Errorf("subprocess: invalid response id: %w", err))
		return
	}

	t.pendingMu.Lock()
	responseCh, ok := t.pending[id.key]
	if ok {
		delete(t.pending, id.key)
	}
	t.pendingMu.Unlock()
	if !ok {
		return
	}

	responseCh <- callResult{
		result: envelope.Result,
		err:    envelope.Error,
	}
	close(responseCh)
}

func (t *transport) handleRequest(envelope rpcEnvelope) {
	if len(envelope.ID) == 0 {
		return
	}

	id, err := parseRPCID(envelope.ID)
	if err != nil {
		t.sendErrorOrFail(
			envelope.ID,
			NewRPCError(codeInvalidRequest, "Invalid request", map[string]string{"reason": err.Error()}),
			"subprocess: send invalid request error",
		)
		return
	}

	switch t.process.currentState() {
	case processStateStarting:
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeNotInitialized, "Not initialized", nil),
			"subprocess: send not initialized error",
		)
		return
	case processStateDraining:
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeShutdownProgress, "Shutdown in progress", nil),
			"subprocess: send shutdown-in-progress error",
		)
		return
	case processStateStopped:
		return
	}

	t.handlersMu.RLock()
	handler, ok := t.handlers[envelope.Method]
	t.handlersMu.RUnlock()
	if !ok {
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeMethodNotFound, "Method not found", map[string]string{transportMethodKey: envelope.Method}),
			"subprocess: send method-not-found error",
		)
		return
	}

	go func() {
		result, callErr := handler(t.process.lifecycleCtx, envelope.Params)
		if callErr != nil {
			if rpcErr, ok := errors.AsType[*RPCError](callErr); ok {
				t.sendErrorOrFail(id.raw, rpcErr, "subprocess: send handler rpc error")
				return
			}
			t.sendErrorOrFail(
				id.raw,
				NewRPCError(codeInternalError, "Internal error", map[string]string{transportErrorKey: callErr.Error()}),
				"subprocess: send internal error",
			)
			return
		}
		t.sendResultOrFail(id.raw, result, "subprocess: send result")
	}()
}

func (t *transport) sendResult(id json.RawMessage, result any) error {
	return t.writeJSON(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
}

func (t *transport) sendError(id json.RawMessage, err *RPCError) error {
	return t.writeJSON(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   err,
	})
}

func (t *transport) closePending(reason error) {
	if reason != nil {
		t.process.recordTransportError(reason)
	}
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	for key, responseCh := range t.pending {
		delete(t.pending, key)
		close(responseCh)
	}
}

func (t *transport) failTransport(err error) {
	if closeErr := t.process.closeInput(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	t.closePending(err)
}

func (t *transport) sendErrorOrFail(id json.RawMessage, rpcErr *RPCError, operation string) {
	if err := t.sendError(id, rpcErr); err != nil {
		t.failTransport(fmt.Errorf("%s: %w", operation, err))
	}
}

func (t *transport) sendResultOrFail(id json.RawMessage, result any, operation string) {
	if err := t.sendResult(id, result); err != nil {
		t.failTransport(fmt.Errorf("%s: %w", operation, err))
	}
}

func parseRPCID(raw json.RawMessage) (rpcID, error) {
	if len(raw) == 0 {
		return rpcID{}, errors.New("missing id")
	}

	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return rpcID{}, fmt.Errorf("decode string id: %w", err)
		}
		if value == "" {
			return rpcID{}, errors.New("string id must not be empty")
		}
		return rpcID{raw: append(json.RawMessage(nil), raw...), key: "s:" + value}, nil
	}

	text := strings.TrimSpace(string(raw))
	if strings.Contains(text, ".") {
		return rpcID{}, errors.New("fractional numeric ids are not supported")
	}
	if _, err := strconv.ParseInt(text, 10, 64); err != nil {
		return rpcID{}, fmt.Errorf("decode numeric id: %w", err)
	}
	return rpcID{raw: append(json.RawMessage(nil), raw...), key: "n:" + text}, nil
}
