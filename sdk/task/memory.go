package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultMemoryKeyTemplate = "{{ .workflow.id }}:{{ .task.id }}"

// MemoryTaskBuilder constructs engine memory task configurations supporting read,
// append, and clear operations while aggregating validation errors for deferred
// reporting.
type MemoryTaskBuilder struct {
	config     *enginetask.Config
	errors     []error
	content    string
	hasContent bool
}

// NewMemoryTask creates a memory task builder initialized with the provided
// identifier and default key template.
func NewMemoryTask(id string) *MemoryTaskBuilder {
	trimmed := strings.TrimSpace(id)
	return &MemoryTaskBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeMemory,
			},
			MemoryTask: enginetask.MemoryTask{
				KeyTemplate: defaultMemoryKeyTemplate,
			},
		},
		errors: make([]error, 0),
	}
}

// WithOperation configures the memory operation to execute. Supported
// operations are "read", "append", and "clear".
func (b *MemoryTaskBuilder) WithOperation(op string) *MemoryTaskBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(op)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("operation cannot be empty"))
		b.config.Operation = ""
		b.config.ClearConfig = nil
		return b
	}

	normalized := strings.ToLower(trimmed)
	switch enginetask.MemoryOpType(normalized) {
	case enginetask.MemoryOpRead:
		b.config.Operation = enginetask.MemoryOpRead
		b.config.ClearConfig = nil
	case enginetask.MemoryOpAppend:
		b.config.Operation = enginetask.MemoryOpAppend
		b.config.ClearConfig = nil
	case enginetask.MemoryOpClear:
		b.config.Operation = enginetask.MemoryOpClear
		b.config.ClearConfig = &enginetask.ClearConfig{Confirm: true}
		b.hasContent = b.hasContent && b.content != ""
	default:
		b.errors = append(b.errors, fmt.Errorf("unsupported memory operation: %s", trimmed))
		b.config.Operation = ""
		b.config.ClearConfig = nil
	}

	return b
}

// WithMemory assigns the memory reference identifier resolved during
// execution.
func (b *MemoryTaskBuilder) WithMemory(memoryID string) *MemoryTaskBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(memoryID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("memory id cannot be empty"))
		b.config.MemoryRef = ""
		return b
	}
	b.config.MemoryRef = trimmed
	return b
}

// WithContent sets the payload appended to memory for append operations.
func (b *MemoryTaskBuilder) WithContent(content string) *MemoryTaskBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("content cannot be empty"))
		b.hasContent = false
		b.content = ""
		return b
	}
	b.content = trimmed
	b.hasContent = true
	return b
}

// WithKeyTemplate overrides the default key template evaluated during memory
// operations.
func (b *MemoryTaskBuilder) WithKeyTemplate(template string) *MemoryTaskBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("key template cannot be empty"))
		return b
	}
	b.config.KeyTemplate = trimmed
	return b
}

// Build validates the accumulated configuration using the provided context and
// returns an immutable engine task configuration.
func (b *MemoryTaskBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if err := b.ensureBuilderState(ctx); err != nil {
		return nil, err
	}
	errs := b.collectBuildErrors(ctx)
	if len(errs) > 0 {
		return nil, &sdkerrors.BuildError{Errors: errs}
	}
	b.applyOperationDefaults()
	b.config.Type = enginetask.TaskTypeMemory
	b.config.Resource = string(core.ConfigTask)
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone memory task config: %w", err)
	}
	return cloned, nil
}

func (b *MemoryTaskBuilder) ensureBuilderState(ctx context.Context) error {
	if b == nil {
		return fmt.Errorf("memory builder is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	logger.FromContext(ctx).Debug(
		"building memory task configuration",
		"task",
		b.config.ID,
		"operation",
		b.config.Operation,
		"memory",
		b.config.MemoryRef,
	)
	return nil
}

func (b *MemoryTaskBuilder) collectBuildErrors(ctx context.Context) []error {
	collected := append(make([]error, 0, len(b.errors)+5), b.errors...)
	collected = append(
		collected,
		b.validateID(ctx),
		b.validateOperation(),
		b.validateMemoryRef(ctx),
		b.validateKeyTemplate(),
		b.applyContent(),
	)
	return filterErrors(collected)
}

func (b *MemoryTaskBuilder) applyOperationDefaults() {
	if b.config.Operation != enginetask.MemoryOpClear {
		return
	}
	if b.config.ClearConfig == nil {
		b.config.ClearConfig = &enginetask.ClearConfig{Confirm: true}
	}
	b.config.Payload = nil
}

func filterErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

func (b *MemoryTaskBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	return nil
}

func (b *MemoryTaskBuilder) validateOperation() error {
	switch b.config.Operation {
	case enginetask.MemoryOpRead, enginetask.MemoryOpAppend, enginetask.MemoryOpClear:
		return nil
	case "":
		return fmt.Errorf("operation is required")
	default:
		return fmt.Errorf("unsupported memory operation: %s", b.config.Operation)
	}
}

func (b *MemoryTaskBuilder) validateMemoryRef(ctx context.Context) error {
	b.config.MemoryRef = strings.TrimSpace(b.config.MemoryRef)
	if err := validate.ID(ctx, b.config.MemoryRef); err != nil {
		return fmt.Errorf("memory reference id is invalid: %w", err)
	}
	return nil
}

func (b *MemoryTaskBuilder) validateKeyTemplate() error {
	b.config.KeyTemplate = strings.TrimSpace(b.config.KeyTemplate)
	if b.config.KeyTemplate == "" {
		return fmt.Errorf("key template cannot be empty")
	}
	return nil
}

func (b *MemoryTaskBuilder) applyContent() error {
	switch b.config.Operation {
	case enginetask.MemoryOpAppend:
		if !b.hasContent {
			return fmt.Errorf("append operation requires content")
		}
		b.config.Payload = b.content
	default:
		b.config.Payload = nil
	}
	return nil
}
