package mcpproxy

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMCPStatus_ThreadSafety tests concurrent access to MCPStatus methods
func TestMCPStatus_ThreadSafety(t *testing.T) {
	initLogger()

	status := NewMCPStatus("test-status")

	// Test concurrent reads and writes
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent status updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				status.UpdateStatus(StatusConnected, "")
				status.UpdateStatus(StatusError, "test error")
				status.RecordRequest(time.Millisecond * 100)
				status.IncrementErrors()
			}
		}()
	}

	// Concurrent status reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				statusCopy := status.SafeCopy()
				assert.NotNil(t, statusCopy)
				assert.Equal(t, "test-status", statusCopy.Name)
				_ = status.CalculateUpTime()
			}
		}()
	}

	wg.Wait()

	// Verify final state is consistent
	finalCopy := status.SafeCopy()
	assert.NotNil(t, finalCopy)
	assert.Equal(t, "test-status", finalCopy.Name)

	// Should have recorded some requests and errors
	assert.Greater(t, finalCopy.TotalRequests, int64(0))
	assert.Greater(t, finalCopy.TotalErrors, int64(0))

	// Verify final state consistency
	assert.True(t, finalCopy.TotalErrors > 0)
	assert.True(t, finalCopy.TotalRequests > 0)

	// Status should be one of the valid values
	assert.Contains(t, []ConnectionStatus{StatusConnected, StatusError}, finalCopy.Status)

	// Average response time should be reasonable
	if finalCopy.TotalRequests > 0 {
		assert.Greater(t, finalCopy.AvgResponseTime, time.Duration(0))
		assert.Less(t, finalCopy.AvgResponseTime, time.Second)
	}
}

// TestMCPStatus_SafeCopy tests that SafeCopy returns independent copies
func TestMCPStatus_SafeCopy(t *testing.T) {
	initLogger()

	original := NewMCPStatus("original")
	original.UpdateStatus(StatusConnected, "")
	original.RecordRequest(time.Second)

	// Get a safe copy
	statusCopy := original.SafeCopy()

	// Modify the original
	original.UpdateStatus(StatusError, "new error")
	original.RecordRequest(time.Millisecond)

	// Copy should be unchanged
	assert.Equal(t, StatusConnected, statusCopy.Status)
	assert.Equal(t, "", statusCopy.LastError)
	assert.Equal(t, time.Second, statusCopy.AvgResponseTime)
	assert.Equal(t, int64(1), statusCopy.TotalRequests)
}

// TestMCPStatus_ConcurrentSafeCopy tests concurrent SafeCopy operations
func TestMCPStatus_ConcurrentSafeCopy(t *testing.T) {
	initLogger()

	original := NewMCPStatus("concurrent-copy-test")
	original.UpdateStatus(StatusConnected, "")

	var wg sync.WaitGroup
	iterations := 50
	copies := make([]*MCPStatus, iterations*5) // 5 goroutines * 50 iterations

	// Concurrent SafeCopy operations while modifying the original
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(goroutineIndex int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				index := goroutineIndex*iterations + j
				copies[index] = original.SafeCopy()

				// Modify original during copy operations
				if j%2 == 0 {
					original.UpdateStatus(StatusError, "concurrent error")
				} else {
					original.UpdateStatus(StatusConnected, "")
				}
				original.RecordRequest(time.Duration(j+1) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify all copies are valid and independent
	for i, copy := range copies {
		assert.NotNil(t, copy, "Copy %d should not be nil", i)
		assert.Equal(t, "concurrent-copy-test", copy.Name, "Copy %d should have correct name", i)

		// Each copy should represent a valid state at some point in time
		assert.Contains(t, []ConnectionStatus{StatusConnected, StatusError}, copy.Status,
			"Copy %d should have valid status", i)
	}
}

// TestMCPStatus_RaceConditionDetection tests for race conditions in field updates
func TestMCPStatus_RaceConditionDetection(t *testing.T) {
	initLogger()

	status := NewMCPStatus("race-test")

	var wg sync.WaitGroup
	iterations := 200

	// Concurrent operations that could cause race conditions
	// 1. Status updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			switch i % 3 {
			case 0:
				status.UpdateStatus(StatusConnected, "")
			case 1:
				status.UpdateStatus(StatusError, "error message")
			case 2:
				status.UpdateStatus(StatusConnecting, "")
			}
		}
	}()

	// 2. Request recording
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			responseTime := time.Duration(i%100+1) * time.Millisecond
			status.RecordRequest(responseTime)
		}
	}()

	// 3. Error incrementing
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			status.IncrementErrors()
		}
	}()

	// 4. Reading operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			statusCopy := status.SafeCopy()
			assert.NotNil(t, statusCopy)
			uptime := status.CalculateUpTime()
			assert.GreaterOrEqual(t, uptime, time.Duration(0))
		}
	}()

	wg.Wait()

	// Verify final consistency
	finalCopy := status.SafeCopy()
	assert.NotNil(t, finalCopy)
	assert.Equal(t, "race-test", finalCopy.Name)

	// Check that counters make sense
	assert.GreaterOrEqual(t, finalCopy.TotalRequests, int64(0))
	assert.GreaterOrEqual(t, finalCopy.TotalErrors, int64(iterations)) // At least from IncrementErrors calls

	// Status should be valid
	assert.Contains(t, []ConnectionStatus{StatusConnected, StatusError, StatusConnecting}, finalCopy.Status)
}

