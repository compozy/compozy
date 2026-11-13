package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/observe/attention"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestBaseHandlersSessionAttentionSurfaces(t *testing.T) {
	t.Parallel()

	t.Run("Should acquire renew and release the caller presence lease", func(t *testing.T) {
		t.Parallel()

		type presenceCall struct {
			leaseID string
			visible bool
		}
		calls := make([]presenceCall, 0, 3)
		manager := attentionRouteSessionManager()
		manager.Attention.PresenceFn = func(
			_ context.Context,
			sessionID string,
			leaseID string,
			visible bool,
		) (string, error) {
			if sessionID != "sess-attention" {
				t.Fatalf("SessionPresence() sessionID = %q, want sess-attention", sessionID)
			}
			calls = append(calls, presenceCall{leaseID: leaseID, visible: visible})
			if leaseID == "" {
				return "lease-1", nil
			}
			return leaseID, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		path := "/workspaces/ws-1/sessions/sess-attention/presence"

		acquire := performRequest(t, fixture.Engine, http.MethodPost, path, []byte(`{"visible":true}`))
		if acquire.Code != http.StatusOK {
			t.Fatalf("presence acquire status = %d, want 200; body=%s", acquire.Code, acquire.Body.String())
		}
		var lease contract.SessionPresenceResponse
		testutil.DecodeJSONResponse(t, acquire, &lease)
		if lease.LeaseID != "lease-1" {
			t.Fatalf("presence lease = %q, want lease-1", lease.LeaseID)
		}

		renew := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			path,
			[]byte(`{"visible":true,"lease_id":"lease-1"}`),
		)
		if renew.Code != http.StatusNoContent {
			t.Fatalf("presence renew status = %d, want 204; body=%s", renew.Code, renew.Body.String())
		}
		release := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			path,
			[]byte(`{"visible":false,"lease_id":"lease-1"}`),
		)
		if release.Code != http.StatusNoContent {
			t.Fatalf("presence release status = %d, want 204; body=%s", release.Code, release.Body.String())
		}
		if len(calls) != 3 || calls[0].leaseID != "" || !calls[0].visible ||
			calls[1].leaseID != "lease-1" || !calls[1].visible ||
			calls[2].leaseID != "lease-1" || calls[2].visible {
			t.Fatalf("presence calls = %#v, want acquire, renew, release", calls)
		}
	})

	t.Run("Should distinguish canceled presence from a server failure", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name   string
			err    error
			status int
		}{
			{name: "Should report client cancellation", err: context.Canceled, status: 499},
			{name: "Should preserve genuine server failures", err: fmt.Errorf("presence store unavailable"), status: http.StatusInternalServerError},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				manager := attentionRouteSessionManager()
				manager.Attention.PresenceFn = func(context.Context, string, string, bool) (string, error) {
					return "", test.err
				}
				fixture := newHandlerFixture(
					t,
					manager,
					testutil.StubObserver{},
					testutil.StubWorkspaceService{},
				)
				response := performRequest(t, fixture.Engine, http.MethodPost,
					"/workspaces/ws-1/sessions/sess-attention/presence", []byte(`{"visible":true}`))
				assertAPIErrorResponse(t, response, test.status, test.err.Error())
			})
		}
	})

	t.Run("Should list a sanitized canonical interaction projection", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.PendingFn = func(
			_ context.Context,
			sessionID string,
			statuses []string,
		) ([]store.PendingInteraction, error) {
			if sessionID != "sess-attention" || len(statuses) != 1 || statuses[0] != "pending" {
				t.Fatalf("PendingInteractions() session/statuses = %q/%#v", sessionID, statuses)
			}
			return []store.PendingInteraction{{
				InteractionID: "interaction-1", SessionID: sessionID,
				Kind: store.PendingInteractionKindPermission, ProviderRequestID: "request-1",
				Title: "Deploy with compozy_claim_secret123", Payload: store.PendingInteractionPayload{
					Choices: []string{"safe", "compozy_claim_secret123"},
					ToolID:  "compozy__terminal_exec",
				},
				Status:    store.PendingInteractionStatusPending,
				CreatedAt: time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC),
			}}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-attention/interactions?status=pending",
			nil,
		)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "compozy_claim_secret123") {
			t.Fatalf("interactions status/body = %d/%s, want sanitized 200", response.Code, response.Body.String())
		}
		var payload contract.SessionInteractionsResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if len(payload.Interactions) != 1 || payload.Interactions[0].InteractionID != "interaction-1" ||
			payload.Interactions[0].Status != store.PendingInteractionStatusPending ||
			payload.Interactions[0].ToolID != "compozy__terminal_exec" {
			t.Fatalf("interactions payload = %#v, want one canonical pending row", payload)
		}
	})

	t.Run("Should return exact totals and workspace counts", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.SummaryFn = func(
			_ context.Context,
			readScope store.ReadScope,
		) (store.SessionAttentionSummary, error) {
			if readScope.ProfileID != "" || !readScope.AllProfiles {
				t.Fatalf("AttentionSummary() read scope = %#v, want all profiles", readScope)
			}
			return store.SessionAttentionSummary{
				TerminalApprovals: []store.TerminalApprovalCount{
					{WorkspaceID: "ws-1", ProfileID: "profile-work", Count: 110},
					{WorkspaceID: "ws-1", ProfileID: "profile-personal", Count: 3},
				},
				NeedsYou: 101,
				Finished: 17,
				ByWorkspace: []store.WorkspaceAttentionSummary{
					{WorkspaceID: "ws-1", NeedsYou: 60, Finished: 7},
					{WorkspaceID: "ws-2", NeedsYou: 41, Finished: 10},
				},
			}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		response := performRequest(t, fixture.Engine, http.MethodGet, "/sessions/attention-summary", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("attention summary status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionAttentionSummaryPayload
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.NeedsYou != 101 || payload.Finished != 17 || len(payload.ByWorkspace) != 2 {
			t.Fatalf("attention summary payload = %#v, want exact aggregate", payload)
		}
		if len(payload.TerminalApprovals) != 2 ||
			payload.TerminalApprovals[0] != (contract.TerminalApprovalCountPayload{WorkspaceID: "ws-1", ProfileID: "profile-work", Count: 110}) ||
			payload.TerminalApprovals[1].ProfileID != "profile-personal" || payload.TerminalApprovals[1].Count != 3 {
			t.Fatalf(
				"terminal approval scopes = %#v, want exact workspace/profile intersections",
				payload.TerminalApprovals,
			)
		}
	})
}

func TestBaseHandlersSessionWait(t *testing.T) {
	t.Parallel()

	t.Run("Should return the normalized state outcome and revision fence", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.WaitFn = func(
			_ context.Context,
			request session.WaitRequest,
		) (session.WaitOutcome, error) {
			if request.SessionID != "sess-attention" || request.Timeout != 2*time.Minute ||
				request.Epoch != 7 || request.ResumeID != "resume-1" ||
				len(request.Until) != 2 || request.Until[0] != session.BadgeWaitingForInput ||
				request.Until[1] != session.BadgeIdle {
				t.Fatalf("WaitForBadge() request = %#v, want normalized route request", request)
			}
			return session.WaitOutcome{
				Outcome:  session.WaitResultStateReached,
				State:    session.BadgeWaitingForInput,
				Waited:   38 * time.Second,
				Revision: 19,
			}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-1/sessions/sess-attention/wait",
			[]byte(
				`{"until":["waiting-for-input","idle","idle"],"timeout_ms":120000,"epoch":7,"resume_id":"resume-1"}`,
			),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("session wait status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionWaitResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.SessionID != "sess-attention" || payload.Outcome != "state-reached" ||
			payload.State != "waiting-for-input" || payload.WaitedMS != 38_000 || payload.Revision != 19 ||
			len(payload.Until) != 2 {
			t.Fatalf("session wait payload = %#v, want state outcome with fence", payload)
		}
	})

	t.Run("Should return timeout as a resumable successful outcome", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.WaitFn = func(
			_ context.Context,
			request session.WaitRequest,
		) (session.WaitOutcome, error) {
			if len(request.Until) != 5 || request.Until[0] != session.BadgeWaitingForInput ||
				request.Until[4] != session.BadgeFailed {
				t.Fatalf("WaitForBadge() default until = %#v, want settled set", request.Until)
			}
			return session.WaitOutcome{
				Outcome:  session.WaitResultTimeout,
				Waited:   request.Timeout,
				Revision: 23,
				ResumeID: "wait-23",
			}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-1/sessions/sess-attention/wait",
			[]byte(`{"timeout_ms":5000}`),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("session wait timeout status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionWaitResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.Outcome != "timeout" || payload.WaitedMS != 5_000 ||
			payload.ResumeID != "wait-23" || payload.Revision != 23 || len(payload.Until) != 5 {
			t.Fatalf("session wait timeout payload = %#v, want resumable 200", payload)
		}
	})

	t.Run("Should reject invalid state and timeout before registering", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.WaitFn = func(
			context.Context,
			session.WaitRequest,
		) (session.WaitOutcome, error) {
			t.Fatal("WaitForBadge() called for invalid request")
			return session.WaitOutcome{}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		tests := []struct {
			name string
			body string
			code string
		}{
			{
				name: "Should reject an unknown state",
				body: `{"until":["sleeping"],"timeout_ms":1}`,
				code: "invalid_state",
			},
			{name: "Should reject an omitted timeout", body: `{}`, code: "invalid_wait"},
			{name: "Should reject an oversized timeout", body: `{"timeout_ms":1800001}`, code: "invalid_wait"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				response := performRequest(
					t,
					fixture.Engine,
					http.MethodPost,
					"/workspaces/ws-1/sessions/sess-attention/wait",
					[]byte(test.body),
				)
				if response.Code != http.StatusUnprocessableEntity {
					t.Fatalf("invalid wait status = %d, want 422; body=%s", response.Code, response.Body.String())
				}
				var payload contract.ErrorPayload
				testutil.DecodeJSONResponse(t, response, &payload)
				if payload.Code != test.code {
					t.Fatalf("invalid wait code = %q, want %q", payload.Code, test.code)
				}
			})
		}
	})

	t.Run("Should distinguish a gone session from an expired resume", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			outcome session.WaitOutcome
			err     error
			code    string
		}{
			{
				name:    "Should return session gone for identity loss",
				outcome: session.WaitOutcome{Outcome: session.WaitResultSessionGone},
				code:    "session_gone",
			},
			{
				name: "Should return wait expired for a stale resume id",
				err:  session.ErrWaitExpired,
				code: "wait_expired",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				manager := attentionRouteSessionManager()
				manager.Attention.WaitFn = func(
					context.Context,
					session.WaitRequest,
				) (session.WaitOutcome, error) {
					return test.outcome, test.err
				}
				fixture := newHandlerFixture(
					t,
					manager,
					testutil.StubObserver{},
					testutil.StubWorkspaceService{},
				)
				response := performRequest(
					t,
					fixture.Engine,
					http.MethodPost,
					"/workspaces/ws-1/sessions/sess-attention/wait",
					[]byte(`{"timeout_ms":5000}`),
				)
				if response.Code != http.StatusGone {
					t.Fatalf("gone wait status = %d, want 410; body=%s", response.Code, response.Body.String())
				}
				var payload contract.ErrorPayload
				testutil.DecodeJSONResponse(t, response, &payload)
				if payload.Code != test.code {
					t.Fatalf("gone wait code = %q, want %q", payload.Code, test.code)
				}
			})
		}
	})
}

