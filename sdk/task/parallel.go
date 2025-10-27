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

// ParallelBuilder constructs engine parallel task configurations while aggregating validation errors.
// It provides fluent helpers for wait-all and wait-first execution semantics.
type ParallelBuilder struct {
	config *enginetask.Config
	errors []error
}

// NewParallel creates a builder for the parallel task identified by the provided id.
// The builder defaults to wait-all strategy and collects validation issues for deferred reporting.
func NewParallel(id string) *ParallelBuilder {
	trimmed := strings.TrimSpace(id)
	return &ParallelBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeParallel,
			},
			ParallelTask: enginetask.ParallelTask{
				Strategy: enginetask.StrategyWaitAll,
			},
			Tasks: make([]enginetask.Config, 0),
		},
		errors: make([]error, 0),
	}
}

// AddTask registers a child task identified by the provided id for parallel execution.
func (b *ParallelBuilder) AddTask(taskID string) *ParallelBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("task id cannot be empty"))
		return b
	}
	if b.hasTask(trimmed) {
		b.errors = append(b.errors, fmt.Errorf("duplicate task id: %s", trimmed))
		return b
	}
	b.config.Tasks = append(b.config.Tasks, enginetask.Config{
		BaseConfig: enginetask.BaseConfig{
			Resource: string(core.ConfigTask),
			ID:       trimmed,
		},
	})
	return b
}

// WithWaitAll configures whether the builder waits for all tasks or returns on the first completion.
func (b *ParallelBuilder) WithWaitAll(waitAll bool) *ParallelBuilder {
	if b == nil {
		return nil
	}
	if waitAll {
		b.config.Strategy = enginetask.StrategyWaitAll
		return b
	}
	b.config.Strategy = enginetask.StrategyRace
	return b
}

// Build validates the accumulated configuration using the provided context and returns engine config.
func (b *ParallelBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("parallel builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug(
		"building parallel task configuration",
		"task",
		b.config.ID,
		"tasks",
		len(b.config.Tasks),
		"strategy",
		b.config.ParallelTask.GetStrategy(),
	)

	collected := append(make([]error, 0, len(b.errors)+2), b.errors...)
	collected = append(collected, b.validateID(ctx), b.validateTasks(ctx))

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
		return nil, fmt.Errorf("failed to clone parallel task config: %w", err)
	}
	return cloned, nil
}

func (b *ParallelBuilder) hasTask(id string) bool {
	for i := range b.config.Tasks {
		if b.config.Tasks[i].ID == id {
			return true
		}
	}
	return false
}

func (b *ParallelBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	return nil
}

func (b *ParallelBuilder) validateTasks(ctx context.Context) error {
	if len(b.config.Tasks) == 0 {
		return fmt.Errorf("parallel tasks require at least one child task")
	}
	for i := range b.config.Tasks {
		child := &b.config.Tasks[i]
		child.ID = strings.TrimSpace(child.ID)
		if err := validate.ID(ctx, child.ID); err != nil {
			return fmt.Errorf("child task id is invalid: %w", err)
		}
		child.Resource = string(core.ConfigTask)
		child.Type = ""
	}
	return nil
}
