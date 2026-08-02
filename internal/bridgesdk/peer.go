package bridgesdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/internal/subprocess"
)

const (
	peerErrorKey = "error"
)

const bridgeSDKJSONRPCVersion = "2.0"

const (
	bridgeSDKRPCCodeMethodNotFound  = -32601
	bridgeSDKRPCCodeInternal        = -32603
	bridgeSDKRPCCodeInvalidParams   = -32602
	bridgeSDKRPCCodeNotInitialized  = -32003
	bridgeSDKRPCCodeShutdownRunning = -32004
)

// RPCHandler handles one inbound JSON-RPC request.
type RPCHandler func(context.Context, json.RawMessage) (any, error)

type rpcEnvelope struct {
	JSONRPC string               `json:"jsonrpc"`
	ID      json.RawMessage      `json:"id,omitempty"`
	Method  string               `json:"method,omitempty"`
	Params  json.RawMessage      `json:"params,omitempty"`
	Result  json.RawMessage      `json:"result,omitempty"`
	Error   *subprocess.RPCError `json:"error,omitempty"`
}

type rpcResult struct {
	result json.RawMessage
	err    *subprocess.RPCError
}

// Peer is the bridge-sdk JSON-RPC transport used by provider runtimes to
// receive daemon requests and issue Host API calls over the same stdio stream.
type Peer struct {
	stdin   io.ReadCloser
	scanner *bufio.Scanner
	stdout  io.Writer

	handlersMu sync.RWMutex
	handlers   map[string]RPCHandler

	stateMu      sync.Mutex
	phase        peerPhase
	serveStarted bool
	terminalDone chan struct{}
	terminalErr  error
	pending      map[string]chan rpcResult

	writeMu sync.Mutex
	wg      sync.WaitGroup
	nextID  atomic.Int64
}

// NewPeer constructs a JSON-RPC peer bound to the provided reader and writer.
// Stdin must implement io.ReadCloser. The peer owns it after construction and
// closes it to interrupt reads when serving terminates.
func NewPeer(stdin io.Reader, stdout io.Writer) (*Peer, error) {
	if isNilPeerIO(stdin) {
		return nil, errors.New("bridgesdk: peer input is required")
	}
	readCloser, ok := stdin.(io.ReadCloser)
	if !ok || isNilPeerIO(readCloser) {
		return nil, errors.New("bridgesdk: peer input must be closable")
	}
	if isNilPeerIO(stdout) {
		return nil, errors.New("bridgesdk: peer output is required")
	}

	scanner := bufio.NewScanner(readCloser)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	return &Peer{
		stdin:        readCloser,
		scanner:      scanner,
		stdout:       stdout,
		handlers:     make(map[string]RPCHandler),
		terminalDone: make(chan struct{}),
		pending:      make(map[string]chan rpcResult),
	}, nil
}

// Handle registers one inbound method handler.
func (p *Peer) Handle(method string, handler RPCHandler) error {
	if p == nil {
		return errors.New("bridgesdk: peer is required")
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("bridgesdk: peer method is required")
	}
	if handler == nil {
		return errors.New("bridgesdk: peer handler is required")
	}

	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	p.handlers[strings.TrimSpace(method)] = handler
	return nil
}

// Call issues one outbound JSON-RPC request and decodes the typed result.
func (p *Peer) Call(ctx context.Context, method string, params any, result any) error {
	if p == nil {
		return errors.New("bridgesdk: peer is required")
	}
	if ctx == nil {
		return errors.New("bridgesdk: call context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("bridgesdk: call method is required")
	}

	paramsRaw, err := marshalParams(params)
	if err != nil {
		return fmt.Errorf("bridgesdk: encode %q params: %w", strings.TrimSpace(method), err)
	}

	requestID := strconv.FormatInt(p.nextID.Add(1), 10)
	responseCh := make(chan rpcResult, 1)
	if err := p.admitCall(ctx, requestID, responseCh); err != nil {
		return err
	}

	if err := p.writeFrame(rpcEnvelope{
		JSONRPC: bridgeSDKJSONRPCVersion,
		ID:      json.RawMessage(strconv.AppendQuote(nil, requestID)),
		Method:  strings.TrimSpace(method),
		Params:  paramsRaw,
	}); err != nil {
		return p.terminate(err)
	}

	select {
	case <-ctx.Done():
		if terminalDone := p.cancelPendingCall(requestID); terminalDone != nil {
			<-terminalDone
			return p.currentTerminalError()
		}
		return ctx.Err()
	case response, ok := <-responseCh:
		if !ok {
			return p.currentTerminalError()
		}
		if response.err != nil {
			return response.err
		}
		if result == nil || len(response.result) == 0 || bytes.Equal(response.result, []byte("null")) {
			return nil
		}
		if err := json.Unmarshal(response.result, result); err != nil {
			return fmt.Errorf("bridgesdk: decode %q response: %w", method, err)
		}
		return nil
	}
}

// Serve owns the peer's sole read loop. On cancellation, EOF, or a transport
// failure it closes the input, cancels inbound handlers, and joins their work.
func (p *Peer) Serve(ctx context.Context) error {
	if p == nil {
		return errors.New("bridgesdk: peer is required")
	}
	if ctx == nil {
		return errors.New("bridgesdk: serve context is required")
	}
	if err := p.beginServe(ctx); err != nil {
		return err
	}

	handlerCtx, cancelHandlers := context.WithCancel(ctx)
	defer cancelHandlers()
	cancelDone := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		defer close(cancelDone)
		p.publishTerminal(ctx.Err())
	})
	defer func() {
		if !stopCancel() {
			<-cancelDone
		}
	}()

	var terminalErr error

	for p.scanner.Scan() {
		if err := ctx.Err(); err != nil {
			terminalErr = p.terminate(err)
			break
		}

		line := bytes.TrimSpace(p.scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var envelope rpcEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			terminalErr = p.terminate(fmt.Errorf("bridgesdk: decode rpc frame: %w", err))
			break
		}
		if envelope.JSONRPC != bridgeSDKJSONRPCVersion {
			terminalErr = p.terminate(fmt.Errorf(
				"bridgesdk: unsupported jsonrpc version %q",
				envelope.JSONRPC,
			))
			break
		}

		if strings.TrimSpace(envelope.Method) != "" {
			if !p.startDispatch(handlerCtx, envelope) {
				terminalErr = p.terminate(ErrPeerClosed)
				break
			}
			continue
		}

		p.handleResponse(envelope)
	}

	if terminalErr == nil {
		cause := ErrPeerClosed
		if err := ctx.Err(); err != nil {
			cause = err
		} else if err := p.scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			cause = fmt.Errorf("bridgesdk: read rpc frame: %w", err)
		}
		terminalErr = p.terminate(cause)
	}
	cancelHandlers()
	p.wg.Wait()
	if terminalErr == ErrPeerClosed {
		return nil
	}
	return terminalErr
}

