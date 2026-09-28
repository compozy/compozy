package udsapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type agentContextServiceFunc func(context.Context, *session.Info) (contract.AgentContextPayload, error)
type agentCoordinatorRoleResolverFunc func(context.Context, string) (compozyconfig.ResolvedCoordinatorRole, error)

func TestAgentContextReturnsSituationPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return the situation payload for the caller session", func(t *testing.T) {
		t.Parallel()

		manager := activeAgentSessionManager(t)
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.AgentContextService = agentContextServiceFunc(
			func(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
				if info.ID != "sess-agent" || info.AgentName != "coder" {
					t.Fatalf("ContextForSession() info = %#v, want caller session", info)
				}
				return contract.AgentContextPayload{
					Self: contract.AgentIdentityPayload{
						SessionID: info.ID,
						AgentName: info.AgentName,
						Provider:  info.Provider,
					},
					Workspace: contract.AgentWorkspacePayload{ID: info.WorkspaceID, RootDir: info.Workspace},
					Session: contract.AgentSessionPayload{
						ID:    info.ID,
						State: info.State,

						CreatedAt: info.CreatedAt,
						UpdatedAt: info.UpdatedAt,
					},
					Task: contract.AgentTaskContextPayload{Available: true},

					Capabilities: contract.AgentCapabilitySectionPayload{},
					Limits:       contract.AgentLimitsPayload{ContextSectionLimit: 20},
					Provenance: contract.AgentContextProvenancePayload{
						GeneratedAt: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
						Source:      "test",
					},
				}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/context",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentContextResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Context.Self.SessionID != "sess-agent" ||
			response.Context.Workspace.ID != "ws-1" ||
			!response.Context.Task.Available {
			t.Fatalf("context = %#v, want validated situation payload", response.Context)
		}
	})
}

func TestAgentCoordinatorRoleRouteReturnsResolvedPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return resolved workspace coordinator payload", func(t *testing.T) {
		t.Parallel()

		manager := activeAgentSessionManager(t)
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.CoordinatorRole = agentCoordinatorRoleResolverFunc(
			func(_ context.Context, workspaceID string) (compozyconfig.ResolvedCoordinatorRole, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ResolveCoordinatorRole() workspaceID = %q, want ws-1", workspaceID)
				}
				return compozyconfig.ResolvedCoordinatorRole{
					Enabled:                       true,
					AgentName:                     "coordinator",
					Provider:                      "codex",
					Model:                         "gpt-4o",
					TTL:                           45 * time.Minute,
					MaxChildren:                   5,
					MaxActiveSessionsPerWorkspace: 5,
				}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/coordinator/config",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentCoordinatorConfigResponse
		decodeJSONResponse(t, recorder, &response)
		if !response.Coordinator.Enabled ||
			response.Coordinator.AgentName != "coordinator" ||
			response.Coordinator.DefaultTTLSeconds != 2700 ||
			response.Coordinator.Source != contract.CoordinatorConfigSourceWorkspace ||
			response.Coordinator.WorkspaceID != "ws-1" {
			t.Fatalf("coordinator = %#v, want workspace coordinator payload", response.Coordinator)
		}
	})
}

func (f agentContextServiceFunc) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	return f(ctx, info)
}

func (f agentCoordinatorRoleResolverFunc) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (compozyconfig.ResolvedCoordinatorRole, error) {
	return f(ctx, workspaceID)
}

func activeAgentSessionManager(t *testing.T) stubSessionManager {
	t.Helper()

	return stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-agent" {
				return nil, session.ErrSessionNotFound
			}
			now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
			return &session.Info{
				ID:          "sess-agent",
				ProfileID:   store.DefaultProfileID,
				Name:        "worker",
				AgentName:   "coder",
				Provider:    "test-provider",
				WorkspaceID: "ws-1",
				Workspace:   "/workspace/project",

				Type:      session.SessionTypeUser,
				State:     session.StateActive,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
}

func performAgentKernelRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, path, bytesReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func agentKernelHeaders() map[string]string {
	return map[string]string{
		agentidentity.HeaderSessionID: "sess-agent",
		agentidentity.HeaderAgent:     "coder",
	}
}

func bytesReader(body []byte) *bytes.Reader {
	if body == nil {
		return bytes.NewReader(nil)
	}
	return bytes.NewReader(body)
}
