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

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type gatewayServiceStub struct {
	revoke  func(context.Context, string) (gateway.RevokeResult, error)
	bind    func(context.Context, gateway.IngressBindRequest) (gateway.IngressBinding, error)
	unbind  func(context.Context, gateway.IngressUnbindRequest) (gateway.UnbindResult, error)
	project func(context.Context, gateway.IngressSubjectRef) (gateway.IngressProjection, error)
	calls   *atomic.Int32
}

func (s gatewayServiceStub) Status(context.Context) (gateway.Status, error) {
	return gateway.Status{}, nil
}

func (s gatewayServiceStub) Audit(context.Context) (gateway.AuditReport, error) {
	return gateway.AuditReport{Ran: true, NoFindings: true, Findings: []gateway.AuditFinding{}}, nil
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

func (s gatewayServiceStub) ResolveIngressSubject(
	context.Context,
	gateway.IngressSubjectRef,
) (gateway.IngressSubject, error) {
	return gateway.IngressSubject{}, gateway.ErrIngressSubjectNotFound
}

func (s gatewayServiceStub) BindIngress(
	ctx context.Context,
	request gateway.IngressBindRequest,
) (gateway.IngressBinding, error) {
	if s.bind != nil {
		return s.bind(ctx, request)
	}
	return gateway.IngressBinding{}, gateway.ErrIngressSubjectNotFound
}

func (s gatewayServiceStub) UnbindIngress(
	ctx context.Context,
	request gateway.IngressUnbindRequest,
) (gateway.UnbindResult, error) {
	if s.unbind != nil {
		return s.unbind(ctx, request)
	}
	return gateway.UnbindResult{}, gateway.ErrIngressSubjectNotFound
}

func (s gatewayServiceStub) ProjectIngress(
	ctx context.Context,
	ref gateway.IngressSubjectRef,
) (gateway.IngressProjection, error) {
	if s.project != nil {
		return s.project(ctx, ref)
	}
	return gateway.IngressProjection{}, gateway.ErrIngressSubjectNotFound
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

	t.Run("Should remove raw claim tokens from every audit payload field [UT-120]", func(t *testing.T) {
		t.Parallel()

		const secret = "compozy_claim_gateway-output-secret"
		data := marshalGatewayAuditPayload(t, gatewayAuditReportWithSecret(secret))
		if strings.Contains(string(data), secret) {
			t.Fatalf("gateway audit payload leaked claim token: %s", data)
		}
		if !strings.Contains(string(data), "[REDACTED]") {
			t.Fatalf("gateway audit payload omitted redaction marker: %s", data)
		}
	})

	t.Run("Should remove device credentials pairing artifacts and stream tickets [UT-121]", func(t *testing.T) {
		t.Parallel()

		for _, secret := range []string{
			"cpz_gwd_device-credential-material-123456",
			"cpz_gwp_pairing-artifact-material-123456",
			"cpz_gwt_stream-ticket-material-123456",
		} {
			t.Run("Should redact "+secret[:7], func(t *testing.T) {
				t.Parallel()
				data := marshalGatewayAuditPayload(t, gatewayAuditReportWithSecret(secret))
				if strings.Contains(string(data), secret) {
					t.Fatalf("gateway audit payload leaked gateway secret: %s", data)
				}
			})
		}
	})

	t.Run("Should redact secret-shaped values even when stored in the wrong field [UT-122]", func(t *testing.T) {
		t.Parallel()

		const secret = "sk-gateway-wrong-field-secret-value"
		data := marshalGatewayAuditPayload(t, gatewayAuditReportWithSecret(secret))
		if strings.Contains(string(data), secret) {
			t.Fatalf("gateway audit payload leaked wrong-field secret: %s", data)
		}
		unbind, err := json.Marshal(GatewayIngressUnbindPayload(gateway.UnbindResult{
			Subject: gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: secret},
			Changed: true,
		}))
		if err != nil {
			t.Fatalf("json.Marshal(GatewayIngressUnbindPayload()) error = %v", err)
		}
		if strings.Contains(string(unbind), secret) {
			t.Fatalf("gateway ingress unbind payload leaked wrong-field secret: %s", unbind)
		}
	})
}

