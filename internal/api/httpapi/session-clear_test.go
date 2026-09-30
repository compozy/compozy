package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
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

func continueTestManager(
	t *testing.T,
	workspaceID string,
	continueFn func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error),
) stubSessionManager {
	t.Helper()
	return stubSessionManager{
		// Derive routes never read the source through Status: it repairs an inactive
		// session's metadata, and ownership is validated by the manager's
		// non-repairing snapshot after receipt replay (ADR-007).
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			t.Errorf("Status(%q) called by a derive route; want no repairing source read in %s", id, workspaceID)
			return nil, session.ErrSessionNotFound
		},
		ContinueFn: continueFn,
		DerivePreviewFn: func(_ context.Context, _, _, messageID string) (session.DerivePreview, error) {
			preview := session.DerivePreview{
				MessageCount: 42, SourceMessageCount: 42, ReplayBytes: 62771,
				Epoch: 3, Generation: 12, MaxSequence: 418,
			}
			if messageID != "" {
				preview.SourceMessageCount = 60
				preview.Cut = &session.DeriveCut{MessageID: messageID, TurnID: "turn-7", TurnSettled: true}
			}
			return preview, nil
		},
	}
}

func continuedChildInfo() *session.Info {
	child := newSessionInfo("sess-child")
	child.WorkspaceID = "ws-workspace"
	child.ProfileID = store.DefaultProfileID
	child.Lineage = &store.SessionLineage{
		ParentSessionID: "sess-123", RootSessionID: "sess-123", SpawnDepth: 1,
		Kind: store.LineageKindContinue, OriginAgentName: "codex",
	}
	child.Derivation = &store.SessionDerivation{
		Kind: store.LineageKindContinue, SourceSessionID: "sess-123", Seed: store.SessionDerivationSeedReplay,
		FirstPrompt: store.SessionFirstPrompt{State: store.SessionDerivationFirstPromptAdmitted},
	}
	return child
}

func continuedResult(replayed bool) session.DeriveResult {
	return session.DeriveResult{
		Child: continuedChildInfo(), ChildSessionID: "sess-child", Kind: store.LineageKindContinue,
		SourceSessionID: "sess-123", OriginAgentName: "codex", ThroughTurnID: "turn-9",
		Seed: session.DeriveSeedReplay, ReplayMessageCount: 42, ReplayBytes: 62771,
		FirstPrompt: store.SessionDerivationFirstPromptAdmitted, Replayed: replayed,
	}
}

