package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/gin-gonic/gin"
)

func TestIngressRateLimit(t *testing.T) {
	t.Parallel()
	t.Run("Should rate limit public ingress by source and endpoint", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := ingress.NewRateLimiter(1, time.Minute, func() time.Time { return now })
		handlers := newHandlers(&handlerConfig{
			ingressLimiter: limiter,
			boundHost:      "127.0.0.1:2123",
		})
		router := gin.New()
		RegisterRoutes(router, handlers)

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
		if payload.Code != "ingress_rate_limited" {
			t.Fatalf("rate-limit payload = %#v, want ingress_rate_limited", payload)
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
