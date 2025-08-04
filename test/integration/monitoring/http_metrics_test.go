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
		// Verify all required HTTP metrics are properly recorded
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				t.Logf("Failed to get metrics: %v", err)
				return false
			}
			// Verify counter metrics with correct route templates
			return strings.Contains(metrics, "compozy_http_requests_total") &&
				strings.Contains(metrics, `http_route="/api/v1/health",http_status_code="200"`) &&
				strings.Contains(metrics, `http_route="/api/v1/users/:id",http_status_code="200"`) &&
				strings.Contains(metrics, `http_route="/",http_status_code="200"`) &&
				strings.Contains(metrics, "compozy_http_request_duration_seconds_bucket") &&
				strings.Contains(metrics, "compozy_http_requests_in_flight")
		}, 2*time.Second, 50*time.Millisecond, "All HTTP metrics must be recorded with correct route templates")
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
		// Wait for error metrics to be recorded
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				return false
			}
			// Check all required error metrics are present
			return strings.Contains(metrics, `http_route="/api/v1/error",http_status_code="500"`) &&
				strings.Contains(metrics, `http_route="/api/v1/not-found",http_status_code="404"`) &&
				strings.Contains(metrics, `http_method="POST"`) &&
				strings.Contains(metrics, `http_status_code="404"`)
		}, 2*time.Second, 50*time.Millisecond, "Expected error metrics should be recorded")
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
		// Wait for route template metrics to be recorded
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				return false
			}
			// Check route template metrics and absence of individual IDs
			userPathCount := strings.Count(metrics, `http_route="/api/v1/users/:id"`)
			return userPathCount > 0 &&
				!strings.Contains(metrics, `http_route="/api/v1/users/user-0"`) &&
				!strings.Contains(metrics, `http_route="/api/v1/users/user-1"`) &&
				strings.Contains(metrics, `http_route="/api/v1/workflows/:workflow_id/executions/:exec_id"`) &&
				!strings.Contains(metrics, `http_route="/api/v1/workflows/wf-123/executions/exec-456"`)
		}, 2*time.Second, 50*time.Millisecond, "Expected route template metrics should be recorded")
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
		// Parse metrics and check in-flight value
		metricFamilies := parseMetrics(t, metrics)
		inFlightMetric := metricFamilies["compozy_http_requests_in_flight"]
		require.NotNil(t, inFlightMetric, "Should have found in-flight metric")
		// Should have some requests in flight
		foundNonZero := false
		for _, metric := range inFlightMetric.Metric {
			if metric.GetGauge().GetValue() > 0 {
				foundNonZero = true
				break
			}
		}
		assert.True(t, foundNonZero, "Should have requests in flight during concurrent requests")
		// Wait for all requests to complete
		for i := 0; i < 5; i++ {
			<-done
		}
		// Wait for in-flight count to return to 0
		assert.Eventually(t, func() bool {
			metrics, err := env.GetMetrics()
			if err != nil {
				return false
			}
			metricFamilies := parseMetrics(t, metrics)
			inFlightMetric := metricFamilies["compozy_http_requests_in_flight"]
			if inFlightMetric == nil {
				return true // No metric means 0 in-flight
			}
			// Check all metrics to ensure they're all 0
			for _, metric := range inFlightMetric.Metric {
				if metric.GetGauge().GetValue() != 0 {
					return false
				}
			}
			return true
		}, 2*time.Second, 50*time.Millisecond, "In-flight requests should return to 0")
	})
}
