package agent

import (
	"context"
	"fmt"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
	sdkknowledge "github.com/compozy/compozy/sdk/knowledge"
	sdkmemory "github.com/compozy/compozy/sdk/memory"
)

// Builder constructs engine agent configurations using a fluent API while collecting validation errors.
type Builder struct {
	config *engineagent.Config
	errors []error
}

// New creates an agent builder initialized with the provided identifier.
func New(id string) *Builder {
	trimmed := strings.TrimSpace(id)
	return &Builder{
		config: &engineagent.Config{
			Resource: string(core.ConfigAgent),
			ID:       trimmed,
			LLMProperties: engineagent.LLMProperties{
				Tools:  make([]tool.Config, 0),
				MCPs:   make([]mcp.Config, 0),
				Memory: make([]core.MemoryReference, 0),
			},
			Actions: make([]*engineagent.ActionConfig, 0),
		},
		errors: make([]error, 0),
	}
}

// WithModel configures an inline model provider and model name for the agent.
func (b *Builder) WithModel(provider, model string) *Builder {
	if b == nil {
		return nil
	}
	trimmedProvider := strings.TrimSpace(provider)
	trimmedModel := strings.TrimSpace(model)
	b.config.Model.Ref = ""
	b.config.Model.Config = core.ProviderConfig{
		Provider: core.ProviderName(strings.ToLower(trimmedProvider)),
		Model:    trimmedModel,
	}
	return b
}

// WithModelRef attaches a model reference that will be resolved from the project registry.
func (b *Builder) WithModelRef(modelID string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(modelID)
	b.config.Model.Ref = trimmed
	b.config.Model.Config = core.ProviderConfig{}
	return b
}

// WithInstructions assigns the system instructions that guide agent behavior.
func (b *Builder) WithInstructions(instructions string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(instructions)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("instructions cannot be empty"))
		return b
	}
	b.config.Instructions = trimmed
	return b
}

// WithKnowledge binds the agent to a knowledge base configuration.
func (b *Builder) WithKnowledge(binding *sdkknowledge.BindingConfig) *Builder {
	if b == nil {
		return nil
	}
	if binding == nil {
		b.errors = append(b.errors, fmt.Errorf("knowledge binding cannot be nil"))
		return b
	}
	cloned := binding.Clone()
	cloned.ID = strings.TrimSpace(cloned.ID)
	if cloned.ID == "" {
		b.errors = append(b.errors, fmt.Errorf("knowledge binding id cannot be empty"))
		return b
	}
	b.config.Knowledge = []core.KnowledgeBinding{cloned}
	return b
}

// WithMemory appends a memory reference that the agent can read or write.
func (b *Builder) WithMemory(ref *sdkmemory.ReferenceConfig) *Builder {
	if b == nil {
		return nil
	}
	if ref == nil {
		b.errors = append(b.errors, fmt.Errorf("memory reference cannot be nil"))
		return b
	}
	copyRef := *ref
	copyRef.ID = strings.TrimSpace(copyRef.ID)
	if copyRef.ID == "" {
		b.errors = append(b.errors, fmt.Errorf("memory reference id cannot be empty"))
		return b
	}
	b.config.Memory = append(b.config.Memory, copyRef)
	return b
}

// AddAction registers an action configuration with the agent definition.
func (b *Builder) AddAction(action *engineagent.ActionConfig) *Builder {
	if b == nil {
		return nil
	}
	if action == nil {
		b.errors = append(b.errors, fmt.Errorf("action cannot be nil"))
		return b
	}
	trimmedID := strings.TrimSpace(action.ID)
	if trimmedID == "" {
		b.errors = append(b.errors, fmt.Errorf("action id cannot be empty"))
		return b
	}
	clone, err := core.DeepCopy(action)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to copy action config: %w", err))
		return b
	}
	clone.ID = trimmedID
	b.config.Actions = append(b.config.Actions, clone)
	return b
}

// AddTool adds a tool reference that will be available to the agent at runtime.
func (b *Builder) AddTool(toolID string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(toolID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("tool id cannot be empty"))
		return b
	}
	b.config.Tools = append(b.config.Tools, tool.Config{ID: trimmed, Resource: string(core.ConfigTool)})
	return b
}

// AddMCP registers an MCP server reference that extends the agent capabilities.
func (b *Builder) AddMCP(mcpID string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(mcpID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("mcp id cannot be empty"))
		return b
	}
	b.config.MCPs = append(b.config.MCPs, mcp.Config{ID: trimmed})
	return b
}

// Build validates the accumulated configuration and returns an agent config when successful.
func (b *Builder) Build(ctx context.Context) (*engineagent.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("agent builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building agent configuration", "agent", b.config.ID, "actions", len(b.config.Actions))
	collected := make([]error, 0, len(b.errors)+8)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateInstructions(ctx))
	collected = append(collected, b.validateModel(ctx))
	collected = append(collected, b.validateKnowledge())
	collected = append(collected, b.validateMemory())
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone agent config: %w", err)
	}
	return cloned, nil
}

func (b *Builder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("agent id is invalid: %w", err)
	}
	return nil
}

func (b *Builder) validateInstructions(ctx context.Context) error {
	b.config.Instructions = strings.TrimSpace(b.config.Instructions)
	if err := validate.ValidateNonEmpty(ctx, "instructions", b.config.Instructions); err != nil {
		return err
	}
	return nil
}

func (b *Builder) validateModel(ctx context.Context) error {
	if b.config.Model.HasRef() {
		b.config.Model.Ref = strings.TrimSpace(b.config.Model.Ref)
		if err := validate.ValidateNonEmpty(ctx, "model reference", b.config.Model.Ref); err != nil {
			return err
		}
		return nil
	}
	if b.config.Model.HasConfig() {
		provider := strings.ToLower(strings.TrimSpace(string(b.config.Model.Config.Provider)))
		modelName := strings.TrimSpace(b.config.Model.Config.Model)
		if err := validate.ValidateNonEmpty(ctx, "model provider", provider); err != nil {
			return err
		}
		if err := validate.ValidateNonEmpty(ctx, "model name", modelName); err != nil {
			return err
		}
		b.config.Model.Config.Provider = core.ProviderName(provider)
		b.config.Model.Config.Model = modelName
		return nil
	}
	return fmt.Errorf("model configuration or reference is required")
}

func (b *Builder) validateKnowledge() error {
	if len(b.config.Knowledge) > 1 {
		return fmt.Errorf("only one knowledge binding is supported")
	}
	if len(b.config.Knowledge) == 1 {
		binding := b.config.Knowledge[0]
		if strings.TrimSpace(binding.ID) == "" {
			return fmt.Errorf("knowledge binding id cannot be empty")
		}
		b.config.Knowledge[0].ID = strings.TrimSpace(binding.ID)
	}
	return nil
}

func (b *Builder) validateMemory() error {
	for idx := range b.config.Memory {
		b.config.Memory[idx].ID = strings.TrimSpace(b.config.Memory[idx].ID)
		if b.config.Memory[idx].ID == "" {
			return fmt.Errorf("memory reference at index %d is missing an id", idx)
		}
	}
	return nil
}
