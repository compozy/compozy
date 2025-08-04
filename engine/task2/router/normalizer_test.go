package router_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/router"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestRouterNormalizer_NewNormalizer(t *testing.T) {
	t.Run("Should create router normalizer", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Act
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Act
		normalizer := router.NewNormalizer(nil, contextBuilder)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle nil context builder", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)

		// Act
		normalizer := router.NewNormalizer(templateEngine, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should handle both nil parameters", func(t *testing.T) {
		// Act
		normalizer := router.NewNormalizer(nil, nil)

		// Assert
		assert.NotNil(t, normalizer)
	})
}

func TestRouterNormalizer_Type(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeRouter, taskType)
	})
}

func TestRouterNormalizer_Normalize(t *testing.T) {
	// Setup
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	normalizer := router.NewNormalizer(templateEngine, contextBuilder)

	t.Run("Should normalize router task with simple string routes", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"success": "success-task",
					"error":   "error-task",
					"default": "default-task",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_var": "test_value",
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "router-task", taskConfig.ID)
		assert.Equal(t, task.TaskTypeRouter, taskConfig.Type)
		require.NotNil(t, taskConfig.Routes)
		assert.Equal(t, "success-task", taskConfig.Routes["success"])
		assert.Equal(t, "error-task", taskConfig.Routes["error"])
		assert.Equal(t, "default-task", taskConfig.Routes["default"])
	})

	t.Run("Should normalize router task with template expressions in routes", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-{{ .route_type }}",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"success": "{{ .success_task }}",
					"error":   "{{ .error_task }}",
					"default": "static-default",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"route_type":   "conditional",
				"success_task": "handle-success",
				"error_task":   "handle-error",
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "router-conditional", taskConfig.ID)
		require.NotNil(t, taskConfig.Routes)
		assert.Equal(t, "handle-success", taskConfig.Routes["success"])
		assert.Equal(t, "handle-error", taskConfig.Routes["error"])
		assert.Equal(t, "static-default", taskConfig.Routes["default"])
	})

	t.Run("Should normalize router task with complex route objects", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"conditional": map[string]any{
						"condition": "{{ .condition }}",
						"task_id":   "{{ .task_id }}",
					},
					"simple": "simple-task",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"condition": "status == 'success'",
				"task_id":   "conditional-task",
			},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, taskConfig.Routes)
		assert.Equal(t, "simple-task", taskConfig.Routes["simple"])

		conditionalRoute, ok := taskConfig.Routes["conditional"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "status == 'success'", conditionalRoute["condition"])
		assert.Equal(t, "conditional-task", conditionalRoute["task_id"])
	})

	t.Run("Should handle router task with nil routes", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: nil,
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "router-task", taskConfig.ID)
		assert.Nil(t, taskConfig.Routes)
	})

	t.Run("Should handle router task with empty routes", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "router-task", taskConfig.ID)
		require.NotNil(t, taskConfig.Routes)
		assert.Empty(t, taskConfig.Routes)
	})

	t.Run("Should handle non-string, non-map route values", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"numeric": 123,
					"boolean": true,
					"array":   []string{"item1", "item2"},
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, taskConfig.Routes)
		assert.Equal(t, float64(123), taskConfig.Routes["numeric"]) // JSON unmarshaling converts to float64
		assert.Equal(t, true, taskConfig.Routes["boolean"])
		assert.Equal(
			t,
			[]any{"item1", "item2"},
			taskConfig.Routes["array"],
		) // JSON unmarshaling converts to []interface{}
	})

	t.Run("Should return error for invalid template in route string", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"invalid": "{{ .nonexistent.field }}",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.ErrorContains(t, err, "failed to normalize router task config")
	})

	t.Run("Should return error for invalid template in route map", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"complex": map[string]any{
						"task_id": "{{ .invalid.template }}",
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}

		// Act
		err := normalizer.Normalize(taskConfig, ctx)

		// Assert
		assert.ErrorContains(t, err, "failed to normalize router task config")
	})
}

