package shared

import (
	"os"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
)

func TestValidationConfig_ValidateConfig(t *testing.T) {
	validator := NewValidationConfig()

	t.Run("Should reject nil config", func(t *testing.T) {
		err := validator.ValidateConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Should reject empty task ID", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})

	t.Run("Should reject invalid task type", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: "invalid-type",
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type")
	})

	t.Run("Should require action for basic tasks", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is required")
	})

	t.Run("Should allow empty action for composite tasks", func(t *testing.T) {
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeComposite,
			},
		}
		err := validator.ValidateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Should reject empty keys in With", func(t *testing.T) {
		with := core.Input{
			"": "value",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &with,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty key in task.With")
	})

	t.Run("Should reject nil values in With", func(t *testing.T) {
		with := core.Input{
			"key": nil,
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &with,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil value for key 'key'")
	})

	t.Run("Should reject empty keys in Env", func(t *testing.T) {
		env := core.EnvMap{
			"": "value",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Env:  &env,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty key in task.Env")
	})

	t.Run("Should reject empty values in Env", func(t *testing.T) {
		env := core.EnvMap{
			"KEY": "",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Env:  &env,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty value for key 'KEY'")
	})

	t.Run("Should accept valid config", func(t *testing.T) {
		with := core.Input{
			"param": "value",
		}
		env := core.EnvMap{
			"KEY": "value",
		}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &with,
				Env:  &env,
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}
		err := validator.ValidateConfig(config)
		assert.NoError(t, err)
	})
}

func TestValidationConfig_ValidateNormalizationContext(t *testing.T) {
	validator := NewValidationConfig()

	t.Run("Should reject nil context", func(t *testing.T) {
		err := validator.ValidateNormalizationContext(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Should reject context without workflow state", func(t *testing.T) {
		ctx := &NormalizationContext{}
		err := validator.ValidateNormalizationContext(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state is required")
	})

	t.Run("Should reject context without workflow config", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{},
		}
		err := validator.ValidateNormalizationContext(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow config is required")
	})

	t.Run("Should reject context without task config", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState:  &workflow.State{},
			WorkflowConfig: &workflow.Config{},
		}
		err := validator.ValidateNormalizationContext(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task config is required")
	})

	t.Run("Should validate task config in context", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState:  &workflow.State{},
			WorkflowConfig: &workflow.Config{},
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					Type: "invalid-type",
				},
			},
		}
		err := validator.ValidateNormalizationContext(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type")
	})

	t.Run("Should accept valid context", func(t *testing.T) {
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
		err := validator.ValidateNormalizationContext(ctx)
		assert.NoError(t, err)
	})
}

func TestInputSanitizer_NewInputSanitizer(t *testing.T) {
	t.Run("Should create sanitizer with default max string length", func(t *testing.T) {
		sanitizer := NewInputSanitizer()
		assert.Equal(t, 10485760, sanitizer.GetMaxStringLength())
	})

	t.Run("Should read max string length from environment variable", func(t *testing.T) {
		// Save original value
		originalVal := os.Getenv("MAX_STRING_LENGTH")
		defer func() {
			if originalVal == "" {
				os.Unsetenv("MAX_STRING_LENGTH")
			} else {
				os.Setenv("MAX_STRING_LENGTH", originalVal)
			}
		}()

		// Set test value
		os.Setenv("MAX_STRING_LENGTH", "5000")

		sanitizer := NewInputSanitizer()
		assert.Equal(t, 5000, sanitizer.GetMaxStringLength())
	})

	t.Run("Should use default when env var is invalid", func(t *testing.T) {
		// Save original value
		originalVal := os.Getenv("MAX_STRING_LENGTH")
		defer func() {
			if originalVal == "" {
				os.Unsetenv("MAX_STRING_LENGTH")
			} else {
				os.Setenv("MAX_STRING_LENGTH", originalVal)
			}
		}()

		// Set invalid value
		os.Setenv("MAX_STRING_LENGTH", "invalid")

		sanitizer := NewInputSanitizer()
		assert.Equal(t, 10485760, sanitizer.GetMaxStringLength())
	})
}

func TestInputSanitizer_WithMaxStringLength(t *testing.T) {
	t.Run("Should set custom max string length", func(t *testing.T) {
		sanitizer := NewInputSanitizer().WithMaxStringLength(1000)
		assert.Equal(t, 1000, sanitizer.GetMaxStringLength())
	})
}

