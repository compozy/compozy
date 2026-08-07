package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

type gatewayServiceStub struct {
	revoke func(context.Context, string) (gateway.RevokeResult, error)
	calls  *atomic.Int32
}

func (s gatewayServiceStub) Status(context.Context) (gateway.Status, error) {
	return gateway.Status{}, nil
}

func (s gatewayServiceStub) SetSurfaceExposure(
	context.Context,
	gateway.SurfaceExposureRequest,
) (gateway.Status, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return gateway.Status{}, nil
}

func (s gatewayServiceStub) EnableProvider(
	context.Context,
	gateway.ProviderEnableRequest,
) (gateway.Status, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return gateway.Status{}, nil
}

func (s gatewayServiceStub) DisableProvider(context.Context, gateway.Tier, string) (gateway.Status, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return gateway.Status{}, nil
}

func (s gatewayServiceStub) MintPairing(context.Context, gateway.PairingRequest) (gateway.PairingArtifact, error) {
	return gateway.PairingArtifact{}, nil
}

func (s gatewayServiceStub) RedeemPairing(context.Context, gateway.RedeemRequest) (gateway.IssuedCredential, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return gateway.IssuedCredential{}, nil
}

func (s gatewayServiceStub) ListDevices(context.Context) ([]gateway.DeviceSession, error) {
	return nil, nil
}

func (s gatewayServiceStub) RevokeDevice(ctx context.Context, id string) (gateway.RevokeResult, error) {
	if s.revoke == nil {
		return gateway.RevokeResult{}, nil
	}
	return s.revoke(ctx, id)
}

func (s gatewayServiceStub) RenameDevice(context.Context, string, string) (gateway.DeviceSession, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return gateway.DeviceSession{}, nil
}

func (s gatewayServiceStub) MintStreamTicket(context.Context, string) (gateway.StreamTicket, error) {
	return gateway.StreamTicket{}, nil
}