func (p *Peer) dispatchRequest(ctx context.Context, envelope rpcEnvelope) {
	method := strings.TrimSpace(envelope.Method)
	idKey := rpcIDKey(envelope.ID)
	if method == "" || idKey == "" {
		return
	}

	p.handlersMu.RLock()
	handler, ok := p.handlers[method]
	p.handlersMu.RUnlock()
	if !ok {
		if err := p.sendError(envelope.ID, subprocess.NewRPCError(
			bridgeSDKRPCCodeMethodNotFound,
			"Method not found",
			map[string]string{"method": method},
		)); err != nil {
			p.failTransport(ctx, fmt.Errorf("bridgesdk: send method-not-found error: %w", err))
		}
		return
	}

	result, err := invokeRPCHandler(ctx, handler, envelope.Params)
	if err != nil {
		if panicErr, ok := errors.AsType[*rpcHandlerPanicError](err); ok {
			recordRPCHandlerPanic(ctx, method, panicErr)
			if sendErr := p.sendError(envelope.ID, subprocess.NewRPCError(
				bridgeSDKRPCCodeInternal,
				"Internal error",
				map[string]string{peerErrorKey: "handler panicked"},
			)); sendErr != nil {
				p.failTransport(ctx, fmt.Errorf("bridgesdk: send handler-panic error: %w", sendErr))
			}
			return
		}
		if rpcErr, ok := errors.AsType[*subprocess.RPCError](err); ok {
			if sendErr := p.sendError(envelope.ID, rpcErr); sendErr != nil {
				p.failTransport(ctx, fmt.Errorf("bridgesdk: send rpc error: %w", sendErr))
			}
			return
		}
		if sendErr := p.sendError(envelope.ID, subprocess.NewRPCError(
			bridgeSDKRPCCodeInternal,
			"Internal error",
			map[string]string{peerErrorKey: err.Error()},
		)); sendErr != nil {
			p.failTransport(ctx, fmt.Errorf("bridgesdk: send internal error: %w", sendErr))
		}
		return
	}

	if sendErr := p.sendResult(envelope.ID, result); sendErr != nil {
		p.failTransport(ctx, fmt.Errorf("bridgesdk: send result: %w", sendErr))
	}
}

func (p *Peer) handleResponse(envelope rpcEnvelope) {
	idKey := rpcIDKey(envelope.ID)
	if idKey == "" {
		return
	}

	p.stateMu.Lock()
	responseCh, ok := p.pending[idKey]
	if ok {
		delete(p.pending, idKey)
	}
	p.stateMu.Unlock()
	if !ok {
		return
	}

	responseCh <- rpcResult{
		result: envelope.Result,
		err:    envelope.Error,
	}
	close(responseCh)
}

func (p *Peer) sendResult(id json.RawMessage, result any) error {
	payload, err := marshalParams(result)
	if err != nil {
		return fmt.Errorf("bridgesdk: encode rpc result: %w", err)
	}
	return p.writeFrame(rpcEnvelope{
		JSONRPC: bridgeSDKJSONRPCVersion,
		ID:      id,
		Result:  payload,
	})
}

func (p *Peer) sendError(id json.RawMessage, rpcErr *subprocess.RPCError) error {
	return p.writeFrame(rpcEnvelope{
		JSONRPC: bridgeSDKJSONRPCVersion,
		ID:      id,
		Error:   rpcErr,
	})
}

func (p *Peer) writeFrame(frame rpcEnvelope) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("bridgesdk: encode rpc frame: %w", err)
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	if written, err := p.stdout.Write(payload); err != nil {
		return fmt.Errorf("bridgesdk: write rpc frame: %w", err)
	} else if written != len(payload) {
		return fmt.Errorf("bridgesdk: write rpc frame: %w", io.ErrShortWrite)
	}
	if written, err := p.stdout.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("bridgesdk: write rpc frame newline: %w", err)
	} else if written != 1 {
		return fmt.Errorf("bridgesdk: write rpc frame newline: %w", io.ErrShortWrite)
	}
	return nil
}

func rpcIDKey(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(bytes.TrimSpace(raw)))
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return ""
		}
		return value
	}

	return trimmed
}

func marshalParams(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage("null"), nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func isNilPeerIO(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
