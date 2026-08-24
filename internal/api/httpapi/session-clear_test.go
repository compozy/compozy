package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
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

func TestRewindSessionConversationHandler(t *testing.T) {
	t.Run("Should return the restarted session and draft for the selected durable message", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				info := newSessionInfo(id)
				info.WorkspaceID = "ws-workspace"
				info.ProfileID = store.DefaultProfileID
				return info, nil
			},
			RewindFn: func(_ context.Context, id string, opts session.ConversationRewindOptions) (session.ConversationRewindResult, error) {
				if id != "sess-123" || opts.MessageID != "msg-2" || opts.IdempotencyKey != "idem-1" ||
					opts.ExpectedEpoch != 3 ||
					opts.ExpectedGeneration != 4 || opts.ExpectedMaxSequence != 12 {
					t.Fatalf("RewindConversation() = %q %#v, want routed request", id, opts)
				}
				return session.ConversationRewindResult{
					Session: newSession(id), TranscriptEpoch: 4, TargetMessageID: opts.MessageID,
					ArchivedFrom: 8, ArchivedThrough: 13, ArchivedEvents: 6,
					Generation: 5, MaxSequence: 7, DraftText: "try again",
				}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
		epoch, generation, maxSequence := int64(3), int64(4), int64(12)
		body, err := json.Marshal(contract.SessionConversationRewindRequest{
			MessageID: "msg-2", IdempotencyKey: "idem-1", ExpectedEpoch: &epoch,
			ExpectedGeneration: &generation, ExpectedMaxSequence: &maxSequence,
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		recorder := performRequest(t, engine, http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/rewind", body)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		var response contract.SessionConversationRewindResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Session.ID != "sess-123" || response.Rewind.DraftText != "try again" ||
			response.Rewind.TranscriptEpoch != 4 {
			t.Fatalf("response = %#v, want restarted session and selected draft", response)
		}
	})

	t.Run("Should reject a session owned by another profile before rewinding", func(t *testing.T) {
		t.Parallel()

		called := false
		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				info := newSessionInfo(id)
				info.WorkspaceID = "ws-workspace"
				info.ProfileID = "01JMARKETINGPROFILE0000000"
				return info, nil
			},
			RewindFn: func(
				context.Context,
				string,
				session.ConversationRewindOptions,
			) (session.ConversationRewindResult, error) {
				called = true
				return session.ConversationRewindResult{}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		epoch, generation, maxSequence := int64(3), int64(4), int64(12)
		body, err := json.Marshal(contract.SessionConversationRewindRequest{
			MessageID: "msg-2", IdempotencyKey: "idem-foreign", ExpectedEpoch: &epoch,
			ExpectedGeneration: &generation, ExpectedMaxSequence: &maxSequence,
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-123/rewind",
			body,
		)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
		}
		if called {
			t.Fatal("RewindConversation() called for a foreign-profile session")
		}
	})
}
