package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestNormalizationContext_BuildTemplateContext(t *testing.T) {
	t.Run("Should return empty map for nil variables", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			Variables: nil,
		}
		// Act
		result := ctx.BuildTemplateContext()
		// Assert
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("Should return variables directly", func(t *testing.T) {
		// Arrange
		variables := map[string]any{
			"test_var": "test_value",
			"nested": map[string]any{
				"key": "value",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: variables,
		}
		// Act
		result := ctx.BuildTemplateContext()
		// Assert
		assert.Equal(t, variables, result)
		assert.Equal(t, "test_value", result["test_var"])
		assert.Equal(t, "value", result["nested"].(map[string]any)["key"])
	})
}

func TestVariableBuilder_BuildBaseVariables(t *testing.T) {
	t.Run("Should build variables with workflow and task data", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		workflowID := "test-workflow"
		workflowExecID := enginecore.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         enginecore.StatusRunning,
			Input:          &enginecore.Input{"workflow_input": "value"},
			Output:         &enginecore.Output{"workflow_output": "result"},
		}
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Opts: workflow.Opts{
				Env: &enginecore.EnvMap{
					"WORKFLOW_ENV": "workflow_value",
				},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &enginecore.Input{"task_input": "task_value"},
				Env:  &enginecore.EnvMap{"TASK_ENV": "task_value"},
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		// Act
		result := builder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)
		// Assert
		assert.NotNil(t, result)

		// Check workflow variables
		workflowVars := result["workflow"].(map[string]any)
		assert.Equal(t, workflowID, workflowVars["id"])
		assert.Equal(t, enginecore.StatusRunning, workflowVars["status"])
		// Input should be dereferenced for template access
		assert.Equal(t, *workflowState.Input, workflowVars["input"])
		assert.Equal(t, workflowState.Output, workflowVars["output"])
		assert.Equal(t, workflowConfig, workflowVars["config"])

		// Check task variables
		taskVars := result["task"].(map[string]any)
		assert.Equal(t, "test-task", taskVars["id"])
		assert.Equal(t, task.TaskTypeBasic, taskVars["type"])
		assert.Equal(t, "test-action", taskVars["action"])
		assert.Equal(t, taskConfig.With, taskVars["with"])
		assert.Equal(t, taskConfig.Env, taskVars["env"])

		// Check env variables
		assert.Equal(t, workflowConfig.Opts.Env, result["env"])
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		result := builder.BuildBaseVariables(nil, nil, taskConfig)
		// Assert
		assert.NotNil(t, result)
		assert.NotContains(t, result, "workflow")
		assert.Contains(t, result, "task")
		assert.NotContains(t, result, "env")
	})

	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Status:     enginecore.StatusRunning,
		}
		workflowConfig := &workflow.Config{ID: "test-workflow"}
		// Act
		result := builder.BuildBaseVariables(workflowState, workflowConfig, nil)
		// Assert
		assert.NotNil(t, result)
		assert.Contains(t, result, "workflow")
		assert.NotContains(t, result, "task")
	})
}

func TestVariableBuilder_AddCurrentInputToVariables(t *testing.T) {
	t.Run("Should add current input to variables", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		vars := make(map[string]any)
		input := &enginecore.Input{
			"field1": "value1",
			"field2": "value2",
		}
		// Act
		builder.AddCurrentInputToVariables(vars, input)
		// Assert
		// Input should be dereferenced for template access
		assert.Equal(t, *input, vars["input"])
		assert.NotContains(t, vars, "item")
		assert.NotContains(t, vars, "index")
	})

	t.Run("Should extract item and index for collection tasks", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		vars := make(map[string]any)
		input := &enginecore.Input{
			"item":  "collection_item",
			"index": 42,
			"other": "data",
		}
		// Act
		builder.AddCurrentInputToVariables(vars, input)
		// Assert
		// Input should be dereferenced for template access
		assert.Equal(t, *input, vars["input"])
		assert.Equal(t, "collection_item", vars["item"])
		assert.Equal(t, 42, vars["index"])
		assert.NotContains(t, vars, "other")
	})

	t.Run("Should handle nil input", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		vars := make(map[string]any)
		// Act
		builder.AddCurrentInputToVariables(vars, nil)
		// Assert
		assert.NotContains(t, vars, "input")
		assert.NotContains(t, vars, "item")
		assert.NotContains(t, vars, "index")
	})
}

func TestVariableBuilder_AddParentToVariables(t *testing.T) {
	t.Run("Should add parent context to variables", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		vars := make(map[string]any)
		parentContext := map[string]any{
			"parent_field": "parent_value",
			"nested": map[string]any{
				"deep": "value",
			},
		}
		// Act
		builder.AddParentToVariables(vars, parentContext)
		// Assert
		assert.Equal(t, parentContext, vars["parent"])
	})

	t.Run("Should handle nil parent context", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		vars := make(map[string]any)
		// Act
		builder.AddParentToVariables(vars, nil)
		// Assert
		assert.NotContains(t, vars, "parent")
	})
}

