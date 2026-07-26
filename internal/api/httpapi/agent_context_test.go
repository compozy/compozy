package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
)

func TestAgentContextHTTPIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should return context for a daemon-validated HTTP agent session", func(t *testing.T) {
		t.Parallel()

		sessionInfo := newSessionInfo("sess-http-agent")
		sessionInfo.AgentName = "reviewer"
		sessionInfo.Provider = "codex"
		sessionInfo.Model = "gpt-5.4"
		sessionInfo.WorkspaceID = "ws-http"
		sessionInfo.Workspace = "/workspace/http"
		sessionInfo.State = session.StateActive

		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				if id != sessionInfo.ID {
					t.Fatalf("Status() id = %q, want %q", id, sessionInfo.ID)
				}
				return sessionInfo, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.MaskInternalErrors = false
		handlers.AgentContextService = httpAgentContextServiceFunc(
			func(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
				if info.ID != sessionInfo.ID || info.AgentName != sessionInfo.AgentName {
					t.Fatalf("ContextForSession() info = %#v, want validated HTTP caller", info)
				}
				return contract.AgentContextPayload{
					Self: contract.AgentIdentityPayload{
						SessionID: info.ID,
						AgentName: info.AgentName,
						Provider:  info.Provider,
						Model:     info.Model,
					},
					Workspace: contract.AgentWorkspacePayload{
						ID:      info.WorkspaceID,
						RootDir: info.Workspace,
					},
					Session: contract.AgentSessionPayload{
						ID:        info.ID,
						State:     info.State,
						CreatedAt: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
						UpdatedAt: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
					},
					Provenance: contract.AgentContextProvenancePayload{
						GeneratedAt: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
						Source:      "test",
					},
				}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performRequestWithHeaders(
			t,
			engine,
			http.MethodGet,
			"/api/agent/context",
			nil,
			map[string]string{
				agentidentity.HeaderSessionID: sessionInfo.ID,
				agentidentity.HeaderAgent:     sessionInfo.AgentName,
			},
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentContextResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Context.Self.SessionID != sessionInfo.ID ||
			response.Context.Self.AgentName != sessionInfo.AgentName ||
			response.Context.Workspace.ID != sessionInfo.WorkspaceID {
			t.Fatalf("context = %#v, want HTTP caller context", response.Context)
		}
	})

	t.Run("Should reject missing HTTP agent session identity", func(t *testing.T) {
		t.Parallel()

		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				t.Fatalf("Status() id = %q, want no lookup without session identity", id)
				return nil, nil
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.MaskInternalErrors = false
		handlers.AgentContextService = httpAgentContextServiceFunc(
			func(context.Context, *session.Info) (contract.AgentContextPayload, error) {
				t.Fatal("ContextForSession() called without validated identity")
				return contract.AgentContextPayload{}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performRequestWithHeaders(t, engine, http.MethodGet, "/api/agent/context", nil, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
		}

		var response contract.ErrorPayload
		decodeJSONResponse(t, recorder, &response)
		if !strings.Contains(response.Error, "AGH_SESSION_ID is required") {
			t.Fatalf("error body = %#v, want missing session identity guidance", response)
		}
	})

	t.Run("Should reject stale HTTP agent session identity", func(t *testing.T) {
		t.Parallel()

		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				if id != "sess-stale" {
					t.Fatalf("Status() id = %q, want stale identity lookup", id)
				}
				return nil, session.ErrSessionNotFound
			},
		}
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.MaskInternalErrors = false
		handlers.AgentContextService = httpAgentContextServiceFunc(
			func(context.Context, *session.Info) (contract.AgentContextPayload, error) {
				t.Fatal("ContextForSession() called with stale identity")
				return contract.AgentContextPayload{}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performRequestWithHeaders(
			t,
			engine,
			http.MethodGet,
			"/api/agent/context",
			nil,
			map[string]string{
				agentidentity.HeaderSessionID: "sess-stale",
				agentidentity.HeaderAgent:     "reviewer",
			},
		)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
		}

		var response contract.ErrorPayload
		decodeJSONResponse(t, recorder, &response)
		if !strings.Contains(response.Error, "agent session identity is not known to the daemon") {
			t.Fatalf("error body = %#v, want stale identity guidance", response)
		}
	})
}

type httpAgentContextServiceFunc func(context.Context, *session.Info) (contract.AgentContextPayload, error)

func (fn httpAgentContextServiceFunc) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	return fn(ctx, info)
}
