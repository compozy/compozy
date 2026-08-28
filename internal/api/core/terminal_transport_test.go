package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestTerminalBrowserIdentityShouldBindAuthorizedClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should create a terminal under the authorized browser identity", func(t *testing.T) {
		t.Parallel()
		windowManager := &terminalWindowManagerServiceStub{}
		terminalManager := &terminalOpenManagerStub{}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi",
			Terminal:      &terminalProviderStub{Manager: terminalManager},
			WindowManager: &terminalWindowManagerProviderStub{service: windowManager},
		})
		router := gin.New()
		router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)

		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals?profile=default",
			strings.NewReader(`{"client_id":"client:web"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(terminalClientAttachmentHeader, "attachment-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf(
				"create status = %d, want 201; body=%s",
				response.Code,
				response.Body.String(),
			)
		}
		var payload contract.TerminalResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		if payload.Terminal.ID != "term-a" || terminalManager.request.Actor.ID != "client:web" {
			t.Fatalf(
				"create terminal/actor = %q/%q, want term-a/client:web",
				payload.Terminal.ID,
				terminalManager.request.Actor.ID,
			)
		}
		if windowManager.clientID != "client:web" || windowManager.token != "attachment-token" {
			t.Fatalf(
				"authorized browser identity = %q/%q, want client:web/attachment-token",
				windowManager.clientID,
				windowManager.token,
			)
		}
	})

	t.Run("Should mint a ticket under the authorized browser identity and reject a forgery", func(t *testing.T) {
		t.Parallel()
		manager := &terminalWindowManagerServiceStub{}
		var mintedActor terminalpkg.Actor
		terminalCore := terminalManagerStub{
			info:        &terminalpkg.Info{ID: "term-a", WS: "workspace-a", ProfileID: store.DefaultProfileID},
			mintedActor: &mintedActor,
		}
		provider := &terminalProviderStub{Manager: terminalCore}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi",
			Terminal:      provider,
			WindowManager: &terminalWindowManagerProviderStub{service: manager},
		})
		router := gin.New()
		router.POST("/api/workspaces/:workspace_id/terminals/:id/attach-ticket", handlers.MintTerminalAttachTicket)

		request := httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/attach-ticket?profile=default",
			strings.NewReader(`{"mode":"read","client_id":"client:web"}`))

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(terminalClientAttachmentHeader, "attachment-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", response.Code, response.Body.String())
		}
		var payload contract.TerminalAttachTicketResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode ticket response: %v", err)
		}
		if mintedActor.ID != "client:web" || manager.clientID != "client:web" ||
			manager.token != "attachment-token" {
			t.Fatalf(
				"browser identity = actor:%q authorized:%q token:%q",
				mintedActor.ID,
				manager.clientID,
				manager.token,
			)
		}

		manager.err = windowmanager.ErrClientUnauthorized
		denied := httptest.NewRecorder()
		request = httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/attach-ticket?profile=default",
			strings.NewReader(`{"mode":"read","client_id":"client:forged"}`))

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(terminalClientAttachmentHeader, "forged-token")
		router.ServeHTTP(denied, request)
		var generic contract.TerminalErrorResponse
		if err := json.Unmarshal(denied.Body.Bytes(), &generic); err != nil {
			t.Fatalf("decode browser identity refusal: %v", err)
		}
		if denied.Code != http.StatusForbidden || generic.Error.Code != terminalTransportForbidden ||
			strings.Contains(denied.Body.String(), "terminal_client_unauthorized") {
			t.Fatalf("denied status/body = %d/%s", denied.Code, denied.Body.String())
		}
	})

	t.Run("Should preserve truthful transport codes in the terminal envelope", func(t *testing.T) {
		t.Parallel()

		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "udsapi",
			Terminal:      &terminalProviderStub{Manager: terminalManagerStub{}},
		})
		router := gin.New()
		router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)
		router.GET("/unavailable", NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi"}).ListTerminals)
		router.GET("/internal", func(c *gin.Context) {
			handlers.respondTerminalMappedError(c, http.StatusInternalServerError, errors.New("backend failed"))
		})
		router.GET("/shutdown", func(c *gin.Context) {
			handlers.respondTerminalError(
				c,
				fmt.Errorf("terminal streams are shutting down: %w", terminalpkg.ErrShuttingDown),
			)
		})
		router.GET("/resolved", func(c *gin.Context) {
			handlers.respondTerminalError(
				c,
				fmt.Errorf("terminal input already rejected: %w", terminalpkg.ErrInputResolved),
			)
		})

		testCases := []struct {
			name       string
			method     string
			path       string
			body       string
			wantStatus int
			wantCode   string
		}{
			{
				name: "malformed body", method: http.MethodPost,
				path: "/api/workspaces/workspace-a/terminals", body: `{`,
				wantStatus: http.StatusBadRequest, wantCode: terminalTransportInvalidRequest,
			},
			{
				name: "absent service", method: http.MethodGet, path: "/unavailable",
				wantStatus: http.StatusServiceUnavailable, wantCode: terminalTransportServiceUnavailable,
			},
			{
				name: "internal failure", method: http.MethodGet, path: "/internal",
				wantStatus: http.StatusInternalServerError, wantCode: terminalTransportInternalError,
			},
			{
				name: "terminal shutdown", method: http.MethodGet, path: "/shutdown",
				wantStatus: http.StatusServiceUnavailable, wantCode: terminalTransportServiceUnavailable,
			},
			{
				name: "resolved input conflict", method: http.MethodGet, path: "/resolved",
				wantStatus: http.StatusConflict, wantCode: terminalTransportAPIError,
			},
		}
		for _, testCase := range testCases {
			t.Run("Should envelope "+testCase.name, func(t *testing.T) {
				request := httptest.NewRequestWithContext(
					t.Context(), testCase.method, testCase.path, strings.NewReader(testCase.body),
				)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				var payload contract.TerminalErrorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode terminal transport error: %v", err)
				}
				if response.Code != testCase.wantStatus || payload.Error.Code != testCase.wantCode {
					t.Fatalf(
						"status/error = %d/%#v, want %d/%q",
						response.Code,
						payload,
						testCase.wantStatus,
						testCase.wantCode,
					)
				}
			})
		}

		streamErr := (&terminalSocket{}).applyClientFrame(t.Context(), terminalwire.Frame{
			Op: terminalwire.ClientOpResize, Payload: []byte(`{`),
		})
		status, code, domain := terminalErrorStatusCode(streamErr)
		if status != http.StatusBadRequest || code != terminalTransportInvalidRequest || domain {
			t.Fatalf(
				"malformed WebSocket status/code/domain = %d/%q/%v, want 400/invalid_request/false",
				status,
				code,
				domain,
			)
		}
		status, code, domain = terminalErrorStatusCode(terminalReadOnlyOperationError("SIGNAL"))
		if status != http.StatusForbidden || code != terminalTransportForbidden || domain {
			t.Fatalf(
				"read-only WebSocket status/code/domain = %d/%q/%v, want 403/forbidden/false",
				status,
				code,
				domain,
			)
		}
	})

	t.Run("Should preserve frozen terminal domain codes and structured metadata", func(t *testing.T) {
		t.Parallel()

		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi"})
		router := gin.New()
		router.GET("/domain", func(c *gin.Context) {
			handlers.respondTerminalError(c, &terminalpkg.Error{
				Code: terminalpkg.ErrorCodeLimitReached, Message: "terminal limit reached",
				Current: 8, Max: 8, Err: terminalpkg.ErrLimitReached,
			})
		})
		response := httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/domain", http.NoBody),
		)
		var payload contract.TerminalErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode terminal domain error: %v", err)
		}
		if !contract.IsTerminalErrorCode(contract.TerminalErrorCode(payload.Error.Code)) ||
			payload.Error.Details == nil || payload.Error.Details.Current == nil ||
			*payload.Error.Details.Current != 8 {
			t.Fatalf("terminal domain error = %#v, want frozen code and current=8", payload)
		}
	})
}

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

func TestTerminalHandlersShouldKeepProfileScopesClosed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	manager := &scopeRecordingTerminalManager{}
	provider := &terminalProviderStub{Manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals", handlers.ListTerminals)
	router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)

	list := httptest.NewRecorder()
	router.ServeHTTP(
		list,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals?all_profiles=true",
			http.NoBody,
		),
	)
	if list.Code != http.StatusOK || !manager.scope.AllProfiles {
		t.Fatalf("aggregate list status/scope = %d/%#v", list.Code, manager.scope)
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(
		get,
		httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/term-a?all_profiles=true",
			http.NoBody),
	)
	if get.Code != http.StatusBadRequest {
		t.Fatalf("single-owner get status = %d, want 400; body=%s", get.Code, get.Body.String())
	}
	create := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(testutil.Context(t),
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals?all_profiles=true",
		strings.NewReader(`{}`))

	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	if create.Code != http.StatusBadRequest {
		t.Fatalf("mutation status = %d, want 400; body=%s", create.Code, create.Body.String())
	}
}

func TestTerminalDownloadsShouldStreamOnlyProfileScopedArtifacts(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	journal := &terminalDownloadJournalStub{}
	provider := &terminalProviderStub{Manager: terminalDownloadManagerStub{journal: journal}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/recordings/:id", handlers.DownloadTerminalRecording)
	router.GET("/api/workspaces/:workspace_id/terminals/artifacts/:id", handlers.DownloadTerminalArtifact)

	recording := httptest.NewRecorder()
	router.ServeHTTP(
		recording,
		httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/recordings/recording-a",
			http.NoBody),
	)
	if recording.Code != http.StatusOK || recording.Body.String() != "asciicast" {
		t.Fatalf("recording status/body = %d/%q, want 200/asciicast", recording.Code, recording.Body.String())
	}
	if got := recording.Header().Get("Content-Type"); got != "application/x-asciicast" {
		t.Fatalf("recording Content-Type = %q", got)
	}
	if journal.recordingScope.ProfileID != store.DefaultProfileID {
		t.Fatalf("recording scope = %#v, want default profile", journal.recordingScope)
	}

	artifact := httptest.NewRecorder()
	router.ServeHTTP(
		artifact,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/artifacts/artifact-a",
			http.NoBody,
		),
	)
	if artifact.Code != http.StatusOK || artifact.Body.String() != "artifact bytes" {
		t.Fatalf("artifact status/body = %d/%q, want 200/artifact bytes", artifact.Code, artifact.Body.String())
	}
	if journal.artifactScope.ProfileID != store.DefaultProfileID {
		t.Fatalf("artifact scope = %#v, want default profile", journal.artifactScope)
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(
		missing,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/recordings/foreign",
			http.NoBody,
		),
	)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"terminal_not_found"`) {
		t.Fatalf("foreign recording status/body = %d/%s, want typed 404", missing.Code, missing.Body.String())
	}
}

func TestTerminalAgentHandlersShouldPreserveUntrustedAndRedactedContracts(t *testing.T) { // IT-025
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handle := terminalHandleStub{
		screenResult: &terminalpkg.ReadResult{Content: "terminal bytes", Seq: 12, Untrusted: true},
		pending:      &terminalpkg.PendingInputRequest{ID: "input-a", Redacted: true},
		answer: &terminalpkg.InputOutcome{
			Outcome: "answered", Redacted: true, Length: 3, DeliveredBytes: 3,
		},
	}
	provider := &terminalProviderStub{Manager: terminalManagerStub{handle: handle}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/:id/read", handlers.ReadTerminal)
	router.POST(
		"/api/workspaces/:workspace_id/terminals/:id/input-requests/:request_id/answer",
		handlers.AnswerTerminalInputRequest,
	)

	read := httptest.NewRecorder()
	router.ServeHTTP(
		read,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/term-a/read?view=tail",
			http.NoBody,
		),
	)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"untrusted":true`) {
		t.Fatalf("read status/body = %d/%s", read.Code, read.Body.String())
	}

	answer := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(testutil.Context(t),
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals/term-a/input-requests/input-a/answer",
		strings.NewReader(`{"input":"secret"}`))

	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(answer, request)
	if answer.Code != http.StatusOK || !strings.Contains(answer.Body.String(), `"redacted":true`) ||
		!strings.Contains(answer.Body.String(), `"delivered_bytes":3`) {
		t.Fatalf("answer status/body = %d/%s", answer.Code, answer.Body.String())
	}
}

func TestTerminalAgentHandlersShouldExecuteEveryUnregisteredBody(t *testing.T) { // IT-009, IT-029, IT-034, IT-037
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handle := &terminalAgentHandleStub{terminalHandleStub: terminalHandleStub{
		pending: &terminalpkg.PendingInputRequest{ID: "input-a", Redacted: true},
	}}
	journal := &terminalAgentJournalStub{}
	manager := &terminalAgentManagerStub{terminalManagerStub: terminalManagerStub{}, handle: handle, journal: journal}
	provider := &terminalProviderStub{Manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.POST("/api/workspaces/:workspace_id/terminals/exec", handlers.ExecTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/wait", handlers.WaitTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/signal", handlers.SignalTerminal)
	router.GET("/api/workspaces/:workspace_id/terminal-input-requests", handlers.ListTerminalInputRequests)
	router.POST(
		"/api/workspaces/:workspace_id/terminals/:id/input-requests/:request_id/reject",
		handlers.RejectTerminalInputRequest,
	)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/recording", handlers.ControlTerminalRecording)
	router.GET("/api/workspaces/:workspace_id/terminal-journal", handlers.QueryTerminalJournal)

	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   []string
	}{
		{
			"exec",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/exec",
			`{"command":"server","yield_ms":250}`,
			http.StatusAccepted,
			[]string{
				`"exit_code":null`,
				`"signal":null`,
				`"output":""`,
				`"truncated":false`,
				`"untrusted":true`,
				`"duration_ms":0`,
				`"command_id":"cmd-a"`,
				`"still_running":true`,
				`"terminal_id":"term-a"`,
			},
		},
		{
			"wait",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/wait",
			`{"until":"match","pattern":"ok"}`,
			http.StatusOK,
			[]string{`"reason":"match"`, `"screen":"ok"`, `"untrusted":true`},
		},
		{
			"signal",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/signal",
			`{"signal":"TERM"}`,
			http.StatusOK,
			[]string{`"delivered":true`},
		},
		{
			"input requests",
			http.MethodGet,
			"/api/workspaces/workspace-a/terminal-input-requests?all_profiles=true",
			"",
			http.StatusOK,
			[]string{`"pending"`, `"resolved"`},
		},
		{
			"reject",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/input-requests/input-a/reject",
			`{"reason":"later"}`,
			http.StatusOK,
			[]string{`"outcome":"rejected"`},
		},
		{
			"record start",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/recording",
			`{"action":"start"}`,
			http.StatusOK,
			[]string{`"state":"recording"`},
		},
		{
			"record stop",
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/recording",
			`{"action":"stop"}`,
			http.StatusOK,
			[]string{`"state":"saved"`},
		},
		{
			"journal",
			http.MethodGet,
			"/api/workspaces/workspace-a/terminal-journal?all_profiles=true&limit=25",
			"",
			http.StatusOK,
			[]string{`"entries"`},
		},
	}
	for _, testCase := range testCases {
		t.Run("Should execute "+testCase.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(
				testutil.Context(t),
				testCase.method,
				testCase.path,
				strings.NewReader(testCase.body),
			)
			if testCase.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status/body = %d/%s, want %d", response.Code, response.Body.String(), testCase.wantStatus)
			}
			for _, expected := range testCase.wantBody {
				if !strings.Contains(response.Body.String(), expected) {
					t.Fatalf("body = %s, want field %s", response.Body.String(), expected)
				}
			}
		})
	}
	if manager.exec.Actor.Kind != terminalpkg.ActorKindHuman || handle.signal != terminalpkg.SignalTERM ||
		handle.rejected != "input-a" || handle.recordStarts != 1 || handle.recordStops != 1 {
		t.Fatalf("handler actor/effects = manager:%#v handle:%#v", manager, handle)
	}
	if !manager.inputScope.AllProfiles || !journal.scope.AllProfiles || journal.query.Limit != 25 {
		t.Fatalf(
			"aggregate scopes/query = input:%#v journal:%#v query:%#v",
			manager.inputScope,
			journal.scope,
			journal.query,
		)
	}
}

func TestTerminalHandlersShouldResolveSandboxCapabilities(t *testing.T) { // IT-016
	t.Parallel()
	gin.SetMode(gin.TestMode)
	manager := &terminalAgentManagerStub{
		terminalManagerStub: terminalManagerStub{},
		handle:              &terminalAgentHandleStub{},
		journal:             &terminalAgentJournalStub{},
	}
	provider := &terminalProviderStub{Manager: manager}
	workspaces := workspaceServiceStub{get: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
		return workspacepkg.Workspace{ID: ref, SandboxRef: "daytona"}, nil
	}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName: "udsapi",
		Terminal:      provider,
		Workspaces:    workspaces,
	})
	router := gin.New()
	router.POST("/api/workspaces/:workspace_id/terminals/exec", handlers.ExecTerminal)

	request := httptest.NewRequestWithContext(testutil.Context(t),
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals/exec",
		strings.NewReader(`{"command":"pwd"}`))

	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", response.Code, response.Body.String())
	}
	if manager.exec.Capabilities.Interactive {
		t.Fatalf("sandbox capabilities = %#v, want interactive disabled", manager.exec.Capabilities)
	}
}

func TestTerminalCatalogShouldReplayExactlyOnceAndResetOldCursors(t *testing.T) {
	t.Parallel()
	t.Run("Should project every named event from its carried fields", func(t *testing.T) {
		exitCode := 7
		exit := &terminalpkg.Exit{Cause: "exit", Code: &exitCode}
		openedInfo := &terminalpkg.Info{
			ID: "term-a", WS: "workspace-a", ProfileID: "profile-a", Title: "build",
			Mode: terminalpkg.ModePTY, Lease: terminalpkg.LeaseHumanOwned,
		}
		testCases := []struct {
			name        string
			event       terminalpkg.Event
			wantName    string
			wantPayload any
		}{
			{
				name: "created",
				event: terminalpkg.Event{
					Kind: terminalpkg.EventKindOpened, ProfileName: "Profile A", Info: openedInfo,
				},
				wantName: "terminal.created",
				wantPayload: gin.H{"terminal": terminalInfoPayload{
					ID: "term-a", WorkspaceID: "workspace-a", ProfileID: "profile-a", ProfileName: "Profile A",
					Title: "build", Mode: contract.TerminalMode(terminalpkg.ModePTY),
					Lease: contract.TerminalLeaseState(terminalpkg.LeaseHumanOwned),
				}},
			},
			{
				name: "closed", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindClosed, TerminalID: "term-a", Exit: exit,
				},
				wantName: "terminal.closed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "exit": &terminalExitPayload{
					Cause: "exit", Code: &exitCode,
				}},
			},
			{
				name: "title", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindTitleChanged, TerminalID: "term-a",
					Detail: &terminalpkg.EventDetail{Title: "tests"},
				},
				wantName:    "terminal.title_changed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "title": "tests"},
			},
			{
				name: "lease", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindLeaseChanged, TerminalID: "term-a",
					Actor: terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent, ID: "requester"}, Reason: "takeover",
					Detail: &terminalpkg.EventDetail{LeaseTo: terminalpkg.LeaseHumanOwned},
					Info: &terminalpkg.Info{Controller: &terminalpkg.Actor{
						Kind: terminalpkg.ActorKindHuman, ID: "operator",
					}},
				},
				wantName: "terminal.lease_changed",
				wantPayload: gin.H{
					"terminal_id": terminalpkg.ID("term-a"), "lease": terminalpkg.LeaseHumanOwned,
					"controller_kind": terminalpkg.ActorKindHuman, "controller_id": "operator", "reason": "takeover",
				},
			},
			{
				name: "mode", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindModeChanged, TerminalID: "term-a",
					Detail: &terminalpkg.EventDetail{Mode: terminalpkg.ModePipe},
				},
				wantName:    "terminal.mode_changed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "mode": terminalpkg.ModePipe},
			},
			{
				name: "recording started", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindRecordingStarted, WorkspaceID: "workspace-a", ProfileID: "profile-a",
					TerminalID: "term-a", At: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
					Detail: &terminalpkg.EventDetail{RecordingID: "rec-a"},
				},
				wantName: "terminal.recording_started",
				wantPayload: gin.H{
					"workspace_id": "workspace-a", "profile_id": "profile-a", "terminal_id": terminalpkg.ID("term-a"),
					"recording_id": "rec-a", "at": time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
				},
			},
			{
				name: "recording stopped", event: terminalpkg.Event{
					Kind: terminalpkg.EventKindRecordingStopped, WorkspaceID: "workspace-a", ProfileID: "profile-a",
					TerminalID: "term-a", At: time.Date(2026, 8, 28, 12, 5, 0, 0, time.UTC),
					Detail: &terminalpkg.EventDetail{RecordingID: "rec-a"},
				},
				wantName: "terminal.recording_stopped",
				wantPayload: gin.H{
					"workspace_id": "workspace-a", "profile_id": "profile-a", "terminal_id": terminalpkg.ID("term-a"),
					"recording_id": "rec-a", "at": time.Date(2026, 8, 28, 12, 5, 0, 0, time.UTC),
				},
			},
		}
		for _, testCase := range testCases {
			t.Run("Should project "+testCase.name, func(t *testing.T) {
				name, payload, err := terminalCatalogPayload(testCase.event)
				if err != nil {
					t.Fatalf("terminalCatalogPayload() error = %v", err)
				}
				if name != testCase.wantName || !reflect.DeepEqual(payload, testCase.wantPayload) {
					t.Fatalf("projection = %q/%#v, want %q/%#v", name, payload, testCase.wantName, testCase.wantPayload)
				}
			})
		}
	})

	t.Run("Should synchronize active recordings after fresh subscribe and reconnect", func(t *testing.T) {
		startedAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
		for _, testCase := range []struct {
			name          string
			reconnect     bool
			replayedStart bool
		}{
			{name: "fresh subscribe"},
			{name: "reconnect", reconnect: true},
			{name: "reconnect with retained recording start", reconnect: true, replayedStart: true},
		} {
			t.Run("Should rehydrate on "+testCase.name, func(t *testing.T) {
				requestContext, cancelRequest := context.WithCancel(t.Context())
				t.Cleanup(cancelRequest)
				query := &terminalRecordingQuery{cancel: cancelRequest}
				manager := terminalManagerStub{
					infos: []terminalpkg.Info{{
						ID: "term-a", WS: "workspace-a", ProfileID: store.DefaultProfileID,
					}},
					activeRecordings: []terminalpkg.RecordingRef{{
						ID: "rec-a", TerminalID: "term-a", ProfileID: store.DefaultProfileID, StartedAt: startedAt,
					}},
					activeRecordingQuery: query,
				}
				provider := &terminalProviderStub{Manager: manager}
				handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
				if testCase.reconnect {
					provider.emit(terminalpkg.Event{
						Kind: terminalpkg.EventKindTitleChanged, WorkspaceID: "workspace-a",
						ProfileID: store.DefaultProfileID, TerminalID: "term-a",
						Detail: &terminalpkg.EventDetail{Title: "build"},
					})
					if testCase.replayedStart {
						provider.emit(terminalpkg.Event{
							Kind: terminalpkg.EventKindRecordingStarted, WorkspaceID: "workspace-a",
							ProfileID: store.DefaultProfileID, TerminalID: "term-a", At: startedAt,
							Detail: &terminalpkg.EventDetail{RecordingID: "rec-a"},
						})
					}
				}
				router := gin.New()
				router.GET("/api/workspaces/:workspace_id/terminals/stream", handlers.StreamTerminalCatalog)
				request := httptest.NewRequestWithContext(
					requestContext, http.MethodGet,
					"/api/workspaces/workspace-a/terminals/stream?profile=default", http.NoBody,
				)
				if testCase.reconnect {
					request.Header.Set("Last-Event-ID", "1")
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				body := response.Body.String()
				if response.Code != http.StatusOK {
					t.Fatalf("catalog stream status/body = %d/%s, want 200", response.Code, body)
				}
				if testCase.reconnect && strings.Contains(body, "event: terminal.snapshot") {
					t.Fatalf("reconnect stream unexpectedly reset: %q", body)
				}
				if count := strings.Count(body, "event: terminal.recording_started"); count != 1 {
					t.Fatalf("recording start event count = %d, want exactly one; body=%q", count, body)
				}
				if !testCase.reconnect {
					snapshotIndex := strings.Index(body, "event: terminal.snapshot")
					recordingIndex := strings.Index(body, "event: terminal.recording_started")
					if snapshotIndex < 0 || recordingIndex <= snapshotIndex {
						t.Fatalf("fresh stream order = %q, want snapshot before recording state", body)
					}
				}
				for _, expected := range []string{
					"event: terminal.recording_started",
					`"workspace_id":"workspace-a"`,
					`"profile_id":"` + store.DefaultProfileID + `"`,
					`"terminal_id":"term-a"`,
					`"recording_id":"rec-a"`,
					`"at":"2026-08-28T12:00:00Z"`,
				} {
					if !strings.Contains(body, expected) {
						t.Fatalf("recording state frame = %q, want %q", body, expected)
					}
				}
				if query.workspaceID != "workspace-a" || query.scope.ProfileID != store.DefaultProfileID ||
					query.scope.AllProfiles {
					t.Fatalf("recording state scope = %#v, want workspace-a/default", query)
				}
			})
		}
	})

	provider := &terminalProviderStub{}
	catalog := newTerminalCatalog(provider)
	replay, reset, fence, changed := catalog.read("workspace-a", "profile-a", 0)
	if len(replay) != 0 || !reset || fence != 0 {
		t.Fatalf("initial subscribe replay/reset/fence = %#v/%v/%d", replay, reset, fence)
	}
	provider.emit(terminalpkg.Event{
		Kind: terminalpkg.EventKindOpened, WorkspaceID: "workspace-a", ProfileID: "profile-a",
		TerminalID: "term-a", Info: &terminalpkg.Info{ID: "term-a"},
	})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("catalog observer did not signal a retained event")
	}
	replay, reset, fence, changed = catalog.read("workspace-a", "profile-a", 1)
	if len(replay) != 0 || reset || fence != 1 {
		t.Fatalf("current cursor replay/reset/fence = %#v/%v/%d", replay, reset, fence)
	}
	provider.emit(terminalpkg.Event{
		Kind: terminalpkg.EventKindTitleChanged, WorkspaceID: "workspace-a", ProfileID: "profile-a",
		TerminalID: "term-a", Detail: &terminalpkg.EventDetail{Title: "build"},
	})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("catalog observer did not signal the title event")
	}
	replay, reset, fence, changed = catalog.read("workspace-a", "profile-a", 1)
	if changed == nil || reset || fence != 2 || len(replay) != 1 || replay[0].Sequence != 2 ||
		replay[0].Event.Kind != terminalpkg.EventKindTitleChanged {
		t.Fatalf("replay = %#v, reset=%v fence=%d", replay, reset, fence)
	}
	provider.emit(terminalpkg.Event{
		Kind: terminalpkg.EventKindRecordingStarted, WorkspaceID: "workspace-a", ProfileID: "profile-a",
		TerminalID: "term-a", Detail: &terminalpkg.EventDetail{RecordingID: "rec-a"},
	})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("catalog observer did not signal the recording event")
	}
	replay, reset, fence, changed = catalog.read("workspace-a", "profile-a", 2)
	if changed == nil || reset || fence != 3 || len(replay) != 1 || replay[0].Sequence != 3 ||
		replay[0].Event.Kind != terminalpkg.EventKindRecordingStarted {
		t.Fatalf("recording replay = %#v, reset=%v fence=%d", replay, reset, fence)
	}
	for range terminalCatalogRetention + 1 {
		provider.emit(terminalpkg.Event{
			Kind: terminalpkg.EventKindLeaseChanged, WorkspaceID: "workspace-a", ProfileID: "profile-a",
			TerminalID: "term-a", Detail: &terminalpkg.EventDetail{LeaseTo: terminalpkg.LeaseAvailable},
		})
	}
	replay, reset, fence, changed = catalog.read("workspace-a", "profile-a", 1)
	if len(replay) != 0 || !reset || fence <= terminalCatalogRetention || changed == nil {
		t.Fatalf("stale cursor replay/reset/fence/changed = %#v/%v/%d/%v", replay, reset, fence, changed)
	}
}

type terminalProviderStub struct {
	terminalpkg.Manager
	observers []func(context.Context, terminalpkg.Event)
}

type terminalWindowManagerProviderStub struct {
	WindowManagerProvider
	service WindowManagerService
}

func (p *terminalWindowManagerProviderStub) WindowManagerFor(string) (WindowManagerService, error) {
	return p.service, nil
}

type terminalWindowManagerServiceStub struct {
	WindowManagerService
	clientID string
	token    string
	err      error
}

func (s *terminalWindowManagerServiceStub) AuthorizeClient(
	_ context.Context,
	_ windowmanager.WorkspaceID,
	clientID windowmanager.ClientID,
	token string,
) error {
	s.clientID = string(clientID)
	s.token = token
	return s.err
}

func (p *terminalProviderStub) Observe(observer func(context.Context, terminalpkg.Event)) {
	p.observers = append(p.observers, observer)
}

func (p *terminalProviderStub) emit(event terminalpkg.Event) {
	for _, observer := range p.observers {
		observer(context.Background(), event)
	}
}

type terminalManagerStub struct {
	handle               terminalpkg.Handle
	info                 *terminalpkg.Info
	infos                []terminalpkg.Info
	mintErr              error
	mintedActor          *terminalpkg.Actor
	activeRecordings     []terminalpkg.RecordingRef
	activeRecordingQuery *terminalRecordingQuery
}

type terminalRecordingQuery struct {
	workspaceID string
	scope       store.ReadScope
	cancel      context.CancelFunc
}

type terminalOpenManagerStub struct {
	terminalManagerStub
	request terminalpkg.OpenRequest
}

func (m *terminalOpenManagerStub) Open(
	_ context.Context,
	request terminalpkg.OpenRequest,
) (terminalpkg.Handle, error) {
	m.request = request
	return terminalHandleStub{info: terminalpkg.Info{
		ID: "term-a", WS: request.WS, ProfileID: request.Actor.ProfileID,
	}}, nil
}

type terminalAgentManagerStub struct {
	terminalManagerStub
	handle     *terminalAgentHandleStub
	journal    *terminalAgentJournalStub
	exec       terminalpkg.ExecRequest
	inputScope store.ReadScope
}

func (m *terminalAgentManagerStub) Exec(
	_ context.Context,
	request terminalpkg.ExecRequest,
) (*terminalpkg.ExecResult, error) {
	m.exec = request
	id := terminalpkg.ID("term-a")
	return &terminalpkg.ExecResult{StillRunning: true, TerminalID: &id, CommandID: "cmd-a", Untrusted: true}, nil
}

func (m *terminalAgentManagerStub) Handle(context.Context, string, string, terminalpkg.ID) (terminalpkg.Handle, error) {
	return m.handle, nil
}

func (m *terminalAgentManagerStub) InputRequests(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	_ terminalpkg.ID,
) ([]terminalpkg.PendingInputRequest, error) {
	m.inputScope = scope
	return []terminalpkg.PendingInputRequest{{ID: "input-a"}}, nil
}

func (m *terminalAgentManagerStub) Journal() terminalpkg.Journal { return m.journal }

type terminalAgentJournalStub struct {
	scope store.ReadScope
	query terminalpkg.Query
}

func (*terminalAgentJournalStub) ReserveCommandID(context.Context, string) (string, error) {
	return "cmd-0000000000000001", nil
}

func (*terminalAgentJournalStub) ReleaseCommandID(string, string) {}

func (*terminalAgentJournalStub) ReserveRecordingID(context.Context, string) (string, error) {
	return "rec-0000000000000001", nil
}

func (*terminalAgentJournalStub) ReleaseRecordingID(string, string) {}

type terminalDownloadManagerStub struct {
	terminalManagerStub
	journal terminalpkg.Journal
}

func (m terminalDownloadManagerStub) Journal() terminalpkg.Journal { return m.journal }

type terminalDownloadJournalStub struct {
	terminalAgentJournalStub
	recordingScope store.ReadScope
	artifactScope  store.ReadScope
}

func (j *terminalDownloadJournalStub) Recording(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	id string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	j.recordingScope = scope
	if id == "foreign" {
		return nil, nil, os.ErrNotExist
	}
	recording := &terminalpkg.RecordingRef{
		ID:    id,
		Bytes: int64(len("asciicast")),
	}
	reader := io.NopCloser(strings.NewReader("asciicast"))
	return recording, reader, nil
}

func (j *terminalDownloadJournalStub) Artifact(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	_ string,
) (io.ReadCloser, error) {
	j.artifactScope = scope
	return io.NopCloser(strings.NewReader("artifact bytes")), nil
}

func (*terminalAgentJournalStub) Record(context.Context, string, terminalpkg.CommandRow) error {
	return nil
}
func (j *terminalAgentJournalStub) RecordQueued(
	ctx context.Context,
	info terminalpkg.Info,
	row terminalpkg.CommandRow,
) error {
	return j.Record(ctx, info.WS, row)
}
func (j *terminalAgentJournalStub) Query(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	query terminalpkg.Query,
) (*terminalpkg.Page, error) {
	j.scope = scope
	j.query = query
	return &terminalpkg.Page{Entries: []terminalpkg.CommandRow{}}, nil
}

func (*terminalAgentJournalStub) LinkRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
) error {
	return nil
}
func (*terminalAgentJournalStub) Recording(
	context.Context,
	string,
	store.ReadScope,
	string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	return nil, nil, terminalpkg.ErrUnsupported
}
func (*terminalAgentJournalStub) Artifact(context.Context, string, store.ReadScope, string) (io.ReadCloser, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (*terminalAgentJournalStub) RemoveWorkspace(context.Context, string) error { return nil }
func (*terminalAgentJournalStub) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}
func (*terminalAgentJournalStub) PrepareWorkspaceRemovalAt(
	context.Context,
	string,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}
func (*terminalAgentJournalStub) ConsumeMarkerFacts(
	context.Context,
	terminalpkg.Info,
	[]terminalpkg.MarkerFacts,
) error {
	return nil
}
func (*terminalAgentJournalStub) RegisterTerminal(
	terminalpkg.Info,
	func(bool),
	func(terminalpkg.Event),
) {
}
func (*terminalAgentJournalStub) CloseTerminal(context.Context, terminalpkg.Info) error { return nil }
func (*terminalAgentJournalStub) ReserveInput(
	terminalpkg.Info,
	terminalpkg.JournalInput,
) (terminalpkg.JournalInputReservation, bool) {
	return terminalAgentJournalReservation{}, true
}
func (*terminalAgentJournalStub) ObserveOutput(terminalpkg.Info, []byte) {}
func (*terminalAgentJournalStub) Shutdown(context.Context) error         { return nil }
func (*terminalAgentJournalStub) PersistRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
	[]byte,
) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}
func (*terminalAgentJournalStub) WriteArtifact(
	context.Context,
	string,
	string,
	string,
	*terminalpkg.ID,
	[]byte,
	time.Time,
) (terminalpkg.SpillRef, error) {
	return terminalpkg.SpillRef{}, nil
}

type terminalAgentJournalReservation struct{}

func (terminalAgentJournalReservation) Commit(terminalpkg.Actor, terminalpkg.JournalInput) {}
func (terminalAgentJournalReservation) Release()                                           {}

type scopeRecordingTerminalManager struct {
	terminalManagerStub
	scope store.ReadScope
}

func (m *scopeRecordingTerminalManager) List(
	_ context.Context,
	_ string,
	scope store.ReadScope,
) ([]terminalpkg.Info, error) {
	m.scope = scope
	return []terminalpkg.Info{}, nil
}

func (terminalManagerStub) Open(context.Context, terminalpkg.OpenRequest) (terminalpkg.Handle, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (terminalManagerStub) Exec(context.Context, terminalpkg.ExecRequest) (*terminalpkg.ExecResult, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (m terminalManagerStub) Handle(context.Context, string, string, terminalpkg.ID) (terminalpkg.Handle, error) {
	return m.handle, nil
}
func (m terminalManagerStub) Get(context.Context, string, string, terminalpkg.ID) (*terminalpkg.Info, error) {
	if m.info != nil {
		return m.info, nil
	}
	return &terminalpkg.Info{}, nil
}
func (m terminalManagerStub) List(context.Context, string, store.ReadScope) ([]terminalpkg.Info, error) {
	return append([]terminalpkg.Info(nil), m.infos...), nil
}

func (m terminalManagerStub) ActiveRecordings(
	_ context.Context,
	workspaceID string,
	scope store.ReadScope,
) ([]terminalpkg.RecordingRef, error) {
	if m.activeRecordingQuery != nil {
		m.activeRecordingQuery.workspaceID = workspaceID
		m.activeRecordingQuery.scope = scope
		if m.activeRecordingQuery.cancel != nil {
			m.activeRecordingQuery.cancel()
		}
	}
	return append([]terminalpkg.RecordingRef(nil), m.activeRecordings...), nil
}

func (terminalManagerStub) Capabilities(context.Context, string) (terminalpkg.Capabilities, error) {
	return terminalpkg.Capabilities{Interactive: true}, nil
}

func (m terminalManagerStub) MintAttachTicket(
	_ context.Context,
	binding terminalpkg.AttachTicketBinding,
	actor terminalpkg.Actor,
) (terminalpkg.AttachTicket, error) {
	if m.mintErr != nil {
		return terminalpkg.AttachTicket{}, m.mintErr
	}
	if m.mintedActor != nil {
		*m.mintedActor = actor
	}
	return terminalpkg.AttachTicket{
		Token: "tkt-test", Binding: binding, Actor: actor, ExpiresAt: time.Now().Add(time.Minute),
	}, nil
}

func (m terminalManagerStub) AttachWithTicket(
	ctx context.Context,
	token string,
	workspaceID string,
	terminalID terminalpkg.ID,
	mode string,
	options terminalpkg.AttachOptions,
) (terminalpkg.Handle, terminalpkg.Subscription, terminalpkg.AttachTicket, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, terminalpkg.AttachTicket{}, &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeTicketInvalid, Message: "terminal attach ticket is invalid",
			Err: terminalpkg.ErrTicketInvalid,
		}
	}
	if m.handle == nil {
		return nil, nil, terminalpkg.AttachTicket{}, terminalpkg.ErrUnsupported
	}
	subscription, err := m.handle.Attach(ctx, options)
	if err != nil {
		return nil, nil, terminalpkg.AttachTicket{}, err
	}
	ticket := terminalpkg.AttachTicket{Token: token, Binding: terminalpkg.AttachTicketBinding{
		WorkspaceID: workspaceID, TerminalID: terminalID, Mode: mode,
	}}
	return m.handle, subscription, ticket, nil
}

func (terminalManagerStub) Claim(context.Context, string, terminalpkg.ID, terminalpkg.Actor) error {
	return nil
}

func (terminalManagerStub) RunEnded(context.Context, string, terminalpkg.Actor) int { return 0 }

func (terminalManagerStub) SessionRunEnded(context.Context, string, string, string, string, int64) int {
	return 0
}

func (terminalManagerStub) RuntimeRecovered(context.Context, string, terminalpkg.Actor, terminalpkg.Actor) int {
	return 0
}

func (terminalManagerStub) InputRequests(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.ID,
) ([]terminalpkg.PendingInputRequest, error) {
	return nil, nil
}

func (terminalManagerStub) ResolvedInputRequests(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.ID,
) ([]terminalpkg.ResolvedInputRequest, error) {
	return nil, nil
}

func (terminalManagerStub) Close(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.Actor,
	terminalpkg.Signal,
) (*terminalpkg.Exit, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (terminalManagerStub) Journal() terminalpkg.Journal                     { return nil }
func (terminalManagerStub) Shutdown(context.Context) error                   { return nil }
func (terminalManagerStub) Observe(func(context.Context, terminalpkg.Event)) {}
func (terminalManagerStub) ArchiveProfile(context.Context, string) error     { return nil }
func (terminalManagerStub) ArchiveWorkspace(context.Context, string) error   { return nil }
func (terminalManagerStub) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}

type terminalAgentWorkspaceRemovalPreparation struct{}

func (terminalAgentWorkspaceRemovalPreparation) BeforeDelete(context.Context) error { return nil }
func (terminalAgentWorkspaceRemovalPreparation) Commit(context.Context) error       { return nil }
func (terminalAgentWorkspaceRemovalPreparation) Rollback(context.Context) error     { return nil }

type terminalHandleStub struct {
	info         terminalpkg.Info
	attachErr    error
	writeErr     error
	subscription terminalpkg.Subscription
	screenResult *terminalpkg.ReadResult
	screenErr    error
	pending      *terminalpkg.PendingInputRequest
	answer       *terminalpkg.InputOutcome
}

type terminalAgentHandleStub struct {
	terminalHandleStub
	signal       terminalpkg.Signal
	rejected     terminalpkg.InputRequestID
	recordStarts int
	recordStops  int
}

func (*terminalAgentHandleStub) Wait(context.Context, terminalpkg.WaitCondition) (*terminalpkg.WaitResult, error) {
	return &terminalpkg.WaitResult{Reason: "match", Screen: "ok", Untrusted: true}, nil
}
func (h *terminalAgentHandleStub) Signal(_ context.Context, _ terminalpkg.Actor, signal terminalpkg.Signal) error {
	h.signal = signal
	return nil
}
func (h *terminalAgentHandleStub) RejectInput(
	_ context.Context,
	_ terminalpkg.Actor,
	id terminalpkg.InputRequestID,
	_ string,
) error {
	h.rejected = id
	return nil
}
func (h *terminalAgentHandleStub) StartRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	h.recordStarts++
	return terminalpkg.RecordingRef{ID: "recording-a"}, nil
}
func (h *terminalAgentHandleStub) StopRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	h.recordStops++
	return terminalpkg.RecordingRef{ID: "recording-a"}, nil
}

func (h terminalHandleStub) Info() terminalpkg.Info { return h.info }
func (terminalHandleStub) MarkerNonce() string      { return "" }
func (h terminalHandleStub) Attach(context.Context, terminalpkg.AttachOptions) (terminalpkg.Subscription, error) {
	return h.subscription, h.attachErr
}
func (h terminalHandleStub) Write(context.Context, terminalpkg.Actor, []byte) error {
	return h.writeErr
}
func (h terminalHandleStub) Screen(context.Context, terminalpkg.ReadOptions) (*terminalpkg.ReadResult, error) {
	return h.screenResult, h.screenErr
}
func (terminalHandleStub) Wait(context.Context, terminalpkg.WaitCondition) (*terminalpkg.WaitResult, error) {
	return nil, nil
}
func (terminalHandleStub) Takeover(context.Context, terminalpkg.Actor, bool) error { return nil }
func (terminalHandleStub) Yield(context.Context, terminalpkg.Actor) error          { return nil }
func (terminalHandleStub) RequestInput(context.Context, terminalpkg.InputRequest) (*terminalpkg.InputOutcome, error) {
	return nil, nil
}

func (h terminalHandleStub) AnswerInput(
	context.Context,
	terminalpkg.Actor,
	terminalpkg.InputRequestID,
	terminalpkg.InputAnswer,
) (*terminalpkg.InputOutcome, error) {
	if h.answer != nil {
		return h.answer, nil
	}
	return &terminalpkg.InputOutcome{Outcome: "answered"}, nil
}
func (terminalHandleStub) RejectInput(context.Context, terminalpkg.Actor, terminalpkg.InputRequestID, string) error {
	return nil
}
func (h terminalHandleStub) PendingInput(terminalpkg.InputRequestID) (*terminalpkg.PendingInputRequest, error) {
	return h.pending, nil
}
func (terminalHandleStub) Signal(context.Context, terminalpkg.Actor, terminalpkg.Signal) error {
	return nil
}
func (terminalHandleStub) StartRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}
func (terminalHandleStub) StopRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}

type terminalSubscriptionStub struct {
	frames chan terminalpkg.Frame
	closed bool
}

func (s *terminalSubscriptionStub) Frames() <-chan terminalpkg.Frame { return s.frames }
func (*terminalSubscriptionStub) Err() error                         { return nil }
func (*terminalSubscriptionStub) Ack(int)                            {}
func (*terminalSubscriptionStub) Resize(uint16, uint16) error        { return nil }
func (s *terminalSubscriptionStub) Close() error {
	s.closed = true
	return nil
}

func terminalReadTransportFrame(t *testing.T, connection *websocket.Conn) terminalwire.Frame {
	t.Helper()
	messageType, encoded, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want binary", messageType)
	}
	frame, err := terminalwire.DecodeServer(encoded)
	if err != nil {
		t.Fatalf("DecodeServer() error = %v", err)
	}
	return frame
}
