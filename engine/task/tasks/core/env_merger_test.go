package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tmplcore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/workflow"
)

func TestEnvMerger_NewEnvMerger(t *testing.T) {
	t.Run("Should create env merger", func(t *testing.T) {
		// Act
		merger := tmplcore.NewEnvMerger()

		// Assert
		assert.NotNil(t, merger)
	})
}

func TestEnvMerger_MergeWorkflowToTask(t *testing.T) {
	merger := tmplcore.NewEnvMerger()

	t.Run("Should merge workflow env to task env", func(t *testing.T) {
		// Arrange
		workflowEnv := &core.EnvMap{
			"WORKFLOW_VAR": "workflow_value",
			"SHARED_VAR":   "workflow_shared",
		}
		taskEnv := &core.EnvMap{
			"TASK_VAR":   "task_value",
			"SHARED_VAR": "task_shared", // Should override workflow value
		}
		workflowConfig := &workflow.Config{
			Opts: workflow.Opts{
				Env: workflowEnv,
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Env: taskEnv,
			},
		}

		// Act
		merged := merger.MergeWorkflowToTask(workflowConfig, taskConfig)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "workflow_value", (*merged)["WORKFLOW_VAR"])
		assert.Equal(t, "task_value", (*merged)["TASK_VAR"])
		assert.Equal(t, "task_shared", (*merged)["SHARED_VAR"]) // Task overrides workflow
	})

	t.Run("Should handle nil workflow env", func(t *testing.T) {
		// Arrange
		taskEnv := &core.EnvMap{
			"TASK_VAR": "task_value",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Env: taskEnv,
			},
		}

		// Act
		merged := merger.MergeWorkflowToTask(nil, taskConfig)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "task_value", (*merged)["TASK_VAR"])
	})

	t.Run("Should handle nil task env", func(t *testing.T) {
		// Arrange
		workflowEnv := &core.EnvMap{
			"WORKFLOW_VAR": "workflow_value",
		}
		workflowConfig := &workflow.Config{
			Opts: workflow.Opts{
				Env: workflowEnv,
			},
		}

		// Act
		merged := merger.MergeWorkflowToTask(workflowConfig, nil)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "workflow_value", (*merged)["WORKFLOW_VAR"])
	})

	t.Run("Should handle both nil envs", func(t *testing.T) {
		// Act
		merged := merger.MergeWorkflowToTask(nil, nil)

		// Assert
		require.NotNil(t, merged)
		assert.Empty(t, *merged)
	})
}

func TestEnvMerger_MergeThreeLevels(t *testing.T) {
	merger := tmplcore.NewEnvMerger()

	t.Run("Should merge three levels of env variables", func(t *testing.T) {
		// Arrange
		workflowEnv := &core.EnvMap{
			"WORKFLOW_VAR": "workflow_value",
			"SHARED_VAR":   "workflow_shared",
		}
		taskEnv := &core.EnvMap{
			"TASK_VAR":   "task_value",
			"SHARED_VAR": "task_shared",
		}
		componentEnv := &core.EnvMap{
			"COMPONENT_VAR": "component_value",
			"SHARED_VAR":    "component_shared", // Should override both
		}
		workflowConfig := &workflow.Config{
			Opts: workflow.Opts{
				Env: workflowEnv,
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Env: taskEnv,
			},
		}

		// Act
		merged := merger.MergeThreeLevels(workflowConfig, taskConfig, componentEnv)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "workflow_value", (*merged)["WORKFLOW_VAR"])
		assert.Equal(t, "task_value", (*merged)["TASK_VAR"])
		assert.Equal(t, "component_value", (*merged)["COMPONENT_VAR"])
		assert.Equal(t, "component_shared", (*merged)["SHARED_VAR"]) // Component has highest priority
	})

	t.Run("Should handle partial nil environments", func(t *testing.T) {
		// Arrange
		workflowEnv := &core.EnvMap{
			"WORKFLOW_VAR": "workflow_value",
		}
		componentEnv := &core.EnvMap{
			"COMPONENT_VAR": "component_value",
		}
		workflowConfig := &workflow.Config{
			Opts: workflow.Opts{
				Env: workflowEnv,
			},
		}

		// Act
		merged := merger.MergeThreeLevels(workflowConfig, nil, componentEnv)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "workflow_value", (*merged)["WORKFLOW_VAR"])
		assert.Equal(t, "component_value", (*merged)["COMPONENT_VAR"])
	})
}

func TestEnvMerger_MergeForComponent(t *testing.T) {
	merger := tmplcore.NewEnvMerger()

	t.Run("Should merge environment for component", func(t *testing.T) {
		// Arrange
		mergedTaskEnv := &core.EnvMap{
			"TASK_VAR":   "task_value",
			"SHARED_VAR": "task_shared",
		}
		componentEnv := &core.EnvMap{
			"COMPONENT_VAR": "component_value",
			"SHARED_VAR":    "component_shared",
		}

		// Act
		merged := merger.MergeForComponent(mergedTaskEnv, componentEnv)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "task_value", (*merged)["TASK_VAR"])
		assert.Equal(t, "component_value", (*merged)["COMPONENT_VAR"])
		assert.Equal(t, "component_shared", (*merged)["SHARED_VAR"]) // Component overrides task
	})

	t.Run("Should handle nil component env", func(t *testing.T) {
		// Arrange
		mergedTaskEnv := &core.EnvMap{
			"TASK_VAR": "task_value",
		}

		// Act
		merged := merger.MergeForComponent(mergedTaskEnv, nil)

		// Assert
		require.NotNil(t, merged)
		assert.Equal(t, "task_value", (*merged)["TASK_VAR"])
	})
}
