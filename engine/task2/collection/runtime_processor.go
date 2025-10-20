package collection

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/tplengine"
)

// RuntimeProcessor handles runtime template processing for collection items
type RuntimeProcessor struct {
	templateEngine *tplengine.TemplateEngine
}

// NewRuntimeProcessor creates a new runtime processor
func NewRuntimeProcessor(templateEngine *tplengine.TemplateEngine) *RuntimeProcessor {
	return &RuntimeProcessor{
		templateEngine: templateEngine,
	}
}

// ProcessItemConfig processes a task configuration with item-specific context
// It resolves all templates in the configuration using the provided context
func (r *RuntimeProcessor) ProcessItemConfig(
	baseConfig *task.Config,
	itemContext map[string]any,
) (*task.Config, error) {
	if baseConfig == nil {
		return nil, fmt.Errorf("base config cannot be nil")
	}
	configMap, err := baseConfig.AsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to map: %w", err)
	}
	processedMap, err := r.processConfigMap(configMap, itemContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process config templates: %w", err)
	}
	newConfig := &task.Config{}
	if err := newConfig.FromMap(processedMap); err != nil {
		return nil, fmt.Errorf("failed to create config from processed map: %w", err)
	}
	return newConfig, nil
}

// processConfigMap recursively processes all template fields in the config map
func (r *RuntimeProcessor) processConfigMap(configMap map[string]any, context map[string]any) (map[string]any, error) {
	processedMap, err := r.templateEngine.ParseMapWithFilter(configMap, context, func(key string) bool {
		return key == "outputs"
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process templates: %w", err)
	}
	resultMap, ok := processedMap.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map result, got %T", processedMap)
	}
	if err := r.processSpecialFields(resultMap, context); err != nil {
		return nil, fmt.Errorf("failed to process special fields: %w", err)
	}
	return resultMap, nil
}

// processSpecialFields handles fields that need special template processing
func (r *RuntimeProcessor) processSpecialFields(configMap map[string]any, context map[string]any) error {
	if err := r.processIDField(configMap, context); err != nil {
		return err
	}
	if err := r.processActionField(configMap, context); err != nil {
		return err
	}
	if err := r.processWithField(configMap, context); err != nil {
		return err
	}
	if err := r.processAgentField(configMap, context); err != nil {
		return err
	}
	if err := r.processToolField(configMap, context); err != nil {
		return err
	}
	return nil
}

// processStringField processes a string field in the config map if it contains templates
func (r *RuntimeProcessor) processStringField(
	configMap map[string]any,
	fieldName string,
	context map[string]any,
) error {
	if value, ok := configMap[fieldName].(string); ok && tplengine.HasTemplate(value) {
		processed, err := r.templateEngine.ParseAny(value, context)
		if err != nil {
			return fmt.Errorf("failed to process %s template: %w", fieldName, err)
		}
		if processedValue, ok := processed.(string); ok {
			configMap[fieldName] = processedValue
		}
	}
	return nil
}

// processNestedMapField processes a map field with special nested handling
func (r *RuntimeProcessor) processNestedMapField(
	configMap map[string]any,
	fieldName string,
	context map[string]any,
	nestedFields map[string]bool,
) error {
	fieldConfig, ok := configMap[fieldName]
	if !ok || fieldConfig == nil {
		return nil
	}
	fieldMap, ok := fieldConfig.(map[string]any)
	if !ok {
		return nil
	}
	processedField, err := r.templateEngine.ParseAny(fieldMap, context)
	if err != nil {
		return fmt.Errorf("failed to process %s config: %w", fieldName, err)
	}
	if processedMap, ok := processedField.(map[string]any); ok {
		for nestedField := range nestedFields {
			if nestedValue, ok := processedMap[nestedField]; ok && nestedValue != nil {
				processedNested, err := r.templateEngine.ParseAny(nestedValue, context)
				if err != nil {
					return fmt.Errorf("failed to process %s %s: %w", fieldName, nestedField, err)
				}
				processedMap[nestedField] = processedNested
			}
		}
		configMap[fieldName] = processedMap
	} else {
		configMap[fieldName] = processedField
	}
	return nil
}

// processIDField processes the ID field if it contains templates
func (r *RuntimeProcessor) processIDField(configMap map[string]any, context map[string]any) error {
	return r.processStringField(configMap, "id", context)
}

// processActionField processes the action field if it contains templates
func (r *RuntimeProcessor) processActionField(configMap map[string]any, context map[string]any) error {
	return r.processStringField(configMap, "action", context)
}

// processWithField processes the 'with' parameters with deep processing for nested maps
func (r *RuntimeProcessor) processWithField(configMap map[string]any, context map[string]any) error {
	if withParams, ok := configMap["with"]; ok && withParams != nil {
		processed, err := r.processWithParameters(withParams, context)
		if err != nil {
			return fmt.Errorf("failed to process 'with' parameters: %w", err)
		}
		configMap["with"] = processed
	}
	return nil
}

// processAgentField processes agent configuration
func (r *RuntimeProcessor) processAgentField(configMap map[string]any, context map[string]any) error {
	if agentConfig, ok := configMap["agent"]; ok && agentConfig != nil {
		processed, err := r.processAgentConfig(agentConfig, context)
		if err != nil {
			return fmt.Errorf("failed to process agent config: %w", err)
		}
		configMap["agent"] = processed
	}
	return nil
}

// processToolField processes tool configuration
func (r *RuntimeProcessor) processToolField(configMap map[string]any, context map[string]any) error {
	if toolConfig, ok := configMap["tool"]; ok && toolConfig != nil {
		processed, err := r.processToolConfig(toolConfig, context)
		if err != nil {
			return fmt.Errorf("failed to process tool config: %w", err)
		}
		configMap["tool"] = processed
	}
	return nil
}

// processWithParameters processes the 'with' parameters deeply
func (r *RuntimeProcessor) processWithParameters(withParams any, context map[string]any) (any, error) {
	return r.templateEngine.ParseWithJSONHandling(withParams, context)
}

// processAgentConfig processes agent configuration templates
func (r *RuntimeProcessor) processAgentConfig(agentConfig any, context map[string]any) (any, error) {
	tempMap := map[string]any{"agent": agentConfig}
	nestedFields := map[string]bool{"settings": true}
	if err := r.processNestedMapField(tempMap, "agent", context, nestedFields); err != nil {
		return nil, err
	}
	return tempMap["agent"], nil
}

// processToolConfig processes tool configuration templates
func (r *RuntimeProcessor) processToolConfig(toolConfig any, context map[string]any) (any, error) {
	tempMap := map[string]any{"tool": toolConfig}
	nestedFields := map[string]bool{"params": true}
	if err := r.processNestedMapField(tempMap, "tool", context, nestedFields); err != nil {
		return nil, err
	}
	return tempMap["tool"], nil
}
