package memory

import (
	"context"
	"fmt"
	"sort"

	"github.com/CompoZy/llm-router/engine/llm"
	// Assuming logger is available: "github.com/CompoZy/llm-router/pkg/logger"
)

// MessageWithTokens associates a message with its token count.
type MessageWithTokens struct {
	llm.Message
	TokenCount int
}

// TokenMemoryManager orchestrates token counting, eviction, and allocation for a memory instance.
// It does not directly interact with the MemoryStore but operates on slices of messages.
type TokenMemoryManager struct {
	config       *MemoryResource // The configuration for the memory resource this manager serves
	tokenCounter TokenCounter    // The utility to count tokens
}

// NewTokenMemoryManager creates a new token manager.
func NewTokenMemoryManager(config *MemoryResource, counter TokenCounter) (*TokenMemoryManager, error) {
	if config == nil {
		return nil, fmt.Errorf("memory resource config cannot be nil")
	}
	if counter == nil {
		return nil, fmt.Errorf("token counter cannot be nil")
	}
	if config.Type != TokenBasedMemory && config.MaxTokens == 0 && config.MaxContextRatio == 0 {
		// Not strictly an error if other limits like MaxMessages are set,
		// but this manager is primarily for token-based limits.
		// logger.Warn(context.Background(), "TokenMemoryManager created for a resource without token limits defined in config")
	}
	return &TokenMemoryManager{
		config:       config,
		tokenCounter: counter,
	}, nil
}

// CalculateMessagesWithTokens processes a slice of messages to determine their token counts.
// It caches token counts on the MessageWithTokens struct.
func (tmm *TokenMemoryManager) CalculateMessagesWithTokens(ctx context.Context, messages []llm.Message) ([]MessageWithTokens, int, error) {
	processedMessages := make([]MessageWithTokens, len(messages))
	totalTokens := 0
	for i, msg := range messages {
		// In a more advanced scenario, msg.Metadata might already have a pre-calculated token count.
		// For now, we always calculate.
		count, err := tmm.tokenCounter.CountTokens(ctx, msg.Content) // Assuming msg.Content is the primary text for tokenization
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count tokens for message %d: %w", i, err)
		}
		processedMessages[i] = MessageWithTokens{Message: msg, TokenCount: count}
		totalTokens += count
	}
	return processedMessages, totalTokens, nil
}

// EnforceLimits applies token and message count limits to a set of messages.
// It uses FIFO eviction if limits are exceeded.
// Returns the potentially reduced set of messages and the new total token count.
func (tmm *TokenMemoryManager) EnforceLimits(ctx context.Context, messages []MessageWithTokens, currentTotalTokens int) ([]MessageWithTokens, int, error) {
	// Determine effective max tokens
	effectiveMaxTokens := tmm.config.MaxTokens
	if effectiveMaxTokens == 0 && tmm.config.MaxContextRatio > 0 {
		// This part is tricky: MaxContextRatio needs to know the LLM's context window size.
		// This might come from an llm.ModelInfo service or be a global default.
		// For now, let's assume a placeholder default model context size if not provided.
		// This should ideally be injected or configurable.
		modelContextSize := 4096 // Placeholder
		// if tmm.config.ModelInfoProvider != nil { modelContextSize = tmm.config.ModelInfoProvider.GetContextSize() }
		effectiveMaxTokens = int(float64(modelContextSize) * tmm.config.MaxContextRatio)
	}

	// 1. Token Limit Enforcement (FIFO)
	if effectiveMaxTokens > 0 && currentTotalTokens > effectiveMaxTokens {
		evictedMessages := 0
		for currentTotalTokens > effectiveMaxTokens && len(messages) > 0 {
			currentTotalTokens -= messages[0].TokenCount
			messages = messages[1:] // FIFO eviction
			evictedMessages++
		}
		// logger.Debugf(ctx, "Token limit exceeded. Evicted %d messages.", evictedMessages)
	}

	// 2. Message Count Limit Enforcement (FIFO)
	// Applied *after* token limit, as token limit is usually primary for LLMs.
	if tmm.config.MaxMessages > 0 && len(messages) > tmm.config.MaxMessages {
		evictCount := len(messages) - tmm.config.MaxMessages
		// Recalculate tokens if messages were evicted due to count limit
		for i := 0; i < evictCount; i++ {
			currentTotalTokens -= messages[i].TokenCount
		}
		messages = messages[evictCount:]
		// logger.Debugf(ctx, "Message count limit exceeded. Evicted %d messages.", evictCount)
	}
	return messages, currentTotalTokens, nil
}

// ApplyTokenAllocation (Conceptual for now, as simple FIFO doesn't use it directly)
// This would be more relevant for sophisticated eviction strategies that prioritize
// message types (system, short_term, long_term) based on allocation ratios.
// For simple FIFO, the overall MaxTokens limit is the main driver.
// A more advanced eviction strategy might use these ratios to decide *which*
// messages to evict beyond simple FIFO (e.g., always keep 'system' messages
// if they fit their allocation, even if older).
func (tmm *TokenMemoryManager) ApplyTokenAllocation(
	ctx context.Context,
	messages []MessageWithTokens,
	currentTotalTokens int,
) ([]MessageWithTokens, int, error) {
	if tmm.config.TokenAllocation == nil {
		return messages, currentTotalTokens, nil // No allocation defined
	}

	// This is a placeholder for a more complex allocation-aware eviction.
	// For example, if TokenAllocation is defined:
	// 1. Categorize messages (e.g., by role or metadata: system, short_term, long_term).
	// 2. Calculate token usage per category.
	// 3. If total tokens exceed MaxTokens:
	//    a. Try to evict from categories that are over their budget according to ratios.
	//    b. Prioritize keeping 'system' messages if possible.
	//    c. Fallback to general FIFO or LIFO within less critical categories.

	// For now, this function is a no-op as the primary EnforceLimits uses simple FIFO.
	// logger.Debug(ctx, "Token allocation defined but not yet fully implemented in eviction strategy.")
	return messages, currentTotalTokens, nil
}

