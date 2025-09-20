package privacy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// TestResilientManager_NewResilientManager tests the creation of a resilient manager
func TestResilientManager_NewResilientManager(t *testing.T) {
	t.Run("Should create with default config when nil", func(t *testing.T) {
		rm := NewResilientManager(nil, nil)
		require.NotNil(t, rm)
		require.NotNil(t, rm.ManagerInterface)
		require.NotNil(t, rm.runner)
		require.NotNil(t, rm.config)
		// Check default config values
		assert.Equal(t, 100*time.Millisecond, rm.config.TimeoutDuration)
		assert.Equal(t, 50, rm.config.ErrorPercentThresholdToOpen)
		assert.Equal(t, 10, rm.config.MinimumRequestToOpen)
		assert.Equal(t, 5*time.Second, rm.config.WaitDurationInOpenState)
		assert.Equal(t, 3, rm.config.RetryTimes)
		assert.Equal(t, 50*time.Millisecond, rm.config.RetryWaitBase)
	})
	t.Run("Should use provided config", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             200 * time.Millisecond,
			ErrorPercentThresholdToOpen: 75,
			MinimumRequestToOpen:        20,
			WaitDurationInOpenState:     10 * time.Second,
			RetryTimes:                  5,
			RetryWaitBase:               100 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		require.NotNil(t, rm)
		assert.Equal(t, config, rm.config)
	})
}

// TestResilientManager_ApplyPrivacyControls tests applying privacy controls with resilience
func TestResilientManager_ApplyPrivacyControls(t *testing.T) {
	t.Run("Should succeed with normal operation", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     1 * time.Second,
			RetryTimes:                  3,
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		ctx := context.Background()
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "test message",
		}
		metadata := memcore.PrivacyMetadata{}
		resultMsg, resultMeta, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		require.NoError(t, err)
		assert.Equal(t, msg, resultMsg)
		// Base manager always sets RedactionApplied even if no policy exists
		assert.True(t, resultMeta.RedactionApplied)
		assert.False(t, resultMeta.DoNotPersist)
	})
	t.Run("Should timeout on slow operation", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             50 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     1 * time.Second,
			RetryTimes:                  0, // No retries for this test
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		// Override the base manager with a slow implementation
		rm.ManagerInterface = &slowPrivacyManager{ManagerInterface: NewManager(), delay: 200 * time.Millisecond}
		ctx := context.Background()
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "test message",
		}
		metadata := memcore.PrivacyMetadata{}
		resultMsg, resultMeta, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		require.NoError(t, err) // Should fallback gracefully
		assert.Equal(t, msg, resultMsg)
		assert.False(t, resultMeta.RedactionApplied)
		assert.True(t, resultMeta.DoNotPersist) // Safety fallback
	})
	t.Run("Should retry on transient failures", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     1 * time.Second,
			RetryTimes:                  3,
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		// Override with a failing manager that succeeds after 2 attempts
		failingManager := &failingPrivacyManager{ManagerInterface: NewManager(), failuresBeforeSuccess: 2}
		rm.ManagerInterface = failingManager
		ctx := context.Background()
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "test message",
		}
		metadata := memcore.PrivacyMetadata{}
		resultMsg, resultMeta, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		require.NoError(t, err)
		assert.Equal(t, msg, resultMsg)
		assert.Equal(t, int32(3), atomic.LoadInt32(&failingManager.attempts)) // 1 initial + 2 retries
		_ = resultMeta                                                        // Use the variable
	})
	t.Run("Should open circuit breaker after failures", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        5, // Lower for testing
			WaitDurationInOpenState:     100 * time.Millisecond,
			RetryTimes:                  0, // No retries to speed up test
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		// Override with always failing manager
		rm.ManagerInterface = &alwaysFailingPrivacyManager{ManagerInterface: NewManager()}
		ctx := context.Background()
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "test message",
		}
		metadata := memcore.PrivacyMetadata{}
		// Make enough requests to open the circuit
		for range 10 {
			_, _, _ = rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		}
		// Circuit should be open now
		isOpen, _, _ := rm.GetCircuitBreakerStatus()
		assert.True(t, isOpen)
	})
}

