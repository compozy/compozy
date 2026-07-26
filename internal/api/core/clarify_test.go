package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestClarifyCoreHandlersAuthorizeLiveSessionScope(t *testing.T) {
	t.Parallel()

	t.Run("Should list and answer only through the authorized session scope", func(t *testing.T) {
		t.Parallel()

		broker := &clarifyBrokerStub{}
		engine := newClarifyCoreRouter(broker)
		list := performClarifyCoreRequest(
			t,
			engine,
			http.MethodGet,
			"/api/workspaces/ws-one/sessions/session-one/clarifications",
			nil,
		)
		if list.Code != http.StatusOK {
			t.Fatalf(
				"list status = %d, want %d; body=%s",
				list.Code,
				http.StatusOK,
				list.Body.String(),
			)
		}
		var payload contract.ClarificationsResponse
		if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode list response error = %v", err)
		}
		if len(payload.Clarifications) != 1 || payload.Clarifications[0].RequestID != "request-one" {
			t.Fatalf("list response = %#v, want request-one", payload)
		}
		if broker.pendingScope.WorkspaceID != "ws-one" ||
			broker.pendingScope.SessionID != "session-one" ||
			broker.pendingScope.AgentName != "coder" {
			t.Fatalf("Pending() scope = %#v, want trusted session scope", broker.pendingScope)
		}

		choice := 0
		answer := performClarifyCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-one/sessions/session-one/clarifications/request-one/answer",
			[]byte(`{"choice_index":0}`),
		)
		if answer.Code != http.StatusOK {
			t.Fatalf(
				"answer status = %d, want %d; body=%s",
				answer.Code,
				http.StatusOK,
				answer.Body.String(),
			)
		}
		if broker.answerScope != broker.pendingScope || broker.requestID != "request-one" ||
			broker.answerRequest.ChoiceIndex == nil || *broker.answerRequest.ChoiceIndex != choice {
			t.Fatalf(
				"Answer() = scope %#v request %q body %#v, want trusted choice",
				broker.answerScope,
				broker.requestID,
				broker.answerRequest,
			)
		}
	})

	t.Run("Should reject a foreign workspace before consulting the broker", func(t *testing.T) {
		t.Parallel()

		broker := &clarifyBrokerStub{}
		response := performClarifyCoreRequest(
			t,
			newClarifyCoreRouter(broker),
			http.MethodGet,
			"/api/workspaces/ws-foreign/sessions/session-one/clarifications",
			nil,
		)
		if response.Code != http.StatusNotFound {
			t.Fatalf(
				"foreign list status = %d, want %d; body=%s",
				response.Code,
				http.StatusNotFound,
				response.Body.String(),
			)
		}
		assertClarifyErrorContains(t, response, errWorkspaceScopedResourceNotFound.Error())
		if broker.pendingCalls != 0 {
			t.Fatalf("Pending() calls = %d, want 0", broker.pendingCalls)
		}
	})

	t.Run("Should reject unknown and ambiguous answer fields", func(t *testing.T) {
		t.Parallel()

		broker := &clarifyBrokerStub{}
		engine := newClarifyCoreRouter(broker)
		unknown := performClarifyCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-one/sessions/session-one/clarifications/request-one/answer",
			[]byte(`{"text":"yes","workspace_id":"ws-foreign"}`),
		)
		if unknown.Code != http.StatusBadRequest {
			t.Fatalf(
				"unknown-field status = %d, want %d; body=%s",
				unknown.Code,
				http.StatusBadRequest,
				unknown.Body.String(),
			)
		}
		assertClarifyErrorContains(t, unknown, ErrUnknownJSONField.Error(), "workspace_id")
		ambiguous := performClarifyCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-one/sessions/session-one/clarifications/request-one/answer",
			[]byte(`{"choice_index":0,"text":"yes"}`),
		)
		if ambiguous.Code != http.StatusBadRequest {
			t.Fatalf(
				"ambiguous status = %d, want %d; body=%s",
				ambiguous.Code,
				http.StatusBadRequest,
				ambiguous.Body.String(),
			)
		}
		assertClarifyErrorContains(
			t,
			ambiguous,
			toolspkg.ErrClarifyInvalid.Error(),
			"exactly one of choice_index or text",
		)
	})

	t.Run("Should classify typed input and internal broker failures", func(t *testing.T) {
		t.Parallel()

		invalid := performClarifyCoreRequest(
			t,
			newClarifyCoreRouter(&clarifyBrokerStub{answerErr: toolspkg.ErrClarifyInvalid}),
			http.MethodPost,
			"/api/workspaces/ws-one/sessions/session-one/clarifications/request-one/answer",
			[]byte(`{"text":"yes"}`),
		)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf(
				"invalid broker status = %d, want %d; body=%s",
				invalid.Code,
				http.StatusBadRequest,
				invalid.Body.String(),
			)
		}
		assertClarifyErrorContains(t, invalid, toolspkg.ErrClarifyInvalid.Error())

		internal := performClarifyCoreRequest(
			t,
			newClarifyCoreRouter(&clarifyBrokerStub{pendingErr: errors.New("storage unavailable")}),
			http.MethodGet,
			"/api/workspaces/ws-one/sessions/session-one/clarifications",
			nil,
		)
		if internal.Code != http.StatusInternalServerError {
			t.Fatalf(
				"internal broker status = %d, want %d; body=%s",
				internal.Code,
				http.StatusInternalServerError,
				internal.Body.String(),
			)
		}
		assertClarifyErrorContains(t, internal, "storage unavailable")
	})
}

