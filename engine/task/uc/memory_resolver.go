package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// memoryResolverAdapter adapts a full memcore.Memory to the llm.Memory interface
type memoryResolverAdapter struct {
	memory memcore.Memory
}

func (m *memoryResolverAdapter) Append(ctx context.Context, msg llm.Message) error {
	return m.memory.Append(ctx, msg)
}

func (m *memoryResolverAdapter) AppendMany(ctx context.Context, msgs []llm.Message) error {
	return m.memory.AppendMany(ctx, msgs)
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
	memoryManager   memcore.ManagerInterface
	templateEngine  *tplengine.TemplateEngine
	workflowContext map[string]any
}

// NewMemoryResolver creates a new memory resolver
func NewMemoryResolver(
	memoryManager memcore.ManagerInterface,
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

	log.Debug("Resolving memory access",
		"memory_id", memoryID,
		"key_template", keyTemplate,
	)

	if r.memoryManager == nil {
		log.Error("Memory manager is nil")
		return nil, nil
	}

	// Allow empty keyTemplate: manager will fall back to memory.Config.default_key_template if available.
	var resolvedKey string
	if strings.TrimSpace(keyTemplate) != "" {
		var err error
		resolvedKey, err = r.resolveKey(ctx, keyTemplate)
		if err != nil {
			return nil, err
		}
	}

	// Create a memory reference for the manager
	// IMPORTANT: We pass the template in the Key field, NOT in ResolvedKey
	// The Manager's resolveMemoryKey method will handle the template resolution
	memRef := core.MemoryReference{
		ID:          memoryID,
		Key:         keyTemplate, // This contains the template string
		ResolvedKey: resolvedKey,
		Mode:        core.MemoryModeReadWrite, // TODO: Get mode from agent memory configuration
	}

	log.Debug("Passing memory reference to manager",
		"memory_id", memoryID,
		"key_template", keyTemplate)

	// Get the memory instance from the manager
	memInstance, err := r.memoryManager.GetInstance(ctx, memRef, r.workflowContext)
	if err != nil {
		log.Error("Failed to get memory instance",
			"memory_id", memoryID,
			"key_template", keyTemplate,
			"error", err,
			"workflow_context", fmt.Sprintf("%+v", r.workflowContext),
		)
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}

	if memInstance == nil {
		return nil, nil
	}

	// Wrap the memory instance to adapt it to the llm.Memory interface
	return &memoryResolverAdapter{memory: memInstance}, nil
}

// resolveKey resolves a key template using the workflow context
func (r *MemoryResolver) resolveKey(ctx context.Context, keyTemplate string) (string, error) {
	log := logger.FromContext(ctx)
	log.Debug("Starting key resolution",
		"key_template", keyTemplate,
		"has_template_engine", r.templateEngine != nil,
		"workflow_context", fmt.Sprintf("%+v", r.workflowContext),
	)

	if r.templateEngine == nil {
		// Check if the key appears to be a template
		if strings.Contains(keyTemplate, "{{") {
			log.Error("Template engine is nil but key has template syntax",
				"key_template", keyTemplate)
			return "", fmt.Errorf("template engine is required to resolve key template: %s", keyTemplate)
		}
		// If no template syntax, return as literal key
		return keyTemplate, nil
	}

	// Execute the template with the workflow context
	resolved, err := r.templateEngine.RenderString(keyTemplate, r.workflowContext)
	if err != nil {
		log.Error("Template resolution failed",
			"key_template", keyTemplate,
			"error", err,
			"workflow_context", fmt.Sprintf("%+v", r.workflowContext))
		return "", fmt.Errorf("failed to execute key template: %w", err)
	}

	log.Debug("Template resolved successfully",
		"key_template", keyTemplate,
		"resolved_key", resolved)

	return resolved, nil
}

// ResolveAgentMemories resolves all memory references for an agent and returns a map of memory instances.
// The returned map is keyed by the memory reference ID.
func (r *MemoryResolver) ResolveAgentMemories(ctx context.Context, agent *agent.Config) (map[string]llm.Memory, error) {
	log := logger.FromContext(ctx)
	memoryRefs := agent.Memory

	// Enhanced logging for debugging memory configuration issues
	log.Debug("ResolveAgentMemories called",
		"agent_id", agent.ID,
		"memory_refs_count", len(memoryRefs),
		"memory_refs", fmt.Sprintf("%+v", memoryRefs),
		"workflow_context", fmt.Sprintf("%+v", r.workflowContext),
	)

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
		if localMemoryRefs[i].Mode == core.MemoryModeReadOnly {
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
