package basic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/basic"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestBasicNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := basic.NewNormalizer(t.Context(), templateEngine)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeBasic, taskType)
	})
}

func TestBasicNormalizer_Integration(t *testing.T) {
	t.Run("Should be based on BaseNormalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		normalizer := basic.NewNormalizer(t.Context(), templateEngine)

		// Assert
		require.NotNil(t, normalizer)

		// Basic normalizer should inherit all BaseNormalizer functionality
		// The BaseNormalizer methods will be tested separately in shared/base_normalizer_test.go
		assert.Equal(t, task.TaskTypeBasic, normalizer.Type())
	})
}

func TestBasicNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	normalizer := basic.NewNormalizer(t.Context(), templateEngine)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(t.Context(), nil, ctx)
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.ErrorContains(t, err, "basic normalizer cannot handle task type: collection")
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
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.ErrorContains(t, err, "failed to normalize basic task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Inject problematic data for serialization
		unsafeField := func() {}
		taskConfig.With = &core.Input{"function": unsafeField}

		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.ErrorContains(t, err, "failed to convert task config to map")
	})

	t.Run("Should process basic task action successfully", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "{{ .action_type }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":        "test",
				"action_type": "run-script",
			},
		}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-task", taskConfig.ID)
		assert.Equal(t, "run-script", taskConfig.Action)
	})

	t.Run("Should handle empty action field", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "empty-action-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "",
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "", taskConfig.Action)
	})
}

func TestBasicNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := basic.NewNormalizer(t.Context(), nil)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert - Should return error instead of panicking
		assert.ErrorContains(t, err, "template engine is required for normalization")
	})

	t.Run("Should handle empty task type for basic tasks", func(t *testing.T) {
		// Arrange
		normalizer := basic.NewNormalizer(t.Context(), templateEngine)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: "", // Empty type should be accepted for basic tasks
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle complex action templates", func(t *testing.T) {
		// Arrange
		normalizer := basic.NewNormalizer(t.Context(), templateEngine)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "complex-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "{{ if .use_docker }}docker run {{ .image }}{{ else }}{{ .command }}{{ end }}",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"use_docker": true,
				"image":      "nginx:latest",
				"command":    "ls -la",
			},
		}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "docker run nginx:latest", taskConfig.Action)
	})

	t.Run("Should preserve basic task configuration", func(t *testing.T) {
		// Arrange
		normalizer := basic.NewNormalizer(t.Context(), templateEngine)
		originalAction := "original-action"
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "preserve-test",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: originalAction,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(t.Context(), taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalAction, taskConfig.Action)
		assert.Equal(t, task.TaskTypeBasic, taskConfig.Type)
	})
}
