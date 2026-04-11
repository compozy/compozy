package exttesting

import (
	"context"
	"errors"
	"io"
	"testing"

	extension "github.com/compozy/compozy/sdk/extension"
)

func TestMockTransportSendReceiveAndClose(t *testing.T) {
	t.Parallel()

	left, right := NewMockTransportPairWithBuffer(1)
	message := extension.Message{
		ID:     extension.MustJSON("1"),
		Method: "ping",
		Params: extension.MustJSON(map[string]string{"hello": "world"}),
	}

	if err := left.Send(context.Background(), message); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	received, err := right.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if received.Method != "ping" {
		t.Fatalf("received method = %q, want ping", received.Method)
	}

	if err := left.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := left.ReadMessage(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadMessage() after close error = %v, want io.EOF", err)
	}
}

func TestMockTransportCanceledContextAndNilReceiver(t *testing.T) {
	t.Parallel()

	left, _ := NewMockTransportPairWithBuffer(1)
	if err := left.WriteMessage(extension.Message{Method: "queued"}); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := left.Send(cancelCtx, extension.Message{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Send(canceled) error = %v, want context.Canceled", err)
	}

	_, right := NewMockTransportPair()
	if _, err := right.Receive(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Receive(canceled) error = %v, want context.Canceled", err)
	}

	var transport *MockTransport
	if err := transport.Close(); err != nil {
		t.Fatalf("Close(nil) error = %v, want nil", err)
	}
	if _, err := transport.ReadMessage(); err == nil {
		t.Fatal("ReadMessage(nil) error = nil, want error")
	}
	if err := transport.WriteMessage(extension.Message{}); err == nil {
		t.Fatal("WriteMessage(nil) error = nil, want error")
	}
}
