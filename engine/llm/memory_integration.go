package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

// Use shared core memory mode constants to avoid drift.

// PrepareMemoryContext prepares messages with memory context.
// It prepends memory messages to the provided messages slice.
func PrepareMemoryContext(
	ctx context.Context,
	memories map[string]Memory,
	messages []llmadapter.Message,
) []llmadapter.Message {
	if len(memories) == 0 {
		return messages
	}

	log := logger.FromContext(ctx)
	var memoryMessages []llmadapter.Message

	// Read messages from each memory
	for memID, memory := range memories {
		memMessages, err := memory.Read(ctx)
		if err != nil {
			log.Warn("Failed to read messages from memory",
				"memory_id", memID,
				"error", err,
			)
			continue
		}

		// Convert llm.Message to adapter.Message
		for _, msg := range memMessages {
			memoryMessages = append(memoryMessages, llmadapter.Message{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	// Prepend memory messages to maintain chronological order
	if len(memoryMessages) > 0 {
		return append(memoryMessages, messages...)
	}

	return messages
}

// StoreResponseInMemory stores the LLM response in the appropriate memories.
// This is called asynchronously after getting a response from the LLM.
func StoreResponseInMemory(
	ctx context.Context,
	memories map[string]Memory,
	memoryRefs []core.MemoryReference,
	assistantResponse *llmadapter.Message,
	userMessage *llmadapter.Message,
) error {
	if len(memories) == 0 {
		return nil
	}
	log := logger.FromContext(ctx)
	var errs []error
	// Store both user message and assistant response in each memory
	for _, ref := range memoryRefs {
		memory, exists := memories[ref.ID]
		if !exists {
			continue
		}
		// Skip read-only memories
		if ref.Mode == core.MemoryModeReadOnly {
			log.Debug("Skipping read-only memory", "memory_id", ref.ID)
			continue
		}
		// Prepare messages for atomic storage
		userMsg := Message{
			Role:    MessageRole(userMessage.Role),
			Content: userMessage.Content,
		}
		assistantMsg := Message{
			Role:    MessageRole(assistantResponse.Role),
			Content: assistantResponse.Content,
		}
		msgs := []Message{userMsg, assistantMsg}
		// Use atomic AppendMany operation to ensure both messages are stored together
		if err := memory.AppendMany(ctx, msgs); err != nil {
			log.Error("Failed to append messages to memory atomically",
				"memory_id", ref.ID,
				"error", err,
			)
			errs = append(errs, fmt.Errorf("failed to append messages to memory %s: %w", ref.ID, err))
			continue
		}
		log.Debug("Messages stored atomically in memory", "memory_id", ref.ID)
	}
	// Return combined errors if any occurred (preserves causes)
	if len(errs) > 0 {
		return fmt.Errorf("memory storage errors: %w", errors.Join(errs...))
	}
	return nil
}
