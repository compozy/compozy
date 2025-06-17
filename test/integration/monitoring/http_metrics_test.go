package monitoring_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetricsIntegration(t *testing.T) {
	t.Run("Should record metrics for successful requests", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Make various HTTP requests
		requests := []struct {
			method string
			path   string
			status int
		}{
			{"GET", "/api/v1/health", http.StatusOK},
			{"GET", "/api/v1/users/123", http.StatusOK},
			{"GET", "/api/v1/users/456", http.StatusOK},
			{"GET", "/", http.StatusOK},
		}
		for _, req := range requests {
			resp, err := env.MakeRequest(req.method, req.path)
			require.NoError(t, err)
			require.Equal(t, req.status, resp.StatusCode)
			resp.Body.Close()
		}
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Verify HTTP request counter
		assert.Contains(t, metrics, "compozy_http_requests_total")
		// OpenTelemetry adds extra labels, so check for the important parts
		assert.Contains(t, metrics, `path="/api/v1/health",status_code="200"`)
		assert.Contains(t, metrics, `path="/api/v1/users/:id",status_code="200"`)
		assert.Contains(t, metrics, `path="/",status_code="200"`)
		// Verify HTTP request duration histogram
		assert.Contains(t, metrics, "compozy_http_request_duration_seconds_bucket")
		assert.Contains(t, metrics, `path="/api/v1/health"`)
		// Verify in-flight gauge
		assert.Contains(t, metrics, "compozy_http_requests_in_flight")
	})
	t.Run("Should record metrics for error responses", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Make error requests
		requests := []struct {
			method string
			path   string
			status int
		}{
			{"GET", "/api/v1/error", http.StatusInternalServerError},
			{"GET", "/api/v1/not-found", http.StatusNotFound},
			{"POST", "/api/v1/unknown", http.StatusNotFound},
		}
		for _, req := range requests {
			resp, err := env.MakeRequest(req.method, req.path)
			require.NoError(t, err)
			require.Equal(t, req.status, resp.StatusCode)
			resp.Body.Close()
		}
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Verify error metrics
		assert.Contains(t, metrics, `path="/api/v1/error",status_code="500"`)
		assert.Contains(t, metrics, `path="/api/v1/not-found",status_code="404"`)
		assert.Contains(t, metrics, `method="POST"`)
		assert.Contains(t, metrics, `status_code="404"`)
	})
	t.Run("Should use route templates to prevent high cardinality", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Make requests with different IDs
		for i := 0; i < 10; i++ {
			resp, err := env.MakeRequest("GET", fmt.Sprintf("/api/v1/users/user-%d", i))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()
		}
		// Make requests with compound paths
		resp, err := env.MakeRequest("GET", "/api/v1/workflows/wf-123/executions/exec-456")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Count occurrences of the path label
		userPathCount := strings.Count(metrics, `path="/api/v1/users/:id"`)
		assert.Greater(t, userPathCount, 0, "Should have metrics for user path template")
		// Should NOT have individual user IDs in metrics
		assert.NotContains(t, metrics, `path="/api/v1/users/user-0"`)
		assert.NotContains(t, metrics, `path="/api/v1/users/user-1"`)
		// Check compound path template
		assert.Contains(t, metrics, `path="/api/v1/workflows/:workflow_id/executions/:exec_id"`)
		assert.NotContains(t, metrics, `path="/api/v1/workflows/wf-123/executions/exec-456"`)
	})
	t.Run("Should track concurrent in-flight requests", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Add a slow endpoint for testing
		env.router.GET("/api/v1/slow", func(c *gin.Context) {
			time.Sleep(200 * time.Millisecond)
			c.JSON(http.StatusOK, gin.H{"message": "slow response"})
		})
		// Start multiple concurrent requests
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func() {
				resp, err := env.MakeRequest("GET", "/api/v1/slow")
				if err == nil {
					resp.Body.Close()
				}
				done <- true
			}()
		}
		// Give requests time to start
		time.Sleep(50 * time.Millisecond)
		// Check metrics while requests are in flight
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Extract in-flight value
		lines := strings.Split(metrics, "\n")
		foundInFlight := false
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_http_requests_in_flight{") {
				foundInFlight = true
				// The value comes after the labels
				parts := strings.Split(line, "} ")
				if len(parts) >= 2 {
					value := strings.TrimSpace(parts[1])
					// Should have some requests in flight
					assert.NotEqual(t, "0", value, "Should have requests in flight during concurrent requests")
				}
			}
		}
		assert.True(t, foundInFlight, "Should have found in-flight metric")
		// Wait for all requests to complete
		for i := 0; i < 5; i++ {
			<-done
		}
		// Give metrics time to update
		time.Sleep(100 * time.Millisecond)
		// Check metrics after requests complete
		metrics, err = env.GetMetrics()
		require.NoError(t, err)
		// In-flight should be back to 0
		lines = strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_http_requests_in_flight ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					assert.Equal(t, "0", parts[1], "Should have no requests in flight")
				}
			}
		}
	})
}
