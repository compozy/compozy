package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
)

func TestClearSessionConversationHandler(t *testing.T) {
	t.Run("ShouldReturnTheClearedSession", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			ClearFn: func(_ context.Context, id string) (*session.Session, error) {
				if id != "sess-123" {
					t.Fatalf("ClearConversation() id = %q, want sess-123", id)
				}
				return newSession(id), nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/clear",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.SessionResponse
		decodeJSONResponse(t, recorder, &response)
		if got, want := response.Session.ID, "sess-123"; got != want {
			t.Fatalf("response.session.id = %q, want %q", got, want)
		}
	})

	t.Run("ShouldReturnConflictWhenPromptIsInProgress", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			ClearFn: func(context.Context, string) (*session.Session, error) {
				return nil, session.ErrPromptInProgress
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/clear",
			nil,
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
		}
	})
}
