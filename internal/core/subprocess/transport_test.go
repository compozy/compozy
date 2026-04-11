package subprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/version"
)

func TestTransportRoundTripsRequestIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   json.RawMessage
	}{
		{name: "integer", id: json.RawMessage("1")},
		{name: "string", id: json.RawMessage(`"req-1"`)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer
			writer := NewTransport(strings.NewReader(""), &buffer)
			if err := writer.WriteMessage(Message{
				ID:     &tc.id,
				Method: "initialize",
				Params: json.RawMessage(`{"protocol_version":"1"}`),
			}); err != nil {
				t.Fatalf("write message: %v", err)
			}

			reader := NewTransport(strings.NewReader(buffer.String()), io.Discard)
			message, err := reader.ReadMessage()
			if err != nil {
				t.Fatalf("read message: %v", err)
			}
			if message.ID == nil || string(*message.ID) != string(tc.id) {
				t.Fatalf("unexpected message id: got %v want %s", message.ID, string(tc.id))
			}
			if message.Method != "initialize" {
				t.Fatalf("unexpected method: %q", message.Method)
			}
		})
	}
}

func TestTransportRejectsMessagesLargerThanTenMiB(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("a", MaxMessageSize+1)
	transport := NewTransport(strings.NewReader(oversized+"\n"), io.Discard)

	_, err := transport.ReadMessage()
	if err == nil {
		t.Fatal("expected oversized message error")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if requestErr.Code != -32603 {
		t.Fatalf("unexpected error code: %d", requestErr.Code)
	}
	data, ok := requestErr.Data.(map[string]any)
	if !ok || data["reason"] != "message_too_large" {
		t.Fatalf("unexpected error data: %#v", requestErr.Data)
	}
}

func TestTransportSkipsBlankLinesAndNeverWritesBlankLines(t *testing.T) {
	t.Parallel()

	transport := NewTransport(
		strings.NewReader("\n\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n\n"),
		io.Discard,
	)
	message, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if message.Method != "ping" {
		t.Fatalf("unexpected method: %q", message.Method)
	}

	var buffer bytes.Buffer
	writer := NewTransport(strings.NewReader(""), &buffer)
	id := json.RawMessage("1")
	if err := writer.WriteMessage(Message{
		ID:     &id,
		Method: "pong",
	}); err != nil {
		t.Fatalf("write message: %v", err)
	}

	encoded := buffer.String()
	if strings.Contains(encoded, "\n\n") {
		t.Fatalf("expected single-line encoding, got %q", encoded)
	}
	if trimmed := strings.TrimSpace(encoded); trimmed == "" {
		t.Fatal("expected encoded message content")
	}
}

func TestTransportParseAndErrorHelpers(t *testing.T) {
	t.Parallel()

	transport := NewTransport(strings.NewReader("{broken}\n"), io.Discard)
	_, err := transport.ReadMessage()
	if err == nil {
		t.Fatal("expected parse error")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if requestErr.Code != -32700 {
		t.Fatalf("unexpected parse error code: %d", requestErr.Code)
	}
	if got := requestErr.Error(); !strings.Contains(got, "Parse error") {
		t.Fatalf("unexpected parse error string: %q", got)
	}

	if got := NewMethodNotFound("ping").Error(); !strings.Contains(got, "Method not found") {
		t.Fatalf("unexpected method-not-found string: %q", got)
	}
	if got := NewInvalidRequest(map[string]any{"reason": "bad"}).Error(); !strings.Contains(got, "Invalid request") {
		t.Fatalf("unexpected invalid-request string: %q", got)
	}
}

func TestInitializeHappyPath(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	clientTransport := NewTransport(clientConn, clientConn)
	serverTransport := NewTransport(serverConn, serverConn)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		message, err := serverTransport.ReadMessage()
		if err != nil {
			t.Errorf("server read initialize: %v", err)
			return
		}

		var request InitializeRequest
		if err := json.Unmarshal(message.Params, &request); err != nil {
			t.Errorf("server decode initialize request: %v", err)
			return
		}
		if request.ProtocolVersion != version.ExtensionProtocolVersion {
			t.Errorf("unexpected protocol version: %q", request.ProtocolVersion)
			return
		}

		result, err := json.Marshal(InitializeResponse{
			ProtocolVersion:      version.ExtensionProtocolVersion,
			AcceptedCapabilities: []string{"events.read"},
		})
		if err != nil {
			t.Errorf("server marshal initialize response: %v", err)
			return
		}
		if err := serverTransport.WriteMessage(Message{
			ID:     message.ID,
			Result: result,
		}); err != nil {
			t.Errorf("server write initialize response: %v", err)
		}
	}()

	response, err := Initialize(context.Background(), clientTransport, nil, InitializeRequest{
		ProtocolVersion:           version.ExtensionProtocolVersion,
		SupportedProtocolVersions: []string{version.ExtensionProtocolVersion},
		GrantedCapabilities:       []string{"events.read", "tasks.read"},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if response.ProtocolVersion != version.ExtensionProtocolVersion {
		t.Fatalf("unexpected negotiated version: %q", response.ProtocolVersion)
	}

	<-serverDone
}

func TestInitializeRejectsUnsupportedProtocolVersion(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	clientTransport := NewTransport(clientConn, clientConn)
	serverTransport := NewTransport(serverConn, serverConn)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		message, err := serverTransport.ReadMessage()
		if err != nil {
			t.Errorf("server read initialize: %v", err)
			return
		}

		result, err := json.Marshal(InitializeResponse{ProtocolVersion: "99"})
		if err != nil {
			t.Errorf("server marshal initialize response: %v", err)
			return
		}
		if err := serverTransport.WriteMessage(Message{
			ID:     message.ID,
			Result: result,
		}); err != nil {
			t.Errorf("server write initialize response: %v", err)
		}
	}()

	_, err := Initialize(context.Background(), clientTransport, nil, InitializeRequest{
		ProtocolVersion:           version.ExtensionProtocolVersion,
		SupportedProtocolVersions: []string{version.ExtensionProtocolVersion},
	})
	if err == nil {
		t.Fatal("expected initialize version mismatch")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if requestErr.Code != -32602 {
		t.Fatalf("unexpected error code: %d", requestErr.Code)
	}
	data, ok := requestErr.Data.(map[string]any)
	if !ok || data["reason"] != "unsupported_protocol_version" {
		t.Fatalf("unexpected error data: %#v", requestErr.Data)
	}

	<-serverDone
}

func TestInitializeRejectsUnexpectedResponseID(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	clientTransport := NewTransport(clientConn, clientConn)
	serverTransport := NewTransport(serverConn, serverConn)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		if _, err := serverTransport.ReadMessage(); err != nil {
			t.Errorf("server read initialize: %v", err)
			return
		}
		badID := json.RawMessage("999")
		result, err := json.Marshal(InitializeResponse{ProtocolVersion: version.ExtensionProtocolVersion})
		if err != nil {
			t.Errorf("server marshal initialize response: %v", err)
			return
		}
		if err := serverTransport.WriteMessage(Message{
			ID:     &badID,
			Result: result,
		}); err != nil {
			t.Errorf("server write initialize response: %v", err)
		}
	}()

	_, err := Initialize(context.Background(), clientTransport, json.RawMessage("1"), InitializeRequest{
		ProtocolVersion:           version.ExtensionProtocolVersion,
		SupportedProtocolVersions: []string{version.ExtensionProtocolVersion},
	})
	if err == nil {
		t.Fatal("expected invalid initialize response id")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if requestErr.Code != -32600 {
		t.Fatalf("unexpected error code: %d", requestErr.Code)
	}

	<-serverDone
}

func TestValidateInitializeResponseRejectsUnsupportedCapabilities(t *testing.T) {
	t.Parallel()

	err := ValidateInitializeResponse(InitializeRequest{
		SupportedProtocolVersions: []string{version.ExtensionProtocolVersion},
		GrantedCapabilities:       []string{"events.read"},
	}, InitializeResponse{
		ProtocolVersion:      version.ExtensionProtocolVersion,
		AcceptedCapabilities: []string{"events.read", "tasks.read"},
	})
	if err == nil {
		t.Fatal("expected invalid accepted capabilities")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if requestErr.Code != -32602 {
		t.Fatalf("unexpected error code: %d", requestErr.Code)
	}
}
