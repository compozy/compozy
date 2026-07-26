package udsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

type bufferFlusher struct {
	bytes.Buffer
}

func (bufferFlusher) Flush() {}

func TestStreamSessionHandlerPollsForNewEvents(t *testing.T) {
	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	callCount := 0
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
		EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
			callCount++
			switch callCount {
			case 1:
				return []store.SessionEvent{{
					ID:        "ev-1",
					SessionID: "sess-123",
					Sequence:  1,
					TurnID:    "turn-1",
					Type:      "agent_message",
					AgentName: "coder",
					Content:   `{"text":"hello"}`,
					Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				}}, nil
			case 2:
				close(done)
				return []store.SessionEvent{{
					ID:        "ev-2",
					SessionID: "sess-123",
					Sequence:  2,
					TurnID:    "turn-1",
					Type:      "done",
					AgentName: "coder",
					Content:   `{"stop_reason":"end_turn"}`,
					Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
				}}, nil
			default:
				return nil, nil
			}
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2; body=%s", len(records), recorder.Body.String())
	}
	if records[0].ID != "1" || records[1].ID != "2" {
		t.Fatalf("records = %#v", records)
	}
}

func TestStreamSessionHandlerStopsWhenSessionIsAlreadyStopped(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			info := newSessionInfo("sess-123")
			info.State = session.StateStopped
			info.UpdatedAt = time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC)
			return info, nil
		},
		EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
			return nil, nil
		},
	}
	handlers := newTestHandlers(t, manager, stubObserver{}, homePaths)
	engine := newTestRouter(t, handlers)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1; body=%s", len(records), recorder.Body.String())
	}
	if records[0].Event != session.EventTypeSessionStopped {
		t.Fatalf("records[0].Event = %q, want %q", records[0].Event, session.EventTypeSessionStopped)
	}
}

func TestStreamSessionHandlerEmitsTerminalErrorWhenTranscriptInitializationFails(t *testing.T) {
	t.Parallel()

	t.Run("Should emit one terminal error after the SSE response starts", func(t *testing.T) {
		t.Parallel()

		initErr := errors.New("transcript projection unavailable")
		pageCalls := 0
		manager := stubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return newSessionInfo("sess-123"), nil
			},
			TranscriptPageFn: func(context.Context, string, transcript.PageQuery) (transcript.Page, error) {
				pageCalls++
				return transcript.Page{}, initErr
			},
			TranscriptChangesFn: func(context.Context, string, transcript.ChangeQuery) (transcript.ChangePage, error) {
				t.Fatal("TranscriptChanges() called after initial transcript failure")
				return transcript.ChangePage{}, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		engine := newTestRouter(t, handlers)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/workspaces/ws-workspace/sessions/sess-123/stream",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		records := parseSSE(t, recorder.Body.String())
		if len(records) != 1 || records[0].Event != "error" {
			t.Fatalf("stream records = %#v, want one terminal error frame", records)
		}
		if !bytes.Contains(records[0].Data, []byte(initErr.Error())) || pageCalls != 1 {
			t.Fatalf(
				"terminal error data/calls = %s/%d, want redacted diagnostic and one initialization",
				records[0].Data,
				pageCalls,
			)
		}
	})
}

func TestStreamLogsPollsForNewEvents(t *testing.T) {
	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	callCount := 0
	recorder := httptest.NewRecorder()
	observer := stubObserver{
		QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
			callCount++
			timestamp := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
			switch callCount {
			case 1:
				if got, want := recorder.Header().Get("Content-Type"), "text/event-stream"; got != want {
					t.Fatalf("Content-Type before replay query = %q, want %q", got, want)
				}
				if got, want := query.Limit, 200; got != want {
					t.Fatalf("initial replay limit = %d, want %d", got, want)
				}
				return []store.EventSummary{
					{ID: "sum-1", SessionID: "sess-1", Type: "agent_message", AgentName: "coder", Timestamp: timestamp},
				}, nil
			case 2:
				if query.Limit != 0 || !query.Since.Equal(timestamp) {
					t.Fatalf("poll query = %#v, want unbounded live read after initial cursor", query)
				}
				close(done)
				return []store.EventSummary{
					{
						ID:        "sum-2",
						SessionID: "sess-1",
						Type:      "done",
						AgentName: "coder",
						Timestamp: timestamp.Add(time.Second),
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/logs/stream?workspace_id=ws-workspace",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2; body=%s", len(records), recorder.Body.String())
	}
	if records[0].ID == records[1].ID {
		t.Fatalf("expected distinct observe SSE ids, got %#v", records)
	}
}

func TestStreamLogsReplayFalseLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should skip retained history and begin polling at the connection boundary", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		done := make(chan struct{})
		callCount := 0
		boundary := time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
		observer := stubObserver{
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				callCount++
				if query.Limit != 0 || !query.Since.Equal(boundary) {
					t.Fatalf("live-only query = %#v, want no replay and connection boundary", query)
				}
				close(done)
				return []store.EventSummary{{
					ID:        "sum-live",
					SessionID: "sess-1",
					Type:      "done",
					AgentName: "coder",
					Timestamp: boundary.Add(time.Second),
				}}, nil
			},
		}
		handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/logs/stream?workspace_id=ws-workspace&replay=false",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		if got, want := callCount, 1; got != want {
			t.Fatalf("QueryEvents() calls = %d, want %d live poll", got, want)
		}
		if records := parseSSE(t, recorder.Body.String()); len(records) != 1 || records[0].ID == "" {
			t.Fatalf("live-only records = %#v, want one event", records)
		}
	})

	t.Run("Should not query after the stream is already closed", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		done := make(chan struct{})
		close(done)
		callCount := 0
		observer := stubObserver{
			QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
				callCount++
				return nil, nil
			},
		}
		handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/logs/stream?workspace_id=ws-workspace&replay=false",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		if callCount != 0 {
			t.Fatalf("QueryEvents() calls = %d, want 0 for closed stream", callCount)
		}
		if got, want := recorder.Header().Get("Content-Type"), "text/event-stream"; got != want {
			t.Fatalf("closed stream Content-Type = %q, want %q", got, want)
		}
	})
}

