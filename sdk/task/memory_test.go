package task

import (
	"testing"

	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryTaskBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	cfg, err := NewMemoryTask("session-append").
		WithOperation("append").
		WithMemory("user-sessions").
		WithContent("{{ .workflow.input.payload }}").
		WithKeyTemplate("session:{{ .workflow.id }}").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.TaskTypeMemory, cfg.Type)
	assert.Equal(t, enginetask.MemoryOpAppend, cfg.Operation)
	assert.Equal(t, "user-sessions", cfg.MemoryRef)
	assert.Equal(t, "session:{{ .workflow.id }}", cfg.KeyTemplate)
	assert.Equal(t, "{{ .workflow.input.payload }}", cfg.Payload)
	assert.Nil(t, cfg.ClearConfig)
}

func TestMemoryTaskBuilderBuildFailsForEmptyOperation(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("read-cache").
		WithMemory("cache-store").
		WithOperation("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "operation")
}

func TestMemoryTaskBuilderBuildFailsForEmptyMemory(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("read-cache").
		WithOperation("read").
		WithMemory("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "memory reference id is invalid")
}

func TestMemoryTaskBuilderAppendRequiresContent(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("append-cache").
		WithOperation("append").
		WithMemory("cache-store").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "append operation requires content")
}

func TestMemoryTaskBuilderClearIgnoresContent(t *testing.T) {
	t.Parallel()

	cfg, err := NewMemoryTask("clear-cache").
		WithContent("should be ignored").
		WithOperation("clear").
		WithMemory("cache-store").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, enginetask.MemoryOpClear, cfg.Operation)
	require.NotNil(t, cfg.ClearConfig)
	assert.True(t, cfg.ClearConfig.Confirm)
	assert.Nil(t, cfg.Payload)
}

func TestMemoryTaskBuilderRequiresContext(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("read-cache").
		WithOperation("read").
		WithMemory("cache-store").
		Build(nil)
	require.Error(t, err)
	assert.Equal(t, "context is required", err.Error())
}

func TestMemoryTaskBuilderAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("   ").
		WithOperation("invalid-op").
		WithMemory("   ").
		WithContent("   ").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
}

func TestMemoryTaskBuilderUsesContextValidation(t *testing.T) {
	t.Parallel()

	_, err := NewMemoryTask("memory-reader").
		WithOperation("read").
		WithMemory("invalid id").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "memory reference id is invalid")
}
