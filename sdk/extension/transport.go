package extension

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	initialBufferSize = 1024 * 1024
	// MaxMessageSize bounds one encoded JSON-RPC message.
	MaxMessageSize = 10 * 1024 * 1024
)

// Message is one line-delimited JSON-RPC envelope.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the public JSON-RPC error object returned by the extension or host.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error returns a stable human-readable message.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if len(bytes.TrimSpace(e.Data)) == 0 {
		return fmt.Sprintf("code %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("code %d: %s (%s)", e.Code, e.Message, string(e.Data))
}

// DecodeData unmarshals the structured error data into target.
func (e *Error) DecodeData(target any) error {
	if e == nil {
		return fmt.Errorf("decode rpc error data: missing error")
	}
	if len(bytes.TrimSpace(e.Data)) == 0 {
		return io.EOF
	}
	return json.Unmarshal(e.Data, target)
}

// Transport exchanges line-delimited JSON-RPC messages.
type Transport interface {
	ReadMessage() (Message, error)
	WriteMessage(Message) error
	Close() error
}

// StdIOTransport implements the line-delimited subprocess transport used by
// executable extensions.
type StdIOTransport struct {
	scanner *bufio.Scanner
	reader  io.Closer
	writer  io.Closer
	out     io.Writer
	mu      sync.Mutex
}

// NewStdIOTransport constructs a line-delimited transport over reader/writer.
func NewStdIOTransport(reader io.Reader, writer io.Writer) *StdIOTransport {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, initialBufferSize)
	scanner.Buffer(buffer, MaxMessageSize)

	transport := &StdIOTransport{
		scanner: scanner,
		out:     writer,
	}
	if closer, ok := reader.(io.Closer); ok {
		transport.reader = closer
	}
	if closer, ok := writer.(io.Closer); ok {
		transport.writer = closer
	}
	return transport
}

// ReadMessage reads the next non-empty message from the transport.
func (t *StdIOTransport) ReadMessage() (Message, error) {
	if t == nil || t.scanner == nil {
		return Message{}, fmt.Errorf("read transport message: missing scanner")
	}

	for t.scanner.Scan() {
		line := bytes.TrimSpace(t.scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var message Message
		if err := json.Unmarshal(line, &message); err != nil {
			return Message{}, newParseError(map[string]any{"error": err.Error()})
		}
		return message, nil
	}

	if err := t.scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return Message{}, newInternalError(map[string]any{"reason": "message_too_large"})
		}
		return Message{}, err
	}
	return Message{}, io.EOF
}

// WriteMessage writes one JSON-RPC message with a trailing newline.
func (t *StdIOTransport) WriteMessage(message Message) error {
	if t == nil || t.out == nil {
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
	_, err = t.out.Write(encoded)
	return err
}

// Close closes the underlying reader and writer when possible.
func (t *StdIOTransport) Close() error {
	if t == nil {
		return nil
	}

	var closeErr error
	if t.reader != nil {
		closeErr = errors.Join(closeErr, t.reader.Close())
	}
	if t.writer != nil && t.writer != t.reader {
		closeErr = errors.Join(closeErr, t.writer.Close())
	}
	return closeErr
}

func marshalJSON(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage("{}"), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func unmarshalJSON(raw json.RawMessage, target any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return json.Unmarshal(trimmed, target)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return ctx.Err()
}

func newParseError(data any) *Error {
	return newRPCError(-32700, "Parse error", data)
}

func newInvalidRequestError(data any) *Error {
	return newRPCError(-32600, "Invalid request", data)
}

func newMethodNotFoundError(method string) *Error {
	return newRPCError(-32601, "Method not found", map[string]any{"method": method})
}

func newInvalidParamsError(data any) *Error {
	return newRPCError(-32602, "Invalid params", data)
}

func newInternalError(data any) *Error {
	return newRPCError(-32603, "Internal error", data)
}

func newRPCError(code int, message string, data any) *Error {
	raw, err := marshalJSON(data)
	if err != nil {
		raw = nil
	}
	return &Error{Code: code, Message: message, Data: raw}
}
