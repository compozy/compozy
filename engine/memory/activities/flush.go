package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/CompoZy/llm-router/engine/memory" // Adjust path as needed
	// Assuming logger: "github.com/CompoZy/llm-router/pkg/logger"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// MemoryActivities encapsulates Temporal activities related to memory management.
type MemoryActivities struct {
	// Dependencies needed for flushing, e.g., access to a MemoryManager
	// or directly to MemoryStore, TokenMemoryManager, FlushingStrategy instances.
	// For this example, let's assume it needs a way to get/reconstruct these.
	// A more robust approach would be to inject interfaces or a factory.
	// For now, let's assume it can get a MemoryInstance or its components.
	// This is a simplification; real implementation needs careful dependency injection.
	MemoryStore    memory.MemoryStore
	LockManager    memory.MemoryLockManager // Using the wrapper from Task 1
	TokenManager   *memory.TokenMemoryManager // Assuming we can construct/get this
	FlushStrategy  *memory.HybridFlushingStrategy // Assuming we can construct/get this
	ResourceConfig *memory.MemoryResource // The config for the specific memory being flushed
}

// NewMemoryActivities creates a new instance of MemoryActivities.
// Dependencies would be injected here. This is a simplified constructor.
func NewMemoryActivities(
	store memory.MemoryStore,
	lockMgr memory.MemoryLockManager,
	tokenMgr *memory.TokenMemoryManager, // Or a factory to get one by resource ID
	strategy *memory.HybridFlushingStrategy, // Or a factory
	resourceCfg *memory.MemoryResource,
) *MemoryActivities {
	return &MemoryActivities{
		MemoryStore:    store,
		LockManager:    lockMgr,
		TokenManager:   tokenMgr,
		FlushStrategy:  strategy,
		ResourceConfig: resourceCfg,
	}
}

// FlushMemoryActivityInput defines the input for the FlushMemory activity.
type FlushMemoryActivityInput struct {
	MemoryInstanceKey string                 // The specific key of the memory instance to flush (e.g., "user123:sessionABC")
	ResourceID        string                 // The ID of the MemoryResource configuration being used
	// Potentially include current token/message counts if known by caller to optimize
}

// FlushMemoryActivityOutput defines the output for the FlushMemory activity.
type FlushMemoryActivityOutput struct {
	MessagesKept    int
	TokensKept      int
	SummaryGenerated bool
	Error           string `json:",omitempty"`
}

