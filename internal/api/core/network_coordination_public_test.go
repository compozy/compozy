package core_test

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
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func coordinationWorkspaceService() testutil.StubWorkspaceService {
	return testutil.StubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{ID: ref, Name: ref, RootDir: "/workspace"}, nil
		},
	}
}

type stubCoordinationCommands struct {
	getFn           func(context.Context, workspacepkg.CoordinationRef, taskpkg.ActorContext) (workspacepkg.CoordinationView, error)
	setFn           func(context.Context, workspacepkg.SetCoordination, taskpkg.ActorContext) (workspacepkg.CoordinationView, error)
	setInvitationFn func(context.Context, workspacepkg.SetInvitation, taskpkg.ActorContext) (workspacepkg.CoordinationView, error)
}

func (s *stubCoordinationCommands) Get(
	ctx context.Context,
	ref workspacepkg.CoordinationRef,
	actor taskpkg.ActorContext,
) (workspacepkg.CoordinationView, error) {
	if s.getFn == nil {
		return workspacepkg.CoordinationView{}, nil
	}
	return s.getFn(ctx, ref, actor)
}

func (s *stubCoordinationCommands) Set(
	ctx context.Context,
	cmd workspacepkg.SetCoordination,
	actor taskpkg.ActorContext,
) (workspacepkg.CoordinationView, error) {
	if s.setFn == nil {
		return workspacepkg.CoordinationView{}, nil
	}
	return s.setFn(ctx, cmd, actor)
}

func (s *stubCoordinationCommands) SetInvitation(
	ctx context.Context,
	cmd workspacepkg.SetInvitation,
	actor taskpkg.ActorContext,
) (workspacepkg.CoordinationView, error) {
	if s.setInvitationFn == nil {
		return workspacepkg.CoordinationView{}, nil
	}
	return s.setInvitationFn(ctx, cmd, actor)
}

func coordinationFixture(t *testing.T) handlerFixture {
	t.Helper()
	return newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		coordinationWorkspaceService(),
		nil,
		nil,
	)
}

func TestNetworkCoordinationHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should return scope provenance invitation and eligibility on GET", func(t *testing.T) {
		t.Parallel()

		fixture := coordinationFixture(t)
		updatedAt := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
		dismissedAt := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
		fixture.Handlers.Coordination = &stubCoordinationCommands{
			getFn: func(
				_ context.Context,
				ref workspacepkg.CoordinationRef,
				actor taskpkg.ActorContext,
			) (workspacepkg.CoordinationView, error) {
				if ref.WorkspaceID != "ws-alpha" || ref.ScopeKind != "task" ||
					ref.TaskID != "task-1" || ref.RunID != "run-1" {
					t.Fatalf("ref = %#v, want explicit task/run scope", ref)
				}
				if actor.Actor.Ref != "local-user" || !actor.Scope.Operator {
					t.Fatalf("actor = %#v, want trusted local operator", actor)
				}
				return workspacepkg.CoordinationView{
					Setting: workspacepkg.CoordinationSetting{
						WorkspaceID: "ws-alpha",
						ScopeKind:   "task",
						ScopeID:     "task-1",
						Enabled:     false,
						Revision:    3,
						UpdatedAt:   updatedAt,
						UpdatedBy:   "local-user",
					},
					Invitation: workspacepkg.CoordinationInvitation{
						WorkspaceID: "ws-alpha",
						ScopeKind:   "task",
						ScopeID:     "task-1",
						Dismissed:   true,
						DismissedAt: dismissedAt,
						DismissedBy: "local-user",
					},
					Eligibility: workspacepkg.CoordinationEligibility{
						Reason:      "dismissed",
						RunID:       "run-1",
						Coordinator: true,
						WorkerCount: 2,
					},
				}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-alpha/network-coordination?scope=task&task_id=task-1&run_id=run-1",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET coordination status = %d body=%s", resp.Code, resp.Body.String())
		}
		var body contract.NetworkCoordinationResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode coordination response: %v", err)
		}
		if body.Coordination.Scope != "task" || body.Coordination.TaskID != "task-1" ||
			body.Coordination.Revision != 3 || body.Coordination.UpdatedAt == nil {
			t.Fatalf("coordination = %#v, want task revision provenance", body.Coordination)
		}
		if body.Coordination.Invitation == nil || !body.Coordination.Invitation.Dismissed ||
			body.Coordination.Eligibility.Reason != "dismissed" {
			t.Fatalf("coordination = %#v, want persisted invitation eligibility", body.Coordination)
		}
	})

	t.Run("Should omit update timestamp for an absent coordination row", func(t *testing.T) {
		t.Parallel()

		fixture := coordinationFixture(t)
		fixture.Handlers.Coordination = &stubCoordinationCommands{
			getFn: func(
				_ context.Context,
				ref workspacepkg.CoordinationRef,
				_ taskpkg.ActorContext,
			) (workspacepkg.CoordinationView, error) {
				return workspacepkg.CoordinationView{
					Setting: workspacepkg.CoordinationSetting{
						WorkspaceID: ref.WorkspaceID,
						ScopeKind:   ref.ScopeKind,
						ScopeID:     ref.WorkspaceID,
					},
				}, nil
			},
		}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-alpha/network-coordination?scope=workspace",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET absent coordination status = %d body=%s", resp.Code, resp.Body.String())
		}
		if strings.Contains(resp.Body.String(), "updated_at") {
			t.Fatalf("body = %s, absent row must not invent updated_at", resp.Body.String())
		}
	})

	t.Run("Should compare-and-set task coordination through one command", func(t *testing.T) {
		t.Parallel()

		fixture := coordinationFixture(t)
		calls := 0
		fixture.Handlers.Coordination = &stubCoordinationCommands{
			setFn: func(
				_ context.Context,
				cmd workspacepkg.SetCoordination,
				actor taskpkg.ActorContext,
			) (workspacepkg.CoordinationView, error) {
				calls++
				if cmd.Ref.ScopeKind != "task" || cmd.Ref.TaskID != "task-1" ||
					cmd.Ref.RunID != "run-1" || !cmd.Enabled || cmd.ExpectedRevision != 4 {
					t.Fatalf("command = %#v, want task-scoped CAS enable", cmd)
				}
				if actor.Actor.Ref != "local-user" {
					t.Fatalf("actor = %#v, want server-derived local user", actor)
				}
				return workspacepkg.CoordinationView{Setting: workspacepkg.CoordinationSetting{
					WorkspaceID: "ws-alpha",
					ScopeKind:   "task",
					ScopeID:     "task-1",
					Enabled:     true,
					Revision:    5,
				}}, nil
			},
		}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/workspaces/ws-alpha/network-coordination",
			[]byte(`{"scope":"task","task_id":"task-1","run_id":"run-1","enabled":true,"expected_revision":4}`),
		)
		if resp.Code != http.StatusOK || calls != 1 {
			t.Fatalf("PUT coordination status/calls = %d/%d body=%s", resp.Code, calls, resp.Body.String())
		}
	})

	t.Run("Should reject omitted booleans revisions and unknown fields", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name string
			body string
		}{
			{name: "enabled", body: `{"scope":"workspace","expected_revision":0}`},
			{name: "expected revision", body: `{"scope":"workspace","enabled":true}`},
			{name: "unknown field", body: `{"scope":"workspace","enabled":true,"expected_revision":0,"network_channel":"builders"}`},
		} {
			t.Run("Should reject missing "+tc.name, func(t *testing.T) {
				t.Parallel()
				fixture := coordinationFixture(t)
				fixture.Handlers.Coordination = &stubCoordinationCommands{}
				resp := performRequest(
					t,
					fixture.Engine,
					http.MethodPut,
					"/workspaces/ws-alpha/network-coordination",
					[]byte(tc.body),
				)
				if resp.Code != http.StatusBadRequest {
					t.Fatalf("PUT invalid status = %d body=%s", resp.Code, resp.Body.String())
				}
			})
		}
	})

	t.Run("Should map unavailable and stale revisions to conflict", func(t *testing.T) {
		t.Parallel()

		for _, commandErr := range []error{participation.ErrUnavailable, workspacepkg.ErrCoordinationConflict} {
			fixture := coordinationFixture(t)
			fixture.Handlers.Coordination = &stubCoordinationCommands{
				setFn: func(
					_ context.Context,
					_ workspacepkg.SetCoordination,
					_ taskpkg.ActorContext,
				) (workspacepkg.CoordinationView, error) {
					return workspacepkg.CoordinationView{}, commandErr
				},
			}
			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodPut,
				"/workspaces/ws-alpha/network-coordination",
				[]byte(`{"scope":"workspace","enabled":true,"expected_revision":0}`),
			)
			if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), commandErr.Error()) {
				t.Fatalf("PUT conflict status = %d body=%s", resp.Code, resp.Body.String())
			}
		}
	})

	t.Run("Should dismiss task invitation with explicit run and revision", func(t *testing.T) {
		t.Parallel()

		fixture := coordinationFixture(t)
		fixture.Handlers.Coordination = &stubCoordinationCommands{
			setInvitationFn: func(
				_ context.Context,
				cmd workspacepkg.SetInvitation,
				actor taskpkg.ActorContext,
			) (workspacepkg.CoordinationView, error) {
				if cmd.Ref.ScopeKind != "task" || cmd.Ref.TaskID != "task-1" ||
					cmd.Ref.RunID != "run-1" || !cmd.Dismissed || cmd.ExpectedRevision != 3 {
					t.Fatalf("command = %#v, want task invitation CAS", cmd)
				}
				return workspacepkg.CoordinationView{
					Setting: workspacepkg.CoordinationSetting{
						WorkspaceID: "ws-alpha",
						ScopeKind:   "task",
						ScopeID:     "task-1",
						Revision:    4,
					},
					Invitation: workspacepkg.CoordinationInvitation{
						WorkspaceID: "ws-alpha",
						ScopeKind:   "task",
						ScopeID:     "task-1",
						Dismissed:   true,
						DismissedBy: actor.Actor.Ref,
					},
				}, nil
			},
		}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/workspaces/ws-alpha/network-coordination/invitation",
			[]byte(`{"scope":"task","task_id":"task-1","run_id":"run-1","dismissed":true,"expected_revision":3}`),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("PUT invitation status = %d body=%s", resp.Code, resp.Body.String())
		}
	})

	t.Run("Should map workspace not found before reading coordination", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				},
			},
			nil,
			nil,
		)
		fixture.Handlers.Coordination = &stubCoordinationCommands{}
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/missing/network-coordination?scope=workspace",
			nil,
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("GET missing workspace status = %d body=%s", resp.Code, resp.Body.String())
		}
	})
}

func TestNetworkWorkspaceAccessMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("Should authorize every agent-authenticated Network route before the handler", func(t *testing.T) {
		t.Parallel()

		const (
			sourceWorkspaceID = "ws_0000000000000000"
			targetWorkspaceID = "ws_0000000000000001"
		)
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return &session.Info{
					ID:          id,
					AgentName:   "coder",
					WorkspaceID: sourceWorkspaceID,
					State:       session.StateActive,
				}, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				if ref != targetWorkspaceID {
					t.Fatalf("Get() ref = %q, want %q", ref, targetWorkspaceID)
				}
				return workspacepkg.Workspace{ID: targetWorkspaceID}, nil
			},
		}
		policyCalls := 0
		handlers := &core.BaseHandlers{
			TransportName: "test",
			Sessions:      manager,
			Workspaces:    workspaces,
			WorkspaceAccess: testutil.StubWorkspaceAccessPolicy{
				AuthorizeFn: func(
					_ context.Context,
					req workspaceaccess.Request,
				) (workspaceaccess.Decision, error) {
					policyCalls++
					if req.Actor.Kind != workspaceaccess.ActorAgentSession ||
						req.Actor.WorkspaceID != sourceWorkspaceID ||
						req.TargetWorkspaceID != targetWorkspaceID ||
						req.Seam != workspaceaccess.SeamCoordination {
						t.Fatalf("Authorize() request = %#v, want canonical Network workspace decision", req)
					}
					return workspaceaccess.Decision{
						Allowed: req.Actor.SessionID == "sess-approve-all",
						Source:  workspaceaccess.SourcePermissionMode,
					}, nil
				},
			},
		}
		engine := gin.New()
		networkRoutes := engine.Group("/workspaces/:workspace_id/network")
		networkRoutes.Use(handlers.AuthorizeNetworkWorkspaceAccess)
		handlerCalls := 0
		networkRoutes.GET("/probe", func(c *gin.Context) {
			handlerCalls++
			c.Status(http.StatusNoContent)
		})

		request := func(sessionID string, withCredentials bool) *httptest.ResponseRecorder {
			t.Helper()
			req := httptest.NewRequest(
				http.MethodGet,
				"/workspaces/"+targetWorkspaceID+"/network/probe",
				http.NoBody,
			)
			if withCredentials {
				req.Header.Set(agentidentity.HeaderSessionID, sessionID)
				req.Header.Set(agentidentity.HeaderAgent, "coder")
			}
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)
			return resp
		}

		denied := request("sess-deny-all", true)
		if denied.Code != http.StatusForbidden || handlerCalls != 0 {
			t.Fatalf("denied status/handler calls = %d/%d, want 403/0; body=%s", denied.Code, handlerCalls, denied.Body)
		}
		allowed := request("sess-approve-all", true)
		if allowed.Code != http.StatusNoContent || handlerCalls != 1 {
			t.Fatalf("allowed status/handler calls = %d/%d, want 204/1", allowed.Code, handlerCalls)
		}
		operator := request("", false)
		if operator.Code != http.StatusNoContent || handlerCalls != 2 || policyCalls != 2 {
			t.Fatalf(
				"operator status/handler/policy calls = %d/%d/%d, want 204/2/2",
				operator.Code,
				handlerCalls,
				policyCalls,
			)
		}
	})
}