func TestVariableBuilder_CopyVariables(t *testing.T) {
	t.Run("Should create deep copy of variables", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		source := map[string]any{
			"simple": "value",
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		// Act
		result, err := builder.CopyVariables(source)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, source, result)

		// Verify it's a deep copy by modifying original
		source["simple"] = "modified"
		source["nested"].(map[string]any)["key"] = "modified_nested"

		assert.Equal(t, "value", result["simple"])
		assert.Equal(t, "nested_value", result["nested"].(map[string]any)["key"])
	})

	t.Run("Should handle nil source", func(t *testing.T) {
		// Arrange
		builder := shared.NewVariableBuilder()
		// Act
		result, err := builder.CopyVariables(nil)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})
}

func TestComplexTemplateContextPropagation(t *testing.T) {
	t.Run("Should handle complex nested template scenarios", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}

		// Create complex nested context with workflow, task, parent, and collection data
		workflowState := &workflow.State{
			WorkflowID: "parent-workflow",
			Status:     enginecore.StatusRunning,
			Input:      &enginecore.Input{"global_param": "global_value"},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeCollection,
				With: &enginecore.Input{
					"template_field":  "{{ .workflow.input.global_param }}_processed",
					"nested_template": "{{ .parent.output.result }}_child",
					"collection_ref":  "item_{{ .item }}_at_{{ .index }}",
				},
			},
		}

		// Build complex variables structure
		builder := shared.NewVariableBuilder()
		variables := builder.BuildBaseVariables(workflowState, nil, taskConfig)

		// Add parent context
		parentContext := map[string]any{
			"output": map[string]any{
				"result": "parent_result",
			},
		}
		builder.AddParentToVariables(variables, parentContext)

		// Add collection-specific input
		collectionInput := &enginecore.Input{
			"item":  "collection_item_1",
			"index": 0,
		}
		builder.AddCurrentInputToVariables(variables, collectionInput)

		ctx := &shared.NormalizationContext{
			Variables: variables,
		}

		// Act - Test template processing
		templateContext := ctx.BuildTemplateContext()

		// Test individual template expressions
		template1 := "{{ .workflow.input.global_param }}_processed"
		result1, err1 := templateEngine.ParseAny(template1, templateContext)

		template2 := "{{ .parent.output.result }}_child"
		result2, err2 := templateEngine.ParseAny(template2, templateContext)

		template3 := "item_{{ .item }}_at_{{ .index }}"
		result3, err3 := templateEngine.ParseAny(template3, templateContext)

		// Assert
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)

		assert.Equal(t, "global_value_processed", result1)
		assert.Equal(t, "parent_result_child", result2)
		assert.Equal(t, "item_collection_item_1_at_0", result3)
	})

	t.Run("Should handle deeply nested context hierarchy", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}

		// Create a complex 3-level nested hierarchy
		variables := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"value": "deep_value",
					},
					"array": []any{"item1", "item2", "item3"},
				},
				"simple": "level1_value",
			},
			"workflow": map[string]any{
				"config": map[string]any{
					"settings": map[string]any{
						"timeout": "30s",
						"retries": 3,
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{Variables: variables}
		templateContext := ctx.BuildTemplateContext()

		testCases := []struct {
			name     string
			template string
			expected string
		}{
			{
				"Deep nested access",
				"{{ .level1.level2.level3.value }}",
				"deep_value",
			},
			{
				"Array index access",
				"{{ index .level1.level2.array 1 }}",
				"item2",
			},
			{
				"Complex nested config",
				"timeout_{{ .workflow.config.settings.timeout }}_retries_{{ .workflow.config.settings.retries }}",
				"timeout_30s_retries_3",
			},
			{
				"Mixed access patterns",
				"{{ .level1.simple }}_{{ .level1.level2.level3.value }}",
				"level1_value_deep_value",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				result, err := templateEngine.ParseAny(tc.template, templateContext)
				// Assert
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Should handle template errors gracefully", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		variables := map[string]any{
			"existing": "value",
		}
		ctx := &shared.NormalizationContext{Variables: variables}
		templateContext := ctx.BuildTemplateContext()

		errorCases := []struct {
			name     string
			template string
		}{
			{"Undefined variable", "{{ .nonexistent.field }}"},
			{"Deep undefined path", "{{ .existing.nonexistent.deep }}"},
		}

		for _, tc := range errorCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				_, err := templateEngine.ParseAny(tc.template, templateContext)
				// Assert
				assert.Error(t, err, "Expected error for template: %s", tc.template)
			})
		}
	})
}