func gatewayAuditReportWithSecret(secret string) gateway.AuditReport {
	status := gateway.Status{
		Enabled: true,
		Providers: []gateway.ProviderStatus{{
			Name: secret, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
			Observed: gateway.ProviderDegraded, Health: gateway.HealthDegraded, Cause: secret,
		}},
		Addresses: []gateway.AddressStatus{{
			Tier: gateway.TierPrivate, Address: "https://gateway.test/" + secret, Live: true,
		}},
		Devices: []gateway.DeviceSession{{
			ID: "dev-safe", Name: secret, PairingOrigin: secret,
		}},
		Bindings: []gateway.IngressBindingStatus{{
			Subject:     gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: secret},
			WorkspaceID: secret, URL: "https://gateway.test/" + secret,
			Reachability: gateway.IngressReachabilityReconfirmation,
		}},
		Refusal: &gateway.Refusal{Cause: secret, Fix: "remove " + secret},
	}
	return gateway.AuditReport{
		Ran: true, Status: status,
		Auth:             gateway.AuditAuthPosture{Mode: secret, RequiredRemotely: true},
		DeviceHighlights: gateway.AuditDeviceHighlights{StaleDeviceIDs: []string{secret}},
		Findings: []gateway.AuditFinding{{
			ID: "gateway.test", Severity: gateway.AuditSeverityCritical,
			Summary: secret, Remediation: "remove " + secret, Resource: secret,
		}},
	}
}

func marshalGatewayAuditPayload(t *testing.T, report gateway.AuditReport) []byte {
	t.Helper()
	data, err := json.Marshal(GatewayAuditPayload(report))
	if err != nil {
		t.Fatalf("json.Marshal(GatewayAuditPayload()) error = %v", err)
	}
	return data
}

func TestGatewayErrorMappings(t *testing.T) {
	t.Parallel()

	t.Run("Should map documented sentinels to stable status and code pairs [UT-106]", func(t *testing.T) {
		t.Parallel()

		for _, mapping := range gatewayErrorMappings {
			t.Run("Should map "+mapping.code, func(t *testing.T) {
				t.Parallel()
				if got := StatusForGatewayError(mapping.target); got != mapping.status {
					t.Fatalf("StatusForGatewayError() = %d, want %d", got, mapping.status)
				}
				if got := GatewayErrorCode(mapping.target); got != mapping.code {
					t.Fatalf("GatewayErrorCode() = %q, want %q", got, mapping.code)
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
		{
			name:   "Should reject a malformed client-provisioned credential before redeeming an artifact",
			method: http.MethodPost,
			path:   "/pairings/redeem",
			body: `{"artifact":"cpz_gwp_pairing","name":"Laptop","actor_kind":"cli_profile",` +
				`"credential":"cpz_gwd_weak"}`,
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

func TestGatewayIngressAgentCallerBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should pass only the daemon-validated agent workspace into a binding mutation", func(t *testing.T) {
		t.Parallel()

		ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: "trigger-1"}
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
			Gateway: gatewayServiceStub{
				bind: func(_ context.Context, request gateway.IngressBindRequest) (gateway.IngressBinding, error) {
					if request.Caller == nil || request.Caller.SessionID != "sess-agent" ||
						request.Caller.AgentName != "coder" || request.Caller.WorkspaceID != "ws-agent" {
						t.Fatalf("BindIngress() caller = %#v", request.Caller)
					}
					return gateway.IngressBinding{Subject: request.Subject, Changed: true}, nil
				},
				project: func(context.Context, gateway.IngressSubjectRef) (gateway.IngressProjection, error) {
					return gateway.IngressProjection{
						Subject:      ref,
						Reachability: gateway.IngressReachabilityUnconfirmed,
					}, nil
				},
			},
		}
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/ingress-bindings", handlers.BindGatewayIngress)
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
				Gateway: gatewayServiceStub{
					bind: func(
						_ context.Context,
						request gateway.IngressBindRequest,
					) (gateway.IngressBinding, error) {
						if request.Caller == nil || request.Caller.WorkspaceID != "ws-agent" {
							t.Fatalf("BindIngress() caller = %#v", request.Caller)
						}
						return gateway.IngressBinding{}, gateway.ErrIngressForbidden
					},
					unbind: func(
						_ context.Context,
						request gateway.IngressUnbindRequest,
					) (gateway.UnbindResult, error) {
						if request.Caller == nil || request.Caller.WorkspaceID != "ws-agent" {
							t.Fatalf("UnbindIngress() caller = %#v", request.Caller)
						}
						return gateway.UnbindResult{}, gateway.ErrIngressForbidden
					},
				},
			}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/ingress-bindings", handlers.BindGatewayIngress)
			router.DELETE("/ingress-bindings/:subject_kind/:subject_id", handlers.UnbindGatewayIngress)
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
				!strings.Contains(response.Body.String(), "gateway_ingress_forbidden") {
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
				!strings.Contains(unbindResponse.Body.String(), "gateway_ingress_forbidden") {
				t.Fatalf("unbind response = (%d, %q), want deterministic ingress denial",
					unbindResponse.Code, unbindResponse.Body.String())
			}
		},
	)
}