func TestBaseHandlersAttentionOperatorScope(t *testing.T) {
	t.Parallel()

	t.Run("Should deny agent identity on every operator-only catalog surface", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixtureWithTasks(
			t,
			attentionRouteSessionManager(),
			testutil.StubObserver{},
			&testutil.StubTaskManager{},
			testutil.StubWorkspaceService{},
		)
		fixture.Engine.GET("/loop-runs", fixture.Handlers.ListLoopRuns)
		fixture.Engine.GET("/loop-requests", fixture.Handlers.ListLoopRequests)
		fixture.Engine.GET("/loops", fixture.Handlers.ListLoops)
		headers := map[string]string{
			agentidentity.HeaderSessionID: "sess-agent",
			agentidentity.HeaderAgent:     "coder",
		}
		requests := []struct {
			method string
			path   string
			body   []byte
		}{
			{method: http.MethodGet, path: "/sessions?all_workspaces=true"},
			{method: http.MethodGet, path: "/loop-runs?all_workspaces=true"},
			{method: http.MethodGet, path: "/loop-requests?all_workspaces=true"},
			{method: http.MethodGet, path: "/loops?all_workspaces=true"},
			{method: http.MethodGet, path: "/tasks?all_workspaces=true"},
			{method: http.MethodGet, path: "/observe/tasks/dashboard?all_workspaces=true"},
			{method: http.MethodGet, path: "/observe/overview?all_workspaces=true"},
			{method: http.MethodGet, path: "/sessions/attention-summary"},
			{
				method: http.MethodPost,
				path:   "/workspaces/ws-1/sessions/sess-attention/presence",
				body:   []byte(`{"visible":true}`),
			},
		}
		for _, request := range requests {
			response := testutil.PerformRequestWithHeaders(
				t,
				fixture.Engine,
				request.method,
				request.path,
				request.body,
				headers,
			)
			if response.Code != http.StatusForbidden {
				t.Fatalf(
					"%s %s status = %d, want 403; body=%s",
					request.method,
					request.path,
					response.Code,
					response.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, response, &payload)
			if payload.Code != "agent_scope_denied" {
				t.Fatalf("%s %s error = %#v, want agent_scope_denied", request.method, request.path, payload)
			}
		}
	})

	t.Run("Should admit approve-all aggregate reads while keeping aggregate writes operator-only", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			attentionRouteSessionManager(),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Engine.GET("/test-layout", fixture.Handlers.GetWorkspaceLayout)
		fixture.Engine.POST("/test-layout", fixture.Handlers.CreateWorkspaceFolder)
		var targets []workspaceaccess.Request
		fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
			AuthorizeFn: func(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
				targets = append(targets, req)
				if req.Target.Kind == workspaceaccess.TargetAggregate && !req.ReadOnly {
					return workspaceaccess.Decision{
						Source: workspaceaccess.SourceDenied, Denial: workspaceaccess.DenialNeedsGrant,
					}, nil
				}
				return workspaceaccess.Decision{Allowed: true, Source: workspaceaccess.SourcePermissionMode}, nil
			},
		}
		headers := map[string]string{
			agentidentity.HeaderSessionID: "sess-agent",
			agentidentity.HeaderAgent:     "coder",
		}
		read := testutil.PerformRequestWithHeaders(
			t, fixture.Engine, http.MethodGet, "/sessions?all_workspaces=true", nil, headers,
		)
		if read.Code != http.StatusOK {
			t.Fatalf("approve-all aggregate read status = %d, want 200; body=%s", read.Code, read.Body.String())
		}
		write := testutil.PerformRequestWithHeaders(
			t, fixture.Engine, http.MethodPost, "/test-layout", []byte(`{"name":"x"}`), headers,
		)
		if write.Code != http.StatusForbidden {
			t.Fatalf("approve-all aggregate write status = %d, want 403; body=%s", write.Code, write.Body.String())
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, write, &payload)
		if payload.Code != "agent_scope_denied" || !strings.Contains(payload.Error, "workspace_access: unrestricted") {
			t.Fatalf("aggregate write error = %#v, want agent_scope_denied naming the grant", payload)
		}
		if len(targets) < 2 || !targets[0].ReadOnly || targets[len(targets)-1].ReadOnly {
			t.Fatalf("policy requests = %#v, want a read then a write aggregate request", targets)
		}
	})

	t.Run("Should answer a denied agent the same for missing and foreign profile names", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			attentionRouteSessionManager(),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.Profiles = profileExistenceStub{}
		fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
			AuthorizeFn: func(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
				if req.Target.Kind == workspaceaccess.TargetAggregate {
					return workspaceaccess.Decision{Allowed: true, Source: workspaceaccess.SourcePermissionMode}, nil
				}
				return workspaceaccess.Decision{
					Source: workspaceaccess.SourceDenied, Denial: workspaceaccess.DenialModeDenyAll,
				}, nil
			},
		}
		fixture.Engine.GET("/test-layout", fixture.Handlers.GetWorkspaceLayout)
		headers := map[string]string{
			agentidentity.HeaderSessionID: "sess-agent",
			agentidentity.HeaderAgent:     "coder",
		}
		foreign := testutil.PerformRequestWithHeaders(
			t, fixture.Engine, http.MethodGet, "/test-layout?profile=marketing", nil, headers,
		)
		if foreign.Code != http.StatusConflict {
			t.Fatalf("denied foreign profile status = %d, want 409; body=%s", foreign.Code, foreign.Body.String())
		}
		for _, name := range []string{"ghost", "archived", "busy"} {
			probe := testutil.PerformRequestWithHeaders(
				t, fixture.Engine, http.MethodGet, "/test-layout?profile="+name, nil, headers,
			)
			if probe.Code != foreign.Code || probe.Body.String() != foreign.Body.String() {
				t.Fatalf("denied profile answers differ:\nforeign: %d %s\n%s: %d %s",
					foreign.Code, foreign.Body.String(), name, probe.Code, probe.Body.String())
			}
		}
		broken := testutil.PerformRequestWithHeaders(
			t, fixture.Engine, http.MethodGet, "/test-layout?profile=broken", nil, headers,
		)
		if broken.Code == http.StatusConflict || broken.Code < http.StatusInternalServerError {
			t.Fatalf("store failure status = %d, want a server error, not the masked conflict; body=%s",
				broken.Code, broken.Body.String())
		}
	})

	t.Run("Should scope agent interaction discovery to the caller workspace", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.PendingFn = func(
			_ context.Context,
			sessionID string,
			_ []string,
		) ([]store.PendingInteraction, error) {
			return []store.PendingInteraction{{
				InteractionID: "interaction-1",
				SessionID:     sessionID,
				Kind:          store.PendingInteractionKindPermission,
				Status:        store.PendingInteractionStatusPending,
			}}, nil
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		headers := map[string]string{
			agentidentity.HeaderSessionID: "sess-agent",
			agentidentity.HeaderAgent:     "coder",
		}
		sameWorkspace := testutil.PerformRequestWithHeaders(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-attention/interactions",
			nil,
			headers,
		)
		if sameWorkspace.Code != http.StatusOK {
			t.Fatalf(
				"same-workspace interactions status = %d, want 200; body=%s",
				sameWorkspace.Code,
				sameWorkspace.Body.String(),
			)
		}

		foreignWorkspace := testutil.PerformRequestWithHeaders(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-foreign/sessions/sess-attention/interactions",
			nil,
			headers,
		)
		if foreignWorkspace.Code != http.StatusForbidden {
			t.Fatalf(
				"foreign-workspace interactions status = %d, want 403; body=%s",
				foreignWorkspace.Code,
				foreignWorkspace.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, foreignWorkspace, &payload)
		if payload.Diagnostic == nil || payload.Diagnostic.Code != contract.CodeIdentityUnauthorized {
			t.Fatalf("foreign-workspace error = %#v, want identity_unauthorized", payload)
		}
	})
}