func TestRouterNormalizer_Normalize_ErrorHandling(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	normalizer := router.NewNormalizer(templateEngine, contextBuilder)

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}
		// Act & Assert - Router normalizer should panic on nil config like the implementation shows
		assert.Panics(t, func() {
			normalizer.Normalize(nil, ctx)
		})
	})

	t.Run("Should return error for wrong task type", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.ErrorContains(t, err, "router normalizer cannot handle task type: basic")
	})

	t.Run("Should handle template parsing errors in main config", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .invalid.deeply.nested.nonexistent.field }}",
				Type: task.TaskTypeRouter,
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
		assert.ErrorContains(t, err, "failed to normalize router task config")
	})

	t.Run("Should handle config serialization errors", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeRouter,
			},
		}
		// Inject problematic data for serialization
		unsafeField := func() {}
		taskConfig.With = &core.Input{"function": unsafeField}

		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.ErrorContains(t, err, "failed to convert task config to map")
	})

	t.Run("Should handle deeply nested template errors in routes", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "router-task",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"nested": map[string]any{
						"level1": map[string]any{
							"level2": "{{ .deeply.nested.nonexistent.field }}",
						},
					},
				},
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
		assert.ErrorContains(t, err, "failed to normalize router task config")
	})

	t.Run("Should process router with conditional expressions successfully", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "{{ .name }}-router",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"condition1": "{{ if .success }}success-task{{ else }}fallback-task{{ end }}",
					"condition2": map[string]any{
						"when":    "{{ .condition }}",
						"task_id": "{{ .target_task }}",
					},
				},
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"name":        "test",
				"success":     true,
				"condition":   "status == 'completed'",
				"target_task": "completion-handler",
			},
		}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "test-router", taskConfig.ID)
		assert.Equal(t, "success-task", taskConfig.Routes["condition1"])
		conditionRoute := taskConfig.Routes["condition2"].(map[string]any)
		assert.Equal(t, "status == 'completed'", conditionRoute["when"])
		assert.Equal(t, "completion-handler", conditionRoute["task_id"])
	})
}

func TestRouterNormalizer_BoundaryConditions(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should handle nil template engine", func(t *testing.T) {
		// Arrange
		normalizer := router.NewNormalizer(nil, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeRouter,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert - Should return error due to nil template engine
		assert.ErrorContains(t, err, "template engine is required for normalization")
	})

	t.Run("Should handle nil context gracefully", func(t *testing.T) {
		// Arrange
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeRouter,
			},
		}
		// Act
		err = normalizer.Normalize(taskConfig, nil)

		// Assert
		assert.ErrorContains(t, err, "invalid context type")
	})

	t.Run("Should handle very large route maps", func(t *testing.T) {
		// Arrange
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)
		routes := make(map[string]any)
		for i := 0; i < 1000; i++ {
			routes[fmt.Sprintf("route_%d", i)] = fmt.Sprintf("task_%d", i)
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "large-router",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: routes,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Len(t, taskConfig.Routes, 1000)
	})

	t.Run("Should handle routes with special characters in keys", func(t *testing.T) {
		// Arrange
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "special-router",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"route.with.dots":    "dot-task",
					"route-with-dashes":  "dash-task",
					"route_with_under":   "under-task",
					"route with spaces":  "space-task",
					"route/with/slashes": "slash-task",
					"route@with@symbols": "symbol-task",
				},
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "dot-task", taskConfig.Routes["route.with.dots"])
		assert.Equal(t, "dash-task", taskConfig.Routes["route-with-dashes"])
		assert.Equal(t, "under-task", taskConfig.Routes["route_with_under"])
		assert.Equal(t, "space-task", taskConfig.Routes["route with spaces"])
		assert.Equal(t, "slash-task", taskConfig.Routes["route/with/slashes"])
		assert.Equal(t, "symbol-task", taskConfig.Routes["route@with@symbols"])
	})

	t.Run("Should preserve route structure and types", func(t *testing.T) {
		// Arrange
		normalizer := router.NewNormalizer(templateEngine, contextBuilder)
		originalRoutes := map[string]any{
			"string_route": "string-task",
			"complex_route": map[string]any{
				"condition": "status == 'ready'",
				"task_id":   "complex-task",
				"metadata": map[string]any{
					"priority": 1,
					"timeout":  30,
				},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "preserve-router",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: originalRoutes,
			},
		}
		ctx := &shared.NormalizationContext{Variables: make(map[string]any)}
		// Act
		err := normalizer.Normalize(taskConfig, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "string-task", taskConfig.Routes["string_route"])
		complexRoute := taskConfig.Routes["complex_route"].(map[string]any)
		assert.Equal(t, "status == 'ready'", complexRoute["condition"])
		assert.Equal(t, "complex-task", complexRoute["task_id"])
		metadata := complexRoute["metadata"].(map[string]any)
		assert.Equal(t, float64(1), metadata["priority"])
		assert.Equal(t, float64(30), metadata["timeout"])
	})
}
