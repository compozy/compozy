package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

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
			Mode: terminalpkg.ModePTY,
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

	t.Run("Should replay retained events exactly once and reset stale cursors", func(t *testing.T) {
		t.Parallel()

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
				Kind: terminalpkg.EventKindModeChanged, WorkspaceID: "workspace-a", ProfileID: "profile-a",
				TerminalID: "term-a", Detail: &terminalpkg.EventDetail{Mode: terminalpkg.ModePTY},
			})
		}
		replay, reset, fence, changed = catalog.read("workspace-a", "profile-a", 1)
		if len(replay) != 0 || !reset || fence <= terminalCatalogRetention || changed == nil {
			t.Fatalf("stale cursor replay/reset/fence/changed = %#v/%v/%d/%v", replay, reset, fence, changed)
		}
	})
}