func attentionRouteSessionManager() *testutil.StubSessionManager {
	return &testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			switch id {
			case "sess-attention":
				return &session.Info{
					ID: id, ProfileID: store.DefaultProfileID, WorkspaceID: "ws-1", State: session.StateActive,
				}, nil
			case "sess-agent":
				return &session.Info{
					ID: id, ProfileID: store.DefaultProfileID, AgentName: "coder",
					WorkspaceID: "ws-1", State: session.StateActive,
				}, nil
			default:
				return nil, session.ErrSessionNotFound
			}
		},
	}
}

// The existing API suite owns operator authorization, complete source pagination and receipt reconciliation.
// Source I/O is stubbed; acknowledgement uses real SQLite and source attention is never mutated.
type notificationRouteObserver struct {
	testutil.StubObserver
	attention.Store
}

func (notificationRouteObserver) TaskAttentionItems(
	context.Context,
	observe.OverviewQuery,
) ([]observe.OverviewAttentionItem, error) {
	return nil, nil
}

type notificationRouteTerminal struct{ terminalpkg.Manager }

func (notificationRouteTerminal) InputRequestsAggregate(
	context.Context,
	store.ReadScope,
) ([]terminalpkg.PendingInputRequest, error) {
	return nil, nil
}

func (notificationRouteTerminal) InputRequests(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.ID,
) ([]terminalpkg.PendingInputRequest, error) {
	return nil, nil
}

