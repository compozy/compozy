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
		globalConfigLimits = nil

		// Number of concurrent goroutines
		numGoroutines := 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Channel to collect results
		results := make(chan *ConfigLimits, numGoroutines)

		// Launch concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
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
		assert.Equal(t, DefaultMaxParentDepth, firstResult.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, firstResult.MaxStringLength)
	})
	t.Run("Should handle concurrent refresh and access", func(t *testing.T) {
		// Reset global state before test
		globalConfigLimits = nil

		// Set test environment variables
		os.Setenv(EnvMaxNestingDepth, "20")
		os.Setenv(EnvMaxStringLength, "2048")
		defer func() {
			os.Unsetenv(EnvMaxNestingDepth)
			os.Unsetenv(EnvMaxStringLength)
		}()

		// Number of concurrent operations
		numOperations := 50
		var wg sync.WaitGroup

		// Launch concurrent readers
		wg.Add(numOperations)
		for i := 0; i < numOperations; i++ {
			go func() {
				defer wg.Done()
				limits := GetGlobalConfigLimits()
				assert.NotNil(t, limits)
				// Values should be either default or updated
				assert.True(t,
					limits.MaxNestingDepth == DefaultMaxParentDepth || limits.MaxNestingDepth == 20,
					"MaxNestingDepth should be either default or updated value")
			}()
		}

		// Launch concurrent refreshers
		wg.Add(numOperations)
		for i := 0; i < numOperations; i++ {
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
		globalConfigLimits = nil

		// First access should initialize
		limits := GetGlobalConfigLimits()
		require.NotNil(t, limits)

		// Verify initialization values
		assert.Equal(t, DefaultMaxParentDepth, limits.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, limits.MaxStringLength)
		assert.Equal(t, DefaultMaxContextDepth, limits.MaxContextDepth)
		assert.Equal(t, DefaultMaxParentDepth, limits.MaxParentDepth)
		assert.Equal(t, DefaultMaxChildrenDepth, limits.MaxChildrenDepth)
		assert.Equal(t, DefaultMaxConfigDepth, limits.MaxConfigDepth)
		assert.Equal(t, DefaultMaxTemplateDepth, limits.MaxTemplateDepth)
	})
}

func TestRefreshGlobalConfigLimits(t *testing.T) {
	t.Run("Should refresh configuration from environment", func(t *testing.T) {
		// Reset global state
		globalConfigLimits = nil

		// Get initial config
		initial := GetGlobalConfigLimits()
		assert.Equal(t, DefaultMaxParentDepth, initial.MaxNestingDepth)

		// Update environment
		os.Setenv(EnvMaxNestingDepth, "30")
		defer os.Unsetenv(EnvMaxNestingDepth)

		// Refresh
		RefreshGlobalConfigLimits()

		// Get updated config
		updated := GetGlobalConfigLimits()
		assert.Equal(t, 30, updated.MaxNestingDepth)
		assert.NotSame(t, initial, updated, "Should be a new instance after refresh")
	})
	t.Run("Should handle concurrent refresh operations", func(t *testing.T) {
		// Reset global state
		globalConfigLimits = nil

		// Set different environment values
		testValues := []string{"15", "25", "35", "45"}

		var wg sync.WaitGroup
		for _, val := range testValues {
			wg.Add(1)
			value := val // Capture loop variable
			go func() {
				defer wg.Done()
				os.Setenv(EnvMaxNestingDepth, value)
				RefreshGlobalConfigLimits()
				os.Unsetenv(EnvMaxNestingDepth)
			}()
		}

		wg.Wait()

		// Final state should be consistent (not corrupted)
		final := GetGlobalConfigLimits()
		assert.NotNil(t, final)
		// Value should be one of the test values or default
		validValues := []int{DefaultMaxParentDepth, 15, 25, 35, 45}
		assert.Contains(t, validValues, final.MaxNestingDepth)
	})
}

func TestGetConfigLimits(t *testing.T) {
	t.Run("Should use default values when no environment variables set", func(t *testing.T) {
		// Ensure no env vars are set
		os.Unsetenv(EnvMaxNestingDepth)
		os.Unsetenv(EnvMaxStringLength)
		os.Unsetenv(EnvMaxTaskContextDepth)

		limits := GetConfigLimits()

		assert.Equal(t, DefaultMaxParentDepth, limits.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, limits.MaxStringLength)
		assert.Equal(t, DefaultMaxContextDepth, limits.MaxContextDepth)
		assert.Equal(t, DefaultMaxParentDepth, limits.MaxParentDepth)
		assert.Equal(t, DefaultMaxChildrenDepth, limits.MaxChildrenDepth)
		assert.Equal(t, DefaultMaxConfigDepth, limits.MaxConfigDepth)
		assert.Equal(t, DefaultMaxTemplateDepth, limits.MaxTemplateDepth)
	})
	t.Run("Should use environment values when set", func(t *testing.T) {
		// Set environment variables
		os.Setenv(EnvMaxNestingDepth, "50")
		os.Setenv(EnvMaxStringLength, "4096")
		os.Setenv(EnvMaxTaskContextDepth, "10")
		defer func() {
			os.Unsetenv(EnvMaxNestingDepth)
			os.Unsetenv(EnvMaxStringLength)
			os.Unsetenv(EnvMaxTaskContextDepth)
		}()

		limits := GetConfigLimits()

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
		// Set invalid values
		os.Setenv(EnvMaxNestingDepth, "invalid")
		os.Setenv(EnvMaxStringLength, "-100")
		defer func() {
			os.Unsetenv(EnvMaxNestingDepth)
			os.Unsetenv(EnvMaxStringLength)
		}()

		limits := GetConfigLimits()

		// Should fall back to defaults for invalid values
		assert.Equal(t, DefaultMaxParentDepth, limits.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, limits.MaxStringLength)
	})
}

// BenchmarkGetGlobalConfigLimits tests performance under concurrent load
func BenchmarkGetGlobalConfigLimits(b *testing.B) {
	// Reset state
	globalConfigLimits = nil

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
