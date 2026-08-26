package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestTerminalTicketStoreShouldEnforceSingleUseBindingTTLAndCapacity(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0).UTC()
	provider := &terminalProviderStub{}
	store := newTerminalTicketStore(provider, func() time.Time { return now })
	binding := terminalTicketBinding{
		WorkspaceID: "workspace-a", ProfileID: "profile-a", TerminalID: "term-a", Mode: "read",
	}
	actor := terminalpkg.Actor{Kind: terminalpkg.ActorKindHuman, ID: "operator", ProfileID: "profile-a"}

	t.Run("Should consume exactly once and reject a foreign binding", func(t *testing.T) {
		if _, err := store.Consume("tkt-not-hex", binding); !errors.Is(err, errTerminalTicketInvalid) {
			t.Fatalf("Consume(malformed) error = %v", err)
		}
		ticket, err := store.Mint(binding, actor)
		if err != nil {
			t.Fatalf("Mint() error = %v", err)
		}
		wrong := binding
		wrong.ProfileID = "profile-b"
		if _, err := store.Consume(ticket.Token, wrong); !errors.Is(err, errTerminalTicketInvalid) {
			t.Fatalf("Consume(wrong) error = %v", err)
		}
		if _, err := store.Consume(ticket.Token, binding); !errors.Is(err, errTerminalTicketInvalid) {
			t.Fatalf("Consume(reused) error = %v", err)
		}
	})

	t.Run("Should reject expiry", func(t *testing.T) {
		ticket, err := store.Mint(binding, actor)
		if err != nil {
			t.Fatalf("Mint() error = %v", err)
		}
		now = now.Add(terminalTicketTTL)
		if _, err := store.Consume(ticket.Token, binding); !errors.Is(err, errTerminalTicketExpired) {
			t.Fatalf("Consume(expired) error = %v", err)
		}
	})

	t.Run("Should evict the oldest ticket at capacity", func(t *testing.T) {
		var oldest terminalTicket
		for index := 0; index <= terminalTicketCapacity; index++ {
			ticket, err := store.Mint(binding, actor)
			if err != nil {
				t.Fatalf("Mint(%d) error = %v", index, err)
			}
			if index == 0 {
				oldest = ticket
			}
		}
		if _, err := store.Consume(oldest.Token, binding); !errors.Is(err, errTerminalTicketInvalid) {
			t.Fatalf("Consume(oldest) error = %v", err)
		}
	})
}

func TestTerminalTicketStoreShouldInvalidateOnTerminalEnd(t *testing.T) {
	t.Parallel()
	provider := &terminalProviderStub{}
	store := newTerminalTicketStore(provider, time.Now)
	for index, reason := range []string{"operator_close", "workspace_deleted", "profile_archived"} {
		binding := terminalTicketBinding{
			WorkspaceID: "workspace-a", ProfileID: "profile-a",
			TerminalID: terminalpkg.ID(fmt.Sprintf("term-%d", index)), Mode: "write",
		}
		tickets := make([]terminalTicket, 0, 2)
		for _, mode := range []string{"read", "write"} {
			candidate := binding
			candidate.Mode = mode
			ticket, err := store.Mint(candidate, terminalpkg.Actor{ProfileID: "profile-a"})
			if err != nil {
				t.Fatalf("Mint(%s) error = %v", reason, err)
			}
			tickets = append(tickets, ticket)
		}
		provider.emit(terminalpkg.TerminalEvent{
			Kind: terminalpkg.EventKindClosed, WorkspaceID: binding.WorkspaceID,
			ProfileID: binding.ProfileID, TerminalID: binding.TerminalID, Reason: reason,
		})
		for _, ticket := range tickets {
			if _, err := store.Consume(ticket.Token, ticket.Binding); !errors.Is(err, errTerminalTicketInvalid) {
				t.Fatalf("Consume(after %s) error = %v", reason, err)
			}
		}
	}
}