func TestAttentionNotificationReceipts(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should reconcile exact counts and preserve later session revisions after a frozen bulk acknowledgement",
		func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "global.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Errorf("close: %v", err)
				}
			})
			revision := int64(1)
			manager := attentionRouteSessionManager()
			manager.ListPageFn = func(_ context.Context, query session.ListQuery) (session.ListPage, error) {
				if !query.AllWorkspaces || !query.ReadScope.AllProfiles {
					t.Fatalf("notification read lost global scope: %+v", query)
				}
				if len(query.Badges) == 1 && query.Badges[0] == session.BadgeDone && !query.AttentionOnly {
					return session.ListPage{Sessions: []*session.Info{{
						ID: "sess-finished", Name: "issue-triage", AgentName: "issue-triage",
						Type: session.SessionTypeSystem, ProfileID: store.DefaultProfileID,
						WorkspaceID: "ws-2", State: session.StateActive, LastSettledRevision: revision,
						AttentionRevision: revision, UpdatedAt: time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC),
					}}}, nil
				}
				if !query.AttentionOnly || len(query.Badges) != 0 {
					t.Fatalf("notification read lost its attention population: %+v", query)
				}
				start, end := 0, 100
				if query.Cursor == "page-two" {
					start, end = 100, 151
				}
				items := make([]*session.Info, 0, end-start)
				for i := start; i < end; i++ {
					items = append(
						items,
						&session.Info{
							ID:                  fmt.Sprintf("sess-%03d", i),
							Name:                "Failed work",
							ProfileID:           store.DefaultProfileID,
							WorkspaceID:         "ws-1",
							PendingClarifyCount: 1,
							InstanceID:          "ainst-triage",
							AttentionRevision:   revision,
							UpdatedAt:           time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
						},
					)
				}
				page := session.ListPage{Sessions: items, HasMore: end == 100}
				if page.HasMore {
					page.NextCursor = "page-two"
				}
				return page, nil
			}
			fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
			fixture.Handlers.Observer = notificationRouteObserver{Store: db}
			fixture.Handlers.Loops = &stubLoopService{
				listLoopNodesFn: func(context.Context, string, core.LoopNodeListQuery) (contract.LoopNodeInventoryResponse, error) {
					return contract.LoopNodeInventoryResponse{}, nil
				},
				listLoopRequestsFn: func(context.Context, string, core.LoopRequestListQuery) (contract.LoopRequestsResponse, error) {
					return contract.LoopRequestsResponse{}, nil
				},
			}
			fixture.Handlers.Terminal = notificationRouteTerminal{}
			fixture.Engine.GET("/notifications/attention", fixture.Handlers.AttentionNotifications)
			fixture.Engine.POST(
				"/notifications/attention/acknowledge",
				fixture.Handlers.AcknowledgeAttentionNotifications,
			)
			response := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/notifications/attention?profile=default",
				nil,
			)
			if response.Code != http.StatusOK {
				t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
			}
			var before contract.AttentionNotificationsResponse
			testutil.DecodeJSONResponse(t, response, &before)
			if before.Total != 152 || before.NeedsYou != 151 || before.Finished != 1 || len(before.Items) != 100 {
				t.Fatalf("incomplete notification count: %+v", before)
			}
			if before.Items[0].SourceID != "sess-finished" || !before.Items[0].Finished {
				t.Fatalf("finished row missing from the notification snapshot: %+v", before.Items[0])
			}
			// A session row carries its origin so the bell can title an automation run by its job.
			if before.Items[0].SessionType != string(session.SessionTypeSystem) ||
				before.Items[1].InstanceID != "ainst-triage" {
				t.Fatalf("session origin missing from notification rows: %+v / %+v", before.Items[0], before.Items[1])
			}
			body, err := json.Marshal(contract.AcknowledgeAttentionRequest{Snapshot: before.Snapshot})
			if err != nil {
				t.Fatal(err)
			}
			revision++ // Every session has a new meaningful occurrence after the captured response.
			for range 2 {
				response = performRequest(
					t,
					fixture.Engine,
					http.MethodPost,
					"/notifications/attention/acknowledge?profile=default",
					body,
				)
				if response.Code != http.StatusNoContent {
					t.Fatalf("ack status=%d body=%s", response.Code, response.Body.String())
				}
			}
			response = performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/notifications/attention?profile=default",
				nil,
			)
			var after contract.AttentionNotificationsResponse
			testutil.DecodeJSONResponse(t, response, &after)
			if response.Code != http.StatusOK || after.Total != 152 || after.NeedsYou != 151 || after.Finished != 1 ||
				after.Snapshot == before.Snapshot {
				t.Fatalf("new occurrences lost: status=%d total=%d", response.Code, after.Total)
			}
			revision-- // Replay the original source revision: its receipts still suppress the rows.
			response = performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/notifications/attention?profile=default",
				nil,
			)
			testutil.DecodeJSONResponse(t, response, &after)
			if response.Code != http.StatusOK || after.Total != 0 || len(after.Items) != 0 {
				t.Fatalf("replay resurrected rows: %+v", after)
			}
		},
	)
}

