package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func TestBrowserMiddlewareWorkspaceRoutingHelpers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want bool
	}{
		{name: "dashboard", path: "/api/ui/dashboard", want: true},
		{name: "workflow list", path: "/api/tasks", want: true},
		{name: "workflow detail", path: "/api/tasks/example", want: true},
		{name: "workflow validate excluded", path: "/api/tasks/example/validate", want: false},
		{name: "sync", path: "/api/sync", want: true},
		{name: "review issues", path: "/api/reviews/example/rounds/1/issues", want: true},
		{name: "review fetch excluded", path: "/api/reviews/example/fetch", want: false},
		{name: "run detail", path: "/api/runs/run-1", want: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := requiresActiveWorkspace(tc.path); got != tc.want {
				t.Fatalf("requiresActiveWorkspace(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestBrowserMiddlewareRequestDetectionAndCSRFCookies(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("Should detect browser requests from Origin and fetch headers", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			headers map[string]string
			want    bool
		}{
			{name: "no browser headers", headers: nil, want: false},
			{name: "origin header", headers: map[string]string{"Origin": "http://127.0.0.1:43123"}, want: true},
			{name: "sec fetch header", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}, want: true},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				req := httptest.NewRequestWithContext(
					context.Background(),
					http.MethodGet,
					"/api/daemon/status",
					http.NoBody,
				)
				for key, value := range tc.headers {
					req.Header.Set(key, value)
				}
				ctx.Request = req

				if got := looksLikeBrowserRequest(ctx); got != tc.want {
					t.Fatalf("looksLikeBrowserRequest() = %v, want %v", got, tc.want)
				}
			})
		}
	})

	t.Run("Should issue and reuse csrf cookies", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/daemon/status",
			http.NoBody,
		)

		token, err := ensureCSRFCookie(ctx)
		if err != nil {
			t.Fatalf("ensureCSRFCookie() error = %v", err)
		}
		if len(token) != 64 {
			t.Fatalf("len(token) = %d, want 64", len(token))
		}
		if _, err := io.ReadAll(strings.NewReader(token)); err != nil {
			t.Fatalf("token readback error = %v", err)
		}
		cookies := recorder.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("len(cookies) = %d, want 1", len(cookies))
		}
		if got := cookies[0].Value; got != token {
			t.Fatalf("issued cookie value = %q, want %q", got, token)
		}

		reuseRecorder := httptest.NewRecorder()
		reuseCtx, _ := gin.CreateTestContext(reuseRecorder)
		reuseReq := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/daemon/status",
			http.NoBody,
		)
		reuseReq.AddCookie(&http.Cookie{Name: core.CookieCSRF, Value: token})
		reuseCtx.Request = reuseReq

		reused, err := ensureCSRFCookie(reuseCtx)
		if err != nil {
			t.Fatalf("ensureCSRFCookie(reuse) error = %v", err)
		}
		if reused != token {
			t.Fatalf("reused token = %q, want %q", reused, token)
		}
	})
}

func TestBrowserMiddlewareHostAndOriginValidationHelpers(t *testing.T) {
	t.Parallel()

	server := &Server{host: "127.0.0.1", port: 43123}

	validHosts := []string{"127.0.0.1:43123", "localhost:43123"}
	for _, authority := range validHosts {
		if !server.hostAllowed(authority) {
			t.Fatalf("hostAllowed(%q) = false, want true", authority)
		}
	}
	for _, authority := range []string{"127.0.0.1", "127.0.0.1:9999", "evil.example:43123", "bad::host"} {
		if server.hostAllowed(authority) {
			t.Fatalf("hostAllowed(%q) = true, want false", authority)
		}
	}

	validOrigins := []string{"http://127.0.0.1:43123", "http://localhost:43123"}
	for _, origin := range validOrigins {
		if !server.originAllowed(origin) {
			t.Fatalf("originAllowed(%q) = false, want true", origin)
		}
	}
	for _, origin := range []string{"https://127.0.0.1:43123", "http://evil.example:43123", "://bad"} {
		if server.originAllowed(origin) {
			t.Fatalf("originAllowed(%q) = true, want false", origin)
		}
	}

	allowed := server.allowedHostnames()
	if !allowed["127.0.0.1"] || !allowed["localhost"] {
		t.Fatalf("allowedHostnames() = %v, want loopback hostnames", allowed)
	}
}

func TestServerOptionsAndAuthorityParsing(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := New(
		WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "test"})),
		WithHost(defaultHost),
		WithPort(43210),
		WithLogger(logger),
		WithEngine(engine),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if server.host != defaultHost {
		t.Fatalf("server.host = %q, want %s", server.host, defaultHost)
	}
	if server.logger != logger {
		t.Fatal("server.logger mismatch")
	}
	if server.engine != engine {
		t.Fatal("server.engine mismatch")
	}
	if got := server.Port(); got != 43210 {
		t.Fatalf("server.Port() = %d, want 43210", got)
	}

	host, port := splitAuthority("[::1]:43210")
	if host != "::1" || port != "43210" {
		t.Fatalf("splitAuthority([::1]:43210) = (%q, %q), want (::1, 43210)", host, port)
	}
	host, port = splitAuthority("LOCALHOST")
	if host != "localhost" || port != "" {
		t.Fatalf("splitAuthority(LOCALHOST) = (%q, %q), want (localhost, \"\")", host, port)
	}
	host, port = splitAuthority("bad::host")
	if host != "" || port != "" {
		t.Fatalf("splitAuthority(bad::host) = (%q, %q), want empty values", host, port)
	}
	if got := normalizeAuthorityHost("[LOCALHOST.]"); got != "localhost" {
		t.Fatalf("normalizeAuthorityHost() = %q, want localhost", got)
	}
	if defaultHost != "127.0.0.1" {
		t.Fatalf("defaultHost = %q, want 127.0.0.1", defaultHost)
	}
}
