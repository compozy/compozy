package exttesting

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/compozy/compozy/sdk/extension"
)

// MockTransport is an in-memory message transport used by SDK tests.
type MockTransport struct {
	incoming   <-chan extension.Message
	outgoing   chan<- extension.Message
	localDone  chan struct{}
	remoteDone <-chan struct{}
	closeOnce  sync.Once
}

var _ extension.Transport = (*MockTransport)(nil)

// NewMockTransportPair creates a connected in-memory transport pair.
func NewMockTransportPair() (*MockTransport, *MockTransport) {
	return NewMockTransportPairWithBuffer(32)
}

// NewMockTransportPairWithBuffer creates a connected in-memory transport pair
// with the provided per-link buffer.
func NewMockTransportPairWithBuffer(buffer int) (*MockTransport, *MockTransport) {
	if buffer <= 0 {
		buffer = 1
	}

	leftToRight := make(chan extension.Message, buffer)
	rightToLeft := make(chan extension.Message, buffer)
	leftDone := make(chan struct{})
	rightDone := make(chan struct{})

	left := &MockTransport{
		incoming:   rightToLeft,
		outgoing:   leftToRight,
		localDone:  leftDone,
		remoteDone: rightDone,
	}
	right := &MockTransport{
		incoming:   leftToRight,
		outgoing:   rightToLeft,
		localDone:  rightDone,
		remoteDone: leftDone,
	}
	return left, right
}

// ReadMessage reads one queued message.
func (m *MockTransport) ReadMessage() (extension.Message, error) {
	if m == nil {
		return extension.Message{}, fmt.Errorf("read mock transport message: missing transport")
	}

	select {
	case <-m.localDone:
		return extension.Message{}, io.EOF
	case message := <-m.incoming:
		return message, nil
	}
}

// Receive reads one queued message with context cancellation.
func (m *MockTransport) Receive(ctx context.Context) (extension.Message, error) {
	if ctx == nil {
		return m.ReadMessage()
	}

	select {
	case <-ctx.Done():
		return extension.Message{}, context.Cause(ctx)
	case <-m.localDone:
		return extension.Message{}, io.EOF
	case message := <-m.incoming:
		return message, nil
	}
}

// WriteMessage enqueues one message for the peer transport.
func (m *MockTransport) WriteMessage(message extension.Message) error {
	if m == nil {
		return fmt.Errorf("write mock transport message: missing transport")
	}

	select {
	case <-m.localDone:
		return io.EOF
	case <-m.remoteDone:
		return io.EOF
	case m.outgoing <- message:
		return nil
	}
}

// Send enqueues one message with context cancellation.
func (m *MockTransport) Send(ctx context.Context, message extension.Message) error {
	if ctx == nil {
		return m.WriteMessage(message)
	}

	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-m.localDone:
		return io.EOF
	case <-m.remoteDone:
		return io.EOF
	case m.outgoing <- message:
		return nil
	}
}

// Close closes this transport endpoint.
func (m *MockTransport) Close() error {
	if m == nil {
		return nil
	}

	m.closeOnce.Do(func() {
		close(m.localDone)
	})
	return nil
}