func TestContinueSessionHandler(t *testing.T) {
	t.Parallel()

	const continuePath = "/api/workspaces/ws-workspace/sessions/sess-123/continue"

	t.Run("Should create the continued session with a runtime body", func(t *testing.T) {
		t.Parallel()

		var captured session.ContinueSessionOpts
		manager := continueTestManager(t, "ws-workspace",
			func(_ context.Context, opts session.ContinueSessionOpts) (session.DeriveResult, error) {
				captured = opts
				return continuedResult(false), nil
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		body := []byte(`{"agent_name":"claude-code","runtime":{"provider":"claude","model":"opus"},` +
			`"message":"go","idempotency_key":"idem-1"}`)
		recorder := performRequest(t, engine, http.MethodPost, continuePath, body)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
		}
		if captured.SourceSessionID != "sess-123" || captured.WorkspaceID != "ws-workspace" ||
			captured.AgentName != "claude-code" || captured.Runtime == nil || captured.Runtime.Model != "opus" ||
			captured.Message != "go" || captured.IdempotencyKey != "idem-1" || captured.ProfileID != "" {
			t.Fatalf("ContinueSession() opts = %#v, want routed request", captured)
		}
		var response contract.SessionDeriveResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Session == nil || response.Session.Lineage == nil || response.Session.Lineage.SpawnDepth != 1 ||
			response.Session.Lineage.Kind != store.LineageKindContinue || response.Session.Derivation == nil ||
			response.Session.Derivation.FirstPrompt != "admitted" {
			t.Fatalf("response.session = %#v, want continued child with derivation", response.Session)
		}
		if response.Derived.FirstPrompt != "admitted" || response.Derived.Replayed ||
			response.Derived.ThroughTurnID != "turn-9" || response.Derived.ReplayMessageCount == nil ||
			*response.Derived.ReplayMessageCount != 42 {
			t.Fatalf("response.derived = %#v, want fresh admitted outcome", response.Derived)
		}
	})

	t.Run("Should accept a declared route body", func(t *testing.T) {
		t.Parallel()

		var route int
		manager := continueTestManager(t, "ws-workspace",
			func(_ context.Context, opts session.ContinueSessionOpts) (session.DeriveResult, error) {
				route = opts.Route
				return continuedResult(false), nil
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"claude-code","route":2,"idempotency_key":"idem-2"}`))
		if recorder.Code != http.StatusCreated || route != 2 {
			t.Fatalf(
				"status = %d route = %d, want 201 with route 2; body=%s",
				recorder.Code,
				route,
				recorder.Body.String(),
			)
		}
	})

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "Should reject runtime and route together", body: `{"agent_name":"b","runtime":{"provider":"claude"},"route":1,"idempotency_key":"k"}`},
		{name: "Should reject partial fences", body: `{"agent_name":"b","idempotency_key":"k","expected_epoch":3}`},
		{name: "Should reject a missing agent", body: `{"idempotency_key":"k"}`},
		{name: "Should reject unknown fields", body: `{"agent_name":"b","idempotency_key":"k","bogus":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			manager := continueTestManager(t, "ws-workspace",
				func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
					called = true
					return session.DeriveResult{}, nil
				})
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performRequest(t, engine, http.MethodPost, continuePath, []byte(tc.body))
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if recorder.Code != http.StatusBadRequest || called || payload.Code != "invalid_request" {
				t.Fatalf("status = %d code = %q called = %t, want 400 invalid_request without a derive; body=%s",
					recorder.Code, payload.Code, called, recorder.Body.String())
			}
		})
	}

	t.Run("Should return 404 session_not_found when the manager finds no source in the workspace", func(t *testing.T) {
		t.Parallel()

		var sources []string
		manager := continueTestManager(t, "ws-workspace",
			func(_ context.Context, opts session.ContinueSessionOpts) (session.DeriveResult, error) {
				sources = append(sources, opts.SourceSessionID+"@"+opts.WorkspaceID)
				return session.DeriveResult{}, session.ErrSessionNotFound
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-missing/continue",
			[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
		var payload contract.ErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if recorder.Code != http.StatusNotFound || payload.Code != "session_not_found" ||
			len(sources) != 1 || sources[0] != "sess-missing@ws-workspace" {
			t.Fatalf("status = %d code = %q sources = %v, want 404 session_not_found from the manager; body=%s",
				recorder.Code, payload.Code, sources, recorder.Body.String())
		}
	})

	t.Run("Should replay a recorded outcome after both child and source were deleted", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(_ context.Context, opts session.ContinueSessionOpts) (session.DeriveResult, error) {
				if opts.IdempotencyKey != "idem-deleted" {
					t.Fatalf("ContinueSession() key = %q, want idem-deleted", opts.IdempotencyKey)
				}
				result := continuedResult(true)
				result.Child = nil
				result.ChildDeleted = true
				return result, nil
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost,
			"/api/workspaces/ws-workspace/sessions/sess-deleted/continue",
			[]byte(`{"agent_name":"b","idempotency_key":"idem-deleted"}`))
		var response contract.SessionDeriveResponse
		decodeJSONResponse(t, recorder, &response)
		if recorder.Code != http.StatusOK || !response.Derived.Replayed || !response.Derived.ChildDeleted {
			t.Fatalf("status = %d response = %#v, want 200 replayed child_deleted", recorder.Code, response)
		}
	})

	t.Run("Should return the committed child id with a post-commit 422", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				return continuedResult(false), &acp.FailureError{
					Kind: store.FailureProviderAuth, Summary: "not authenticated",
				}
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","message":"go","idempotency_key":"k"}`))
		var payload contract.SessionDeriveErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if recorder.Code != http.StatusUnprocessableEntity || payload.ChildSessionID != "sess-child" {
			t.Fatalf("status = %d payload = %#v, want 422 with child_session_id sess-child; body=%s",
				recorder.Code, payload, recorder.Body.String())
		}
	})

	t.Run("Should promote a post-commit model refusal's diagnostic code to the top-level code", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				return continuedResult(false), deriveModelUnavailableError()
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","message":"go","idempotency_key":"k"}`))
		var payload contract.SessionDeriveErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if recorder.Code != http.StatusUnprocessableEntity || payload.ChildSessionID != "sess-child" ||
			payload.Code != contract.CodeModelUnavailable || payload.Diagnostic == nil ||
			payload.Diagnostic.Code != contract.CodeModelUnavailable {
			t.Fatalf("status = %d payload = %#v, want 422 code model_unavailable with the child; body=%s",
				recorder.Code, payload, recorder.Body.String())
		}
	})

	for _, tc := range []struct {
		name        string
		cause       error
		wantMessage string
	}{
		{
			name:        "Should report a post-commit refusal by its diagnostic without the wrapped chain or agent stderr",
			cause:       deriveModelUnavailableError(),
			wantMessage: `Provider configuration is unavailable: acp: model "gone" is unavailable`,
		},
		{
			name:        "Should report a post-commit failure without a diagnostic in a plain sentence",
			cause:       errors.New("acp: subprocess exited: exit status 1"),
			wantMessage: "The new session couldn't start.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := continueTestManager(t, "ws-workspace",
				func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
					return continuedResult(false), deriveFirstMessageStartFailure(tc.cause)
				})
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performRequest(t, engine, http.MethodPost, continuePath,
				[]byte(`{"agent_name":"b","message":"go","idempotency_key":"k"}`))
			var payload contract.SessionDeriveErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.ChildSessionID != "sess-child" || payload.Error != tc.wantMessage ||
				strings.Contains(recorder.Body.String(), "stderr") {
				t.Fatalf("status = %d payload error = %q, want %q with the child and no agent stderr; body=%s",
					recorder.Code, payload.Error, tc.wantMessage, recorder.Body.String())
			}
		})
	}

	t.Run("Should return the committed child id when activation fails after the commit", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				return session.DeriveResult{
					ChildSessionID: "sess-child", Kind: store.LineageKindContinue, SourceSessionID: "sess-123",
				}, errors.New("session: hydrate attention projection: attention store unavailable")
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
		var payload contract.SessionDeriveErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if recorder.Code != http.StatusInternalServerError || payload.ChildSessionID != "sess-child" {
			t.Fatalf("status = %d payload = %#v, want 500 with child_session_id sess-child; body=%s",
				recorder.Code, payload, recorder.Body.String())
		}
	})

	t.Run("Should not report a child for a failure before the commit", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				return session.DeriveResult{}, session.ErrDeriveFenceConflict
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
		var payload contract.SessionDeriveErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if payload.ChildSessionID != "" {
			t.Fatalf("payload = %#v, want no child_session_id before the commit", payload)
		}
	})

	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"Should map draining admission to 503", admission.ErrDraining, http.StatusServiceUnavailable, "new_work_admission_unavailable"},
		{"Should map a missing source", session.ErrSessionNotFound, http.StatusNotFound, "session_not_found"},
		{"Should map a manager validation failure", session.ErrValidation, http.StatusBadRequest, "invalid_request"},
		{"Should map a non-user source", session.ErrSessionNotDerivable, http.StatusBadRequest, "session_not_derivable"},
		{"Should map an archived source", session.ErrDeriveSourceArchived, http.StatusConflict, "session_archived"},
		{"Should map an unknown agent", session.ErrDeriveAgentNotFound, http.StatusNotFound, "agent_not_found"},
		{"Should map a missing route", session.ErrDeriveRouteNotFound, http.StatusConflict, "route_not_found"},
		{"Should map a fence conflict", session.ErrDeriveFenceConflict, http.StatusConflict, "session_fence_conflict"},
		{"Should map an idempotency conflict", session.ErrDeriveIdempotencyConflict, http.StatusConflict, "idempotency_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := continueTestManager(t, "ws-workspace",
				func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
					return session.DeriveResult{}, tc.err
				})
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performRequest(t, engine, http.MethodPost, continuePath,
				[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if recorder.Code != tc.status || payload.Code != tc.code {
				t.Fatalf("status = %d code = %q, want %d %q; body=%s",
					recorder.Code, payload.Code, tc.status, tc.code, recorder.Body.String())
			}
		})
	}

	t.Run("Should return 200 with the recorded outcome on a replay", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				return continuedResult(true), nil
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
		var response contract.SessionDeriveResponse
		decodeJSONResponse(t, recorder, &response)
		if recorder.Code != http.StatusOK || !response.Derived.Replayed || response.Session == nil {
			t.Fatalf("status = %d response = %#v, want 200 replayed", recorder.Code, response)
		}
	})

	t.Run("Should return 200 with child_deleted and no session after the child was deleted", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace",
			func(context.Context, session.ContinueSessionOpts) (session.DeriveResult, error) {
				result := continuedResult(true)
				result.Child = nil
				result.ChildDeleted = true
				return result, nil
			})
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodPost, continuePath,
			[]byte(`{"agent_name":"b","idempotency_key":"k"}`))
		var response contract.SessionDeriveResponse
		decodeJSONResponse(t, recorder, &response)
		if recorder.Code != http.StatusOK || !response.Derived.ChildDeleted || response.Session != nil ||
			response.Derived.ChildSessionID != "sess-child" {
			t.Fatalf("status = %d response = %#v, want 200 child_deleted without session", recorder.Code, response)
		}
	})
}

func TestPreviewSessionDeriveHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should report the cut for a message and omit it for the whole session", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace", nil)
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodGet,
			"/api/workspaces/ws-workspace/sessions/sess-123/derive/preview?message_id=msg-3", nil)
		var withCut contract.SessionDerivePreviewResponse
		decodeJSONResponse(t, recorder, &withCut)
		if recorder.Code != http.StatusOK || withCut.Cut == nil || withCut.Cut.TurnID != "turn-7" ||
			!withCut.Cut.TurnSettled || withCut.NativeForkPossible || withCut.Transcript.MaxSequence != 418 ||
			withCut.SourceMessageCount != 60 {
			t.Fatalf("status = %d preview = %#v, want cut turn-7 of 60 source messages", recorder.Code, withCut)
		}
		recorder = performRequest(t, engine, http.MethodGet,
			"/api/workspaces/ws-workspace/sessions/sess-123/derive/preview", nil)
		var whole contract.SessionDerivePreviewResponse
		decodeJSONResponse(t, recorder, &whole)
		if recorder.Code != http.StatusOK || whole.Cut != nil || whole.MessageCount != 42 {
			t.Fatalf("status = %d preview = %#v, want whole-session preview without cut", recorder.Code, whole)
		}
	})

	t.Run("Should report a missing source with session_not_found from the manager", func(t *testing.T) {
		t.Parallel()

		manager := continueTestManager(t, "ws-workspace", nil)
		manager.DerivePreviewFn = func(context.Context, string, string, string) (session.DerivePreview, error) {
			return session.DerivePreview{}, session.ErrSessionNotFound
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performRequest(t, engine, http.MethodGet,
			"/api/workspaces/ws-workspace/sessions/sess-missing/derive/preview", nil)
		var payload contract.ErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if recorder.Code != http.StatusNotFound || payload.Code != "session_not_found" {
			t.Fatalf("status = %d code = %q, want 404 session_not_found; body=%s",
				recorder.Code, payload.Code, recorder.Body.String())
		}
	})
}

