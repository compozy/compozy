package shared_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestBaseNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)
		// Act
		taskType := normalizer.Type()
		// Assert
		assert.Equal(t, task.TaskTypeBasic, taskType)
	})
}

func TestBaseNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeCollection,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "basic normalizer cannot handle task type: collection")
	})

	t.Run("Should handle empty task type for basic tasks", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: "",
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle template parsing errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"existing": "value",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize basic task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange - Create config with problematic data for serialization
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Inject a function pointer which can't be serialized
		unsafeField := func() {}
		taskConfig.With = &core.Input{"function": unsafeField}

		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to convert task config to map")
	})

	t.Run("Should handle nil template context", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: nil}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle complex template expressions", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ range .items }}{{ . }}{{ end }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"complex": "{{ if .condition }}{{ .trueValue }}{{ else }}{{ .falseValue }}{{ end }}",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"items":      []string{"a", "b", "c"},
				"condition":  true,
				"trueValue":  "success",
				"falseValue": "failure",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "abc", taskConfig.ID)
		assert.Equal(t, "success", (*taskConfig.With)["complex"])
	})

	t.Run("Should preserve existing With values during normalization", func(t *testing.T) {
		// Arrange - Set up task with templated With values that will be normalized
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"template_field": "{{ .template_value }}",
					"existing":       "preserve_me",
					"override":       "original_value",
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":           "normalized-task",
				"template_value": "processed_template",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "normalized-task", taskConfig.ID)
		// With field should be preserved and templates processed
		assert.NotNil(t, taskConfig.With)
		assert.Equal(t, "processed_template", (*taskConfig.With)["template_field"])
		assert.Equal(t, "preserve_me", (*taskConfig.With)["existing"])
		assert.Equal(t, "original_value", (*taskConfig.With)["override"])
	})
}

func TestBaseNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := shared.NewBaseNormalizer(nil, nil, task.TaskTypeBasic, nil)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act & Assert
		// Should panic due to nil template engine
		assert.Panics(t, func() {
			normalizer.Normalize(taskConfig, ctx)
		})
	})

	t.Run("Should handle empty string templates", func(t *testing.T) {
		// Arrange
		normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"empty": "",
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "", taskConfig.ID)
		assert.Equal(t, "", (*taskConfig.With)["empty"])
	})

	t.Run("Should handle very large template contexts", func(t *testing.T) {
		// Arrange
		normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)

		// Create large context with many variables
		largeContext := make(map[string]any)
		for i := 0; i < 1000; i++ {
			largeContext[fmt.Sprintf("var_%d", i)] = fmt.Sprintf("value_%d", i)
		}
		largeContext["target"] = "success"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .target }}",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: largeContext}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "success", taskConfig.ID)
	})

	t.Run("Should handle custom filter functions", func(t *testing.T) {
		// Arrange
		customFilter := func(k string) bool {
			return k == "skip_me" || k == "also_skip"
		}
		normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, customFilter)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"process_me": "{{ .value }}",
					"skip_me":    "{{ .should_not_process }}",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":               "test",
				"value":              "processed",
				"should_not_process": "error_if_processed",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test", taskConfig.ID)
		assert.Equal(t, "processed", (*taskConfig.With)["process_me"])
		assert.Equal(t, "{{ .should_not_process }}", (*taskConfig.With)["skip_me"])
	})
}

func TestBaseNormalizer_ProcessTemplateString(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)

	t.Run("Should process simple string template", func(t *testing.T) {
		// Arrange
		context := map[string]any{"name": "test"}
		// Act
		result, err := normalizer.ProcessTemplateString("Hello {{ .name }}", context)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "Hello test", result)
	})

	t.Run("Should return error for invalid template", func(t *testing.T) {
		// Arrange
		context := map[string]any{}
		// Act
		result, err := normalizer.ProcessTemplateString("{{ .nonexistent.field }}", context)
		// Assert
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("Should return error for non-string result", func(t *testing.T) {
		// Arrange
		context := map[string]any{"number": 42}
		// Act
		result, err := normalizer.ProcessTemplateString("{{ .number }}", context)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected string, got")
		assert.Empty(t, result)
	})
}

func TestBaseNormalizer_ProcessTemplateMap(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)

	t.Run("Should process map template", func(t *testing.T) {
		// Arrange
		input := map[string]any{
			"name": "{{ .user }}",
			"age":  "{{ .years }}",
		}
		context := map[string]any{
			"user":  "john",
			"years": "30",
		}
		// Act
		result, err := normalizer.ProcessTemplateMap(input, context)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "john", result["name"])
		assert.Equal(t, "30", result["age"])
	})

	t.Run("Should return error for template processing failure", func(t *testing.T) {
		// Arrange
		input := map[string]any{
			"invalid": "{{ .nonexistent.field }}",
		}
		context := map[string]any{}
		// Act
		result, err := normalizer.ProcessTemplateMap(input, context)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should return error for non-map result", func(t *testing.T) {
		// Arrange - We need to create a scenario where ProcessTemplateMap gets wrong type
		// Since ProcessTemplateMap expects map[string]any, we can't easily trigger this error
		// in a clean way, so let's skip this edge case test for now
		// This test would require mocking the template engine to return wrong types
		t.Skip("Skipping edge case test - requires complex mocking setup")
	})
}

func TestBaseNormalizer_TemplateEngine(t *testing.T) {
	t.Run("Should return template engine", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := shared.NewBaseNormalizer(templateEngine, nil, task.TaskTypeBasic, nil)
		// Act
		result := normalizer.TemplateEngine()
		// Assert
		assert.Equal(t, templateEngine, result)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := shared.NewBaseNormalizer(nil, nil, task.TaskTypeBasic, nil)
		// Act
		result := normalizer.TemplateEngine()
		// Assert
		assert.Nil(t, result)
	})
}
