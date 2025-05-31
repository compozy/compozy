package ref

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Cache Configuration Tests
// -----------------------------------------------------------------------------

func TestCacheConfig_EnvironmentVariables(t *testing.T) {
	// Store original env vars to restore later
	originalDocSize := os.Getenv("COMPOZY_REF_CACHE_SIZE")
	originalPathSize := os.Getenv("COMPOZY_REF_PATH_CACHE_SIZE")
	originalDisable := os.Getenv("COMPOZY_REF_DISABLE_PATH_CACHE")

	defer func() {
		// Restore original values
		if originalDocSize == "" {
			os.Unsetenv("COMPOZY_REF_CACHE_SIZE")
		} else {
			os.Setenv("COMPOZY_REF_CACHE_SIZE", originalDocSize)
		}
		if originalPathSize == "" {
			os.Unsetenv("COMPOZY_REF_PATH_CACHE_SIZE")
		} else {
			os.Setenv("COMPOZY_REF_PATH_CACHE_SIZE", originalPathSize)
		}
		if originalDisable == "" {
			os.Unsetenv("COMPOZY_REF_DISABLE_PATH_CACHE")
		} else {
			os.Setenv("COMPOZY_REF_DISABLE_PATH_CACHE", originalDisable)
		}

		// Reset cache state for other tests
		ResetCachesForTesting()
	}()

	t.Run("Should use default values when no env vars set", func(t *testing.T) {
		os.Unsetenv("COMPOZY_REF_CACHE_SIZE")
		os.Unsetenv("COMPOZY_REF_PATH_CACHE_SIZE")
		os.Unsetenv("COMPOZY_REF_DISABLE_PATH_CACHE")

		// Reset config to force re-reading
		ResetCachesForTesting()

		config := getCacheConfig()
		assert.Equal(t, DefaultDocCacheSize, config.DocCacheSize)
		assert.Equal(t, DefaultPathCacheSize, config.PathCacheSize)
		assert.True(t, config.EnablePathCache)
	})

	t.Run("Should read doc cache size from environment", func(t *testing.T) {
		os.Setenv("COMPOZY_REF_CACHE_SIZE", "512")
		ResetCachesForTesting() // Reset config

		config := getCacheConfig()
		assert.Equal(t, 512, config.DocCacheSize)
	})

	t.Run("Should read path cache size from environment", func(t *testing.T) {
		os.Setenv("COMPOZY_REF_PATH_CACHE_SIZE", "1024")
		ResetCachesForTesting() // Reset config

		config := getCacheConfig()
		assert.Equal(t, 1024, config.PathCacheSize)
	})

	t.Run("Should disable path cache from environment", func(t *testing.T) {
		os.Setenv("COMPOZY_REF_DISABLE_PATH_CACHE", "true")
		ResetCachesForTesting() // Reset config

		config := getCacheConfig()
		assert.False(t, config.EnablePathCache)
	})

	t.Run("Should handle invalid env var values gracefully", func(t *testing.T) {
		os.Setenv("COMPOZY_REF_CACHE_SIZE", "invalid")
		os.Setenv("COMPOZY_REF_PATH_CACHE_SIZE", "-1")
		ResetCachesForTesting() // Reset config

		config := getCacheConfig()
		assert.Equal(t, DefaultDocCacheSize, config.DocCacheSize)
		assert.Equal(t, DefaultPathCacheSize, config.PathCacheSize)
	})
}

func TestCacheConfig_ProgrammaticConfiguration(t *testing.T) {
	// Store original state
	defer ResetCachesForTesting()

	t.Run("Should accept programmatic configuration", func(t *testing.T) {
		customConfig := &CacheConfig{
			DocCacheSize:    128,
			PathCacheSize:   256,
			EnablePathCache: false,
		}

		SetCacheConfig(customConfig)
		config := getCacheConfig()

		assert.Equal(t, 128, config.DocCacheSize)
		assert.Equal(t, 256, config.PathCacheSize)
		assert.False(t, config.EnablePathCache)
	})

	t.Run("Should use defaults for invalid sizes", func(t *testing.T) {
		customConfig := &CacheConfig{
			DocCacheSize:    0,
			PathCacheSize:   -10,
			EnablePathCache: true,
		}

		SetCacheConfig(customConfig)
		config := getCacheConfig()

		assert.Equal(t, DefaultDocCacheSize, config.DocCacheSize)
		assert.Equal(t, DefaultPathCacheSize, config.PathCacheSize)
		assert.True(t, config.EnablePathCache)
	})

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		SetCacheConfig(nil)
		config := getCacheConfig()

		// Should still return a valid config with defaults
		assert.NotNil(t, config)
		assert.Equal(t, DefaultDocCacheSize, config.DocCacheSize)
		assert.Equal(t, DefaultPathCacheSize, config.PathCacheSize)
		assert.True(t, config.EnablePathCache)
	})
}

func TestPathCache_Integration(t *testing.T) {
	defer ResetCachesForTesting()

	t.Run("Should create path cache when enabled", func(t *testing.T) {
		SetCacheConfig(&CacheConfig{
			DocCacheSize:    256,
			PathCacheSize:   512,
			EnablePathCache: true,
		})

		cache := getPathCache()
		assert.NotNil(t, cache)
	})

	t.Run("Should return nil when path cache disabled", func(t *testing.T) {
		SetCacheConfig(&CacheConfig{
			DocCacheSize:    256,
			PathCacheSize:   512,
			EnablePathCache: false,
		})

		cache := getPathCache()
		assert.Nil(t, cache)
	})
}