// TestResilientManager_RedactContent tests content redaction with resilience
func TestResilientManager_RedactContent(t *testing.T) {
	t.Run("Should redact content successfully", func(t *testing.T) {
		config := DefaultResilienceConfig()
		rm := NewResilientManager(nil, config)
		content := "My SSN is 123-45-6789 and email is test@example.com"
		patterns := []string{`\d{3}-\d{2}-\d{4}`, `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`}
		result, err := rm.RedactContent(context.Background(), content, patterns, "[REDACTED]")
		require.NoError(t, err)
		assert.Equal(t, "My SSN is [REDACTED] and email is [REDACTED]", result)
	})
	t.Run("Should handle timeout gracefully", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             10 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     1 * time.Second,
			RetryTimes:                  0,
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		// Override with slow redaction
		rm.ManagerInterface = &slowPrivacyManager{ManagerInterface: NewManager(), delay: 100 * time.Millisecond}
		content := "My SSN is 123-45-6789"
		patterns := []string{`\d{3}-\d{2}-\d{4}`}
		result, err := rm.RedactContent(context.Background(), content, patterns, "[REDACTED]")
		require.Error(t, err)
		assert.Equal(t, "[REDACTED]", result) // Should return safe fallback on timeout
		assert.Contains(t, err.Error(), "redaction failed")
	})
}

// TestResilientManager_UpdateConfig tests configuration updates
func TestResilientManager_UpdateConfig(t *testing.T) {
	t.Run("Should update configuration", func(t *testing.T) {
		initialConfig := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     5 * time.Second,
			RetryTimes:                  3,
			RetryWaitBase:               50 * time.Millisecond,
		}
		rm := NewResilientManager(nil, initialConfig)
		newConfig := &ResilienceConfig{
			TimeoutDuration:             200 * time.Millisecond,
			ErrorPercentThresholdToOpen: 75,
			MinimumRequestToOpen:        20,
			WaitDurationInOpenState:     10 * time.Second,
			RetryTimes:                  5,
			RetryWaitBase:               100 * time.Millisecond,
		}
		rm.UpdateConfig(newConfig)
		assert.Equal(t, newConfig, rm.config)
		assert.NotNil(t, rm.runner) // Runner should be recreated
	})
}

