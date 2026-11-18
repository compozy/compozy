package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func streamEventSummary(summary store.EventSummary, content json.RawMessage) store.EventSummary {
	summary.SetContent(content)
	return summary
}

type bufferFlusher struct {
	bytes.Buffer
}

func (bufferFlusher) Flush() {}

func TestStreamSessionHandlerPollsForNewEvents(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	callCount := 0
	manager := &stubSessionManager{
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
		t.Context(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw&limit=200",
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
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := &stubSessionManager{
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
		t.Context(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream?frames=raw&limit=200",
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
	var payload sessionEventPayload
	if err := json.Unmarshal(records[0].Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal(stop payload) error = %v; data=%s", err, string(records[0].Data))
	}
	if payload.WorkspaceID != "ws-workspace" || payload.WorkspacePath != "/workspace" {
		t.Fatalf("stop payload = %#v", payload)
	}
}

func TestStreamSessionHandlerEmitsTerminalErrorWhenTranscriptInitializationFails(t *testing.T) {
	t.Parallel()

	t.Run("Should emit one terminal error after the SSE response starts", func(t *testing.T) {
		t.Parallel()

		initErr := errors.New("transcript projection unavailable")
		pageCalls := 0
		manager := &stubSessionManager{
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
			t.Context(),
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
	t.Parallel()

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
					{
						ID:        "sum-1",
						Sequence:  1,
						SessionID: "sess-1",
						Type:      "agent_message",
						AgentName: "coder",
						Timestamp: timestamp,
					},
				}, nil
			case 2:
				if query.Limit <= 0 || query.Limit > 500 || query.AfterSequence != 1 || !query.Forward {
					t.Fatalf("poll query = %#v, want bounded forward read after the durable cursor", query)
				}
				close(done)
				return []store.EventSummary{
					{
						ID:        "sum-2",
						Sequence:  2,
						SessionID: "sess-1",
						Type:      "done",
						AgentName: "coder",
						Timestamp: timestamp.Add(-time.Second),
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}
	handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		t.Context(),
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

func TestStreamLogsResumePages(t *testing.T) {
	t.Parallel()

	t.Run("Should drain full legacy resume pages before advancing to the head", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		boundary := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
		events := make([]store.EventSummary, 202)
		for index := range events {
			events[index] = store.EventSummary{
				ID: "delayed", Sequence: int64(index + 1), Type: "settings.changed",
				Timestamp: boundary.Add(time.Hour),
			}
		}
		events[0].Timestamp = boundary.Add(2 * time.Hour)
		calls := 0
		observer := stubObserver{
			EventSequenceHeadFn: func(context.Context) (int64, error) { return 500, nil },
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				calls++
				if !query.Since.Equal(boundary) {
					t.Fatalf("resume since = %v, want %v", query.Since, boundary)
				}
				switch calls {
				case 1:
					if query.Limit != 1 || query.Forward || query.SequenceOrder {
						t.Fatalf("legacy initial query = %#v, want timestamp-ordered page of one", query)
					}
					return events[:1], nil
				case 2:
					if query.AfterSequence != 1 || !query.Forward || query.Limit != 200 {
						t.Fatalf("first poll = %#v, want 200 rows after sequence 1", query)
					}
					return events[1:201], nil
				default:
					if query.AfterSequence != 201 || !query.Forward {
						t.Fatalf("second poll = %#v, want read after sequence 201", query)
					}
					close(done)
					return events[201:], nil
				}
			},
		}
		handlers := newTestHandlers(t, &stubSessionManager{}, observer, newTestHomePaths(t))
		handlers.setStreamDone(done)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/logs/stream?limit=1", http.NoBody)
		req.Header.Set("Last-Event-ID", boundary.Format(time.RFC3339Nano)+"|legacy-id")
		newTestRouter(t, handlers).ServeHTTP(recorder, req)

		records := parseSSE(t, recorder.Body.String())
		if recorder.Code != http.StatusOK || len(records) != len(events) {
			t.Fatalf("stream status/count = %d/%d, want 200/%d", recorder.Code, len(records), len(events))
		}
		for index, record := range records {
			if record.ID != core.LogEventID(events[index]) {
				t.Fatalf("record %d ID = %q, want %q", index, record.ID, core.LogEventID(events[index]))
			}
		}
	})
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
			EventSequenceHeadFn: func(context.Context) (int64, error) { return 40, nil },
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				callCount++
				if query.Limit <= 0 || query.Limit > 500 || query.AfterSequence != 40 || !query.Forward {
					t.Fatalf("live-only query = %#v, want bounded read after the connection's durable head", query)
				}
				close(done)
				return []store.EventSummary{{
					ID:        "sum-live",
					Sequence:  41,
					SessionID: "sess-1",
					Type:      "done",
					AgentName: "coder",
					Timestamp: boundary.Add(time.Second),
				}}, nil
			},
		}
		handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/logs/stream?workspace_id=ws-workspace&replay=false",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		if got, want := callCount, 1; got != want {
			t.Fatalf("QueryEvents() calls = %d, want %d live read without a retained-history scan", got, want)
		}
		if records := parseSSE(
			t,
			recorder.Body.String(),
		); len(records) != 1 ||
			records[0].ID != core.LogEventID(store.EventSummary{Sequence: 41, Timestamp: boundary.Add(time.Second)}) {
			t.Fatalf("live-only records = %#v, want one event", records)
		}
	})

	t.Run("Should advance past unmatched events when the filter matches nothing", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		done := make(chan struct{})
		heads := []int64{40, 55}
		headCalls := 0
		var afterSequences []int64
		observer := stubObserver{
			EventSequenceHeadFn: func(context.Context) (int64, error) {
				head := heads[min(headCalls, len(heads)-1)]
				headCalls++
				return head, nil
			},
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				if !query.Forward {
					t.Fatalf("poll query = %#v, want forward read", query)
				}
				afterSequences = append(afterSequences, query.AfterSequence)
				if len(afterSequences) == 2 {
					close(done)
				}
				return nil, nil
			},
		}
		handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/logs/stream?component=profile&replay=false",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		// Other components appended 15 rows before the first poll. The second poll
		// must start after them instead of rereading from sequence 40.
		if want := []int64{40, 55}; !slices.Equal(afterSequences, want) {
			t.Fatalf("poll AfterSequence = %v, want %v", afterSequences, want)
		}
		if records := parseSSE(t, recorder.Body.String()); len(records) != 0 {
			t.Fatalf("records = %#v, want none for an unmatched filter", records)
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
		handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
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
	t.Parallel()

	homePaths := newTestHomePaths(t)
	done := make(chan struct{})
	doneOnce := sync.OnceFunc(func() { close(done) })
	var capturedQuery store.EventSummaryQuery
	observer := stubObserver{
		QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
			capturedQuery = query
			doneOnce()
			return []store.EventSummary{{
				ID:        "sum-harness",
				SessionID: "sess-harness",
				Type:      "harness.context_resolved",
				AgentName: "coder",
				Summary:   "surface=startup sections=skills|situation",
				Timestamp: time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC),
			}}, nil
		},
	}
	handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
	handlers.setStreamDone(done)
	engine := newTestRouter(t, handlers)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(),
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
	if got, want := capturedQuery.SessionID, "sess-harness"; got != want {
		t.Fatalf("observer query session_id = %q, want %q", got, want)
	}
	if !bytes.Contains(records[0].Data, []byte("sections=skills|situation")) {
		t.Fatalf("payload = %s, want harness summary content", string(records[0].Data))
	}
}

