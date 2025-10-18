package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// createValidTaskConfig creates a task config that passes validation
func createValidTaskConfig(id string, taskType task.Type) *task.Config {
	cwd, _ := core.CWDFromPath(".")
	return &task.Config{
		BaseConfig: task.BaseConfig{
			ID:   id,
			Type: taskType,
			CWD:  cwd,
		},
	}
}

func TestExpander_ImplementsInterface(t *testing.T) {
	t.Run("Should implement CollectionExpander interface", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := NewNormalizer(templateEngine, contextBuilder)
		configBuilder := NewConfigBuilder(templateEngine)

		expander := NewExpander(normalizer, contextBuilder, configBuilder)

		// Act & Assert - This ensures interface compliance
		var _ shared.CollectionExpander = expander
		assert.NotNil(t, expander)
	})
}

func TestNewExpander(t *testing.T) {
	t.Run("Should create expander with all dependencies", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		normalizer := NewNormalizer(templateEngine, contextBuilder)
		configBuilder := NewConfigBuilder(templateEngine)

		// Act
		expander := NewExpander(normalizer, contextBuilder, configBuilder)

		// Assert
		assert.NotNil(t, expander)
		assert.Equal(t, normalizer, expander.normalizer)
		assert.Equal(t, contextBuilder, expander.contextBuilder)
		assert.Equal(t, configBuilder, expander.configBuilder)
	})
}

func TestExpander_ValidateInputs(t *testing.T) {
	t.Run("Should pass validation for valid inputs", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeCollection,
			},
		}
		workflowState := &workflow.State{}
		workflowConfig := &workflow.Config{}

		// Act
		err := expander.validateInputs(config, workflowState, workflowConfig)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Should fail validation for nil config", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		workflowState := &workflow.State{}
		workflowConfig := &workflow.Config{}

		// Act
		err := expander.validateInputs(nil, workflowState, workflowConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should fail validation for wrong task type", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeBasic,
			},
		}
		workflowState := &workflow.State{}
		workflowConfig := &workflow.Config{}

		// Act
		err := expander.validateInputs(config, workflowState, workflowConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected collection task type")
	})

	t.Run("Should fail validation for nil workflow state", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeCollection,
			},
		}
		workflowConfig := &workflow.Config{}

		// Act
		err := expander.validateInputs(config, nil, workflowConfig)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state cannot be nil")
	})

	t.Run("Should fail validation for nil workflow config", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeCollection,
			},
		}
		workflowState := &workflow.State{}

		// Act
		err := expander.validateInputs(config, workflowState, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow config cannot be nil")
	})
}

func TestExpander_ValidateExpansion(t *testing.T) {
	t.Run("Should pass validation for valid expansion result", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config1 := createValidTaskConfig("task1", task.TaskTypeBasic)
		config2 := createValidTaskConfig("task2", task.TaskTypeBasic)
		result := &shared.ExpansionResult{
			ChildConfigs: []*task.Config{config1, config2},
			ItemCount:    2,
			SkippedCount: 0,
		}

		// Act
		err := expander.ValidateExpansion(t.Context(), result)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Should fail validation for nil result", func(t *testing.T) {
		// Arrange
		expander := &Expander{}

		// Act
		err := expander.ValidateExpansion(t.Context(), nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expansion result cannot be nil")
	})

	t.Run("Should fail validation for negative item count", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		result := &shared.ExpansionResult{
			ChildConfigs: []*task.Config{},
			ItemCount:    -1,
			SkippedCount: 0,
		}

		// Act
		err := expander.ValidateExpansion(t.Context(), result)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "item count cannot be negative")
	})

	t.Run("Should fail validation for negative skipped count", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		result := &shared.ExpansionResult{
			ChildConfigs: []*task.Config{},
			ItemCount:    0,
			SkippedCount: -1,
		}

		// Act
		err := expander.ValidateExpansion(t.Context(), result)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skipped count cannot be negative")
	})

	t.Run("Should fail validation for mismatched counts", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config1 := createValidTaskConfig("task1", task.TaskTypeBasic)
		result := &shared.ExpansionResult{
			ChildConfigs: []*task.Config{config1},
			ItemCount:    2, // Mismatch!
			SkippedCount: 0,
		}

		// Act
		err := expander.ValidateExpansion(t.Context(), result)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child configs count (1) does not match item count (2)")
	})

	t.Run("Should fail validation for nil child config", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		result := &shared.ExpansionResult{
			ChildConfigs: []*task.Config{nil},
			ItemCount:    1,
			SkippedCount: 0,
		}

		// Act
		err := expander.ValidateExpansion(t.Context(), result)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child config at index 0 is nil")
	})
}

