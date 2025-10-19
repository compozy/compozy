package router_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/router"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestRouterNormalizer_InlineTaskInheritance(t *testing.T) {
	t.Run("Should inherit CWD to inline task configurations in routes", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := router.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		routerTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router",
				Type:     task.TaskTypeRouter,
				CWD:      &core.PathCWD{Path: "/router/base"},
				FilePath: "router.yaml",
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"condition1": "existing-task", // Simple reference
					"condition2": map[string]any{ // Inline task config
						"id":     "inline-task",
						"type":   "basic",
						"action": "process_data",
						// No CWD or FilePath - should inherit from router
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), routerTask, ctx)

		// Assert
		require.NoError(t, err)

		// Check that simple reference is unchanged
		assert.Equal(t, "existing-task", routerTask.Routes["condition1"])

		// Check that inline task has inherited CWD and FilePath
		inlineTaskMap, ok := routerTask.Routes["condition2"].(map[string]any)
		require.True(t, ok, "Route should be a map")

		// Extract CWD from the inline task
		if cwdMap, hasCWD := inlineTaskMap["CWD"].(map[string]any); hasCWD {
			assert.Equal(t, "/router/base", cwdMap["path"], "Inline task should inherit router CWD")
		} else {
			t.Error("Inline task should have inherited CWD")
		}

		// Check FilePath inheritance
		assert.Equal(t, "router.yaml", inlineTaskMap["file_path"], "Inline task should inherit router FilePath")
	})

	t.Run("Should not override existing CWD in inline task", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := router.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		routerTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router",
				Type:     task.TaskTypeRouter,
				CWD:      &core.PathCWD{Path: "/router/base"},
				FilePath: "router.yaml",
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"condition1": map[string]any{
						"id":     "inline-task",
						"type":   "basic",
						"action": "process_data",
						"CWD": map[string]any{
							"path": "/inline/custom",
						},
						"file_path": "inline.yaml",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), routerTask, ctx)

		// Assert
		require.NoError(t, err)

		// Check that inline task kept its own CWD and FilePath
		inlineTaskMap, ok := routerTask.Routes["condition1"].(map[string]any)
		require.True(t, ok, "Route should be a map")

		// Extract CWD from the inline task
		if cwdMap, hasCWD := inlineTaskMap["CWD"].(map[string]any); hasCWD {
			assert.Equal(t, "/inline/custom", cwdMap["path"], "Inline task should keep its own CWD")
		}

		assert.Equal(t, "inline.yaml", inlineTaskMap["file_path"], "Inline task should keep its own FilePath")
	})

	t.Run("Should handle mixed route types", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := router.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		routerTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router",
				Type:     task.TaskTypeRouter,
				CWD:      &core.PathCWD{Path: "/router/base"},
				FilePath: "router.yaml",
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"string-route": "task-reference",
					"inline-route": map[string]any{
						"id":     "inline-basic",
						"type":   "basic",
						"action": "process",
					},
					"condition-route": map[string]any{
						"condition": "{{ gt .value 10 }}",
						"task_id":   "conditional-task",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"value": 15,
			},
		}

		// Act
		err = normalizer.Normalize(t.Context(), routerTask, ctx)

		// Assert
		require.NoError(t, err)

		// String route should remain unchanged
		assert.Equal(t, "task-reference", routerTask.Routes["string-route"])

		// Inline route should have inherited CWD
		inlineRoute, ok := routerTask.Routes["inline-route"].(map[string]any)
		require.True(t, ok)
		if cwdMap, hasCWD := inlineRoute["CWD"].(map[string]any); hasCWD {
			assert.Equal(t, "/router/base", cwdMap["path"])
		}

		// Condition route should have processed template but no inheritance
		conditionRoute, ok := routerTask.Routes["condition-route"].(map[string]any)
		require.True(t, ok)
		// Template processed - compare as string since template engine may return string
		conditionStr, _ := conditionRoute["condition"].(string)
		assert.Equal(t, "true", conditionStr)
		assert.Equal(t, "conditional-task", conditionRoute["task_id"])
		_, hasCWD := conditionRoute["CWD"]
		assert.False(t, hasCWD, "Condition route should not have CWD")
	})
}

func TestRouterNormalizer_InlineTaskValidation(t *testing.T) {
	t.Run("Should return error for malformed inline task config with type field", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := router.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		routerTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router",
				Type:     task.TaskTypeRouter,
				CWD:      &core.PathCWD{Path: "/router/base"},
				FilePath: "router.yaml",
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"invalid-route": map[string]any{
						"type": "basic",
						// Malformed data that will cause FromMap to fail
						"CWD": "invalid-string-instead-of-object", // CWD should be an object
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), routerTask, ctx)

		// Assert
		require.Error(t, err, "Should return error for invalid inline task config")
		assert.Contains(t, err.Error(), "invalid inline task config for route")
		assert.Contains(t, err.Error(), "invalid-route")
	})

	t.Run("Should process regular map without type field correctly", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := router.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		routerTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-router",
				Type: task.TaskTypeRouter,
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"regular-route": map[string]any{
						// No type field - should be processed as regular map
						"condition": "{{ .value }}",
						"task_id":   "some-task",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"value": "test-value",
			},
		}

		// Act
		err = normalizer.Normalize(t.Context(), routerTask, ctx)

		// Assert
		require.NoError(t, err, "Should process regular map without error")

		// Verify the route was processed correctly
		routeMap, ok := routerTask.Routes["regular-route"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-value", routeMap["condition"])
		assert.Equal(t, "some-task", routeMap["task_id"])
	})
}