// TestMCPStatus_TimestampConsistency tests timestamp field consistency under concurrent access
func TestMCPStatus_TimestampConsistency(t *testing.T) {
	initLogger()

	status := NewMCPStatus("timestamp-test")

	var wg sync.WaitGroup

	// Set initial connected state
	status.UpdateStatus(StatusConnected, "")

	// Concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				if j%2 == 0 {
					status.UpdateStatus(StatusConnected, "")
				} else {
					status.UpdateStatus(StatusError, "test error")
				}

				// Read the state
				statusCopy := status.SafeCopy()

				// Verify timestamp consistency
				if statusCopy.LastConnected != nil && statusCopy.LastErrorTime != nil {
					// Both timestamps exist, verify they're reasonable
					assert.True(t,
						statusCopy.LastConnected.Before(time.Now()) ||
							statusCopy.LastConnected.Equal(time.Now()),
					)
					assert.True(t,
						statusCopy.LastErrorTime.Before(time.Now()) ||
							statusCopy.LastErrorTime.Equal(time.Now()),
					)
				}

				// If status is connected and LastConnected exists, uptime should be reasonable
				if statusCopy.Status == StatusConnected && statusCopy.LastConnected != nil {
					assert.GreaterOrEqual(t, statusCopy.UpTime, time.Duration(0))
					assert.Less(t, statusCopy.UpTime, time.Hour) // Should be less than an hour for this test
				}
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	finalCopy := status.SafeCopy()
	assert.NotNil(t, finalCopy)

	// If connected, should have LastConnected timestamp
	if finalCopy.Status == StatusConnected {
		assert.NotNil(t, finalCopy.LastConnected)
	}
}

// TestMCPStatus_DataIntegrity tests that data remains consistent across concurrent operations
func TestMCPStatus_DataIntegrity(t *testing.T) {
	initLogger()

	status := NewMCPStatus("integrity-test")

	var wg sync.WaitGroup
	iterations := 100

	// Track expected totals
	var expectedRequests, expectedErrors int64
	var requestMutex, errorMutex sync.Mutex

	// Concurrent request recording
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				status.RecordRequest(time.Millisecond * 50)

				requestMutex.Lock()
				expectedRequests++
				requestMutex.Unlock()
			}
		}()
	}

	// Concurrent error incrementing
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				status.IncrementErrors()

				errorMutex.Lock()
				expectedErrors++
				errorMutex.Unlock()
			}
		}()
	}

	wg.Wait()

	// Verify data integrity
	finalCopy := status.SafeCopy()
	assert.Equal(t, expectedRequests, finalCopy.TotalRequests,
		"Total requests should match expected count")
	assert.GreaterOrEqual(t, finalCopy.TotalErrors, expectedErrors,
		"Total errors should be at least expected count (may include UpdateStatus errors)")

	// Verify average response time is reasonable
	if finalCopy.TotalRequests > 0 {
		assert.Greater(t, finalCopy.AvgResponseTime, time.Duration(0))
		assert.LessOrEqual(t, finalCopy.AvgResponseTime, time.Second)
	}
}