// TestValidateConfig tests configuration validation
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *ResilienceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Should fail for nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "config cannot be nil",
		},
		{
			name: "Should fail for zero timeout",
			config: &ResilienceConfig{
				TimeoutDuration:             0,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  3,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "timeout duration must be positive",
		},
		{
			name: "Should fail for invalid error percent",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 101,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  3,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "error percent threshold must be between 0 and 100",
		},
		{
			name: "Should fail for zero minimum requests",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        0,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  3,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "minimum request to open must be positive",
		},
		{
			name: "Should fail for zero wait duration",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     0,
				RetryTimes:                  3,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "wait duration in open state must be positive",
		},
		{
			name: "Should fail for negative retry times",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  -1,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "retry times cannot be negative",
		},
		{
			name: "Should fail for negative retry wait base",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  3,
				RetryWaitBase:               -1 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "retry wait base cannot be negative",
		},
		{
			name: "Should succeed for valid config",
			config: &ResilienceConfig{
				TimeoutDuration:             100 * time.Millisecond,
				ErrorPercentThresholdToOpen: 50,
				MinimumRequestToOpen:        10,
				WaitDurationInOpenState:     5 * time.Second,
				RetryTimes:                  3,
				RetryWaitBase:               50 * time.Millisecond,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestResilientManager_ConcurrentRequests tests concurrent request handling
func TestResilientManager_ConcurrentRequests(t *testing.T) {
	t.Run("Should handle concurrent requests", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50,
			MinimumRequestToOpen:        10,
			WaitDurationInOpenState:     1 * time.Second,
			RetryTimes:                  1,
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		ctx := context.Background()
		concurrency := 10
		var wg sync.WaitGroup
		wg.Add(concurrency)
		errors := make([]error, concurrency)
		for i := range concurrency {
			go func(idx int) {
				defer wg.Done()
				msg := llm.Message{
					Role:    llm.MessageRoleUser,
					Content: "test message",
				}
				metadata := memcore.PrivacyMetadata{}
				_, _, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
				errors[idx] = err
			}(i)
		}
		wg.Wait()
		// All requests should succeed
		for _, err := range errors {
			assert.NoError(t, err)
		}
	})
}

// Test helpers

// slowPrivacyManager simulates a slow privacy manager
type slowPrivacyManager struct {
	ManagerInterface
	delay time.Duration
}

func (m *slowPrivacyManager) ApplyPrivacyControls(
	ctx context.Context,
	msg llm.Message,
	_ string,
	metadata memcore.PrivacyMetadata,
) (llm.Message, memcore.PrivacyMetadata, error) {
	select {
	case <-time.After(m.delay):
		return msg, metadata, nil
	case <-ctx.Done():
		return msg, metadata, ctx.Err()
	}
}

func (m *slowPrivacyManager) RedactContent(
	_ context.Context,
	content string,
	_ []string,
	_ string,
) (string, error) {
	time.Sleep(m.delay)
	return content, nil
}

// failingPrivacyManager fails a certain number of times before succeeding
type failingPrivacyManager struct {
	ManagerInterface
	failuresBeforeSuccess int
	attempts              int32
}

func (m *failingPrivacyManager) ApplyPrivacyControls(
	_ context.Context,
	msg llm.Message,
	_ string,
	metadata memcore.PrivacyMetadata,
) (llm.Message, memcore.PrivacyMetadata, error) {
	attempt := atomic.AddInt32(&m.attempts, 1)
	if int(attempt) <= m.failuresBeforeSuccess {
		return msg, metadata, errors.New("transient error")
	}
	return msg, metadata, nil
}

// alwaysFailingPrivacyManager always fails
type alwaysFailingPrivacyManager struct {
	ManagerInterface
}

func (m *alwaysFailingPrivacyManager) ApplyPrivacyControls(
	_ context.Context,
	msg llm.Message,
	_ string,
	metadata memcore.PrivacyMetadata,
) (llm.Message, memcore.PrivacyMetadata, error) {
	return msg, metadata, errors.New("permanent error")
}

// TestResilientManager_CircuitBreakerIntegration tests circuit breaker behavior
func TestResilientManager_CircuitBreakerIntegration(t *testing.T) {
	t.Run("Should trip circuit after threshold", func(t *testing.T) {
		config := &ResilienceConfig{
			TimeoutDuration:             100 * time.Millisecond,
			ErrorPercentThresholdToOpen: 50, // 50% error rate
			MinimumRequestToOpen:        4,  // Need at least 4 requests
			WaitDurationInOpenState:     200 * time.Millisecond,
			RetryTimes:                  0, // No retries for this test
			RetryWaitBase:               10 * time.Millisecond,
		}
		rm := NewResilientManager(nil, config)
		// Override with failing manager
		rm.ManagerInterface = &alwaysFailingPrivacyManager{ManagerInterface: NewManager()}
		ctx := context.Background()
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "test"}
		metadata := memcore.PrivacyMetadata{}
		// Make requests to trip the circuit
		for range 6 {
			_, _, _ = rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
			time.Sleep(10 * time.Millisecond) // Small delay between requests
		}
		// Circuit should be open
		isOpen, _, _ := rm.GetCircuitBreakerStatus()
		assert.True(t, isOpen)
		// Next request should fail immediately without calling the underlying manager
		start := time.Now()
		_, _, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		duration := time.Since(start)
		// Should fail fast when circuit is open
		assert.NoError(t, err)                        // Resilient manager handles it gracefully
		assert.Less(t, duration, 10*time.Millisecond) // Should be very fast
		// Wait for circuit to close
		time.Sleep(250 * time.Millisecond)
		// Circuit should be closed now
		isOpen, _, _ = rm.GetCircuitBreakerStatus()
		assert.False(t, isOpen)
	})
}

// TestResilientManager_EdgeCases tests edge cases and error conditions
func TestResilientManager_EdgeCases(t *testing.T) {
	t.Run("Should handle context cancellation", func(t *testing.T) {
		config := DefaultResilienceConfig()
		rm := NewResilientManager(nil, config)
		// Create a canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "test"}
		metadata := memcore.PrivacyMetadata{}
		_, resultMeta, err := rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		assert.NoError(t, err)                  // Fallback should handle it
		assert.True(t, resultMeta.DoNotPersist) // Safety fallback
	})
}

// TestResilientManager_PerformanceBaseline ensures resilience doesn't add excessive overhead
func TestResilientManager_PerformanceBaseline(t *testing.T) {
	t.Run("Should have minimal overhead for successful operations", func(t *testing.T) {
		config := DefaultResilienceConfig()
		rm := NewResilientManager(nil, config)
		ctx := context.Background()
		msg := llm.Message{Role: llm.MessageRoleUser, Content: strings.Repeat("test ", 100)}
		metadata := memcore.PrivacyMetadata{}
		// Warm up
		for range 10 {
			_, _, _ = rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		}
		// Measure baseline (direct manager)
		baseManager := NewManager()
		baseStart := time.Now()
		iterations := 100
		for range iterations {
			_, _, _ = baseManager.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		}
		baseDuration := time.Since(baseStart)
		// Measure with resilience
		resilientStart := time.Now()
		for range iterations {
			_, _, _ = rm.ApplyPrivacyControls(ctx, msg, "resource-1", metadata)
		}
		resilientDuration := time.Since(resilientStart)
		// Overhead is expected due to middleware chaining
		// For such a fast operation, the overhead will be significant in relative terms
		overheadRatio := float64(resilientDuration) / float64(baseDuration)
		t.Logf(
			"Base duration: %v, Resilient duration: %v, Overhead ratio: %.2f",
			baseDuration,
			resilientDuration,
			overheadRatio,
		)
		// Ensure the resilient manager completes in reasonable time (< 1ms per operation)
		avgDuration := resilientDuration / time.Duration(iterations)
		assert.Less(t, avgDuration, 1*time.Millisecond, "Average operation should complete in less than 1ms")
	})
}
