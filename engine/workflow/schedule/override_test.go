package schedule

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverrideCache_NewOverrideCache(t *testing.T) {
	t.Run("Should create new cache with empty overrides", func(t *testing.T) {
		cache := NewOverrideCache()

		assert.NotNil(t, cache)
		assert.NotNil(t, cache.overrides)
		assert.Equal(t, 0, len(cache.overrides))
	})
}

func TestOverrideCache_SetAndGetOverride(t *testing.T) {
	t.Run("Should store and retrieve override", func(t *testing.T) {
		cache := NewOverrideCache()
		workflowID := "test-workflow"
		values := map[string]any{"enabled": true}

		cache.SetOverride(workflowID, values)

		override, exists := cache.GetOverride(workflowID)
		require.True(t, exists)
		assert.Equal(t, workflowID, override.WorkflowID)
		assert.Equal(t, values, override.Values)
		assert.True(t, override.ModifiedAt.Before(time.Now().Add(time.Second)))
	})

	t.Run("Should return false for non-existent override", func(t *testing.T) {
		cache := NewOverrideCache()

		override, exists := cache.GetOverride("non-existent")

		assert.False(t, exists)
		assert.Nil(t, override)
	})

	t.Run("Should return copy to prevent concurrent modification", func(t *testing.T) {
		cache := NewOverrideCache()
		workflowID := "test-workflow"
		values := map[string]any{"enabled": true}

		cache.SetOverride(workflowID, values)

		override1, _ := cache.GetOverride(workflowID)
		override2, _ := cache.GetOverride(workflowID)

		// Modify one copy
		override1.Values["enabled"] = false

		// Other copy should be unchanged
		assert.True(t, override2.Values["enabled"].(bool))
	})
}

func TestOverrideCache_ShouldSkipReconciliation(t *testing.T) {
	t.Run("Should return false when no override exists", func(t *testing.T) {
		cache := NewOverrideCache()

		shouldSkip := cache.ShouldSkipReconciliation("test-workflow", time.Now())

		assert.False(t, shouldSkip)
	})

	t.Run("Should return true when override is newer than YAML", func(t *testing.T) {
		cache := NewOverrideCache()
		yamlModTime := time.Now().Add(-time.Hour)
		workflowID := "test-workflow"

		cache.SetOverride(workflowID, map[string]any{"enabled": false})

		shouldSkip := cache.ShouldSkipReconciliation(workflowID, yamlModTime)

		assert.True(t, shouldSkip)
	})

	t.Run("Should return false when YAML is newer than override", func(t *testing.T) {
		cache := NewOverrideCache()
		workflowID := "test-workflow"

		// Create override at a specific time
		cache.SetOverride(workflowID, map[string]any{"enabled": false})
		override, _ := cache.GetOverride(workflowID)

		// YAML modification is 1 hour later (newer than override)
		yamlModTime := override.ModifiedAt.Add(1 * time.Hour)
		shouldSkip := cache.ShouldSkipReconciliation(workflowID, yamlModTime)

		assert.False(t, shouldSkip)
	})
}

func TestOverrideCache_ClearOverride(t *testing.T) {
	t.Run("Should clear existing override", func(t *testing.T) {
		cache := NewOverrideCache()
		workflowID := "test-workflow"

		cache.SetOverride(workflowID, map[string]any{"enabled": false})

		cleared := cache.ClearOverride(workflowID)

		assert.True(t, cleared)
		_, exists := cache.GetOverride(workflowID)
		assert.False(t, exists)
	})

	t.Run("Should return false for non-existent override", func(t *testing.T) {
		cache := NewOverrideCache()

		cleared := cache.ClearOverride("non-existent")

		assert.False(t, cleared)
	})
}

func TestOverrideCache_ListOverrides(t *testing.T) {
	t.Run("Should return all overrides", func(t *testing.T) {
		cache := NewOverrideCache()

		cache.SetOverride("workflow1", map[string]any{"enabled": true})
		cache.SetOverride("workflow2", map[string]any{"enabled": false})

		overrides := cache.ListOverrides()

		assert.Equal(t, 2, len(overrides))
		assert.Contains(t, overrides, "workflow1")
		assert.Contains(t, overrides, "workflow2")
	})

	t.Run("Should return copies to prevent modification", func(t *testing.T) {
		cache := NewOverrideCache()
		cache.SetOverride("workflow1", map[string]any{"enabled": true})

		overrides := cache.ListOverrides()

		// Modify the returned map
		overrides["workflow1"].Values["enabled"] = false

		// Original should be unchanged
		original, _ := cache.GetOverride("workflow1")
		assert.True(t, original.Values["enabled"].(bool))
	})
}

