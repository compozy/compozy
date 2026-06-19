package subprocess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

const (
	initialBufferSize = 1024 * 1024
	// MaxMessageSize bounds the encoded JSON-RPC message size accepted by the transport.
	MaxMessageSize = 10 * 1024 * 1024
)

// Message is a line-framed JSON-RPC 2.0 envelope.
type Message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RequestError    `json:"error,omitempty"`
}

// RequestError represents a JSON-RPC error object.
type RequestError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error returns a stable human-readable JSON-RPC error string.
func (e *RequestError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Data == nil {
		return fmt.Sprintf("code %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("code %d: %s (data: %v)", e.Code, e.Message, e.Data)
}

// Transport reads and writes line-delimited JSON-RPC messages.
type Transport struct {
	scanner   *bufio.Scanner
	reader    io.Reader
	writer    io.Writer
	mu        sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// NewTransport constructs a line-delimited JSON-RPC transport.
func NewTransport(reader io.Reader, writer io.Writer) *Transport {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, initialBufferSize)
	scanner.Buffer(buffer, MaxMessageSize)

	return &Transport{
		scanner: scanner,
		reader:  reader,
		writer:  writer,
	}
}

// ReadMessage reads the next non-empty JSON-RPC message from the transport.
func (t *Transport) ReadMessage() (Message, error) {
	if t == nil || t.scanner == nil {
		return Message{}, fmt.Errorf("read transport message: missing scanner")
	}

	for t.scanner.Scan() {
		line := t.scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var message Message
		if err := json.Unmarshal(line, &message); err != nil {
			return Message{}, NewParseError(map[string]any{"error": err.Error()})
		}
		return message, nil
	}

	if err := t.scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return Message{}, NewInternalError(map[string]any{"reason": "message_too_large"})
		}
		return Message{}, err
	}

	return Message{}, io.EOF
}

// WriteMessage writes one JSON-RPC message and appends a single trailing newline.
func (t *Transport) WriteMessage(message Message) error {
	if t == nil || t.writer == nil {
		return fmt.Errorf("write transport message: missing writer")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	message.JSONRPC = "2.0"
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = t.writer.Write(encoded)
	return err
}

// Close releases transport endpoints that expose close semantics.
func (t *Transport) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		t.closeErr = errors.Join(
			closeTransportEndpoint(t.reader),
			closeTransportEndpoint(t.writer),
		)
	})
	return t.closeErr
}

func closeTransportEndpoint(endpoint any) error {
	closer, ok := endpoint.(io.Closer)
	if !ok || closer == nil {
		return nil
	}
	if err := closer.Close(); err != nil {
		if errors.Is(err, os.ErrClosed) ||
			errors.Is(err, io.ErrClosedPipe) ||
			errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	return nil
}

// NewParseError creates a JSON-RPC parse error.
func NewParseError(data any) *RequestError {
	return &RequestError{Code: -32700, Message: "Parse error", Data: data}
}

// NewInvalidRequest creates a JSON-RPC invalid request error.
func NewInvalidRequest(data any) *RequestError {
	return &RequestError{Code: -32600, Message: "Invalid request", Data: data}
}

// NewMethodNotFound creates a JSON-RPC method-not-found error.
func NewMethodNotFound(method string) *RequestError {
	return &RequestError{Code: -32601, Message: "Method not found", Data: map[string]any{"method": method}}
}

// NewInvalidParams creates a JSON-RPC invalid params error.
func NewInvalidParams(data any) *RequestError {
	return &RequestError{Code: -32602, Message: "Invalid params", Data: data}
}

// NewInternalError creates a JSON-RPC internal error.
func NewInternalError(data any) *RequestError {
	return &RequestError{Code: -32603, Message: "Internal error", Data: data}
}
