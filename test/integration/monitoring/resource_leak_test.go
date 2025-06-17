package monitoring_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitoringResourceLeaks(t *testing.T) {
	t.Run("Should not leak goroutines", func(t *testing.T) {
		// Get baseline goroutine count
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		baselineGoroutines := runtime.NumGoroutine()
		// Create and destroy multiple monitoring environments
		for i := 0; i < 5; i++ {
			env := SetupTestEnvironment(t)
			// Make some requests
			for j := 0; j < 10; j++ {
				resp, err := env.MakeRequest("GET", "/api/v1/health")
				require.NoError(t, err)
				resp.Body.Close()
			}
			// Get metrics
			_, err := env.GetMetrics()
			require.NoError(t, err)
			// Cleanup
			env.Cleanup()
		}
		// Give time for goroutines to exit
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		// Check goroutine count
		finalGoroutines := runtime.NumGoroutine()
		// Allow for some variance but should be close to baseline
		diff := finalGoroutines - baselineGoroutines
		assert.LessOrEqual(
			t,
			diff,
			2,
			"Should not leak goroutines (baseline: %d, final: %d)",
			baselineGoroutines,
			finalGoroutines,
		)
	})
	t.Run("Should handle high volume of requests without memory issues", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Force GC and get baseline memory
		runtime.GC()
		var baseline runtime.MemStats
		runtime.ReadMemStats(&baseline)
		// Generate high volume of requests
		numRequests := 1000
		for i := 0; i < numRequests; i++ {
			// Vary the endpoints to create different metric labels
			endpoints := []string{
				"/api/v1/health",
				fmt.Sprintf("/api/v1/users/%d", i%100),
				"/api/v1/error",
				"/",
			}
			path := endpoints[i%len(endpoints)]
			resp, err := env.MakeRequest("GET", path)
			require.NoError(t, err)
			resp.Body.Close()
			// Periodically get metrics to exercise the exporter
			if i%100 == 0 {
				_, _ = env.GetMetrics()
			}
		}
		// Force GC and check memory
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		// Calculate memory growth
		heapGrowth := int64(after.HeapAlloc) - int64(baseline.HeapAlloc)
		heapGrowthMB := float64(heapGrowth) / (1024 * 1024)
		// Memory growth should be reasonable for the number of requests
		// Allow up to 50MB growth for 1000 requests
		assert.Less(t, heapGrowthMB, 50.0, "Memory growth should be reasonable (grew by %.2f MB)", heapGrowthMB)
	})
	t.Run("Should properly cleanup when monitoring is shutdown", func(t *testing.T) {
		// Get baseline goroutine count
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		baselineGoroutines := runtime.NumGoroutine()
		// Create environment
		env := SetupTestEnvironment(t)
		// Make requests to ensure monitoring is active
		for i := 0; i < 10; i++ {
			resp, err := env.MakeRequest("GET", "/api/v1/health")
			require.NoError(t, err)
			resp.Body.Close()
		}
		// Check goroutines during operation
		activeGoroutines := runtime.NumGoroutine()
		assert.Greater(t, activeGoroutines, baselineGoroutines, "Should have additional goroutines when active")
		// Cleanup
		env.Cleanup()
		// Give time for cleanup
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		// Check goroutines after cleanup
		finalGoroutines := runtime.NumGoroutine()
		diff := finalGoroutines - baselineGoroutines
		assert.LessOrEqual(t, diff, 2, "Should cleanup goroutines after shutdown")
	})
	t.Run("Should handle repeated initialization and shutdown cycles", func(t *testing.T) {
		// Get baseline
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		baselineGoroutines := runtime.NumGoroutine()
		var baselineMemStats runtime.MemStats
		runtime.ReadMemStats(&baselineMemStats)
		// Perform multiple init/shutdown cycles
		cycles := 10
		for i := 0; i < cycles; i++ {
			env := SetupTestEnvironment(t)
			// Use the monitoring
			for j := 0; j < 5; j++ {
				resp, err := env.MakeRequest("GET", "/api/v1/health")
				require.NoError(t, err)
				resp.Body.Close()
			}
			_, _ = env.GetMetrics()
			// Cleanup
			env.Cleanup()
			// Small delay between cycles
			time.Sleep(50 * time.Millisecond)
		}
		// Final checks
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		finalGoroutines := runtime.NumGoroutine()
		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)
		// Check goroutines
		goroutineDiff := finalGoroutines - baselineGoroutines
		assert.LessOrEqual(t, goroutineDiff, 2, "Should not accumulate goroutines over cycles")
		// Check memory
		heapGrowth := int64(finalMemStats.HeapAlloc) - int64(baselineMemStats.HeapAlloc)
		heapGrowthMB := float64(heapGrowth) / (1024 * 1024)
		assert.Less(t, heapGrowthMB, 10.0, "Should not accumulate memory over cycles (grew by %.2f MB)", heapGrowthMB)
	})
}
