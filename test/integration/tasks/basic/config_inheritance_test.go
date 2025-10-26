package basic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/basic"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
)

// TestBasicConfigInheritance validates that basic tasks properly inherit
// CWD and FilePath when used as child tasks in parent task configurations
func TestBasicConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath as child task", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create basic task config without explicit CWD/FilePath
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-basic-child",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"input_data": "{{ .parent_output }}",
					"format":     "json",
				},
				// No CWD/FilePath - will be inherited by parent normalizer
			},
			BasicTask: task.BasicTask{
				Action: "process_data",
			},
		}

		// Simulate inheritance by parent normalizer
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/basic/directory"},
				FilePath: "configs/parent_basic.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer to test normalization with inherited context
		normalizer := basic.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"parent_output": map[string]any{
					"result": "success",
					"data":   []int{1, 2, 3},
				},
			},
		}

		// Normalize the basic task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Basic normalization should succeed")

		// Verify basic task inherited context
		require.NotNil(t, taskConfig.CWD, "Basic task should have inherited CWD")
		assert.Equal(t, "/parent/basic/directory", taskConfig.CWD.Path,
			"Basic task should inherit parent CWD")
		assert.Equal(t, "configs/parent_basic.yaml", taskConfig.FilePath,
			"Basic task should inherit parent FilePath")

		// Verify template processing worked correctly
		actualData := taskConfig.With.Prop("input_data").(map[string]any)
		assert.Equal(t, "success", actualData["result"],
			"Basic input_data result should be templated correctly")
		assert.Equal(t, []int{1, 2, 3}, actualData["data"],
			"Basic input_data array should be templated correctly")
		assert.Equal(t, "json", taskConfig.With.Prop("format"),
			"Basic format should be preserved")

		// Verify action is preserved
		assert.Equal(t, "process_data", taskConfig.Action,
			"Basic action should be preserved")
	})

	t.Run("Should preserve explicit CWD and FilePath", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create basic task with explicit CWD/FilePath
		explicitCWD := &core.PathCWD{Path: "/explicit/basic/path"}
		explicitFilePath := "explicit_basic.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-basic-explicit",
				Type:     task.TaskTypeBasic,
				CWD:      explicitCWD,      // Explicit CWD
				FilePath: explicitFilePath, // Explicit FilePath
				With: &core.Input{
					"input_data": "{{ .user_input }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "validate_input",
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
		normalizer := basic.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"user_input": "valid_data",
			},
		}

		// Normalize the basic task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Basic normalization should succeed")

		// Verify explicit values are preserved
		require.NotNil(t, taskConfig.CWD, "Basic task should have CWD")
		assert.Equal(t, "/explicit/basic/path", taskConfig.CWD.Path,
			"Basic task should preserve explicit CWD")
		assert.Equal(t, "explicit_basic.yaml", taskConfig.FilePath,
			"Basic task should preserve explicit FilePath")

		// Verify template processing still works
		assert.Equal(t, "valid_data", taskConfig.With.Prop("input_data"),
			"Basic input_data should be templated correctly")

		// Verify action is preserved
		assert.Equal(t, "validate_input", taskConfig.Action,
			"Basic action should be preserved")
	})

	t.Run("Should handle three-level inheritance chain", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create grandchild basic task (level 3)
		grandchildConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-basic-grandchild",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"nested_input": "{{ .deep_data }}",
				},
				// No CWD/FilePath - will be inherited
			},
			BasicTask: task.BasicTask{
				Action: "process_nested",
			},
		}

		// Create child config (level 2) - also no explicit CWD/FilePath
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-basic-child",
				Type: task.TaskTypeComposite,
				// No CWD/FilePath - will be inherited
			},
		}

		// Create parent config (level 1) - root with explicit values
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-basic-parent",
				Type:     task.TaskTypeParallel,
				CWD:      &core.PathCWD{Path: "/root/basic/workspace"},
				FilePath: "configs/root_basic.yaml",
			},
		}

		// Apply three-level inheritance chain
		shared.InheritTaskConfig(childConfig, parentConfig)
		shared.InheritTaskConfig(grandchildConfig, childConfig)

		// Create normalizer
		normalizer := basic.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"deep_data": map[string]any{
					"level": 3,
					"type":  "nested_processing",
				},
			},
		}

		// Normalize the grandchild basic task
		err := normalizer.Normalize(t.Context(), grandchildConfig, normCtx)
		require.NoError(t, err, "Basic normalization should succeed")

		// Verify inheritance propagated through the chain
		require.NotNil(t, grandchildConfig.CWD, "Grandchild basic should have inherited CWD")
		assert.Equal(t, "/root/basic/workspace", grandchildConfig.CWD.Path,
			"Grandchild basic should inherit root CWD through chain")
		assert.Equal(t, "configs/root_basic.yaml", grandchildConfig.FilePath,
			"Grandchild basic should inherit root FilePath through chain")

		// Verify template processing worked
		expectedNestedData := map[string]any{
			"level": 3,
			"type":  "nested_processing",
		}
		assert.Equal(t, expectedNestedData, grandchildConfig.With.Prop("nested_input"),
			"Basic nested_input should be templated correctly")

		// Verify action is preserved
		assert.Equal(t, "process_nested", grandchildConfig.Action,
			"Basic action should be preserved")
	})

	t.Run("Should handle complex templating with inheritance", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create basic task with complex templating
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-basic-complex",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"file_path":   "{{ .base_path }}/{{ .file_name }}",
					"config_data": "{{ .config }}",
					"array_data":  []any{"{{ .item1 }}", "{{ .item2 }}", "static_item"},
				},
				// No CWD/FilePath - will be inherited
			},
			BasicTask: task.BasicTask{
				Action: "{{ .action_type }}",
			},
		}

		// Simulate inheritance by parent normalizer
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/complex/directory"},
				FilePath: "configs/parent_complex.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := basic.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
		)

		// Setup normalization context with complex data
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"base_path":   "/data/files",
				"file_name":   "input.json",
				"action_type": "process_file",
				"config": map[string]any{
					"timeout": 30,
					"retry":   true,
				},
				"item1": "dynamic_value_1",
				"item2": "dynamic_value_2",
			},
		}

		// Normalize the basic task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Basic normalization should succeed")

		// Verify inheritance
		require.NotNil(t, taskConfig.CWD, "Basic task should have inherited CWD")
		assert.Equal(t, "/parent/complex/directory", taskConfig.CWD.Path,
			"Basic task should inherit parent CWD")
		assert.Equal(t, "configs/parent_complex.yaml", taskConfig.FilePath,
			"Basic task should inherit parent FilePath")

		// Verify complex template processing
		assert.Equal(t, "/data/files/input.json", taskConfig.With.Prop("file_path"),
			"Basic file_path should be templated correctly")

		expectedConfig := map[string]any{
			"timeout": 30,
			"retry":   true,
		}
		assert.Equal(t, expectedConfig, taskConfig.With.Prop("config_data"),
			"Basic config_data should be templated correctly")

		expectedArray := []any{"dynamic_value_1", "dynamic_value_2", "static_item"}
		assert.Equal(t, expectedArray, taskConfig.With.Prop("array_data"),
			"Basic array_data should be templated correctly")

		// Verify action templating
		assert.Equal(t, "process_file", taskConfig.Action,
			"Basic action should be templated correctly")
	})
}
