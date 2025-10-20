package collection

import (
	"fmt"

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

// parseInputTemplate parses a template input and returns it as core.Input
// parseInputTemplate parses a template input and returns it as core.Input.
// It defers unresolved runtime task references by leveraging ParseMapWithFilter.
// parseInputTemplate parses a template input and returns it as core.Input
func (cb *ConfigBuilder) parseInputTemplate(input core.Input, context map[string]any) (core.Input, error) {
	processedWith, err := cb.templateEngine.ParseAny(input, context)
	if err != nil {
		return nil, fmt.Errorf("failed to process templates: %w", err)
	}
	switch v := processedWith.(type) {
	case map[string]any:
		return v, nil
	case core.Input:
		return v, nil
	default:
		return nil, fmt.Errorf("processed field is not a map: %T", processedWith)
	}
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
	itemContext := cb.createItemContext(context, collectionConfig, item, index)
	taskConfig, err := parentTaskConfig.Task.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone task template: %w", err)
	}
	mergedInput, err := cb.buildMergedInput(parentTaskConfig, taskConfig, collectionConfig, item, index, itemContext)
	if err != nil {
		return nil, err
	}
	taskConfig.With = &mergedInput
	if err := cb.processTaskID(taskConfig, itemContext); err != nil {
		return nil, err
	}
	if err := shared.InheritTaskConfig(taskConfig, parentTaskConfig); err != nil {
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
	if parentTaskConfig.With != nil {
		processed, err := cb.parseInputTemplate(*parentTaskConfig.With, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to process parent with field: %w", err)
		}
		mergedInput = core.CopyMaps(mergedInput, processed)
	}
	if err := cb.processTaskWith(taskConfig, itemContext, mergedInput); err != nil {
		return nil, err
	}
	cb.addCollectionContext(mergedInput, collectionConfig, item, index)
	if workflow, ok := itemContext["workflow"]; ok {
		if _, exists := mergedInput["workflow"]; !exists {
			mergedInput["workflow"] = workflow
		}
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
	processed, err := cb.parseInputTemplate(*taskConfig.With, itemContext)
	if err != nil {
		return fmt.Errorf("failed to process with field: %w", err)
	}
	merged := core.CopyMaps(mergedInput, processed)
	for k, v := range merged {
		mergedInput[k] = v
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
	mergedInput["item"] = item
	mergedInput["index"] = index
	if collectionConfig.GetItemVar() != "" {
		mergedInput[collectionConfig.GetItemVar()] = item
		mergedInput[shared.FieldCollectionItemVar] = collectionConfig.GetItemVar()
	}
	if collectionConfig.GetIndexVar() != "" {
		mergedInput[collectionConfig.GetIndexVar()] = index
		mergedInput[shared.FieldCollectionIndexVar] = collectionConfig.GetIndexVar()
	}
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

// createItemContext creates a context for a collection item
func (cb *ConfigBuilder) createItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	itemContext := make(map[string]any)
	keys := shared.SortedMapKeys(baseContext)
	for _, k := range keys {
		itemContext[k] = baseContext[k]
	}
	itemContext["item"] = item
	itemContext["index"] = index
	if config.GetItemVar() != "" {
		itemContext[config.GetItemVar()] = item
	}
	if config.GetIndexVar() != "" {
		itemContext[config.GetIndexVar()] = index
	}
	return itemContext
}