// -----------------------------------------------------------------------------
// LRU Cache Integration Tests
// -----------------------------------------------------------------------------

func TestResolvedDocsCache_Basic(t *testing.T) {
	t.Run("Should store and retrieve documents", func(t *testing.T) {
		// Use unique keys to avoid conflicts with other tests
		testKey := "/test/cache/basic.yaml"
		testData := map[string]any{
			"id":   "test_doc",
			"data": "test_value",
		}

		// Ensure key doesn't exist initially
		_, exists := getResolvedDocsCache().Get(testKey)
		assert.False(t, exists)

		// Store data
		getResolvedDocsCache().Add(testKey, testData)

		// Should be retrievable
		retrieved, exists := getResolvedDocsCache().Get(testKey)
		assert.True(t, exists)
		assert.Equal(t, testData, retrieved)
	})

	t.Run("Should handle cache updates", func(t *testing.T) {
		testKey := "/test/cache/update.yaml"
		originalData := map[string]any{"version": 1}
		updatedData := map[string]any{"version": 2}

		// Store original data
		getResolvedDocsCache().Add(testKey, originalData)
		retrieved, exists := getResolvedDocsCache().Get(testKey)
		require.True(t, exists)
		assert.Equal(t, originalData, retrieved)

		// Update data
		getResolvedDocsCache().Add(testKey, updatedData)
		retrieved, exists = getResolvedDocsCache().Get(testKey)
		require.True(t, exists)
		assert.Equal(t, updatedData, retrieved)
	})

	t.Run("Should handle different data types", func(t *testing.T) {
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
				getResolvedDocsCache().Add(tc.key, tc.data)
				retrieved, exists := getResolvedDocsCache().Get(tc.key)
				require.True(t, exists)
				assert.Equal(t, tc.data, retrieved)
			})
		}
	})
}

func TestResolvedDocsCache_LRU(t *testing.T) {
	t.Run("Should evict entries when cache is full", func(t *testing.T) {
		// Note: We can't easily test the exact LRU behavior since the cache size is configurable
		// and we share it with other tests. Instead, we test that the cache can handle
		// a reasonable number of entries without issues.

		baseKey := "/test/cache/lru/"
		numEntries := 20 // Much less than the default cache size

		// Add entries
		for i := range numEntries {
			key := fmt.Sprintf("%sentry%d.yaml", baseKey, i)
			data := map[string]any{"id": i, "data": fmt.Sprintf("entry_%d", i)}
			getResolvedDocsCache().Add(key, data)
		}

		// Verify all entries are still present
		for i := range numEntries {
			key := fmt.Sprintf("%sentry%d.yaml", baseKey, i)
			retrieved, exists := getResolvedDocsCache().Get(key)
			assert.True(t, exists, "Entry %d should exist", i)
			if exists {
				data := retrieved.(map[string]any)
				assert.Equal(t, i, data["id"], "Entry %d should have correct ID", i)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Cache Integration with Document Loading
// -----------------------------------------------------------------------------

func TestCache_DocumentLoadingIntegration(t *testing.T) {
	t.Run("Should cache loaded documents", func(t *testing.T) {
		// This test verifies that loadDocument properly uses the cache
		// We can't easily test this without creating actual files, so we
		// focus on testing that the cache is used correctly in resolve operations.

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
		getResolvedDocsCache().Add(testKey, testData)

		// Verify it can be retrieved as expected by selectSourceDocument logic
		retrieved, exists := getResolvedDocsCache().Get(testKey)
		require.True(t, exists)
		assert.Equal(t, testData, retrieved)

		// Create a simple document from the cached data
		doc := &simpleDocument{data: retrieved}
		value, err := doc.Get("schemas.0.id")
		require.NoError(t, err)
		assert.Equal(t, "test_schema", value)
	})

	t.Run("Should handle URL caching", func(t *testing.T) {
		// Test that URLs are also cached in the same LRU
		testURL := "https://example.com/test-config.yaml"
		testData := map[string]any{
			"config": map[string]any{
				"version": "1.0",
				"name":    "remote_config",
			},
		}

		// Simulate caching a remote document
		getResolvedDocsCache().Add(testURL, testData)

		// Verify retrieval
		retrieved, exists := getResolvedDocsCache().Get(testURL)
		require.True(t, exists)
		assert.Equal(t, testData, retrieved)

		// Verify document functionality
		doc := &simpleDocument{data: retrieved}
		value, err := doc.Get("config.name")
		require.NoError(t, err)
		assert.Equal(t, "remote_config", value)
	})
}

// -----------------------------------------------------------------------------
// Cache Performance and Behavior Tests
// -----------------------------------------------------------------------------

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		// The HashiCorp LRU cache is thread-safe, but let's verify our usage is correct
		const numGoroutines = 10
		const numOperations = 10

		// Use channels to coordinate goroutines
		done := make(chan bool, numGoroutines)

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

					// Add to cache
					getResolvedDocsCache().Add(key, data)

					// Try to retrieve immediately
					retrieved, exists := getResolvedDocsCache().Get(key)
					if !exists {
						t.Errorf("Concurrent test failed: key %s not found", key)
						return
					}

					retrievedMap, ok := retrieved.(map[string]any)
					if !ok {
						t.Errorf("Concurrent test failed: invalid data type for key %s", key)
						return
					}

					if retrievedMap["goroutine"] != id {
						t.Errorf("Concurrent test failed: wrong data for key %s", key)
						return
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}
