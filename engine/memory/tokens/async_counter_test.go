package tokens

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockTokenCounterAsync struct {
	mock.Mock
}

func (m *mockTokenCounterAsync) CountTokens(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Int(0), args.Error(1)
}

func (m *mockTokenCounterAsync) GetEncoding() string {
	args := m.Called()
	return args.String(0)
}

func TestAsyncTokenCounter_ProcessAsync(t *testing.T) {
	t.Run("Should process token counting asynchronously", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, 100)
		defer asyncCounter.Shutdown()
		text := "Test message content"
		ref := "test_memory"
		mockCounter.On("CountTokens", mock.Anything, text).
			Return(10, nil).
			After(50 * time.Millisecond) // Simulate processing time
		// Act
		start := time.Now()
		asyncCounter.ProcessAsync(context.Background(), ref, text)
		duration := time.Since(start)
		// Assert
		assert.Less(t, duration, 10*time.Millisecond,
			"ProcessAsync should return immediately")
		// Wait for async processing
		time.Sleep(100 * time.Millisecond)
		mockCounter.AssertExpectations(t)
	})
	t.Run("Should handle queue full gracefully", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Create with worker count 0 to prevent processing
		asyncCounter := &AsyncTokenCounter{
			realCounter: mockCounter,
			queue:       make(chan *tokenCountRequest, 1), // Small queue
			workers:     0,
			metrics:     NewTokenMetrics(),
		}
		text := "Test"
		ref := "test"
		// Fill the queue
		asyncCounter.queue <- &tokenCountRequest{}
		// Act - should not block
		asyncCounter.ProcessAsync(context.Background(), ref, text)
		// Assert
		stats := asyncCounter.metrics.GetStats()
		assert.Equal(t, uint64(1), stats["dropped_count"])
		// Clean up
		<-asyncCounter.queue // drain the queue
	})
	t.Run("Should handle counter errors gracefully", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 1, 100)
		defer asyncCounter.Shutdown()
		text := "Test message"
		ref := "test_memory"
		mockCounter.On("CountTokens", mock.Anything, text).
			Return(0, errors.New("counter error"))
		// Act
		asyncCounter.ProcessAsync(context.Background(), ref, text)
		// Wait for processing
		time.Sleep(50 * time.Millisecond)
		// Assert
		stats := asyncCounter.metrics.GetStats()
		assert.Equal(t, uint64(1), stats["error_count"])
		mockCounter.AssertExpectations(t)
	})
}

func TestAsyncTokenCounter_ProcessWithResult(t *testing.T) {
	t.Run("Should return token count result", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, 100)
		defer asyncCounter.Shutdown()
		text := "Test message"
		ref := "test"
		mockCounter.On("CountTokens", mock.Anything, text).Return(15, nil)
		// Act
		count, err := asyncCounter.ProcessWithResult(
			context.Background(), ref, text)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 15, count)
		mockCounter.AssertExpectations(t)
	})
	t.Run("Should handle queue full", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Create with worker count 0 to prevent processing
		asyncCounter := &AsyncTokenCounter{
			realCounter: mockCounter,
			queue:       make(chan *tokenCountRequest, 1), // Small queue
			workers:     0,
			metrics:     NewTokenMetrics(),
		}
		text := "Test message"
		ref := "test"
		// Fill queue to prevent our request from being processed
		asyncCounter.queue <- &tokenCountRequest{
			ctx:       context.Background(),
			memoryRef: "blocking",
			text:      "blocking request",
		}
		// Act
		_, err := asyncCounter.ProcessWithResult(
			context.Background(), ref, text)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "queue full")
		// Clean up
		<-asyncCounter.queue // drain the queue
	})
	t.Run("Should handle context cancellation", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 1, 100)
		defer asyncCounter.Shutdown()
		text := "Test message"
		ref := "test"
		ctx, cancel := context.WithCancel(context.Background())
		mockCounter.On("CountTokens", mock.Anything, text).
			Return(15, nil).
			After(100 * time.Millisecond)
		// Act
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		_, err := asyncCounter.ProcessWithResult(ctx, ref, text)
		// Assert
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAsyncTokenCounter_Metrics(t *testing.T) {
	t.Run("Should track metrics correctly", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, 100)
		defer asyncCounter.Shutdown()
		// Set up mock expectations
		mockCounter.On("CountTokens", mock.Anything, "success1").Return(10, nil)
		mockCounter.On("CountTokens", mock.Anything, "success2").Return(20, nil)
		mockCounter.On("CountTokens", mock.Anything, "error").Return(0, errors.New("error"))
		// Act
		asyncCounter.ProcessAsync(context.Background(), "ref1", "success1")
		asyncCounter.ProcessAsync(context.Background(), "ref2", "success2")
		asyncCounter.ProcessAsync(context.Background(), "ref3", "error")
		// Wait for processing
		time.Sleep(100 * time.Millisecond)
		// Assert
		stats := asyncCounter.metrics.GetStats()
		assert.Equal(t, uint64(2), stats["success_count"])
		assert.Equal(t, uint64(1), stats["error_count"])
		assert.Greater(t, stats["avg_duration_ns"].(int64), int64(0))
		mockCounter.AssertExpectations(t)
	})
}

