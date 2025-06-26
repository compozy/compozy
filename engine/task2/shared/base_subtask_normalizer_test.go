package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// testSubTaskNormalizer is a simple implementation for testing
type testSubTaskNormalizer struct {
	taskType string
}

func (n *testSubTaskNormalizer) Normalize(_ any, _ *shared.NormalizationContext) error {
	return nil
}

func (n *testSubTaskNormalizer) Type() string {
	return n.taskType
}

// testSubTaskNormalizerFactory is a simple implementation for testing
type testSubTaskNormalizerFactory struct{}

func (f *testSubTaskNormalizerFactory) CreateNormalizer(taskType string) (shared.TaskNormalizer, error) {
	return &testSubTaskNormalizer{taskType: taskType}, nil
}

func TestBaseSubTaskNormalizer_NewBaseSubTaskNormalizer(t *testing.T) {
	t.Run("Should create base sub-task normalizer", func(t *testing.T) {
		// Arrange
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		factory := &testSubTaskNormalizerFactory{}

		// Act
		normalizer := shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			factory,
			task.TaskTypeParallel,
			"parallel",
		)

		// Assert
		assert.NotNil(t, normalizer)
		assert.Equal(t, task.TaskTypeParallel, normalizer.Type())
	})
}

func TestBaseSubTaskNormalizer_Normalize(t *testing.T) {
	// Setup
	tplEngine := tplengine.NewEngine(tplengine.FormatText)
	templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	factory := &testSubTaskNormalizerFactory{}

	normalizer := shared.NewBaseSubTaskNormalizer(
		templateEngine,
		contextBuilder,
		factory,
		task.TaskTypeParallel,
		"parallel",
	)

	t.Run("Should normalize task with sub-tasks", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel1",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"prefix": "task",
				},
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sub1",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "action1",
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"global": "value",
			},
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		ctx := &shared.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfig:     taskConfig,
			Variables: map[string]any{
				"workflow": map[string]any{
					"id":    "test-workflow",
					"input": workflowState.Input,
				},
				"with": taskConfig.With,
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		require.Len(t, taskConfig.Tasks, 1)
		assert.Equal(t, "sub1", taskConfig.Tasks[0].ID)
	})

	t.Run("Should handle nil config", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.Normalize(nil, ctx)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wrong-type",
				Type: task.TaskTypeBasic, // Wrong type
			},
		}
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel normalizer cannot handle task type")
	})

	t.Run("Should handle task with no sub-tasks", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel1",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, taskConfig.Tasks)
	})
}
