package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestOutputTransformer_TransformOutput(t *testing.T) {
	t.Run("Should return output unchanged when outputsConfig is nil", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{"key": "value"}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, nil, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, output, result)
	})

	t.Run("Should return output unchanged when output is nil", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		outputsConfig := &enginecore.Input{"template": "{{ .output.key }}"}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{}
		// Act
		result, err := transformer.TransformOutput(t.Context(), nil, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should transform output for regular task", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{"key": "value"}
		outputsConfig := &enginecore.Input{
			"transformed": "{{ .output.key }}_transformed",
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "value_transformed", (*result)["transformed"])
	})

	t.Run("Should handle collection task with nested outputs", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{
			"outputs": map[string]any{
				"child1": map[string]any{"result": "result1"},
				"child2": map[string]any{"result": "result2"},
			},
		}
		outputsConfig := &enginecore.Input{
			"summary": "{{ len .output }}",
		}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "2", (*result)["summary"])
	})

	t.Run("Should handle collection task without aggregated outputs", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{"raw": "data"}
		outputsConfig := &enginecore.Input{
			"count": "{{ len .output }}",
		}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "0", (*result)["count"])
	})

	t.Run("Should handle parallel task with nested outputs", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{
			"outputs": map[string]any{
				"parallel1": map[string]any{"result": "result1"},
				"parallel2": map[string]any{"result": "result2"},
			},
		}
		outputsConfig := &enginecore.Input{
			"combined": "{{ index .output \"parallel1\" }}_{{ index .output \"parallel2\" }}",
		}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, (*result)["combined"], "result")
	})

	t.Run("Should handle collection task with children context", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{
			"outputs": map[string]any{
				"child1": map[string]any{"result": "result1"},
			},
		}
		outputsConfig := &enginecore.Input{
			"children_count": "{{ len .children }}",
		}
		taskState := &task.State{
			TaskID:        "collection-task",
			ExecutionType: task.ExecutionCollection,
			Status:        enginecore.StatusSuccess,
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				Tasks: map[string]*task.State{
					"collection-task": taskState,
				},
			},
			ChildrenIndex: map[string][]string{
				"collection-task": {"child1"},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestTransformOutputFields(t *testing.T) {
	t.Run("Should transform output fields with template engine", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		outputsConfig := map[string]any{
			"field1": "{{ .data.value }}",
			"field2": "prefix_{{ .data.name }}_suffix",
		}
		transformCtx := map[string]any{
			"data": map[string]any{
				"value": "test_value",
				"name":  "test_name",
			},
		}
		// Act
		result, err := core.TransformOutputFields(templateEngine, outputsConfig, transformCtx, "test")
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test_value", result["field1"])
		assert.Equal(t, "prefix_test_name_suffix", result["field2"])
	})

	t.Run("Should handle empty outputsConfig", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		outputsConfig := map[string]any{}
		transformCtx := map[string]any{}
		// Act
		result, err := core.TransformOutputFields(templateEngine, outputsConfig, transformCtx, "test")
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("Should maintain deterministic order with sorted keys", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		outputsConfig := map[string]any{
			"z_field": "{{ .value }}",
			"a_field": "{{ .value }}",
			"m_field": "{{ .value }}",
		}
		transformCtx := map[string]any{
			"value": "test",
		}
		// Act
		result, err := core.TransformOutputFields(templateEngine, outputsConfig, transformCtx, "test")
		// Assert
		assert.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Equal(t, "test", result["a_field"])
		assert.Equal(t, "test", result["m_field"])
		assert.Equal(t, "test", result["z_field"])
	})
}

func TestOutputTransformer_TemplateError(t *testing.T) {
	t.Run("Should return error when template parsing fails", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		transformer := core.NewOutputTransformer(templateEngine)
		output := &enginecore.Output{"key": "value"}
		outputsConfig := &enginecore.Input{"field": "{{ .invalid.nonexistent }}"}
		ctx := &shared.NormalizationContext{}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{Type: task.TaskTypeBasic},
		}
		// Act
		result, err := transformer.TransformOutput(t.Context(), output, outputsConfig, ctx, taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to transform task output field")
	})
}
