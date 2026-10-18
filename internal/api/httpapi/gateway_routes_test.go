package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	apispec "github.com/compozy/compozy/internal/api/spec"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

type gatewayHTTPServiceStub struct {
	core.GatewayService
	redeem  func(context.Context, gateway.RedeemRequest) (gateway.IssuedCredential, error)
	acquire func(gateway.Tier, gateway.Surface) (func(), error)
}

func (s gatewayHTTPServiceStub) RedeemPairing(
	ctx context.Context,
	request gateway.RedeemRequest,
) (gateway.IssuedCredential, error) {
	if s.redeem == nil {
		return gateway.IssuedCredential{}, nil
	}
	return s.redeem(ctx, request)
}

func (s gatewayHTTPServiceStub) Acquire(tier gateway.Tier, surface gateway.Surface) (func(), error) {
	if s.acquire == nil {
		return func() {}, nil
	}
	return s.acquire(tier, surface)
}

type gatewayHTTPAuthenticatorStub struct {
	authenticate func(context.Context, string) (gateway.DeviceSession, error)
}

type gatewayStreamAuthenticatorStub struct {
	registry *gateway.LiveConnectionRegistry
	device   gateway.DeviceSession
}

func (s gatewayStreamAuthenticatorStub) Authenticate(
	context.Context,
	string,
) (gateway.DeviceSession, error) {
	return gateway.DeviceSession{}, gateway.ErrDeviceUnauthenticated
}

func (s gatewayStreamAuthenticatorStub) ConsumeStreamTicket(
	context.Context,
	string,
) (gateway.DeviceSession, error) {
	return s.device, nil
}

func (s gatewayStreamAuthenticatorStub) RevalidateForCommit(
	context.Context,
	string,
	uint64,
) error {
	return nil
}

func (s gatewayStreamAuthenticatorStub) AcquireMutation(
	ctx context.Context,
	_ string,
	_ uint64,
) (context.Context, error) {
	return ctx, nil
}

func (s gatewayStreamAuthenticatorStub) Connections() gateway.ConnectionRegistry {
	return s.registry
}

func (s gatewayHTTPAuthenticatorStub) Authenticate(
	ctx context.Context,
	credential string,
) (gateway.DeviceSession, error) {
	if s.authenticate == nil {
		return gateway.DeviceSession{}, gateway.ErrDeviceUnauthenticated
	}
	return s.authenticate(ctx, credential)
}

func (gatewayHTTPAuthenticatorStub) ConsumeStreamTicket(
	context.Context,
	string,
) (gateway.DeviceSession, error) {
	return gateway.DeviceSession{}, gateway.ErrStreamTicketInvalid
}

func (gatewayHTTPAuthenticatorStub) RevalidateForCommit(context.Context, string, uint64) error {
	return nil
}

func (gatewayHTTPAuthenticatorStub) AcquireMutation(
	ctx context.Context,
	_ string,
	_ uint64,
) (context.Context, error) {
	return ctx, nil
}

func TestGatewayTierRouteMatricesIT063IT064(t *testing.T) {
	t.Parallel()

	service := gatewayHTTPServiceStub{}
	authenticator := gatewayHTTPAuthenticatorStub{}
	tests := []struct {
		name       string
		surfaceSet SurfaceSet
		present    []string
		absent     []string
		wantRoutes int
	}{
		{
			name:       "Should expose pairing and management only on the private tier",
			surfaceSet: SurfaceSetPrivate,
			present: []string{
				"POST /api/gateway/pairings/redeem",
				"GET /api/gateway/audit",
				"GET /api/gateway/status",
				"POST /api/gateway/pairings",
				"GET /api/status",
				"GET /api/tasks/:id",
				"GET /api/resources/:kind/:id",
			},
			absent: []string{
				"POST /api/webhooks/global/:endpoint",
				"POST /api/tasks/:id/runs",
				"GET /api/scheduler",
				"PUT /api/resources/:kind/:id",
			},
		},
		{
			name:       "Should expose only self-verifying webhooks on the public ingress tier",
			surfaceSet: SurfaceSetPublicIngress,
			present: []string{
				"POST /api/webhooks/global/:endpoint",
				"POST /api/webhooks/workspaces/:workspace_id/:endpoint",
			},
			absent: []string{
				"GET /api/status",
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"GET /api/gateway/audit",
				"POST /api/gateway/stream-tickets",
			},
			wantRoutes: 2,
		},
		{
			name:       "Should expose safe gateway management without pairing on the public operator tier",
			surfaceSet: SurfaceSetPublicOperator,
			present: []string{
				"GET /api/status",
				"GET /api/tasks/:id",
				"GET /api/resources/:kind/:id",
				"GET /api/gateway/audit",
				"GET /api/gateway/status",
				"POST /api/gateway/surfaces",
				"POST /api/gateway/providers/:name/enable",
				"POST /api/gateway/providers/:name/disable",
				"GET /api/gateway/devices",
				"PATCH /api/gateway/devices/:id",
				"DELETE /api/gateway/devices/:id",
				"POST /api/gateway/stream-tickets",
			},
			absent: []string{
				"POST /api/webhooks/global/:endpoint",
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"POST /api/tasks/:id/runs",
				"GET /api/scheduler",
				"PUT /api/resources/:kind/:id",
			},
		},
		{
			name:       "Should compose ingress ahead of authenticated operator routes on the combined tier",
			surfaceSet: SurfaceSetPublicCombined,
			present: []string{
				"POST /api/webhooks/global/:endpoint",
				"GET /api/status",
				"GET /api/tasks/:id",
				"GET /api/resources/:kind/:id",
				"GET /api/gateway/audit",
				"GET /api/gateway/status",
				"POST /api/gateway/surfaces",
				"POST /api/gateway/providers/:name/enable",
				"POST /api/gateway/providers/:name/disable",
				"GET /api/gateway/devices",
				"PATCH /api/gateway/devices/:id",
				"DELETE /api/gateway/devices/:id",
				"POST /api/gateway/stream-tickets",
			},
			absent: []string{
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"POST /api/tasks/:id/runs",
				"GET /api/scheduler",
				"PUT /api/resources/:kind/:id",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := newGatewaySurfaceTestRouter(test.surfaceSet, service, authenticator)
			routes := gatewayRouteKeys(router.Routes())
			for _, route := range test.present {
				if _, ok := routes[route]; !ok {
					t.Fatalf("route %q is absent from %s listener", route, test.surfaceSet)
				}
			}
			for _, route := range test.absent {
				if _, ok := routes[route]; ok {
					t.Fatalf("route %q is present on %s listener", route, test.surfaceSet)
				}
			}
			if test.wantRoutes > 0 && len(routes) != test.wantRoutes {
				t.Fatalf("route count = %d, want %d; routes = %#v", len(routes), test.wantRoutes, routes)
			}
			for route := range routes {
				if isForbiddenRemoteRoute(route) {
					t.Fatalf("forbidden route %q is present on %s listener", route, test.surfaceSet)
				}
			}
		})
	}

	t.Run("Should derive pairing trust from the listener surface set", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			surfaceSet SurfaceSet
			want       gateway.PairingSource
		}{
			{surfaceSet: SurfaceSetLocal, want: gateway.PairingSourceLocal},
			{surfaceSet: SurfaceSetPrivate, want: gateway.PairingSourcePrivate},
			{surfaceSet: SurfaceSetPublicIngress},
			{surfaceSet: SurfaceSetPublicOperator},
			{surfaceSet: SurfaceSetPublicCombined},
		} {
			handlers := newHandlers(&handlerConfig{surfaceSet: test.surfaceSet})
			if got := gateway.PairingSource(handlers.GatewayPairingSource); got != test.want {
				t.Fatalf("pairing source for %s = %q, want %q", test.surfaceSet, got, test.want)
			}
		}
	})
}

