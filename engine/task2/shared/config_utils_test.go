package shared

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalConfigLimits_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent access without race conditions", func(t *testing.T) {
		// Reset global state before test
		resetGlobalConfigLimits()

		// Number of concurrent goroutines
		numGoroutines := 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Channel to collect results
		results := make(chan *ConfigLimits, numGoroutines)

		// Launch concurrent goroutines
		for range numGoroutines {
			go func() {
				defer wg.Done()
				limits := GetGlobalConfigLimits()
				results <- limits
			}()
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(results)

		// Verify all goroutines got the same instance
		var firstResult *ConfigLimits
		for result := range results {
			if firstResult == nil {
				firstResult = result
			} else {
				assert.Same(t, firstResult, result, "All goroutines should get the same singleton instance")
			}
		}

		// Verify the singleton was initialized correctly
		assert.NotNil(t, firstResult)
		assert.Equal(t, 20, firstResult.MaxNestingDepth)       // Provider default is 20
		assert.Equal(t, 10485760, firstResult.MaxStringLength) // Provider default is 10MB
	})
	t.Run("Should handle concurrent refresh and access", func(t *testing.T) {
		// Reset global state before test
		resetGlobalConfigLimits()

		// Set test environment variables
		os.Setenv("LIMITS_MAX_NESTING_DEPTH", "20")
		os.Setenv("LIMITS_MAX_STRING_LENGTH", "2048")
		defer func() {
			os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")
			os.Unsetenv("LIMITS_MAX_STRING_LENGTH")
		}()

		// Number of concurrent operations
		numOperations := 50
		var wg sync.WaitGroup

		// Launch concurrent readers
		wg.Add(numOperations)
		for range numOperations {
			go func() {
				defer wg.Done()
				limits := GetGlobalConfigLimits()
				assert.NotNil(t, limits)
				// Values should be either default or updated
				assert.True(t,
					limits.MaxNestingDepth == 20,
					"MaxNestingDepth should be the value we set")
			}()
		}

		// Launch concurrent refreshers
		wg.Add(numOperations)
		for range numOperations {
			go func() {
				defer wg.Done()
				RefreshGlobalConfigLimits()
			}()
		}

		// Wait for all operations to complete
		wg.Wait()

		// Final state should have the updated values
		finalLimits := GetGlobalConfigLimits()
		assert.Equal(t, 20, finalLimits.MaxNestingDepth)
		assert.Equal(t, 2048, finalLimits.MaxStringLength)
	})
	t.Run("Should properly initialize singleton on first access", func(t *testing.T) {
		// Reset global state
		resetGlobalConfigLimits()

		// First access should initialize
		limits := GetGlobalConfigLimits()
		require.NotNil(t, limits)

		// Verify initialization values from provider defaults
		assert.Equal(t, 20, limits.MaxNestingDepth)       // Provider default is 20
		assert.Equal(t, 10485760, limits.MaxStringLength) // Provider default is 10MB
		assert.Equal(t, 5, limits.MaxContextDepth)        // Provider default is 5 for MaxTaskContextDepth
		assert.Equal(t, 20, limits.MaxParentDepth)        // Uses MaxNestingDepth value
		assert.Equal(t, 20, limits.MaxChildrenDepth)      // Uses MaxNestingDepth value
		assert.Equal(t, 20, limits.MaxConfigDepth)        // Uses MaxNestingDepth value
		assert.Equal(t, DefaultMaxTemplateDepth, limits.MaxTemplateDepth)
	})
}

func TestRefreshGlobalConfigLimits(t *testing.T) {
	t.Run("Should refresh configuration from environment", func(t *testing.T) {
		// Reset global state
		resetGlobalConfigLimits()

		// Get initial config
		initial := GetGlobalConfigLimits()
		assert.Equal(t, 20, initial.MaxNestingDepth) // Provider default is 20

		// Update environment
		os.Setenv("LIMITS_MAX_NESTING_DEPTH", "30")
		defer os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")

		// Refresh
		RefreshGlobalConfigLimits()

		// Get updated config
		updated := GetGlobalConfigLimits()
		assert.Equal(t, 30, updated.MaxNestingDepth)
		assert.NotSame(t, initial, updated, "Should be a new instance after refresh")
	})
	t.Run("Should handle concurrent refresh operations", func(t *testing.T) {
		// Reset global state
		resetGlobalConfigLimits()

		// Set different environment values
		testValues := []string{"15", "25", "35", "45"}

		var wg sync.WaitGroup
		for _, val := range testValues {
			wg.Add(1)
			value := val // Capture loop variable
			go func() {
				defer wg.Done()
				os.Setenv("LIMITS_MAX_NESTING_DEPTH", value)
				RefreshGlobalConfigLimits()
				os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")
			}()
		}

		wg.Wait()

		// Final state should be consistent (not corrupted)
		final := GetGlobalConfigLimits()
		assert.NotNil(t, final)
		// Value should be one of the test values or provider default
		validValues := []int{20, 15, 25, 35, 45} // 20 is provider default
		assert.Contains(t, validValues, final.MaxNestingDepth)
	})
}

