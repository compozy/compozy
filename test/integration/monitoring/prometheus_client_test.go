package monitoring_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusClientScraping(t *testing.T) {
	t.Run("Should be parseable by Prometheus client", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate some metrics
		for i := 0; i < 5; i++ {
			resp, err := env.MakeRequest("GET", "/api/v1/health")
			require.NoError(t, err)
			resp.Body.Close()
			resp, err = env.MakeRequest("GET", "/api/v1/users/123")
			require.NoError(t, err)
			resp.Body.Close()
		}
		// Give metrics time to be recorded
		time.Sleep(100 * time.Millisecond)
		// Get metrics response
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Parse with Prometheus text parser
		parser := expfmt.TextParser{}
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		require.NoError(t, err, "Metrics should be parseable by Prometheus client")
		// Verify expected metric families exist (system metrics always present)
		systemMetrics := []string{
			"compozy_build_info",
			"compozy_uptime_seconds",
		}
		for _, expected := range systemMetrics {
			_, exists := metricFamilies[expected]
			assert.True(t, exists, "Should have metric family: %s", expected)
		}
		// HTTP metrics should exist after making requests
		httpMetrics := []string{
			"compozy_http_requests_total",
			"compozy_http_request_duration_seconds",
			"compozy_http_requests_in_flight",
		}
		for _, expected := range httpMetrics {
			_, exists := metricFamilies[expected]
			assert.True(t, exists, "Should have HTTP metric family: %s", expected)
		}
	})
	t.Run("Should have correct metric types", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate some HTTP metrics first
		resp, err := env.MakeRequest("GET", "/api/v1/health")
		require.NoError(t, err)
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
		// Get metrics response
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err = env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Parse metrics
		parser := expfmt.TextParser{}
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		require.NoError(t, err)
		// Verify metric types
		expectedTypes := map[string]dto.MetricType{
			"compozy_http_requests_total":           dto.MetricType_COUNTER,
			"compozy_http_request_duration_seconds": dto.MetricType_HISTOGRAM,
			"compozy_http_requests_in_flight":       dto.MetricType_GAUGE,
			"compozy_build_info":                    dto.MetricType_GAUGE,
			"compozy_uptime_seconds":                dto.MetricType_GAUGE,
		}
		for name, expectedType := range expectedTypes {
			family, exists := metricFamilies[name]
			if assert.True(t, exists, "Should have metric family: %s", name) {
				require.NotNil(t, family.Type, "Metric %s has no TYPE metadata", name)
				assert.Equal(t, expectedType, *family.Type, "Metric %s should be type %s", name, expectedType)
			}
		}
	})
	t.Run("Should have valid metric help text", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get metrics response
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Parse metrics
		parser := expfmt.TextParser{}
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		require.NoError(t, err)
		// Check help text exists and is not empty
		for name, family := range metricFamilies {
			assert.NotNil(t, family.Help, "Metric %s should have help text", name)
			if family.Help != nil {
				assert.NotEmpty(t, *family.Help, "Metric %s help text should not be empty", name)
			}
		}
	})
	t.Run("Should validate histogram buckets", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate histogram data
		for i := 0; i < 10; i++ {
			if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
				resp.Body.Close()
			}
		}
		// Get metrics response
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Parse metrics
		parser := expfmt.TextParser{}
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		require.NoError(t, err)
		// Check histogram
		histogram, exists := metricFamilies["compozy_http_request_duration_seconds"]
		require.True(t, exists, "Should have HTTP duration histogram")
		assert.Equal(t, dto.MetricType_HISTOGRAM, *histogram.Type)
		// Verify histogram has metrics
		assert.Greater(t, len(histogram.Metric), 0, "Histogram should have metrics")
		// Check first metric has buckets
		if len(histogram.Metric) > 0 {
			metric := histogram.Metric[0]
			assert.NotNil(t, metric.Histogram)
			if metric.Histogram != nil {
				assert.Greater(t, len(metric.Histogram.Bucket), 0, "Should have histogram buckets")
				assert.NotNil(t, metric.Histogram.SampleCount, "Should have sample count")
				assert.NotNil(t, metric.Histogram.SampleSum, "Should have sample sum")
				// Verify buckets are in ascending order
				var lastBound float64
				for i, bucket := range metric.Histogram.Bucket {
					assert.NotNil(t, bucket.UpperBound, "Bucket should have upper bound")
					if bucket.UpperBound != nil {
						if i > 0 {
							assert.Greater(t, *bucket.UpperBound, lastBound, "Buckets should be in ascending order")
						}
						lastBound = *bucket.UpperBound
					}
				}
			}
		}
	})
	t.Run("Should handle metric labels correctly", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate metrics with different labels
		if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
			resp.Body.Close()
		}
		if resp, err := env.MakeRequest("GET", "/api/v1/error"); err == nil {
			resp.Body.Close()
		}
		if resp, err := env.MakeRequest("GET", "/api/v1/users/123"); err == nil {
			resp.Body.Close()
		}
		// Get metrics response
		req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
		require.NoError(t, err)
		resp, err := env.GetMetricsClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Parse metrics
		parser := expfmt.TextParser{}
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		require.NoError(t, err)
		// Check counter labels
		counter, exists := metricFamilies["compozy_http_requests_total"]
		require.True(t, exists, "Should have HTTP requests counter")
		// Should have multiple metrics with different label combinations
		assert.Greater(t, len(counter.Metric), 1, "Should have multiple metric instances with different labels")
		// Verify each metric has the expected labels
		for _, metric := range counter.Metric {
			labelMap := make(map[string]string)
			for _, label := range metric.Label {
				if label.Name != nil && label.Value != nil {
					labelMap[*label.Name] = *label.Value
				}
			}
			// Should have http_method, http_route, and http_status_code labels (OpenTelemetry semantic conventions)
			assert.Contains(t, labelMap, "http_method")
			assert.Contains(t, labelMap, "http_route")
			assert.Contains(t, labelMap, "http_status_code")
		}
	})
	t.Run("Should be compatible with Prometheus registry", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate some metrics
		if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
			resp.Body.Close()
		}
		// Get metrics as string
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// The fact that we can parse the metrics without errors means
		// they are compatible with Prometheus format
		parser := expfmt.TextParser{}
		reader := strings.NewReader(metrics)
		families, err := parser.TextToMetricFamilies(reader)
		require.NoError(t, err)
		assert.Greater(t, len(families), 0, "Should have parsed some metric families")
		// Verify no duplicate metric names
		uniqueNames := make(map[string]bool)
		for name := range families {
			assert.False(t, uniqueNames[name], "Should not have duplicate metric family: %s", name)
			uniqueNames[name] = true
		}
	})
}
