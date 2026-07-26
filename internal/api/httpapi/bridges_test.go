package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/gin-gonic/gin"
)

func TestBridgeHandlersShouldHandleBridgeRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		path    string
		body    []byte
		bridges stubBridgeService
		assert  func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "ShouldListBridgeProviders",
			method: http.MethodGet,
			path:   "/api/bridges/providers",
			bridges: stubBridgeService{
				ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
					return []bridgepkg.BridgeProvider{{
						Platform:      "telegram",
						ExtensionName: "telegram-reference",
						DisplayName:   "Telegram",
						Description:   "Reference Telegram bridge adapter",
						SecretSlots: []bridgepkg.BridgeSecretSlot{{
							Name:        "bot_token",
							Description: "Bot token",
							Required:    true,
						}},
						ConfigSchema: &bridgepkg.BridgeProviderConfigSchema{
							Schema:  "agh.bridge.telegram",
							Version: "v1",
						},
						Enabled: true,
						State:   "active",
						Health:  "healthy",
					}}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}

				var response contract.BridgeProvidersResponse
				decodeJSONResponse(t, recorder, &response)
				if got, want := len(response.Providers), 1; got != want {
					t.Fatalf("len(providers) = %d, want %d", got, want)
				}
				if response.Providers[0].ExtensionName != "telegram-reference" {
					t.Fatalf("provider = %#v", response.Providers[0])
				}
				if len(response.Providers[0].SecretSlots) != 1 ||
					response.Providers[0].SecretSlots[0].Name != "bot_token" {
					t.Fatalf("provider secret slots = %#v", response.Providers[0].SecretSlots)
				}
				if response.Providers[0].ConfigSchema == nil ||
					response.Providers[0].ConfigSchema.Schema != "agh.bridge.telegram" {
					t.Fatalf("provider config schema = %#v", response.Providers[0].ConfigSchema)
				}
			},
		},
		{
			name:   "Should return Slack manifest for requested instance",
			method: http.MethodGet,
			path:   "/api/bridges/providers/slack/manifest?instance=brg-slack",
			bridges: stubBridgeService{
				GetInstanceFn: func(_ context.Context, id string) (*bridgepkg.BridgeInstance, error) {
					if id != "brg-slack" {
						t.Fatalf("GetInstance() id = %q, want brg-slack", id)
					}
					return &bridgepkg.BridgeInstance{
						ID:             id,
						Scope:          bridgepkg.ScopeGlobal,
						Platform:       "slack",
						ExtensionName:  "slack-reference",
						DisplayName:    "Support agent",
						ProviderConfig: []byte(`{"webhook":{"public_url":"https://bridge.example.com/slack/support"}}`),
					}, nil
				},
				ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
					return []bridgepkg.BridgeProvider{{
						Platform:      "slack",
						ExtensionName: "slack-reference",
						DisplayName:   "Slack",
						Description:   "Connect Slack messages, commands, actions, and reactions to AGH.",
					}}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}
				var response contract.SlackAppManifestResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Manifest.DisplayInformation.Name != "Support agent" ||
					response.Manifest.Settings.EventSubscriptions.RequestURL !=
						"https://bridge.example.com/slack/support" {
					t.Fatalf("manifest = %#v", response.Manifest)
				}
			},
		},
		{
			name:    "Should reject Slack manifest without instance",
			method:  http.MethodGet,
			path:    "/api/bridges/providers/slack/manifest",
			bridges: stubBridgeService{},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf(
						"status = %d, want %d; body=%s",
						recorder.Code,
						http.StatusBadRequest,
						recorder.Body.String(),
					)
				}
				if !strings.Contains(recorder.Body.String(), "instance query parameter is required") {
					t.Fatalf("body = %s, want instance query remediation", recorder.Body.String())
				}
			},
		},
		{
			name:   "ShouldCreateBridgeInstance",
			method: http.MethodPost,
			path:   "/api/bridges",
			body: []byte(
				`{"scope":"workspace","workspace_id":"ws-alpha","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"dm_policy":"pairing","routing_policy":{"include_peer":true},"provider_config":{"mode":"bot","tenant":"acme"},"delivery_defaults":{"peer_id":"peer-default","mode":"reply"}}`,
			),
			bridges: stubBridgeService{
				CreateInstanceFn: func(_ context.Context, req bridgepkg.CreateInstanceRequest) (*bridgepkg.BridgeInstance, error) {
					if req.Scope != bridgepkg.ScopeWorkspace || req.WorkspaceID != "ws-alpha" ||
						req.Platform != "telegram" ||
						req.ExtensionName != "ext-telegram" ||
						req.DisplayName != "Support" {
						t.Fatalf("CreateInstance() req = %#v", req)
					}
					if !req.Enabled || req.Status != bridgepkg.BridgeStatusStarting || !req.RoutingPolicy.IncludePeer {
						t.Fatalf("CreateInstance() lifecycle = %#v", req)
					}
					if req.DMPolicy != bridgepkg.BridgeDMPolicyPairing {
						t.Fatalf(
							"CreateInstance().DMPolicy = %q, want %q",
							req.DMPolicy,
							bridgepkg.BridgeDMPolicyPairing,
						)
					}
					if got, want := string(req.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
						t.Fatalf("CreateInstance().ProviderConfig = %s, want %s", got, want)
					}
					if got, want := string(
						req.DeliveryDefaults,
					), `{"mode":"reply","peer_id":"peer-default"}`; got != want {
						t.Fatalf("CreateInstance().DeliveryDefaults = %s, want %s", got, want)
					}
					return &bridgepkg.BridgeInstance{
						ID:               "brg-1",
						Scope:            req.Scope,
						WorkspaceID:      req.WorkspaceID,
						Platform:         req.Platform,
						ExtensionName:    req.ExtensionName,
						DisplayName:      req.DisplayName,
						Enabled:          req.Enabled,
						Status:           req.Status,
						DMPolicy:         req.DMPolicy,
						RoutingPolicy:    req.RoutingPolicy,
						ProviderConfig:   req.ProviderConfig,
						DeliveryDefaults: req.DeliveryDefaults,
						Degradation:      req.Degradation,
						CreatedAt:        time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
						UpdatedAt:        time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
					}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusCreated {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
				}

				var response contract.BridgeResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Bridge.ID != "brg-1" || response.Bridge.WorkspaceID != "ws-alpha" ||
					response.Bridge.Status != bridgepkg.BridgeStatusStarting {
					t.Fatalf("response.Bridge = %#v", response.Bridge)
				}
				if got, want := string(response.Bridge.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
					t.Fatalf("response.Bridge.ProviderConfig = %s, want %s", got, want)
				}
			},
		},
		{
			name:   "ShouldListRequestedBridgeRoutes",
			method: http.MethodGet,
			path:   "/api/bridges/brg-1/routes",
			bridges: stubBridgeService{
				ListRoutesFn: func(_ context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeRoute, error) {
					if bridgeInstanceID != "brg-1" {
						t.Fatalf("ListRoutes() bridgeInstanceID = %q, want brg-1", bridgeInstanceID)
					}
					return []bridgepkg.BridgeRoute{{
						RoutingKeyHash:   "hash-1",
						Scope:            bridgepkg.ScopeWorkspace,
						WorkspaceID:      "ws-alpha",
						BridgeInstanceID: "brg-1",
						PeerID:           "peer-1",
						ThreadID:         "thread-1",
						GroupID:          "group-1",
						SessionID:        "sess-1",
						AgentName:        "coder",
						LastActivityAt:   time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
						CreatedAt:        time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
						UpdatedAt:        time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
					}}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}

				var response contract.BridgeRoutesResponse
				decodeJSONResponse(t, recorder, &response)
				if got, want := len(response.Routes), 1; got != want {
					t.Fatalf("len(routes) = %d, want %d", got, want)
				}
				if response.Routes[0].BridgeInstanceID != "brg-1" || response.Routes[0].ThreadID != "thread-1" {
					t.Fatalf("route = %#v", response.Routes[0])
				}
			},
		},
		{
			name:   "ShouldResolveTypedDeliveryTarget",
			method: http.MethodPost,
			path:   "/api/bridges/brg-1/test-delivery",
			body: []byte(
				`{"message":"hello","target":{"peer_id":"peer-1","thread_id":"thread-1","group_id":"group-1","mode":"reply"}}`,
			),
			bridges: stubBridgeService{
				ResolveDeliveryTargetFn: func(_ context.Context, req bridgepkg.ResolveDeliveryTargetRequest) (*bridgepkg.DeliveryTarget, error) {
					if req.BridgeInstanceID != "brg-1" || req.PeerID != "peer-1" || req.ThreadID != "thread-1" ||
						req.GroupID != "group-1" ||
						req.Mode != bridgepkg.DeliveryModeReply {
						t.Fatalf("ResolveDeliveryTarget() req = %#v", req)
					}
					return &bridgepkg.DeliveryTarget{
						BridgeInstanceID: req.BridgeInstanceID,
						PeerID:           req.PeerID,
						ThreadID:         req.ThreadID,
						GroupID:          req.GroupID,
						Mode:             req.Mode,
					}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}

				var response contract.BridgeTestDeliveryResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Status != "resolved" || response.DeliveryTarget.BridgeInstanceID != "brg-1" ||
					response.DeliveryTarget.Mode != bridgepkg.DeliveryModeReply {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:   "ShouldReturnStructuredBridgeVerificationChecks",
			method: http.MethodPost,
			path:   "/api/bridges/brg-slack/verify",
			bridges: stubBridgeService{
				GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
					return &bridgepkg.BridgeInstance{ID: "brg-slack", ExtensionName: "slack"}, nil
				},
				CheckBridgeFn: func(
					_ context.Context,
					extensionName string,
					request bridgepkg.BridgeCheckRequest,
				) (bridgepkg.BridgeCheckResponse, error) {
					if extensionName != "slack" || request.BridgeInstanceID != "brg-slack" {
						t.Fatalf("CheckBridge() extension=%q request=%#v", extensionName, request)
					}
					return bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{{
						Check:       "provider.identity",
						Status:      bridgepkg.BridgeCheckStatusFail,
						Remediation: "Replace the bot_token binding.",
					}}}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}
				var response contract.BridgeVerifyResponse
				decodeJSONResponse(t, recorder, &response)
				if response.BridgeInstanceID != "brg-slack" || len(response.Checks) != 1 ||
					response.Checks[0].Status != bridgepkg.BridgeCheckStatusFail {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:   "ShouldRegisterBridgeWebhookThroughSharedControlHandler",
			method: http.MethodPost,
			path:   "/api/bridges/brg-telegram/webhook/register",
			bridges: stubBridgeService{
				GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
					return &bridgepkg.BridgeInstance{ID: "brg-telegram", ExtensionName: "telegram"}, nil
				},
				RegisterBridgeWebhookFn: func(
					_ context.Context,
					extensionName string,
					request bridgepkg.BridgeWebhookRegistrationRequest,
				) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
					if extensionName != "telegram" || request.BridgeInstanceID != "brg-telegram" {
						t.Fatalf("RegisterBridgeWebhook() extension=%q request=%#v", extensionName, request)
					}
					return bridgepkg.BridgeWebhookRegistrationResponse{
						Status: bridgepkg.BridgeCheckStatusPass,
					}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}
				var response contract.BridgeWebhookRegistrationResponse
				decodeJSONResponse(t, recorder, &response)
				if response.BridgeInstanceID != "brg-telegram" ||
					response.Status != bridgepkg.BridgeCheckStatusPass {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:   "ShouldSendBridgeTestThroughRealDeliveryTransport",
			method: http.MethodPost,
			path:   "/api/bridges/brg-telegram/send-test",
			body: []byte(
				`{"message":"Connection check","target":{"peer_id":"peer-1","mode":"direct-send"}}`,
			),
			bridges: stubBridgeService{
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
					extensionName string,
					request bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					if extensionName != "telegram" || request.Event.Content.Text != "Connection check" {
						t.Fatalf("DeliverBridge() extension=%q request=%#v", extensionName, request)
					}
					return bridgepkg.DeliveryAck{
						DeliveryID:      request.Event.DeliveryID,
						Seq:             request.Event.Seq,
						RemoteMessageID: "remote-1",
					}, nil
				},
			},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				t.Helper()
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}
				var response contract.BridgeSendTestResponse
				decodeJSONResponse(t, recorder, &response)
				if response.Status != "delivered" || response.BridgeInstanceID != "brg-telegram" ||
					response.RemoteMessageID != "remote-1" {
					t.Fatalf("response = %#v", response)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths := newTestHomePaths(t)
			engine := newTestRouter(
				t,
				newTestHandlersWithBridges(
					t,
					stubSessionManager{},
					stubObserver{},
					tc.bridges,
					stubWorkspaceService{},
					homePaths,
				),
			)
			recorder := performRequest(t, engine, tc.method, tc.path, tc.body)
			tc.assert(t, recorder)
		})
	}
}