func TestGetConfigLimits(t *testing.T) {
	t.Run("Should use default values when no environment variables set", func(t *testing.T) {
		// Reset global state before test
		resetGlobalConfigLimits()

		// Ensure no env vars are set
		os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")
		os.Unsetenv("LIMITS_MAX_STRING_LENGTH")
		os.Unsetenv("LIMITS_MAX_TASK_CONTEXT_DEPTH")

		limits := GetConfigLimits()

		// The new config system has different defaults from the provider
		assert.Equal(t, 20, limits.MaxNestingDepth)       // Provider default is 20
		assert.Equal(t, 10485760, limits.MaxStringLength) // Provider default is 10MB
		assert.Equal(
			t,
			5,
			limits.MaxContextDepth,
		) // Provider default is 5 for MaxTaskContextDepth
		assert.Equal(t, 20, limits.MaxParentDepth)                        // Uses MaxNestingDepth value
		assert.Equal(t, 20, limits.MaxChildrenDepth)                      // Uses MaxNestingDepth value
		assert.Equal(t, 20, limits.MaxConfigDepth)                        // Uses MaxNestingDepth value
		assert.Equal(t, DefaultMaxTemplateDepth, limits.MaxTemplateDepth) // Still uses constant
	})
	t.Run("Should use environment values when set", func(t *testing.T) {
		// Reset global state before test to ensure clean config loading
		resetGlobalConfigLimits()

		// Set environment variables BEFORE the config is loaded
		os.Setenv("LIMITS_MAX_NESTING_DEPTH", "50")
		os.Setenv("LIMITS_MAX_STRING_LENGTH", "4096")
		os.Setenv("LIMITS_MAX_TASK_CONTEXT_DEPTH", "10")
		defer func() {
			os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")
			os.Unsetenv("LIMITS_MAX_STRING_LENGTH")
			os.Unsetenv("LIMITS_MAX_TASK_CONTEXT_DEPTH")
		}()

		limits := GetConfigLimits()

		// Debug output
		t.Logf("Environment vars: LIMITS_MAX_NESTING_DEPTH=%s", os.Getenv("LIMITS_MAX_NESTING_DEPTH"))
		t.Logf("Got MaxNestingDepth: %d", limits.MaxNestingDepth)
		t.Logf("Got MaxStringLength: %d", limits.MaxStringLength)
		t.Logf("Got MaxContextDepth: %d", limits.MaxContextDepth)

		// MaxNestingDepth affects multiple fields
		assert.Equal(t, 50, limits.MaxNestingDepth)
		assert.Equal(t, 50, limits.MaxParentDepth)
		assert.Equal(t, 50, limits.MaxChildrenDepth)
		assert.Equal(t, 50, limits.MaxConfigDepth)

		// Specific overrides
		assert.Equal(t, 4096, limits.MaxStringLength)
		assert.Equal(t, 10, limits.MaxContextDepth) // Overridden by specific env var
	})
	t.Run("Should handle invalid environment values", func(t *testing.T) {
		// Reset global state before test
		resetGlobalConfigLimits()

		// Set invalid values
		os.Setenv("LIMITS_MAX_NESTING_DEPTH", "invalid")
		os.Setenv("LIMITS_MAX_STRING_LENGTH", "-100")
		defer func() {
			os.Unsetenv("LIMITS_MAX_NESTING_DEPTH")
			os.Unsetenv("LIMITS_MAX_STRING_LENGTH")
		}()

		limits := GetConfigLimits()

		// Should fall back to provider defaults for invalid values
		assert.Equal(t, 20, limits.MaxNestingDepth)       // Provider default is 20
		assert.Equal(t, 10485760, limits.MaxStringLength) // Provider default is 10MB
	})
}

// BenchmarkGetGlobalConfigLimits tests performance under concurrent load
func BenchmarkGetGlobalConfigLimits(b *testing.B) {
	// Reset state
	resetGlobalConfigLimits()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = GetGlobalConfigLimits()
		}
	})
}

// BenchmarkRefreshGlobalConfigLimits tests refresh performance
func BenchmarkRefreshGlobalConfigLimits(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			RefreshGlobalConfigLimits()
		}
	})
}
