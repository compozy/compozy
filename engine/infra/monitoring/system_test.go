package monitoring

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestSystemMetrics(t *testing.T) {
	t.Run("Should initialize build info gauge", func(t *testing.T) {
		resetSystemMetrics()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		InitSystemMetrics(ctx, meter)
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)
		// Find build_info metric
		buildInfoFound := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_build_info" {
					buildInfoFound = true
					gauge, ok := m.Data.(metricdata.Gauge[float64])
					require.True(t, ok, "build_info should be a float64 gauge")
					require.Len(t, gauge.DataPoints, 1, "should have one data point")
					// Verify value is 1
					assert.Equal(t, float64(1), gauge.DataPoints[0].Value)
					// Verify labels
					attrs := gauge.DataPoints[0].Attributes.ToSlice()
					labelMap := make(map[string]string)
					for _, attr := range attrs {
						labelMap[string(attr.Key)] = attr.Value.AsString()
					}
					// Check required labels exist
					assert.Contains(t, labelMap, "version")
					assert.Contains(t, labelMap, "commit_hash")
					assert.Contains(t, labelMap, "go_version")
					// Verify go_version matches runtime
					assert.Equal(t, runtime.Version(), labelMap["go_version"])
				}
			}
		}
		assert.True(t, buildInfoFound, "compozy_build_info metric not found")
	})
	t.Run("Should initialize uptime gauge", func(t *testing.T) {
		resetSystemMetrics()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		InitSystemMetrics(ctx, meter)
		// Wait a bit to ensure uptime is > 0
		time.Sleep(100 * time.Millisecond)
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)
		// Find uptime metric
		uptimeFound := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_uptime_seconds" {
					uptimeFound = true
					gauge, ok := m.Data.(metricdata.Gauge[float64])
					require.True(t, ok, "uptime should be a float64 gauge")
					require.Len(t, gauge.DataPoints, 1, "should have one data point")
					// Verify uptime is positive
					assert.Greater(t, gauge.DataPoints[0].Value, float64(0), "uptime should be positive")
					// Verify uptime is reasonable (less than 1 second for test)
					assert.Less(t, gauge.DataPoints[0].Value, float64(1), "uptime should be less than 1 second in test")
				}
			}
		}
		assert.True(t, uptimeFound, "compozy_uptime_seconds metric not found")
	})
	t.Run("Should have monotonic uptime", func(t *testing.T) {
		resetSystemMetrics()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		InitSystemMetrics(ctx, meter)
		// First reading
		var rm1 metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm1)
		require.NoError(t, err)
		uptime1 := getUptimeValue(t, &rm1)
		// Wait and take second reading
		time.Sleep(50 * time.Millisecond)
		var rm2 metricdata.ResourceMetrics
		err = reader.Collect(ctx, &rm2)
		require.NoError(t, err)
		uptime2 := getUptimeValue(t, &rm2)
		// Verify monotonic increase
		assert.Greater(t, uptime2, uptime1, "uptime should increase monotonically")
	})
}

func TestBuildInfoExtraction(t *testing.T) {
	t.Run("Should use ldflags values when set", func(t *testing.T) {
		// Save original values
		origVersion := Version
		origCommit := CommitHash
		defer func() {
			Version = origVersion
			CommitHash = origCommit
		}()
		// Set test values
		Version = "v1.2.3"
		CommitHash = "abc123"
		version, commit, goVersion := getBuildInfo()
		assert.Equal(t, "v1.2.3", version)
		assert.Equal(t, "abc123", commit)
		assert.Equal(t, runtime.Version(), goVersion)
	})
	t.Run("Should fallback to unknown when ldflags not set", func(t *testing.T) {
		// Save original values
		origVersion := Version
		origCommit := CommitHash
		defer func() {
			Version = origVersion
			CommitHash = origCommit
		}()
		// Set to unknown
		Version = "unknown"
		CommitHash = "unknown"
		version, commit, goVersion := getBuildInfo()
		// Version might get a value from debug.ReadBuildInfo
		assert.NotEmpty(t, version)
		assert.Equal(t, "unknown", commit)
		assert.Equal(t, runtime.Version(), goVersion)
	})
}

func TestSystemMetricsIdempotency(t *testing.T) {
	t.Run("Should handle multiple initializations safely", func(t *testing.T) {
		resetSystemMetrics()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		// Initialize multiple times
		InitSystemMetrics(ctx, meter)
		InitSystemMetrics(ctx, meter)
		InitSystemMetrics(ctx, meter)
		// Should still work correctly
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)
		// Count metrics
		buildInfoCount := 0
		uptimeCount := 0
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_build_info" {
					buildInfoCount++
				}
				if m.Name == "compozy_uptime_seconds" {
					uptimeCount++
				}
			}
		}
		// Should have exactly one of each metric
		assert.Equal(t, 1, buildInfoCount, "should have exactly one build_info metric")
		assert.Equal(t, 1, uptimeCount, "should have exactly one uptime metric")
	})
}

func TestLabelValidation(t *testing.T) {
	t.Run("Should only use allowed labels", func(t *testing.T) {
		resetSystemMetrics()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		InitSystemMetrics(ctx, meter)
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)
		// Allowed labels from tech spec
		allowedLabels := map[string]bool{
			"version":     true,
			"commit_hash": true,
			"go_version":  true,
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if !strings.HasPrefix(m.Name, "compozy_") {
					continue
				}
				gauge, ok := m.Data.(metricdata.Gauge[float64])
				require.True(t, ok, "metric %s should be a gauge", m.Name)
				switch m.Name {
				case "compozy_build_info":
					require.Len(t, gauge.DataPoints, 1)
					attrs := gauge.DataPoints[0].Attributes.ToSlice()
					assert.Equal(
						t,
						len(allowedLabels),
						len(attrs),
						"build_info should have %d labels",
						len(allowedLabels),
					)
					for _, attr := range attrs {
						assert.True(
							t,
							allowedLabels[string(attr.Key)],
							"label %s is not allowed for build_info",
							string(attr.Key),
						)
					}
				case "compozy_uptime_seconds":
					require.Len(t, gauge.DataPoints, 1)
					assert.Zero(t, gauge.DataPoints[0].Attributes.Len(), "uptime metric should not have any labels")
				}
			}
		}
	})
}

func TestSpecialCharactersInVersion(t *testing.T) {
	t.Run("Should handle special characters in version strings", func(t *testing.T) {
		// Save original values
		origVersion := Version
		defer func() {
			Version = origVersion
		}()
		// Test with special characters
		Version = "v1.2.3-beta+build.456"
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")
		ctx := context.Background()
		// Reset metrics for clean test
		resetSystemMetrics()
		InitSystemMetrics(ctx, meter)
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)
		// Find and verify version label
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_build_info" {
					gauge, ok := m.Data.(metricdata.Gauge[float64])
					require.True(t, ok)
					attrs := gauge.DataPoints[0].Attributes.ToSlice()
					for _, attr := range attrs {
						if string(attr.Key) == "version" {
							assert.Equal(t, "v1.2.3-beta+build.456", attr.Value.AsString())
						}
					}
				}
			}
		}
	})
}

// Helper function to extract uptime value from metrics
func getUptimeValue(t *testing.T, rm *metricdata.ResourceMetrics) float64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "compozy_uptime_seconds" {
				gauge, ok := m.Data.(metricdata.Gauge[float64])
				require.True(t, ok)
				require.Len(t, gauge.DataPoints, 1)
				return gauge.DataPoints[0].Value
			}
		}
	}
	t.Fatal("uptime metric not found")
	return 0
}