func TestGatewayOperatorResponsesIdentifyListenerTier(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		surfaceSet SurfaceSet
		wantTier   string
	}{
		{
			name:       "Should identify the local operator listener",
			surfaceSet: SurfaceSetLocal,
			wantTier:   string(SurfaceSetLocal),
		},
		{
			name:       "Should identify the private operator listener",
			surfaceSet: SurfaceSetPrivate,
			wantTier:   string(gateway.TierPrivate),
		},
		{
			name:       "Should identify the public operator listener",
			surfaceSet: SurfaceSetPublicOperator,
			wantTier:   string(gateway.TierPublic),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := newGatewaySurfaceTestRouter(
				test.surfaceSet,
				gatewayHTTPServiceStub{},
				gatewayHTTPAuthenticatorStub{},
			)
			request := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "http://127.0.0.1:2123/api/status", http.NoBody,
			)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if got := response.Header().Get(gatewayTierHeader); got != test.wantTier {
				t.Fatalf("%s = %q, want %q", gatewayTierHeader, got, test.wantTier)
			}
		})
	}
}

func isForbiddenRemoteRoute(route string) bool {
	parts := strings.SplitN(route, " ", 2)
	if len(parts) != 2 {
		return true
	}
	method, path := parts[0], parts[1]
	if path == "/api/agent" || strings.HasPrefix(path, "/api/agent/") ||
		path == "/api/internal" || strings.HasPrefix(path, "/api/internal/") ||
		path == "/api/scheduler" || strings.HasPrefix(path, "/api/scheduler/") {
		return true
	}
	if (path == "/api/resources" || strings.HasPrefix(path, "/api/resources/")) &&
		method != http.MethodGet {
		return true
	}
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	return path == "/api/tasks" || strings.HasPrefix(path, "/api/tasks/") ||
		strings.HasPrefix(path, "/api/task-runs/") ||
		strings.HasPrefix(path, "/api/task-reviews/") ||
		strings.HasPrefix(path, "/api/runs/")
}

func TestGatewayIngressRateLimit(t *testing.T) {
	t.Parallel()
	t.Run("Should rate limit public ingress by source and endpoint", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := gateway.NewIngressRateLimiter(1, time.Minute, func() time.Time { return now })
		handlers := newHandlers(&handlerConfig{
			gatewayAdmission: gatewayHTTPServiceStub{},
			ingressLimiter:   limiter,
			surfaceSet:       SurfaceSetPublicIngress,
			boundHost:        "127.0.0.1:2123",
		})
		router := gin.New()
		RegisterSurfaceRoutes(router, handlers, SurfaceSetPublicIngress)

		request := func(endpoint string, remoteAddr string, forwardedFor string) *httptest.ResponseRecorder {
			t.Helper()
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://127.0.0.1:2123/api/webhooks/global/"+endpoint,
				bytes.NewBufferString(`{"payload":"deploy"}`),
			)
			req.RemoteAddr = remoteAddr
			req.Header.Set("X-Forwarded-For", forwardedFor)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			return response
		}

		first := request("deploy--wbh_one", "203.0.113.8:43001", "198.51.100.10")
		if first.Code == http.StatusTooManyRequests {
			t.Fatalf("first delivery status = %d, want request admitted", first.Code)
		}
		limited := request("deploy--wbh_one", "203.0.113.8:43002", "198.51.100.11")
		if limited.Code != http.StatusTooManyRequests {
			t.Fatalf(
				"second delivery status = %d, want %d; body=%s",
				limited.Code,
				http.StatusTooManyRequests,
				limited.Body.String(),
			)
		}
		if got := limited.Header().Get("Retry-After"); got != "60" {
			t.Fatalf("Retry-After = %q, want %q", got, "60")
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(limited.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal(rate-limit response) error = %v", err)
		}
		if payload.Code != "gateway_ingress_rate_limited" {
			t.Fatalf("rate-limit payload = %#v, want gateway_ingress_rate_limited", payload)
		}

		otherEndpoint := request("release--wbh_two", "203.0.113.8:43003", "198.51.100.12")
		if otherEndpoint.Code == http.StatusTooManyRequests {
			t.Fatalf("other endpoint status = %d, want independent endpoint budget", otherEndpoint.Code)
		}
		otherSource := request("deploy--wbh_one", "203.0.113.9:43004", "203.0.113.8")
		if otherSource.Code == http.StatusTooManyRequests {
			t.Fatalf("other source status = %d, want independent source budget", otherSource.Code)
		}
	})
}

