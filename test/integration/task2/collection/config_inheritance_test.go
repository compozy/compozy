package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
)

// TestCollectionConfigInheritance validates that collection tasks properly inherit
// CWD and FilePath to their task templates during normalization
func TestCollectionConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath to collection task template", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create collection task config with CWD and FilePath
		collectionCWD := &core.PathCWD{Path: "/collection/working/directory"}
		collectionFilePath := "configs/collection.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-collection",
				Type:     task.TaskTypeCollection,
				CWD:      collectionCWD,
				FilePath: collectionFilePath,
			},
			CollectionConfig: task.CollectionConfig{
				Items:   "{{ .test_items }}",
				ItemVar: "current_item",
				Mode:    task.CollectionModeParallel,
			},
			// Task template without explicit CWD/FilePath - should inherit
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "collection-template",
					Type: task.TaskTypeBasic,
					With: &core.Input{
						"item": "{{ .current_item }}",
					},
				},
				BasicTask: task.BasicTask{
					Action: "process_item",
				},
			},
		}

		// Create normalizer to test inheritance
		normalizer := collection.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_items": []string{"item1", "item2", "item3"},
			},
		}

		// Normalize the collection task
		err := normalizer.Normalize(taskConfig, normCtx)
		require.NoError(t, err, "Collection normalization should succeed")

		// Verify task template inherited CWD and FilePath
		require.NotNil(t, taskConfig.Task, "Collection should have task template")
		require.NotNil(t, taskConfig.Task.CWD, "Task template should inherit CWD")
		assert.Equal(t, "/collection/working/directory", taskConfig.Task.CWD.Path,
			"Task template should inherit collection CWD")
		assert.Equal(t, "configs/collection.yaml", taskConfig.Task.FilePath,
			"Task template should inherit collection FilePath")
	})

	t.Run("Should preserve explicit CWD and FilePath in task template", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create collection task with explicit template CWD/FilePath
		collectionCWD := &core.PathCWD{Path: "/collection/parent/path"}
		collectionFilePath := "parent.yaml"
		templateCWD := &core.PathCWD{Path: "/template/explicit/path"}
		templateFilePath := "template.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-collection-explicit",
				Type:     task.TaskTypeCollection,
				CWD:      collectionCWD,
				FilePath: collectionFilePath,
			},
			CollectionConfig: task.CollectionConfig{
				Items:   "{{ .test_items }}",
				ItemVar: "item",
				Mode:    task.CollectionModeSequential,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:       "explicit-template",
					Type:     task.TaskTypeBasic,
					CWD:      templateCWD,      // Explicit CWD
					FilePath: templateFilePath, // Explicit FilePath
				},
				BasicTask: task.BasicTask{
					Action: "process_with_explicit_context",
				},
			},
		}

		// Create normalizer
		normalizer := collection.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"test_items": []string{"a", "b"},
			},
		}
		err := normalizer.Normalize(taskConfig, normCtx)
		require.NoError(t, err)

		// Verify template preserved its explicit values
		require.NotNil(t, taskConfig.Task.CWD)
		assert.Equal(t, "/template/explicit/path", taskConfig.Task.CWD.Path,
			"Task template should preserve explicit CWD")
		assert.Equal(t, "template.yaml", taskConfig.Task.FilePath,
			"Task template should preserve explicit FilePath")
	})

	t.Run("Should handle collection with custom variable names and inheritance", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create collection with custom variable names
		collectionCWD := &core.PathCWD{Path: "/custom/collection/dir"}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-collection-custom",
				Type: task.TaskTypeCollection,
				CWD:  collectionCWD,
			},
			CollectionConfig: task.CollectionConfig{
				Items:   "{{ .dynamic_items }}",
				ItemVar: "my_item",
				Mode:    task.CollectionModeParallel,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "custom-var-template",
					Type: task.TaskTypeBasic,
					With: &core.Input{
						"data": "{{ .my_item }}",
					},
				},
				BasicTask: task.BasicTask{
					Action: "process_custom",
				},
			},
		}

		// Create normalizer
		normalizer := collection.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize with dynamic items
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"dynamic_items": []string{"x", "y", "z"},
			},
		}
		err := normalizer.Normalize(taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance and custom variable
		require.NotNil(t, taskConfig.Task.CWD)
		assert.Equal(t, "/custom/collection/dir", taskConfig.Task.CWD.Path,
			"Task template should inherit CWD with custom variables")
		assert.Equal(t, "my_item", taskConfig.ItemVar,
			"Custom item variable name should be preserved")
	})

	t.Run("Should handle nested collection inheritance", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create nested collection scenario
		rootCWD := &core.PathCWD{Path: "/root/collection"}
		rootFilePath := "root.yaml"

		// Outer collection
		outerConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "outer-collection",
				Type:     task.TaskTypeCollection,
				CWD:      rootCWD,
				FilePath: rootFilePath,
			},
			CollectionConfig: task.CollectionConfig{
				Items:   "{{ .test_groups }}",
				ItemVar: "group",
				Mode:    task.CollectionModeSequential,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "inner-collection",
					Type: task.TaskTypeCollection,
					// Should inherit from outer
				},
				CollectionConfig: task.CollectionConfig{
					Items:   "{{ .group }}",
					ItemVar: "element",
					Mode:    task.CollectionModeParallel,
				},
				Task: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "process-element",
						Type: task.TaskTypeBasic,
						// Should inherit from inner, which inherited from outer
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
		}

		// Create normalizer
		normalizer := collection.NewNormalizer(
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize outer collection
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{},
		}
		err := normalizer.Normalize(outerConfig, normCtx)
		require.NoError(t, err)

		// Verify outer template (inner collection) inherited
		innerCollection := outerConfig.Task
		require.NotNil(t, innerCollection)
		require.NotNil(t, innerCollection.CWD)
		assert.Equal(t, "/root/collection", innerCollection.CWD.Path,
			"Inner collection should inherit from outer")
		assert.Equal(t, "root.yaml", innerCollection.FilePath,
			"Inner collection should inherit FilePath")

		// The inner collection's template needs separate normalization
		// in real execution, but we verify the structure is correct
		require.NotNil(t, innerCollection.Task)
		assert.Equal(t, "process-element", innerCollection.Task.ID)
	})
}
