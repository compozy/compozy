package udsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestAgentMeRejectsInvalidCallerIdentity(t *testing.T) {
	t.Parallel()

	active := &session.Info{
		ID:          "sess-1",
		AgentName:   "coder",
		Provider:    "test-provider",
		WorkspaceID: "ws-1",
		Workspace:   "/workspace",
		State:       session.StateActive,
		CreatedAt:   time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 26, 10, 0, 1, 0, time.UTC),
	}

	tests := []struct {
		name       string
		headers    map[string]string
		statusInfo *session.Info
		statusErr  error
		wantStatus int
	}{
		{
			name:       "Should reject missing env headers",
			headers:    map[string]string{},
			statusInfo: active,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Should reject stopped sessions",
			headers: map[string]string{
				agentidentity.HeaderSessionID: "sess-1",
				agentidentity.HeaderAgent:     "coder",
			},
			statusInfo: func() *session.Info {
				info := *active
				info.State = session.StateStopped
				return &info
			}(),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Should reject agent mismatches",
			headers: map[string]string{
				agentidentity.HeaderSessionID: "sess-1",
				agentidentity.HeaderAgent:     "reviewer",
			},
			statusInfo: active,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Should preserve lookup unavailable status",
			headers: map[string]string{
				agentidentity.HeaderSessionID: "sess-1",
				agentidentity.HeaderAgent:     "coder",
			},
			statusErr:  context.DeadlineExceeded,
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manager := stubSessionManager{
				StatusFn: func(_ context.Context, id string) (*session.Info, error) {
					if tt.statusErr != nil {
						return nil, tt.statusErr
					}
					if tt.statusInfo == nil {
						return nil, session.ErrSessionNotFound
					}
					if id != tt.statusInfo.ID {
						return nil, session.ErrSessionNotFound
					}
					return tt.statusInfo, nil
				},
			}
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
			recorder := performAgentMeRequest(t, engine, tt.headers)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error == "" {
				t.Fatalf("error payload = %#v, want actionable error", payload)
			}
		})
	}
}

func TestAgentMeReturnsValidatedCallerIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should return validated caller identity", func(t *testing.T) {
		t.Parallel()

		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				if id != "sess-1" {
					return nil, session.ErrSessionNotFound
				}
				now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
				return &session.Info{
					ID:                   "sess-1",
					Name:                 "worker",
					AgentName:            "coder",
					Provider:             "test-provider",
					Model:                "test-model",
					WorkspaceID:          "ws-1",
					Workspace:            "/workspace",
					NetworkParticipation: udsTestLiveParticipation("ws-1", "coord"),
					Type:                 session.SessionTypeUser,
					State:                session.StateActive,
					CreatedAt:            now,
					UpdatedAt:            now,
				}, nil
			},
		}
		engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t)))
		recorder := performAgentMeRequest(t, engine, map[string]string{
			agentidentity.HeaderSessionID: "sess-1",
			agentidentity.HeaderAgent:     "coder",
		})
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentMeResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Me.Self.SessionID != "sess-1" ||
			response.Me.Self.AgentName != "coder" ||
			response.Me.Self.Model != "test-model" {
			t.Fatalf("response.Me.Self = %#v, want validated caller with model", response.Me.Self)
		}
		if response.Me.Session.State != session.StateActive || response.Me.Workspace.ID != "ws-1" {
			encoded, err := json.Marshal(response.Me)
			if err != nil {
				t.Fatalf("json.Marshal(AgentMePayload) error = %v", err)
			}
			t.Fatalf("response.Me = %s, want active session in workspace ws-1", encoded)
		}
	})
}