// FlushMemory is a Temporal activity that performs a memory flush operation.
// It's designed to be called asynchronously.
func (ma *MemoryActivities) FlushMemory(ctx context.Context, input FlushMemoryActivityInput) (*FlushMemoryActivityOutput, error) {
	log := activity.GetLogger(ctx) // Use Temporal's logger for activities
	log.Info("FlushMemory activity started", "MemoryKey", input.MemoryInstanceKey, "ResourceID", input.ResourceID)

	// 1. Acquire distributed lock for this memory instance key to prevent concurrent flushes/writes.
	// The lock TTL should be long enough for the flush operation but not indefinite.
	lockTTL := 5 * time.Minute // Example: Make this configurable
	lock, err := ma.LockManager.Acquire(ctx, input.MemoryInstanceKey, lockTTL)
	if err != nil {
		log.Error("Failed to acquire lock for flushing", "MemoryKey", input.MemoryInstanceKey, "Error", err)
		// Non-retryable if lock acquisition itself is the problem and indicates contention or system issue
		return nil, temporal.NewApplicationError("failed to acquire lock for flush", "LOCK_ACQUISITION_FAILED", err)
	}
	defer func() {
		if err := lock.Release(context.Background()); err != nil { // Use a background context for release
			log.Error("Failed to release lock after flushing", "MemoryKey", input.MemoryInstanceKey, "Error", err)
		}
	}()
	log.Info("Lock acquired for flushing", "MemoryKey", input.MemoryInstanceKey)

	// Heartbeat to Temporal server to indicate activity is alive, especially if flushing can be long.
	activity.RecordHeartbeat(ctx, "Lock acquired, starting flush process")

	// 2. Read current messages from MemoryStore
	currentRawMessages, err := ma.MemoryStore.ReadMessages(ctx, input.MemoryInstanceKey)
	if err != nil {
		log.Error("Failed to read messages for flushing", "MemoryKey", input.MemoryInstanceKey, "Error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("failed to read messages: %s", err.Error()), "READ_FAILED")
	}

	if len(currentRawMessages) == 0 {
		log.Info("No messages to flush", "MemoryKey", input.MemoryInstanceKey)
		return &FlushMemoryActivityOutput{MessagesKept: 0, TokensKept: 0}, nil
	}

	// Dependencies for TokenManager and HybridFlushingStrategy
	// In a real app, these might be fetched based on input.ResourceID or pre-configured
	// For this example, we assume they are available on `ma` (MemoryActivities struct).
	// This implies that the worker hosting this activity is configured for a specific MemoryResource,
	// or can dynamically construct these.
	if ma.TokenManager == nil || ma.FlushStrategy == nil || ma.ResourceConfig == nil {
		log.Error("Activity dependencies (TokenManager, FlushStrategy, ResourceConfig) are not initialized.")
		return nil, temporal.NewApplicationError("activity misconfigured", "ACTIVITY_DEPENDENCY_NIL")
	}
	// Potentially re-configure them if they are stateful and depend on input.ResourceID
	// ma.TokenManager.SetConfig(ma.ResourceConfig) // If such a method exists

	// 3. Process messages with TokenManager to get token counts
	currentMessagesWithTokens, currentTotalTokens, err := ma.TokenManager.CalculateMessagesWithTokens(ctx, currentRawMessages)
	if err != nil {
		log.Error("Failed to calculate tokens during flush", "MemoryKey", input.MemoryInstanceKey, "Error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("token calculation failed: %s", err.Error()), "TOKEN_CALC_FAILED")
	}
	log.Info("Calculated initial tokens", "MemoryKey", input.MemoryInstanceKey, "Messages", len(currentMessagesWithTokens), "Tokens", currentTotalTokens)

	activity.RecordHeartbeat(ctx, fmt.Sprintf("Read %d messages, %d tokens. Checking flush condition.", len(currentMessagesWithTokens), currentTotalTokens))

	// 4. Check if flushing is actually needed (could have changed since activity was scheduled)
	if !ma.FlushStrategy.ShouldFlush(ctx, currentMessagesWithTokens, currentTotalTokens) {
		log.Info("Flush condition not met, no flush performed.", "MemoryKey", input.MemoryInstanceKey, "Tokens", currentTotalTokens)
		// Still, apply general limits just in case something is over budget without meeting specific flush *thresholds*.
		finalLimitedMessages, finalLimitedTokens, limErr := ma.TokenManager.EnforceLimits(ctx, currentMessagesWithTokens, currentTotalTokens)
		if limErr != nil {
			log.Error("Error enforcing limits even when flush threshold not met", "Error", limErr)
			// Decide if this is fatal for the activity
		}
		if len(finalLimitedMessages) != len(currentMessagesWithTokens) { // If EnforceLimits did something
			if err := ma.MemoryStore.ReplaceMessages(ctx, input.MemoryInstanceKey, memory.MessagesWithTokensToLLMMessages(finalLimitedMessages)); err != nil {
				log.Error("Failed to save messages after limit enforcement (no flush)", "MemoryKey", input.MemoryInstanceKey, "Error", err)
				return nil, temporal.NewApplicationError(fmt.Sprintf("failed to save messages post-limit (no flush): %s", err.Error()), "SAVE_POST_LIMIT_FAILED")
			}
			log.Info("Applied general limits", "MemoryKey", input.MemoryInstanceKey, "MessagesKept", len(finalLimitedMessages), "TokensKept", finalLimitedTokens)
			return &FlushMemoryActivityOutput{MessagesKept: len(finalLimitedMessages), TokensKept: finalLimitedTokens}, nil
		}
		return &FlushMemoryActivityOutput{MessagesKept: len(currentMessagesWithTokens), TokensKept: currentTotalTokens}, nil
	}
	log.Info("Flush condition met, proceeding with flush.", "MemoryKey", input.MemoryInstanceKey)

	// 5. Perform the flush using HybridFlushingStrategy
	newMessagesWithTokens, newTotalTokens, summaryGenerated, flushErr := ma.FlushStrategy.FlushMessages(ctx, currentMessagesWithTokens)
	if flushErr != nil {
		log.Error("Failed to flush messages", "MemoryKey", input.MemoryInstanceKey, "Error", flushErr)
		return nil, temporal.NewApplicationError(fmt.Sprintf("flushing failed: %s", flushErr.Error()), "FLUSH_EXECUTION_FAILED")
	}
	log.Info("Flush executed", "MemoryKey", input.MemoryInstanceKey, "MessagesAfterFlush", len(newMessagesWithTokens), "TokensAfterFlush", newTotalTokens, "SummaryGenerated", summaryGenerated)

	activity.RecordHeartbeat(ctx, fmt.Sprintf("Flush complete. New state: %d messages, %d tokens.", len(newMessagesWithTokens), newTotalTokens))

	// 6. (Re-)Apply general limits with TokenMemoryManager on the new set of messages.
	// This ensures that even after summarization, the result fits MaxTokens/MaxMessages.
	// The summary message itself has tokens and counts towards the limits.
	finalMessagesAfterLimits, finalTokensAfterLimits, limitErr := ma.TokenManager.EnforceLimits(ctx, newMessagesWithTokens, newTotalTokens)
	if limitErr != nil {
		log.Error("Failed to enforce limits after flushing", "MemoryKey", input.MemoryInstanceKey, "Error", limitErr)
		return nil, temporal.NewApplicationError(fmt.Sprintf("limit enforcement post-flush failed: %s", limitErr.Error()), "LIMIT_POST_FLUSH_FAILED")
	}
	log.Info("Limits enforced post-flush", "MemoryKey", input.MemoryInstanceKey, "MessagesAfterLimits", len(finalMessagesAfterLimits), "TokensAfterLimits", finalTokensAfterLimits)

	// 7. Write the new state back to MemoryStore
	finalLLMMessages := memory.MessagesWithTokensToLLMMessages(finalMessagesAfterLimits)
	if err := ma.MemoryStore.ReplaceMessages(ctx, input.MemoryInstanceKey, finalLLMMessages); err != nil {
		log.Error("Failed to save messages after flushing", "MemoryKey", input.MemoryInstanceKey, "Error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("failed to save messages post-flush: %s", err.Error()), "SAVE_POST_FLUSH_FAILED")
	}

	log.Info("FlushMemory activity completed successfully", "MemoryKey", input.MemoryInstanceKey, "MessagesKept", len(finalMessagesAfterLimits), "TokensKept", finalTokensAfterLimits)
	return &FlushMemoryActivityOutput{
		MessagesKept:    len(finalMessagesAfterLimits),
		TokensKept:      finalTokensAfterLimits,
		SummaryGenerated: summaryGenerated,
	}, nil
}

// Helper function (should be in memory package or a common place)
// func MessagesWithTokensToLLMMessages(mwt []MessageWithTokens) []llm.Message { ... }
// For now, it's assumed to exist in the memory package.
// Add it to memory/types.go or a new memory/utils.go if it doesn't.