func TestStreamLogsCarriesProfileLifecyclePayloads(t *testing.T) {
	t.Parallel()
	t.Run("Should carry profile lifecycle payloads without a workspace filter", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		done := make(chan struct{})
		doneOnce := sync.OnceFunc(func() { close(done) })
		var capturedQuery store.EventSummaryQuery
		observer := stubObserver{
			QueryEventsFn: func(_ context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
				capturedQuery = query
				if query.Limit == 1 && !query.Forward {
					return nil, nil
				}
				doneOnce()
				// Profile events are global scope: they carry no workspace, session, or
				// agent, and the registry allows that.
				return []store.EventSummary{streamEventSummary(store.EventSummary{
					ID:        "sum-profile",
					Sequence:  1,
					Type:      "profile.selection_changed",
					Outcome:   "info",
					Summary:   "profile.selection_changed",
					Timestamp: time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC),
				}, json.RawMessage(`{"name":"profile.selection_changed","profile_id":"01J9","profile_name":"marketing"}`))}, nil
			},
		}
		handlers := newTestHandlers(t, &stubSessionManager{}, observer, homePaths)
		handlers.setStreamDone(done)
		engine := newTestRouter(t, handlers)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/logs/stream?component=profile&replay=false",
			http.NoBody,
		)
		engine.ServeHTTP(recorder, req)

		if got, want := capturedQuery.Component, "profile"; got != want {
			t.Fatalf("observer query component = %q, want %q", got, want)
		}
		// An exact-match workspace filter would drop every global-scope profile row.
		if got := capturedQuery.WorkspaceID; got != "" {
			t.Fatalf("observer query workspace_id = %q, want empty for global-scope profile events", got)
		}

		records := parseSSE(t, recorder.Body.String())
		if got, want := len(records), 1; got != want {
			t.Fatalf("len(records) = %d, want %d; body=%s", got, want, recorder.Body.String())
		}
		if got, want := records[0].Event, "profile.selection_changed"; got != want {
			t.Fatalf("records[0].Event = %q, want %q", got, want)
		}

		var payload logEventPayload
		if err := json.Unmarshal(records[0].Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(profile payload) error = %v", err)
		}
		if got, want := payload.Type, "profile.selection_changed"; got != want {
			t.Fatalf("payload.Type = %q, want %q", got, want)
		}
		if !bytes.Contains(payload.Content, []byte(`"profile_name":"marketing"`)) {
			t.Fatalf("payload.Content = %s, want snake_case profile payload", string(payload.Content))
		}
	})
}