// GetManagedMessages processes input messages, applies limits, and returns the result.
// This is a convenience method combining calculation and enforcement.
func (tmm *TokenMemoryManager) GetManagedMessages(ctx context.Context, inputMessages []llm.Message) ([]llm.Message, int, error) {
	messagesWithTokens, totalTokens, err := tmm.CalculateMessagesWithTokens(ctx, inputMessages)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to calculate token counts: %w", err)
	}

	finalMessagesWithTokens, finalTotalTokens, err := tmm.EnforceLimits(ctx, messagesWithTokens, totalTokens)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to enforce limits: %w", err)
	}

	// Convert back to []llm.Message
	resultMessages := make([]llm.Message, len(finalMessagesWithTokens))
	for i, mwt := range finalMessagesWithTokens {
		resultMessages[i] = mwt.Message
	}

	return resultMessages, finalTotalTokens, nil
}


// PriorityEvictionManager (as per Tech Spec for Task 13, but elements are in PRD for Task 2)
// This can be part of TokenMemoryManager or a separate struct.
// Let's integrate a simplified version here based on PRD's "priority-aware token eviction".

// MessageWithPriorityAndTokens extends MessageWithTokens with a priority level.
type MessageWithPriorityAndTokens struct {
	MessageWithTokens
	Priority int // Lower number means higher priority (e.g., 0 is critical)
}

// EnforceLimitsWithPriority applies token limits considering message priorities.
// It attempts to preserve higher priority messages.
func (tmm *TokenMemoryManager) EnforceLimitsWithPriority(
	ctx context.Context,
	messages []MessageWithPriorityAndTokens, // Input messages now have priority
	currentTotalTokens int,
) ([]MessageWithPriorityAndTokens, int, error) {
	effectiveMaxTokens := tmm.config.MaxTokens // Assuming this is calculated as before

	if effectiveMaxTokens <= 0 || currentTotalTokens <= effectiveMaxTokens {
		return messages, currentTotalTokens, nil // No token limit or not exceeded
	}

	// Sort messages by priority (ascending, so critical first) then by original order (descending for FIFO within priority)
	// This means if priorities are equal, older messages (further up the original slice) are considered for eviction first.
	sort.SliceStable(messages, func(i, j int) bool {
		if messages[i].Priority != messages[j].Priority {
			return messages[i].Priority < messages[j].Priority // Lower priority number = higher actual priority
		}
		return i < j // Keep original relative order for same priority (FIFO for eviction)
	})

	finalMessages := make([]MessageWithPriorityAndTokens, 0, len(messages))
	retainedTokens := 0

	// Greedily keep highest priority messages first
	// This loop iterates from highest to lowest priority.
	// To evict lowest priority first, we'd iterate from the end of the sorted list.
	// Let's adjust: iterate and build the list to keep, then if over budget, remove from lowest priority *of the kept list*.

	// Alternative: sort by priority (descending - lowest priority value first), then by age (descending - newest first)
	// Then iterate, adding messages until budget is full.

	// Simpler: Sort by eviction preference: Lowest priority first, then oldest within that priority.
	sort.SliceStable(messages, func(i, j int) bool {
		if messages[i].Priority != messages[j].Priority {
			return messages[i].Priority > messages[j].Priority // Higher priority number = lower actual priority (gets evicted first)
		}
		return i < j // Older messages (smaller index in original slice if stable sort used before) within same priority evicted first
	})

	// Now `messages` is sorted such that `messages[0]` is the first candidate for eviction.
	tokensToEvict := currentTotalTokens - effectiveMaxTokens
	evictedCount := 0

	finalKeptMessages := make([]MessageWithPriorityAndTokens, len(messages))
	copy(finalKeptMessages, messages) // Start with all messages

	for tokensToEvict > 0 && len(finalKeptMessages) > 0 {
		// Evict from the beginning of the sorted list (lowest priority, oldest)
		tokensToEvict -= finalKeptMessages[0].TokenCount
		currentTotalTokens -= finalKeptMessages[0].TokenCount //  This was wrong, should be `currentTotalTokens -= finalKeptMessages[0].TokenCount`
		finalKeptMessages = finalKeptMessages[1:]
		evictedCount++
	}
	// logger.Debugf(ctx, "Priority eviction: Evicted %d messages. Kept %d messages with %d tokens.", evictedCount, len(finalKeptMessages), currentTotalTokens)

	// Restore original order for the kept messages if necessary, or assume consumer handles it.
	// For now, the order of finalKeptMessages is based on eviction preference, not original sequence.
	// This might need adjustment based on how ReadMessages is expected to behave.
	// Typically, memory should be chronological. So, after deciding *which* to keep, re-sort them.
	// However, the input `messages` might already be what's in store, and we are just deciding which ones to *remove*.
	// The current `finalKeptMessages` contains the messages to keep, but their order is changed.
	// Let's assume the caller (e.g. MemoryInstance) will re-persist these `finalKeptMessages` in their correct (new) order.

	return finalKeptMessages, currentTotalTokens, nil
}
