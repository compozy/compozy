package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/gin-gonic/gin"
)

func TestBridgeControlHandlersPreserveStructuredProviderResults(t *testing.T) {
	t.Run("Should return provider check records without converting failures to logs", func(t *testing.T) {
		t.Parallel()

		_, engine := newBridgeControlHandlerFixture(t, testutil.StubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				return &bridgepkg.BridgeInstance{ID: "brg-1", ExtensionName: "slack"}, nil
			},
			CheckBridgeFn: func(
				_ context.Context,
				extensionName string,
				request bridgepkg.BridgeCheckRequest,
			) (bridgepkg.BridgeCheckResponse, error) {
				if extensionName != "slack" || request.BridgeInstanceID != "brg-1" {
					t.Fatalf("CheckBridge() extension=%q request=%#v", extensionName, request)
				}
				return bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{{
					Check:       "provider.identity",
					Status:      bridgepkg.BridgeCheckStatusFail,
					Remediation: "Replace the bot_token binding.",
				}}}, nil
			},
		})

		response := performRequest(t, engine, http.MethodPost, "/bridges/brg-1/verify", nil)
		if got, want := response.Code, http.StatusOK; got != want {
			t.Fatalf("verify status = %d, want %d; body=%s", got, want, response.Body.String())
		}
		var payload contract.BridgeVerifyResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.BridgeInstanceID != "brg-1" || len(payload.Checks) != 1 ||
			payload.Checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("verify payload = %#v", payload)
		}
	})

	t.Run("Should return the typed Telegram webhook registration result", func(t *testing.T) {
		t.Parallel()

		_, engine := newBridgeControlHandlerFixture(t, testutil.StubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				return &bridgepkg.BridgeInstance{ID: "brg-tg", ExtensionName: "telegram"}, nil
			},
			RegisterBridgeWebhookFn: func(
				_ context.Context,
				extensionName string,
				request bridgepkg.BridgeWebhookRegistrationRequest,
			) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
				if extensionName != "telegram" || request.BridgeInstanceID != "brg-tg" {
					t.Fatalf("RegisterBridgeWebhook() extension=%q request=%#v", extensionName, request)
				}
				return bridgepkg.BridgeWebhookRegistrationResponse{Status: bridgepkg.BridgeCheckStatusPass}, nil
			},
		})

		response := performRequest(t, engine, http.MethodPost, "/bridges/brg-tg/webhook/register", nil)
		if got, want := response.Code, http.StatusOK; got != want {
			t.Fatalf("register status = %d, want %d; body=%s", got, want, response.Body.String())
		}
		var payload contract.BridgeWebhookRegistrationResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.BridgeInstanceID != "brg-tg" || payload.Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("registration payload = %#v", payload)
		}
	})
}

