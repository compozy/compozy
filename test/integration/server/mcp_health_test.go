package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	engserver "github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMCPHealth_EndpointExposed(t *testing.T) {
	t.Setenv("MCP_PROXY_MODE", "standalone")
	gin.SetMode(gin.TestMode)
	m := config.NewManager(config.NewService())
	if _, err := m.Load(t.Context(), config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		t.Fatalf("load config: %v", err)
	}
	ctx := config.ContextWithManager(t.Context(), m)
	srv, err := engserver.NewServer(ctx, ".", "", "")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	r := gin.New()
	// We validate MCP affects overall health by using the exported health handler
	r.GET("/health", engserver.CreateHealthHandler(srv, "v0"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w, req)
	// On a fresh standalone server (without MCP ready), health should be not ready
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
