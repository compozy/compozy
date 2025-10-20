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
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/compozy/compozy/engine/memory/store"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	// DefaultSummarizeThreshold is the default threshold for summarization
	DefaultSummarizeThreshold = 0.8
)

// FlushHandler handles memory flushing operations with retry logic
type FlushHandler struct {
	instanceID        string
	projectID         string // NEW: for namespacing
	store             core.Store
	lockManager       LockManager
	flushingStrategy  core.FlushStrategy
	strategyFactory   *strategies.StrategyFactory // NEW: for dynamic strategy creation
	requestedStrategy string                      // NEW: requested strategy type
	tokenCounter      core.TokenCounter           // NEW: for strategy creation
	metrics           Metrics
	resourceConfig    *core.Resource
}

// PerformFlush executes the complete memory flush operation with retry logic
func (f *FlushHandler) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	start := time.Now()
	result, err := f.runFlushWithRetry(ctx)
	duration := time.Since(start)
	messageCount := 0
	if result != nil {
		messageCount = result.MessageCount
	}
	f.metrics.RecordFlush(ctx, duration, messageCount, err)
	return result, err
}

// executeFlush performs the actual flush operation
func (f *FlushHandler) executeFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
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
	strategy, err := f.selectStrategy()
	if err != nil {
		return nil, fmt.Errorf("failed to select flush strategy: %w", err)
	}
	result, err := strategy.PerformFlush(ctx, messages, f.resourceConfig)
	if err != nil {
		return nil, fmt.Errorf("flushing strategy failed: %w", err)
	}
	if err := f.applyFlushResults(ctx, result, len(messages)); err != nil {
		return nil, fmt.Errorf("failed to apply flush results: %w", err)
	}
	return result, nil
}

// selectStrategy determines which strategy to use for the flush operation
func (f *FlushHandler) selectStrategy() (core.FlushStrategy, error) {
	if f.requestedStrategy != "" && f.strategyFactory != nil {
		strategyConfig := &core.FlushingStrategyConfig{
			Type:               core.FlushingStrategyType(f.requestedStrategy),
			SummarizeThreshold: f.getThreshold(),
		}

		opts := strategies.GetDefaultStrategyOptions()
		if f.resourceConfig != nil && f.resourceConfig.MaxTokens > 0 {
			opts.MaxTokens = f.resourceConfig.MaxTokens
		}

		return f.strategyFactory.CreateStrategy(strategyConfig, opts)
	}
	return f.flushingStrategy, nil
}

func (f *FlushHandler) runFlushWithRetry(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	retryConfig := retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond))
	var (
		result   *core.FlushMemoryActivityOutput
		finalErr error
	)
	err := retry.Do(ctx, retryConfig, func(ctx context.Context) error {
		res, retryable, execErr := f.executeFlushWithLock(ctx)
		if execErr != nil {
			finalErr = execErr
			if retryable {
				return retry.RetryableError(execErr)
			}
			return execErr
		}
		finalErr = nil
		result = res
		return nil
	})
	if err != nil {
		if finalErr != nil {
			return result, finalErr
		}
		return result, err
	}
	return result, nil
}

func (f *FlushHandler) executeFlushWithLock(ctx context.Context) (*core.FlushMemoryActivityOutput, bool, error) {
	log := logger.FromContext(ctx)
	unlock, err := f.lockManager.AcquireFlushLock(ctx, f.instanceID)
	if err != nil {
		if errors.Is(err, core.ErrFlushLockFailed) || errors.Is(err, core.ErrLockAcquisitionFailed) {
			log.Debug("Flush already in progress for instance",
				"instance_id", f.instanceID)
			return nil, false, core.ErrFlushAlreadyPending
		}
		if isTransientError(err) {
			log.Warn("Transient error acquiring flush lock, will retry",
				"error", err,
				"instance_id", f.instanceID)
			return nil, true, err
		}
		return nil, false, fmt.Errorf("failed to acquire flush lock: %w", err)
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			log.Error("Failed to release flush lock",
				"error", unlockErr,
				"instance_id", f.instanceID)
		}
	}()
	result, err := f.executeFlush(ctx)
	if err != nil {
		if isTransientError(err) {
			log.Warn("Transient error during flush, will retry",
				"error", err,
				"instance_id", f.instanceID)
			return nil, true, err
		}
		return nil, false, err
	}
	return result, false, nil
}

// getThreshold returns the summarize threshold from the resource config
func (f *FlushHandler) getThreshold() float64 {
	if f.resourceConfig != nil &&
		f.resourceConfig.FlushingStrategy != nil &&
		f.resourceConfig.FlushingStrategy.SummarizeThreshold > 0 {
		return f.resourceConfig.FlushingStrategy.SummarizeThreshold
	}
	return DefaultSummarizeThreshold
}

// applyFlushResults applies the results of a flush operation
func (f *FlushHandler) applyFlushResults(
	ctx context.Context,
	result *core.FlushMemoryActivityOutput,
	originalMessageCount int,
) error {
	log := logger.FromContext(ctx)
	if !result.Success || result.MessageCount == 0 {
		return nil
	}
	if result.MessageCount >= originalMessageCount {
		return nil
	}
	atomicStore, ok := f.store.(store.AtomicStore)
	if !ok {
		log.Warn("Store doesn't support atomic trim operations",
			"instance_id", f.instanceID,
			"store_type", fmt.Sprintf("%T", f.store))
		return nil
	}
	err := atomicStore.TrimMessagesWithMetadata(ctx, f.instanceID, result.MessageCount, result.TokenCount)
	if err != nil {
		return fmt.Errorf("failed to trim messages after flush for instance %s: %w", f.instanceID, err)
	}
	return nil
}

// isTransientError determines if an error is transient and should be retried
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
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
	if errors.Is(err, redis.Nil) {
		return false // Not found is not transient
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ETIMEDOUT, syscall.ECONNRESET:
			return true
		}
	}
	return false
}
