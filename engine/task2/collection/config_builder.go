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
	if templateEngine == nil {
		panic("templateEngine cannot be nil")
	}
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
	// Build merged input for the task - this will include workflow context
	mergedInput, err := cb.buildMergedInput(parentTaskConfig, taskConfig, collectionConfig, item, index, itemContext)
	if err != nil {
		return nil, err
	}
	taskConfig.With = &mergedInput
	// Process task ID if it contains templates
	if err := cb.processTaskID(taskConfig, itemContext); err != nil {
		return nil, err
	}
	// Inherit parent config properties
	if err := cb.inheritParentConfig(taskConfig, parentTaskConfig); err != nil {
		return nil, fmt.Errorf("failed to inherit parent config: %w", err)
	}
	return taskConfig, nil
}

// buildMergedInput builds the merged input for a collection item task
func (cb *ConfigBuilder) buildMergedInput(
	parentTaskConfig *task.Config,
	taskConfig *task.Config,
	collectionConfig *task.CollectionConfig,
	item any,
	index int,
	itemContext map[string]any,
) (core.Input, error) {
	mergedInput := make(core.Input)
	// Start with parent task with if available
	if parentTaskConfig.With != nil {
		// Process templates in the with field using the item context
		processedWith, err := cb.templateEngine.ParseAny(*parentTaskConfig.With, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process parent with field templates: %w", err)
		}
		// Convert processed result to Input map
		switch v := processedWith.(type) {
		case map[string]any:
			maps.Copy(mergedInput, v)
		case core.Input:
			maps.Copy(mergedInput, v)
		default:
			return nil, fmt.Errorf("processed parent with field is not a map: %T", processedWith)
		}
	}
	// Process and add task template with
	if err := cb.processTaskWith(taskConfig, itemContext, mergedInput); err != nil {
		return nil, err
	}
	// Add collection context fields
	cb.addCollectionContext(mergedInput, collectionConfig, item, index)
	// Add workflow context from itemContext for nested tasks to access
	if workflow, ok := itemContext["workflow"]; ok {
		mergedInput["workflow"] = workflow
	}
	return mergedInput, nil
}

// processTaskWith processes the task's with field and adds it to the merged input
func (cb *ConfigBuilder) processTaskWith(
	taskConfig *task.Config,
	itemContext map[string]any,
	mergedInput core.Input,
) error {
	if taskConfig.With == nil {
		return nil
	}
	// Process templates in the with field using the item context
	processedWith, err := cb.templateEngine.ParseAny(*taskConfig.With, itemContext)
	if err != nil {
		return fmt.Errorf("failed to process with field templates: %w", err)
	}
	// Convert processed result to Input map
	switch v := processedWith.(type) {
	case map[string]any:
		maps.Copy(mergedInput, v)
	case core.Input:
		maps.Copy(mergedInput, v)
	default:
		return fmt.Errorf("processed with field is not a map: %T", processedWith)
	}
	return nil
}

// addCollectionContext adds collection-specific fields to the input
func (cb *ConfigBuilder) addCollectionContext(
	mergedInput core.Input,
	collectionConfig *task.CollectionConfig,
	item any,
	index int,
) {
	// Add standard fields
	mergedInput["item"] = item
	mergedInput["index"] = index
	// Apply custom item/index keys if specified
	if collectionConfig.GetItemVar() != "" {
		mergedInput[collectionConfig.GetItemVar()] = item
		// Store the custom variable name so it can be used during output transformation
		mergedInput[shared.FieldCollectionItemVar] = collectionConfig.GetItemVar()
	}
	if collectionConfig.GetIndexVar() != "" {
		mergedInput[collectionConfig.GetIndexVar()] = index
		// Store the custom variable name so it can be used during output transformation
		mergedInput[shared.FieldCollectionIndexVar] = collectionConfig.GetIndexVar()
	}
	// Store the standard collection fields for output transformation
	mergedInput[shared.FieldCollectionItem] = item
	mergedInput[shared.FieldCollectionIndex] = index
}

// processTaskID processes the task ID if it contains templates
func (cb *ConfigBuilder) processTaskID(taskConfig *task.Config, itemContext map[string]any) error {
	if taskConfig.ID == "" {
		return nil
	}
	processedID, err := cb.templateEngine.ParseAny(taskConfig.ID, itemContext)
	if err != nil {
		return fmt.Errorf("failed to process task ID template: %w", err)
	}
	value, ok := processedID.(string)
	if !ok {
		return fmt.Errorf("task ID is not a string")
	}
	taskConfig.ID = value
	return nil
}

// inheritParentConfig copies relevant fields from parent to child config
func (cb *ConfigBuilder) inheritParentConfig(taskConfig, parentTaskConfig *task.Config) error {
	// Copy CWD from parent config to child config if not already set
	if taskConfig.CWD == nil && parentTaskConfig.CWD != nil {
		taskConfig.CWD = parentTaskConfig.CWD
		// Recursively propagate CWD to all nested tasks
		if err := task.PropagateSingleTaskCWD(taskConfig, taskConfig.CWD, "collection item task"); err != nil {
			return fmt.Errorf("failed to propagate CWD to nested tasks: %w", err)
		}
	}
	// Copy FilePath from parent config to child config if not already set
	if taskConfig.FilePath == "" && parentTaskConfig.FilePath != "" {
		taskConfig.FilePath = parentTaskConfig.FilePath
	}
	return nil
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
