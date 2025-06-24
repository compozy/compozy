package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
)

// ValidationConfig provides validation functionality for task configurations and normalization contexts
type ValidationConfig struct{}

// NewValidationConfig creates a new validation configuration
func NewValidationConfig() *ValidationConfig {
	return &ValidationConfig{}
}

func (vc *ValidationConfig) ValidateConfig(config *task.Config) error {
	if config == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if config.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if !isValidTaskType(config.Type) {
		return fmt.Errorf("invalid task type: %s", config.Type)
	}
	if config.Action == "" && config.Type != task.TaskTypeComposite && config.Type != task.TaskTypeParallel {
		return fmt.Errorf("action is required for task type: %s", config.Type)
	}
	if config.With != nil {
		for key, value := range *config.With {
			if key == "" {
				return fmt.Errorf("empty key in task.With is not allowed")
			}
			if value == nil {
				return fmt.Errorf("nil value for key '%s' in task.With is not allowed", key)
			}
		}
	}
	if config.Env != nil {
		for key, value := range *config.Env {
			if key == "" {
				return fmt.Errorf("empty key in task.Env is not allowed")
			}
			if value == "" {
				return fmt.Errorf("empty value for key '%s' in task.Env is not allowed", key)
			}
		}
	}
	return nil
}

func (vc *ValidationConfig) ValidateNormalizationContext(ctx *NormalizationContext) error {
	if ctx == nil {
		return fmt.Errorf("normalization context cannot be nil")
	}
	if ctx.WorkflowState == nil {
		return fmt.Errorf("workflow state is required in normalization context")
	}
	if ctx.WorkflowConfig == nil {
		return fmt.Errorf("workflow config is required in normalization context")
	}
	if ctx.TaskConfig == nil {
		return fmt.Errorf("task config is required in normalization context")
	}
	return vc.ValidateConfig(ctx.TaskConfig)
}

func isValidTaskType(taskType task.Type) bool {
	validTypes := []task.Type{
		task.TaskTypeBasic,
		task.TaskTypeParallel,
		task.TaskTypeCollection,
		task.TaskTypeRouter,
		task.TaskTypeWait,
		task.TaskTypeAggregate,
		task.TaskTypeComposite,
		task.TaskTypeSignal,
		"", // Empty type defaults to basic
	}
	for _, valid := range validTypes {
		if taskType == valid {
			return true
		}
	}
	return false
}

// InputSanitizer provides input sanitization functionality for template inputs and configuration maps
type InputSanitizer struct{}

// NewInputSanitizer creates a new input sanitizer
func NewInputSanitizer() *InputSanitizer {
	return &InputSanitizer{}
}

func (s *InputSanitizer) SanitizeTemplateInput(input map[string]any) map[string]any {
	if input == nil {
		return make(map[string]any)
	}
	sanitized := make(map[string]any)
	for key, value := range input {
		if key == "" {
			continue
		}
		switch v := value.(type) {
		case string:
			if len(v) > 10000 {
				sanitized[key] = v[:10000]
			} else {
				sanitized[key] = v
			}
		case map[string]any:
			sanitized[key] = s.SanitizeTemplateInput(v)
		default:
			sanitized[key] = value
		}
	}
	return sanitized
}

func (s *InputSanitizer) SanitizeConfigMap(configMap map[string]any) error {
	if configMap == nil {
		return nil
	}
	const maxDepth = 10
	return s.validateDepth(configMap, 0, maxDepth)
}

func (s *InputSanitizer) validateDepth(obj any, currentDepth, maxDepth int) error {
	if currentDepth > maxDepth {
		return fmt.Errorf("configuration structure exceeds maximum depth of %d", maxDepth)
	}
	switch v := obj.(type) {
	case map[string]any:
		for _, value := range v {
			if err := s.validateDepth(value, currentDepth+1, maxDepth); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := s.validateDepth(item, currentDepth+1, maxDepth); err != nil {
				return err
			}
		}
	}
	return nil
}
