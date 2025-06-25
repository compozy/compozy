package privacy

import (
	"context"
	"fmt"
	"time"

	"github.com/slok/goresilience"
	"github.com/slok/goresilience/circuitbreaker"
	"github.com/slok/goresilience/errors"
	"github.com/slok/goresilience/retry"
	"github.com/slok/goresilience/timeout"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/metrics"
	"github.com/compozy/compozy/pkg/logger"
)

// ResilientManager wraps the privacy manager with resilience patterns
type ResilientManager struct {
	ManagerInterface // Embed interface for flexibility
	runner           goresilience.Runner
	config           *ResilienceConfig
	logger           logger.Logger
}

// ResilienceConfig holds configuration for resilience patterns
type ResilienceConfig struct {
	TimeoutDuration             time.Duration
	ErrorPercentThresholdToOpen int
	MinimumRequestToOpen        int
	WaitDurationInOpenState     time.Duration
	RetryTimes                  int
	RetryWaitBase               time.Duration
}

// DefaultResilienceConfig returns default resilience configuration
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		TimeoutDuration:             100 * time.Millisecond,
		ErrorPercentThresholdToOpen: 50, // Open circuit at 50% error rate
		MinimumRequestToOpen:        10, // Need at least 10 requests to open
		WaitDurationInOpenState:     5 * time.Second,
		RetryTimes:                  3,
		RetryWaitBase:               50 * time.Millisecond,
	}
}

// NewResilientManager creates a new resilient privacy manager
func NewResilientManager(baseManager ManagerInterface, config *ResilienceConfig, log logger.Logger) *ResilientManager {
	if config == nil {
		config = DefaultResilienceConfig()
	}
	if log == nil {
		log = logger.NewForTests() // Use test logger for no-op behavior
	}
	if baseManager == nil {
		baseManager = NewManager()
	}
	// Create circuit breaker middleware
	cbMiddleware := circuitbreaker.NewMiddleware(circuitbreaker.Config{
		ErrorPercentThresholdToOpen:        config.ErrorPercentThresholdToOpen,
		MinimumRequestToOpen:               config.MinimumRequestToOpen,
		SuccessfulRequiredOnHalfOpen:       1,
		WaitDurationInOpenState:            config.WaitDurationInOpenState,
		MetricsSlidingWindowBucketQuantity: 10,
		MetricsBucketDuration:              1 * time.Second,
	})
	// Create timeout middleware
	timeoutMiddleware := timeout.NewMiddleware(timeout.Config{
		Timeout: config.TimeoutDuration,
	})
	// Create retry middleware
	retryMiddleware := retry.NewMiddleware(retry.Config{
		Times:    config.RetryTimes,
		WaitBase: config.RetryWaitBase,
	})
	// Chain the middleware: timeout -> circuit breaker -> retry
	// This order ensures timeouts are enforced first, then circuit breaker, then retry
	runner := goresilience.RunnerChain(
		timeoutMiddleware,
		cbMiddleware,
		retryMiddleware,
	)
	return &ResilientManager{
		ManagerInterface: baseManager,
		runner:           runner,
		config:           config,
		logger:           log,
	}
}

// ApplyPrivacyControls applies privacy controls with resilience patterns
func (rm *ResilientManager) ApplyPrivacyControls(
	ctx context.Context,
	msg llm.Message,
	resourceID string,
	metadata memcore.PrivacyMetadata,
) (llm.Message, memcore.PrivacyMetadata, error) {
	start := time.Now()
	var result struct {
		message  llm.Message
		metadata memcore.PrivacyMetadata
		err      error
	}
	// Execute with resilience patterns
	err := rm.runner.Run(ctx, func(ctx context.Context) (runErr error) {
		// Recover from panics and convert to errors
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("panic recovered: %v", r)
			}
		}()
		// Call original method with resilience wrapper
		msg, meta, err := rm.ManagerInterface.ApplyPrivacyControls(ctx, msg, resourceID, metadata)
		result.message = msg
		result.metadata = meta
		result.err = err
		return err
	})
	// Record metrics
	duration := time.Since(start)
	if err != nil {
		// Record circuit breaker trip if it's a circuit breaker error
		if err == errors.ErrCircuitOpen {
			metrics.RecordCircuitBreakerTrip(ctx, resourceID, "")
		}
		// Resilience patterns failed - use fallback
		return rm.handleResilienceFailure(ctx, msg, resourceID, metadata, err)
	}
	// Now it's safe to access result since runner.Run has completed successfully
	opType := "privacy_apply"
	if result.metadata.RedactionApplied {
		opType = "privacy_redaction"
		// Record redaction operation
		metrics.RecordRedactionOperation(ctx, resourceID, 1, "")
	}
	metrics.RecordMemoryOp(ctx, resourceID, "", opType, duration, 0, result.err)
	return result.message, result.metadata, result.err
}

