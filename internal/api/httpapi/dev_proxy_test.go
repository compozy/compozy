package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func TestParseDevProxyTargetRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		target        string
		wantErrSubstr string
	}{
		{
			name:          "empty",
			target:        " ",
			wantErrSubstr: "target is required",
		},
		{
			name:          "missing scheme",
			target:        "127.0.0.1:3000",
			wantErrSubstr: "must use http or https",
		},
		{
			name:          "unsupported scheme",
			target:        "ws://127.0.0.1:3000",
			wantErrSubstr: "must use http or https",
		},
		{
			name:          "missing host",
			target:        "http://",
			wantErrSubstr: "must include a host",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseDevProxyTarget(tt.target)
			if err == nil {
				t.Fatalf("parseDevProxyTarget(%q) error = nil, want %q", tt.target, tt.wantErrSubstr)
			}
			if got := err.Error(); !contains(got, tt.wantErrSubstr) {
				t.Fatalf("parseDevProxyTarget(%q) error = %q, want substring %q", tt.target, got, tt.wantErrSubstr)
			}
		})
	}
}

func TestDevProxyRoutesServeFrontendRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	var upstreamHosts []string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamHosts = append(upstreamHosts, request.Host)
		switch request.URL.Path {
		case "/":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = writer.Write([]byte("<!doctype html><div id=\"app\">dev</div>"))
		case "/workflows/daemon":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = writer.Write([]byte("<!doctype html><div id=\"app\">workflow</div>"))
		case "/assets/app.js":
			writer.Header().Set("Content-Type", "application/javascript")
			_, _ = writer.Write([]byte("console.log('vite');"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(upstream.Close)

	engine := newDevProxyTestEngine(t, upstream.URL)

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "root",
			method:     http.MethodGet,
			target:     "/",
			wantStatus: http.StatusOK,
			wantBody:   `<div id="app">dev</div>`,
		},
		{
			name:       "deep link",
			method:     http.MethodGet,
			target:     "/workflows/daemon",
			wantStatus: http.StatusOK,
			wantBody:   `<div id="app">workflow</div>`,
		},
		{
			name:       "asset",
			method:     http.MethodGet,
			target:     "/assets/app.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log('vite');",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			response := performStaticRequest(t, engine, tt.method, tt.target)
			if response.Code != tt.wantStatus {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tt.method,
					tt.target,
					response.Code,
					tt.wantStatus,
					response.Body.String(),
				)
			}
			if got := response.Body.String(); !contains(got, tt.wantBody) {
				t.Fatalf("%s %s body = %q, want substring %q", tt.method, tt.target, got, tt.wantBody)
			}
		})
	}

	for _, host := range upstreamHosts {
		if host != "example.com" {
			t.Fatalf("upstream host = %q, want preserved inbound host example.com", host)
		}
	}
}

func TestDevProxyRoutesBypassAPIAndUnsupportedMethods(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalled = true
		http.NotFound(writer, request)
	}))
	t.Cleanup(upstream.Close)

	engine := newDevProxyTestEngine(t, upstream.URL)

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{
			name:       "missing api route",
			method:     http.MethodGet,
			target:     "/api/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unsupported post",
			method:     http.MethodPost,
			target:     "/workflows/daemon",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			response := performStaticRequest(t, engine, tt.method, tt.target)
			if response.Code != tt.wantStatus {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tt.method,
					tt.target,
					response.Code,
					tt.wantStatus,
					response.Body.String(),
				)
			}
		})
	}

	if upstreamCalled {
		t.Fatal("expected bypassed requests to avoid the dev proxy upstream")
	}
}

func TestDevProxyReturnsBadGatewayWhenUpstreamIsUnavailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := newDevProxyTestEngine(t, "http://127.0.0.1:1")
	response := performStaticRequest(t, engine, http.MethodGet, "/")
	if response.Code != http.StatusBadGateway {
		t.Fatalf("GET / status = %d, want %d; body=%s", response.Code, http.StatusBadGateway, response.Body.String())
	}
	if got := response.Body.String(); !contains(got, "frontend dev proxy unavailable") {
		t.Fatalf("GET / body = %q, want dev proxy failure message", got)
	}
}

func TestNewWithDevProxyTargetPrefersProxyOverEmbeddedStaticFS(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	server, err := New(
		WithHandlers(core.NewHandlers(&core.HandlerConfig{TransportName: "test"})),
		WithDevProxyTarget("http://127.0.0.1:3000"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if server.devProxy == nil {
		t.Fatal("expected dev proxy to be configured")
	}
	if server.staticFS != nil {
		t.Fatal("expected embedded static filesystem to be skipped in dev proxy mode")
	}
}

func newDevProxyTestEngine(t *testing.T, target string) *gin.Engine {
	t.Helper()

	devProxy, err := newDevProxyHandler(target)
	if err != nil {
		t.Fatalf("newDevProxyHandler(%q) error = %v", target, err)
	}

	engine := gin.New()
	RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{TransportName: "test"}), devProxy.serve)
	return engine
}

func contains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}
