package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func init() {
	logger.InitForTests()
}

// TestHTTPMetrics_SuccessfulRequest tests that metrics are properly recorded for successful requests
func TestHTTPMetrics_SuccessfulRequest(t *testing.T) {
	t.Run("Should record metrics for successful GET request", func(t *testing.T) {
		// Reset metrics for clean test state
		ResetMetricsForTesting()

		// Create an in-memory metric reader
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		// Create a test router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/users/:id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
		})

		// Make a test request
		req := httptest.NewRequest("GET", "/users/123", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		// Verify metrics were recorded
		assert.NotEmpty(t, rm.ScopeMetrics)

		// Check that we have the expected metrics
		foundTotal := false
		foundDuration := false
		foundInFlight := false

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch m.Name {
				case "compozy_http_requests_total":
					foundTotal = true
					// Verify labels
					if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
						assert.Len(t, sum.DataPoints, 1)
						dp := sum.DataPoints[0]
						attrs := dp.Attributes.ToSlice()
						assert.Contains(t, attrs, attribute.String("method", "GET"))
						assert.Contains(t, attrs, attribute.String("path", "/users/:id"))
						assert.Contains(t, attrs, attribute.String("status_code", "200"))
						assert.Equal(t, int64(1), dp.Value)
					}
				case "compozy_http_request_duration_seconds":
					foundDuration = true
					// Verify histogram was recorded
					if hist, ok := m.Data.(metricdata.Histogram[float64]); ok {
						assert.Len(t, hist.DataPoints, 1)
						dp := hist.DataPoints[0]
						attrs := dp.Attributes.ToSlice()
						assert.Contains(t, attrs, attribute.String("method", "GET"))
						assert.Contains(t, attrs, attribute.String("path", "/users/:id"))
						assert.Contains(t, attrs, attribute.String("status_code", "200"))
						assert.Greater(t, dp.Sum, float64(0))
					}
				case "compozy_http_requests_in_flight":
					foundInFlight = true
				}
			}
		}

		assert.True(t, foundTotal, "http_requests_total metric not found")
		assert.True(t, foundDuration, "http_request_duration_seconds metric not found")
		assert.True(t, foundInFlight, "http_requests_in_flight metric not found")
	})

	t.Run("Should handle POST request with different status code", func(t *testing.T) {
		ResetMetricsForTesting()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.POST("/users", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"created": true})
		})

		req := httptest.NewRequest("POST", "/users", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		// Verify metrics include POST method and 201 status
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_http_requests_total" {
					if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
						dp := sum.DataPoints[0]
						attrs := dp.Attributes.ToSlice()
						assert.Contains(t, attrs, attribute.String("method", "POST"))
						assert.Contains(t, attrs, attribute.String("path", "/users"))
						assert.Contains(t, attrs, attribute.String("status_code", "201"))
					}
				}
			}
		}
	})
}

// TestHTTPMetrics_HighCardinalityPrevention tests that 404s don't create high cardinality
func TestHTTPMetrics_HighCardinalityPrevention(t *testing.T) {
	t.Run("Should use 'unmatched' path for 404 requests", func(t *testing.T) {
		ResetMetricsForTesting()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/users/:id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
		})

		// Make requests to non-existent paths
		paths := []string{"/unknown/path", "/another/missing/route", "/404/test"}
		for _, path := range paths {
			req := httptest.NewRequest("GET", path, http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)
		}

		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		// Verify all 404s use "unmatched" path
		unmatchedFound := false
		var totalValue int64
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_http_requests_total" {
					if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
						for _, dp := range sum.DataPoints {
							attrs := dp.Attributes.ToSlice()
							var pathValue string
							for _, attr := range attrs {
								if attr.Key == "path" {
									pathValue = attr.Value.AsString()
								}
							}
							if pathValue == "unmatched" {
								unmatchedFound = true
								totalValue = dp.Value
							}
						}
					}
				}
			}
		}
		assert.True(t, unmatchedFound, "Should find 'unmatched' path in metrics")
		assert.Equal(t, int64(3), totalValue, "All 404 requests should be grouped under 'unmatched' path")
	})

	t.Run("Should use route template for dynamic paths", func(t *testing.T) {
		ResetMetricsForTesting()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/api/v1/users/:id/posts/:postId", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"userId": c.Param("id"),
				"postId": c.Param("postId"),
			})
		})

		// Make requests with different IDs
		ids := [][]string{{"123", "456"}, {"789", "012"}, {"abc", "def"}}
		for _, idPair := range ids {
			req := httptest.NewRequest("GET", "/api/v1/users/"+idPair[0]+"/posts/"+idPair[1], http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		// Verify all requests use the template path
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_http_requests_total" {
					if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
						assert.Len(t, sum.DataPoints, 1, "All requests should be grouped under one template path")
						dp := sum.DataPoints[0]
						attrs := dp.Attributes.ToSlice()
						assert.Contains(t, attrs, attribute.String("path", "/api/v1/users/:id/posts/:postId"))
						assert.Equal(t, int64(3), dp.Value)
					}
				}
			}
		}
	})
}