func assertClarifyErrorContains(
	t *testing.T,
	response *httptest.ResponseRecorder,
	want ...string,
) {
	t.Helper()
	var payload contract.ErrorPayload
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	for _, fragment := range want {
		if !strings.Contains(payload.Error, fragment) {
			t.Fatalf("error payload = %q, want fragment %q", payload.Error, fragment)
		}
	}
}

func newClarifyCoreRouter(broker *clarifyBrokerStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	manager := sessionManagerStub{
		status: func(_ context.Context, id string) (*session.Info, error) {
			return &session.Info{ID: id, WorkspaceID: "ws-one", AgentName: "coder"}, nil
		},
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName: "core-test",
		Sessions:      manager,
		Workspaces: workspaceResolveServiceStub{
			resolve: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: ref, Name: ref, RootDir: "/tmp/" + ref},
					WorkspaceID: ref,
				}, nil
			},
		},
		Clarify: broker,
	})
	engine := gin.New()
	engine.GET(
		"/api/workspaces/:workspace_id/sessions/:session_id/clarifications",
		handlers.ListSessionClarifications,
	)
	engine.POST(
		"/api/workspaces/:workspace_id/sessions/:session_id/clarifications/:request_id/answer",
		handlers.AnswerSessionClarification,
	)
	return engine
}

func performClarifyCoreRequest(
	t *testing.T,
	engine *gin.Engine,
	method string,
	path string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

type clarifyBrokerStub struct {
	pendingCalls  int
	pendingScope  toolspkg.Scope
	answerScope   toolspkg.Scope
	requestID     string
	answerRequest toolspkg.ClarifyAnswerRequest
	pendingErr    error
	answerErr     error
}

func (*clarifyBrokerStub) Ask(
	context.Context,
	toolspkg.Scope,
	toolspkg.ClarifyQuestion,
) (toolspkg.ClarifyAnswer, error) {
	return toolspkg.ClarifyAnswer{}, nil
}

func (s *clarifyBrokerStub) Pending(
	_ context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.ClarifyPending, error) {
	s.pendingCalls++
	s.pendingScope = scope
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	now := time.Now().UTC()
	return []toolspkg.ClarifyPending{{
		RequestID:   "request-one",
		WorkspaceID: "ws-one",
		SessionID:   "session-one",
		AgentName:   "coder",
		Question:    "Continue?",
		AskedAt:     now,
		Deadline:    now.Add(time.Minute),
	}}, nil
}

func (s *clarifyBrokerStub) Answer(
	_ context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswer, error) {
	s.answerScope = scope
	s.requestID = requestID
	s.answerRequest = request
	if s.answerErr != nil {
		return toolspkg.ClarifyAnswer{}, s.answerErr
	}
	return request.Normalize(toolspkg.ClarifyQuestion{Question: "Continue?", Choices: []string{"Yes"}})
}
