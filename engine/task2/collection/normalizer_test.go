package collection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestCollectionNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create collection normalizer", func(t *testing.T) {
		// Arrange
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Act
		normalizer := collection.NewNormalizer(templateEngine, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeCollection), string(normalizer.Type()))
	})
}

func TestCollectionNormalizer_Normalize(t *testing.T) {
	// Setup
	tplEngine := tplengine.NewEngine(tplengine.FormatText)
	templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	normalizer := collection.NewNormalizer(templateEngine, contextBuilder)

	t.Run("Should normalize collection task with array items", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection1",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "[\"item1\", \"item2\", \"item3\"]",
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "process-{{ .item }}",
					Type: task.TaskTypeBasic,
				},
				BasicTask: task.BasicTask{
					Action: "process",
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
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, taskConfig.Task)
	})

	t.Run("Should normalize collection task with string items template", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection1",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "{{ .workflow.input.items }}",
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "process-{{ .item }}",
					Type: task.TaskTypeBasic,
				},
				BasicTask: task.BasicTask{
					Action: "process",
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"items": []string{"a", "b", "c"},
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
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		require.NoError(t, err)
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