type bellTaskObserver struct {
	notificationRouteObserver
	queries *[]observe.OverviewQuery
	items   []observe.OverviewAttentionItem
}

func (o bellTaskObserver) TaskAttentionItems(
	_ context.Context,
	query observe.OverviewQuery,
) ([]observe.OverviewAttentionItem, error) {
	*o.queries = append(*o.queries, query)
	return o.items, nil
}

// Invariant: the bell reads task attention once for every registered population, keeping each
// task's notification identity and workspace binding. Owner: core bell handler.
func TestAttentionNotificationTaskPopulation(t *testing.T) {
	t.Parallel()
	t.Run("Should read tasks once and keep registered workspace bindings", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "global.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close: %v", err)
			}
		})
		manager := attentionRouteSessionManager()
		manager.ListPageFn = func(context.Context, session.ListQuery) (session.ListPage, error) {
			return session.ListPage{}, nil
		}
		workspaces := testutil.StubWorkspaceService{
			ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
				return []workspacepkg.Workspace{{ID: "ws-1", Name: "Alpha"}, {ID: "ws-2", Name: "Beta"}}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces)
		at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		var queries []observe.OverviewQuery
		fixture.Handlers.Observer = bellTaskObserver{
			Store:   db,
			queries: &queries,
			items: []observe.OverviewAttentionItem{
				{NotificationID: "n-global", TaskID: "task-global", Kind: "approval", Title: "G", OccurredAt: at},
				{
					NotificationID: "n-ws1",
					TaskID:         "task-ws1",
					WorkspaceID:    "ws-1",
					Kind:           "failure",
					Title:          "A",
					OccurredAt:     at,
				},
				{
					NotificationID: "n-gone",
					TaskID:         "task-gone",
					WorkspaceID:    "ws-gone",
					Kind:           "failure",
					Title:          "X",
					OccurredAt:     at,
				},
			},
		}
		var loopWorkspaces []string
		fixture.Handlers.Loops = &stubLoopService{
			listLoopNodesFn: func(context.Context, string, core.LoopNodeListQuery) (contract.LoopNodeInventoryResponse, error) {
				return contract.LoopNodeInventoryResponse{}, nil
			},
			listLoopRequestsFn: func(
				_ context.Context, workspace string, query core.LoopRequestListQuery,
			) (contract.LoopRequestsResponse, error) {
				if !query.AllWorkspaces || !query.ReadScope.AllProfiles {
					t.Fatalf("bell scope = %+v", query)
				}
				loopWorkspaces = append(loopWorkspaces, workspace)
				return contract.LoopRequestsResponse{}, nil
			},
		}
		fixture.Handlers.Terminal = notificationRouteTerminal{}
		fixture.Engine.GET("/notifications/attention", fixture.Handlers.AttentionNotifications)

		response := performRequest(t, fixture.Engine, http.MethodGet, "/notifications/attention?profile=default", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		var payload contract.AttentionNotificationsResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if len(queries) != 1 || queries[0].TaskScope != taskpkg.CatalogScopeAll || queries[0].WorkspaceID != "" ||
			!queries[0].ReadScope.AllProfiles {
			t.Fatalf("task attention reads = %+v, want one all-population read", queries)
		}
		got := map[string]string{}
		for _, item := range payload.Items {
			got[item.ID] = item.SourceID + "@" + item.WorkspaceID + "/" + item.WorkspaceLabel
		}
		want := map[string]string{"n-global": "task-global@/User", "n-ws1": "task-ws1@ws-1/Alpha"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("bell task rows = %v, want %v", got, want)
		}
		if !reflect.DeepEqual(loopWorkspaces, []string{""}) {
			t.Fatalf("loop reads = %v, want one aggregate read", loopWorkspaces)
		}
	})
}

