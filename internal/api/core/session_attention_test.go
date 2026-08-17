package core_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
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
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
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
				Kind: store.PendingInteractionKindClarify, ProviderRequestID: "request-1",
				Title: "Deploy with compozy_claim_secret123", Payload: store.PendingInteractionPayload{
					Choices: []string{"safe", "compozy_claim_secret123"},
				},
				Status:    store.PendingInteractionStatusPending,
				CreatedAt: time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC),
			}}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
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
			payload.Interactions[0].Status != store.PendingInteractionStatusPending {
			t.Fatalf("interactions payload = %#v, want one canonical pending row", payload)
		}
	})

	t.Run("Should return exact totals and workspace counts", func(t *testing.T) {
		t.Parallel()

		manager := attentionRouteSessionManager()
		manager.Attention.SummaryFn = func(context.Context) (store.SessionAttentionSummary, error) {
			return store.SessionAttentionSummary{
				NeedsYou: 101,
				Finished: 17,
				ByWorkspace: []store.WorkspaceAttentionSummary{
					{WorkspaceID: "ws-1", NeedsYou: 60, Finished: 7},
					{WorkspaceID: "ws-2", NeedsYou: 41, Finished: 10},
				},
			}, nil
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		response := performRequest(t, fixture.Engine, http.MethodGet, "/sessions/attention-summary", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("attention summary status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.SessionAttentionSummaryPayload
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.NeedsYou != 101 || payload.Finished != 17 || len(payload.ByWorkspace) != 2 {
			t.Fatalf("attention summary payload = %#v, want exact aggregate", payload)
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
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
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
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
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
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
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
					nil,
					nil,
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

		fixture := newHandlerFixture(
			t,
			attentionRouteSessionManager(),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		headers := map[string]string{
			agentidentity.HeaderSessionID: "sess-agent",
			agentidentity.HeaderAgent:     "coder",
		}
		requests := []struct {
			method string
			path   string
			body   []byte
		}{
			{method: http.MethodGet, path: "/sessions"},
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
			nil,
			nil,
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

func attentionRouteSessionManager() testutil.StubSessionManager {
	return testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			switch id {
			case "sess-attention":
				return &session.Info{ID: id, WorkspaceID: "ws-1", State: session.StateActive}, nil
			case "sess-agent":
				return &session.Info{
					ID:          id,
					AgentName:   "coder",
					WorkspaceID: "ws-1",
					State:       session.StateActive,
				}, nil
			default:
				return nil, session.ErrSessionNotFound
			}
		},
	}
}
