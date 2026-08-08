package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
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

func TestCORSMiddlewareRejectsReboundRequestHost(t *testing.T) {
	t.Run("Should reject matching attacker controlled Host and Origin on a loopback bind", func(
		t *testing.T,
	) {
		t.Parallel()

		engine := gin.New()
		engine.Use(corsMiddleware("127.0.0.1:2123"))
		engine.GET("/api/status", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"http://evil.example:2123/api/status",
			http.NoBody,
		)
		request.Host = "evil.example:2123"
		request.Header.Set("Origin", "http://evil.example:2123")
		engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusForbidden,
				recorder.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		decodeJSONResponse(t, recorder, &payload)
		if payload.Error != errOriginNotAllowed.Error() {
			t.Fatalf("error = %q, want %q", payload.Error, errOriginNotAllowed.Error())
		}
	})
}

func TestGatewayForwardedOriginProtection(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a same-origin gateway request forwarded to a loopback tier", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(corsMiddlewareWithForwardedTarget("127.0.0.1:43123", true))
		engine.Use(browserRequestProtectionMiddlewareWithForwardedTarget("127.0.0.1:43123", true))
		engine.POST("/api/action", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		request := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "https://gateway.example.test/api/action", http.NoBody,
		)
		request.Host = "gateway.example.test"
		request.Header.Set("Origin", "https://gateway.example.test")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
		}
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://gateway.example.test" {
			t.Fatalf("Access-Control-Allow-Origin = %q", got)
		}
	})

	t.Run("Should reject a cross-origin request even when its target was forwarded", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(corsMiddlewareWithForwardedTarget("127.0.0.1:43123", true))
		engine.Use(browserRequestProtectionMiddlewareWithForwardedTarget("127.0.0.1:43123", true))
		engine.POST("/api/action", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		request := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "https://gateway.example.test/api/action", http.NoBody,
		)
		request.Host = "gateway.example.test"
		request.Header.Set("Origin", "https://evil.example.test")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
	})
}

func TestBrowserRequestProtectionRouteBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should reject cross-site unsafe requests on ordinary API routes", func(t *testing.T) {
		t.Parallel()

		engine := newTestRouter(
			t,
			newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)),
		)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://127.0.0.1/api/workspaces/resolve",
			strings.NewReader(`{}`),
		)
		request.Host = "127.0.0.1"
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusForbidden,
				recorder.Body.String(),
			)
		}
	})

	t.Run("Should reject same-site browser metadata for an unapproved request Host", func(
		t *testing.T,
	) {
		t.Parallel()

		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t))
		engine := gin.New()
		RegisterRoutes(engine, handlers)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://evil.example/api/workspaces/resolve",
			strings.NewReader(`{}`),
		)
		request.Host = "evil.example"
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusForbidden,
				recorder.Body.String(),
			)
		}
	})

	t.Run("Should allow same-origin browser metadata for a loopback request Host", func(t *testing.T) {
		t.Parallel()

		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t))
		engine := gin.New()
		RegisterRoutes(engine, handlers)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://127.0.0.1/api/workspaces/resolve",
			strings.NewReader(`{}`),
		)
		request.Host = "127.0.0.1"
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		engine.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusForbidden {
			t.Fatalf(
				"same-origin request status = %d, want route handler response; body=%s",
				recorder.Code,
				recorder.Body.String(),
			)
		}
	})

	t.Run("Should leave public webhooks outside browser CSRF middleware", func(t *testing.T) {
		t.Parallel()

		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t))
		engine := gin.New()
		RegisterRoutes(engine, handlers)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://127.0.0.1/api/webhooks/global/missing",
			http.NoBody,
		)
		request.Host = "127.0.0.1"
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		engine.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusForbidden {
			t.Fatalf(
				"public webhook status = %d, want route handler response; body=%s",
				recorder.Code,
				recorder.Body.String(),
			)
		}
	})

	t.Run("Should preserve OpenAI compatible cross-origin errors", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(browserRequestProtectionMiddleware("127.0.0.1"))
		engine.POST("/api/openai/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://127.0.0.1/api/openai/v1/responses",
			http.NoBody,
		)
		request.Host = "127.0.0.1"
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusForbidden,
				recorder.Body.String(),
			)
		}
		var payload contract.OpenAIErrorResponse
		decodeJSONResponse(t, recorder, &payload)
		if payload.Error.Code != "forbidden" ||
			payload.Error.Message != errCrossOriginNotAllowed.Error() {
			t.Fatalf("OpenAI error = %#v, want cross-origin forbidden", payload.Error)
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