// TestHTTPMetrics_ErrorHandling tests error handling and recovery
func TestHTTPMetrics_ErrorHandling(t *testing.T) {
	t.Run("Should recover from panic in middleware", func(t *testing.T) {
		ResetMetricsForTesting()
		// Use a no-op meter that will cause nil pointer access
		meter := noop.NewMeterProvider().Meter("test")

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		w := httptest.NewRecorder()

		// Should not panic
		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		// Request should complete successfully
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		ResetMetricsForTesting()
		router := gin.New()
		router.Use(HTTPMetrics(nil))
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestHTTPMetrics_InFlightRequests tests in-flight request tracking
func TestHTTPMetrics_InFlightRequests(t *testing.T) {
	t.Run("Should track concurrent in-flight requests", func(t *testing.T) {
		ResetMetricsForTesting()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		const numRequests = 3
		startChan := make(chan struct{}, numRequests)
		unblockChan := make(chan struct{})

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/slow", func(c *gin.Context) {
			startChan <- struct{}{} // Signal that handler has started
			<-unblockChan           // Block until signaled
			c.String(http.StatusOK, "done")
		})

		// Start multiple concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/slow", http.NoBody)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}()
		}

		// Wait for all requests to be in-flight
		for i := 0; i < numRequests; i++ {
			<-startChan
		}

		// Check in-flight metric while requests are blocked
		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		// Assert peak in-flight value
		inFlightValue := getInFlightValue(t, &rm)
		assert.Equal(t, int64(numRequests), inFlightValue, "in-flight should be at its peak")

		// Unblock requests
		close(unblockChan)
		wg.Wait()

		// Check that in-flight returns to 0
		err = reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)
		inFlightValue = getInFlightValue(t, &rm)
		assert.Equal(t, int64(0), inFlightValue, "in-flight should return to 0")
	})
}

// getInFlightValue extracts the in-flight metric value from ResourceMetrics
func getInFlightValue(_ *testing.T, rm *metricdata.ResourceMetrics) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "compozy_http_requests_in_flight" {
				if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
					if len(sum.DataPoints) > 0 {
						return sum.DataPoints[0].Value
					}
				}
			}
		}
	}
	return 0
}

// TestHTTPMetrics_HistogramBuckets tests that histogram uses correct bucket boundaries
func TestHTTPMetrics_HistogramBuckets(t *testing.T) {
	t.Run("Should use specified bucket boundaries", func(t *testing.T) {
		ResetMetricsForTesting()
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := provider.Meter("test")

		router := gin.New()
		router.Use(HTTPMetrics(meter))
		router.GET("/test", func(c *gin.Context) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var rm metricdata.ResourceMetrics
		err := reader.Collect(context.Background(), &rm)
		assert.NoError(t, err)

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "compozy_http_request_duration_seconds" {
					if hist, ok := m.Data.(metricdata.Histogram[float64]); ok {
						dp := hist.DataPoints[0]
						// Verify bucket boundaries match specification
						expectedBounds := []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
						assert.Equal(t, expectedBounds, dp.Bounds)
					}
				}
			}
		}
	})
}
