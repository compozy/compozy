//go:build integration

package udsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gorilla/websocket"
)

func TestUDSTerminalWebSocketLifecycle(t *testing.T) {
	t.Run("Should complete a real terminal lifecycle over the Unix socket", func(t *testing.T) {
		t.Parallel()
		runtime := newIntegrationRuntime(t)
		workspaceBody, err := json.Marshal(contract.CreateWorkspaceRequest{RootDir: runtime.workspace})
		if err != nil {
			t.Fatalf("json.Marshal(create workspace) error = %v", err)
		}
		workspaceResponse := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPost,
			"http://unix/api/workspaces",
			workspaceBody,
			nil,
		)
		if workspaceResponse.StatusCode != http.StatusCreated {
			body := readAndCloseHTTPBody(t, workspaceResponse)
			t.Fatalf(
				"create workspace status = %d, want %d; body=%s",
				workspaceResponse.StatusCode,
				http.StatusCreated,
				body,
			)
		}
		var workspacePayload contract.WorkspaceResponse
		decodeHTTPJSON(t, workspaceResponse, &workspacePayload)
		workspaceID := workspacePayload.Workspace.ID
		if workspaceID == "" {
			t.Fatal("created workspace id is empty")
		}

		createResponse := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPost,
			"http://unix/api/workspaces/"+workspaceID+"/terminals",
			[]byte(`{"cols":80,"rows":24}`),
			nil,
		)
		if createResponse.StatusCode != http.StatusCreated {
			body := readAndCloseHTTPBody(t, createResponse)
			t.Fatalf(
				"create terminal status = %d, want %d; body=%s",
				createResponse.StatusCode,
				http.StatusCreated,
				body,
			)
		}
		var created contract.TerminalResponse
		decodeHTTPJSON(t, createResponse, &created)
		if created.Terminal.ID == "" {
			t.Fatal("created terminal id is empty")
		}
		if created.Terminal.WorkspaceID == "" || created.Terminal.ProfileID == "" {
			t.Fatalf(
				"created terminal scope = workspace %q, profile %q",
				created.Terminal.WorkspaceID,
				created.Terminal.ProfileID,
			)
		}
		terminalWorkspaceID := created.Terminal.WorkspaceID
		ticketResponse := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPost,
			"http://unix/api/workspaces/"+terminalWorkspaceID+"/terminals/"+
				string(created.Terminal.ID)+"/attach-ticket?profile=default",
			[]byte(`{"mode":"write"}`),
			nil,
		)
		if ticketResponse.StatusCode != http.StatusCreated {
			body := readAndCloseHTTPBody(t, ticketResponse)
			t.Fatalf("mint terminal ticket status = %d, want 201; body=%s", ticketResponse.StatusCode, body)
		}
		var ticket contract.TerminalAttachTicketResponse
		decodeHTTPJSON(t, ticketResponse, &ticket)
		if ticket.Ticket == "" {
			t.Fatal("terminal attach ticket is empty")
		}

		streamURL := "ws://unix/api/workspaces/" + terminalWorkspaceID + "/terminals/" +
			string(created.Terminal.ID) + "/stream?mode=write&flow=ack&ticket=" + ticket.Ticket
		dialer := websocket.Dialer{
			Subprotocols: []string{terminalwire.Subprotocol},
			NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var netDialer net.Dialer
				return netDialer.DialUnix(ctx, "unix", nil, &net.UnixAddr{Name: runtime.socket, Net: "unix"})
			},
		}
		dialContext, cancelDial := context.WithTimeout(t.Context(), 2*time.Second)
		connection, response, err := dialer.DialContext(dialContext, streamURL, nil)
		cancelDial()
		if err != nil {
			if response != nil && response.Body != nil {
				body := readAndCloseHTTPBody(t, response)
				t.Fatalf("DialContext(terminal stream) error = %v; body=%s", err, body)
			}
			t.Fatalf("DialContext(terminal stream) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				t.Errorf("terminal websocket Close() error = %v", closeErr)
			}
		})
		if connection.Subprotocol() != terminalwire.Subprotocol {
			t.Fatalf("terminal subprotocol = %q, want %q", connection.Subprotocol(), terminalwire.Subprotocol)
		}
		if frame := readUDSTerminalServerFrame(t, connection); frame.Op != terminalwire.ServerOpAttached {
			t.Fatalf("first terminal opcode = 0x%02x, want ATTACHED", frame.Op)
		}

		input, err := terminalwire.EncodeClient(terminalwire.Frame{
			Op: terminalwire.ClientOpInput, Payload: []byte("echo uds-wire-ok\n"),
		})
		if err != nil {
			t.Fatalf("terminalwire.EncodeClient(INPUT) error = %v", err)
		}
		if err := connection.WriteMessage(websocket.BinaryMessage, input); err != nil {
			t.Fatalf("WriteMessage(INPUT) error = %v", err)
		}
		foundEcho := false
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			frame := readUDSTerminalServerFrame(t, connection)
			if frame.Op == terminalwire.ServerOpOutput && bytes.Contains(frame.Payload, []byte("uds-wire-ok")) {
				foundEcho = true
				break
			}
		}
		if !foundEcho {
			t.Fatal("terminal stream did not echo input over the Unix socket")
		}

		closeResponse := mustUnixRequest(
			t,
			runtime.client,
			http.MethodDelete,
			"http://unix/api/workspaces/"+terminalWorkspaceID+"/terminals/"+string(
				created.Terminal.ID,
			)+"?profile=default",
			[]byte(`{"signal":"HUP"}`),
			nil,
		)
		if closeResponse.StatusCode != http.StatusOK {
			body := readAndCloseHTTPBody(t, closeResponse)
			t.Fatalf("close terminal status = %d, want %d; body=%s", closeResponse.StatusCode, http.StatusOK, body)
		}
		closeHTTPBody(t, closeResponse.Body)
		for {
			if frame := readUDSTerminalServerFrame(t, connection); frame.Op == terminalwire.ServerOpExit {
				break
			}
		}
	})
}

func readUDSTerminalServerFrame(t *testing.T, connection *websocket.Conn) terminalwire.Frame {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline(terminal frame) error = %v", err)
	}
	messageType, encoded, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(terminal frame) error = %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("terminal message type = %d, want binary", messageType)
	}
	frame, err := terminalwire.DecodeServer(encoded)
	if err != nil {
		t.Fatalf("terminalwire.DecodeServer() error = %v", err)
	}
	return frame
}
