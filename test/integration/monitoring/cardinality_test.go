package monitoring_test

import (
	"fmt"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseMetrics parses Prometheus metrics text format into metric families
func parseMetrics(t *testing.T, metricsText string) map[string]*dto.MetricFamily {
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(strings.NewReader(metricsText))
	require.NoError(t, err)
	return mf
}

// extractLabelValues extracts unique values for a specific label from a metric family
func extractLabelValues(mf *dto.MetricFamily, labelName string) map[string]bool {
	values := make(map[string]bool)
	if mf == nil {
		return values
	}
	for _, metric := range mf.Metric {
		for _, label := range metric.Label {
			if label.GetName() == labelName {
				values[label.GetValue()] = true
				break
			}
		}
	}
	return values
}

// countMetricsWithLabels counts metrics that match specific label criteria
func countMetricsWithLabels(mf *dto.MetricFamily, labelCriteria map[string]string) int {
	if mf == nil {
		return 0
	}
	count := 0
	for _, metric := range mf.Metric {
		matches := true
		for expectedName, expectedValue := range labelCriteria {
			found := false
			for _, label := range metric.Label {
				if label.GetName() == expectedName && label.GetValue() == expectedValue {
					found = true
					break
				}
			}
			if !found {
				matches = false
				break
			}
		}
		if matches {
			count++
		}
	}
	return count
}

func TestMetricCardinalityLimits(t *testing.T) {
	t.Run("Should use route templates to prevent cardinality explosion", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate requests with many different IDs
		uniqueIDs := 100
		for i := 0; i < uniqueIDs; i++ {
			// Different user IDs
			resp, err := env.MakeRequest("GET", fmt.Sprintf("/api/v1/users/user-%d", i))
			require.NoError(t, err)
			resp.Body.Close()
			// Different workflow and execution IDs
			resp, err = env.MakeRequest("GET", fmt.Sprintf("/api/v1/workflows/wf-%d/executions/exec-%d", i, i*2))
			require.NoError(t, err)
			resp.Body.Close()
		}
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Parse metrics and extract path labels
		metricFamilies := parseMetrics(t, metrics)
		pathLabels := extractLabelValues(metricFamilies["compozy_http_requests_total"], "http_route")
		// Should have only template paths, not individual IDs
		assert.Contains(t, pathLabels, "/api/v1/users/:id")
		assert.Contains(t, pathLabels, "/api/v1/workflows/:workflow_id/executions/:exec_id")
		// Should NOT have individual paths
		for i := 0; i < uniqueIDs; i++ {
			assert.NotContains(t, pathLabels, fmt.Sprintf("/api/v1/users/user-%d", i))
			assert.NotContains(t, pathLabels, fmt.Sprintf("/api/v1/workflows/wf-%d/executions/exec-%d", i, i*2))
		}
		// Total unique paths should be limited
		assert.Less(t, len(pathLabels), 10, "Should have limited number of unique path labels")
	})
	t.Run("Should handle unmatched routes with single label", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate requests to non-existent routes
		nonExistentRoutes := []string{
			"/api/v1/unknown",
			"/api/v1/missing/endpoint",
			"/api/v2/test",
			"/random/path",
			"/another/missing/route",
		}
		for _, route := range nonExistentRoutes {
			resp, err := env.MakeRequest("GET", route)
			require.NoError(t, err)
			assert.Equal(t, 404, resp.StatusCode)
			resp.Body.Close()
		}
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Parse metrics and count 404 responses with "unmatched" path
		metricFamilies := parseMetrics(t, metrics)
		unmatchedCount := countMetricsWithLabels(metricFamilies["compozy_http_requests_total"], map[string]string{
			"http_route":       "unmatched",
			"http_status_code": "404",
		})
		// Should have exactly one "unmatched" metric line for 404s
		assert.Equal(t, 1, unmatchedCount, "Should consolidate all unmatched routes to single metric")
		// Should NOT have individual unmatched paths in metrics
		for _, route := range nonExistentRoutes {
			assert.NotContains(t, metrics, fmt.Sprintf(`path=%q`, route))
		}
	})
	t.Run("Should maintain reasonable cardinality with mixed traffic", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate mixed traffic pattern
		for i := 0; i < 50; i++ {
			// Static routes
			if resp, err := env.MakeRequest("GET", "/"); err == nil {
				resp.Body.Close()
			}
			if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
				resp.Body.Close()
			}
			// Dynamic routes with parameters
			if resp, err := env.MakeRequest("GET", fmt.Sprintf("/api/v1/users/%d", i)); err == nil {
				resp.Body.Close()
			}
			if resp, err := env.MakeRequest("GET", fmt.Sprintf("/api/v1/workflows/wf-%d/executions/exec-%d", i, i)); err == nil {
				resp.Body.Close()
			}
			// Error routes
			if resp, err := env.MakeRequest("GET", "/api/v1/error"); err == nil {
				resp.Body.Close()
			}
			if resp, err := env.MakeRequest("GET", fmt.Sprintf("/api/v1/notfound-%d", i)); err == nil {
				resp.Body.Close()
			}
			// Different methods
			if resp, err := env.MakeRequest("POST", fmt.Sprintf("/api/v1/users/%d", i)); err == nil {
				resp.Body.Close()
			}
			if resp, err := env.MakeRequest("PUT", fmt.Sprintf("/api/v1/users/%d", i)); err == nil {
				resp.Body.Close()
			}
		}
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Parse metrics and count unique label combinations
		metricFamilies := parseMetrics(t, metrics)
		httpMetrics := metricFamilies["compozy_http_requests_total"]
		uniqueCombinations := make(map[string]bool)
		if httpMetrics != nil {
			for _, metric := range httpMetrics.Metric {
				// Create a string representation of all labels
				var labelPairs []string
				for _, label := range metric.Label {
					labelPairs = append(labelPairs, fmt.Sprintf(`%s=%q`, label.GetName(), label.GetValue()))
				}
				labelString := fmt.Sprintf("{%s}", strings.Join(labelPairs, ","))
				uniqueCombinations[labelString] = true
			}
		}
		// Should have reasonable number of combinations
		// With proper templating: ~5 paths × ~3 methods × ~3 status codes = ~45 combinations max
		assert.Less(t, len(uniqueCombinations), 50, "Should maintain reasonable cardinality")
	})
	t.Run("Should apply cardinality limits to Temporal metrics", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Note: We can't easily test Temporal metrics cardinality without
		// actually running workflows, but we can verify the structure
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Parse metrics and check Temporal metric structure
		metricFamilies := parseMetrics(t, metrics)
		// Check for any Temporal workflow metrics
		for name, mf := range metricFamilies {
			if strings.Contains(name, "compozy_temporal_workflow_") {
				// Should use workflow_type label, not workflow_id
				workflowTypeLabels := extractLabelValues(mf, "workflow_type")
				assert.NotEmpty(t, workflowTypeLabels, "Should have workflow_type labels")
				// Should NOT have high-cardinality labels
				workflowIDLabels := extractLabelValues(mf, "workflow_id")
				executionIDLabels := extractLabelValues(mf, "execution_id")
				runIDLabels := extractLabelValues(mf, "run_id")
				assert.Empty(t, workflowIDLabels, "Should not have workflow_id labels")
				assert.Empty(t, executionIDLabels, "Should not have execution_id labels")
				assert.Empty(t, runIDLabels, "Should not have run_id labels")
			}
		}
	})
}
