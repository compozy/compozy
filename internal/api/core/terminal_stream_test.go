package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestTerminalStreamShouldHardenOriginHostAndUpgradeCap(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("Should reject missing TCP origin and spoofed host but allow UDS", func(t *testing.T) {
		t.Parallel()
		httpHandlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi", Config: compozyconfig.Config{HTTP: compozyconfig.HTTPConfig{Host: "localhost"}},
		})
		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"http://localhost/stream",
			http.NoBody,
		)
		if httpHandlers.terminalOriginAllowed(request) {
			t.Fatal("TCP request without Origin was accepted")
		}
		request.Header.Set("Origin", "http://localhost")
		if !httpHandlers.terminalOriginAllowed(request) || !httpHandlers.terminalHostAllowed(request) {
			t.Fatal("same-host TCP request was rejected")
		}
		request.Header.Set("Origin", "http://localhost:9999")
		if httpHandlers.terminalOriginAllowed(request) {
			t.Fatal("cross-port TCP origin was accepted")
		}
		request.Header.Set("Origin", "http://localhost")
		if terminalProtocolAllowed(request) {
			t.Fatal("request without the frozen terminal subprotocol was accepted")
		}
		request.Header.Set("Sec-WebSocket-Protocol", "compozy.terminal.v1")
		if terminalProtocolAllowed(request) {
			t.Fatal("request with the retired terminal v1 subprotocol was accepted")
		}
		request.Header.Set("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		if !terminalProtocolAllowed(request) {
			t.Fatal("request with the frozen terminal subprotocol was rejected")
		}
		request.Host = "attacker.example"
		if httpHandlers.terminalHostAllowed(request) {
			t.Fatal("spoofed Host was accepted")
		}
		udsHandlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi"})
		udsRequest := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"http://localhost/stream",
			http.NoBody,
		)
		if !udsHandlers.terminalOriginAllowed(udsRequest) {
			t.Fatal("UDS request without Origin was rejected")
		}
	})

	t.Run("Should enforce the subscriber cap after a stale ticket was minted", func(t *testing.T) {
		provider := &terminalProviderStub{Manager: terminalManagerStub{
			handle: terminalHandleStub{attachErr: &terminalpkg.Error{
				Code:    "subscriber_limit_reached",
				Message: "terminal subscriber limit reached",
				Err:     terminalpkg.ErrSubscriberLimit,
			}},
		}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi", Terminal: provider,
			Config: compozyconfig.Config{HTTP: compozyconfig.HTTPConfig{Host: "localhost"}},
		})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
		request := httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodGet,
			"http://localhost/api/workspaces/workspace-a/terminals/term-a/stream?mode=read&ticket=tkt-stale",
			http.NoBody)

		request.Header.Set("Origin", "http://localhost")
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
		}
	})

	t.Run("Should require a ticket on UDS instead of trusting the local transport", func(t *testing.T) {
		provider := &terminalProviderStub{Manager: terminalManagerStub{}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/term-a/stream?mode=read",
			http.NoBody,
		)
		request.Header.Set("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("UDS stream without ticket status = %d, want 403; body=%s", response.Code, response.Body.String())
		}
		var payload contract.TerminalErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode UDS ticket refusal: %v", err)
		}
		if payload.Error.Code != "ticket_invalid" {
			t.Fatalf("UDS ticket refusal = %#v, want ticket_invalid", payload)
		}
	})

	t.Run("Should reject input and signal frames on read attachments", func(t *testing.T) {
		socket := &terminalSocket{mode: "read", handle: terminalHandleStub{}}
		for _, frame := range []terminalwire.Frame{
			{Op: terminalwire.ClientOpInput, Payload: []byte("hidden")},
			{Op: terminalwire.ClientOpSignal, Payload: []byte(`{"signal":"TERM"}`)},
		} {
			err := socket.applyClientFrame(t.Context(), frame)
			if !errors.Is(err, terminalpkg.ErrWriteAttachmentRequired) {
				t.Fatalf("applyClientFrame(%d) error = %v, want read-only refusal", frame.Op, err)
			}
			status, code, domain := terminalErrorStatusCode(err)
			if status != http.StatusForbidden || code != terminalTransportForbidden || domain {
				t.Fatalf("read-only frame status/code = %d/%q", status, code)
			}
		}
	})

	t.Run("Should keep the watcher attached after a recoverable lease denial", func(t *testing.T) {
		subscription := &terminalSubscriptionStub{frames: make(chan terminalpkg.Frame, 1)}
		provider := &terminalProviderStub{Manager: terminalManagerStub{
			handle: terminalHandleStub{
				subscription: subscription,
				writeErr: &terminalpkg.Error{
					Code: terminalpkg.ErrorCodeLeaseRevoked, Message: "terminal lease was revoked",
					Err: terminalpkg.ErrLeaseRevoked,
				},
			},
		}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		dialer := websocket.Dialer{Subprotocols: []string{terminalwire.Subprotocol}}
		connection, response, err := dialer.DialContext(
			t.Context(),
			"ws"+strings.TrimPrefix(server.URL, "http")+
				"/api/workspaces/workspace-a/terminals/term-a/stream?mode=write&ticket=tkt-valid",
			nil,
		)
		if err != nil {
			if response != nil && response.Body != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("close failed WebSocket response: %v", closeErr)
				}
			}
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("websocket Close() error = %v", err)
			}
		})
		encoded, err := terminalwire.EncodeClient(terminalwire.Frame{
			Op: terminalwire.ClientOpInput, Payload: []byte("denied"),
		})
		if err != nil {
			t.Fatalf("EncodeClient() error = %v", err)
		}
		if err := connection.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
		errorFrame := terminalReadTransportFrame(t, connection)
		if errorFrame.Op != terminalwire.ServerOpError {
			t.Fatalf("denial opcode = %d, want ERROR", errorFrame.Op)
		}
		var payload contract.TerminalErrorResponse
		if err := json.Unmarshal(errorFrame.Payload, &payload); err != nil {
			t.Fatalf("decode denial payload: %v", err)
		}
		if payload.Error.Code != string(terminalpkg.ErrorCodeLeaseRevoked) {
			t.Fatalf("denial payload = %#v, want lease_revoked", payload)
		}
		subscription.frames <- terminalpkg.Frame{
			Op: terminalwire.ServerOpOutput, Seq: 9, Payload: []byte("still watching"),
		}
		output := terminalReadTransportFrame(t, connection)
		if output.Op != terminalwire.ServerOpOutput || output.Seq != 9 || string(output.Payload) != "still watching" {
			t.Fatalf("post-denial output = %#v", output)
		}
	})

	t.Run("Should classify audit and generation denials as recoverable operations", func(t *testing.T) {
		t.Parallel()
		for _, testCase := range []struct {
			name string
			err  error
		}{
			{name: "journal unavailable", err: terminalpkg.ErrJournalUnavailable},
			{name: "generation fenced", err: terminalpkg.ErrGenerationFenced},
		} {
			t.Run("Should retain the watcher after "+testCase.name, func(t *testing.T) {
				t.Parallel()
				wrapped := fmt.Errorf("client operation denied: %w", testCase.err)
				if !terminalRecoverableClientOperationError(wrapped) {
					t.Fatalf("terminalRecoverableClientOperationError(%v) = false, want true", wrapped)
				}
			})
		}
	})

	t.Run("Should close the subscription when the WebSocket upgrade fails", func(t *testing.T) {
		subscription := &terminalSubscriptionStub{frames: make(chan terminalpkg.Frame)}
		provider := &terminalProviderStub{Manager: terminalManagerStub{
			handle: terminalHandleStub{subscription: subscription},
		}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		request, err := http.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			server.URL+"/api/workspaces/workspace-a/terminals/term-a/stream?mode=read&ticket=tkt-valid",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Version", "13")
		request.Header.Set("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
		if !subscription.closed {
			t.Fatal("subscription remained open after WebSocket upgrade failure")
		}
	})

	t.Run("Should reject ticket mint advisorily without reserving capacity", func(t *testing.T) {
		provider := &terminalProviderStub{Manager: terminalManagerStub{
			mintErr: &terminalpkg.Error{
				Code: terminalpkg.ErrorCodeSubscriberLimitReached, Message: "terminal subscriber limit reached",
				Current: 1, Max: 1, Err: terminalpkg.ErrSubscriberLimit,
			},
		}}
		config := compozyconfig.Config{}
		config.Terminal.MaxSubscribers = 1
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider, Config: config})
		router := gin.New()
		router.POST("/api/workspaces/:workspace_id/terminals/:id/attach-ticket", handlers.MintTerminalAttachTicket)
		request := httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/attach-ticket",
			strings.NewReader(`{"mode":"read"}`))

		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("mint status = %d, want 409; body=%s", response.Code, response.Body.String())
		}
		var payload contract.TerminalErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode subscriber limit response: %v", err)
		}
		if payload.Error.Code != "subscriber_limit_reached" ||
			payload.Error.Message != "terminal subscriber limit reached" ||
			payload.Error.Details == nil || payload.Error.Details.Current == nil ||
			payload.Error.Details.Max == nil || *payload.Error.Details.Current != 1 ||
			*payload.Error.Details.Max != 1 {
			t.Fatalf("subscriber limit payload = %#v", payload)
		}
	})
}
