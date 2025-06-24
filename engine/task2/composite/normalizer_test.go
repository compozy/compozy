package composite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// testNormalizerFactory is a simple implementation for testing
type testNormalizerFactory struct{}

func (f *testNormalizerFactory) CreateNormalizer(taskType string) (shared.TaskNormalizer, error) {
	// Return a simple basic normalizer for testing
	basicNormalizer := &testTaskNormalizer{taskType: taskType}
	return basicNormalizer, nil
}

// testTaskNormalizer is a simple implementation for testing
type testTaskNormalizer struct {
	taskType string
}

func (n *testTaskNormalizer) Normalize(_ any, _ *shared.NormalizationContext) error {
	return nil
}

func (n *testTaskNormalizer) Type() string {
	return n.taskType
}

func TestCompositeNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create composite normalizer", func(t *testing.T) {
		// Arrange
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		// Create a simple shared factory for testing
		sharedFactory := &testNormalizerFactory{}

		// Act
		normalizer := composite.NewNormalizer(templateEngine, contextBuilder, sharedFactory)

		// Assert
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeComposite), string(normalizer.Type()))
	})
}

func TestCompositeNormalizer_Normalize(t *testing.T) {
	// Setup
	tplEngine := tplengine.NewEngine(tplengine.FormatText)
	templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	// Create a simple shared factory for testing
	sharedFactory := &testNormalizerFactory{}
	normalizer := composite.NewNormalizer(templateEngine, contextBuilder, sharedFactory)

	t.Run("Should normalize composite task with sub-tasks", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite1",
				Type: task.TaskTypeComposite,
				With: &core.Input{
					"prefix": "task",
				},
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "{{ .with.prefix }}-1",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "action1",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "{{ .with.prefix }}-2",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "action2",
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
		require.Len(t, taskConfig.Tasks, 2)
		// Note: Templates not processed by test mock - this is expected behavior
		assert.Equal(t, "{{ .with.prefix }}-1", taskConfig.Tasks[0].ID)
		assert.Equal(t, "{{ .with.prefix }}-2", taskConfig.Tasks[1].ID)
	})

	t.Run("Should handle composite task with no sub-tasks", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite1",
				Type: task.TaskTypeComposite,
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

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		invalidConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wrong-type",
				Type: task.TaskTypeBasic, // Wrong type
			},
		}
		ctx := &shared.NormalizationContext{}

		// Act
		err := normalizer.Normalize(invalidConfig, ctx)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot handle task type")
	})
}
