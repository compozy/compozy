package task2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	task2 "github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/composite"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/parallel"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/test/integration/task2/helpers"
)

// TestContextInheritanceIntegration validates context inheritance in real workflow scenarios
func TestContextInheritanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	// Create test setup with all required infrastructure
	setup := helpers.NewTestSetup(t)

	t.Run("Should inherit context in parallel task normalization", func(t *testing.T) {
		t.Parallel()

		// Create parallel task configuration with inheritance scenario
		parentCWD := &core.PathCWD{Path: "/parallel/execution/path"}
		parentFilePath := "configs/parallel.yaml"

		parallelConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "integration-parallel",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "child-basic-1",
						Type: task.TaskTypeBasic,
						// No CWD or FilePath - should inherit from parent
					},
					BasicTask: task.BasicTask{
						Action: "integration_test_action",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "child-basic-2",
						Type: task.TaskTypeBasic,
						// No CWD or FilePath - should inherit from parent
					},
					BasicTask: task.BasicTask{
						Action: "another_integration_action",
					},
				},
			},
		}

		// Create real factory for true integration testing
		factory, err := task2.NewFactory(t.Context(), &task2.FactoryConfig{
			TemplateEngine: setup.TemplateEngine,
			EnvMerger:      task2core.NewEnvMerger(),
		})
		require.NoError(t, err, "Factory creation should succeed")

		parallelNormalizer := parallel.NewNormalizer(
			t.Context(),
			setup.TemplateEngine,
			setup.ContextBuilder,
			factory,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Execute parallel task normalization
		err = parallelNormalizer.Normalize(t.Context(), parallelConfig, normCtx)
		require.NoError(t, err, "Parallel normalization should succeed")

		// Verify context inheritance for all child tasks
		for i, childTask := range parallelConfig.Tasks {
			require.NotNil(t, childTask.CWD, "Child task %d should inherit parent CWD", i+1)
			assert.Equal(t, "/parallel/execution/path", childTask.CWD.Path,
				"Child task %d should inherit correct CWD path", i+1)
			assert.Equal(t, "configs/parallel.yaml", childTask.FilePath,
				"Child task %d should inherit correct FilePath", i+1)

			// Verify task-specific properties are preserved
			assert.Equal(t, task.TaskTypeBasic, childTask.Type,
				"Child task %d should preserve its type", i+1)
			if i == 0 {
				assert.Equal(t, "integration_test_action", childTask.Action,
					"First child task should preserve its action")
			} else {
				assert.Equal(t, "another_integration_action", childTask.Action,
					"Second child task should preserve its action")
			}
		}
	})

	t.Run("Should inherit context in composite task with nested levels", func(t *testing.T) {
		t.Parallel()

		// Create composite task with three-level nesting
		rootCWD := &core.PathCWD{Path: "/composite/base/directory"}
		rootFilePath := "configs/root_composite.yaml"

		compositeConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "integration-composite",
				Type:     task.TaskTypeComposite,
				CWD:      rootCWD,
				FilePath: rootFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "level1-parallel",
						Type: task.TaskTypeParallel,
						// Should inherit CWD from root composite
						FilePath: "configs/level1.yaml", // Explicit FilePath override
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "level2-basic",
								Type: task.TaskTypeBasic,
								// Should inherit CWD from root, FilePath from level1
							},
							BasicTask: task.BasicTask{
								Action: "nested_integration_action",
							},
						},
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "level1-basic",
						Type: task.TaskTypeBasic,
						// Should inherit directly from root composite
					},
					BasicTask: task.BasicTask{
						Action: "direct_child_action",
					},
				},
			},
		}

		// Create factory with real normalizers for proper inheritance
		factory, err := task2.NewFactory(t.Context(), &task2.FactoryConfig{
			TemplateEngine: setup.TemplateEngine,
			EnvMerger:      task2core.NewEnvMerger(),
		})
		require.NoError(t, err, "Factory creation should succeed")

		compositeNormalizer := composite.NewNormalizer(
			t.Context(),
			setup.TemplateEngine,
			setup.ContextBuilder,
			factory,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Execute composite normalization
		err = compositeNormalizer.Normalize(t.Context(), compositeConfig, normCtx)
		require.NoError(t, err, "Composite normalization should succeed")

		// Verify first-level inheritance
		level1Parallel := &compositeConfig.Tasks[0]
		require.NotNil(t, level1Parallel.CWD, "Level1 parallel should inherit CWD")
		assert.Equal(t, "/composite/base/directory", level1Parallel.CWD.Path,
			"Level1 parallel should inherit root CWD")
		assert.Equal(t, "configs/level1.yaml", level1Parallel.FilePath,
			"Level1 parallel should preserve its explicit FilePath")

		// Verify second-level inheritance (nested in parallel)
		level2Basic := &level1Parallel.Tasks[0]
		require.NotNil(t, level2Basic.CWD, "Level2 basic should inherit CWD")
		assert.Equal(t, "/composite/base/directory", level2Basic.CWD.Path,
			"Level2 basic should inherit root CWD through chain")
		assert.Equal(t, "configs/level1.yaml", level2Basic.FilePath,
			"Level2 basic should inherit level1 FilePath")
		assert.Equal(t, "nested_integration_action", level2Basic.Action,
			"Level2 basic should preserve its action")

		// Verify direct child inheritance
		level1Basic := &compositeConfig.Tasks[1]
		require.NotNil(t, level1Basic.CWD, "Level1 basic should inherit CWD")
		assert.Equal(t, "/composite/base/directory", level1Basic.CWD.Path,
			"Level1 basic should inherit root CWD")
		assert.Equal(t, "configs/root_composite.yaml", level1Basic.FilePath,
			"Level1 basic should inherit root FilePath")
		assert.Equal(t, "direct_child_action", level1Basic.Action,
			"Level1 basic should preserve its action")
	})

	t.Run("Should inherit context in collection task template", func(t *testing.T) {
		t.Parallel()

		// Create collection task with template inheritance
		collectionCWD := &core.PathCWD{Path: "/collection/processing/dir"}
		collectionFilePath := "configs/collection.yaml"

		collectionConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "integration-collection",
				Type:     task.TaskTypeCollection,
				CWD:      collectionCWD,
				FilePath: collectionFilePath,
			},
			CollectionConfig: task.CollectionConfig{
				Items:   "{{ .test_items }}",
				ItemVar: "current_item",
				Mode:    task.CollectionModeParallel,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "collection-template",
					Type: task.TaskTypeBasic,
					// Should inherit from collection parent
				},
				BasicTask: task.BasicTask{
					Action: "process_collection_item",
				},
			},
		}

		// Create collection normalizer (doesn't use factory)
		collectionNormalizer := collection.NewNormalizer(
			t.Context(),
			setup.TemplateEngine,
			setup.ContextBuilder,
		)

		// Setup normalization context with test data
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_items": []string{"item1", "item2", "item3"},
			},
		}

		// Execute collection normalization
		err := collectionNormalizer.Normalize(t.Context(), collectionConfig, normCtx)
		require.NoError(t, err, "Collection normalization should succeed")

		// Verify collection template inheritance
		collectionTemplate := collectionConfig.Task
		require.NotNil(t, collectionTemplate, "Collection should have task template")
		require.NotNil(t, collectionTemplate.CWD, "Collection template should inherit CWD")
		assert.Equal(t, "/collection/processing/dir", collectionTemplate.CWD.Path,
			"Collection template should inherit collection CWD")
		assert.Equal(t, "configs/collection.yaml", collectionTemplate.FilePath,
			"Collection template should inherit collection FilePath")
		assert.Equal(t, "process_collection_item", collectionTemplate.Action,
			"Collection template should preserve its action")

		// Verify collection configuration is preserved
		assert.Equal(t, task.CollectionModeParallel, collectionConfig.Mode,
			"Collection mode should be preserved")
		assert.Equal(t, "current_item", collectionConfig.ItemVar,
			"Collection item variable should be preserved")
	})

	t.Run("Should preserve existing context values when child has explicit settings", func(t *testing.T) {
		t.Parallel()

		// Create parallel task where child has explicit CWD and FilePath
		parentCWD := &core.PathCWD{Path: "/parent/directory"}
		parentFilePath := "configs/parent.yaml"
		childCWD := &core.PathCWD{Path: "/child/specific/directory"}
		childFilePath := "configs/child_specific.yaml"

		parallelConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "inheritance-override-test",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:       "explicit-child",
						Type:     task.TaskTypeBasic,
						CWD:      childCWD,      // Explicit CWD - should not be overridden
						FilePath: childFilePath, // Explicit FilePath - should not be overridden
					},
					BasicTask: task.BasicTask{
						Action: "explicit_child_action",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "inherit-child",
						Type: task.TaskTypeBasic,
						// No CWD or FilePath - should inherit from parent
					},
					BasicTask: task.BasicTask{
						Action: "inherit_child_action",
					},
				},
			},
		}

		// Create real factory for true integration testing
		factory, err := task2.NewFactory(t.Context(), &task2.FactoryConfig{
			TemplateEngine: setup.TemplateEngine,
			EnvMerger:      task2core.NewEnvMerger(),
		})
		require.NoError(t, err, "Factory creation should succeed")

		parallelNormalizer := parallel.NewNormalizer(
			t.Context(),
			setup.TemplateEngine,
			setup.ContextBuilder,
			factory,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Execute normalization
		err = parallelNormalizer.Normalize(t.Context(), parallelConfig, normCtx)
		require.NoError(t, err, "Parallel normalization should succeed")

		// Verify explicit child preserves its settings
		explicitChild := &parallelConfig.Tasks[0]
		require.NotNil(t, explicitChild.CWD, "Explicit child should have CWD")
		assert.Equal(t, "/child/specific/directory", explicitChild.CWD.Path,
			"Explicit child should preserve its own CWD")
		assert.Equal(t, "configs/child_specific.yaml", explicitChild.FilePath,
			"Explicit child should preserve its own FilePath")
		assert.Equal(t, "explicit_child_action", explicitChild.Action,
			"Explicit child should preserve its action")

		// Verify inherit child gets parent values
		inheritChild := &parallelConfig.Tasks[1]
		require.NotNil(t, inheritChild.CWD, "Inherit child should inherit CWD")
		assert.Equal(t, "/parent/directory", inheritChild.CWD.Path,
			"Inherit child should inherit parent CWD")
		assert.Equal(t, "configs/parent.yaml", inheritChild.FilePath,
			"Inherit child should inherit parent FilePath")
		assert.Equal(t, "inherit_child_action", inheritChild.Action,
			"Inherit child should preserve its action")
	})
}
