package instance

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// HealthChecker handles health check operations for memory instances
type HealthChecker struct {
	instanceID   string
	store        core.Store
	lockManager  LockManager
	tokenCounter core.TokenCounter
	operations   *Operations
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(
	instanceID string,
	store core.Store,
	lockManager LockManager,
	tokenCounter core.TokenCounter,
	operations *Operations,
) *HealthChecker {
	return &HealthChecker{
		instanceID:   instanceID,
		store:        store,
		lockManager:  lockManager,
		tokenCounter: tokenCounter,
		operations:   operations,
	}
}

// GetMemoryHealth returns diagnostic information about the memory instance
func (h *HealthChecker) GetMemoryHealth(ctx context.Context) (*core.Health, error) {
	health := &core.Health{}

	// Get message count
	messageCount, err := h.store.GetMessageCount(ctx, h.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}
	health.MessageCount = messageCount

	// Get token count
	tokenCount, err := h.operations.GetTokenCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token count: %w", err)
	}
	health.TokenCount = tokenCount

	// Get flush pending status
	if flushStore, ok := h.store.(core.FlushStateStore); ok {
		isPending, err := flushStore.IsFlushPending(ctx, h.instanceID)
		if err != nil {
			// Log error but continue with default
			isPending = false
		}
		if isPending {
			health.FlushStrategy = "flush_pending"
		} else {
			health.FlushStrategy = "ready"
		}
	}

	return health, nil
}

// PerformHealthCheck performs a comprehensive health check
func (h *HealthChecker) PerformHealthCheck(ctx context.Context) error {
	// Set a timeout for the entire health check
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Check store connectivity
	if err := h.checkStoreConnectivity(ctx); err != nil {
		return fmt.Errorf("store connectivity check failed: %w", err)
	}

	// Check basic operations
	if err := h.checkBasicOperations(ctx); err != nil {
		return fmt.Errorf("basic operations check failed: %w", err)
	}

	// Check lock operations
	if err := h.checkLockOperations(ctx); err != nil {
		return fmt.Errorf("lock operations check failed: %w", err)
	}

	// Check token operations
	if err := h.checkTokenOperations(ctx); err != nil {
		return fmt.Errorf("token operations check failed: %w", err)
	}

	// Check metadata operations
	if err := h.checkMetadataOperations(ctx); err != nil {
		return fmt.Errorf("metadata operations check failed: %w", err)
	}

	return nil
}

// checkStoreConnectivity verifies the store is accessible
func (h *HealthChecker) checkStoreConnectivity(ctx context.Context) error {
	testKey := fmt.Sprintf("%s:health_check", h.instanceID)

	// Try to read a non-existent key
	_, err := h.store.ReadMessages(ctx, testKey)
	if err != nil {
		// Check if it's a "not found" error which is expected
		if err == core.ErrMemoryNotFound {
			return nil
		}
		return err
	}

	return nil
}

// checkBasicOperations tests basic read/write operations
func (h *HealthChecker) checkBasicOperations(ctx context.Context) error {
	testKey := fmt.Sprintf("%s:health_check_basic", h.instanceID)
	testMsg := llm.Message{
		Role:    llm.MessageRoleSystem,
		Content: "health check test message",
	}

	// Write test message
	if err := h.store.AppendMessage(ctx, testKey, testMsg); err != nil {
		return fmt.Errorf("failed to append test message: %w", err)
	}

	// Read test message
	messages, err := h.store.ReadMessages(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to read test message: %w", err)
	}

	if len(messages) != 1 {
		return fmt.Errorf("expected 1 message, got %d", len(messages))
	}

	// Clean up
	if err := h.store.DeleteMessages(ctx, testKey); err != nil {
		return fmt.Errorf("failed to delete test message: %w", err)
	}

	return nil
}

// checkLockOperations tests lock acquisition and release
func (h *HealthChecker) checkLockOperations(ctx context.Context) error {
	testKey := fmt.Sprintf("%s:health_check_lock", h.instanceID)

	// Try to acquire a lock
	unlock, err := h.lockManager.AcquireAppendLock(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to acquire test lock: %w", err)
	}

	// Release the lock
	if err := unlock(); err != nil {
		return fmt.Errorf("failed to release test lock: %w", err)
	}

	return nil
}

// checkTokenOperations tests token counting and metadata
func (h *HealthChecker) checkTokenOperations(ctx context.Context) error {
	testKey := fmt.Sprintf("%s:health_check_tokens", h.instanceID)
	testMsg := llm.Message{
		Role:    llm.MessageRoleSystem,
		Content: "test token counting",
	}

	// Calculate token count
	tokenCount := 0
	if h.tokenCounter != nil {
		count, err := h.tokenCounter.CountTokens(ctx, testMsg.Content)
		if err != nil {
			return fmt.Errorf("failed to count tokens: %w", err)
		}
		tokenCount = count
	}

	// Test atomic append with token count
	if atomicStore, ok := h.store.(core.AtomicOperations); ok {
		if err := atomicStore.AppendMessageWithTokenCount(ctx, testKey, testMsg, tokenCount); err != nil {
			return fmt.Errorf("failed to append with token count: %w", err)
		}

		// Verify token count was saved
		savedCount, err := h.store.GetTokenCount(ctx, testKey)
		if err != nil {
			return fmt.Errorf("failed to get token count: %w", err)
		}

		if savedCount != tokenCount {
			return fmt.Errorf("token count mismatch: expected %d, got %d", tokenCount, savedCount)
		}
	}

	// Clean up
	if err := h.store.DeleteMessages(ctx, testKey); err != nil {
		return fmt.Errorf("failed to clean up test data: %w", err)
	}

	return nil
}

// checkMetadataOperations tests metadata operations
func (h *HealthChecker) checkMetadataOperations(ctx context.Context) error {
	testKey := fmt.Sprintf("%s:health_check_metadata", h.instanceID)

	// Test setting token count
	testTokenCount := 42
	if err := h.store.SetTokenCount(ctx, testKey, testTokenCount); err != nil {
		return fmt.Errorf("failed to set token count: %w", err)
	}

	// Test getting token count
	count, err := h.store.GetTokenCount(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to get token count: %w", err)
	}

	if count != testTokenCount {
		return fmt.Errorf("token count mismatch: expected %d, got %d", testTokenCount, count)
	}

	// Test incrementing token count
	incrementBy := 10
	if err := h.store.IncrementTokenCount(ctx, testKey, incrementBy); err != nil {
		return fmt.Errorf("failed to increment token count: %w", err)
	}

	// Verify increment
	newCount, err := h.store.GetTokenCount(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to get incremented token count: %w", err)
	}

	expectedCount := testTokenCount + incrementBy
	if newCount != expectedCount {
		return fmt.Errorf("incremented count mismatch: expected %d, got %d", expectedCount, newCount)
	}

	return nil
}
