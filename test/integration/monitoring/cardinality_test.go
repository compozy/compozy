package monitoring_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		// Count unique path labels in HTTP metrics
		pathLabels := make(map[string]bool)
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "compozy_http_requests_total{") {
				// Extract path label
				start := strings.Index(line, `path="`)
				if start >= 0 {
					start += 6
					end := strings.Index(line[start:], `"`)
					if end >= 0 {
						path := line[start : start+end]
						pathLabels[path] = true
					}
				}
			}
		}
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
		// Count 404 responses with "unmatched" path
		unmatchedCount := 0
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "compozy_http_requests_total") &&
				strings.Contains(line, `path="unmatched"`) &&
				strings.Contains(line, `status_code="404"`) {
				unmatchedCount++
			}
		}
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
		// Count total unique label combinations for HTTP metrics
		uniqueCombinations := make(map[string]bool)
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "compozy_http_requests_total{") {
				// Extract the label portion
				start := strings.Index(line, "{")
				end := strings.Index(line, "}")
				if start >= 0 && end > start {
					labels := line[start : end+1]
					uniqueCombinations[labels] = true
				}
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
		// Check that Temporal metrics use workflow_type label, not workflow_id
		if strings.Contains(metrics, "compozy_temporal_workflow_") {
			// Should use workflow_type label
			assert.Contains(t, metrics, `workflow_type="`)
			// Should NOT have workflow_id or execution_id labels
			assert.NotContains(t, metrics, `workflow_id="`)
			assert.NotContains(t, metrics, `execution_id="`)
			assert.NotContains(t, metrics, `run_id="`)
		}
	})
}