func forkedResult() session.DeriveResult {
	child := newSessionInfo("sess-fork")
	child.WorkspaceID = "ws-workspace"
	child.ProfileID = store.DefaultProfileID
	child.Lineage = &store.SessionLineage{
		ParentSessionID: "sess-123", RootSessionID: "sess-123", SpawnDepth: 1,
		Kind: store.LineageKindFork, OriginAgentName: "codex", OriginMessageID: "msg-3",
	}
	child.Derivation = &store.SessionDerivation{
		Kind: store.LineageKindFork, SourceSessionID: "sess-123", Seed: store.SessionDerivationSeedNativeFork,
		Native:      &store.SessionNativeBootstrap{ACPSessionID: "acp-clone", State: store.SessionNativeStatePending},
		FirstPrompt: store.SessionFirstPrompt{State: store.SessionDerivationFirstPromptStaged},
	}
	return session.DeriveResult{
		Child: child, ChildSessionID: "sess-fork", Kind: store.LineageKindFork, SourceSessionID: "sess-123",
		OriginAgentName: "codex", OriginMessageID: "msg-3", ThroughTurnID: "turn-3",
		Seed: session.DeriveSeedNativeFork, NativeState: store.SessionNativeStatePending, ACPSessionID: "acp-clone",
		ReplayMessageCount: 17, ReplayBytes: 23347, FirstPrompt: store.SessionDerivationFirstPromptStaged,
	}
}

