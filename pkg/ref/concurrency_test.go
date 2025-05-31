package ref

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Parallel Cycle Detection Tests
// -----------------------------------------------------------------------------

func TestParallelCycleDetection(t *testing.T) {
	// The parallel threshold is now hardcoded in the implementation to 4 elements
	// and runtime.NumCPU() > 1

	t.Run("Should detect cycles across parallel branches", func(t *testing.T) {
		// Create a map with two keys that reference each other
		// This should trigger parallel resolution where each key is resolved in a separate goroutine
		data := map[string]any{
			"a": map[string]any{"$ref": "b"},
			"b": map[string]any{"$ref": "a"},
			"c": map[string]any{"$ref": "d"},
			"d": map[string]any{"$ref": "c"},
			// Add more entries to ensure we hit the parallel threshold
			"e": "value1",
			"f": "value2",
			"g": "value3",
			"h": "value4",
		}

		// Force parallel processing by making sure we have enough entries
		// and multiple CPU cores
		if runtime.NumCPU() < 2 {
			t.Skip("Skipping parallel test on single-core system")
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular reference detected")
	})

	t.Run("Should detect sibling-to-sibling cycles", func(t *testing.T) {
		testCases := []struct {
			name      string
			data      map[string]any
			shouldErr bool
		}{
			{
				name: "simple_sibling_cycle",
				data: map[string]any{
					"x": map[string]any{"$ref": "y"},
					"y": map[string]any{"$ref": "x"},
					// Add enough entries to trigger parallel processing
					"a": "value1",
					"b": "value2",
					"c": "value3",
					"d": "value4",
				},
				shouldErr: true,
			},
			{
				name: "self_reference",
				data: map[string]any{
					"self": map[string]any{"$ref": "self"},
					// Add enough entries to trigger parallel processing
					"a": "value1",
					"b": "value2",
					"c": "value3",
					"d": "value4",
				},
				shouldErr: true,
			},
			{
				name: "no_cycle",
				data: map[string]any{
					"x":      map[string]any{"$ref": "target"},
					"y":      map[string]any{"$ref": "target"},
					"target": "shared_value",
					// Add enough entries to trigger parallel processing
					"a": "value1",
					"b": "value2",
					"c": "value3",
					"d": "value4",
				},
				shouldErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Skip on single-core systems where parallel processing won't be enabled
				if runtime.NumCPU() < 2 {
					t.Skip("Skipping parallel test on single-core system")
				}

				ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
				ctx := context.Background()

				_, err := ref.Resolve(ctx, tc.data, "/test/file.yaml", "/test")
				if tc.shouldErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "circular reference detected")
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should handle concurrent access to VisitedRefs safely", func(t *testing.T) {
		// Create a large map to force parallel processing
		data := make(map[string]any)
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("key%d", i)
			// Mix of regular values and references to create concurrent access patterns
			if i%3 == 0 {
				data[key] = map[string]any{
					"$ref":  "nested.value",
					"extra": fmt.Sprintf("data%d", i),
				}
			} else {
				data[key] = fmt.Sprintf("value%d", i)
			}
		}

		// Add the nested structure being referenced
		data["nested"] = map[string]any{
			"value": "resolved_value",
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		// Run multiple goroutines concurrently to test thread safety
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Verify no errors occurred
		for err := range errors {
			t.Errorf("Concurrent resolution failed: %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// Parallel Processing Tests
// -----------------------------------------------------------------------------

func TestParallelProcessing(t *testing.T) {
	t.Parallel()

	// Skip on single-core systems where parallel processing won't be enabled
	if runtime.NumCPU() < 2 {
		t.Skip("Skipping parallel test on single-core system")
	}

	t.Run("Should process large maps in parallel", func(t *testing.T) {
		// Create a large enough map to trigger parallel processing
		data := make(map[string]any)
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("item%d", i)
			data[key] = map[string]any{
				"id":    i,
				"value": fmt.Sprintf("item_%d_value", i),
			}
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		start := time.Now()
		result, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		duration := time.Since(start)

		require.NoError(t, err)
		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Len(t, resultMap, 20)

		// Verify all items were processed correctly
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("item%d", i)
			item, exists := resultMap[key]
			require.True(t, exists, "item %s should exist", key)

			itemMap, ok := item.(map[string]any)
			require.True(t, ok, "item %s should be a map", key)

			assert.Equal(t, float64(i), itemMap["id"])
			assert.Equal(t, fmt.Sprintf("item_%d_value", i), itemMap["value"])
		}

		t.Logf("Parallel processing took %v", duration)
	})

	t.Run("Should process large arrays in parallel", func(t *testing.T) {
		// Create a large array to trigger parallel processing
		data := make([]any, 20)
		for i := 0; i < 20; i++ {
			data[i] = map[string]any{
				"index": i,
				"data":  fmt.Sprintf("array_item_%d", i),
			}
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		result, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		require.NoError(t, err)

		resultArray, ok := result.([]any)
		require.True(t, ok)
		assert.Len(t, resultArray, 20)

		// Verify all items were processed correctly and in order
		for i := 0; i < 20; i++ {
			item, ok := resultArray[i].(map[string]any)
			require.True(t, ok, "array item %d should be a map", i)
			assert.Equal(t, float64(i), item["index"])
			assert.Equal(t, fmt.Sprintf("array_item_%d", i), item["data"])
		}
	})
}

// -----------------------------------------------------------------------------
// Race Condition Tests
// -----------------------------------------------------------------------------

func TestRaceConditions(t *testing.T) {
	t.Parallel()

	t.Run("Should handle concurrent cache access", func(t *testing.T) {
		// Test concurrent access to the Ristretto cache
		const numGoroutines = 50
		const filePath = "/test/concurrent.yaml"

		testData := map[string]any{"test": "value"}

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Try to set and get from cache concurrently
				GetGlobalCache().Set(filePath, testData, 1)
				GetGlobalCache().Wait() // Ensure the set is processed
				retrieved, exists := GetGlobalCache().Get(filePath)

				if !exists {
					errors <- fmt.Errorf("goroutine %d: cache entry not found", id)
					return
				}

				retrievedMap, ok := retrieved.(map[string]any)
				if !ok {
					errors <- fmt.Errorf("goroutine %d: invalid data type", id)
					return
				}

				value, exists := retrievedMap["test"]
				if !exists {
					errors <- fmt.Errorf("goroutine %d: test key not found", id)
					return
				}

				if value != "value" {
					errors <- fmt.Errorf("goroutine %d: expected 'value', got %v", id, value)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Race condition error: %v", err)
			errorCount++
		}

		if errorCount > 0 {
			t.Fatalf("Found %d race condition errors", errorCount)
		}
	})

	t.Run("Should handle concurrent reference resolution", func(t *testing.T) {
		data := map[string]any{
			"shared_ref": map[string]any{
				"$ref": "target.value",
			},
			"target": map[string]any{
				"value": "shared_result",
			},
		}

		const numGoroutines = 20
		var wg sync.WaitGroup
		results := make(chan any, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				ref := &Ref{Type: TypeProperty, Path: "shared_ref", Mode: ModeMerge}
				ctx := context.Background()

				result, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
				if err != nil {
					errors <- err
					return
				}

				results <- result
			}()
		}

		wg.Wait()
		close(results)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent resolution error: %v", err)
		}

		// Verify all results are consistent
		var firstResult any
		resultCount := 0
		for result := range results {
			if firstResult == nil {
				firstResult = result
			} else {
				assert.Equal(t, firstResult, result, "All concurrent resolutions should return the same result")
			}
			resultCount++
		}

		assert.Equal(t, numGoroutines, resultCount, "Should receive results from all goroutines")
	})
}

// -----------------------------------------------------------------------------
// Performance Tests
// -----------------------------------------------------------------------------

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("Should scale well with parallel processing", func(t *testing.T) {
		// Create increasingly large datasets and measure performance
		sizes := []int{10, 50, 100, 200}

		for _, size := range sizes {
			t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
				data := make(map[string]any)
				for i := 0; i < size; i++ {
					key := fmt.Sprintf("item%d", i)
					data[key] = map[string]any{
						"nested": map[string]any{
							"value": fmt.Sprintf("nested_value_%d", i),
						},
					}
				}

				ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
				ctx := context.Background()

				start := time.Now()
				result, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
				duration := time.Since(start)

				require.NoError(t, err)
				resultMap, ok := result.(map[string]any)
				require.True(t, ok)
				assert.Len(t, resultMap, size)

				t.Logf("Size %d took %v (%.2f ms per item)",
					size, duration, float64(duration.Nanoseconds())/float64(size)/1e6)
			})
		}
	})
}

// -----------------------------------------------------------------------------
// Regression Tests for False Cycle Detection
// -----------------------------------------------------------------------------

func TestParallelDuplicateReference(t *testing.T) {
	t.Run("Should not detect false cycles with parallel duplicate references", func(t *testing.T) {
		// This test would fail with the old shared VisitedRefs approach
		// but should pass with the new per-goroutine cycle detection
		data := map[string]any{
			"a": map[string]any{"$ref": "shared.value"},
			"b": map[string]any{"$ref": "shared.value"}, // Same reference as 'a'
			"c": map[string]any{"$ref": "shared.value"}, // Same reference as 'a' and 'b'
			"d": map[string]any{"$ref": "shared.value"}, // Same reference as others
			// Add more entries to ensure parallel processing
			"e": "value1",
			"f": "value2",
			"g": "value3",
			"h": "value4",
		}

		// Add the shared target
		data["shared"] = map[string]any{
			"value": "resolved_shared_value",
		}

		// Skip on single-core systems where parallel processing won't be enabled
		if runtime.NumCPU() < 2 {
			t.Skip("Skipping parallel test on single-core system")
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		result, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		require.NoError(t, err, "duplicate refs in parallel must not look like cycles")

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)

		// Verify all references were resolved correctly
		for _, key := range []string{"a", "b", "c", "d"} {
			value, exists := resultMap[key]
			require.True(t, exists, "key %s should exist", key)
			assert.Equal(t, "resolved_shared_value", value, "key %s should resolve to shared value", key)
		}

		// Verify the shared target still exists
		shared, exists := resultMap["shared"]
		require.True(t, exists)
		sharedMap, ok := shared.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "resolved_shared_value", sharedMap["value"])
	})

	t.Run("Should still detect real circular references", func(t *testing.T) {
		// Make sure we didn't break real cycle detection
		data := map[string]any{
			"cycle_a": map[string]any{"$ref": "cycle_b.value"},
			"cycle_b": map[string]any{"value": map[string]any{"$ref": "cycle_a"}},
			// Add more entries to trigger parallel processing
			"other1": "value1",
			"other2": "value2",
			"other3": "value3",
			"other4": "value4",
		}

		ref := &Ref{Type: TypeProperty, Path: "", Mode: ModeMerge}
		ctx := context.Background()

		_, err := ref.Resolve(ctx, data, "/test/file.yaml", "/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular reference detected")
	})
}
