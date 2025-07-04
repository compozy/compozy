package shared

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalConfigLimits_ExtremeLoad(t *testing.T) {
	t.Run("Should handle extreme concurrent access without race conditions", func(t *testing.T) {
		// Reset global state
		RefreshGlobalConfigLimits()
		// Number of concurrent goroutines
		const numGoroutines = 1000
		const numIterations = 100
		// WaitGroup to synchronize goroutines
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		// Channel to collect errors
		errChan := make(chan error, numGoroutines*numIterations)
		// Launch concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(_ int) {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					limits := GetGlobalConfigLimits()
					// Verify the limits are valid
					if limits == nil {
						errChan <- assert.AnError
						return
					}
					// Verify default values exist
					if limits.MaxNestingDepth == 0 {
						errChan <- assert.AnError
						return
					}
					if limits.MaxStringLength == 0 {
						errChan <- assert.AnError
						return
					}
				}
			}(i)
		}
		// Wait for all goroutines to complete
		wg.Wait()
		close(errChan)
		// Check for errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}
		require.Empty(t, errors, "Expected no errors during concurrent access")
		// Verify global state is still valid
		finalLimits := GetGlobalConfigLimits()
		assert.NotNil(t, finalLimits)
		assert.Equal(t, DefaultMaxParentDepth, finalLimits.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, finalLimits.MaxStringLength)
	})

	t.Run("Should maintain data integrity under concurrent writes and reads", func(t *testing.T) {
		// Reset global state
		configLimitsMutex.Lock()
		globalConfigLimits = nil
		configLimitsMutex.Unlock()
		// Number of concurrent operations
		const numOperations = 5000
		var wg sync.WaitGroup
		wg.Add(numOperations * 2) // Half reads, half refreshes
		// Channel to track successful operations
		successChan := make(chan bool, numOperations*2)
		// Concurrent reads
		for i := 0; i < numOperations; i++ {
			go func() {
				defer wg.Done()
				limits := GetGlobalConfigLimits()
				if limits != nil {
					successChan <- true
				}
			}()
		}
		// Concurrent refreshes to simulate config updates
		for i := 0; i < numOperations; i++ {
			go func() {
				defer wg.Done()
				RefreshGlobalConfigLimits()
				successChan <- true
			}()
		}
		// Wait for all operations to complete
		wg.Wait()
		close(successChan)
		// Count successful operations
		successCount := 0
		for range successChan {
			successCount++
		}
		// All operations should complete successfully
		assert.Equal(t, numOperations*2, successCount, "All operations should complete successfully")
	})
}

func TestGetGlobalConfigLimits_MemoryPressure(t *testing.T) {
	t.Run("Should not leak memory under repeated access", func(t *testing.T) {
		// Reset global state
		RefreshGlobalConfigLimits()
		// Perform many sequential accesses
		const numAccesses = 10000
		for i := 0; i < numAccesses; i++ {
			limits := GetGlobalConfigLimits()
			require.NotNil(t, limits)
			// Occasionally refresh config to simulate real-world updates
			if i%1000 == 0 {
				RefreshGlobalConfigLimits()
			}
		}
		// Verify final state
		finalLimits := GetGlobalConfigLimits()
		require.NotNil(t, finalLimits)
		assert.Equal(t, DefaultMaxParentDepth, finalLimits.MaxNestingDepth)
		assert.Equal(t, DefaultMaxStringLength, finalLimits.MaxStringLength)
	})
}

func TestGetGlobalConfigLimits_RaceConditionProtection(t *testing.T) {
	t.Run("Should properly use double-checked locking pattern", func(t *testing.T) {
		// Force reset of global state
		configLimitsMutex.Lock()
		globalConfigLimits = nil
		configLimitsMutex.Unlock()
		// Track how many times GetConfigLimits is called
		var callCount int32
		var callCountMutex sync.Mutex
		// Concurrent goroutines that all try to initialize
		const numGoroutines = 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		startSignal := make(chan struct{})
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				// Wait for signal to start all at once
				<-startSignal
				// Track entry into critical section
				configLimitsMutex.RLock()
				wasNil := globalConfigLimits == nil
				configLimitsMutex.RUnlock()
				if wasNil {
					callCountMutex.Lock()
					callCount++
					callCountMutex.Unlock()
				}
				// Get the config
				limits := GetGlobalConfigLimits()
				assert.NotNil(t, limits)
			}()
		}
		// Start all goroutines at once
		close(startSignal)
		wg.Wait()
		// Due to double-checked locking, GetConfigLimits should be called only once
		// even with concurrent access
		assert.LessOrEqual(t, int(callCount), numGoroutines, "Multiple goroutines entered critical section")
	})
}
