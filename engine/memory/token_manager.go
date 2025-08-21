package memory

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// TokenMemoryManager orchestrates token counting, eviction, and allocation for a memory instance.
// It does not directly interact with the MemoryStore but operates on slices of messages.
type TokenMemoryManager struct {
	config       *memcore.Resource    // The configuration for the memory resource this manager serves
	tokenCounter memcore.TokenCounter // The utility to count tokens
}

// NewTokenMemoryManager creates a new token manager.
func NewTokenMemoryManager(
	ctx context.Context,
	config *memcore.Resource,
	counter memcore.TokenCounter,
) (*TokenMemoryManager, error) {
	log := logger.FromContext(ctx)
	if config == nil {
		return nil, fmt.Errorf("memory resource config cannot be nil")
	}
	if counter == nil {
		return nil, fmt.Errorf("token counter cannot be nil")
	}
	// Validate token limits configuration
	if config.MaxTokens == 0 && config.MaxContextRatio == 0 {
		// Log warning about potential misconfiguration
		// This memory will effectively have no token limits
		log.Warn(
			"Memory resource has no token limits configured (MaxTokens=0, MaxContextRatio=0). "+
				"This may lead to unbounded memory growth.",
			"resource_id", config.ID,
		)
	}
	return &TokenMemoryManager{
		config:       config,
		tokenCounter: counter,
	}, nil
}

// GetTokenCounter returns the core token counter for dependency injection
func (tmm *TokenMemoryManager) GetTokenCounter() memcore.TokenCounter {
	return tmm.tokenCounter
}

// CalculateMessagesWithTokens processes a slice of messages to determine their token counts.
// It caches token counts on the MessageWithTokens struct.
func (tmm *TokenMemoryManager) CalculateMessagesWithTokens(
	ctx context.Context,
	messages []llm.Message,
) ([]memcore.MessageWithTokens, int, error) {
	processedMessages := make([]memcore.MessageWithTokens, len(messages))
	totalTokens := 0
	for i, msg := range messages {
		// In a more advanced scenario, msg.Metadata might already have a pre-calculated token count.
		// For now, we always calculate.
		count, err := tmm.tokenCounter.CountTokens(
			ctx,
			msg.Content,
		) // Assuming msg.Content is the primary text for tokenization
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count tokens for message %d: %w", i, err)
		}
		processedMessages[i] = memcore.MessageWithTokens{Message: msg, TokenCount: count}
		totalTokens += count
	}
	return processedMessages, totalTokens, nil
}

// EnforceLimits applies token and message count limits to a set of messages.
// It uses FIFO eviction if limits are exceeded.
// Returns the potentially reduced set of messages and the new total token count.
func (tmm *TokenMemoryManager) EnforceLimits(
	_ context.Context,
	messages []memcore.MessageWithTokens,
	currentTotalTokens int,
) ([]memcore.MessageWithTokens, int, error) {
	// Determine effective max tokens
	effectiveMaxTokens := tmm.config.MaxTokens
	if effectiveMaxTokens == 0 && tmm.config.MaxContextRatio > 0 {
		// Use ModelContextSize from config if available, otherwise use default
		modelContextSize := tmm.config.ModelContextSize
		if modelContextSize == 0 {
			modelContextSize = 4096 // Default fallback
		}
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
	}

	// 2. Message Count Limit Enforcement (FIFO)
	// Applied *after* token limit, as token limit is usually primary for LLMs.
	if tmm.config.MaxMessages > 0 && len(messages) > tmm.config.MaxMessages {
		evictCount := len(messages) - tmm.config.MaxMessages
		// Recalculate tokens if messages were evicted due to count limit
		for i := range evictCount {
			currentTotalTokens -= messages[i].TokenCount
		}
		messages = messages[evictCount:]
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
	_ context.Context,
	messages []memcore.MessageWithTokens,
	currentTotalTokens int,
) ([]memcore.MessageWithTokens, int, error) {
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
func (tmm *TokenMemoryManager) GetManagedMessages(
	ctx context.Context,
	inputMessages []llm.Message,
) ([]llm.Message, int, error) {
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
		if msg, ok := mwt.Message.(llm.Message); ok {
			resultMessages[i] = msg
		} else {
			// Handle unexpected type - should not happen if properly structured
			resultMessages[i] = llm.Message{Role: "system", Content: "Invalid message type"}
		}
	}

	return resultMessages, finalTotalTokens, nil
}

// PriorityEvictionManager (as per Tech Spec for Task 13, but elements are in PRD for Task 2)
// This can be part of TokenMemoryManager or a separate struct.
// Let's integrate a simplified version here based on PRD's "priority-aware token eviction".

// MessageWithPriorityAndTokens extends MessageWithTokens with a priority level.
type MessageWithPriorityAndTokens struct {
	memcore.MessageWithTokens
	Priority      int // Lower number means higher priority (e.g., 0 is critical)
	OriginalIndex int // Track original position for order restoration (-1 if unset)
}

// EnforceLimitsWithPriority applies token limits considering message priorities.
// It attempts to preserve higher priority messages while maintaining chronological order.
func (tmm *TokenMemoryManager) EnforceLimitsWithPriority(
	_ context.Context,
	messages []MessageWithPriorityAndTokens, // Input messages now have priority
	currentTotalTokens int,
) ([]MessageWithPriorityAndTokens, int, error) {
	effectiveMaxTokens := tmm.config.MaxTokens // Assuming this is calculated as before

	if effectiveMaxTokens <= 0 || currentTotalTokens <= effectiveMaxTokens {
		return messages, currentTotalTokens, nil // No token limit or not exceeded
	}

	// Set original indices if not already set
	for i := range messages {
		if messages[i].OriginalIndex == -1 {
			messages[i].OriginalIndex = i
		}
	}

	// Create a copy to avoid modifying the input slice
	messagesToSort := make([]MessageWithPriorityAndTokens, len(messages))
	copy(messagesToSort, messages)

	// Sort by eviction preference: Lowest priority first, then oldest within that priority.
	sort.SliceStable(messagesToSort, func(i, j int) bool {
		if messagesToSort[i].Priority != messagesToSort[j].Priority {
			// Higher priority number = lower actual priority (gets evicted first)
			return messagesToSort[i].Priority > messagesToSort[j].Priority
		}
		// Older messages evicted first within same priority
		return messagesToSort[i].OriginalIndex < messagesToSort[j].OriginalIndex
	})

	// Now `messagesToSort` is sorted such that `messagesToSort[0]` is the first candidate for eviction.
	tokensToEvict := currentTotalTokens - effectiveMaxTokens
	evictedCount := 0

	// Create a map to track which messages to keep
	keptIndices := make(map[int]bool)
	for i := range messages {
		keptIndices[i] = true // Start with all messages kept
	}

	// Evict messages starting from the lowest priority
	for i := 0; i < len(messagesToSort) && tokensToEvict > 0; i++ {
		msg := messagesToSort[i]
		tokensToEvict -= msg.TokenCount
		currentTotalTokens -= msg.TokenCount
		delete(keptIndices, msg.OriginalIndex)
		evictedCount++
	}

	// Reconstruct the kept messages in their original chronological order
	finalKeptMessages := make([]MessageWithPriorityAndTokens, 0, len(messages)-evictedCount)
	for i, msg := range messages {
		if keptIndices[i] {
			finalKeptMessages = append(finalKeptMessages, msg)
		}
	}

	return finalKeptMessages, currentTotalTokens, nil
}