func TestHelperBuildersCoverRemainingBranches(t *testing.T) {
	if acpCapsPayloadFromInfo(acp.Caps{SupportsLoadSession: true}) == nil {
		t.Fatal("expected non-nil caps payload")
	}
	usage := int64(10)
	payload := core.TokenUsagePayloadFromUsage(
		&acp.TokenUsage{InputTokens: &usage, Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)},
	)
	if payload == nil || payload.Timestamp.IsZero() {
		t.Fatalf("TokenUsagePayloadFromUsage() = %#v", payload)
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

func TestPayloadAndStatusHelpersCoverRemainingBranches(t *testing.T) {
	if got := string(payloadJSON("")); got != "null" {
		t.Fatalf("payloadJSON(blank) = %q, want null", got)
	}
	if got := string(payloadJSON("plain-text")); got == "" || got == "plain-text" {
		t.Fatalf("payloadJSON(plain-text) = %q, want quoted JSON", got)
	}
	if status := core.StatusForSessionError(os.ErrNotExist); status != http.StatusNotFound {
		t.Fatalf("statusForSessionError(os.ErrNotExist) = %d, want %d", status, http.StatusNotFound)
	}
	if status := core.StatusForSessionError(session.ErrPromptInProgress); status != http.StatusConflict {
		t.Fatalf("statusForSessionError(ErrPromptInProgress) = %d, want %d", status, http.StatusConflict)
	}
	if status := core.StatusForSessionError(workspacepkg.ErrWorkspaceNotFound); status != http.StatusNotFound {
		t.Fatalf("statusForSessionError(ErrWorkspaceNotFound) = %d, want %d", status, http.StatusNotFound)
	}
	if status := core.StatusForSessionError(workspacepkg.ErrWorkspaceRootMissing); status != http.StatusGone {
		t.Fatalf("statusForSessionError(ErrWorkspaceRootMissing) = %d, want %d", status, http.StatusGone)
	}
	if status := statusForWorkspaceError(workspacepkg.ErrWorkspacePathTaken); status != http.StatusConflict {
		t.Fatalf("statusForWorkspaceError(ErrWorkspacePathTaken) = %d, want %d", status, http.StatusConflict)
	}
	if status := statusForWorkspaceError(workspacepkg.ErrWorkspaceHasSessions); status != http.StatusConflict {
		t.Fatalf("statusForWorkspaceError(ErrWorkspaceHasSessions) = %d, want %d", status, http.StatusConflict)
	}
	if status := core.StatusForSessionError(errors.New("boom")); status != http.StatusInternalServerError {
		t.Fatalf("statusForSessionError(default) = %d, want %d", status, http.StatusInternalServerError)
	}
}

func TestExtractPromptMessageCoversContentFallbacks(t *testing.T) {
	t.Parallel()

	message, err := extractPromptMessage(promptRequest{
		MessageID:      "msg-content",
		IdempotencyKey: "idem-content",
		Messages: []uiMessageEnvelope{{
			ID:      "msg-content",
			Role:    "user",
			Content: "content path",
		}},
	})
	if err != nil || message != "content path" {
		t.Fatalf("extractPromptMessage(content) = %q, %v", message, err)
	}

	message, err = extractPromptMessage(promptRequest{
		MessageID:      "msg-parts",
		IdempotencyKey: "idem-parts",
		Messages: []uiMessageEnvelope{{
			Role: "assistant",
		}, {
			ID:   "msg-parts",
			Role: "user",
			Parts: []uiMessageTextPart{
				{Type: "tool-call", Text: "ignored"},
				{Type: "text", Text: "part one"},
				{Type: " text ", Text: "part one point five"},
				{Type: "", Text: "part two"},
			},
		}},
	})
	if err != nil || message != "part one\npart one point five\npart two" {
		t.Fatalf("extractPromptMessage(parts) = %q, %v", message, err)
	}

	if _, err := extractPromptMessage(promptRequest{}); err == nil {
		t.Fatal("extractPromptMessage(empty) error = nil, want non-nil")
	}
}
