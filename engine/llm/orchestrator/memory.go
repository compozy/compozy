package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/compozy/compozy/pkg/logger"
)

type MemoryContext struct {
	memories   map[string]contracts.Memory
	references []core.MemoryReference
}

type MemoryManager interface {
	Prepare(ctx context.Context, request Request) *MemoryContext
	Inject(ctx context.Context, base []llmadapter.Message, ctxData *MemoryContext) []llmadapter.Message
	StoreAsync(
		ctx context.Context,
		ctxData *MemoryContext,
		response *llmadapter.LLMResponse,
		messages []llmadapter.Message,
		request Request,
	)
}

type memoryManager struct {
	provider contracts.MemoryProvider
	sync     MemorySync
	hook     AsyncHook
}

func NewMemoryManager(provider contracts.MemoryProvider, sync MemorySync, hook AsyncHook) MemoryManager {
	return &memoryManager{provider: provider, sync: sync, hook: hook}
}

func (m *memoryManager) Prepare(ctx context.Context, request Request) *MemoryContext {
	log := logger.FromContext(ctx)
	if m.provider == nil {
		log.Debug("No memory provider available")
		return nil
	}

	references := request.Agent.Memory
	if len(references) == 0 {
		log.Debug("No memory references configured for agent", "agent_id", request.Agent.ID)
		return nil
	}

	memories := make(map[string]contracts.Memory)
	for _, ref := range references {
		memory, err := m.provider.GetMemory(ctx, ref.ID, ref.Key)
		if err != nil {
			log.Error("Failed to get memory instance", "memory_id", ref.ID, "error", core.RedactError(err))
			continue
		}
		if memory == nil {
			log.Warn("Memory instance is nil", "memory_id", ref.ID)
			continue
		}
		log.Debug("Memory instance retrieved successfully", "memory_id", ref.ID, "instance_id", memory.GetID())
		memories[ref.ID] = memory
	}

	if len(memories) == 0 {
		return nil
	}

	return &MemoryContext{memories: memories, references: references}
}

func (m *memoryManager) Inject(
	ctx context.Context,
	base []llmadapter.Message,
	ctxData *MemoryContext,
) []llmadapter.Message {
	if ctxData == nil || len(ctxData.memories) == 0 {
		return base
	}

	log := logger.FromContext(ctx)
	var memoryMessages []llmadapter.Message
	for memID, memory := range ctxData.memories {
		msgs, err := memory.Read(ctx)
		if err != nil {
			log.Warn("Failed to read messages from memory", "memory_id", memID, "error", core.RedactError(err))
			continue
		}
		for _, msg := range msgs {
			memoryMessages = append(memoryMessages, llmadapter.Message{Role: string(msg.Role), Content: msg.Content})
		}
	}

	if len(memoryMessages) == 0 {
		return base
	}

	combined := make([]llmadapter.Message, 0, len(memoryMessages)+len(base))
	combined = append(combined, memoryMessages...)
	combined = append(combined, base...)
	return combined
}

func (m *memoryManager) StoreAsync(
	ctx context.Context,
	ctxData *MemoryContext,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
	request Request,
) {
	if ctxData == nil || len(ctxData.memories) == 0 || response == nil || response.Content == "" {
		return
	}

	go func() {
		log := logger.FromContext(ctx)
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		memoryIDs := make([]string, 0, len(ctxData.memories))
		for _, memory := range ctxData.memories {
			if memory != nil {
				memoryIDs = append(memoryIDs, memory.GetID())
			}
		}

		storeFn := func() error {
			if len(messages) == 0 {
				return nil
			}
			assistantMsg := llmadapter.Message{Role: "assistant", Content: response.Content}
			lastMsg := messages[len(messages)-1]
			return m.store(bgCtx, ctxData.memories, ctxData.references, &assistantMsg, &lastMsg)
		}

		var err error
		if m.sync != nil {
			m.sync.WithMultipleLocks(memoryIDs, func() {
				err = storeFn()
			})
		} else {
			err = storeFn()
		}

		if err != nil {
			log.Error(
				"Failed to store response in memory",
				"error",
				core.RedactError(err),
				"agent_id",
				request.Agent.ID,
				"action_id",
				request.Action.ID,
			)
		}
		if m.hook != nil {
			m.hook.OnMemoryStoreComplete(err)
		}
	}()
}

func (m *memoryManager) store(
	ctx context.Context,
	memories map[string]contracts.Memory,
	refs []core.MemoryReference,
	assistant *llmadapter.Message,
	user *llmadapter.Message,
) error {
	if assistant == nil || user == nil {
		return fmt.Errorf("memory store: nil message pointer(s): assistant=%v user=%v", assistant == nil, user == nil)
	}
	if len(memories) == 0 {
		return nil
	}

	log := logger.FromContext(ctx)
	var errs []error
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("memory store canceled: %w", err)
		}
		memory, exists := memories[ref.ID]
		if !exists {
			log.Debug("Memory reference not found; skipping", "memory_id", ref.ID)
			continue
		}
		if ref.Mode == core.MemoryModeReadOnly {
			log.Debug("Skipping read-only memory", "memory_id", ref.ID)
			continue
		}

		msgs := []contracts.Message{
			{Role: contracts.MessageRole(user.Role), Content: user.Content},
			{Role: contracts.MessageRole(assistant.Role), Content: assistant.Content},
		}
		if err := memory.AppendMany(ctx, msgs); err != nil {
			log.Error(
				"Failed to append messages to memory atomically",
				"memory_id",
				ref.ID,
				"error",
				core.RedactError(err),
			)
			err = fmt.Errorf("failed to append messages to memory %s: %w", ref.ID, err)
			errs = append(errs, err)
			continue
		}
		log.Debug("Messages stored atomically in memory", "memory_id", ref.ID)
	}
	if len(errs) > 0 {
		return fmt.Errorf("memory storage errors: %w", errors.Join(errs...))
	}
	return nil
}
