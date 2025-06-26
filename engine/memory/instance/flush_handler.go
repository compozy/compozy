package instance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sethvargo/go-retry"

	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
)

// FlushHandler handles memory flushing operations with retry logic
type FlushHandler struct {
	instanceID       string
	store            core.Store
	lockManager      LockManager
	flushingStrategy FlushStrategy
	logger           logger.Logger
	metrics          Metrics
	resourceConfig   *core.Resource
}

// PerformFlush executes the complete memory flush operation with retry logic
func (f *FlushHandler) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	start := time.Now()
	var result *core.FlushMemoryActivityOutput
	var finalErr error

	// Retry logic with exponential backoff for transient failures
	retryConfig := retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond))

	err := retry.Do(ctx, retryConfig, func(ctx context.Context) error {
		// Acquire flush lock
		unlock, err := f.lockManager.AcquireFlushLock(ctx, f.instanceID)
		if err != nil {
			// Check if it's a lock contention error
			if errors.Is(err, core.ErrFlushLockFailed) || errors.Is(err, core.ErrLockAcquisitionFailed) {
				f.logger.Debug("Flush already in progress for instance",
					"instance_id", f.instanceID)
				return core.ErrFlushAlreadyPending
			}
			if isTransientError(err) {
				f.logger.Warn("Transient error acquiring flush lock, will retry",
					"error", err,
					"instance_id", f.instanceID)
				return retry.RetryableError(err)
			}
			return fmt.Errorf("failed to acquire flush lock: %w", err)
		}
		defer func() {
			if err := unlock(); err != nil {
				f.logger.Error("Failed to release flush lock",
					"error", err,
					"instance_id", f.instanceID)
			}
		}()

		// Execute the flush directly
		result, err = f.executeFlush(ctx)
		if err != nil {
			if isTransientError(err) {
				f.logger.Warn("Transient error during flush, will retry",
					"error", err,
					"instance_id", f.instanceID)
				return retry.RetryableError(err)
			}
			finalErr = err
			return err
		}

		finalErr = nil
		return nil
	})

	// Record metrics
	if result != nil {
		f.metrics.RecordFlush(ctx, time.Since(start), result.MessageCount, finalErr)
	} else {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, finalErr)
	}

	return result, err
}

// executeFlush performs the actual flush operation
func (f *FlushHandler) executeFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	// Read all messages
	messages, err := f.store.ReadMessages(ctx, f.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages for flush: %w", err)
	}

	if len(messages) == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			MessageCount:     0,
			TokenCount:       0,
			SummaryGenerated: false,
		}, nil
	}

	// Execute flushing strategy
	result, err := f.flushingStrategy.PerformFlush(ctx, messages, f.resourceConfig)
	if err != nil {
		return nil, fmt.Errorf("flushing strategy failed: %w", err)
	}

	// Apply the flush results
	if err := f.applyFlushResults(ctx, result, len(messages)); err != nil {
		return nil, fmt.Errorf("failed to apply flush results: %w", err)
	}

	return result, nil
}

// applyFlushResults applies the results of a flush operation
func (f *FlushHandler) applyFlushResults(
	ctx context.Context,
	result *core.FlushMemoryActivityOutput,
	originalMessageCount int,
) error {
	// If no messages were flushed, nothing to do
	if !result.Success || result.MessageCount == 0 {
		return nil
	}

	// If all messages were flushed, we've already handled this in the strategy
	if result.MessageCount >= originalMessageCount {
		return nil
	}

	// Check if store supports atomic operations
	atomicStore, ok := f.store.(store.AtomicStore)
	if !ok {
		f.logger.Warn("Store doesn't support atomic trim operations", "instance_id", f.instanceID)
		return nil
	}

	// Trim messages to keep only the remaining ones
	err := atomicStore.TrimMessagesWithMetadata(ctx, f.instanceID, result.MessageCount, result.TokenCount)
	if err != nil {
		return fmt.Errorf("failed to trim messages after flush: %w", err)
	}

	return nil
}

// isTransientError determines if an error is transient and should be retried
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Common transient error patterns
	transientPatterns := []string{
		"connection refused",
		"timeout",
		"connection reset",
		"temporary failure",
		"network is unreachable",
		"lock timeout",
	}

	errStr := strings.ToLower(err.Error())
	for _, pattern := range transientPatterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	// Redis-specific errors
	if errors.Is(err, redis.Nil) {
		return false // Not found is not transient
	}

	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Syscall errors that are transient
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ETIMEDOUT, syscall.ECONNRESET:
			return true
		}
	}

	return false
}
