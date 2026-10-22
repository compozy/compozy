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
	"github.com/gin-gonic/gin"
)

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
				name: "closed",
				event: terminalpkg.Event{
					Kind: terminalpkg.EventKindClosed, TerminalID: "term-a", Exit: exit,
				},
				wantName: "terminal.closed",
				wantPayload: gin.H{
					"workspace_id": "",
					"profile_id":   "",
					"terminal_id":  terminalpkg.ID("term-a"),
					"exit": &terminalExitPayload{
						Cause: "exit", Code: &exitCode,
					},
				},
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

func TestTerminalCatalogAggregateSelection(t *testing.T) {
	t.Parallel()
	t.Run("Should replay only matching owners and reset an expired fence", func(t *testing.T) {
		t.Parallel()
		catalog := &terminalCatalog{inbox: make(chan terminalCatalogEvent, terminalCatalogMailbox)}
		catalog.signal.Store(newTerminalCatalogSignal())
		for _, event := range []terminalpkg.Event{
			{Kind: terminalpkg.EventKindClosed, WorkspaceID: "a", ProfileID: "p1", TerminalID: "one"},
			{Kind: terminalpkg.EventKindClosed, WorkspaceID: "b", ProfileID: "p2", TerminalID: "two"},
			{Kind: terminalpkg.EventKindClosed, WorkspaceID: "a", ProfileID: "p1", TerminalID: "three"},
		} {
			catalog.observe(t.Context(), event)
		}
		replay, reset, fence, _ := catalog.readSelection(
			1,
			func(event terminalpkg.Event) bool { return event.WorkspaceID == "a" && event.ProfileID == "p1" },
		)
		if reset || fence != 3 || len(replay) != 1 || replay[0].Event.TerminalID != "three" {
			t.Fatalf("replay=%v reset=%v fence=%d", replay, reset, fence)
		}
		for range terminalCatalogRetention + 1 {
			catalog.observe(t.Context(), terminalpkg.Event{Kind: terminalpkg.EventKindClosed})
		}
		expiredReplay, expiredReset, expiredFence, changed := catalog.readSelection(
			1,
			func(terminalpkg.Event) bool { return true },
		)
		if len(expiredReplay) != 0 || expiredFence <= fence || changed == nil {
			t.Fatalf("invalid reset fence %d replay %v", expiredFence, expiredReplay)
		}
		reset = expiredReset
		if !reset {
			t.Fatal("expired cursor did not reset")
		}
	})
}

func TestTerminalAggregateCursorSelection(t *testing.T) {
	t.Parallel()
	t.Run("Should reset when the selection changes and preserve matching replay positions", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			raw  string
			want uint64
		}{{"12.p1", 12}, {"12.p2", 0}, {"12", 0}, {"", 0}} {
			got, err := terminalAggregateCursor(tc.raw, "p1")
			if err != nil || got != tc.want {
				t.Fatalf("cursor %q = %d,%v", tc.raw, got, err)
			}
		}
		if _, err := terminalAggregateCursor("bad.p1", "p1"); err == nil {
			t.Fatal("accepted invalid matching cursor")
		}
	})
}

// Invariant: server shutdown waits for open SSE requests before it closes the terminal stream
// lifecycle, so every terminal stream must also end on the shared stream-done signal; otherwise a
// runtime restart stalls until the shutdown deadline.
func TestTerminalStreamsEndOnStreamDone(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name  string
		route string
		path  string
		mount func(*BaseHandlers) gin.HandlerFunc
	}{
		{
			name:  "workspace catalog stream",
			route: "/api/workspaces/:workspace_id/terminals/stream",
			path:  "/api/workspaces/workspace-a/terminals/stream?profile=default",
			mount: func(h *BaseHandlers) gin.HandlerFunc { return h.StreamTerminalCatalog },
		},
		{
			name:  "All projects aggregate stream",
			route: "/api/terminals/catalog-stream",
			path:  "/api/terminals/catalog-stream?all_workspaces=true&profile=default",
			mount: func(h *BaseHandlers) gin.HandlerFunc { return h.StreamTerminalAggregate },
		},
	} {
		t.Run("Should end the "+testCase.name, func(t *testing.T) {
			t.Parallel()
			streamDone := make(chan struct{})
			provider := terminalAggregateProviderStub{&terminalProviderStub{Manager: terminalManagerStub{}}}
			handlers := NewBaseHandlers(&BaseHandlerConfig{
				TransportName: "udsapi",
				Terminal:      provider,
				Workspaces:    workspaceServiceStub{},
				StreamDone:    streamDone,
			})
			router := gin.New()
			router.GET(testCase.route, testCase.mount(handlers))
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testCase.path, http.NoBody)
			response := httptest.NewRecorder()
			served := make(chan struct{})
			go func() {
				defer close(served)
				router.ServeHTTP(response, request)
			}()
			// The stream is live until the server starts shutting down.
			select {
			case <-served:
				t.Fatalf("stream ended before shutdown: %d/%q", response.Code, response.Body.String())
			case <-time.After(200 * time.Millisecond):
			}
			close(streamDone)
			select {
			case <-served:
			case <-time.After(5 * time.Second):
				t.Fatal("terminal stream stayed open after the stream-done signal")
			}
			if response.Code != http.StatusOK {
				t.Fatalf("stream status/body = %d/%q, want an opened stream", response.Code, response.Body.String())
			}
		})
	}
}

// terminalAggregateProviderStub adds the empty machine-wide terminal read the aggregate stream lists.
type terminalAggregateProviderStub struct{ *terminalProviderStub }

func (terminalAggregateProviderStub) ListAggregate(context.Context, store.ReadScope) ([]terminalpkg.Info, error) {
	return nil, nil
}
