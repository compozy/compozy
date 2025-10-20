package monitoring_test

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
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
		for i := range 10 {
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
		for range 5 {
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
		for range 5 {
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
	t.Run("Should expose request and response size histograms", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()

		const requestPath = "/api/v1/upload"
		reqPayload := strings.Repeat("a", 2048)
		respPayload := strings.Repeat("b", 4096)

		env.router.POST(requestPath, func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if len(body) == 0 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "empty body"})
				return
			}
			c.Data(http.StatusCreated, "text/plain", []byte(respPayload))
		})

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			env.httpServer.URL+requestPath,
			strings.NewReader(reqPayload),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := env.GetHTTPClient().Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.NoError(t, resp.Body.Close())

		metrics, err := env.GetMetrics()
		require.NoError(t, err)

		metricFamilies := parseMetrics(t, metrics)
		labels := map[string]string{
			"http_method":      http.MethodPost,
			"http_route":       requestPath,
			"http_status_code": "201",
		}

		requestHistogram := findHistogramMetric(metricFamilies["compozy_http_request_size_bytes"], labels)
		require.NotNil(t, requestHistogram, "request size histogram metric should exist")
		assert.GreaterOrEqual(t, requestHistogram.GetHistogram().GetSampleCount(), uint64(1))
		assert.InDelta(t, float64(len(reqPayload)), requestHistogram.GetHistogram().GetSampleSum(), 1.0)
		assertBucketBoundaries(t, requestHistogram, []float64{100, 1000, 10000, 100000, 1000000, 10000000, 100000000})

		responseHistogram := findHistogramMetric(metricFamilies["compozy_http_response_size_bytes"], labels)
		require.NotNil(t, responseHistogram, "response size histogram metric should exist")
		assert.GreaterOrEqual(t, responseHistogram.GetHistogram().GetSampleCount(), uint64(1))
		assert.InDelta(t, float64(len(respPayload)), responseHistogram.GetHistogram().GetSampleSum(), 1.0)
		assertBucketBoundaries(t, responseHistogram, []float64{100, 1000, 10000, 100000, 1000000, 10000000, 100000000})
	})
}

func findHistogramMetric(mf *dto.MetricFamily, labels map[string]string) *dto.Metric {
	if mf == nil {
		return nil
	}
	for _, metric := range mf.Metric {
		if metric.GetHistogram() == nil {
			continue
		}
		if matchMetricLabels(metric, labels) {
			return metric
		}
	}
	return nil
}

func matchMetricLabels(metric *dto.Metric, labels map[string]string) bool {
	for key, value := range labels {
		found := false
		for _, label := range metric.Label {
			if label.GetName() == key && label.GetValue() == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func assertBucketBoundaries(t *testing.T, metric *dto.Metric, expected []float64) {
	t.Helper()
	buckets := metric.GetHistogram().GetBucket()
	require.GreaterOrEqual(t, len(buckets), len(expected))
	for index, boundary := range expected {
		assert.InDelta(t, boundary, buckets[index].GetUpperBound(), 0.001)
	}
	if len(buckets) > len(expected) {
		assert.True(t, math.IsInf(buckets[len(buckets)-1].GetUpperBound(), 1))
	}
}
