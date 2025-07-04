package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// Tests for NewValidationConfig method to increase coverage
func TestNewValidationConfig(t *testing.T) {
	t.Run("Should create new validation config", func(t *testing.T) {
		// Act
		config := NewValidationConfig()
		// Assert
		assert.NotNil(t, config)
	})
}

// Tests for isValidTaskType method to increase coverage
func TestIsValidTaskType(t *testing.T) {
	t.Run("Should validate basic task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeBasic))
	})

	t.Run("Should validate parallel task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeParallel))
	})

	t.Run("Should validate collection task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeCollection))
	})

	t.Run("Should validate composite task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeComposite))
	})

	t.Run("Should validate signal task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeSignal))
	})

	t.Run("Should validate router task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeRouter))
	})

	t.Run("Should validate wait task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeWait))
	})

	t.Run("Should validate aggregate task type", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(task.TaskTypeAggregate))
	})

	t.Run("Should validate empty task type as basic", func(t *testing.T) {
		// Act & Assert
		assert.True(t, isValidTaskType(""))
	})

	t.Run("Should reject invalid task type", func(t *testing.T) {
		// Act & Assert
		assert.False(t, isValidTaskType("invalid_type"))
	})
}

// Tests for NewInputSanitizer method to increase coverage
func TestNewInputSanitizer(t *testing.T) {
	t.Run("Should create input sanitizer with default settings", func(t *testing.T) {
		// Act
		sanitizer := NewInputSanitizer()
		// Assert
		assert.NotNil(t, sanitizer)
		assert.Equal(t, 10485760, sanitizer.GetMaxStringLength()) // 10MB default
	})
}

// Tests for InputSanitizer methods to increase coverage
func TestInputSanitizer_WithMaxStringLength(t *testing.T) {
	t.Run("Should set max string length", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		// Act
		result := sanitizer.WithMaxStringLength(1024)
		// Assert
		assert.Same(t, sanitizer, result) // Should return same instance
		assert.Equal(t, 1024, sanitizer.GetMaxStringLength())
	})
}

func TestInputSanitizer_GetMaxStringLength(t *testing.T) {
	t.Run("Should return current max string length", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		sanitizer.WithMaxStringLength(2048)
		// Act
		length := sanitizer.GetMaxStringLength()
		// Assert
		assert.Equal(t, 2048, length)
	})
}

// Tests for SanitizeTemplateInput method to increase coverage
func TestInputSanitizer_SanitizeTemplateInput(t *testing.T) {
	t.Run("Should handle nil input", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		// Act
		result := sanitizer.SanitizeTemplateInput(nil)
		// Assert
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("Should sanitize normal input", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		input := map[string]any{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
		// Act
		result := sanitizer.SanitizeTemplateInput(input)
		// Assert
		assert.NotNil(t, result)
		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, 123, result["key2"])
		assert.Equal(t, true, result["key3"])
	})

	t.Run("Should filter out empty keys", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		input := map[string]any{
			"":     "empty_key_value",
			"key1": "value1",
		}
		// Act
		result := sanitizer.SanitizeTemplateInput(input)
		// Assert
		assert.NotNil(t, result)
		assert.NotContains(t, result, "")
		assert.Contains(t, result, "key1")
		assert.Equal(t, "value1", result["key1"])
	})
}