func TestSessionInteractionDescendants(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the child identity when descendants are requested", func(t *testing.T) {
		t.Parallel()
		manager := attentionRouteSessionManager()
		manager.Attention.DescendantPendingFn = func(_ context.Context, id string, statuses []string) ([]store.PendingInteraction, error) {
			if id != "sess-attention" || len(statuses) != 1 || statuses[0] != "pending" {
				t.Fatalf("query = %q/%v", id, statuses)
			}
			return []store.PendingInteraction{
				{
					InteractionID:     "child-interaction",
					SessionID:         "child-session",
					ProviderRequestID: "original-request",
					Kind:              store.PendingInteractionKindPermission,
					Status:            store.PendingInteractionStatusPending,
				},
			}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-attention/interactions?descendants=true&status=pending",
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status/body = %d/%s", response.Code, response.Body.String())
		}
		var payload contract.SessionInteractionsResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if len(payload.Interactions) != 1 || payload.Interactions[0].SessionID != "child-session" ||
			payload.Interactions[0].ProviderRequestID != "original-request" {
			t.Fatalf("interactions = %#v", payload.Interactions)
		}
	})
	t.Run("Should reject an invalid descendants parameter", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			attentionRouteSessionManager(),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-attention/interactions?descendants=invalid",
			nil,
		)
		assertAPIErrorResponse(t, response, http.StatusBadRequest, "api: descendants must be a boolean")
	})
}

// profileExistenceStub knows the default and marketing profiles, an archived and a busy one, and
// binds sessions like
// the profile manager: a session-bound resolution rejects any other name without a lookup.
type profileExistenceStub struct{ sessionProfileServiceStub }