func TestTerminalStreamShouldHardenOriginHostAndUpgradeCap(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("Should reject missing TCP origin and spoofed host but allow UDS", func(t *testing.T) {
		t.Parallel()
		httpHandlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi", Config: compozyconfig.Config{HTTP: compozyconfig.HTTPConfig{Host: "localhost"}},
		})
		request := httptest.NewRequest(http.MethodGet, "http://localhost/stream", nil)
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
		if !terminalProtocolAllowed(request) {
			t.Fatal("request with the frozen terminal subprotocol was rejected")
		}
		request.Host = "attacker.example"
		if httpHandlers.terminalHostAllowed(request) {
			t.Fatal("spoofed Host was accepted")
		}
		udsHandlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi"})
		udsRequest := httptest.NewRequest(http.MethodGet, "http://localhost/stream", nil)
		if !udsHandlers.terminalOriginAllowed(udsRequest) {
			t.Fatal("UDS request without Origin was rejected")
		}
	})

	t.Run("Should enforce the subscriber cap after a stale ticket was minted", func(t *testing.T) {
		provider := &terminalProviderStub{manager: terminalManagerStub{
			handle: terminalHandleStub{attachErr: &terminalpkg.Error{
				Code: "subscriber_limit_reached", Message: "terminal subscriber limit reached", Err: terminalpkg.ErrSubscriberLimit,
			}},
		}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "httpapi", Terminal: provider,
			Config: compozyconfig.Config{HTTP: compozyconfig.HTTPConfig{Host: "localhost"}},
		})
		binding := terminalTicketBinding{
			WorkspaceID: "workspace-a", ProfileID: "profile-a", TerminalID: "term-a", Mode: "read",
		}
		ticket, err := handlers.terminalTickets.Mint(binding, terminalpkg.Actor{ProfileID: "profile-a"})
		if err != nil {
			t.Fatalf("Mint() error = %v", err)
		}
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id/stream", handlers.StreamTerminal)
		request := httptest.NewRequest(
			http.MethodGet,
			"http://localhost/api/workspaces/workspace-a/terminals/term-a/stream?mode=read&ticket="+ticket.Token,
			nil,
		)
		request.Header.Set("Origin", "http://localhost")
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Sec-WebSocket-Protocol", "compozy.terminal.v1")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
		}
	})

	t.Run("Should reject ticket mint advisorily without reserving capacity", func(t *testing.T) {
		provider := &terminalProviderStub{manager: terminalManagerStub{
			info: &terminalpkg.Info{ID: "term-a", WS: "workspace-a", ProfileID: "default", Viewers: 1},
		}}
		config := compozyconfig.Config{}
		config.Terminal.MaxSubscribers = 1
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider, Config: config})
		router := gin.New()
		router.POST("/api/workspaces/:workspace_id/terminals/:id/attach-ticket", handlers.MintTerminalAttachTicket)
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/workspaces/workspace-a/terminals/term-a/attach-ticket",
			strings.NewReader(`{"mode":"read"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusConflict || len(handlers.terminalTickets.tickets) != 0 {
			t.Fatalf("mint status/tickets = %d/%d, want 409/0; body=%s", response.Code, len(handlers.terminalTickets.tickets), response.Body.String())
		}
	})
}

func TestTerminalHandlersShouldKeepProfileScopesClosed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	manager := &scopeRecordingTerminalManager{}
	provider := &terminalProviderStub{manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals", handlers.ListTerminals)
	router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/workspaces/workspace-a/terminals?all_profiles=true", nil))
	if list.Code != http.StatusOK || !manager.scope.AllProfiles {
		t.Fatalf("aggregate list status/scope = %d/%#v", list.Code, manager.scope)
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/workspaces/workspace-a/terminals/term-a?all_profiles=true", nil))
	if get.Code != http.StatusBadRequest {
		t.Fatalf("single-owner get status = %d, want 400; body=%s", get.Code, get.Body.String())
	}
	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/workspace-a/terminals?all_profiles=true", strings.NewReader(`{}`))
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
	provider := &terminalProviderStub{manager: terminalDownloadManagerStub{journal: journal}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/recordings/:id", handlers.DownloadTerminalRecording)
	router.GET("/api/workspaces/:workspace_id/terminals/artifacts/:id", handlers.DownloadTerminalArtifact)

	recording := httptest.NewRecorder()
	router.ServeHTTP(recording, httptest.NewRequest(
		http.MethodGet, "/api/workspaces/workspace-a/terminals/recordings/recording-a", nil,
	))
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
	router.ServeHTTP(artifact, httptest.NewRequest(
		http.MethodGet, "/api/workspaces/workspace-a/terminals/artifacts/artifact-a", nil,
	))
	if artifact.Code != http.StatusOK || artifact.Body.String() != "artifact bytes" {
		t.Fatalf("artifact status/body = %d/%q, want 200/artifact bytes", artifact.Code, artifact.Body.String())
	}
	if journal.artifactScope.ProfileID != store.DefaultProfileID {
		t.Fatalf("artifact scope = %#v, want default profile", journal.artifactScope)
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(
		http.MethodGet, "/api/workspaces/workspace-a/terminals/recordings/foreign", nil,
	))
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
		answer:       &terminalpkg.InputOutcome{Outcome: "answered", Redacted: true, Length: 3},
	}
	provider := &terminalProviderStub{manager: terminalManagerStub{handle: handle}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/:id/read", handlers.ReadTerminal)
	router.POST(
		"/api/workspaces/:workspace_id/terminals/:id/input-requests/:request_id/answer",
		handlers.AnswerTerminalInputRequest,
	)

	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(
		http.MethodGet, "/api/workspaces/workspace-a/terminals/term-a/read?view=tail", nil,
	))
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"untrusted":true`) {
		t.Fatalf("read status/body = %d/%s", read.Code, read.Body.String())
	}

	answer := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals/term-a/input-requests/input-a/answer",
		strings.NewReader(`{"input":"secret"}`),
	)
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
	provider := &terminalProviderStub{manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.POST("/api/workspaces/:workspace_id/terminals/exec", handlers.ExecTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/wait", handlers.WaitTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/signal", handlers.SignalTerminal)
	router.GET("/api/workspaces/:workspace_id/terminal-input-requests", handlers.ListTerminalInputRequests)
	router.POST("/api/workspaces/:workspace_id/terminals/:id/input-requests/:request_id/reject", handlers.RejectTerminalInputRequest)
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
		{"exec", http.MethodPost, "/api/workspaces/workspace-a/terminals/exec", `{"command":"server","yield_ms":250}`, http.StatusAccepted, []string{`"exit_code":null`, `"signal":null`, `"output":""`, `"truncated":false`, `"untrusted":true`, `"duration_ms":0`, `"command_id":"cmd-a"`, `"still_running":true`, `"terminal_id":"term-a"`}},
		{"wait", http.MethodPost, "/api/workspaces/workspace-a/terminals/term-a/wait", `{"until":"match","pattern":"ok"}`, http.StatusOK, []string{`"reason":"match"`, `"screen":"ok"`, `"untrusted":true`}},
		{"signal", http.MethodPost, "/api/workspaces/workspace-a/terminals/term-a/signal", `{"signal":"TERM"}`, http.StatusOK, []string{`"delivered":true`}},
		{"input requests", http.MethodGet, "/api/workspaces/workspace-a/terminal-input-requests?all_profiles=true", "", http.StatusOK, []string{`"requests"`}},
		{"reject", http.MethodPost, "/api/workspaces/workspace-a/terminals/term-a/input-requests/input-a/reject", `{"reason":"later"}`, http.StatusOK, []string{`"outcome":"rejected"`}},
		{"record start", http.MethodPost, "/api/workspaces/workspace-a/terminals/term-a/recording", `{"action":"start"}`, http.StatusOK, []string{`"state":"recording"`}},
		{"record stop", http.MethodPost, "/api/workspaces/workspace-a/terminals/term-a/recording", `{"action":"stop"}`, http.StatusOK, []string{`"state":"saved"`}},
		{"journal", http.MethodGet, "/api/workspaces/workspace-a/terminal-journal?all_profiles=true&limit=25", "", http.StatusOK, []string{`"entries"`}},
	}
	for _, testCase := range testCases {
		t.Run("Should execute "+testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
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
		t.Fatalf("aggregate scopes/query = input:%#v journal:%#v query:%#v", manager.inputScope, journal.scope, journal.query)
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
	provider := &terminalProviderStub{manager: manager}
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

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals/exec",
		strings.NewReader(`{"command":"pwd"}`),
	)
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
			event       terminalpkg.TerminalEvent
			wantName    string
			wantPayload any
		}{
			{
				name: "created",
				event: terminalpkg.TerminalEvent{
					Kind: terminalpkg.EventKindOpened, ProfileName: "Profile A", Info: openedInfo,
				},
				wantName: "terminal.created",
				wantPayload: gin.H{"terminal": terminalInfoPayload{
					ID: "term-a", WorkspaceID: "workspace-a", ProfileID: "profile-a", ProfileName: "Profile A",
					Title: "build", Mode: terminalpkg.ModePTY, Lease: terminalpkg.LeaseHumanOwned,
				}},
			},
			{
				name: "closed", event: terminalpkg.TerminalEvent{
					Kind: terminalpkg.EventKindClosed, TerminalID: "term-a", Exit: exit,
				},
				wantName: "terminal.closed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "exit": &terminalExitPayload{
					Cause: "exit", Code: &exitCode,
				}},
			},
			{
				name: "title", event: terminalpkg.TerminalEvent{
					Kind: terminalpkg.EventKindTitleChanged, TerminalID: "term-a",
					Detail: terminalpkg.EventDetail{Title: "tests"},
				},
				wantName:    "terminal.title_changed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "title": "tests"},
			},
			{
				name: "lease", event: terminalpkg.TerminalEvent{
					Kind: terminalpkg.EventKindLeaseChanged, TerminalID: "term-a",
					Actor: terminalpkg.Actor{Kind: terminalpkg.ActorKindAgent, ID: "requester"}, Reason: "takeover",
					Detail: terminalpkg.EventDetail{LeaseTo: terminalpkg.LeaseHumanOwned},
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
				name: "mode", event: terminalpkg.TerminalEvent{
					Kind: terminalpkg.EventKindModeChanged, TerminalID: "term-a",
					Detail: terminalpkg.EventDetail{Mode: terminalpkg.ModePipe},
				},
				wantName:    "terminal.mode_changed",
				wantPayload: gin.H{"terminal_id": terminalpkg.ID("term-a"), "mode": terminalpkg.ModePipe},
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

	provider := &terminalProviderStub{}
	catalog := newTerminalCatalog(provider)
	_, reset, fence, changed := catalog.read("workspace-a", "profile-a", 0)
	if !reset || fence != 0 {
		t.Fatalf("initial subscribe reset/fence = %v/%d", reset, fence)
	}
	provider.emit(terminalpkg.TerminalEvent{
		Kind: terminalpkg.EventKindOpened, WorkspaceID: "workspace-a", ProfileID: "profile-a",
		TerminalID: "term-a", Info: &terminalpkg.Info{ID: "term-a"},
	})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("catalog observer did not signal a retained event")
	}
	_, _, _, changed = catalog.read("workspace-a", "profile-a", 1)
	provider.emit(terminalpkg.TerminalEvent{
		Kind: terminalpkg.EventKindTitleChanged, WorkspaceID: "workspace-a", ProfileID: "profile-a",
		TerminalID: "term-a", Detail: terminalpkg.EventDetail{Title: "build"},
	})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("catalog observer did not signal the title event")
	}
	replay, reset, fence, _ := catalog.read("workspace-a", "profile-a", 1)
	if reset || fence != 2 || len(replay) != 1 || replay[0].Sequence != 2 || replay[0].Event.Kind != terminalpkg.EventKindTitleChanged {
		t.Fatalf("replay = %#v, reset=%v fence=%d", replay, reset, fence)
	}
	for index := 0; index < terminalCatalogRetention+1; index++ {
		provider.emit(terminalpkg.TerminalEvent{
			Kind: terminalpkg.EventKindLeaseChanged, WorkspaceID: "workspace-a", ProfileID: "profile-a",
			TerminalID: "term-a", Detail: terminalpkg.EventDetail{LeaseTo: terminalpkg.LeaseAvailable},
		})
	}
	_, reset, _, _ = catalog.read("workspace-a", "profile-a", 1)
	if !reset {
		t.Fatal("cursor older than retained catalog did not request a snapshot reset")
	}
}

