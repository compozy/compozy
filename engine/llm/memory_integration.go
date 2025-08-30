package llm

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

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
	assistantResponse llmadapter.Message,
	userMessage llmadapter.Message,
) error {
	if len(memories) == 0 {
		return nil
	}

	log := logger.FromContext(ctx)
	var errors []error

	// Store both user message and assistant response in each memory
	for _, ref := range memoryRefs {
		memory, exists := memories[ref.ID]
		if !exists {
			continue
		}

		// Skip read-only memories
		if ref.Mode == "read-only" {
			log.Debug("Skipping read-only memory", "memory_id", ref.ID)
			continue
		}

		// Append user message
		userMsg := Message{
			Role:    MessageRole(userMessage.Role),
			Content: userMessage.Content,
		}
		if err := memory.Append(ctx, userMsg); err != nil {
			log.Error("Failed to append user message to memory",
				"memory_id", ref.ID,
				"error", err,
			)
			errors = append(errors, fmt.Errorf("failed to append user message to memory %s: %w", ref.ID, err))
			continue
		}

		// Append assistant response
		assistantMsg := Message{
			Role:    MessageRole(assistantResponse.Role),
			Content: assistantResponse.Content,
		}
		if err := memory.Append(ctx, assistantMsg); err != nil {
			log.Error("Failed to append assistant response to memory",
				"memory_id", ref.ID,
				"error", err,
			)
			errors = append(errors, fmt.Errorf("failed to append assistant response to memory %s: %w", ref.ID, err))
			continue
		}

		log.Debug("Messages stored in memory", "memory_id", ref.ID)
	}

	// Return combined errors if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("memory storage errors: %v", errors)
	}

	return nil
}