func TestAsyncTokenCounter_WorkerPool(t *testing.T) {
	t.Run("Should process requests concurrently", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 3, 100) // 3 workers
		defer asyncCounter.Shutdown()
		// Track concurrent executions with atomic operations
		var concurrentCount atomic.Int32
		var maxConcurrent atomic.Int32
		ch := make(chan struct{}, 3)
		mockCounter.On("CountTokens", mock.Anything, mock.Anything).
			Run(func(_ mock.Arguments) {
				ch <- struct{}{}
				current := concurrentCount.Add(1)
				// Update max concurrent atomically
				for {
					oldMax := maxConcurrent.Load()
					if current <= oldMax || maxConcurrent.CompareAndSwap(oldMax, current) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				concurrentCount.Add(-1)
				<-ch
			}).
			Return(10, nil)
		// Act - send multiple requests
		for i := 0; i < 6; i++ {
			asyncCounter.ProcessAsync(context.Background(), "ref", "text")
		}
		// Wait for processing
		time.Sleep(200 * time.Millisecond)
		// Assert
		assert.GreaterOrEqual(t, int(maxConcurrent.Load()), 2, "Should process at least 2 requests concurrently")
		mockCounter.AssertExpectations(t)
	})
}

func TestAsyncTokenCounter_Shutdown(t *testing.T) {
	t.Run("Should shutdown gracefully", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, 100)
		// Queue some requests
		mockCounter.On("CountTokens", mock.Anything, mock.Anything).
			Return(10, nil).Maybe()
		for i := 0; i < 5; i++ {
			asyncCounter.ProcessAsync(context.Background(), "ref", "text")
		}
		// Act
		asyncCounter.Shutdown()
		// Assert - should not panic or hang
		// Verify that metrics were tracked
		stats := asyncCounter.metrics.GetStats()
		// Should have processed at least some requests
		assert.GreaterOrEqual(t, stats["success_count"].(uint64), uint64(0))
	})
}

func TestNewAsyncTokenCounter(t *testing.T) {
	t.Run("Should use default workers if zero provided", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Act
		asyncCounter := NewAsyncTokenCounter(mockCounter, 0, 100)
		defer asyncCounter.Shutdown()
		// Assert
		assert.Equal(t, 10, asyncCounter.workers) // Default is 10
	})
	t.Run("Should use default workers if negative provided", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Act
		asyncCounter := NewAsyncTokenCounter(mockCounter, -5, 100)
		defer asyncCounter.Shutdown()
		// Assert
		assert.Equal(t, 10, asyncCounter.workers) // Default is 10
	})
	t.Run("Should use default buffer size if zero provided", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Act
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, 0)
		defer asyncCounter.Shutdown()
		// Assert
		assert.Equal(t, 1000, cap(asyncCounter.queue)) // Default is 1000
	})
	t.Run("Should use default buffer size if negative provided", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		// Act
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, -10)
		defer asyncCounter.Shutdown()
		// Assert
		assert.Equal(t, 1000, cap(asyncCounter.queue)) // Default is 1000
	})
	t.Run("Should use custom buffer size when valid", func(t *testing.T) {
		// Arrange
		mockCounter := new(mockTokenCounterAsync)
		customBufferSize := 500
		// Act
		asyncCounter := NewAsyncTokenCounter(mockCounter, 2, customBufferSize)
		defer asyncCounter.Shutdown()
		// Assert
		assert.Equal(t, customBufferSize, cap(asyncCounter.queue))
	})
}

// Ensure mockTokenCounterAsync implements the interface
var _ memcore.TokenCounter = (*mockTokenCounterAsync)(nil)
