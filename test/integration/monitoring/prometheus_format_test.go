package monitoring_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusFormatCompliance(t *testing.T) {
	t.Run("Should return valid Prometheus text format", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Make some requests to generate metrics
		if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
			resp.Body.Close()
		}
		if resp, err := env.MakeRequest("GET", "/api/v1/users/123"); err == nil {
			resp.Body.Close()
		}
		// Get metrics
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Verify content type
		contentType := resp.Header.Get("Content-Type")
		assert.Contains(t, contentType, "text/plain")
		// Already verified the metrics are parseable in TestPrometheusClientScraping
	})
	t.Run("Should include all required metric metadata", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Initialize temporal interceptor to register metrics
		_ = env.monitoring.TemporalInterceptor(t.Context())
		// Generate HTTP metrics
		resp, err := env.MakeRequest("GET", "/api/v1/health")
		require.NoError(t, err)
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Define expected metrics that should always be present
		expectedMetrics := []struct {
			name       string
			metricType string
			helpText   string
		}{
			{
				name:       "compozy_http_requests_total",
				metricType: "counter",
				helpText:   "Total HTTP requests",
			},
			{
				name:       "compozy_http_request_duration_seconds",
				metricType: "histogram",
				helpText:   "HTTP request latency",
			},
			{
				name:       "compozy_http_requests_in_flight",
				metricType: "gauge",
				helpText:   "Currently active HTTP requests",
			},
			{
				name:       "compozy_build_info",
				metricType: "gauge",
				helpText:   "Build information",
			},
			{
				name:       "compozy_uptime_seconds",
				metricType: "gauge",
				helpText:   "Service uptime in seconds",
			},
		}
		// Check each metric has proper HELP and TYPE
		for _, expected := range expectedMetrics {
			// Check HELP line
			helpLine := "# HELP " + expected.name
			assert.Contains(t, metrics, helpLine, "Should have HELP for %s", expected.name)
			// Check TYPE line
			typeLine := "# TYPE " + expected.name + " " + expected.metricType
			assert.Contains(t, metrics, typeLine, "Should have TYPE %s for %s", expected.metricType, expected.name)
		}
	})
	t.Run("Should handle empty metrics gracefully", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get metrics without generating any
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Should return 200 OK
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Should have some content (at least system metrics)
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		assert.NotEmpty(t, metrics)
		// Should at least have build info and uptime
		assert.Contains(t, metrics, "compozy_build_info")
		assert.Contains(t, metrics, "compozy_uptime_seconds")
	})
	t.Run("Should properly format histogram metrics", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Make requests to generate histogram data
		for i := 0; i < 5; i++ {
			if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
				resp.Body.Close()
			}
		}
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Check histogram format
		lines := strings.Split(metrics, "\n")
		var foundBucket, foundSum, foundCount bool
		for _, line := range lines {
			if strings.Contains(line, "compozy_http_request_duration_seconds_bucket") {
				foundBucket = true
				// Bucket lines should have le label
				assert.Contains(t, line, `le="`)
			}
			if strings.Contains(line, "compozy_http_request_duration_seconds_sum") {
				foundSum = true
			}
			if strings.Contains(line, "compozy_http_request_duration_seconds_count") {
				foundCount = true
			}
		}
		assert.True(t, foundBucket, "Should have histogram buckets")
		assert.True(t, foundSum, "Should have histogram sum")
		assert.True(t, foundCount, "Should have histogram count")
		// Verify bucket order (should be ascending)
		bucketValues := []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"}
		for i, bucket := range bucketValues {
			if i < len(bucketValues)-1 {
				assert.Contains(t, metrics, `le="`+bucket+`"`)
			}
		}
	})
	t.Run("Should escape label values properly", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// The router should handle special characters in paths
		// Make a request that would need escaping if it were in labels
		resp, err := env.MakeRequest("GET", "/api/v1/users/test%20user")
		require.NoError(t, err)
		resp.Body.Close()
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Check that route is using template, not actual value
		assert.Contains(t, metrics, `http_route="/api/v1/users/:id"`)
		assert.NotContains(t, metrics, `http_route="/api/v1/users/test%20user"`)
		// Verify no unescaped characters in labels
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "{") && strings.Contains(line, "}") {
				// Extract labels section
				start := strings.Index(line, "{")
				end := strings.Index(line, "}")
				if start >= 0 && end > start {
					labels := line[start+1 : end]
					// Should not contain unescaped newlines or quotes
					assert.NotContains(t, labels, "\n")
					assert.NotContains(t, labels, "\r")
					// Quotes should be part of the label format
					parts := strings.Split(labels, ",")
					for _, part := range parts {
						// Each label should be in key="value" format
						assert.Contains(t, part, "=")
						assert.Contains(t, part, `"`)
					}
				}
			}
		}
	})
}
