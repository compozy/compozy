package ref

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Ristretto Cache Integration Tests
// -----------------------------------------------------------------------------

func TestGlobalRistrettoCache_Basic(t *testing.T) {
	defer ResetRistrettoCacheForTesting() // Ensure cache is clean after this test suite

	t.Run("Should store and retrieve documents", func(t *testing.T) {
		ResetRistrettoCacheForTesting() // Clean before each sub-test
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		testKey := "/test/cache/basic.yaml"
		testData := map[string]any{
			"id":   "test_doc",
			"data": "test_value",
		}

		// Ensure key doesn't exist initially
		_, exists := cache.Get(testKey)
		assert.False(t, exists)

		// Store data (cost 1, no TTL for simplicity in basic test)
		cache.Set(testKey, testData, 1)
		cache.Wait() // Ensure value is processed

		// Should be retrievable
		retrieved, exists := cache.Get(testKey)
		assert.True(t, exists)
		assert.Equal(t, testData, retrieved)
	})

	t.Run("Should handle cache updates", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		testKey := "/test/cache/update.yaml"
		originalData := map[string]any{"version": 1}
		updatedData := map[string]any{"version": 2}

		// Store original data
		cache.Set(testKey, originalData, 1)
		cache.Wait()
		retrieved, exists := cache.Get(testKey)
		require.True(t, exists)
		assert.Equal(t, originalData, retrieved)

		// Update data
		cache.Set(testKey, updatedData, 1)
		cache.Wait()
		retrieved, exists = cache.Get(testKey)
		require.True(t, exists)
		assert.Equal(t, updatedData, retrieved)
	})

	t.Run("Should handle different data types", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		testCases := []struct {
			name string
			key  string
			data any
		}{
			{"map", "/test/cache/map.yaml", map[string]any{"key": "value"}},
			{"array", "/test/cache/array.yaml", []any{"item1", "item2"}},
			{"string", "/test/cache/string.yaml", "simple string"},
			{"number", "/test/cache/number.yaml", 42},
			{"boolean", "/test/cache/bool.yaml", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cache.Set(tc.key, tc.data, 1)
				cache.Wait()
				retrieved, exists := cache.Get(tc.key)
				require.True(t, exists)
				assert.Equal(t, tc.data, retrieved)
			})
		}
	})
}

func TestGlobalRistrettoCache_Eviction(t *testing.T) {
	defer ResetRistrettoCacheForTesting()

	t.Run("Should evict entries when cache approaches max cost", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		// Ristretto's MaxCost is 1GB. We'll add items with a cost that will eventually trigger eviction.
		// This test is a bit conceptual as exact eviction is hard to pin down without knowing Ristretto's internals deeply.
		// We'll add more items than NumCounters to ensure some level of contention.
		// Each item will have a cost of 1MB. MaxCost is 1GB, so ~1024 items. NumCounters is 10M.
		// We will add a bit more than what seems reasonable for a small test to see if it handles it.

		baseKey := "/test/cache/eviction/"
		// MaxCost is 1 << 30 (1GB). Let's assume a cost of 1 for each small entry for this test.
		// The cache is configured with NumCounters: 1e7, MaxCost: 1 << 30.
		// We will add a large number of small items.
		numEntries := 1000 // Add 1000 items

		// Add entries
		for i := 0; i < numEntries; i++ {
			key := fmt.Sprintf("%sentry%d.yaml", baseKey, i)
			data := map[string]any{"id": i, "data": fmt.Sprintf("entry_%d_long_data_to_ensure_some_cost_if_ristretto_calculates_it_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", i)}
			// Using cost of 1 for each item.
			cache.Set(key, data, 1)
		}
		cache.Wait() // Wait for all sets to be processed

		// Check if some items were potentially evicted.
		// This is not a perfect test for eviction as Ristretto's eviction is probabilistic (TinyLFU).
		// We are checking that the cache doesn't error out and still holds *some* data.
		// A more robust test would require a cache with much smaller MaxCost to reliably see evictions.
		// For now, we verify that at least the last item is likely there (due to frequency if not recency).
		lastKey := fmt.Sprintf("%sentry%d.yaml", baseKey, numEntries-1)
		_, exists := cache.Get(lastKey)
		assert.True(t, exists, "Last entry should ideally exist after many additions")

		// Check that the cache is not empty
		var foundCount int
		for i := 0; i < numEntries; i++ {
			key := fmt.Sprintf("%sentry%d.yaml", baseKey, i)
			if _, ok := cache.Get(key); ok {
				foundCount++
			}
		}
		// It's hard to assert an exact number of items after eviction without specific cache sizing for the test.
		// We expect some items to be present, and likely not all if MaxCost was truly hit by these items.
		// Given MaxCost 1GB and 1000 items with cost 1, no eviction should occur based on cost alone.
		// This test mainly ensures the cache operates under a load of many small items.
		assert.Greater(t, foundCount, numEntries/2, "Expected a significant portion of items to remain in cache")
		assert.Equal(t, numEntries, foundCount, "With cost 1 and 1GB max cost, all 1000 items should remain")

	})
}