func TestBridgeRoutesRequirePrivilegedHTTPForMutationsAndControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "Should guard bridge creation",
			method: http.MethodPost,
			path:   "/api/bridges",
			body:   `{}`,
		},
		{
			name:   "Should guard bridge updates",
			method: http.MethodPatch,
			path:   "/api/bridges/brg-1",
			body:   `{}`,
		},
		{name: "Should guard bridge enable", method: http.MethodPost, path: "/api/bridges/brg-1/enable"},
		{name: "Should guard bridge disable", method: http.MethodPost, path: "/api/bridges/brg-1/disable"},
		{name: "Should guard bridge restart", method: http.MethodPost, path: "/api/bridges/brg-1/restart"},
		{
			name:   "Should guard bridge verification",
			method: http.MethodPost,
			path:   "/api/bridges/brg-1/verify",
		},
		{
			name:   "Should guard real test delivery",
			method: http.MethodPost,
			path:   "/api/bridges/brg-1/send-test",
			body:   `{}`,
		},
		{
			name:   "Should guard dry-run test delivery",
			method: http.MethodPost,
			path:   "/api/bridges/brg-1/test-delivery",
			body:   `{}`,
		},
		{
			name:   "Should guard webhook registration",
			method: http.MethodPost,
			path:   "/api/bridges/brg-1/webhook/register",
		},
		{
			name:   "Should guard secret binding writes",
			method: http.MethodPut,
			path:   "/api/bridges/brg-1/secret-bindings/bot_token",
			body:   `{}`,
		},
		{
			name:   "Should guard secret binding deletion",
			method: http.MethodDelete,
			path:   "/api/bridges/brg-1/secret-bindings/bot_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handlers := newTestHandlersWithBridges(
				t,
				stubSessionManager{},
				stubObserver{},
				stubBridgeService{},
				stubWorkspaceService{},
				newTestHomePaths(t),
			)
			handlers.boundHost = "0.0.0.0"

			engine := gin.New()
			engine.Use(errorMiddleware())
			registerBridgeRoutes(engine.Group("/api"), handlers)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				context.Background(),
				tt.method,
				tt.path,
				strings.NewReader(tt.body),
			)
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
			}
			if !strings.Contains(strings.ToLower(recorder.Body.String()), "loopback") {
				t.Fatalf("body = %s, want loopback remediation", recorder.Body.String())
			}
		})
	}
}
