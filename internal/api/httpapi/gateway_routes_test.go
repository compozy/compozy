package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
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

func TestGatewayTierRouteMatrices(t *testing.T) {
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
				"GET /api/gateway/status",
				"POST /api/gateway/pairings",
				"GET /api/status",
			},
			absent: []string{"POST /api/webhooks/global/:endpoint"},
		},
		{
			name:       "Should expose only self-verifying webhooks on the public ingress tier",
			surfaceSet: SurfaceSetPublicIngress,
			present: []string{
				"POST /api/webhooks/global/:endpoint",
				"POST /api/webhooks/workspaces/:workspace_id/:endpoint",
				"GET /api/bridge-callbacks/:id",
				"POST /api/bridge-callbacks/:id",
				"HEAD /api/bridge-callbacks/:id",
			},
			absent: []string{
				"GET /api/status",
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"POST /api/gateway/stream-tickets",
			},
			wantRoutes: 5,
		},
		{
			name:       "Should expose authenticated operator routes without ingress on the public operator tier",
			surfaceSet: SurfaceSetPublicOperator,
			present:    []string{"GET /api/status", "POST /api/gateway/stream-tickets"},
			absent: []string{
				"POST /api/webhooks/global/:endpoint",
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"GET /api/gateway/status",
			},
		},
		{
			name:       "Should compose ingress ahead of authenticated operator routes on the combined tier",
			surfaceSet: SurfaceSetPublicCombined,
			present: []string{
				"POST /api/webhooks/global/:endpoint",
				"GET /api/status",
				"POST /api/gateway/stream-tickets",
			},
			absent: []string{
				"POST /api/gateway/pairings",
				"POST /api/gateway/pairings/redeem",
				"GET /api/gateway/status",
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
				path := strings.SplitN(route, " ", 2)[1]
				if path == "/api/agent" || strings.HasPrefix(path, "/api/agent/") ||
					path == "/api/resources" || strings.HasPrefix(path, "/api/resources/") ||
					path == "/api/internal" || strings.HasPrefix(path, "/api/internal/") {
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

func TestGatewayIngressRateLimit(t *testing.T) {
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
		t.Fatalf("second delivery status = %d, want %d; body=%s", limited.Code, http.StatusTooManyRequests, limited.Body.String())
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
	sort.Strings(actual)

	want := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if slices.Contains(operation.Transports, apispec.TransportHTTP) &&
			strings.HasPrefix(operation.Path, "/api/gateway") {
			want = append(want, operation.Method+" "+operation.Path)
		}
	}
	sort.Strings(want)
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
