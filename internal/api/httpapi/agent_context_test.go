package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
		if !strings.Contains(response.Error, "COMPOZY_SESSION_ID is required") {
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

func TestAgentCrossWorkspaceHTTPIdentityMapping(t *testing.T) {
	t.Parallel()

	const (
		sourceWorkspaceID = "ws-source"
		targetWorkspaceID = "ws-target"
	)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-approve-reads" && id != "sess-approve-all" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:          id,
				AgentName:   "coder",
				WorkspaceID: sourceWorkspaceID,
				Workspace:   "/workspace/source",
				State:       session.StateActive,
			}, nil
		},
	}
	created := 0
	tasks := &stubTaskManager{
		CreateTaskFn: func(_ context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
			created++
			if spec.WorkspaceID != targetWorkspaceID || actor.Actor.Ref == "" {
				t.Fatalf("CreateTask() spec=%#v actor=%#v, want foreign workspace agent actor", spec, actor)
			}
			return &taskpkg.Task{
				ID:          "task-cross-workspace",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: targetWorkspaceID,
				Title:       spec.Title,
				Status:      taskpkg.TaskStatusPending,
				CreatedBy:   actor.Actor,
				Origin:      actor.Origin,
			}, nil
		},
	}
	workspaces := stubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "target" {
				t.Fatalf("Get() ref = %q, want target", ref)
			}
			return workspacepkg.Workspace{ID: targetWorkspaceID, Name: "target"}, nil
		},
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref != targetWorkspaceID {
				t.Fatalf("Resolve() ref = %q, want %q", ref, targetWorkspaceID)
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: targetWorkspaceID, Name: "target"},
				WorkspaceID: targetWorkspaceID,
			}, nil
		},
	}
	policyCalls := 0
	policy := apitestutil.StubWorkspaceAccessPolicy{
		AuthorizeFn: func(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
			policyCalls++
			if req.Actor.Kind != workspaceaccess.ActorAgentSession ||
				req.Actor.WorkspaceID != sourceWorkspaceID ||
				req.TargetWorkspaceID != targetWorkspaceID ||
				req.Seam != workspaceaccess.SeamIdentity {
				t.Fatalf("Authorize() request = %#v, want HTTP identity mapping", req)
			}
			return workspaceaccess.Decision{
				Allowed: req.Actor.SessionID == "sess-approve-all",
				Source:  workspaceaccess.SourcePermissionMode,
			}, nil
		},
	}
	handlers := newTestHandlersWithAutomationBridgesTasksAndWorkspace(
		t,
		manager,
		stubObserver{},
		nil,
		tasks,
		nil,
		workspaces,
		newTestHomePaths(t),
	)
	handlers.MaskInternalErrors = false
	handlers.WorkspaceAccess = policy
	engine := newTestRouter(t, handlers)
	body := []byte(`{"scope":"workspace","workspace":"target","title":"Cross workspace"}`)

	denied := performRequestWithHeaders(t, engine, http.MethodPost, "/api/tasks", body, map[string]string{
		agentidentity.HeaderSessionID: "sess-approve-reads",
		agentidentity.HeaderAgent:     "coder",
	})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("approve-reads status = %d, want %d; body=%s", denied.Code, http.StatusForbidden, denied.Body.String())
	}
	if !strings.Contains(denied.Body.String(), workspaceaccess.DenialHint) {
		t.Fatalf("approve-reads body = %s, want denial hint", denied.Body.String())
	}
	if created != 0 {
		t.Fatalf("CreateTask() calls after deny = %d, want 0", created)
	}

	allowed := performRequestWithHeaders(t, engine, http.MethodPost, "/api/tasks", body, map[string]string{
		agentidentity.HeaderSessionID: "sess-approve-all",
		agentidentity.HeaderAgent:     "coder",
	})
	if allowed.Code != http.StatusCreated {
		t.Fatalf("approve-all status = %d, want %d; body=%s", allowed.Code, http.StatusCreated, allowed.Body.String())
	}
	if created != 1 || policyCalls != 2 {
		t.Fatalf("created=%d policy_calls=%d, want 1 and 2", created, policyCalls)
	}
}

type httpAgentContextServiceFunc func(context.Context, *session.Info) (contract.AgentContextPayload, error)

func (fn httpAgentContextServiceFunc) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	return fn(ctx, info)
}
