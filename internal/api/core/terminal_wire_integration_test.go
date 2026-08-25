package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestTerminalWireShouldCompleteRealLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("interactive PTY lifecycle requires the Unix adapter")
	}
	gin.SetMode(gin.TestMode)
	manager, err := terminalpkg.NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "httpapi", Terminal: manager})
	router := gin.New()
	router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/attach-ticket", handlers.MintTerminalAttachTicket)
	router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
	router.DELETE("/api/workspaces/:workspace_id/terminals/:id", handlers.DeleteTerminal)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	created := terminalTestJSONRequest(t, server.Client(), http.MethodPost,
		server.URL+"/api/workspaces/workspace-a/terminals", `{"shell":"sh","cols":80,"rows":24}`)
	var createResponse struct {
		Terminal struct {
			ID string `json:"id"`
		} `json:"terminal"`
	}
	if err := json.Unmarshal(created, &createResponse); err != nil {
		t.Fatalf("decode create response: %v; body=%s", err, created)
	}
	if createResponse.Terminal.ID == "" {
		t.Fatalf("create terminal id is empty; body=%s", created)
	}

	ticketBody := terminalTestJSONRequest(t, server.Client(), http.MethodPost,
		server.URL+"/api/workspaces/workspace-a/terminals/"+createResponse.Terminal.ID+"/attach-ticket",
		`{"mode":"write"}`)
	var ticketResponse struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(ticketBody, &ticketResponse); err != nil {
		t.Fatalf("decode ticket response: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/workspaces/workspace-a/terminals/" +
		createResponse.Terminal.ID + "/stream?mode=write&flow=ack&ticket=" + ticketResponse.Ticket
	dialer := websocket.Dialer{Subprotocols: []string{terminalwire.Subprotocol}}
	headers := http.Header{"Origin": []string{server.URL}}
	conn, response, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if response != nil && response.Body != nil {
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			t.Fatalf("Dial() error = %v, read error = %v, close error = %v, body=%s", err, readErr, closeErr, body)
		}
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("websocket Close() error = %v", err)
		}
	})
	if conn.Subprotocol() != terminalwire.Subprotocol {
		t.Fatalf("negotiated subprotocol = %q, want %q", conn.Subprotocol(), terminalwire.Subprotocol)
	}
	attached := terminalReadServerFrame(t, conn)
	if attached.Op != terminalwire.ServerOpAttached {
		t.Fatalf("first opcode = 0x%02x, want ATTACHED", attached.Op)
	}
	input, err := terminalwire.EncodeClient(terminalwire.Frame{Op: terminalwire.ClientOpInput, Payload: []byte("echo wire-ok\n")})
	if err != nil {
		t.Fatalf("EncodeClient(INPUT) error = %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("WriteMessage(INPUT) error = %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	foundEcho := false
	for time.Now().Before(deadline) {
		frame := terminalReadServerFrame(t, conn)
		if frame.Op == terminalwire.ServerOpOutput && bytes.Contains(frame.Payload, []byte("wire-ok")) {
			foundEcho = true
			break
		}
	}
	if !foundEcho {
		t.Fatal("terminal stream did not echo input")
	}
	resize, err := terminalwire.EncodeClient(terminalwire.Frame{Op: terminalwire.ClientOpResize, Payload: json.RawMessage(`{"cols":100,"rows":30}`)})
	if err != nil {
		t.Fatalf("EncodeClient(RESIZE) error = %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, resize); err != nil {
		t.Fatalf("WriteMessage(RESIZE) error = %v", err)
	}
	for {
		frame := terminalReadServerFrame(t, conn)
		if frame.Op == terminalwire.ServerOpResized {
			break
		}
	}
	terminalTestJSONRequest(t, server.Client(), http.MethodDelete,
		server.URL+"/api/workspaces/workspace-a/terminals/"+createResponse.Terminal.ID, `{"signal":"HUP"}`)
	for {
		frame := terminalReadServerFrame(t, conn)
		if frame.Op == terminalwire.ServerOpExit {
			break
		}
	}

	secondCreated := terminalTestJSONRequest(t, server.Client(), http.MethodPost,
		server.URL+"/api/workspaces/workspace-a/terminals", `{"shell":"sh","cols":80,"rows":24}`)
	if err := json.Unmarshal(secondCreated, &createResponse); err != nil {
		t.Fatalf("decode second create response: %v", err)
	}
	secondTicket := terminalTestJSONRequest(t, server.Client(), http.MethodPost,
		server.URL+"/api/workspaces/workspace-a/terminals/"+createResponse.Terminal.ID+"/attach-ticket",
		`{"mode":"read"}`)
	if err := json.Unmarshal(secondTicket, &ticketResponse); err != nil {
		t.Fatalf("decode second ticket response: %v", err)
	}
	secondURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/workspaces/workspace-a/terminals/" +
		createResponse.Terminal.ID + "/stream?mode=read&flow=drop&ticket=" + ticketResponse.Ticket
	secondConn, secondResponse, err := dialer.Dial(secondURL, headers)
	if err != nil {
		if secondResponse != nil && secondResponse.Body != nil {
			body, readErr := io.ReadAll(secondResponse.Body)
			closeErr := secondResponse.Body.Close()
			t.Fatalf("second Dial() error = %v, read error = %v, close error = %v, body=%s", err, readErr, closeErr, body)
		}
		t.Fatalf("second Dial() error = %v", err)
	}
	t.Cleanup(func() {
		if err := secondConn.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("second websocket Close() error = %v", err)
		}
	})
	if frame := terminalReadServerFrame(t, secondConn); frame.Op != terminalwire.ServerOpAttached {
		t.Fatalf("second first opcode = 0x%02x, want ATTACHED", frame.Op)
	}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- handlers.ShutdownTerminalStreams(shutdownCtx)
	}()
	if err := secondConn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("second SetReadDeadline() error = %v", err)
	}
	_, _, err = secondConn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
		t.Fatalf("ReadMessage(shutdown) error = %v, want GoingAway", err)
	}
	if closeErr, ok := err.(*websocket.CloseError); !ok || closeErr.Text != "daemon shutdown" {
		t.Fatalf("shutdown close = %#v, want daemon shutdown reason", err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("ShutdownTerminalStreams() error = %v", err)
	}
}

func terminalTestJSONRequest(t *testing.T, client *http.Client, method, target, body string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(%s) error = %v", method, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(%s) error = %v", method, err)
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read/close %s response errors = %v / %v", method, readErr, closeErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("%s %s status = %d, body=%s", method, target, response.StatusCode, payload)
	}
	return payload
}

func terminalReadServerFrame(t *testing.T, conn *websocket.Conn) terminalwire.Frame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	messageType, encoded, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want binary", messageType)
	}
	frame, err := terminalwire.DecodeServer(encoded)
	if err != nil {
		t.Fatalf("DecodeServer() error = %v, encoded=%s", err, fmt.Sprintf("%x", encoded))
	}
	return frame
}