func TestGatewayHTTPRouteUnionMatchesDocumentedOperations(t *testing.T) {
	t.Parallel()
	t.Run("Should equal the documented gateway HTTP operation union", func(t *testing.T) {
		t.Parallel()
		assertGatewayHTTPRouteUnionMatchesDocumentedOperations(t)
	})
}

func assertGatewayHTTPRouteUnionMatchesDocumentedOperations(t *testing.T) {
	t.Helper()
	service := gatewayHTTPServiceStub{}
	authenticator := gatewayHTTPAuthenticatorStub{}
	actualSet := make(map[string]struct{})
	for _, surfaceSet := range []SurfaceSet{
		SurfaceSetLocal,
		SurfaceSetPrivate,
		SurfaceSetPublicIngress,
		SurfaceSetPublicOperator,
		SurfaceSetPublicCombined,
	} {
		router := newGatewaySurfaceTestRouter(surfaceSet, service, authenticator)
		for route := range gatewayRouteKeys(router.Routes()) {
			parts := strings.SplitN(route, " ", 2)
			if len(parts) != 2 || !strings.HasPrefix(parts[1], "/api/gateway") {
				continue
			}
			path := strings.NewReplacer(
				":name", "{name}", ":id", "{id}",
				":subject_kind", "{subject_kind}", ":subject_id", "{subject_id}",
			).Replace(parts[1])
			actualSet[parts[0]+" "+path] = struct{}{}
		}
	}
	actual := make([]string, 0, len(actualSet))
	for route := range actualSet {
		actual = append(actual, route)
	}
	slices.Sort(actual)

	want := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if slices.Contains(operation.Transports, apispec.TransportHTTP) &&
			strings.HasPrefix(operation.Path, "/api/gateway") {
			want = append(want, operation.Method+" "+operation.Path)
		}
	}
	slices.Sort(want)
	if !slices.Equal(actual, want) {
		t.Fatalf("HTTP gateway route union = %v, want documented operations %v", actual, want)
	}
}

