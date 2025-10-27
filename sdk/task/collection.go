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

// CollectionBuilder creates engine collection task configurations while aggregating validation errors.
// It exposes fluent helpers for configuring the collection source, per-item task template, and item variable naming.
type CollectionBuilder struct {
	config *enginetask.Config
	errors []error
}

// NewCollection constructs a collection task builder initialized with the provided identifier.
func NewCollection(id string) *CollectionBuilder {
	trimmed := strings.TrimSpace(id)
	return &CollectionBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeCollection,
			},
		},
		errors: make([]error, 0),
	}
}

// WithCollection assigns the template expression used to resolve the collection items at runtime.
func (b *CollectionBuilder) WithCollection(collection string) *CollectionBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(collection)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("collection source expression cannot be empty"))
		return b
	}
	b.config.Items = trimmed
	return b
}

// WithTask sets the identifier of the task executed for each item in the collection.
func (b *CollectionBuilder) WithTask(taskID string) *CollectionBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("task id cannot be empty"))
		b.config.Task = nil
		return b
	}
	b.config.Task = &enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			Resource: string(core.ConfigTask),
			ID:       trimmed,
		},
	}
	return b
}

// WithItemVar customizes the template variable name that references the current item during iteration.
// When not specified the variable defaults to "item".
func (b *CollectionBuilder) WithItemVar(varName string) *CollectionBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(varName)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("item variable name cannot be empty"))
		b.config.ItemVar = ""
		return b
	}
	b.config.ItemVar = trimmed
	return b
}

// Build validates the accumulated configuration and returns a populated engine task config using the provided context.
func (b *CollectionBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("collection builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug(
		"building collection task configuration",
		"task",
		b.config.ID,
		"collection",
		b.config.Items,
		"hasTask",
		b.config.Task != nil,
	)

	collected := append(make([]error, 0, len(b.errors)+3), b.errors...)
	collected = append(collected, b.validateID(ctx), b.validateCollection(ctx), b.validateTask(ctx))

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	b.config.Default()
	if b.config.Task != nil {
		b.config.Task.Resource = string(core.ConfigTask)
		b.config.Task.Type = ""
	}
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone collection task config: %w", err)
	}
	return cloned, nil
}

func (b *CollectionBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	b.config.Resource = string(core.ConfigTask)
	b.config.Type = enginetask.TaskTypeCollection
	return nil
}

func (b *CollectionBuilder) validateCollection(ctx context.Context) error {
	items := strings.TrimSpace(b.config.Items)
	if err := validate.NonEmpty(ctx, "collection items", items); err != nil {
		return err
	}
	b.config.Items = items
	if trimmed := strings.TrimSpace(b.config.ItemVar); trimmed != "" {
		b.config.ItemVar = trimmed
	}
	return nil
}

func (b *CollectionBuilder) validateTask(ctx context.Context) error {
	if b.config.Task == nil {
		return fmt.Errorf("collection task template is required")
	}
	b.config.Task.ID = strings.TrimSpace(b.config.Task.ID)
	if err := validate.ID(ctx, b.config.Task.ID); err != nil {
		return fmt.Errorf("collection task id is invalid: %w", err)
	}
	b.config.Task.Resource = string(core.ConfigTask)
	b.config.Task.Type = ""
	return nil
}
