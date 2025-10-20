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
	log.Debug("Resolving memory access", "memory_id", memoryID)
	if r.skipMemoryResolution(ctx, memoryID) {
		return nil, nil
	}
	resolvedKey, err := r.prepareMemoryKey(ctx, keyTemplate)
	if err != nil {
		return nil, err
	}
	memRef := r.buildMemoryReference(memoryID, keyTemplate, resolvedKey)
	return r.fetchMemoryInstance(ctx, memRef)
}

// skipMemoryResolution reports whether resolution should stop due to invalid setup.
func (r *MemoryResolver) skipMemoryResolution(ctx context.Context, memoryID string) bool {
	trimmedID := strings.TrimSpace(memoryID)
	log := logger.FromContext(ctx)
	if trimmedID == "" {
		log.Warn("Empty memory ID provided")
		return true
	}
	if r.memoryManager == nil {
		log.Warn("Memory manager is nil")
		return true
	}
	return false
}

// prepareMemoryKey resolves the key template when provided.
func (r *MemoryResolver) prepareMemoryKey(ctx context.Context, keyTemplate string) (string, error) {
	if strings.TrimSpace(keyTemplate) == "" {
		return "", nil
	}
	resolvedKey, err := r.resolveKey(ctx, keyTemplate)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolvedKey) == "" {
		return "", fmt.Errorf("resolved key template to empty string: %q", keyTemplate)
	}
	return resolvedKey, nil
}

// buildMemoryReference constructs the reference passed to the memory manager.
func (r *MemoryResolver) buildMemoryReference(
	memoryID string,
	keyTemplate string,
	resolvedKey string,
) core.MemoryReference {
	memRef := core.MemoryReference{ID: memoryID, Mode: core.MemoryModeReadWrite}
	if strings.TrimSpace(resolvedKey) != "" {
		memRef.ResolvedKey = resolvedKey
		return memRef
	}
	memRef.Key = keyTemplate
	return memRef
}

// fetchMemoryInstance retrieves and adapts the underlying memory implementation.
func (r *MemoryResolver) fetchMemoryInstance(ctx context.Context, memRef core.MemoryReference) (llm.Memory, error) {
	log := logger.FromContext(ctx)
	log.Debug("Passing memory reference to manager", "memory_id", memRef.ID)
	memInstance, err := r.memoryManager.GetInstance(ctx, memRef, r.workflowContext)
	if err != nil {
		log.Error("Failed to get memory instance",
			"memory_id", memRef.ID,
			"error", err,
			"workflow_context", fmt.Sprintf("%+v", r.workflowContext),
		)
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	if memInstance == nil {
		return nil, nil
	}
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
		if tplengine.HasTemplate(keyTemplate) {
			log.Error("Template engine is nil but key has template syntax",
				"key_template", keyTemplate)
			return "", fmt.Errorf("template engine is required to resolve key template: %s", keyTemplate)
		}
		return keyTemplate, nil
	}
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
	// NOTE: Copy memory references to keep agent config immutable across concurrent executions.
	localMemoryRefs := make([]core.MemoryReference, len(memoryRefs))
	copy(localMemoryRefs, memoryRefs)
	for i := range localMemoryRefs {
		if localMemoryRefs[i].Mode == core.MemoryModeReadOnly {
			log.Warn("Skipping read-only memory; mode not supported", "memory_id", localMemoryRefs[i].ID)
			continue
		}

		memory, err := r.GetMemory(ctx, localMemoryRefs[i].ID, localMemoryRefs[i].Key)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve memory %s: %w", localMemoryRefs[i].ID, err)
		}

		if memory != nil {
			memories[localMemoryRefs[i].ID] = memory
		}
	}
	log.Debug("Resolved agent memories",
		"agent_id", agent.ID,
		"memory_count", len(memories),
	)
	return memories, nil
}