type terminalProviderStub struct {
	manager   terminalpkg.Manager
	observers []func(context.Context, terminalpkg.TerminalEvent)
}

func (p *terminalProviderStub) TerminalFor(string) (terminalpkg.Manager, error) {
	if p.manager == nil {
		p.manager = terminalManagerStub{}
	}
	return p.manager, nil
}

func (p *terminalProviderStub) Observe(observer func(context.Context, terminalpkg.TerminalEvent)) {
	p.observers = append(p.observers, observer)
}

func (p *terminalProviderStub) emit(event terminalpkg.TerminalEvent) {
	for _, observer := range p.observers {
		observer(context.Background(), event)
	}
}

type terminalManagerStub struct {
	handle terminalpkg.Handle
	info   *terminalpkg.Info
}

type terminalAgentManagerStub struct {
	terminalManagerStub
	handle     *terminalAgentHandleStub
	journal    *terminalAgentJournalStub
	exec       terminalpkg.ExecRequest
	inputScope store.ReadScope
}

func (m *terminalAgentManagerStub) Exec(_ context.Context, request terminalpkg.ExecRequest) (*terminalpkg.ExecResult, error) {
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
	return &terminalpkg.RecordingRef{ID: id, Bytes: int64(len("asciicast"))}, io.NopCloser(strings.NewReader("asciicast")), nil
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
func (*terminalAgentJournalStub) LinkRecording(context.Context, string, terminalpkg.ID, terminalpkg.RecordingRef) error {
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

type scopeRecordingTerminalManager struct {
	terminalManagerStub
	scope store.ReadScope
}

func (m *scopeRecordingTerminalManager) List(_ context.Context, _ string, scope store.ReadScope) ([]terminalpkg.Info, error) {
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
func (terminalManagerStub) List(context.Context, string, store.ReadScope) ([]terminalpkg.Info, error) {
	return []terminalpkg.Info{}, nil
}
func (terminalManagerStub) Close(context.Context, string, terminalpkg.ID, terminalpkg.Actor, terminalpkg.Signal) (*terminalpkg.Exit, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (terminalManagerStub) Journal() terminalpkg.Journal                             { return nil }
func (terminalManagerStub) Shutdown(context.Context) error                           { return nil }
func (terminalManagerStub) Observe(func(context.Context, terminalpkg.TerminalEvent)) {}
func (terminalManagerStub) ArchiveProfile(context.Context, string) error             { return nil }

type terminalHandleStub struct {
	attachErr    error
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

func (terminalHandleStub) Info() terminalpkg.Info { return terminalpkg.Info{} }
func (terminalHandleStub) MarkerNonce() string    { return "" }
func (h terminalHandleStub) Attach(context.Context, terminalpkg.AttachOptions) (terminalpkg.Subscription, error) {
	return nil, h.attachErr
}
func (terminalHandleStub) Write(context.Context, terminalpkg.Actor, []byte) error { return nil }
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
func (h terminalHandleStub) AnswerInput(context.Context, terminalpkg.Actor, terminalpkg.InputRequestID, terminalpkg.InputAnswer) (*terminalpkg.InputOutcome, error) {
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