// Tests for ValidateConfig method to increase coverage
func TestValidationConfig_ValidateConfig(t *testing.T) {
	config := NewValidationConfig()

	t.Run("Should reject nil config", func(t *testing.T) {
		// Act
		err := config.ValidateConfig(nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should reject config with empty ID", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "", // Empty ID
				Type: task.TaskTypeBasic,
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task ID is required")
	})

	t.Run("Should reject config with invalid task type", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: "invalid_type",
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type")
	})

	t.Run("Should validate basic task config successfully", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should validate composite task without action", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-composite",
				Type: task.TaskTypeComposite,
				// No action required for composite tasks
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should validate parallel task without action", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-parallel",
				Type: task.TaskTypeParallel,
				// No action required for parallel tasks
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should reject non-composite task without action", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-basic",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				// Missing action for basic task
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is required")
	})

	t.Run("Should reject config with empty key in With", func(t *testing.T) {
		// Arrange
		with := core.Input{
			"":     "value_for_empty_key", // Invalid empty key
			"key1": "value1",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &with,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty key in task.With is not allowed")
	})

	t.Run("Should reject config with nil value in With", func(t *testing.T) {
		// Arrange
		with := core.Input{
			"key1": nil, // Invalid nil value
			"key2": "value2",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &with,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		// Act
		err := config.ValidateConfig(taskConfig)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil value for key 'key1' in task.With is not allowed")
	})
}

// Tests for SanitizeConfigMap method to increase coverage
func TestInputSanitizer_SanitizeConfigMap(t *testing.T) {
	t.Run("Should handle nil config map", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		// Act
		err := sanitizer.SanitizeConfigMap(nil)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should validate simple config map", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		configMap := map[string]any{
			"key1": "value1",
			"key2": 123,
		}
		// Act
		err := sanitizer.SanitizeConfigMap(configMap)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should validate nested config map", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		configMap := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": "value",
				},
			},
		}
		// Act
		err := sanitizer.SanitizeConfigMap(configMap)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should validate config map with arrays", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		configMap := map[string]any{
			"array": []any{
				"item1",
				map[string]any{"nested": "value"},
				123,
			},
		}
		// Act
		err := sanitizer.SanitizeConfigMap(configMap)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should reject config map exceeding max depth", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		// Create deeply nested structure (11 levels deep)
		configMap := map[string]any{
			"l1": map[string]any{
				"l2": map[string]any{
					"l3": map[string]any{
						"l4": map[string]any{
							"l5": map[string]any{
								"l6": map[string]any{
									"l7": map[string]any{
										"l8": map[string]any{
											"l9": map[string]any{
												"l10": map[string]any{
													"l11": "too deep",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Act
		err := sanitizer.SanitizeConfigMap(configMap)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration structure exceeds maximum depth of 10")
	})

	t.Run("Should reject array exceeding max depth", func(t *testing.T) {
		// Arrange
		sanitizer := NewInputSanitizer()
		// Create deeply nested array structure
		configMap := map[string]any{
			"array": []any{
				[]any{
					[]any{
						[]any{
							[]any{
								[]any{
									[]any{
										[]any{
											[]any{
												[]any{
													[]any{"too deep"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Act
		err := sanitizer.SanitizeConfigMap(configMap)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration structure exceeds maximum depth of 10")
	})
}

// Tests for ValidateNormalizationContext method to increase coverage
func TestValidationConfig_ValidateNormalizationContext(t *testing.T) {
	config := NewValidationConfig()

	t.Run("Should reject nil context", func(t *testing.T) {
		// Act
		err := config.ValidateNormalizationContext(nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "normalization context cannot be nil")
	})

	t.Run("Should reject context with nil workflow state", func(t *testing.T) {
		// Arrange
		ctx := &NormalizationContext{
			WorkflowState:  nil, // Invalid
			WorkflowConfig: &workflow.Config{},
			TaskConfig:     &task.Config{BaseConfig: task.BaseConfig{ID: "test", Type: task.TaskTypeBasic}},
		}
		// Act
		err := config.ValidateNormalizationContext(ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state is required")
	})

	t.Run("Should reject context with nil workflow config", func(t *testing.T) {
		// Arrange
		ctx := &NormalizationContext{
			WorkflowState:  &workflow.State{},
			WorkflowConfig: nil, // Invalid
			TaskConfig:     &task.Config{BaseConfig: task.BaseConfig{ID: "test", Type: task.TaskTypeBasic}},
		}
		// Act
		err := config.ValidateNormalizationContext(ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow config is required")
	})

	t.Run("Should reject context with nil task config", func(t *testing.T) {
		// Arrange
		ctx := &NormalizationContext{
			WorkflowState:  &workflow.State{},
			WorkflowConfig: &workflow.Config{},
			TaskConfig:     nil, // Invalid
		}
		// Act
		err := config.ValidateNormalizationContext(ctx)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task config is required")
	})

	t.Run("Should validate complete context successfully", func(t *testing.T) {
		// Arrange
		ctx := &NormalizationContext{
			WorkflowState:  &workflow.State{},
			WorkflowConfig: &workflow.Config{},
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					Type: task.TaskTypeBasic,
				},
				BasicTask: task.BasicTask{
					Action: "test-action",
				},
			},
		}
		// Act
		err := config.ValidateNormalizationContext(ctx)
		// Assert
		assert.NoError(t, err)
	})
}
