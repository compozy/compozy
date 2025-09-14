package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	m := config.NewManager(config.NewService())
	if _, err := m.Load(t.Context(), config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		t.Fatalf("load config: %v", err)
	}
	// Defaults are fine; ensure FromContext works
	ctx := config.ContextWithManager(t.Context(), m)
	srv, err := NewServer(ctx, ".", "", "")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func TestLivenessEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)
	r := gin.New()
	setupDiagnosticEndpoints(r, "v0", "/api/v0", srv)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadinessEndpoint_Matrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name        string
		recReady    bool
		tempReady   bool
		workerReady bool
		expect      int
	}{
		{"all_not_ready", false, false, false, http.StatusServiceUnavailable},
		{"schedules_not_ready", false, true, true, http.StatusServiceUnavailable},
		{"temporal_not_ready", true, false, true, http.StatusServiceUnavailable},
		{"worker_not_ready", true, true, false, http.StatusServiceUnavailable},
		{"all_ready", true, true, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			if tc.recReady {
				srv.reconciliationState.setCompleted()
			}
			if tc.tempReady {
				srv.setTemporalReady(true)
			}
			if tc.workerReady {
				srv.setWorkerReady(true)
			}
			r := gin.New()
			setupDiagnosticEndpoints(r, "v0", "/api/v0", srv)
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.expect, w.Code)
		})
	}
}

func TestReadinessEndpoint_BecomesReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)
	srv.reconciliationState.setCompleted()
	srv.setTemporalReady(true)
	srv.setWorkerReady(true)
	r := gin.New()
	setupDiagnosticEndpoints(r, "v0", "/api/v0", srv)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLegacyHealthEndpoint_FollowsAggregateReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)
	r := gin.New()
	setupDiagnosticEndpoints(r, "v0", "/api/v0", srv)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Make fully ready
	srv.reconciliationState.setCompleted()
	srv.setTemporalReady(true)
	srv.setWorkerReady(true)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}