func TestForkSessionHandler(t *testing.T) {
	t.Parallel()

	const forkPath = "/api/workspaces/ws-workspace/sessions/sess-123/fork"

	t.Run("Should create the fork through a message with the native pending shape", func(t *testing.T) {
		t.Parallel()

		var captured session.ForkSessionOpts
		manager := continueTestManager(t, "ws-workspace", nil)
		manager.ForkFn = func(_ context.Context, opts session.ForkSessionOpts) (session.DeriveResult, error) {
			captured = opts
			return forkedResult(), nil
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		body := []byte(`{"message_id":"msg-3","name":"alt","idempotency_key":"idem-fork",` +
			`"expected_epoch":3,"expected_generation":12,"expected_max_sequence":418}`)
		recorder := performRequest(t, engine, http.MethodPost, forkPath, body)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
		}
		if captured.SourceSessionID != "sess-123" || captured.WorkspaceID != "ws-workspace" ||
			captured.MessageID != "msg-3" || captured.Name != "alt" || captured.IdempotencyKey != "idem-fork" ||
			captured.Fences.ExpectedMaxSequence == nil || *captured.Fences.ExpectedMaxSequence != 418 ||
			captured.ProfileID != "" {
			t.Fatalf("ForkSession() opts = %#v, want routed request", captured)
		}
		var response contract.SessionDeriveResponse
		decodeJSONResponse(t, recorder, &response)
		derived := response.Derived
		if derived.Kind != "fork" || derived.OriginMessageID != "msg-3" || derived.ThroughTurnID != "turn-3" ||
			derived.Seed != "native_fork" || derived.NativeState != "pending" || derived.ACPSessionID != "acp-clone" ||
			derived.ReplayMessageCount == nil || *derived.ReplayMessageCount != 17 {
			t.Fatalf("response.derived = %#v, want the native pending fork outcome", derived)
		}
		if response.Session == nil || response.Session.Lineage == nil ||
			response.Session.Lineage.OriginMessageID != "msg-3" || response.Session.Derivation == nil ||
			response.Session.Derivation.NativeState != "pending" {
			t.Fatalf("response.session = %#v, want the fork lineage and native state", response.Session)
		}
	})

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "Should reject a missing idempotency key", body: `{"message_id":"msg-3"}`},
		{name: "Should reject partial fences", body: `{"idempotency_key":"k","expected_epoch":3}`},
		{name: "Should reject agent fields", body: `{"idempotency_key":"k","agent_name":"b"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := continueTestManager(t, "ws-workspace", nil)
			manager.ForkFn = func(context.Context, session.ForkSessionOpts) (session.DeriveResult, error) {
				t.Fatal("ForkSession() called for an invalid request")
				return session.DeriveResult{}, nil
			}
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performRequest(t, engine, http.MethodPost, forkPath, []byte(tc.body))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}

	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"Should map an unknown message", session.ErrDeriveMessageNotFound, http.StatusNotFound, "message_not_found"},
		{"Should map an unsettled cut turn", session.ErrDeriveTurnInProgress, http.StatusConflict, "session_turn_in_progress"},
		{"Should map a fence conflict", session.ErrDeriveFenceConflict, http.StatusConflict, "session_fence_conflict"},
		{"Should map a non-user source", session.ErrSessionNotDerivable, http.StatusBadRequest, "session_not_derivable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := continueTestManager(t, "ws-workspace", nil)
			manager.ForkFn = func(context.Context, session.ForkSessionOpts) (session.DeriveResult, error) {
				return session.DeriveResult{}, tc.err
			}
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performRequest(t, engine, http.MethodPost, forkPath, []byte(`{"idempotency_key":"k"}`))
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if recorder.Code != tc.status || payload.Code != tc.code {
				t.Fatalf("status = %d code = %q, want %d %q; body=%s",
					recorder.Code, payload.Code, tc.status, tc.code, recorder.Body.String())
			}
		})
	}
}

// deriveFirstMessageStartFailure wraps cause the way a derive's first-message admission
// reports a runtime start failure after the child was committed: the session and ACP
// chain plus the agent's stderr, which only the daemon log may show.
func deriveFirstMessageStartFailure(cause error) error {
	return fmt.Errorf("session: admit first message of derived session %q: %w", "sess-child",
		&acp.AcceptedStartError{Cause: fmt.Errorf(
			"acp: set session model for %q: %w: stderr=2026/09/29 INFO connection closed", "b", cause,
		)})
}

// deriveModelUnavailableError is the provider refusal of a derive's first message after
// the child was committed: a model_unavailable diagnostic over the negotiation failure.
func deriveModelUnavailableError() error {
	return diagnostics.NewStructuredError(
		diagnostics.NewItem(diagnostics.ItemSpec{
			ID: "provider.negotiation.model_unavailable", Code: contract.CodeModelUnavailable,
			Category: contract.CategoryProvider, Title: "Provider configuration is unavailable",
			Message: `acp: model "gone" is unavailable`, Severity: contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		}),
		&acp.NegotiationError{Code: contract.CodeModelUnavailable, Stage: "model", Requested: "gone"},
	)
}
