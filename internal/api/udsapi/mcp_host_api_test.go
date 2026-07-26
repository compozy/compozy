package udsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/gin-gonic/gin"
)

func TestMCPHostAPIInvokeRoute(t *testing.T) {
	t.Parallel()

	t.Run("Should relay the raw Host API result over UDS HTTP", func(t *testing.T) {
		t.Parallel()

		invoker := &udsMCPHostAPIInvoker{result: json.RawMessage(`{"sessions":[]}`)}
		engine := gin.New()
		RegisterRoutes(engine, &Handlers{MCPHostAPI: invoker})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/internal/mcp/host-api/invoke",
			strings.NewReader(
				`{"serve_session_id":"serve-1","workspace":"alpha","method":"sessions/list","params":{}}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if got := response.Code; got != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", got, http.StatusOK, response.Body.String())
		}
		if got := strings.TrimSpace(response.Body.String()); got != `{"result":{"sessions":[]}}` {
			t.Fatalf("body = %s, want raw result envelope", got)
		}
		call := invoker.lastCall(t)
		if call.serveSessionID != "serve-1" || call.workspace != "alpha" ||
			call.method != "sessions/list" || string(call.params) != "{}" {
			t.Fatalf("InvokeHostAPI call = %#v, want request fields preserved", call)
		}
	})

	t.Run("Should relay the serve-session close lifecycle", func(t *testing.T) {
		t.Parallel()

		invoker := &udsMCPHostAPIInvoker{}
		engine := gin.New()
		RegisterRoutes(engine, &Handlers{MCPHostAPI: invoker})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/internal/mcp/host-api/session/close",
			strings.NewReader(`{"serve_session_id":"serve-1"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if got := response.Code; got != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", got, http.StatusNoContent, response.Body.String())
		}
		if got, want := invoker.lastClosedSession(t), "serve-1"; got != want {
			t.Fatalf("closed serve session = %q, want %q", got, want)
		}
	})

	t.Run("Should return status and body when the façade is unavailable", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		RegisterRoutes(engine, &Handlers{})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/internal/mcp/host-api/invoke",
			strings.NewReader(`{"workspace":"alpha","method":"sessions/list","params":{}}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if got := response.Code; got != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d; body=%s", got, http.StatusServiceUnavailable, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "unavailable") {
			t.Fatalf("body = %s, want unavailable diagnostic", response.Body.String())
		}
	})
}

type udsMCPHostAPIInvoker struct {
	mu     sync.Mutex
	result json.RawMessage
	calls  []udsMCPHostAPICall
	closed []string
}

type udsMCPHostAPICall struct {
	serveSessionID string
	workspace      string
	method         string
	params         json.RawMessage
}

func (i *udsMCPHostAPIInvoker) InvokeHostAPI(
	_ context.Context,
	serveSessionID string,
	workspace string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, udsMCPHostAPICall{
		serveSessionID: serveSessionID,
		workspace:      workspace,
		method:         method,
		params:         append(json.RawMessage(nil), params...),
	})
	return append(json.RawMessage(nil), i.result...), nil
}

func (i *udsMCPHostAPIInvoker) CloseHostAPISession(_ context.Context, serveSessionID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.closed = append(i.closed, serveSessionID)
	return nil
}

func (i *udsMCPHostAPIInvoker) lastClosedSession(t *testing.T) string {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.closed) == 0 {
		t.Fatal("CloseHostAPISession was not called")
	}
	return i.closed[len(i.closed)-1]
}

func (i *udsMCPHostAPIInvoker) lastCall(t *testing.T) udsMCPHostAPICall {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.calls) == 0 {
		t.Fatal("InvokeHostAPI was not called")
	}
	return i.calls[len(i.calls)-1]
}

var _ mcppkg.HostAPIInvoker = (*udsMCPHostAPIInvoker)(nil)
