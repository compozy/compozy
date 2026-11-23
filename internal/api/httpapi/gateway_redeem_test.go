package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/gin-gonic/gin"
)

// Invariant: hand-typed pairing codes have their own remote failure budget, which a
// device-authenticated success cannot reset, while the local operator surface stays exempt.
// Owning layer: HTTP pairing redeem handler and gateway pairing limiter.
// Canonical suite: this pairing redeem transport suite.
func TestGatewayRedeemRateLimiting(t *testing.T) {
	t.Parallel()

	t.Run("Should rate limit repeated wrong codes on a remote surface", func(t *testing.T) {
		t.Parallel()
		router, submissions := newGatewayRedeemTestRouter(t, SurfaceSetPrivate, 2)
		for attempt := range 2 {
			response := postGatewayRedeem(
				t,
				router,
				`{"manual_code":"000000","name":"Laptop","actor_kind":"operator_device"}`,
			)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
			}
		}
		response := postGatewayRedeem(
			t,
			router,
			`{"manual_code":"000000","name":"Laptop","actor_kind":"operator_device"}`,
		)
		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("rate-limited status = %d, want %d", response.Code, http.StatusTooManyRequests)
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Code != "gateway_auth_rate_limited" {
			t.Fatalf("rate-limited code = %q", payload.Code)
		}
		if submissions.Load() != 2 {
			t.Fatalf("redeem submissions = %d, want 2 before the limiter refused", submissions.Load())
		}
	})

	t.Run("Should keep pairing failures when device authentication succeeds for the same source", func(t *testing.T) {
		t.Parallel()
		clock := func() time.Time {
			return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
		}
		handlers := newHandlers(&handlerConfig{
			gateway: gatewayHTTPServiceStub{redeem: func(
				context.Context,
				gateway.RedeemRequest,
			) (gateway.IssuedCredential, error) {
				return gateway.IssuedCredential{}, gateway.ErrPairingInvalid
			}},
			deviceAuth:       gatewayHTTPAuthenticatorStub{},
			surfaceSet:       SurfaceSetPrivate,
			gatewayAdmission: gatewayHTTPServiceStub{},
			boundHost:        "127.0.0.1",
			authLimiter:      remotehost.NewAuthFailureLimiter(1, time.Minute, clock),
			pairingLimiter:   remotehost.NewAuthFailureLimiter(1, time.Minute, clock),
		})
		router := gin.New()
		RegisterSurfaceRoutes(router, handlers, SurfaceSetPrivate)
		probe := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/gateway/pairings/redeem",
			strings.NewReader("{}"),
		)
		source := gatewayAuthSource(probe)

		// One wrong code exhausts the pairing budget.
		first := postGatewayRedeem(
			t,
			router,
			`{"manual_code":"000000","name":"Laptop","actor_kind":"operator_device"}`,
		)
		if first.Code != http.StatusUnauthorized {
			t.Fatalf("first redeem status = %d, want %d", first.Code, http.StatusUnauthorized)
		}

		// The device-authentication middleware clears its own budget on success;
		// that reset must not reach the pairing budget.
		handlers.authLimiter.Success(source)

		second := postGatewayRedeem(
			t,
			router,
			`{"manual_code":"000000","name":"Laptop","actor_kind":"operator_device"}`,
		)
		if second.Code != http.StatusTooManyRequests {
			t.Fatalf(
				"status after device-auth success = %d, want %d",
				second.Code,
				http.StatusTooManyRequests,
			)
		}
	})

	t.Run("Should exempt the local operator surface from the failure budget", func(t *testing.T) {
		t.Parallel()
		router, _ := newGatewayRedeemTestRouter(t, SurfaceSetLocal, 1)
		for attempt := range 4 {
			response := postGatewayRedeem(
				t,
				router,
				`{"manual_code":"000000","name":"Laptop","actor_kind":"operator_device"}`,
			)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("local attempt %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
			}
		}
	})

	t.Run("Should redeem a manual code and keep the artifact error shape", func(t *testing.T) {
		t.Parallel()
		handlers := newHandlers(&handlerConfig{
			gateway: gatewayHTTPServiceStub{redeem: func(
				_ context.Context,
				request gateway.RedeemRequest,
			) (gateway.IssuedCredential, error) {
				if request.ManualCode != "123456" || request.Artifact != "" {
					return gateway.IssuedCredential{}, gateway.ErrPairingInvalid
				}
				return gateway.IssuedCredential{
					Device:     gateway.DeviceSession{ID: "device-1", Name: "Laptop"},
					Credential: "cpz_gwd_credential",
				}, nil
			}},
			deviceAuth: gatewayHTTPAuthenticatorStub{}, surfaceSet: SurfaceSetPrivate,
			gatewayAdmission: gatewayHTTPServiceStub{},
			boundHost:        "127.0.0.1",
		})
		router := gin.New()
		RegisterSurfaceRoutes(router, handlers, SurfaceSetPrivate)
		response := postGatewayRedeem(
			t,
			router,
			`{"manual_code":"123456","name":"Laptop","actor_kind":"operator_device"}`,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("redeem status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
		}
	})

	t.Run("Should reject a redeem request with neither representation", func(t *testing.T) {
		t.Parallel()
		router, submissions := newGatewayRedeemTestRouter(t, SurfaceSetPrivate, 5)
		response := postGatewayRedeem(t, router, `{"name":"Laptop","actor_kind":"operator_device"}`)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
		if submissions.Load() != 0 {
			t.Fatalf("redeem submissions = %d, want the request refused before the service", submissions.Load())
		}
	})
}

func newGatewayRedeemTestRouter(
	t *testing.T,
	surfaceSet SurfaceSet,
	failureLimit int,
) (*gin.Engine, *atomic.Int32) {
	t.Helper()
	var submissions atomic.Int32
	handlers := newHandlers(&handlerConfig{
		gateway: gatewayHTTPServiceStub{redeem: func(
			context.Context,
			gateway.RedeemRequest,
		) (gateway.IssuedCredential, error) {
			submissions.Add(1)
			return gateway.IssuedCredential{}, gateway.ErrPairingInvalid
		}},
		deviceAuth:       gatewayHTTPAuthenticatorStub{},
		surfaceSet:       surfaceSet,
		gatewayAdmission: gatewayHTTPServiceStub{},
		boundHost:        "127.0.0.1",
		authLimiter: remotehost.NewAuthFailureLimiter(
			failureLimit,
			time.Minute,
			func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) },
		),
		pairingLimiter: remotehost.NewAuthFailureLimiter(
			failureLimit,
			time.Minute,
			func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) },
		),
	})
	router := gin.New()
	RegisterSurfaceRoutes(router, handlers, surfaceSet)
	return router, &submissions
}

func postGatewayRedeem(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/gateway/pairings/redeem",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}
