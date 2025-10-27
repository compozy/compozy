package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	engineruntime "github.com/compozy/compozy/engine/runtime"
)

func TestNewNativeToolsCreatesEmptyConfig(t *testing.T) {
	t.Parallel()

	builder := NewNativeTools()
	require.NotNil(t, builder)
	require.NotNil(t, builder.config)
	require.False(t, builder.config.CallAgents)
	require.False(t, builder.config.CallWorkflows)
}

func TestWithCallAgentsEnablesTool(t *testing.T) {
	t.Parallel()

	builder := NewNativeTools().WithCallAgents()
	require.True(t, builder.config.CallAgents)
	require.False(t, builder.config.CallWorkflows)
}

func TestWithCallWorkflowsEnablesTool(t *testing.T) {
	t.Parallel()

	builder := NewNativeTools().WithCallWorkflows()
	require.True(t, builder.config.CallWorkflows)
	require.False(t, builder.config.CallAgents)
}

func TestBuildReturnsConfigWhenContextProvided(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	builder := NewNativeTools().WithCallAgents().WithCallWorkflows()
	config := builder.Build(ctx)
	require.NotNil(t, config)
	require.NotSame(t, builder.config, config)
	require.True(t, config.CallAgents)
	require.True(t, config.CallWorkflows)
}

func TestBuildReturnsNilWhenContextMissing(t *testing.T) {
	t.Parallel()

	builder := NewNativeTools()
	config := builder.Build(nil)
	require.Nil(t, config)
}

func TestBuildHandlesNoToolsEnabled(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	config := NewNativeTools().Build(ctx)
	require.NotNil(t, config)
	require.False(t, config.CallAgents)
	require.False(t, config.CallWorkflows)
}

func TestWithCallAgentsCopiesValuesIntoRuntimeBuilder(t *testing.T) {
	t.Parallel()

	tools := &engineruntime.NativeToolsConfig{CallAgents: true}
	builder := NewBun().WithEntrypoint("./main.ts").WithNativeTools(tools)
	require.NotNil(t, builder.config.NativeTools)
	require.True(t, builder.config.NativeTools.CallAgents)
	require.NotSame(t, tools, builder.config.NativeTools)
}