func TestInputSanitizer_SanitizeTemplateInput(t *testing.T) {
	t.Run("Should handle nil input", func(t *testing.T) {
		sanitizer := NewInputSanitizer()
		result := sanitizer.SanitizeTemplateInput(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("Should remove empty keys", func(t *testing.T) {
		sanitizer := NewInputSanitizer()
		input := map[string]any{
			"":      "should be removed",
			"valid": "should remain",
		}
		result := sanitizer.SanitizeTemplateInput(input)
		assert.NotContains(t, result, "")
		assert.Contains(t, result, "valid")
		assert.Equal(t, "should remain", result["valid"])
	})

	t.Run("Should truncate long strings using configurable limit", func(t *testing.T) {
		sanitizer := NewInputSanitizer().WithMaxStringLength(1000)
		longString := make([]byte, 1500)
		for i := range longString {
			longString[i] = 'a'
		}
		input := map[string]any{
			"long": string(longString),
		}
		result := sanitizer.SanitizeTemplateInput(input)
		assert.Len(t, result["long"].(string), 1000)
	})

	t.Run("Should not truncate strings under limit", func(t *testing.T) {
		sanitizer := NewInputSanitizer().WithMaxStringLength(1000)
		shortString := "short string"
		input := map[string]any{
			"short": shortString,
		}
		result := sanitizer.SanitizeTemplateInput(input)
		assert.Equal(t, shortString, result["short"])
	})

	t.Run("Should recursively sanitize nested maps", func(t *testing.T) {
		sanitizer := NewInputSanitizer().WithMaxStringLength(1000)
		longString := make([]byte, 1500)
		for i := range longString {
			longString[i] = 'b'
		}
		input := map[string]any{
			"nested": map[string]any{
				"":      "should be removed",
				"valid": "should remain",
				"long":  string(longString),
			},
		}
		result := sanitizer.SanitizeTemplateInput(input)
		nested := result["nested"].(map[string]any)
		assert.NotContains(t, nested, "")
		assert.Contains(t, nested, "valid")
		assert.Contains(t, nested, "long")
		assert.Len(t, nested["long"].(string), 1000)
	})
}

func TestInputSanitizer_SanitizeConfigMap(t *testing.T) {
	sanitizer := NewInputSanitizer()

	t.Run("Should handle nil config", func(t *testing.T) {
		err := sanitizer.SanitizeConfigMap(nil)
		assert.NoError(t, err)
	})

	t.Run("Should reject deeply nested structures", func(t *testing.T) {
		// Create a structure with 12 levels of nesting (exceeds limit of 10)
		deeply := make(map[string]any)
		current := deeply
		for i := 0; i < 12; i++ {
			next := make(map[string]any)
			current["next"] = next
			current = next
		}
		err := sanitizer.SanitizeConfigMap(deeply)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum depth")
	})

	t.Run("Should accept normally nested structures", func(t *testing.T) {
		config := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": "value",
				},
			},
		}
		err := sanitizer.SanitizeConfigMap(config)
		assert.NoError(t, err)
	})

	t.Run("Should handle arrays in structures", func(t *testing.T) {
		config := map[string]any{
			"array": []any{
				map[string]any{
					"nested": "value",
				},
			},
		}
		err := sanitizer.SanitizeConfigMap(config)
		assert.NoError(t, err)
	})
}

func TestIsValidTaskType(t *testing.T) {
	validTypes := []task.Type{
		task.TaskTypeBasic,
		task.TaskTypeParallel,
		task.TaskTypeCollection,
		task.TaskTypeRouter,
		task.TaskTypeWait,
		task.TaskTypeAggregate,
		task.TaskTypeComposite,
		task.TaskTypeSignal,
		"",
	}

	t.Run("Should accept all valid task types", func(t *testing.T) {
		for _, taskType := range validTypes {
			assert.True(t, isValidTaskType(taskType), "Task type %s should be valid", taskType)
		}
	})

	t.Run("Should reject invalid task types", func(t *testing.T) {
		invalidTypes := []task.Type{"invalid", "unknown", "bad-type"}
		for _, taskType := range invalidTypes {
			assert.False(t, isValidTaskType(taskType), "Task type %s should be invalid", taskType)
		}
	})
}