func TestAgentMeReportsUnavailableWhenSessionServiceMissing(t *testing.T) {
	t.Parallel()

	t.Run("Should return service unavailable when session service is missing", func(t *testing.T) {
		t.Parallel()

		engine := newTestRouter(t, newTestHandlers(t, nil, stubObserver{}, newTestHomePaths(t)))
		recorder := performAgentMeRequest(t, engine, map[string]string{
			agentidentity.HeaderSessionID: "sess-1",
			agentidentity.HeaderAgent:     "coder",
		})
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusServiceUnavailable,
				recorder.Body.String(),
			)
		}
	})
}

func TestAgentCrossWorkspaceUDSIdentityMapping(t *testing.T) {
	t.Parallel()

	t.Run("Should map cross-workspace UDS identity permissions", func(t *testing.T) {
		t.Parallel()

		const (
			sourceWorkspaceID = "ws-source"
			targetWorkspaceID = "ws-target"
		)
		manager := stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				if id != "sess-approve-reads" {
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
		tasks := &stubTaskManager{
			CreateTaskFn: func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("CreateTask() called after a denied UDS identity decision")
				return nil, nil
			},
		}
		workspaces := stubWorkspaceService{
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != "target" && ref != targetWorkspaceID {
					t.Fatalf("Get() ref = %q, want target or %q", ref, targetWorkspaceID)
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
		policyRequests := make([]workspaceaccess.Request, 0, 2)
		handlers := newTestHandlersWithRuntime(
			t,
			manager,
			stubObserver{},
			nil,
			tasks,
			nil,
			workspaces,
			nil,
			newTestHomePaths(t),
		)
		handlers.MaskInternalErrors = false
		handlers.WorkspaceAccess = apitestutil.StubWorkspaceAccessPolicy{
			AuthorizeFn: func(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
				policyRequests = append(policyRequests, req)
				if req.Actor.Kind != workspaceaccess.ActorAgentSession ||
					req.Actor.SessionID != "sess-approve-reads" ||
					req.Actor.WorkspaceID != sourceWorkspaceID ||
					req.TargetWorkspaceID != targetWorkspaceID {
					t.Fatalf("Authorize() request = %#v, want UDS identity mapping", req)
				}
				return workspaceaccess.Decision{Source: workspaceaccess.SourceDenied}, nil
			},
		}
		engine := newTestRouter(t, handlers)
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/api/tasks",
			strings.NewReader(`{"scope":"workspace","workspace":"target","title":"Cross workspace"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(agentidentity.HeaderSessionID, "sess-approve-reads")
		req.Header.Set(agentidentity.HeaderAgent, "coder")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), workspaceaccess.DenialHint) {
			t.Fatalf("body = %s, want denial hint", recorder.Body.String())
		}
		if len(policyRequests) != 1 || policyRequests[0].Seam != workspaceaccess.SeamIdentity {
			t.Fatalf("task policy requests = %#v, want identity seam", policyRequests)
		}

		coordinationReq := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/workspaces/"+targetWorkspaceID+"/network-coordination",
			http.NoBody,
		)
		coordinationReq.Header.Set(agentidentity.HeaderSessionID, "sess-approve-reads")
		coordinationReq.Header.Set(agentidentity.HeaderAgent, "coder")
		coordinationRecorder := httptest.NewRecorder()
		engine.ServeHTTP(coordinationRecorder, coordinationReq)
		if coordinationRecorder.Code != http.StatusForbidden {
			t.Fatalf(
				"coordination status = %d, want %d; body=%s",
				coordinationRecorder.Code,
				http.StatusForbidden,
				coordinationRecorder.Body.String(),
			)
		}
		if !strings.Contains(coordinationRecorder.Body.String(), workspaceaccess.DenialHint) {
			t.Fatalf("coordination body = %s, want denial hint", coordinationRecorder.Body.String())
		}
		if len(policyRequests) != 2 || policyRequests[1].Seam != workspaceaccess.SeamCoordination {
			t.Fatalf("coordination policy requests = %#v, want coordination seam", policyRequests)
		}
	})
}

func performAgentMeRequest(t *testing.T, engine http.Handler, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/agent/me", http.NoBody)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}
