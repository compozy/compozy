package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/router"
	"github.com/compozy/compozy/engine/task2/shared"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
)

// TestRouterConfigInheritance validates that router tasks properly inherit
// CWD and FilePath when used as child tasks in parent task configurations
func TestRouterConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath as child task", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create router task config without explicit CWD/FilePath
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-router-child",
				Type: task.TaskTypeRouter,
				With: &core.Input{
					"routing_key": "{{ .route_selector }}",
					"default":     "fallback_route",
				},
				// No CWD/FilePath - will be inherited by parent normalizer
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"route_a": map[string]any{
						"id":     "route-a-task",
						"type":   string(task.TaskTypeBasic),
						"action": "process_route_a",
					},
					"route_b": map[string]any{
						"id":     "route-b-task",
						"type":   string(task.TaskTypeBasic),
						"action": "process_route_b",
					},
				},
			},
		}

		// Simulate inheritance by parent normalizer
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/router/directory"},
				FilePath: "configs/parent_routing.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer to test normalization with inherited context
		normalizer := router.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"route_selector": "route_a",
			},
		}

		// Normalize the router task
		err := normalizer.Normalize(taskConfig, normCtx)
		require.NoError(t, err, "Router normalization should succeed")

		// Verify router task inherited context
		require.NotNil(t, taskConfig.CWD, "Router task should have inherited CWD")
		assert.Equal(t, "/parent/router/directory", taskConfig.CWD.Path,
			"Router task should inherit parent CWD")
		assert.Equal(t, "configs/parent_routing.yaml", taskConfig.FilePath,
			"Router task should inherit parent FilePath")

		// Verify template processing worked correctly
		assert.Equal(t, "route_a", taskConfig.With.Prop("routing_key"),
			"Router routing_key should be templated correctly")
		assert.Equal(t, "fallback_route", taskConfig.With.Prop("default"),
			"Router default should be preserved")

		// Verify routes are properly configured
		require.NotNil(t, taskConfig.Routes, "Router should have routes configured")
		assert.Len(t, taskConfig.Routes, 2, "Router should have 2 routes")

		// Verify route inheritance - child tasks should also inherit context
		routeA, exists := taskConfig.Routes["route_a"]
		require.True(t, exists, "Route A should exist")
		routeAMap, ok := routeA.(map[string]any)
		require.True(t, ok, "Route A should be a map")
		assert.Equal(t, "/parent/router/directory", routeAMap["CWD"].(map[string]any)["path"],
			"Route A should inherit parent CWD")

		routeB, exists := taskConfig.Routes["route_b"]
		require.True(t, exists, "Route B should exist")
		routeBMap, ok := routeB.(map[string]any)
		require.True(t, ok, "Route B should be a map")
		assert.Equal(t, "/parent/router/directory", routeBMap["CWD"].(map[string]any)["path"],
			"Route B should inherit parent CWD")
	})

	t.Run("Should preserve explicit CWD and FilePath", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create router task with explicit CWD/FilePath
		explicitCWD := &core.PathCWD{Path: "/explicit/router/path"}
		explicitFilePath := "explicit_router.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router-explicit",
				Type:     task.TaskTypeRouter,
				CWD:      explicitCWD,      // Explicit CWD
				FilePath: explicitFilePath, // Explicit FilePath
				With: &core.Input{
					"routing_key": "{{ .route_selector }}",
				},
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"default": map[string]any{
						"id":     "default-route-task",
						"type":   string(task.TaskTypeBasic),
						"action": "handle_default",
					},
				},
			},
		}

		// Try to inherit from parent (should not override explicit values)
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/different/directory"},
				FilePath: "configs/parent_different.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := router.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"route_selector": "default",
			},
		}

		// Normalize the router task
		err := normalizer.Normalize(taskConfig, normCtx)
		require.NoError(t, err, "Router normalization should succeed")

		// Verify explicit values are preserved
		require.NotNil(t, taskConfig.CWD, "Router task should have CWD")
		assert.Equal(t, "/explicit/router/path", taskConfig.CWD.Path,
			"Router task should preserve explicit CWD")
		assert.Equal(t, "explicit_router.yaml", taskConfig.FilePath,
			"Router task should preserve explicit FilePath")

		// Verify template processing still works
		assert.Equal(t, "default", taskConfig.With.Prop("routing_key"),
			"Router routing_key should be templated correctly")
	})

	t.Run("Should handle three-level inheritance chain", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create grandchild router task (level 3)
		grandchildConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-router-grandchild",
				Type: task.TaskTypeRouter,
				With: &core.Input{
					"routing_key": "{{ .nested_route }}",
				},
				// No CWD/FilePath - will be inherited
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"nested": map[string]any{
						"id":     "nested-route-task",
						"type":   string(task.TaskTypeBasic),
						"action": "handle_nested",
					},
				},
			},
		}

		// Create child config (level 2) - also no explicit CWD/FilePath
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-router-child",
				Type: task.TaskTypeComposite,
				// No CWD/FilePath - will be inherited
			},
		}

		// Create parent config (level 1) - root with explicit values
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-router-parent",
				Type:     task.TaskTypeParallel,
				CWD:      &core.PathCWD{Path: "/root/router/workspace"},
				FilePath: "configs/root_router.yaml",
			},
		}

		// Apply three-level inheritance chain
		shared.InheritTaskConfig(childConfig, parentConfig)
		shared.InheritTaskConfig(grandchildConfig, childConfig)

		// Create normalizer
		normalizer := router.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"nested_route": "nested",
			},
		}

		// Normalize the grandchild router task
		err := normalizer.Normalize(grandchildConfig, normCtx)
		require.NoError(t, err, "Router normalization should succeed")

		// Verify inheritance propagated through the chain
		require.NotNil(t, grandchildConfig.CWD, "Grandchild router should have inherited CWD")
		assert.Equal(t, "/root/router/workspace", grandchildConfig.CWD.Path,
			"Grandchild router should inherit root CWD through chain")
		assert.Equal(t, "configs/root_router.yaml", grandchildConfig.FilePath,
			"Grandchild router should inherit root FilePath through chain")

		// Verify template processing worked
		assert.Equal(t, "nested", grandchildConfig.With.Prop("routing_key"),
			"Router routing_key should be templated correctly")

		// Verify nested route also inherited context
		nestedRoute, exists := grandchildConfig.Routes["nested"]
		require.True(t, exists, "Nested route should exist")
		nestedRouteMap, ok := nestedRoute.(map[string]any)
		require.True(t, ok, "Nested route should be a map")
		assert.Equal(t, "/root/router/workspace", nestedRouteMap["CWD"].(map[string]any)["path"],
			"Nested route should inherit root CWD through chain")
	})
}
