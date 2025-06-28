package collection

import (
	"encoding/json"
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
	// Convert config to map for processing
	configMap, err := baseConfig.AsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to map: %w", err)
	}
	// Process all fields with templates
	processedMap, err := r.processConfigMap(configMap, itemContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process config templates: %w", err)
	}
	// Create new config from processed map
	newConfig := &task.Config{}
	if err := newConfig.FromMap(processedMap); err != nil {
		return nil, fmt.Errorf("failed to create config from processed map: %w", err)
	}
	return newConfig, nil
}

// processConfigMap recursively processes all template fields in the config map
func (r *RuntimeProcessor) processConfigMap(configMap map[string]any, context map[string]any) (map[string]any, error) {
	// Process all fields except outputs (which should be processed after child task execution)
	processedMap, err := r.templateEngine.ParseMapWithFilter(configMap, context, func(key string) bool {
		// Skip outputs field - it will be processed after task execution
		return key == "outputs"
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process templates: %w", err)
	}
	// Ensure the result is a map
	resultMap, ok := processedMap.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map result, got %T", processedMap)
	}
	// Special handling for specific fields that need deep processing
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

// processIDField processes the ID field if it contains templates
func (r *RuntimeProcessor) processIDField(configMap map[string]any, context map[string]any) error {
	if id, ok := configMap["id"].(string); ok && tplengine.HasTemplate(id) {
		processed, err := r.templateEngine.ParseAny(id, context)
		if err != nil {
			return fmt.Errorf("failed to process ID template: %w", err)
		}
		if processedID, ok := processed.(string); ok {
			configMap["id"] = processedID
		}
	}
	return nil
}

// processActionField processes the action field if it contains templates
func (r *RuntimeProcessor) processActionField(configMap map[string]any, context map[string]any) error {
	if action, ok := configMap["action"].(string); ok && tplengine.HasTemplate(action) {
		processed, err := r.templateEngine.ParseAny(action, context)
		if err != nil {
			return fmt.Errorf("failed to process action template: %w", err)
		}
		if processedAction, ok := processed.(string); ok {
			configMap["action"] = processedAction
		}
	}
	return nil
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
	// Handle different types of with parameters
	switch v := withParams.(type) {
	case map[string]any:
		// Process as a map
		return r.templateEngine.ParseAny(v, context)
	case string:
		// It might be a JSON string or a template
		if tplengine.HasTemplate(v) {
			processed, err := r.templateEngine.ParseAny(v, context)
			if err != nil {
				return nil, err
			}
			// If the result is a string that looks like JSON, parse it
			if str, ok := processed.(string); ok && (str[0] == '{' || str[0] == '[') {
				var parsed any
				if err := json.Unmarshal([]byte(str), &parsed); err == nil {
					return parsed, nil
				}
			}
			return processed, nil
		}
		// Try to parse as JSON
		if v != "" && (v[0] == '{' || v[0] == '[') {
			var parsed any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				// Now process any templates in the parsed JSON
				return r.templateEngine.ParseAny(parsed, context)
			}
		}
		return v, nil
	default:
		// For other types, process as-is
		return r.templateEngine.ParseAny(v, context)
	}
}

// processAgentConfig processes agent configuration templates
func (r *RuntimeProcessor) processAgentConfig(agentConfig any, context map[string]any) (any, error) {
	agentMap, ok := agentConfig.(map[string]any)
	if !ok {
		return agentConfig, nil
	}
	// Process all agent fields
	processedAgent, err := r.templateEngine.ParseAny(agentMap, context)
	if err != nil {
		return nil, err
	}
	// Special handling for nested agent settings
	if processedMap, ok := processedAgent.(map[string]any); ok {
		if settings, ok := processedMap["settings"]; ok && settings != nil {
			processedSettings, err := r.templateEngine.ParseAny(settings, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process agent settings: %w", err)
			}
			processedMap["settings"] = processedSettings
		}
	}
	return processedAgent, nil
}

// processToolConfig processes tool configuration templates
func (r *RuntimeProcessor) processToolConfig(toolConfig any, context map[string]any) (any, error) {
	toolMap, ok := toolConfig.(map[string]any)
	if !ok {
		return toolConfig, nil
	}
	// Process all tool fields
	processedTool, err := r.templateEngine.ParseAny(toolMap, context)
	if err != nil {
		return nil, err
	}
	// Special handling for tool parameters
	if processedMap, ok := processedTool.(map[string]any); ok {
		if params, ok := processedMap["params"]; ok && params != nil {
			processedParams, err := r.templateEngine.ParseAny(params, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process tool params: %w", err)
			}
			processedMap["params"] = processedParams
		}
	}
	return processedTool, nil
}
