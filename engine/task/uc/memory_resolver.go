package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// memoryResolverAdapter adapts a full memory.Memory to the llm.Memory interface
type memoryResolverAdapter struct {
	memory memory.Memory
}

func (m *memoryResolverAdapter) Append(ctx context.Context, msg llm.Message) error {
	return m.memory.Append(ctx, msg)
}

func (m *memoryResolverAdapter) Read(ctx context.Context) ([]llm.Message, error) {
	return m.memory.Read(ctx)
}

func (m *memoryResolverAdapter) GetID() string {
	return m.memory.GetID()
}

// Use memory.ManagerInterface directly

// MemoryResolver provides memory instances for agents during task execution.
// It implements the llm.MemoryProvider interface.
type MemoryResolver struct {
	memoryManager   memory.ManagerInterface
	templateEngine  *tplengine.TemplateEngine
	workflowContext map[string]any
}

// NewMemoryResolver creates a new memory resolver
func NewMemoryResolver(
	memoryManager memory.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
	workflowContext map[string]any,
) *MemoryResolver {
	return &MemoryResolver{
		memoryManager:   memoryManager,
		templateEngine:  templateEngine,
		workflowContext: workflowContext,
	}
}

// GetMemory retrieves a memory instance by ID and resolved key.
// Returns nil if no memory is configured or available.
func (r *MemoryResolver) GetMemory(ctx context.Context, memoryID string, keyTemplate string) (llm.Memory, error) {
	log := logger.FromContext(ctx)

	if r.memoryManager == nil {
		log.Debug("Memory manager not available")
		return nil, nil
	}

	// Resolve the key template using the workflow context
	resolvedKey, err := r.resolveKey(ctx, keyTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve memory key template: %w", err)
	}

	log.Debug("Resolving memory instance",
		"memory_id", memoryID,
		"key_template", keyTemplate,
		"resolved_key", resolvedKey,
	)

	// Create a memory reference for the manager
	memRef := core.MemoryReference{
		ID:          memoryID,
		Key:         keyTemplate,
		ResolvedKey: resolvedKey,
		Mode:        "read-write", // TODO: Get mode from agent memory configuration
	}

	// Get the memory instance from the manager
	memInstance, err := r.memoryManager.GetInstance(ctx, memRef, r.workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}

	if memInstance == nil {
		return nil, nil
	}

	// Wrap the memory instance to adapt it to the llm.Memory interface
	return &memoryResolverAdapter{memory: memInstance}, nil
}

// resolveKey resolves a key template using the workflow context
func (r *MemoryResolver) resolveKey(_ context.Context, keyTemplate string) (string, error) {
	if r.templateEngine == nil {
		// If no template engine, return the key as-is
		return keyTemplate, nil
	}

	// Execute the template with the workflow context
	resolved, err := r.templateEngine.RenderString(keyTemplate, r.workflowContext)
	if err != nil {
		return "", fmt.Errorf("failed to execute key template: %w", err)
	}

	return resolved, nil
}

// ResolveAgentMemories resolves all memory references for an agent and returns a map of memory instances.
// The returned map is keyed by the memory reference ID.
func (r *MemoryResolver) ResolveAgentMemories(ctx context.Context, agent *agent.Config) (map[string]llm.Memory, error) {
	log := logger.FromContext(ctx)

	memoryRefs := agent.GetResolvedMemoryReferences()
	if len(memoryRefs) == 0 {
		log.Debug("No memory references configured for agent", "agent_id", agent.ID)
		return nil, nil
	}

	memories := make(map[string]llm.Memory)
	// Create a copy of memory references to avoid mutating the shared agent config
	// This is critical for thread safety in concurrent workflow executions
	localMemoryRefs := make([]core.MemoryReference, len(memoryRefs))
	copy(localMemoryRefs, memoryRefs)

	for i := range localMemoryRefs {
		// Skip read-only memories for now (will be handled in Task 8)
		if localMemoryRefs[i].Mode == "read-only" {
			log.Debug("Skipping read-only memory (not yet implemented)",
				"memory_id", localMemoryRefs[i].ID,
				"mode", localMemoryRefs[i].Mode,
			)
			continue
		}

		memory, err := r.GetMemory(ctx, localMemoryRefs[i].ID, localMemoryRefs[i].Key)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve memory %s: %w", localMemoryRefs[i].ID, err)
		}

		if memory != nil {
			memories[localMemoryRefs[i].ID] = memory
			// The resolved key was already computed in GetMemory
			// and is available in the memory instance if needed for logging
		}
	}

	log.Debug("Resolved agent memories",
		"agent_id", agent.ID,
		"memory_count", len(memories),
	)

	return memories, nil
}
