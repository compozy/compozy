// Suite: bridge control UDS routes
// Invariant: UDS exposes the same typed verify, webhook-registration, and real-send responses as HTTP.
// Boundary IN: UDS route registration and shared BaseHandlers invocation.
// Boundary OUT: provider control and delivery semantics, owned by daemon and adapter suites.
package udsapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestBridgeControlHandlersReturnTypedPayloadsOverUDS(t *testing.T) {
	t.Run("Should return provider verification records", func(t *testing.T) {
		t.Parallel()

		engine := newBridgeControlTestRouter(t, stubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				return &bridgepkg.BridgeInstance{ID: "brg-slack", ExtensionName: "slack"}, nil
			},
			CheckBridgeFn: func(
				context.Context,
				string,
				bridgepkg.BridgeCheckRequest,
			) (bridgepkg.BridgeCheckResponse, error) {
				return bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{
					{Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusPass},
				}}, nil
			},
		})
		recorder := performRequest(t, engine, http.MethodPost, "/api/bridges/brg-slack/verify", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		var response contract.BridgeVerifyResponse
		decodeJSONResponse(t, recorder, &response)
		if response.BridgeInstanceID != "brg-slack" || len(response.Checks) != 1 ||
			response.Checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("response = %#v", response)
		}
	})

	t.Run("Should return provider webhook registration status", func(t *testing.T) {
		t.Parallel()

		engine := newBridgeControlTestRouter(t, stubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				return &bridgepkg.BridgeInstance{ID: "brg-telegram", ExtensionName: "telegram"}, nil
			},
			RegisterBridgeWebhookFn: func(
				context.Context,
				string,
				bridgepkg.BridgeWebhookRegistrationRequest,
			) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
				return bridgepkg.BridgeWebhookRegistrationResponse{
					Status:      bridgepkg.BridgeCheckStatusWarn,
					Remediation: "Retry after the provider timeout.",
				}, nil
			},
		})
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/bridges/brg-telegram/webhook/register",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		var response contract.BridgeWebhookRegistrationResponse
		decodeJSONResponse(t, recorder, &response)
		if response.BridgeInstanceID != "brg-telegram" ||
			response.Status != bridgepkg.BridgeCheckStatusWarn || response.Remediation == "" {
			t.Fatalf("response = %#v", response)
		}
	})

	t.Run("Should return a real delivery acknowledgment", func(t *testing.T) {
		t.Parallel()

		engine := newBridgeControlTestRouter(t, stubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
				return &bridgepkg.BridgeInstance{
					ID:            "brg-telegram",
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "telegram",
					ExtensionName: "telegram",
					DisplayName:   "Telegram",
					Source:        bridgepkg.BridgeInstanceSourceDynamic,
					Enabled:       true,
					Status:        bridgepkg.BridgeStatusReady,
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
					CreatedAt:     now,
					UpdatedAt:     now,
				}, nil
			},
			ResolveDeliveryTargetFn: func(
				_ context.Context,
				request bridgepkg.ResolveDeliveryTargetRequest,
			) (*bridgepkg.DeliveryTarget, error) {
				return &bridgepkg.DeliveryTarget{
					BridgeInstanceID: request.BridgeInstanceID,
					PeerID:           request.PeerID,
					Mode:             request.Mode,
				}, nil
			},
			DeliverBridgeFn: func(
				_ context.Context,
				_ string,
				request bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{
					DeliveryID:      request.Event.DeliveryID,
					Seq:             request.Event.Seq,
					RemoteMessageID: "remote-uds",
				}, nil
			},
		})
		recorder := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/bridges/brg-telegram/send-test",
			[]byte(`{"message":"Connection check","target":{"peer_id":"peer-1","mode":"direct-send"}}`),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		var response contract.BridgeSendTestResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Status != "delivered" || response.RemoteMessageID != "remote-uds" {
			t.Fatalf("response = %#v", response)
		}
	})
}

func newBridgeControlTestRouter(t *testing.T, bridges stubBridgeService) http.Handler {
	t.Helper()

	homePaths := newTestHomePaths(t)
	return newTestRouter(
		t,
		newTestHandlersWithBridges(
			t,
			stubSessionManager{},
			stubObserver{},
			bridges,
			stubWorkspaceService{},
			homePaths,
		),
	)
}