func TestOverrideCache_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent operations safely", func(t *testing.T) {
		cache := NewOverrideCache()
		var wg sync.WaitGroup
		numGoroutines := 10
		numOperations := 100

		// Concurrent writes
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					workflowID := fmt.Sprintf("workflow-%d-%d", id, j)
					cache.SetOverride(workflowID, map[string]any{"enabled": true})
				}
			}(i)
		}

		// Concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					workflowID := fmt.Sprintf("workflow-%d-%d", id, j)
					cache.GetOverride(workflowID)
					cache.ShouldSkipReconciliation(workflowID, time.Now())
				}
			}(i)
		}

		wg.Wait()

		// Should not panic and should have some overrides
		overrides := cache.ListOverrides()
		assert.True(t, len(overrides) > 0)
	})
}

func TestCopyValues(t *testing.T) {
	t.Run("Should create shallow copy for primitive types", func(t *testing.T) {
		original := map[string]any{
			"enabled": true,
			"count":   42,
			"name":    "test",
		}

		copied := copyValues(original)

		assert.Equal(t, original, copied)

		// Modify copied
		copied["enabled"] = false
		copied["count"] = 100

		// Original should be unchanged
		assert.True(t, original["enabled"].(bool))
		assert.Equal(t, 42, original["count"].(int))
	})

	t.Run("Should deep copy string slices", func(t *testing.T) {
		original := map[string]any{
			"tags": []string{"tag1", "tag2", "tag3"},
		}

		result := copyValues(original)

		// Verify the slice was copied
		assert.Equal(t, original, result)

		// Modify the original slice
		originalTags := original["tags"].([]string)
		originalTags[0] = "modified"

		// Result should not be affected
		resultTags := result["tags"].([]string)
		assert.Equal(t, "tag1", resultTags[0])
	})

	t.Run("Should deep copy nested maps", func(t *testing.T) {
		original := map[string]any{
			"config": map[string]any{
				"enabled": true,
				"timeout": 30,
			},
		}

		result := copyValues(original)

		// Verify the map was copied
		assert.Equal(t, original, result)

		// Modify the original nested map
		originalConfig := original["config"].(map[string]any)
		originalConfig["enabled"] = false

		// Result should not be affected
		resultConfig := result["config"].(map[string]any)
		assert.True(t, resultConfig["enabled"].(bool))
	})

	t.Run("Should deep copy any slices", func(t *testing.T) {
		original := map[string]any{
			"values": []any{1, "two", true},
		}

		result := copyValues(original)

		// Verify the slice was copied
		assert.Equal(t, original, result)

		// Modify the original slice
		originalValues := original["values"].([]any)
		originalValues[0] = 999

		// Result should not be affected
		resultValues := result["values"].([]any)
		assert.Equal(t, 1, resultValues[0])
	})

	t.Run("Should handle nil values", func(t *testing.T) {
		copied := copyValues(nil)

		assert.Nil(t, copied)
	})

	t.Run("Should handle empty map", func(t *testing.T) {
		original := make(map[string]any)

		copied := copyValues(original)

		assert.NotNil(t, copied)
		assert.Equal(t, 0, len(copied))
	})
}

func TestOverrideCache_CronOverrides(t *testing.T) {
	t.Run("Should store and retrieve cron override", func(t *testing.T) {
		cache := NewOverrideCache()
		workflowID := "test-workflow"
		values := map[string]any{"enabled": false, "cron": "0 */10 * * *"}

		cache.SetOverride(workflowID, values)

		override, exists := cache.GetOverride(workflowID)
		require.True(t, exists)
		assert.Equal(t, workflowID, override.WorkflowID)
		assert.Equal(t, false, override.Values["enabled"])
		assert.Equal(t, "0 */10 * * *", override.Values["cron"])
		assert.True(t, override.ModifiedAt.Before(time.Now().Add(time.Second)))
	})
}