func TestExpander_InjectCollectionContext(t *testing.T) {
	t.Run("Should inject standard collection variables", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child",
			},
		}
		parentConfig := &task.Config{}
		item := map[string]any{"name": "test"}
		index := 2

		// Act
		expander.injectCollectionContext(childConfig, parentConfig, item, index)

		// Assert
		assert.NotNil(t, childConfig.With)

		withMap := map[string]any(*childConfig.With)
		assert.Equal(t, item, withMap["_collection_item"])
		assert.Equal(t, index, withMap["_collection_index"])
	})

	t.Run("Should inject custom variable names", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child",
			},
		}
		parentConfig := &task.Config{
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "user",
				IndexVar: "idx",
			},
		}
		item := map[string]any{"name": "john"}
		index := 1

		// Act
		expander.injectCollectionContext(childConfig, parentConfig, item, index)

		// Assert
		assert.NotNil(t, childConfig.With)

		withMap := map[string]any(*childConfig.With)
		assert.Equal(t, item, withMap["_collection_item"])
		assert.Equal(t, index, withMap["_collection_index"])
		assert.Equal(t, item, withMap["user"])
		assert.Equal(t, index, withMap["idx"])
	})

	t.Run("Should create With field if nil", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child",
				With: nil,
			},
		}
		parentConfig := &task.Config{}
		item := "test"
		index := 0

		// Act
		expander.injectCollectionContext(childConfig, parentConfig, item, index)

		// Assert
		assert.NotNil(t, childConfig.With)

		withMap := map[string]any(*childConfig.With)
		assert.Equal(t, item, withMap["_collection_item"])
		assert.Equal(t, index, withMap["_collection_index"])
	})
}

func TestExpander_InjectCollectionContext_PreservesParentContext(t *testing.T) {
	t.Run("Should preserve existing With context when injecting collection variables", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		parentContext := core.Input{
			"dir":       "/test/directory",
			"someParam": "value",
		}
		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child",
				With: &parentContext,
			},
		}
		parentConfig := &task.Config{
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "file",
				IndexVar: "idx",
			},
		}
		item := "test.go"
		index := 0

		// Act
		expander.injectCollectionContext(childConfig, parentConfig, item, index)

		// Assert
		assert.NotNil(t, childConfig.With)
		withMap := map[string]any(*childConfig.With)

		// Check that parent context is preserved
		assert.Equal(t, "/test/directory", withMap["dir"], "Parent context 'dir' should be preserved")
		assert.Equal(t, "value", withMap["someParam"], "Parent context 'someParam' should be preserved")

		// Check that collection variables are added
		assert.Equal(t, item, withMap["_collection_item"])
		assert.Equal(t, index, withMap["_collection_index"])
		assert.Equal(t, item, withMap["file"])
		assert.Equal(t, index, withMap["idx"])
	})
}

func TestExpander_InjectCollectionContext_DeepCopiesParentWith(t *testing.T) {
	t.Run("Should deep copy parent With into child to avoid aliasing", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		parentWith := core.Input{"dir": "/root", "keep": "yes"}
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{With: &parentWith},
			CollectionConfig: task.CollectionConfig{
				ItemVar:  "file",
				IndexVar: "idx",
			},
		}
		childConfig := &task.Config{BaseConfig: task.BaseConfig{ID: "child"}}
		item := "a.txt"
		index := 3

		// Act
		expander.injectCollectionContext(childConfig, parentConfig, item, index)

		// Assert
		require.NotNil(t, childConfig.With)
		// Mutate child and ensure parent not affected
		(*childConfig.With)["dir"] = "/mutated"
		assert.Equal(t, "/root", (*parentConfig.With)["dir"], "Parent With must not be mutated by child injections")
	})
}

func TestExpander_ValidateChildConfigs(t *testing.T) {
	t.Run("Should pass validation for valid child configs", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config1 := createValidTaskConfig("task1", task.TaskTypeBasic)
		config2 := createValidTaskConfig("task2", task.TaskTypeBasic)
		childConfigs := []*task.Config{config1, config2}

		// Act
		err := expander.validateChildConfigs(t.Context(), childConfigs)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Should fail validation for missing ID", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		config1 := createValidTaskConfig("", task.TaskTypeBasic) // Empty ID
		childConfigs := []*task.Config{config1}

		// Act
		err := expander.validateChildConfigs(t.Context(), childConfigs)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child config at index 0 missing required ID field")
	})

	t.Run("Should handle empty config list", func(t *testing.T) {
		// Arrange
		expander := &Expander{}
		childConfigs := []*task.Config{}

		// Act
		err := expander.validateChildConfigs(t.Context(), childConfigs)

		// Assert
		require.NoError(t, err)
	})
}
