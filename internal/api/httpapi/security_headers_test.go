package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersMiddlewareSetsTightScriptCSP(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(securityHeadersMiddleware())
	engine.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	csp := recorder.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self';") {
		t.Fatalf("Content-Security-Policy = %q, want script-src 'self';", csp)
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline';") {
		t.Fatalf("Content-Security-Policy = %q, want no unsafe-inline script directive", csp)
	}
}
