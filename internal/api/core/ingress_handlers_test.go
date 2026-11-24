package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func TestIngressAgentCallerBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should pass only the daemon-validated agent workspace into a binding mutation", func(t *testing.T) {
		t.Parallel()

		ref := ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: "trigger-1"}
		handlers := &BaseHandlers{
			TransportName: "uds-api",
			Sessions: sessionManagerStub{status: func(_ context.Context, id string) (*session.Info, error) {
				return &session.Info{
					ID:          id,
					ProfileID:   store.DefaultProfileID,
					AgentName:   "coder",
					WorkspaceID: "ws-agent",
					State:       session.StateActive,
				}, nil
			}},
			Ingress: ingressServiceStub{
				bind: func(_ context.Context, request ingress.BindRequest) (ingress.Binding, error) {
					if request.Caller == nil || request.Caller.SessionID != "sess-agent" ||
						request.Caller.AgentName != "coder" || request.Caller.WorkspaceID != "ws-agent" {
						t.Fatalf("BindIngress() caller = %#v", request.Caller)
					}
					return ingress.Binding{Subject: request.Subject, Changed: true}, nil
				},
				project: func(context.Context, ingress.SubjectRef) (ingress.Projection, error) {
					return ingress.Projection{
						Subject:      ref,
						Reachability: ingress.ReachabilityUnconfirmed,
					}, nil
				},
			},
		}
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/ingress-bindings", handlers.BindIngress)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/ingress-bindings",
			strings.NewReader(`{"subject_kind":"webhook_trigger","subject_id":"trigger-1","confirmed":true}`),
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
		request.Header.Set(agentidentity.HeaderAgent, "coder")

		router.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
	})

	t.Run(
		"Should return deterministic forbidden responses for cross-workspace bind and unbind [IT-049]",
		func(t *testing.T) {
			t.Parallel()

			handlers := &BaseHandlers{
				TransportName: "uds-api",
				Sessions: sessionManagerStub{status: func(_ context.Context, id string) (*session.Info, error) {
					return &session.Info{
						ID:          id,
						ProfileID:   store.DefaultProfileID,
						AgentName:   "coder",
						WorkspaceID: "ws-agent",
						State:       session.StateActive,
					}, nil
				}},
				Ingress: ingressServiceStub{
					bind: func(
						_ context.Context,
						request ingress.BindRequest,
					) (ingress.Binding, error) {
						if request.Caller == nil || request.Caller.WorkspaceID != "ws-agent" {
							t.Fatalf("BindIngress() caller = %#v", request.Caller)
						}
						return ingress.Binding{}, ingress.ErrForbidden
					},
					unbind: func(
						_ context.Context,
						request ingress.UnbindRequest,
					) (ingress.UnbindResult, error) {
						if request.Caller == nil || request.Caller.WorkspaceID != "ws-agent" {
							t.Fatalf("UnbindIngress() caller = %#v", request.Caller)
						}
						return ingress.UnbindResult{}, ingress.ErrForbidden
					},
				},
			}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/ingress-bindings", handlers.BindIngress)
			router.DELETE("/ingress-bindings/:subject_kind/:subject_id", handlers.UnbindIngress)
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, "/ingress-bindings",
				strings.NewReader(`{"subject_kind":"webhook_trigger","subject_id":"foreign-trigger","confirmed":true}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
			request.Header.Set(agentidentity.HeaderAgent, "coder")

			router.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden ||
				!strings.Contains(response.Body.String(), "ingress_forbidden") {
				t.Fatalf("response = (%d, %q), want deterministic ingress denial",
					response.Code, response.Body.String())
			}

			unbindResponse := httptest.NewRecorder()
			unbindRequest := httptest.NewRequestWithContext(
				t.Context(), http.MethodDelete, "/ingress-bindings/webhook_trigger/foreign-trigger", http.NoBody,
			)
			unbindRequest.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
			unbindRequest.Header.Set(agentidentity.HeaderAgent, "coder")
			router.ServeHTTP(unbindResponse, unbindRequest)
			if unbindResponse.Code != http.StatusForbidden ||
				!strings.Contains(unbindResponse.Body.String(), "ingress_forbidden") {
				t.Fatalf("unbind response = (%d, %q), want deterministic ingress denial",
					unbindResponse.Code, unbindResponse.Body.String())
			}
		},
	)
}