func (profileExistenceStub) Resolve(_ context.Context, in profilepkg.ResolveInput) (profilepkg.Resolution, error) {
	if in.SessionProfileID != "" && in.Flag != "" && in.Flag != "default" {
		return profilepkg.Resolution{}, &profilepkg.Error{
			Code: "profile_session_conflict", Message: `session is bound to profile "default"`,
			Cause: profilepkg.ErrSessionConflict,
		}
	}
	switch in.Flag {
	case "marketing":
		return profilepkg.Resolution{Profile: profilepkg.Profile{
			ID: "profile-marketing", Name: "marketing", State: profilepkg.StateActive,
		}}, nil
	case "archived":
		return profilepkg.Resolution{}, &profilepkg.Error{
			Code: "profile_archived", Message: "profile archived is archived", Cause: profilepkg.ErrArchived,
		}
	case "busy":
		return profilepkg.Resolution{}, &profilepkg.Error{
			Code: "profile_unavailable", Message: "profile busy has lifecycle work op-1",
			Cause: profilepkg.ErrUnavailable,
		}
	case "broken":
		return profilepkg.Resolution{}, errors.New("profile store unavailable")
	case "", "default":
		return profilepkg.Resolution{Profile: profilepkg.Profile{
			ID: store.DefaultProfileID, Name: "default", State: profilepkg.StateActive,
		}}, nil
	default:
		return profilepkg.Resolution{}, &profilepkg.Error{
			Code: "profile_not_found", Message: "profile " + in.Flag + " not found", Cause: profilepkg.ErrNotFound,
		}
	}
}

type bellMigrationSource struct {
	core.WindowManagerProvider
	notices []windowmanager.MigrationNotice
	reads   int
	acked   []string
}

func (s *bellMigrationSource) MigrationNotifications(
	_ context.Context, _ []windowmanager.WorkspaceID,
) ([]windowmanager.MigrationNotice, error) {
	s.reads++
	return s.notices, nil
}

func (s *bellMigrationSource) AcknowledgeMigrationNotification(
	_ context.Context,
	notice windowmanager.MigrationNotice,
) error {
	s.acked = append(s.acked, notice.ProfileID)
	for i := range s.notices {
		if s.notices[i].ProfileID == notice.ProfileID && s.notices[i].WorkspaceID == notice.WorkspaceID {
			s.notices[i].Status.Acknowledged = true
		}
	}
	return nil
}

var _ core.WindowManagerMigrationNotifications = (*bellMigrationSource)(nil)

func newBellMigrationFixture(t *testing.T, source *bellMigrationSource) handlerFixture {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	manager := attentionRouteSessionManager()
	manager.ListPageFn = func(context.Context, session.ListQuery) (session.ListPage, error) {
		return session.ListPage{}, nil
	}
	f := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{
		ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
			return []workspacepkg.Workspace{{ID: "ws-1", Name: "Alpha"}}, nil
		},
	})
	f.Handlers.Observer = notificationRouteObserver{Store: db}
	f.Handlers.Profiles = profileExistenceStub{}
	f.Handlers.WindowManager = source
	f.Handlers.Loops = &stubLoopService{
		listLoopNodesFn: func(context.Context, string, core.LoopNodeListQuery) (contract.LoopNodeInventoryResponse, error) {
			return contract.LoopNodeInventoryResponse{}, nil
		},
		listLoopRequestsFn: func(context.Context, string, core.LoopRequestListQuery) (contract.LoopRequestsResponse, error) {
			return contract.LoopRequestsResponse{}, nil
		},
	}
	f.Handlers.Terminal = notificationRouteTerminal{}
	f.Engine.GET("/notifications/attention", f.Handlers.AttentionNotifications)
	f.Engine.POST("/notifications/attention/acknowledge", f.Handlers.AcknowledgeAttentionNotifications)
	return f
}