func TestGatewayTierAuthentication(t *testing.T) {
	t.Parallel()

	t.Run("Should never trust a loopback source or forwarded header as device identity", func(t *testing.T) {
		t.Parallel()

		authCalls := 0
		authenticator := gatewayHTTPAuthenticatorStub{authenticate: func(
			_ context.Context,
			credential string,
		) (gateway.DeviceSession, error) {
			authCalls++
			if credential != "" {
				t.Fatalf("Authenticate() credential = %q, want empty", credential)
			}
			return gateway.DeviceSession{}, gateway.ErrDeviceUnauthenticated
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, gatewayHTTPServiceStub{}, authenticator)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "http://127.0.0.1:2123/api/status", http.NoBody,
		)
		request.RemoteAddr = "127.0.0.1:43123"
		request.Header.Set("X-Forwarded-For", "203.0.113.20")
		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
		}
		if got := response.Header().Get(gatewayTierHeader); got != string(gateway.TierPrivate) {
			t.Fatalf("%s = %q, want %q", gatewayTierHeader, got, gateway.TierPrivate)
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Code != "gateway_device_unauthenticated" {
			t.Fatalf("error code = %q, want gateway_device_unauthenticated", payload.Code)
		}
		if authCalls != 1 {
			t.Fatalf("Authenticate() calls = %d, want 1", authCalls)
		}
	})

	t.Run("Should return browser credentials only through a secure HttpOnly cookie", func(t *testing.T) {
		t.Parallel()

		credential := "cpz_gwd_" + strings.Repeat("z", 43)
		service := gatewayHTTPServiceStub{redeem: func(
			_ context.Context,
			request gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			if request.Kind != gateway.ActorKindOperatorDevice || request.Source != gateway.PairingSourcePrivate {
				t.Fatalf("RedeemPairing() request = %#v", request)
			}
			return gateway.IssuedCredential{
				Device: gateway.DeviceSession{
					ID: "device-browser", Name: request.Name, ActorKind: request.Kind,
					PairingOrigin: string(request.Source), CreatedAt: time.Date(2026, 8, 6, 14, 0, 0, 0, time.UTC),
				},
				Credential: credential,
			}, nil
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, service, gatewayHTTPAuthenticatorStub{})
		body := bytes.NewBufferString(
			`{"artifact":"cpz_gwp_pairing","name":"Browser","actor_kind":"operator_device"}`,
		)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://127.0.0.1:2123/api/gateway/pairings/redeem",
			body,
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://127.0.0.1:2123")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		if got := response.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
		}
		if strings.Contains(response.Body.String(), credential) {
			t.Fatalf("browser response body leaked raw credential: %s", response.Body.String())
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies = %#v, want one device credential cookie", cookies)
		}
		cookie := cookies[0]
		if cookie.Name != gatewayDeviceCookieName || cookie.Value != credential || !cookie.Secure ||
			!cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
			t.Fatalf("device credential cookie = %#v", cookie)
		}
	})

	t.Run("Should redeem a CLI-provisioned credential only after it reaches the daemon", func(t *testing.T) {
		t.Parallel()

		credential := "cpz_gwd_" + strings.Repeat("c", 43)
		service := gatewayHTTPServiceStub{redeem: func(
			_ context.Context,
			request gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			if request.Kind != gateway.ActorKindCLIProfile || request.Credential != credential {
				t.Fatalf("RedeemPairing() request = %#v", request)
			}
			return gateway.IssuedCredential{
				Device:     gateway.DeviceSession{ID: "device-cli", Name: request.Name, ActorKind: request.Kind},
				Credential: request.Credential,
			}, nil
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, service, gatewayHTTPAuthenticatorStub{})
		body := bytes.NewBufferString(
			`{"artifact":"cpz_gwp_pairing","name":"CLI","actor_kind":"cli_profile","credential":"` +
				credential + `"}`,
		)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://127.0.0.1:2123/api/gateway/pairings/redeem",
			body,
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
		}
		var payload contract.GatewayIssuedCredentialPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Credential != credential || payload.Device.ID != "device-cli" {
			t.Fatalf("CLI pairing response = %#v", payload)
		}
		if len(response.Result().Cookies()) != 0 {
			t.Fatalf("CLI pairing response cookies = %#v, want none", response.Result().Cookies())
		}
	})

	t.Run("Should keep local browser redemption cookie-only with local origin", func(t *testing.T) {
		t.Parallel()

		credential := "cpz_gwd_" + strings.Repeat("l", 43)
		service := gatewayHTTPServiceStub{redeem: func(
			_ context.Context,
			request gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			if request.Kind != gateway.ActorKindOperatorDevice || request.Source != gateway.PairingSourceLocal {
				t.Fatalf("RedeemPairing() request = %#v", request)
			}
			return gateway.IssuedCredential{
				Device:     gateway.DeviceSession{ID: "device-local", Name: request.Name, ActorKind: request.Kind},
				Credential: credential,
			}, nil
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetLocal, service, gatewayHTTPAuthenticatorStub{})
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://127.0.0.1:2123/api/gateway/pairings/redeem",
			bytes.NewBufferString(
				`{"artifact":"cpz_gwp_pairing","name":"Local browser","actor_kind":"operator_device"}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://127.0.0.1:2123")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
		}
		if strings.Contains(response.Body.String(), credential) {
			t.Fatalf("local browser response body leaked raw credential: %s", response.Body.String())
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Value != credential || !cookies[0].Secure || !cookies[0].HttpOnly {
			t.Fatalf("local browser credential cookie = %#v", cookies)
		}
	})

	t.Run("Should reject a browser attempt to request a raw CLI credential", func(t *testing.T) {
		t.Parallel()

		redeemCalls := 0
		service := gatewayHTTPServiceStub{redeem: func(
			context.Context,
			gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			redeemCalls++
			return gateway.IssuedCredential{
				Credential: "cpz_gwd_" + strings.Repeat("x", 43),
			}, nil
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, service, gatewayHTTPAuthenticatorStub{})
		body := bytes.NewBufferString(
			`{"artifact":"cpz_gwp_pairing","name":"Browser","actor_kind":"cli_profile"}`,
		)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://127.0.0.1:2123/api/gateway/pairings/redeem",
			body,
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://127.0.0.1:2123")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Code != "gateway_invalid_request" {
			t.Fatalf("error payload = %#v, want gateway_invalid_request", payload)
		}
		if redeemCalls != 0 {
			t.Fatalf("RedeemPairing() calls = %d, want 0", redeemCalls)
		}
		if strings.Contains(response.Body.String(), "cpz_gwd_") || len(response.Result().Cookies()) != 0 {
			t.Fatalf(
				"browser rejection exposed credential material: body=%s cookies=%#v",
				response.Body.String(),
				response.Result().Cookies(),
			)
		}
	})

	t.Run("Should leave the SPA gate reachable while authenticating unknown API paths", func(t *testing.T) {
		t.Parallel()

		authCalls := 0
		authenticator := gatewayHTTPAuthenticatorStub{authenticate: func(
			context.Context,
			string,
		) (gateway.DeviceSession, error) {
			authCalls++
			return gateway.DeviceSession{}, gateway.ErrDeviceUnauthenticated
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, gatewayHTTPServiceStub{}, authenticator)
		staticResponse := httptest.NewRecorder()
		staticRequest := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "http://127.0.0.1:2123/not-a-real-page", http.NoBody,
		)
		router.ServeHTTP(staticResponse, staticRequest)
		if staticResponse.Code == http.StatusUnauthorized {
			t.Fatalf("SPA fallback status = %d, must remain reachable before pairing", staticResponse.Code)
		}

		apiResponse := httptest.NewRecorder()
		apiRequest := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "http://127.0.0.1:2123/api/not-a-real-endpoint", http.NoBody,
		)
		router.ServeHTTP(apiResponse, apiRequest)
		if apiResponse.Code != http.StatusUnauthorized {
			t.Fatalf(
				"unknown API status = %d, want %d; body = %s",
				apiResponse.Code,
				http.StatusUnauthorized,
				apiResponse.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(apiResponse.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal(unknown API) error = %v", err)
		}
		if payload.Code != "gateway_device_unauthenticated" {
			t.Fatalf("unknown API error payload = %#v", payload)
		}
		if authCalls != 1 {
			t.Fatalf("Authenticate() calls = %d, want 1", authCalls)
		}
	})

	t.Run("Should return stable status and code for invalid pairing artifacts", func(t *testing.T) {
		t.Parallel()

		service := gatewayHTTPServiceStub{redeem: func(
			context.Context,
			gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			return gateway.IssuedCredential{}, gateway.ErrPairingInvalid
		}}
		router := newGatewaySurfaceTestRouter(SurfaceSetPrivate, service, gatewayHTTPAuthenticatorStub{})
		body := bytes.NewBufferString(
			`{"artifact":"unknown","name":"Browser","actor_kind":"operator_device"}`,
		)
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://127.0.0.1:2123/api/gateway/pairings/redeem",
			body,
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Code != "gateway_pairing_invalid" {
			t.Fatalf("error payload = %#v", payload)
		}
	})
}

func TestGatewayTierAdmission(t *testing.T) {
	t.Parallel()

	t.Run("Should reject before the tier handler starts", func(t *testing.T) {
		t.Parallel()
		var acquiredTier gateway.Tier
		var acquiredSurface gateway.Surface
		handled := false
		service := gatewayHTTPServiceStub{acquire: func(
			tier gateway.Tier,
			surface gateway.Surface,
		) (func(), error) {
			acquiredTier = tier
			acquiredSurface = surface
			return nil, gateway.RefusalError{Cause: "admission closed", Fix: "retry after exposure is on"}
		}}
		handlers := newHandlers(&handlerConfig{gatewayAdmission: service})
		router := gin.New()
		router.GET(
			"/operator",
			handlers.gatewayAdmissionMiddleware(gateway.TierPrivate, gateway.SurfaceOperatorUI),
			func(c *gin.Context) {
				handled = true
				c.Status(http.StatusNoContent)
			},
		)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/operator", http.NoBody)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusConflict || handled {
			t.Fatalf("admission response = %d handled=%t body=%s", response.Code, handled, response.Body.String())
		}
		if acquiredTier != gateway.TierPrivate || acquiredSurface != gateway.SurfaceOperatorUI {
			t.Fatalf("Acquire() target = %s/%s", acquiredTier, acquiredSurface)
		}
	})

	t.Run("Should release every admitted request after the handler returns", func(t *testing.T) {
		t.Parallel()
		released := false
		service := gatewayHTTPServiceStub{acquire: func(
			gateway.Tier,
			gateway.Surface,
		) (func(), error) {
			return func() { released = true }, nil
		}}
		handlers := newHandlers(&handlerConfig{gatewayAdmission: service})
		router := gin.New()
		router.GET(
			"/ingress",
			handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceWebhookIngress),
			func(c *gin.Context) {
				if released {
					t.Fatal("admission released before handler completion")
				}
				c.Status(http.StatusNoContent)
			},
		)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ingress", http.NoBody)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent || !released {
			t.Fatalf("admitted response = %d released=%t", response.Code, released)
		}
	})
}

func TestGatewayStreamRequestClassification(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a request without a URL", func(t *testing.T) {
		t.Parallel()
		if isGatewayStreamRequest(&http.Request{Method: http.MethodGet}) {
			t.Fatal("isGatewayStreamRequest(request without URL) = true")
		}
	})

	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{
			name: "Should classify a session prompt response as a stream", method: http.MethodPost,
			path: "/api/workspaces/ws-1/sessions/session-1/prompt", want: true,
		},
		{
			name: "Should not classify prompt cancellation as a stream", method: http.MethodPost,
			path: "/api/workspaces/ws-1/sessions/session-1/prompt/cancel", want: false,
		},
		{
			name: "Should classify extension log follow aliases with the canonical parser", method: http.MethodGet,
			path: "/api/extensions/demo/logs?follow=1", want: true,
		},
		{
			name: "Should leave a non-following extension log request non-streaming", method: http.MethodGet,
			path: "/api/extensions/demo/logs?follow=false", want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(
				t.Context(), test.method, "http://127.0.0.1:2123"+test.path, http.NoBody,
			)
			if got := isGatewayStreamRequest(request); got != test.want {
				t.Fatalf("isGatewayStreamRequest(%s %s) = %t, want %t", test.method, test.path, got, test.want)
			}
		})
	}
}

func TestGatewayStreamRevocation(t *testing.T) {
	t.Parallel()

	t.Run("Should terminate and tear down an authenticated stream before cancellation returns", func(t *testing.T) {
		t.Parallel()

		registry := gateway.NewConnectionRegistry()
		device := gateway.DeviceSession{ID: "device-stream", RevokeEpoch: 3}
		authenticator := gatewayStreamAuthenticatorStub{registry: registry, device: device}
		handlers := newHandlers(&handlerConfig{
			deviceAuth: authenticator,
			boundHost:  "127.0.0.1:2123",
		})
		started := make(chan struct{})
		finished := make(chan struct{})
		router := gin.New()
		router.GET("/api/logs/stream", handlers.deviceAuthMiddleware(), func(c *gin.Context) {
			close(started)
			<-c.Request.Context().Done()
			close(finished)
		})
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://127.0.0.1:2123/api/logs/stream?ticket=cpz_gwt_once",
			http.NoBody,
		)
		served := make(chan struct{})
		go func() {
			router.ServeHTTP(response, request)
			close(served)
		}()

		<-started
		if canceled, err := registry.CancelDevice(t.Context(), device.ID); err != nil || canceled != 1 {
			t.Fatalf("CancelDevice() = (%d, %v), want (1, nil)", canceled, err)
		}
		select {
		case <-finished:
		default:
			t.Fatal("CancelDevice() returned before the stream handler finished")
		}
		<-served
		if remaining, err := registry.CancelDevice(t.Context(), device.ID); err != nil || remaining != 0 {
			t.Fatalf("CancelDevice(after teardown) = (%d, %v), want (0, nil)", remaining, err)
		}
	})
}

func newGatewaySurfaceTestRouter(
	surfaceSet SurfaceSet,
	service core.GatewayService,
	authenticator gateway.DeviceAuthenticator,
) *gin.Engine {
	handlers := newHandlers(&handlerConfig{
		gateway: service, deviceAuth: authenticator, surfaceSet: surfaceSet,
		boundHost: "127.0.0.1:2123",
	})
	router := gin.New()
	RegisterSurfaceRoutes(router, handlers, surfaceSet)
	return router
}

func gatewayRouteKeys(routes gin.RoutesInfo) map[string]struct{} {
	keys := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		keys[route.Method+" "+route.Path] = struct{}{}
	}
	return keys
}

type appTokenHTTPAuthenticatorStub struct {
	authenticate func(context.Context, string) (gateway.AppToken, error)
	origin       func(string) bool
	touch        func(context.Context, string, uint64) error
	acquire      func(context.Context, string, uint64) (context.Context, error)
	revalidate   func(context.Context, string, uint64) error
	registry     gateway.ConnectionRegistry
}

func (s appTokenHTTPAuthenticatorStub) AuthenticateAppToken(
	ctx context.Context,
	secret string,
) (gateway.AppToken, error) {
	if s.authenticate == nil {
		return gateway.AppToken{}, gateway.ErrAppTokenUnauthenticated
	}
	return s.authenticate(ctx, secret)
}

func (s appTokenHTTPAuthenticatorStub) AppTokenOriginAllowed(origin string) bool {
	return s.origin != nil && s.origin(origin)
}

func (s appTokenHTTPAuthenticatorStub) TouchAppTokenLastSeen(ctx context.Context, id string, epoch uint64) error {
	if s.touch != nil {
		return s.touch(ctx, id, epoch)
	}
	return nil
}

func (s appTokenHTTPAuthenticatorStub) AcquireMutation(
	ctx context.Context,
	id string,
	epoch uint64,
) (context.Context, error) {
	if s.acquire != nil {
		return s.acquire(ctx, id, epoch)
	}
	return ctx, nil
}

func (s appTokenHTTPAuthenticatorStub) RevalidateForCommit(ctx context.Context, id string, epoch uint64) error {
	if s.revalidate != nil {
		return s.revalidate(ctx, id, epoch)
	}
	return nil
}

func (s appTokenHTTPAuthenticatorStub) Connections() gateway.ConnectionRegistry { return s.registry }

func TestAppTokenBrowserAdmission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, method, path, origin, credential, cookie string
		allowedOrigin                                  bool
		authError                                      error
		wantStatus                                     int
		wantCode, wantOrigin                           string
		wantTouches                                    int
	}{
		{
			name:       "Should preserve tokenless no-origin reads",
			method:     http.MethodGet,
			path:       "/api/settings/general",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Should preserve tokenless own-origin mutations",
			method:     http.MethodPatch,
			path:       "/api/settings/general",
			origin:     "http://127.0.0.1:2123",
			wantStatus: http.StatusOK,
			wantOrigin: "http://127.0.0.1:2123",
		},
		{name: "Should reject invalid app credentials without an Origin", method: http.MethodGet,
			credential: "cpz_app_invalid", authError: gateway.ErrAppTokenUnauthenticated,
			wantStatus: http.StatusUnauthorized, wantCode: "app_token_unauthenticated"},
		{
			name:          "Should accept normalized foreign-origin preflight without authentication",
			method:        http.MethodOptions,
			origin:        "HTTP://A.TEST:80/",
			allowedOrigin: true,
			wantStatus:    http.StatusNoContent,
			wantOrigin:    "HTTP://A.TEST:80/",
		},
		{name: "Should reject preflight outside the active origin union", method: http.MethodOptions,
			origin: "http://a.test", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject opaque origins even when the origin union accepts them", method: http.MethodOptions,
			origin: "null", allowedOrigin: true, wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject an ungranted foreign origin without credentials", method: http.MethodGet,
			origin: "http://a.test", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{name: "Should reject opaque actual-request origins without credentials", method: http.MethodGet,
			origin: "null", wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{
			name:          "Should require a bearer from an allowed foreign origin",
			method:        http.MethodGet,
			origin:        "http://a.test",
			allowedOrigin: true,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_required",
			wantOrigin:    "http://a.test",
		},
		{name: "Should never authenticate a foreign origin with a device cookie", method: http.MethodGet,
			origin: "http://a.test", allowedOrigin: true, cookie: "cpz_gwd_device",
			wantStatus: http.StatusUnauthorized, wantCode: "app_token_required", wantOrigin: "http://a.test"},
		{name: "Should deny a token whose own allowlist lacks the origin", method: http.MethodGet,
			origin: "http://b.test", allowedOrigin: true, credential: "cpz_app_valid",
			wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed", wantOrigin: "http://b.test"},
		{name: "Should admit a foreign-origin mutation after bearer authorization", method: http.MethodPost,
			origin: "http://a.test", allowedOrigin: true, credential: "cpz_app_valid",
			wantStatus: http.StatusOK, wantOrigin: "http://a.test", wantTouches: 1},
		{
			name:        "Should authenticate and record a no-origin app bearer",
			method:      http.MethodPatch,
			path:        "/api/settings/general",
			credential:  "cpz_app_valid",
			wantStatus:  http.StatusOK,
			wantTouches: 1,
		},
		{name: "Should reject opaque actual-request origins", method: http.MethodGet,
			origin: "null", credential: "cpz_app_valid", allowedOrigin: true,
			wantStatus: http.StatusForbidden, wantCode: "app_origin_not_allowed"},
		{
			name:          "Should reject malformed foreign-origin credentials",
			method:        http.MethodGet,
			origin:        "http://a.test",
			credential:    "cpz_app_bad",
			allowedOrigin: true,
			authError:     gateway.ErrAppTokenUnauthenticated,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_unauthenticated",
			wantOrigin:    "http://a.test",
		},
		{
			name:          "Should report revoked credentials without recording use",
			method:        http.MethodGet,
			origin:        "http://a.test",
			allowedOrigin: true,
			credential:    "cpz_app_revoked",
			authError:     gateway.ErrAppTokenRevoked,
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "app_token_revoked",
			wantOrigin:    "http://a.test",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls, touches := 0, 0
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(origin string) bool {
					if origin != "http://a.test" && origin != "http://b.test" {
						t.Errorf("origin lookup = %q, want normalized origin", origin)
					}
					return test.allowedOrigin
				},
				authenticate: func(context.Context, string) (gateway.AppToken, error) {
					return gateway.AppToken{Session: gateway.DeviceSession{ID: "apt_browser", RevokeEpoch: 3},
						Origins: []string{"http://a.test"}}, test.authError
				},
				touch: func(_ context.Context, id string, epoch uint64) error {
					touches++
					if id != "apt_browser" || epoch != 3 {
						t.Errorf("touch = %s/%d, want authenticated token epoch", id, epoch)
					}
					return nil
				},
			}
			router := newAppTokenTestRouter(
				t,
				auth,
				func(_ string, path string) bool { return path == "/api/sessions" },
				nil,
			)
			path := test.path
			if path == "" {
				path = "/api/sessions"
			}
			router.Handle(test.method, path, func(c *gin.Context) {
				calls++
				c.JSON(http.StatusOK, gin.H{"admitted": true})
			})
			request := appTokenTestRequest(t, test.method, path, test.origin, test.credential)
			if test.cookie != "" {
				request.AddCookie(&http.Cookie{Name: gatewayDeviceCookieName, Value: test.cookie})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertAppTokenHTTPResponse(t, response, test.wantStatus, test.wantCode)
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Fatalf("Allow-Origin = %q, want %q", got, test.wantOrigin)
			}
			if touches != test.wantTouches {
				t.Fatalf("last_seen touches = %d, want %d", touches, test.wantTouches)
			}
			if test.wantStatus == http.StatusOK {
				if calls != 1 || !strings.Contains(response.Body.String(), `"admitted":true`) {
					t.Fatalf("admitted response = calls:%d body:%s", calls, response.Body.String())
				}
				if !strings.Contains(
					response.Header().Get("Access-Control-Expose-Headers"),
					"x-vercel-ai-ui-message-stream",
				) {
					t.Fatal("stream protocol header is not exposed")
				}
			} else if calls != 0 {
				t.Fatalf("handler calls = %d, want zero", calls)
			}
			if test.wantStatus == http.StatusNoContent {
				if response.Body.Len() != 0 || response.Header().Get("Access-Control-Max-Age") != "600" ||
					!strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Authorization") ||
					!strings.Contains(response.Header().Get("Vary"), "Origin") {
					t.Fatalf("preflight = headers:%v body:%s", response.Header(), response.Body.String())
				}
			}
		})
	}
}

func TestAppTokenRouteAuthorization(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/workspaces/:workspace_id/terminals/exec"},
		{http.MethodGet, "/api/status"},
		{http.MethodPost, "/api/workspaces/resolve"},
		{http.MethodGet, "/api/fs/browse"},
		{http.MethodPatch, "/api/settings/general"},
		{http.MethodGet, "/api/vault/secrets"},
		{http.MethodPost, "/api/app-tokens"},
		{http.MethodGet, "/api/gateway/devices"},
		{http.MethodGet, "/api/agent/me"},
		{http.MethodGet, "/api/unregistered"},
	} {
		t.Run("Should forbid browser access to "+route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			calls, touches := 0, 0
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (gateway.AppToken, error) {
					return gateway.AppToken{
						Session: gateway.DeviceSession{ID: "apt_routes"},
						Origins: []string{"http://a.test"},
					}, nil
				},
				touch: func(context.Context, string, uint64) error { touches++; return nil },
			}
			router := newAppTokenTestRouter(t, auth, func(method, path string) bool {
				return method == http.MethodGet && path == "/api/sessions"
			}, nil)
			if route.path != "/api/unregistered" {
				router.Handle(route.method, route.path, func(c *gin.Context) {
					calls++
					c.JSON(http.StatusOK, gin.H{"admitted": true})
				})
			}
			router.NoRoute(func(c *gin.Context) { calls++; c.JSON(http.StatusNotFound, gin.H{"missing": true}) })
			response := httptest.NewRecorder()
			path := strings.ReplaceAll(route.path, ":workspace_id", "ws-1")
			router.ServeHTTP(response, appTokenTestRequest(t, route.method, path, "http://a.test", "cpz_app_valid"))
			assertAppTokenHTTPResponse(t, response, http.StatusForbidden, "app_token_route_forbidden")
			if calls != 0 || touches != 0 {
				t.Fatalf("denied request side effects = handler:%d touches:%d", calls, touches)
			}
		})
	}
}

func TestAppTokenRequestLifecycle(t *testing.T) {
	t.Parallel()
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should attribute local requests and register only streams=%t", stream), func(t *testing.T) {
			t.Parallel()
			registry := gateway.NewConnectionRegistry()
			auth := appTokenHTTPAuthenticatorStub{
				registry: registry,
				origin:   func(string) bool { return true },
				authenticate: func(context.Context, string) (gateway.AppToken, error) {
					return gateway.AppToken{
						Session: gateway.DeviceSession{ID: "apt_lifecycle"},
						Origins: []string{"http://a.test"},
					}, nil
				},
			}
			router := newAppTokenTestRouter(t, auth, func(string, string) bool { return true }, nil)
			path := "/api/sessions"
			if stream {
				path = "/api/workspaces/ws-1/sessions/sess-1/stream"
			}
			entered, finished := make(chan struct{}), make(chan struct{})
			router.GET(path, func(c *gin.Context) {
				if token, ok := core.AppTokenFromContext(
					c.Request.Context(),
				); !ok ||
					token.Session.ID != "apt_lifecycle" {
					t.Error("app credential was not attributed")
				}
				if _, ok := gateway.DeviceFromContext(c.Request.Context()); ok {
					t.Error("local app attribution acquired remote device privileges")
				}
				close(entered)
				if stream {
					<-c.Request.Context().Done()
				}
				c.Status(http.StatusOK)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			go func() {
				defer close(finished)
				router.ServeHTTP(
					httptest.NewRecorder(),
					appTokenTestRequest(t, http.MethodGet, path, "http://a.test", "cpz_app_valid").WithContext(ctx),
				)
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("request did not reach its handler")
			}
			canceled, err := registry.CancelDevice(ctx, "apt_lifecycle")
			want := 0
			if stream {
				want = 1
			}
			if err != nil || canceled != want {
				t.Fatalf("CancelDevice(app) = %d, %v, want %d", canceled, err, want)
			}
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("request did not release its stream registration")
			}
		})
	}
}

func TestAppTokenAuthenticationFailureWindows(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate origins and preserve ten failures until the sixty-second window expires", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
		limiter := gateway.NewAuthFailureLimiter(10, time.Minute, func() time.Time { return now })
		lookups := 0
		auth := appTokenHTTPAuthenticatorStub{
			origin: func(string) bool { return true },
			authenticate: func(_ context.Context, secret string) (gateway.AppToken, error) {
				lookups++
				if secret != "cpz_app_valid" {
					return gateway.AppToken{}, gateway.ErrAppTokenUnauthenticated
				}
				return gateway.AppToken{Session: gateway.DeviceSession{ID: "apt_rate"},
					Origins: []string{"http://a.test", "http://b.test"}}, nil
			},
		}
		router := newAppTokenTestRouter(t, auth, func(string, string) bool { return true }, limiter)
		router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
		request := func(origin, secret string, status int, code string) {
			t.Helper()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, appTokenTestRequest(t, http.MethodGet, "/api/sessions", origin, secret))
			assertAppTokenHTTPResponse(t, response, status, code)
		}
		for range 5 {
			request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("http://a.test", "cpz_app_valid", http.StatusOK, "")
		for range 5 {
			request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("HTTP://A.TEST:80/", "cpz_app_invalid", http.StatusTooManyRequests, "app_token_rate_limited")
		if lookups != 11 {
			t.Fatalf("lookups after throttled origin = %d, want 11", lookups)
		}
		request("http://b.test", "cpz_app_valid", http.StatusOK, "")
		for range 10 {
			request("", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		}
		request("", "cpz_app_invalid", http.StatusTooManyRequests, "app_token_rate_limited")
		now = now.Add(time.Minute)
		request("http://a.test", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
		request("", "cpz_app_invalid", http.StatusUnauthorized, "app_token_unauthenticated")
	})
}

func TestAppTokenRequestLogging(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		secret string
		err    error
		reason string
	}{
		{
			name:   "Should warn when a browser omits the required app token",
			reason: "app_token_required",
		},
		{
			name:   "Should warn about invalid credentials without exposing the secret",
			secret: "cpz_app_log_invalid_secret",
			err:    gateway.ErrAppTokenUnauthenticated,
			reason: "app_token_unauthenticated",
		},
		{
			name:   "Should warn about revoked credentials without exposing the secret",
			secret: "cpz_app_log_revoked_secret",
			err:    gateway.ErrAppTokenRevoked,
			reason: "app_token_revoked",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			auth := appTokenHTTPAuthenticatorStub{
				origin: func(string) bool { return true },
				authenticate: func(context.Context, string) (gateway.AppToken, error) {
					return gateway.AppToken{}, fmt.Errorf("authentication denied: %w", tc.err)
				},
			}
			server := &Server{host: "127.0.0.1:2123", appTokenAuth: auth,
				logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			router := gin.New()
			router.Use(server.appTokenAuthMiddleware())
			router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
			request := appTokenTestRequest(t, http.MethodGet, "/api/sessions", "HTTP://APP.TEST:80/", tc.secret)
			request.RemoteAddr = "127.0.0.1:4000"
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertAppTokenHTTPResponse(t, response, http.StatusUnauthorized, tc.reason)
			var entry map[string]any
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatalf("decode authentication warning: %v", err)
			}
			for key, want := range map[string]string{
				"level": "WARN", "msg": "app_token.auth_failed", "origin": "http://app.test",
				"remote": request.RemoteAddr, "reason": tc.reason,
			} {
				if entry[key] != want {
					t.Errorf("warning %s = %v, want %q", key, entry[key], want)
				}
			}
			if strings.Contains(logs.String(), "cpz_app_") || strings.Contains(logs.String(), "authentication denied") {
				t.Fatalf("authentication warning exposed credential details: %s", logs.String())
			}
		})
	}
	t.Run("Should record the token identity without exposing its secret", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		auth := appTokenHTTPAuthenticatorStub{authenticate: func(context.Context, string) (gateway.AppToken, error) {
			return gateway.AppToken{Session: gateway.DeviceSession{ID: "apt_logged"}}, nil
		}}
		router := gin.New()
		router.Use(requestLoggingMiddleware(logger))
		server := &Server{host: "127.0.0.1:2123", appTokenAuth: auth}
		WithAppTokenAuthLimiter(gateway.NewAuthFailureLimiter(10, time.Minute, nil))(server)
		router.Use(server.appTokenAuthMiddleware())
		router.GET("/api/sessions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"admitted": true}) })
		response := httptest.NewRecorder()
		router.ServeHTTP(response, appTokenTestRequest(t, http.MethodGet, "/api/sessions", "", "cpz_app_log_secret"))
		assertAppTokenHTTPResponse(t, response, http.StatusOK, "")
		if !strings.Contains(logs.String(), `"app_token_id":"apt_logged"`) ||
			strings.Contains(logs.String(), "cpz_app_") {
			t.Fatalf("request log did not protect token identity: %s", logs.String())
		}
	})
}

func TestGatewayRejectsAppTokensBeforeAuthentication(t *testing.T) {
	t.Parallel()
	for _, surface := range []SurfaceSet{SurfaceSetPrivate, SurfaceSetPublicOperator, SurfaceSetPublicIngress} {
		t.Run("Should reject app-token prefixes on the "+string(surface)+" tier before lookup", func(t *testing.T) {
			t.Parallel()
			lookups := 0
			auth := gatewayHTTPAuthenticatorStub{
				authenticate: func(context.Context, string) (gateway.DeviceSession, error) {
					lookups++
					return gateway.DeviceSession{}, gateway.ErrDeviceUnauthenticated
				},
			}
			router := gin.New()
			router.Use(rejectTierAppTokens())
			handlers := newHandlers(&handlerConfig{gateway: gatewayHTTPServiceStub{}, deviceAuth: auth,
				surfaceSet: surface, boundHost: "127.0.0.1:2123"})
			RegisterSurfaceRoutes(router, handlers, surface)
			for _, path := range []string{"/api/status", "/api/unregistered"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(
					response,
					appTokenTestRequest(t, http.MethodGet, path, "http://foreign.test", "cpz_app_any"),
				)
				assertAppTokenHTTPResponse(t, response, http.StatusForbidden, "gateway_app_token_local_only")
			}
			if lookups != 0 {
				t.Fatalf("device lookups = %d, want prefix rejection before lookup", lookups)
			}
		})
	}
}

func newAppTokenTestRouter(t *testing.T, auth gateway.AppTokenAuthenticator,
	allowed func(string, string) bool, limiter *gateway.AuthFailureLimiter,
) *gin.Engine {
	t.Helper()
	if limiter == nil {
		limiter = gateway.NewAuthFailureLimiter(10, time.Minute, nil)
	}
	router := gin.New()
	server := &Server{host: "127.0.0.1:2123", appTokenAuth: auth, appAccess: allowed}
	WithAppTokenAuthLimiter(limiter)(server)
	router.Use(server.appTokenAuthMiddleware())
	router.Use(corsMiddleware("127.0.0.1:2123"))
	router.Use(browserRequestProtectionMiddleware("127.0.0.1:2123"))
	return router
}

func appTokenTestRequest(t *testing.T, method, path, origin, secret string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "http://127.0.0.1:2123"+path, http.NoBody)
	if origin != "" {
		request.Header.Set("Origin", origin)
		if origin != "http://127.0.0.1:2123" {
			request.Header.Set("Sec-Fetch-Site", "cross-site")
		}
	}
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	return request
}

func assertAppTokenHTTPResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response = %d/%s, want status %d", response.Code, response.Body.String(), status)
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("credentials header unexpectedly present: %v", response.Header())
	}
	if code != "" {
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
		}
		if payload.Code != code || payload.Error == "" {
			t.Fatalf("error payload = %#v, want code %s and message", payload, code)
		}
	}
}
