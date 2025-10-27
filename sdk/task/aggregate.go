package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const (
	aggregateStrategyConcat = "concat"
	aggregateStrategyMerge  = "merge"
	aggregateStrategyCustom = "custom"
)

// AggregateBuilder constructs aggregate task configurations that consolidate
// results from previously executed tasks using configurable aggregation
// strategies. It accumulates validation errors and applies them during Build.
type AggregateBuilder struct {
	config   *enginetask.Config
	errors   []error
	tasks    []string
	strategy string
	function string
}

// NewAggregate creates a builder for an aggregate task identified by the
// provided id. The builder defaults to the concat strategy.
func NewAggregate(id string) *AggregateBuilder {
	trimmed := strings.TrimSpace(id)
	return &AggregateBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeAggregate,
			},
		},
		errors:   make([]error, 0),
		tasks:    make([]string, 0),
		strategy: aggregateStrategyConcat,
	}
}

// AddTask registers a task identifier whose output should participate in the
// aggregation result.
func (b *AggregateBuilder) AddTask(taskID string) *AggregateBuilder {
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
	b.tasks = append(b.tasks, trimmed)
	return b
}

// WithStrategy selects the aggregation strategy applied to referenced task
// outputs. Supported strategies are "concat", "merge", and "custom".
func (b *AggregateBuilder) WithStrategy(strategy string) *AggregateBuilder {
	if b == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	if normalized == "" {
		b.errors = append(b.errors, fmt.Errorf("aggregation strategy cannot be empty"))
		return b
	}
	switch normalized {
	case aggregateStrategyConcat, aggregateStrategyMerge, aggregateStrategyCustom:
		b.strategy = normalized
	default:
		b.errors = append(b.errors, fmt.Errorf("invalid aggregation strategy: %s", normalized))
		b.strategy = normalized
	}
	return b
}

// WithFunction configures a custom aggregation function template. Providing a
// function implicitly switches the builder to the custom strategy.
func (b *AggregateBuilder) WithFunction(fn string) *AggregateBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(fn)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("custom aggregation function cannot be empty"))
		return b
	}
	b.function = trimmed
	b.strategy = aggregateStrategyCustom
	return b
}

// Build validates the builder state using the provided context and returns an
// engine task configuration ready for execution.
func (b *AggregateBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("aggregate builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug(
		"building aggregate task configuration",
		"task",
		b.config.ID,
		"strategy",
		b.strategy,
		"tasks",
		len(b.tasks),
		"hasFunction",
		b.function != "",
	)

	collected := append(make([]error, 0, len(b.errors)+4), b.errors...)
	collected = append(collected, b.validateID(ctx), b.validateTasks(ctx), b.validateStrategy())

	var outputs *core.Input
	if len(b.tasks) > 0 {
		var outputsErr error
		outputs, outputsErr = b.buildOutputs()
		collected = append(collected, outputsErr)
	}

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	b.config.Outputs = outputs
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone aggregate task config: %w", err)
	}
	return cloned, nil
}

func (b *AggregateBuilder) hasTask(id string) bool {
	for _, taskID := range b.tasks {
		if taskID == id {
			return true
		}
	}
	return false
}

func (b *AggregateBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	b.config.Resource = string(core.ConfigTask)
	b.config.Type = enginetask.TaskTypeAggregate
	return nil
}

func (b *AggregateBuilder) validateTasks(ctx context.Context) error {
	if len(b.tasks) == 0 {
		return fmt.Errorf("aggregate tasks require at least one task reference")
	}
	for idx := range b.tasks {
		taskID := strings.TrimSpace(b.tasks[idx])
		if err := validate.ID(ctx, taskID); err != nil {
			return fmt.Errorf("aggregate task reference %q is invalid: %w", taskID, err)
		}
		b.tasks[idx] = taskID
	}
	return nil
}

func (b *AggregateBuilder) validateStrategy() error {
	if b.strategy == "" {
		b.strategy = aggregateStrategyConcat
	}
	switch b.strategy {
	case aggregateStrategyConcat, aggregateStrategyMerge:
		return nil
	case aggregateStrategyCustom:
		if strings.TrimSpace(b.function) == "" {
			return fmt.Errorf("custom aggregation strategy requires a function")
		}
		return nil
	default:
		return fmt.Errorf("invalid aggregation strategy: %s", b.strategy)
	}
}

func (b *AggregateBuilder) buildOutputs() (*core.Input, error) {
	if len(b.tasks) == 0 {
		return nil, fmt.Errorf("aggregate tasks require at least one task reference")
	}
	taskRefs := make(map[string]any, len(b.tasks))
	for _, taskID := range b.tasks {
		taskRefs[taskID] = fmt.Sprintf("{{ .tasks.%s.output }}", taskID)
	}

	payload := map[string]any{
		shared.FieldStrategy: b.strategy,
		shared.TasksKey:      taskRefs,
		"result":             b.buildAggregationExpression(),
	}
	if b.strategy == aggregateStrategyCustom {
		payload["function"] = b.function
	}

	outputs := core.Input{
		shared.FieldAggregated: payload,
	}
	return &outputs, nil
}

func (b *AggregateBuilder) buildAggregationExpression() string {
	switch b.strategy {
	case aggregateStrategyMerge:
		return b.buildMergeExpression()
	case aggregateStrategyCustom:
		return wrapTemplateExpression(b.function)
	default:
		return b.buildConcatExpression()
	}
}

func (b *AggregateBuilder) buildConcatExpression() string {
	if len(b.tasks) == 1 {
		return fmt.Sprintf("{{ .tasks.%s.output }}", b.tasks[0])
	}
	refs := make([]string, 0, len(b.tasks))
	for _, taskID := range b.tasks {
		refs = append(refs, fmt.Sprintf(".tasks.%s.output", taskID))
	}
	return fmt.Sprintf("{{ list %s }}", strings.Join(refs, " "))
}

func (b *AggregateBuilder) buildMergeExpression() string {
	if len(b.tasks) == 1 {
		return fmt.Sprintf("{{ .tasks.%s.output }}", b.tasks[0])
	}
	refs := make([]string, 0, len(b.tasks))
	for _, taskID := range b.tasks {
		refs = append(refs, fmt.Sprintf(".tasks.%s.output", taskID))
	}
	return fmt.Sprintf("{{ merge (dict) %s }}", strings.Join(refs, " "))
}

func wrapTemplateExpression(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		return trimmed
	}
	return fmt.Sprintf("{{ %s }}", trimmed)
}