// Invariant: all-profile conversion occurrences obey the bell's scope, frozen membership and copy contract.
// Owner: core attention handler; canonical suite: session_attention_test.go.
func TestAttentionNotificationLayoutUpgrades(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	notice := func(profile string) windowmanager.MigrationNotice {
		return windowmanager.MigrationNotice{WorkspaceID: "ws-1", ProfileID: profile,
			Status: windowmanager.MigrationStatus{State: "migrated", At: at}}
	}
	t.Run(
		"Should include all source profiles with stable identities and only pending registered conversions",
		func(t *testing.T) {
			t.Parallel()
			source := &bellMigrationSource{notices: []windowmanager.MigrationNotice{
				notice(
					store.DefaultProfileID,
				),
				notice("profile-marketing"),
				notice("gone"),
				notice("read"),
				notice("failed"),
				notice("none"),
			}}
			source.notices[2].WorkspaceID = "unregistered"
			source.notices[3].Status.Acknowledged = true
			source.notices[4].Status.State = "failed"
			source.notices[5].Status.State = "none"
			f := newBellMigrationFixture(t, source)
			for _, query := range []string{"profile=default", "profile=marketing&receipt_profile=default", "all_profiles=true&receipt_profile=marketing"} {
				response := performRequest(t, f.Engine, http.MethodGet, "/notifications/attention?"+query, nil)
				var payload contract.AttentionNotificationsResponse
				testutil.DecodeJSONResponse(t, response, &payload)
				if response.Code != http.StatusOK || payload.Total != 2 || payload.Finished != 2 ||
					payload.NeedsYou != 0 {
					t.Fatalf("scope %s: status=%d payload=%+v", query, response.Code, payload)
				}
				seen := make(map[string]bool)
				for _, item := range payload.Items {
					seen[item.ID] = true
					if item.Kind != "layout-upgrade" || item.Title != "Your windows are now tabs" || item.SourceID != "ws-1" ||
						item.WorkspaceID != "ws-1" ||
						item.WorkspaceLabel != "Alpha" ||
						!item.OccurredAt.Equal(at) {
						t.Fatalf("item=%+v", item)
					}
					wantDefault := attention.Identity(
						"layout-upgrade",
						"ws-1",
						store.DefaultProfileID,
						at.Format(time.RFC3339Nano),
					)
					wantMarketing := attention.Identity(
						"layout-upgrade",
						"ws-1",
						"profile-marketing",
						at.Format(time.RFC3339Nano),
					)
					if item.ID != wantDefault && item.ID != wantMarketing {
						t.Fatalf("identity=%q", item.ID)
					}
				}
				if len(seen) != 2 {
					t.Fatal("profile occurrences shared an identity")
				}
			}
			if source.reads != 3 {
				t.Fatalf("source reads=%d, want one per request", source.reads)
			}
		},
	)
	for _, tc := range []struct {
		name    string
		summary *windowmanager.MigrationSummary
		want    string
	}{
		{"Should describe an absent summary", nil, "Floating and minimized windows are tabs now."},
		{"Should describe one floating window", &windowmanager.MigrationSummary{FloatingToTabs: 1}, "1 floating window is a tab now."},
		{"Should describe several floating windows", &windowmanager.MigrationSummary{FloatingToTabs: 2}, "2 floating windows are tabs now."},
		{"Should describe one minimized window", &windowmanager.MigrationSummary{MinimizedToTabs: 1}, "1 minimized window is a tab now."},
		{"Should describe several minimized windows", &windowmanager.MigrationSummary{MinimizedToTabs: 2}, "2 minimized windows are tabs now."},
		{"Should join moved window counts and describe one dropped pin", &windowmanager.MigrationSummary{FloatingToTabs: 1, MinimizedToTabs: 2, PinsDropped: 1}, "1 floating window and 2 minimized windows are tabs now. 1 pin inside a split was removed."},
		{"Should describe several dropped pins with no moved windows", &windowmanager.MigrationSummary{PinsDropped: 2}, "Floating and minimized windows are tabs now. 2 pins inside splits were removed."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := notice(store.DefaultProfileID)
			n.Status.Summary = tc.summary
			f := newBellMigrationFixture(t, &bellMigrationSource{notices: []windowmanager.MigrationNotice{n}})
			response := performRequest(t, f.Engine, http.MethodGet, "/notifications/attention?profile=default", nil)
			var payload contract.AttentionNotificationsResponse
			testutil.DecodeJSONResponse(t, response, &payload)
			if response.Code != http.StatusOK || len(payload.Items) != 1 || payload.Items[0].Detail != tc.want {
				t.Fatalf("status=%d payload=%+v, want detail %q", response.Code, payload, tc.want)
			}
		})
	}
	for _, single := range []bool{true, false} {
		name := "Should clear only frozen conversions including rows beyond the displayed page"
		if single {
			name = "Should acknowledge one conversion across receipt profiles"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := &bellMigrationSource{}
			for i := range 102 {
				source.notices = append(source.notices, notice(fmt.Sprintf("profile-%03d", i)))
			}
			f := newBellMigrationFixture(t, source)
			read := func(query string) contract.AttentionNotificationsResponse {
				t.Helper()
				response := performRequest(t, f.Engine, http.MethodGet, "/notifications/attention?"+query, nil)
				if response.Code != http.StatusOK {
					t.Fatalf("read status=%d body=%s", response.Code, response.Body.String())
				}
				var payload contract.AttentionNotificationsResponse
				testutil.DecodeJSONResponse(t, response, &payload)
				return payload
			}
			before := read("profile=default")
			if before.Total != 102 || len(before.Items) != 100 {
				t.Fatalf("before=%+v", before)
			}
			request := contract.AcknowledgeAttentionRequest{Snapshot: before.Snapshot, ID: "not-in-snapshot"}
			ack := func(want int) {
				t.Helper()
				body, err := json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				response := performRequest(
					t,
					f.Engine,
					http.MethodPost,
					"/notifications/attention/acknowledge?profile=default&surface=bell",
					body,
				)
				if response.Code != want {
					t.Fatalf("ack status=%d body=%s", response.Code, response.Body.String())
				}
			}
			ack(http.StatusConflict)
			if len(source.acked) != 0 {
				t.Fatal("invalid member mutated migration state")
			}
			request.ID = ""
			if single {
				request.ID = before.Items[0].ID
			}
			// A later successful retry of an existing source cannot be cleared by the older snapshot.
			changed := 0
			if single &&
				attention.Identity(
					"layout-upgrade",
					"ws-1",
					source.notices[0].ProfileID,
					at.Format(time.RFC3339Nano),
				) == request.ID {
				changed = 1
			}
			source.notices[changed].Status.At = at.Add(time.Hour)
			source.notices = append(source.notices, notice("later"))
			ack(http.StatusNoContent)
			wantAcked, wantRemaining := 101, 2
			if single {
				wantAcked, wantRemaining = 1, 102
			}
			if len(source.acked) != wantAcked {
				t.Fatalf("acked=%d want %d", len(source.acked), wantAcked)
			}
			ack(http.StatusNoContent)
			if len(source.acked) != wantAcked {
				t.Fatal("retry repeated a source mutation")
			}
			after := read("profile=marketing&receipt_profile=marketing")
			if after.Total != wantRemaining {
				t.Fatalf("other profile total=%d want %d", after.Total, wantRemaining)
			}
		})
	}
}
