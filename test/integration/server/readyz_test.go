package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	engserver "github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIntegrationServer creates a new server instance with default config manager
func newIntegrationServer(t *testing.T) *engserver.Server {
	t.Helper()
	m := config.NewManager(config.NewService())
	if _, err := m.Load(t.Context(), config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		t.Fatalf("load config: %v", err)
	}
	ctx := config.ContextWithManager(t.Context(), m)
	srv, err := engserver.NewServer(ctx, ".", "", "")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func TestServer_Health_And_Liveness_Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newIntegrationServer(t)
	r := gin.New()
	r.GET("/health", engserver.CreateHealthHandler(srv, "v0"))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	t.Run("Should report liveness OK", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("Should return 503 when server is not fully ready", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}
