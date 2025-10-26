package task

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelBuilderBuildWaitAll(t *testing.T) {
	t.Parallel()

	cfg, err := NewParallel("process").
		AddTask("fetch-user").
		AddTask("fetch-orders").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeParallel, cfg.Type)
	assert.Equal(t, enginetask.StrategyWaitAll, cfg.ParallelTask.GetStrategy())
	require.Len(t, cfg.Tasks, 2)
	assert.Equal(t, "fetch-user", cfg.Tasks[0].ID)
	assert.Equal(t, "fetch-orders", cfg.Tasks[1].ID)
	for _, child := range cfg.Tasks {
		assert.Equal(t, string(core.ConfigTask), child.Resource)
		assert.Empty(t, child.Type)
	}
}

func TestParallelBuilderBuildWaitFirst(t *testing.T) {
	t.Parallel()

	cfg, err := NewParallel("process-race").
		WithWaitAll(false).
		AddTask("primary").
		AddTask("secondary").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.StrategyRace, cfg.ParallelTask.GetStrategy())
}

func TestParallelBuilderAddTaskChaining(t *testing.T) {
	t.Parallel()

	builder := NewParallel("sync")
	builder = builder.AddTask("a").AddTask("b")
	cfg, err := builder.Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Tasks, 2)
	assert.Equal(t, "a", cfg.Tasks[0].ID)
	assert.Equal(t, "b", cfg.Tasks[1].ID)
}

func TestParallelBuilderBuildFailsNoTasks(t *testing.T) {
	t.Parallel()

	_, err := NewParallel("empty").Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, buildErr.Error(), "at least one child task")
}

func TestParallelBuilderBuildFailsEmptyTaskID(t *testing.T) {
	t.Parallel()

	_, err := NewParallel("loader").
		AddTask("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "task id cannot be empty")
}

func TestParallelBuilderBuildFailsDuplicateTask(t *testing.T) {
	t.Parallel()

	_, err := NewParallel("dup").
		AddTask("cache").
		AddTask("cache").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.ErrorContains(t, buildErr, "duplicate task id")
}

func TestParallelBuilderBuildAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewParallel("   ").
		AddTask("cache").
		AddTask("cache").
		AddTask("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 2)

	unwrapped := errors.Unwrap(buildErr)
	require.NotNil(t, unwrapped)
}
