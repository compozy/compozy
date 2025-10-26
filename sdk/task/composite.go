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

// CompositeBuilder constructs engine composite task configurations for workflow composition.
// It captures validation errors across fluent calls and defers reporting until Build(ctx).
type CompositeBuilder struct {
	config *enginetask.Config
	errors []error
}

// NewComposite creates a composite task builder initialized with the provided task identifier.
func NewComposite(id string) *CompositeBuilder {
	trimmed := strings.TrimSpace(id)
	return &CompositeBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeComposite,
			},
		},
		errors: make([]error, 0),
	}
}

// WithWorkflow registers the nested workflow identifier executed by this composite task.
func (b *CompositeBuilder) WithWorkflow(workflowID string) *CompositeBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(workflowID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("workflow id cannot be empty"))
		b.config.Action = ""
		return b
	}
	b.config.Action = trimmed
	return b
}

// WithInput configures template-driven input parameters passed to the nested workflow.
func (b *CompositeBuilder) WithInput(input map[string]string) *CompositeBuilder {
	if b == nil {
		return nil
	}
	if input == nil {
		b.config.With = nil
		return b
	}
	values := make(map[string]any, len(input))
	for rawKey, value := range input {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			b.errors = append(b.errors, fmt.Errorf("input key cannot be empty"))
			continue
		}
		values[key] = value
	}
	mapped := core.Input(values)
	b.config.With = &mapped
	return b
}

// Build validates the accumulated configuration using the provided context and returns an engine config.
func (b *CompositeBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("composite builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug(
		"building composite task configuration",
		"task",
		b.config.ID,
		"workflow",
		b.config.Action,
		"hasInput",
		b.config.With != nil,
	)

	collected := make([]error, 0, len(b.errors)+2)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	collected = append(collected, b.validateWorkflow(ctx))

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
		return nil, fmt.Errorf("failed to clone composite task config: %w", err)
	}
	return cloned, nil
}

func (b *CompositeBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	b.config.Resource = string(core.ConfigTask)
	b.config.Type = enginetask.TaskTypeComposite
	return nil
}

func (b *CompositeBuilder) validateWorkflow(ctx context.Context) error {
	b.config.Action = strings.TrimSpace(b.config.Action)
	if b.config.Action == "" {
		return fmt.Errorf("workflow id is required")
	}
	if err := validate.ValidateID(ctx, b.config.Action); err != nil {
		return fmt.Errorf("workflow id is invalid: %w", err)
	}
	return nil
}
