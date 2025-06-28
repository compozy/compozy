package collection

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ConfigBuilder builds collection task configurations
type ConfigBuilder struct {
	templateEngine *tplengine.TemplateEngine
}

// NewConfigBuilder creates a new config builder
func NewConfigBuilder(templateEngine *tplengine.TemplateEngine) *ConfigBuilder {
	return &ConfigBuilder{
		templateEngine: templateEngine,
	}
}

// GetTemplateEngine returns the template engine instance
func (cb *ConfigBuilder) GetTemplateEngine() *tplengine.TemplateEngine {
	return cb.templateEngine
}

// BuildTaskConfig builds a task config for a collection item
func (cb *ConfigBuilder) BuildTaskConfig(
	collectionConfig *task.CollectionConfig,
	parentTaskConfig *task.Config,
	item any,
	index int,
	context map[string]any,
) (*task.Config, error) {
	if parentTaskConfig.Task == nil {
		return nil, fmt.Errorf("collection task template is required")
	}
	// Create item context with item and index
	itemContext := cb.createItemContext(context, collectionConfig, item, index)
	// Clone the task template using deep copy to avoid shared references
	taskConfig, err := parentTaskConfig.Task.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone task template: %w", err)
	}
	// Merge inputs: parent.with -> task.with -> item context
	mergedInput := make(core.Input)
	// Start with parent task with if available
	if parentTaskConfig.With != nil {
		maps.Copy(mergedInput, *parentTaskConfig.With)
	}
	// Add task template with
	if taskConfig.With != nil {
		maps.Copy(mergedInput, *taskConfig.With)
	}
	// Add item context
	mergedInput["item"] = item
	mergedInput["index"] = index
	// Apply custom item/index keys if specified
	if collectionConfig.GetItemVar() != "" {
		mergedInput[collectionConfig.GetItemVar()] = item
	}
	if collectionConfig.GetIndexVar() != "" {
		mergedInput[collectionConfig.GetIndexVar()] = index
	}
	taskConfig.With = &mergedInput
	// Generate unique task ID if it contains templates
	if taskConfig.ID != "" {
		processedID, err := cb.templateEngine.ParseAny(taskConfig.ID, itemContext)
		value, ok := processedID.(string)
		if !ok {
			return nil, fmt.Errorf("task ID is not a string")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to process task ID template: %w", err)
		}
		taskConfig.ID = value
	}
	return taskConfig, nil
}

// createItemContext creates a context for a collection item
func (cb *ConfigBuilder) createItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	// Clone base context in deterministic order
	itemContext := make(map[string]any)
	keys := shared.SortedMapKeys(baseContext)
	for _, k := range keys {
		itemContext[k] = baseContext[k]
	}
	// Add item and index
	itemContext["item"] = item
	itemContext["index"] = index
	// Add custom keys if specified
	if config.GetItemVar() != "" {
		itemContext[config.GetItemVar()] = item
	}
	if config.GetIndexVar() != "" {
		itemContext[config.GetIndexVar()] = index
	}
	return itemContext
}