func TestGatewayPayloadMappings(t *testing.T) {
	t.Parallel()

	t.Run("Should map pairing, issued credential, and stream ticket fields [UT-050]", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, time.August, 6, 12, 1, 0, 0, time.UTC)
		pairing := GatewayPairingArtifactPayload(gateway.PairingArtifact{
			Artifact: "cmp_pair_artifact", ExpiresAt: expiresAt,
		})
		if pairing.Artifact != "cmp_pair_artifact" || pairing.ExpiresAt != expiresAt {
			t.Fatalf("GatewayPairingArtifactPayload() = %#v", pairing)
		}

		issued := GatewayIssuedCredentialPayload(gateway.IssuedCredential{
			Device: gateway.DeviceSession{
				ID: "device-1", Name: "Laptop", ActorKind: gateway.ActorKindCLIProfile,
				PairingOrigin: "private", RevokeEpoch: 3, CreatedAt: expiresAt,
			},
			Credential: "cmp_dev_credential",
		})
		if issued.Credential != "cmp_dev_credential" || issued.Device.ID != "device-1" ||
			issued.Device.Name != "Laptop" || issued.Device.ActorKind != "cli_profile" ||
			issued.Device.PairingOrigin != "private" || issued.Device.RevokeEpoch != 3 ||
			issued.Device.CreatedAt != expiresAt {
			t.Fatalf("GatewayIssuedCredentialPayload() = %#v", issued)
		}

		ticket := GatewayStreamTicketPayload(gateway.StreamTicket{
			Ticket: "cmp_stream_ticket", ExpiresAt: expiresAt,
		})
		if ticket.Ticket != "cmp_stream_ticket" || ticket.ExpiresAt != expiresAt {
			t.Fatalf("GatewayStreamTicketPayload() = %#v", ticket)
		}
	})

	t.Run("Should map every gateway status and device field", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
		lastSeenAt := createdAt.Add(time.Minute)
		revokedAt := createdAt.Add(2 * time.Minute)
		payload := GatewayStatusPayload(gateway.Status{
			Enabled: true,
			Changed: true,
			Tiers: []gateway.TierStatus{{
				Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
				Observed: "up", ListenerAddress: "127.0.0.1:43123", Advertised: true,
			}},
			Surfaces: []gateway.SurfaceStatus{{
				Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate,
				Desired: gateway.DesiredEnabled, Observed: gateway.SurfaceOn, Generation: 7,
			}},
			Providers: []gateway.ProviderStatus{{
				Name: "tailscale", Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
				Observed: gateway.ProviderUp, Generation: 8, Health: gateway.HealthHealthy,
				Cause: "verified",
			}},
			Addresses: []gateway.AddressStatus{{Tier: gateway.TierPrivate, Address: "https://private", Live: true}},
			Devices: []gateway.DeviceSession{{
				ID: "device-1", Name: "Laptop", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local", RevokeEpoch: 3, CreatedAt: createdAt,
				LastSeenAt: lastSeenAt, RevokedAt: revokedAt,
			}},
			Refusal: &gateway.Refusal{Cause: "unverified", Fix: "verify endpoint"},
		})

		if !payload.Enabled || !payload.Changed || len(payload.Tiers) != 1 || len(payload.Surfaces) != 1 ||
			len(payload.Providers) != 1 || len(payload.Addresses) != 1 || len(payload.Devices) != 1 {
			t.Fatalf("GatewayStatusPayload() omitted fields: %#v", payload)
		}
		device := payload.Devices[0]
		if device.ID != "device-1" || device.Name != "Laptop" || device.ActorKind != "operator_device" ||
			device.PairingOrigin != "local" || device.RevokeEpoch != 3 || device.CreatedAt != createdAt ||
			device.LastSeenAt == nil || *device.LastSeenAt != lastSeenAt ||
			device.RevokedAt == nil || *device.RevokedAt != revokedAt {
			t.Fatalf("GatewayStatusPayload().Devices[0] = %#v", device)
		}
		if payload.Refusal == nil || payload.Refusal.Cause != "unverified" || payload.Refusal.Fix != "verify endpoint" {
			t.Fatalf("GatewayStatusPayload().Refusal = %#v", payload.Refusal)
		}
		if payload.Tiers[0].ListenerAddress != "127.0.0.1:43123" {
			t.Fatalf("GatewayStatusPayload().Tiers[0] = %#v", payload.Tiers[0])
		}
	})

	t.Run("Should surface the idempotent revoke result in the response body", func(t *testing.T) {
		t.Parallel()

		gin.SetMode(gin.TestMode)
		handlers := &BaseHandlers{Gateway: gatewayServiceStub{revoke: func(
			_ context.Context,
			id string,
		) (gateway.RevokeResult, error) {
			return gateway.RevokeResult{
				Device:   gateway.DeviceSession{ID: id, Name: "Laptop", RevokeEpoch: 2},
				Changed:  true,
				Canceled: 4,
			}, nil
		}}}
		router := gin.New()
		router.DELETE("/devices/:id", handlers.RevokeGatewayDevice)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodDelete, "/devices/device-1", http.NoBody,
		)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.GatewayRevokePayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Device.ID != "device-1" || !payload.Changed || payload.Canceled != 4 {
			t.Fatalf("revoke payload = %#v", payload)
		}
	})

	t.Run("Should redact and bound provider and refusal diagnostics", func(t *testing.T) {
		t.Parallel()

		providerSecret := "sk-gateway-provider-secret"
		refusalSecret := "gateway-refusal-secret"
		payload := GatewayStatusPayload(gateway.Status{
			Providers: []gateway.ProviderStatus{{
				Name: "provider", Cause: "api_key=" + providerSecret + strings.Repeat("x", maxDiagnosticPayloadBytes),
			}},
			Refusal: &gateway.Refusal{
				Cause: "token=" + refusalSecret,
				Fix:   "replace token=" + refusalSecret + strings.Repeat("y", maxDiagnosticPayloadBytes),
			},
		})
		for label, value := range map[string]string{
			"provider cause": payload.Providers[0].Cause,
			"refusal cause":  payload.Refusal.Cause,
			"refusal fix":    payload.Refusal.Fix,
		} {
			if strings.Contains(value, providerSecret) || strings.Contains(value, refusalSecret) {
				t.Fatalf("%s leaked secret: %q", label, value)
			}
			if len(value) > maxDiagnosticPayloadBytes {
				t.Fatalf("%s length = %d, want <= %d", label, len(value), maxDiagnosticPayloadBytes)
			}
		}
	})
}

