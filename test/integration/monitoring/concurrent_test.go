package monitoring_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentMetricUpdates(t *testing.T) {
	t.Run("Should handle concurrent HTTP requests without race conditions", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Number of concurrent goroutines
		numGoroutines := 50
		// Number of requests per goroutine
		requestsPerGoroutine := 10
		// Track successful requests
		var successCount int64
		var errorCount int64
		// WaitGroup for synchronization
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		// Start concurrent requests
		for i := 0; i < numGoroutines; i++ {
			go func(routineID int) {
				defer wg.Done()
				client := env.GetHTTPClient()
				for j := 0; j < requestsPerGoroutine; j++ {
					// Mix of different endpoints
					paths := []string{
						"/api/v1/health",
						fmt.Sprintf("/api/v1/users/%d", routineID*10+j),
						"/api/v1/error",
						"/",
					}
					path := paths[j%len(paths)]
					req, err := http.NewRequestWithContext(
						context.Background(),
						"GET",
						env.httpServer.URL+path,
						http.NoBody,
					)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}
		// Wait for all goroutines to complete
		wg.Wait()
		// Verify all requests completed
		totalRequests := int64(numGoroutines * requestsPerGoroutine)
		assert.Equal(t, totalRequests, successCount+errorCount)
		assert.Equal(t, int64(0), errorCount, "Should have no errors")
		// Give metrics time to be recorded
		time.Sleep(200 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Verify metrics are consistent
		assert.Contains(t, metrics, "compozy_http_requests_total")
		// Parse total request count from metrics
		// The total across all labels should equal our request count
		// Note: This is a basic check - a more thorough test would parse
		// all counter values and sum them
		assert.Contains(t, metrics, `http_route="/api/v1/health"`)
		// Check for either templated path or literal paths for users endpoint
		hasUsersRoute := strings.Contains(metrics, `http_route="/api/v1/users/:id"`) ||
			strings.Contains(metrics, `http_route="/api/v1/users/`)
		assert.True(t, hasUsersRoute, "Should contain users route (templated or literal)")
		assert.Contains(t, metrics, `http_route="/api/v1/error"`)
	})
	t.Run("Should handle concurrent metrics endpoint access", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Generate some initial metrics
		for i := 0; i < 10; i++ {
			if resp, err := env.MakeRequest("GET", "/api/v1/health"); err == nil {
				resp.Body.Close()
			}
		}
		// Number of concurrent metrics readers
		numReaders := 20
		var wg sync.WaitGroup
		wg.Add(numReaders)
		// Track successful reads
		var successCount int64
		// Start concurrent metrics reads
		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				client := env.GetMetricsClient()
				// Each reader makes multiple requests
				for j := 0; j < 5; j++ {
					req, err := http.NewRequestWithContext(context.Background(), "GET", env.metricsURL, http.NoBody)
					if err != nil {
						continue
					}
					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
					}
					resp.Body.Close() // Always close the body
					// Small delay between requests
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}
		// Wait for all readers to complete
		wg.Wait()
		// Verify all reads were successful
		expectedReads := int64(numReaders * 5)
		assert.Equal(t, expectedReads, successCount, "All metrics reads should succeed")
	})
	t.Run("Should maintain metric accuracy under concurrent load", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Use a specific endpoint for accurate counting
		testPath := "/api/v1/health"
		numRequests := 100
		// Make requests concurrently
		var wg sync.WaitGroup
		wg.Add(numRequests)
		for i := 0; i < numRequests; i++ {
			go func() {
				defer wg.Done()
				resp, err := env.MakeRequest("GET", testPath)
				if err == nil {
					resp.Body.Close()
				}
			}()
		}
		// Wait for all requests to complete
		wg.Wait()
		// Give metrics time to be recorded
		time.Sleep(200 * time.Millisecond)
		// Get metrics
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		// Parse the specific counter value
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "compozy_http_requests_total") &&
				strings.Contains(line, `http_route="/api/v1/health"`) &&
				strings.Contains(line, `http_status_code="200"`) {
				// Extract the counter value
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					var count float64
					_, err := fmt.Sscanf(parts[1], "%f", &count)
					require.NoError(t, err, "failed to parse Prometheus counter")
					assert.Equal(t, float64(numRequests), count, "Counter should match number of requests")
				}
			}
		}
	})
	t.Run("Should handle mixed read and write operations concurrently", func(t *testing.T) {
		env := SetupTestEnvironment(t)
		defer env.Cleanup()
		// Start background load
		stopChan := make(chan bool)
		go func() {
			client := env.GetHTTPClient()
			for {
				select {
				case <-stopChan:
					return
				default:
					req, _ := http.NewRequestWithContext(
						context.Background(),
						"GET",
						env.httpServer.URL+"/api/v1/health",
						http.NoBody,
					)
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
		// Concurrently read metrics while requests are being made
		var wg sync.WaitGroup
		numReaders := 10
		wg.Add(numReaders)
		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					metrics, err := env.GetMetrics()
					assert.NoError(t, err)
					assert.NotEmpty(t, metrics)
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}
		// Let it run for a bit
		time.Sleep(500 * time.Millisecond)
		// Stop background load
		close(stopChan)
		// Wait for readers to complete
		wg.Wait()
		// Final metrics check
		metrics, err := env.GetMetrics()
		require.NoError(t, err)
		assert.Contains(t, metrics, "compozy_http_requests_total")
	})
}
