package workflow_test

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/v2/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MinimalConfig(t *testing.T) {
	t.Run("Should create workflow with single task", func(t *testing.T) {
		cfg, err := workflow.New(t.Context(), "test-workflow",
			workflow.WithTasks([]task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			}),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", cfg.ID)
		assert.Len(t, cfg.Tasks, 1)
		assert.Equal(t, "task1", cfg.Tasks[0].ID)
	})
}

func TestNew_FullConfig(t *testing.T) {
	t.Run("Should create workflow with all options", func(t *testing.T) {
		author := &core.Author{Name: "Test Author", Email: "test@example.com"}
		inputSchema := &schema.Schema{"type": "object"}
		outputs := &core.Output{"result": "{{ .tasks.task1.output }}"}
		schedule := &engineworkflow.Schedule{Cron: "0 0 * * *"}
		cfg, err := workflow.New(t.Context(), "full-workflow",
			workflow.WithVersion("1.0.0"),
			workflow.WithDescription("A full workflow configuration"),
			workflow.WithOpts(engineworkflow.Opts{InputSchema: inputSchema}),
			workflow.WithAuthor(author),
			workflow.WithTools([]tool.Config{
				{
					ID:          "tool1",
					Name:        "Test Tool",
					Description: "A test tool",
					Runtime:     "bun",
					Code:        "console.log('test')",
				},
			}),
			workflow.WithAgents([]agent.Config{
				{ID: "agent1", Instructions: "Test instructions"},
			}),
			workflow.WithTasks([]task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
				{BaseConfig: task.BaseConfig{ID: "task2", Type: task.TaskTypeBasic}},
			}),
			workflow.WithOutputs(outputs),
			workflow.WithSchedule(schedule),
		)
		require.NoError(t, err)
		assert.Equal(t, "full-workflow", cfg.ID)
		assert.Equal(t, "1.0.0", cfg.Version)
		assert.Equal(t, "A full workflow configuration", cfg.Description)
		assert.Equal(t, inputSchema, cfg.Opts.InputSchema)
		assert.Equal(t, author, cfg.Author)
		assert.Len(t, cfg.Tools, 1)
		assert.Equal(t, "tool1", cfg.Tools[0].ID)
		assert.Len(t, cfg.Agents, 1)
		assert.Equal(t, "agent1", cfg.Agents[0].ID)
		assert.Len(t, cfg.Tasks, 2)
		assert.Equal(t, outputs, cfg.Outputs)
		assert.Equal(t, schedule, cfg.Schedule)
	})
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		opts    []workflow.Option
		wantErr string
	}{
		{
			name: "empty id",
			id:   "",
			opts: []workflow.Option{workflow.WithTasks([]task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			})},
			wantErr: "workflow id is invalid",
		},
		{
			name:    "no tasks",
			id:      "test-workflow",
			opts:    []workflow.Option{},
			wantErr: "at least one task must be registered",
		},
		{
			name: "duplicate task ids",
			id:   "test-workflow",
			opts: []workflow.Option{
				workflow.WithTasks([]task.Config{
					{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
					{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
				}),
			},
			wantErr: "duplicate task ids found: task1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := workflow.New(t.Context(), tt.id, tt.opts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_NilContext(t *testing.T) {
	t.Run("Should return error for nil context", func(t *testing.T) {
		_, err := workflow.New(nil, "test-workflow")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
}

func TestNew_DeepCopy(t *testing.T) {
	t.Run("Should return independent copy of configuration", func(t *testing.T) {
		tasks := []task.Config{
			{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
		}
		cfg1, err := workflow.New(t.Context(), "test-workflow", workflow.WithTasks(tasks))
		require.NoError(t, err)
		cfg1.Tasks[0].ID = "modified"
		cfg2, err := workflow.New(t.Context(), "test-workflow", workflow.WithTasks(tasks))
		require.NoError(t, err)
		assert.NotEqual(t, cfg1.Tasks[0].ID, cfg2.Tasks[0].ID)
		assert.Equal(t, "task1", cfg2.Tasks[0].ID)
	})
}

func TestNew_MultipleErrors(t *testing.T) {
	t.Run("Should collect all validation errors", func(t *testing.T) {
		_, err := workflow.New(t.Context(), "", workflow.WithTasks([]task.Config{
			{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
		}))
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.Len(t, buildErr.Errors, 2)
	})
}

func TestNew_TaskCollectionHandling(t *testing.T) {
	t.Run("Should handle multiple tasks correctly", func(t *testing.T) {
		tasks := []task.Config{
			{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			{BaseConfig: task.BaseConfig{ID: "task2", Type: task.TaskTypeParallel}},
			{BaseConfig: task.BaseConfig{ID: "task3", Type: task.TaskTypeWait}},
		}
		cfg, err := workflow.New(t.Context(), "multi-task", workflow.WithTasks(tasks))
		require.NoError(t, err)
		assert.Len(t, cfg.Tasks, 3)
		assert.Equal(t, "task1", cfg.Tasks[0].ID)
		assert.Equal(t, "task2", cfg.Tasks[1].ID)
		assert.Equal(t, "task3", cfg.Tasks[2].ID)
	})
}

func TestNew_AgentCollection(t *testing.T) {
	t.Run("Should handle multiple agents correctly", func(t *testing.T) {
		agents := []agent.Config{
			{ID: "agent1", Instructions: "First agent"},
			{ID: "agent2", Instructions: "Second agent"},
		}
		cfg, err := workflow.New(t.Context(), "multi-agent",
			workflow.WithAgents(agents),
			workflow.WithTasks([]task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			}),
		)
		require.NoError(t, err)
		assert.Len(t, cfg.Agents, 2)
		assert.Equal(t, "agent1", cfg.Agents[0].ID)
		assert.Equal(t, "agent2", cfg.Agents[1].ID)
	})
}

func TestNew_WhitespaceTrimming(t *testing.T) {
	t.Run("Should trim whitespace from id", func(t *testing.T) {
		cfg, err := workflow.New(t.Context(), "  test-workflow  ",
			workflow.WithTasks([]task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
			}),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-workflow", cfg.ID)
	})
}