func TestGatewayErrorMappings(t *testing.T) {
	t.Parallel()

	t.Run("Should map documented sentinels to stable status and code pairs [UT-106]", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			err        error
			wantStatus int
			wantCode   string
		}{
			{
				name: "Should map expired pairing", err: gateway.ErrPairingExpired,
				wantStatus: http.StatusGone, wantCode: "gateway_pairing_expired",
			},
			{
				name: "Should map pairing capacity", err: gateway.ErrPairingLimit,
				wantStatus: http.StatusTooManyRequests, wantCode: "gateway_pairing_limit",
			},
			{
				name: "Should map digest confirmation", err: gateway.ErrDigestConfirmationRequired,
				wantStatus: http.StatusPreconditionRequired, wantCode: "gateway_digest_confirmation_required",
			},
			{
				name: "Should map stale provider trust", err: gateway.ErrProviderTrustStale,
				wantStatus: http.StatusConflict, wantCode: "gateway_provider_trust_stale",
			},
			{
				name: "Should map forbidden ingress", err: gateway.ErrIngressForbidden,
				wantStatus: http.StatusForbidden, wantCode: "gateway_ingress_forbidden",
			},
			{
				name: "Should map a revoked device", err: gateway.ErrDeviceRevoked,
				wantStatus: http.StatusUnauthorized, wantCode: "gateway_device_revoked",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if got := StatusForGatewayError(test.err); got != test.wantStatus {
					t.Fatalf("StatusForGatewayError() = %d, want %d", got, test.wantStatus)
				}
				if got := GatewayErrorCode(test.err); got != test.wantCode {
					t.Fatalf("GatewayErrorCode() = %q, want %q", got, test.wantCode)
				}
			})
		}
	})

	t.Run("Should mask unknown HTTP failures and preserve raw UDS failures [UT-107]", func(t *testing.T) {
		t.Parallel()

		failure := errors.New("database password leaked")
		masked := ErrorPayloadForStatus(StatusForGatewayError(failure), failure, true)
		if masked.Error != http.StatusText(http.StatusInternalServerError) || masked.Code != "" {
			t.Fatalf("masked payload = %#v", masked)
		}
		raw := ErrorPayloadForStatus(StatusForGatewayError(failure), failure, false)
		if raw.Error != failure.Error() || raw.Code != "" {
			t.Fatalf("raw payload = %#v", raw)
		}
	})
}

func TestGatewayRequestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		mount  func(*gin.Engine, *BaseHandlers)
	}{
		{
			name: "Should reject an unknown surface before invoking policy", method: http.MethodPost,
			path: "/surfaces", body: `{"surface":"unknown","tier":"private","desired":"enabled"}`,
			mount: func(router *gin.Engine, handlers *BaseHandlers) {
				router.POST("/surfaces", handlers.SetGatewaySurface)
			},
		},
		{
			name: "Should reject an unknown provider tier before invoking policy", method: http.MethodPost,
			path: "/providers/tailscale/enable",
			body: `{"tier":"unknown","install_source":"builtin:tailscale"}`,
			mount: func(router *gin.Engine, handlers *BaseHandlers) {
				router.POST("/providers/:name/enable", handlers.EnableGatewayProvider)
			},
		},
		{
			name: "Should reject an unknown disable tier before invoking policy", method: http.MethodPost,
			path: "/providers/tailscale/disable?tier=unknown",
			mount: func(router *gin.Engine, handlers *BaseHandlers) {
				router.POST("/providers/:name/disable", handlers.DisableGatewayProvider)
			},
		},
		{
			name: "Should reject a blank device name before invoking storage", method: http.MethodPatch,
			path: "/devices/device-1", body: `{"name":"   "}`,
			mount: func(router *gin.Engine, handlers *BaseHandlers) {
				router.PATCH("/devices/:id", handlers.RenameGatewayDevice)
			},
		},
		{
			name: "Should reject an unknown actor kind before redeeming an artifact", method: http.MethodPost,
			path: "/pairings/redeem",
			body: `{"artifact":"cpz_gwp_pairing","name":"Laptop","actor_kind":"unknown"}`,
			mount: func(router *gin.Engine, handlers *BaseHandlers) {
				router.POST("/pairings/redeem", handlers.RedeemGatewayPairing)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			handlers := &BaseHandlers{Gateway: gatewayServiceStub{calls: &calls}}
			router := gin.New()
			test.mount(router, handlers)
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				t.Context(), test.method, test.path, strings.NewReader(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					http.StatusBadRequest,
					response.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if payload.Code != "gateway_invalid_request" {
				t.Fatalf("error payload = %#v, want gateway_invalid_request", payload)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("gateway service calls = %d, want 0", got)
			}
		})
	}
}
