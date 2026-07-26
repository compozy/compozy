package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func TestCanonicalHostNormalizesBoundHostPorts(t *testing.T) {
	t.Run("Should normalize TCP host port forms", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			raw      string
			want     string
			loopback bool
			wildcard bool
		}{
			{name: "Should normalize loopback IPv4", raw: "127.0.0.1:2123", want: "127.0.0.1", loopback: true},
			{name: "Should normalize loopback name", raw: "localhost:2123", want: "localhost", loopback: true},
			{name: "Should normalize loopback IPv6", raw: "[::1]:2123", want: "::1", loopback: true},
			{name: "Should normalize loopback URL", raw: "http://127.0.0.1:2123", want: "127.0.0.1", loopback: true},
			{name: "Should normalize wildcard IPv4", raw: "0.0.0.0:2123", want: "0.0.0.0", wildcard: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := canonicalHost(tt.raw)
				if got != tt.want {
					t.Fatalf("canonicalHost(%q) = %q, want %q", tt.raw, got, tt.want)
				}
				if isLoopbackHost(got) != tt.loopback {
					t.Fatalf("isLoopbackHost(%q) = %v, want %v", got, isLoopbackHost(got), tt.loopback)
				}
				if isWildcardHost(got) != tt.wildcard {
					t.Fatalf("isWildcardHost(%q) = %v, want %v", got, isWildcardHost(got), tt.wildcard)
				}
			})
		}
	})
}

func TestCORSMiddlewareAllowsPatchPreflight(t *testing.T) {
	t.Run("Should advertise PATCH for API preflight", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(corsMiddleware("127.0.0.1:2123"))
		engine.OPTIONS("/api/settings/general", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodOptions,
			"/api/settings/general",
			http.NoBody,
		)
		request.Host = "127.0.0.1:2123"
		request.Header.Set("Origin", "http://127.0.0.1:2123")
		request.Header.Set("Access-Control-Request-Method", http.MethodPatch)

		engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
		}
		if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPatch) {
			t.Fatalf("Access-Control-Allow-Methods = %q, want it to include %q", got, http.MethodPatch)
		}
		if got, want := recorder.Header().Get("Access-Control-Allow-Origin"), "http://127.0.0.1:2123"; got != want {
			t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, want)
		}
		if got := recorder.Body.String(); got != "" {
			t.Fatalf("body = %q, want empty body", got)
		}
	})
}

func TestLoopbackGuardsHandleBoundHostPorts(t *testing.T) {
	tests := []struct {
		name       string
		boundHost  string
		wantStatus int
		wantError  string
	}{
		{
			name:       "Should allow loopback host with port",
			boundHost:  "127.0.0.1:2123",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "Should block wildcard host with port",
			boundHost:  "0.0.0.0:2123",
			wantStatus: http.StatusForbidden,
			wantError:  errLoopbackAPIRequired.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := gin.New()
			engine.GET("/guarded", loopbackAPIGuard(tt.boundHost), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/guarded", http.NoBody)
			engine.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantError == "" {
				if got := recorder.Body.String(); got != "" {
					t.Fatalf("body = %q, want empty body", got)
				}
				return
			}

			var payload contract.ErrorPayload
			decodeJSONResponse(t, recorder, &payload)
			if payload.Error != tt.wantError {
				t.Fatalf("payload.Error = %q, want %q", payload.Error, tt.wantError)
			}
		})
	}
}