func TestStreamLogsCarriesHarnessLifecyclePayloads(t *testing.T) {
	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	var doneOnce sync.Once
	observer := stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			doneOnce.Do(func() { close(done) })
			return []store.EventSummary{{
				ID:        "sum-harness",
				SessionID: "sess-harness",
				Type:      "harness.context_resolved",
				AgentName: "coder",
				Summary:   "surface=startup sections=memory|skills|network",
				Timestamp: time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC),
			}}, nil
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/logs/stream?workspace_id=ws-workspace&session_id=sess-harness",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(records) = %d, want %d; body=%s", got, want, recorder.Body.String())
	}

	var payload logEventPayload
	if err := json.Unmarshal(records[0].Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal(observe payload) error = %v", err)
	}
	if got, want := payload.Type, "harness.context_resolved"; got != want {
		t.Fatalf("payload.Type = %q, want %q", got, want)
	}
	if got, want := payload.SessionID, "sess-harness"; got != want {
		t.Fatalf("payload.SessionID = %q, want %q", got, want)
	}
	if !bytes.Contains(records[0].Data, []byte("sections=memory|skills|network")) {
		t.Fatalf("payload = %s, want harness summary content", string(records[0].Data))
	}
}

func TestStreamBridgeHealthPollsForChangedSnapshots(t *testing.T) {
	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	callCount := 0
	observer := stubObserver{
		QueryBridgeHealthFn: func(context.Context) ([]observe.BridgeInstanceHealth, error) {
			callCount++
			switch callCount {
			case 1, 2:
				return []observe.BridgeInstanceHealth{{
					BridgeInstanceID: "brg-123",
					Status:           bridgepkg.BridgeStatusAuthRequired,
				}}, nil
			case 3:
				close(done)
				return []observe.BridgeInstanceHealth{{
					BridgeInstanceID:      "brg-123",
					Status:                bridgepkg.BridgeStatusReady,
					RouteCount:            3,
					DeliveryBacklog:       1,
					AuthFailuresTotal:     1,
					DeliveryFailuresTotal: 2,
				}}, nil
			default:
				return nil, nil
			}
		},
	}
	handlers := newTestHandlersWithBridges(
		t,
		stubSessionManager{},
		observer,
		stubBridgeService{
			ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
				t.Fatal("ListInstances() must not run for the bounded stream")
				return nil, nil
			},
			ListInstancesByIDsFn: func(context.Context, []string) ([]bridgepkg.BridgeInstance, error) {
				return []bridgepkg.BridgeInstance{{
					ID: "brg-123", Scope: bridgepkg.ScopeGlobal, Enabled: true, Status: bridgepkg.BridgeStatusReady,
				}}, nil
			},
		},
		stubWorkspaceService{},
		homePaths,
	)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/bridges/health/stream?bridge_ids=brg-123",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, req)

	records := parseSSE(t, recorder.Body.String())
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2; body=%s", len(records), recorder.Body.String())
	}
	if records[0].Event != "snapshot" || records[1].Event != "snapshot" {
		t.Fatalf("events = %#v, want snapshot events", records)
	}

	var second contract.BridgeHealthStreamPayload
	if err := json.Unmarshal(records[1].Data, &second); err != nil {
		t.Fatalf("json.Unmarshal(second snapshot) error = %v", err)
	}
	if got, want := second.BridgeHealth["brg-123"].Status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("second status = %q, want %q", got, want)
	}
	if got, want := second.BridgeHealth["brg-123"].RouteCount, 3; got != want {
		t.Fatalf("second route_count = %d, want %d", got, want)
	}
}

func TestHelperBuildersCoverRemainingBranches(t *testing.T) {
	if acpCapsPayloadFromInfo(acp.Caps{SupportsLoadSession: true}) == nil {
		t.Fatal("expected non-nil caps payload")
	}
	usage := int64(10)
	if tokenUsagePayloadFromUsage(&acp.TokenUsage{InputTokens: &usage}) == nil {
		t.Fatal("expected non-nil token usage payload")
	}
	if !observeEventAfterCursor(
		store.EventSummary{ID: "b", Sequence: 2, Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)},
		logsCursor{Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC), Sequence: 1},
	) {
		t.Fatal("expected event to sort after cursor")
	}

	writer := &bufferFlusher{}
	if err := core.WriteSSE(
		writer,
		core.SSEMessage{ID: "1", Name: "done", Data: map[string]string{"ok": "true"}},
	); err != nil {
		t.Fatalf("writeSSE() error = %v", err)
	}
	if got := writer.String(); got == "" || !bytes.Contains([]byte(got), []byte("event: done")) {
		t.Fatalf("writeSSE output = %q", got)
	}
}

func TestNewHandlersAppliesDefaults(t *testing.T) {
	handlers := newHandlers(&handlerConfig{})
	if handlers.Logger == nil {
		t.Fatal("expected default logger")
	}
	if handlers.Now == nil {
		t.Fatal("expected default clock")
	}
	if handlers.PollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %v, want %v", handlers.PollInterval, defaultPollInterval)
	}
	if handlers.AgentLoader == nil {
		t.Fatal("expected default agent loader")
	}
	if handlers.StartedAt.IsZero() {
		t.Fatal("expected non-zero startedAt")
	}
}
