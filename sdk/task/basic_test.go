package task

import (
	"errors"
	"testing"

	enginetask "github.com/compozy/compozy/engine/task"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		builder *BasicBuilder
		check   func(t *testing.T, cfg *enginetask.Config)
	}{
		{
			name: "agent execution",
			builder: NewBasic("greet").
				WithAgent("assistant").
				WithAction("greet").
				WithInput(map[string]string{"name": "{{ .workflow.input.name }}"}).
				WithOutput("summary={{ .task.output.summary }}").
				WithCondition(" input.name != \"\" ").
				WithFinal(true),
			check: func(t *testing.T, cfg *enginetask.Config) {
				t.Helper()
				require.NotNil(t, cfg.Agent)
				assert.Equal(t, "assistant", cfg.Agent.ID)
				assert.Nil(t, cfg.Tool)
				assert.Equal(t, "greet", cfg.Action)
				require.NotNil(t, cfg.With)
				assert.Equal(t, "{{ .workflow.input.name }}", (*cfg.With)["name"])
				require.NotNil(t, cfg.Outputs)
				assert.Equal(t, "{{ .task.output.summary }}", (*cfg.Outputs)["summary"])
				assert.Equal(t, `input.name != ""`, cfg.Condition)
				assert.True(t, cfg.Final)
			},
		},
		{
			name: "tool execution",
			builder: NewBasic("resize").
				WithTool("image-resizer").
				WithInput(map[string]string{"path": "{{ .workflow.input.path }}"}),
			check: func(t *testing.T, cfg *enginetask.Config) {
				t.Helper()
				require.NotNil(t, cfg.Tool)
				assert.Equal(t, "image-resizer", cfg.Tool.ID)
				assert.Nil(t, cfg.Agent)
				assert.Empty(t, cfg.Action)
				require.NotNil(t, cfg.With)
				assert.Equal(t, "{{ .workflow.input.path }}", (*cfg.With)["path"])
				assert.False(t, cfg.Final)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := tc.builder.Build(t.Context())
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, enginetask.TaskTypeBasic, cfg.Type)
			tc.check(t, cfg)
		})
	}
}

func TestBasicBuilderWithOutputDefaultAlias(t *testing.T) {
	t.Parallel()

	cfg, err := NewBasic("script").
		WithTool("shell-runner").
		WithOutput("{{ .task.output }}").
		Build(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg.Outputs)
	assert.Equal(t, "{{ .task.output }}", (*cfg.Outputs)[defaultOutputAlias])
}

func TestBasicBuilderBuildFailsWhenMissingExecutor(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("orphan").Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "either an agent or a tool")
}

func TestBasicBuilderBuildFailsWhenBothAgentAndTool(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("mixed").
		WithAgent("assistant").
		WithAction("greet").
		WithTool("email-sender").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "cannot reference both an agent and a tool")
}

func TestBasicBuilderBuildFailsForEmptyID(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("   ").
		WithTool("runner").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "task id is invalid")
}

func TestBasicBuilderBuildFailsForAgentWithoutAction(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("greet").
		WithAgent("assistant").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "must specify an action")
}

func TestBasicBuilderBuildFailsForToolWithAction(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("export").
		WithTool("s3-uploader").
		WithAction("upload").
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "cannot specify an action")
}

func TestBasicBuilderBuildAggregatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("  ").
		WithTool("tool-1").
		WithAction("do").
		WithInput(map[string]string{"  ": "value"}).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 2)

	for _, aggregated := range buildErr.Errors {
		assert.NotNil(t, aggregated)
	}
}

func TestBasicBuilderErrorImplementsUnwrap(t *testing.T) {
	t.Parallel()

	_, err := NewBasic("   ").
		WithTool("runner").
		WithInput(map[string]string{"": "value"}).
		Build(t.Context())
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotNil(t, buildErr)

	unwrapped := errors.Unwrap(buildErr)
	require.NotNil(t, unwrapped)
}
