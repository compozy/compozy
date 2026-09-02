package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestTerminalAgentHandlersShouldPreserveUntrustedAndRedactedContracts(t *testing.T) { // IT-025
	t.Run("Should preserve untrusted reads and redacted answers", func(t *testing.T) {
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
	})
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