// RedactContent applies redaction with resilience patterns
func (rm *ResilientManager) RedactContent(content string, patterns []string, defaultRedaction string) (string, error) {
	start := time.Now()
	var result string
	var resultErr error
	// Create a context with timeout for the operation
	ctx, cancel := context.WithTimeout(context.Background(), rm.config.TimeoutDuration*2)
	defer cancel()
	err := rm.runner.Run(ctx, func(_ context.Context) (runErr error) {
		// Recover from panics and convert to errors
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("panic recovered: %v", r)
			}
		}()
		var err error
		result, err = rm.ManagerInterface.RedactContent(content, patterns, defaultRedaction)
		resultErr = err
		return err
	})
	// Record metrics
	duration := time.Since(start)
	if err != nil {
		// Record circuit breaker trip if it's a circuit breaker error
		if err == errors.ErrCircuitOpen {
			metrics.RecordCircuitBreakerTrip(ctx, "", "")
		}
		// If resilience patterns fail, return original content
		rm.logger.Error("Redaction failed with resilience patterns",
			"error", err,
			"fallback", "no_redaction")
		return content, nil
	}
	if resultErr == nil && len(patterns) > 0 {
		// Record successful redaction
		metrics.RecordRedactionOperation(ctx, "", int64(len(patterns)), "")
	}
	metrics.RecordMemoryOp(ctx, "", "", "privacy_redact_content", duration, 0, resultErr)
	return result, resultErr
}

// handleResilienceFailure handles failures when resilience patterns fail
func (rm *ResilientManager) handleResilienceFailure(
	_ context.Context,
	msg llm.Message,
	resourceID string,
	metadata memcore.PrivacyMetadata,
	resilienceErr error,
) (llm.Message, memcore.PrivacyMetadata, error) {
	rm.logger.Error("Privacy controls failed with resilience patterns",
		"resource_id", resourceID,
		"resilience_error", resilienceErr,
		"fallback", "no_redaction")
	// Fallback: pass through without redaction but mark metadata
	metadata.RedactionApplied = false
	metadata.DoNotPersist = true // Safe fallback - don't persist potentially sensitive data
	return msg, metadata, nil
}

// GetCircuitBreakerState returns the current state of the circuit breaker
// This method provides compatibility with the existing interface
func (rm *ResilientManager) GetCircuitBreakerStatus() (isOpen bool, consecutiveErrors int, maxErrors int) {
	// Since goresilience doesn't expose internal state directly,
	// we return a simplified view based on whether the circuit is allowing requests
	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := rm.runner.Run(testCtx, func(_ context.Context) error {
		return nil
	})
	isOpen = err != nil
	// These values are approximations since goresilience manages state internally
	if isOpen {
		return true, rm.config.MinimumRequestToOpen, rm.config.MinimumRequestToOpen
	}
	return false, 0, rm.config.MinimumRequestToOpen
}

// ResetCircuitBreaker is a no-op for goresilience as it manages state internally
func (rm *ResilientManager) ResetCircuitBreaker() {
	// goresilience manages circuit breaker state internally
	// and doesn't provide a direct reset mechanism
	rm.logger.Info("Circuit breaker reset requested - goresilience manages state internally")
}

// UpdateConfig updates the resilience configuration
// Note: This requires recreating the runner
func (rm *ResilientManager) UpdateConfig(config *ResilienceConfig) {
	rm.config = config
	// Recreate the runner with new configuration
	cbMiddleware := circuitbreaker.NewMiddleware(circuitbreaker.Config{
		ErrorPercentThresholdToOpen:        config.ErrorPercentThresholdToOpen,
		MinimumRequestToOpen:               config.MinimumRequestToOpen,
		SuccessfulRequiredOnHalfOpen:       1,
		WaitDurationInOpenState:            config.WaitDurationInOpenState,
		MetricsSlidingWindowBucketQuantity: 10,
		MetricsBucketDuration:              1 * time.Second,
	})
	timeoutMiddleware := timeout.NewMiddleware(timeout.Config{
		Timeout: config.TimeoutDuration,
	})
	retryMiddleware := retry.NewMiddleware(retry.Config{
		Times:    config.RetryTimes,
		WaitBase: config.RetryWaitBase,
	})
	rm.runner = goresilience.RunnerChain(
		timeoutMiddleware,
		cbMiddleware,
		retryMiddleware,
	)
}

// ValidateConfig validates resilience configuration
func ValidateConfig(config *ResilienceConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if config.TimeoutDuration <= 0 {
		return fmt.Errorf("timeout duration must be positive")
	}
	if config.ErrorPercentThresholdToOpen < 0 || config.ErrorPercentThresholdToOpen > 100 {
		return fmt.Errorf("error percent threshold must be between 0 and 100")
	}
	if config.MinimumRequestToOpen <= 0 {
		return fmt.Errorf("minimum request to open must be positive")
	}
	if config.WaitDurationInOpenState <= 0 {
		return fmt.Errorf("wait duration in open state must be positive")
	}
	if config.RetryTimes < 0 {
		return fmt.Errorf("retry times cannot be negative")
	}
	if config.RetryWaitBase < 0 {
		return fmt.Errorf("retry wait base cannot be negative")
	}
	return nil
}
