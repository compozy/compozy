package shared

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
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
		err := IterateSortedMap(*config.With, func(key string, value any) error {
			if key == "" {
				return fmt.Errorf("empty key in task.With is not allowed")
			}
			if value == nil {
				return fmt.Errorf("nil value for key '%s' in task.With is not allowed", key)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if config.Env != nil {
		err := IterateSortedMap(*config.Env, func(key, value string) error {
			if key == "" {
				return fmt.Errorf("empty key in task.Env is not allowed")
			}
			if value == "" {
				return fmt.Errorf("empty value for key '%s' in task.Env is not allowed", key)
			}
			return nil
		})
		if err != nil {
			return err
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

var validTaskTypes = map[task.Type]bool{
	task.TaskTypeBasic:      true,
	task.TaskTypeParallel:   true,
	task.TaskTypeCollection: true,
	task.TaskTypeRouter:     true,
	task.TaskTypeWait:       true,
	task.TaskTypeAggregate:  true,
	task.TaskTypeComposite:  true,
	task.TaskTypeSignal:     true,
	"":                      true, // Empty type defaults to basic
}

func isValidTaskType(taskType task.Type) bool {
	return validTaskTypes[taskType]
}

// InputSanitizer provides input sanitization functionality for template inputs and configuration maps
type InputSanitizer struct {
	maxStringLength int
}

// NewInputSanitizer creates a new input sanitizer with configurable string length limit
func NewInputSanitizer() *InputSanitizer {
	maxStringLength := 10485760 // 10MB default
	if envVal := os.Getenv("MAX_STRING_LENGTH"); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			maxStringLength = val
		}
	}
	return &InputSanitizer{
		maxStringLength: maxStringLength,
	}
}

// WithMaxStringLength sets the maximum string length for truncation
func (s *InputSanitizer) WithMaxStringLength(length int) *InputSanitizer {
	s.maxStringLength = length
	return s
}

// GetMaxStringLength returns the current maximum string length setting
func (s *InputSanitizer) GetMaxStringLength() int {
	return s.maxStringLength
}

// SanitizeTemplateInput sanitizes template input by truncating long strings and removing empty keys.
// Empty keys are logged as warnings since they may indicate bugs in caller code.
func (s *InputSanitizer) SanitizeTemplateInput(input map[string]any) map[string]any {
	if input == nil {
		return make(map[string]any)
	}
	sanitized := make(map[string]any)
	log := logger.FromContext(context.Background())
	keys := SortedMapKeys(input)
	for _, key := range keys {
		value := input[key]
		if key == "" {
			log.Warn("Empty key found in template input - this may indicate a bug in caller code")
			continue
		}
		switch v := value.(type) {
		case string:
			if len(v) > s.maxStringLength {
				log.Warn("String value truncated from %d to %d characters for key '%s'", len(v), s.maxStringLength, key)
				sanitized[key] = v[:s.maxStringLength]
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
