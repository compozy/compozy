package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
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
