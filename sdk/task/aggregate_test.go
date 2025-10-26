package task

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateBuilderConcatStrategy(t *testing.T) {
	t.Parallel()

	cfg, err := NewAggregate("combine-results").
		AddTask("collect-alpha").
		AddTask("collect-beta").
		WithStrategy("concat").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, enginetask.TaskTypeAggregate, cfg.Type)
	assert.Equal(t, string(core.ConfigTask), cfg.Resource)
	require.NotNil(t, cfg.Outputs)

	aggregated, ok := (*cfg.Outputs)[shared.FieldAggregated].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, aggregateStrategyConcat, aggregated[shared.FieldStrategy])
	resultExpr, ok := aggregated["result"].(string)
	require.True(t, ok)
	assert.Equal(t, "{{ list .tasks.collect-alpha.output .tasks.collect-beta.output }}", resultExpr)

	tasksMap, ok := aggregated[shared.TasksKey].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{ .tasks.collect-alpha.output }}", tasksMap["collect-alpha"])
	assert.Equal(t, "{{ .tasks.collect-beta.output }}", tasksMap["collect-beta"])
}

func TestAggregateBuilderMergeStrategy(t *testing.T) {
	t.Parallel()

	cfg, err := NewAggregate("merge-datasets").
		AddTask("primary").
		AddTask("secondary").
		WithStrategy("merge").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.NotNil(t, cfg.Outputs)
	aggregated, ok := (*cfg.Outputs)[shared.FieldAggregated].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, aggregateStrategyMerge, aggregated[shared.FieldStrategy])
	assert.Equal(
		t,
		"{{ merge (dict) .tasks.primary.output .tasks.secondary.output }}",
		aggregated["result"],
	)
}

func TestAggregateBuilderCustomFunction(t *testing.T) {
	t.Parallel()

	cfg, err := NewAggregate("custom-aggregate").
		AddTask("base").
		WithFunction("customAggregate .tasks").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	aggregated, ok := (*cfg.Outputs)[shared.FieldAggregated].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, aggregateStrategyCustom, aggregated[shared.FieldStrategy])
	assert.Equal(t, "customAggregate .tasks", aggregated["function"])
	assert.Equal(t, "{{ customAggregate .tasks }}", aggregated["result"])
}

func TestAggregateBuilderAddTaskChaining(t *testing.T) {
	t.Parallel()

	builder := NewAggregate("chain")
	builder = builder.AddTask("first").AddTask("second").AddTask("third")

	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	aggregated, ok := (*cfg.Outputs)[shared.FieldAggregated].(map[string]any)
	require.True(t, ok)
	tasksMap, ok := aggregated[shared.TasksKey].(map[string]any)
	require.True(t, ok)
	assert.Len(t, tasksMap, 3)
}

func TestAggregateBuilderBuildFailsNoTasks(t *testing.T) {
	t.Parallel()

	_, err := NewAggregate("empty").Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "at least one task reference")
}

func TestAggregateBuilderBuildFailsInvalidStrategy(t *testing.T) {
	t.Parallel()

	_, err := NewAggregate("invalid").
		AddTask("source").
		WithStrategy("unknown").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "invalid aggregation strategy")
}

func TestAggregateBuilderBuildFailsCustomWithoutFunction(t *testing.T) {
	t.Parallel()

	_, err := NewAggregate("custom-missing").
		AddTask("input").
		WithStrategy("custom").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "requires a function")
}

func TestAggregateBuilderAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewAggregate("   ").
		AddTask("dup").
		AddTask("dup").
		AddTask("   ").
		WithStrategy("custom").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)

	unwrapped := errors.Unwrap(buildErr)
	require.NotNil(t, unwrapped)
}
