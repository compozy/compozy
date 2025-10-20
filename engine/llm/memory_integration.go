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
	for memID, memory := range memories {
		memMessages, err := memory.Read(ctx)
		if err != nil {
			log.Warn("Failed to read messages from memory",
				"memory_id", memID,
				"error", err,
			)
			continue
		}

		for _, msg := range memMessages {
			memoryMessages = append(memoryMessages, llmadapter.Message{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}
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
	if err := validateMemoryInputs(assistantResponse, userMessage); err != nil {
		return err
	}
	if len(memories) == 0 {
		return nil
	}
	batch := buildMemoryBatch(assistantResponse, userMessage)
	var errs []error
	for _, ref := range memoryRefs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("StoreResponseInMemory canceled: %w", err)
		}
		memory, ok := resolveWritableMemory(ctx, memories, ref)
		if !ok {
			continue
		}
		appendMemoryBatch(ctx, memory, ref.ID, batch, &errs)
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory storage errors: %w", errors.Join(errs...))
	}
	return nil
}

func validateMemoryInputs(assistantResponse, userMessage *llmadapter.Message) error {
	if assistantResponse == nil || userMessage == nil {
		return fmt.Errorf(
			"StoreResponseInMemory: nil message pointer(s): assistant=%v user=%v",
			assistantResponse == nil, userMessage == nil,
		)
	}
	return nil
}

func buildMemoryBatch(assistantResponse, userMessage *llmadapter.Message) []Message {
	return []Message{
		{
			Role:    MessageRole(userMessage.Role),
			Content: userMessage.Content,
		},
		{
			Role:    MessageRole(assistantResponse.Role),
			Content: assistantResponse.Content,
		},
	}
}

func resolveWritableMemory(
	ctx context.Context,
	memories map[string]Memory,
	ref core.MemoryReference,
) (Memory, bool) {
	log := logger.FromContext(ctx)
	memory, exists := memories[ref.ID]
	if !exists {
		log.Debug("Memory reference not found; skipping", "memory_id", ref.ID)
		return nil, false
	}
	if ref.Mode == core.MemoryModeReadOnly {
		log.Debug("Skipping read-only memory", "memory_id", ref.ID)
		return nil, false
	}
	return memory, true
}

func appendMemoryBatch(
	ctx context.Context,
	memory Memory,
	memoryID string,
	batch []Message,
	errs *[]error,
) {
	log := logger.FromContext(ctx)
	if err := memory.AppendMany(ctx, batch); err != nil {
		log.Error(
			"Failed to append messages to memory atomically",
			"memory_id", memoryID,
			"error", err,
		)
		*errs = append(*errs, fmt.Errorf("failed to append messages to memory %s: %w", memoryID, err))
		return
	}
	log.Debug("Messages stored atomically in memory", "memory_id", memoryID)
}
