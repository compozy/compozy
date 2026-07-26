package udsapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/agh/internal/store"
)

func TestSessionEventsLastEventIDHeader(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer Last-Event-ID over after_sequence query", func(t *testing.T) {
		t.Parallel()

		var gotQuery store.EventQuery
		manager := stubSessionManager{
			EventsFn: func(_ context.Context, _ string, query store.EventQuery) ([]store.SessionEvent, error) {
				gotQuery = query
				return nil, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		engine := newTestRouter(t, handlers)

		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/workspaces/ws-workspace/sessions/sess-123/events?after_sequence=5&limit=10",
			http.NoBody,
		)
		request.Header.Set("Last-Event-ID", "9")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if gotQuery.AfterSequence != 9 || gotQuery.Limit != 10 {
			t.Fatalf("query = %#v, want Last-Event-ID sequence and query limit", gotQuery)
		}
	})
}