// -----------------------------------------------------------------------------
// Cache Integration with Document Loading (Conceptual)
// -----------------------------------------------------------------------------

func TestCache_DocumentLoadingIntegration(t *testing.T) {
	defer ResetRistrettoCacheForTesting()

	t.Run("Should cache loaded documents (simulated)", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		testKey := "/test/cache/integration.yaml"
		testData := map[string]any{
			"schemas": []any{
				map[string]any{
					"id":   "test_schema",
					"type": "object",
				},
			},
		}

		// Manually add to cache to simulate a loaded document
		// Cost 1
		cache.Set(testKey, testData, 1)
		cache.Wait()

		// Verify it can be retrieved as expected by selectSourceDocument logic
		retrieved, exists := cache.Get(testKey)
		require.True(t, exists)
		assert.Equal(t, testData, retrieved)

		// Create a simple document from the cached data
		doc := &simpleDocument{data: retrieved}
		value, err := doc.Get("schemas.0.id")
		require.NoError(t, err)
		assert.Equal(t, "test_schema", value)
	})

	t.Run("Should handle URL caching (simulated)", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		// Test that URLs are also cached
		testURL := "https://example.com/test-config.yaml"
		testDataURL := map[string]any{
			"config": map[string]any{
				"version": "1.0",
				"name":    "remote_config",
			},
		}

		// Simulate caching a remote document
		cache.Set(testURL, testDataURL, 1)
		cache.Wait()

		// Verify retrieval
		retrieved, exists := cache.Get(testURL)
		require.True(t, exists)
		assert.Equal(t, testDataURL, retrieved)

		// Verify document functionality
		doc := &simpleDocument{data: retrieved}
		value, err := doc.Get("config.name")
		require.NoError(t, err)
		assert.Equal(t, "remote_config", value)
	})
}

// -----------------------------------------------------------------------------
// Cache Performance and Behavior Tests (Ristretto is inherently concurrent)
// -----------------------------------------------------------------------------

func TestRistrettoCache_ConcurrentAccess(t *testing.T) {
	defer ResetRistrettoCacheForTesting()

	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		ResetRistrettoCacheForTesting()
		cache := GetGlobalCache()
		require.NotNil(t, cache)

		// Ristretto is designed for concurrent access.
		const numGoroutines = 20
		const numOperations = 200 // Increased operations

		// Use channels to coordinate goroutines
		done := make(chan bool, numGoroutines)
		errCh := make(chan error, numGoroutines*numOperations) // Channel for errors

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("/test/cache/concurrent/g%d_op%d.yaml", id, j)
					data := map[string]any{
						"goroutine": id,
						"operation": j,
						"timestamp": fmt.Sprintf("g%d_op%d", id, j),
					}

					// Add to cache (cost 1)
					cache.Set(key, data, 1)
					// Ristretto's Set is async, but for testing retrieval, a Wait might be needed
					// if we want to guarantee the item is findable immediately by *another* goroutine.
					// However, within the same goroutine, Get should see its own Set after Wait.

					// Try to retrieve immediately (or after a short wait for processing)
					cache.Wait() // Ensure set is processed before Get
					retrieved, exists := cache.Get(key)
					if !exists {
						errCh <- fmt.Errorf("Concurrent test failed: key %s not found for goroutine %d", key, id)
						return
					}

					retrievedMap, ok := retrieved.(map[string]any)
					if !ok {
						errCh <- fmt.Errorf("Concurrent test failed: invalid data type for key %s goroutine %d", key, id)
						return
					}

					if val, ok := retrievedMap["goroutine"].(int); !ok || val != id {
						errCh <- fmt.Errorf("Concurrent test failed: wrong data for key %s goroutine %d, got %v", key, id, retrievedMap["goroutine"])
						return
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		close(errCh)

		// Check for errors
		for err := range errCh {
			t.Error(err)
		}
	})
}
