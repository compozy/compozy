package monitoring_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemHealthMetrics(t *testing.T) {
	t.Run("Should expose build info metrics", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Give metrics time to initialize
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Verify build info gauge exists
		assert.Contains(t, metrics, "compozy_build_info")
		// Parse build info line to verify labels
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_build_info{") {
				// Should have version, commit_hash, and go_version labels
				assert.Contains(t, line, `version=`)
				assert.Contains(t, line, `commit_hash=`)
				assert.Contains(t, line, `go_version=`)
				// Value should always be 1
				assert.Contains(t, line, "} 1")
				// Extract go_version to verify it matches runtime
				if strings.Contains(line, `go_version="`) {
					start := strings.Index(line, `go_version="`) + len(`go_version="`)
					end := strings.Index(line[start:], `"`)
					if end > 0 {
						goVersion := line[start : start+end]
						// Should contain the Go version
						assert.Contains(t, goVersion, "go")
					}
				}
			}
		}
	})
	t.Run("Should track uptime metrics", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Get initial metrics
		metrics1, err := env.GetMetrics()
		require.NoError(t, err)
		// Extract initial uptime value
		var uptime1 float64
		lines := strings.Split(metrics1, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_uptime_seconds{") {
				// Parse value after the labels
				parts := strings.Split(line, "} ")
				if len(parts) >= 2 {
					_, err := fmt.Sscanf(parts[1], "%f", &uptime1)
					require.NoError(t, err)
				}
			}
		}
		// Wait a bit
		time.Sleep(500 * time.Millisecond)
		// Get metrics again
		metrics2, err := env.GetMetrics()
		require.NoError(t, err)
		// Extract new uptime value
		var uptime2 float64
		lines = strings.Split(metrics2, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_uptime_seconds{") {
				// Parse value after the labels
				parts := strings.Split(line, "} ")
				if len(parts) >= 2 {
					_, err := fmt.Sscanf(parts[1], "%f", &uptime2)
					require.NoError(t, err)
				}
			}
		}
		// Uptime should have increased
		assert.Greater(t, uptime2, uptime1)
		// The difference should be approximately the sleep time
		diff := uptime2 - uptime1
		assert.InDelta(t, 0.5, diff, 0.1, "Uptime difference should be approximately 0.5 seconds")
	})
	t.Run("Should use appropriate metric types", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Give metrics time to initialize
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Check HELP and TYPE lines
		lines := strings.Split(metrics, "\n")
		var foundBuildInfoType, foundUptimeType bool
		var foundBuildInfoHelp, foundUptimeHelp bool
		for _, line := range lines {
			// Check TYPE lines
			if line == "# TYPE compozy_build_info gauge" {
				foundBuildInfoType = true
			}
			if line == "# TYPE compozy_uptime_seconds_total counter" {
				foundUptimeType = true
			}
			// Check HELP lines
			if strings.HasPrefix(line, "# HELP compozy_build_info") {
				foundBuildInfoHelp = true
				assert.Contains(t, line, "Build information")
			}
			if strings.HasPrefix(line, "# HELP compozy_uptime_seconds_total") {
				foundUptimeHelp = true
				assert.Contains(t, line, "uptime")
			}
		}
		assert.True(t, foundBuildInfoType, "Should have TYPE for build_info as gauge")
		assert.True(t, foundUptimeType, "Should have TYPE for uptime as gauge")
		assert.True(t, foundBuildInfoHelp, "Should have HELP for build_info")
		assert.True(t, foundUptimeHelp, "Should have HELP for uptime")
	})
	t.Run("Should handle build info with special characters", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Give metrics time to initialize
		time.Sleep(100 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Verify that special characters in version strings are properly escaped
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "compozy_build_info{") {
				// Labels should be properly quoted
				assert.Contains(t, line, `version="`)
				assert.Contains(t, line, `commit_hash="`)
				assert.Contains(t, line, `go_version="`)
				// Verify no unescaped quotes within label values
				// This is a basic check - Prometheus client should handle escaping
				labelStart := strings.Index(line, "{")
				labelEnd := strings.Index(line, "}")
				if labelStart >= 0 && labelEnd > labelStart {
					labels := line[labelStart+1 : labelEnd]
					// Count quotes - should be even (paired)
					quoteCount := strings.Count(labels, `"`)
					assert.Equal(t, 0, quoteCount%2, "Quotes should be properly paired")
				}
			}
		}
	})
}