func TestBridgeSendTestUsesRealDeliveryWhileDryRunDoesNot(t *testing.T) {
	t.Run("Should call the extension delivery transport only for send-test", func(t *testing.T) {
		t.Parallel()

		deliveryCalls := 0
		service := testutil.StubBridgeService{
			GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
				now := time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC)
				return &bridgepkg.BridgeInstance{
					ID:            "brg-1",
					Platform:      "slack",
					ExtensionName: "slack",
					DisplayName:   "Slack support",
					Scope:         bridgepkg.ScopeGlobal,
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
				extensionName string,
				request bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				deliveryCalls++
				if extensionName != "slack" || request.Event.Content.Text != "Connection check" ||
					request.Event.EventType != bridgepkg.DeliveryEventTypeFinal || !request.Event.Final {
					t.Fatalf("DeliverBridge() extension=%q request=%#v", extensionName, request)
				}
				return bridgepkg.DeliveryAck{
					DeliveryID:      request.Event.DeliveryID,
					Seq:             request.Event.Seq,
					RemoteMessageID: "remote-1",
				}, nil
			},
		}
		_, engine := newBridgeControlHandlerFixture(t, service)
		body, err := json.Marshal(contract.BridgeSendTestRequest{
			Message: "Connection check",
			Target: contract.BridgeDeliveryTargetInput{
				PeerID: "peer-1",
				Mode:   bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		sent := performRequest(t, engine, http.MethodPost, "/bridges/brg-1/send-test", body)
		if got, want := sent.Code, http.StatusOK; got != want {
			t.Fatalf("send-test status = %d, want %d; body=%s", got, want, sent.Body.String())
		}
		if deliveryCalls != 1 {
			t.Fatalf("delivery calls after send-test = %d, want 1", deliveryCalls)
		}

		dryRun := performRequest(t, engine, http.MethodPost, "/bridges/brg-1/test-delivery", body)
		if got, want := dryRun.Code, http.StatusOK; got != want {
			t.Fatalf("test-delivery status = %d, want %d; body=%s", got, want, dryRun.Body.String())
		}
		if deliveryCalls != 1 {
			t.Fatalf("delivery calls after dry-run = %d, want still 1", deliveryCalls)
		}
	})

	indeterminateTests := []struct {
		name          string
		ack           func(bridgepkg.DeliveryRequest) bridgepkg.DeliveryAck
		wantError     string
		forbiddenText string
	}{
		{
			name: "Should report an indeterminate committed mutation without claiming delivery",
			ack: func(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryAck {
				return bridgepkg.DeliveryAck{
					DeliveryID: request.Event.DeliveryID,
					Seq:        request.Event.Seq,
					Outcome:    bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable,
					Error: &bridgepkg.DeliveryErrorDetail{
						Message: "provider accepted the mutation but omitted api_key=sk-send-test-secret",
					},
				}
			},
			wantError:     "[REDACTED]",
			forbiddenText: "sk-send-test-secret",
		},
		{
			name: "Should treat a successful post without a remote identity as indeterminate",
			ack: func(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryAck {
				return bridgepkg.DeliveryAck{
					DeliveryID: request.Event.DeliveryID,
					Seq:        request.Event.Seq,
				}
			},
			wantError: bridgepkg.DeliveryResultUnavailableMessage,
		},
	}
	for _, test := range indeterminateTests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := testutil.StubBridgeService{
				GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
					now := time.Date(2026, 7, 13, 18, 0, 0, 0, time.UTC)
					return &bridgepkg.BridgeInstance{
						ID:            "brg-committed",
						Platform:      "slack",
						ExtensionName: "slack",
						DisplayName:   "Slack committed outcome",
						Scope:         bridgepkg.ScopeGlobal,
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
					return test.ack(request), nil
				},
			}
			_, engine := newBridgeControlHandlerFixture(t, service)
			body, err := json.Marshal(contract.BridgeSendTestRequest{
				Message: "Connection check",
				Target: contract.BridgeDeliveryTargetInput{
					PeerID: "peer-committed",
					Mode:   bridgepkg.DeliveryModeDirectSend,
				},
			})
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			response := performRequest(
				t,
				engine,
				http.MethodPost,
				"/bridges/brg-committed/send-test",
				body,
			)
			if got, want := response.Code, http.StatusOK; got != want {
				t.Fatalf("send-test status = %d, want %d; body=%s", got, want, response.Body.String())
			}
			var payload contract.BridgeSendTestResponse
			testutil.DecodeJSONResponse(t, response, &payload)
			if got, want := payload.Status, contract.BridgeSendTestStatusCommittedResultUnavailable; got != want {
				t.Fatalf("send-test payload status = %q, want %q", got, want)
			}
			if payload.RemoteMessageID != "" {
				t.Fatalf("send-test payload remote message id = %q, want empty", payload.RemoteMessageID)
			}
			if payload.Error == nil || !strings.Contains(payload.Error.Message, test.wantError) ||
				(test.forbiddenText != "" && strings.Contains(payload.Error.Message, test.forbiddenText)) {
				t.Fatalf(
					"send-test payload error = %#v, want %q without %q",
					payload.Error,
					test.wantError,
					test.forbiddenText,
				)
			}
		})
	}
}

func newBridgeControlHandlerFixture(
	t *testing.T,
	service testutil.StubBridgeService,
) (*core.BaseHandlers, *gin.Engine) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	handlers := &core.BaseHandlers{Bridges: service}
	engine := gin.New()
	engine.POST("/bridges/:id/verify", handlers.VerifyBridge)
	engine.POST("/bridges/:id/send-test", handlers.SendBridgeTest)
	engine.POST("/bridges/:id/webhook/register", handlers.RegisterBridgeWebhook)
	engine.POST("/bridges/:id/test-delivery", handlers.TestBridgeDelivery)
	return handlers, engine
}
