package collection

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// Expander implements the CollectionExpander domain service
type Expander struct {
	normalizer     *Normalizer
	contextBuilder *shared.ContextBuilder
	configBuilder  *ConfigBuilder
}

// NewExpander creates a new collection expander with required dependencies
func NewExpander(
	normalizer *Normalizer,
	contextBuilder *shared.ContextBuilder,
	configBuilder *ConfigBuilder,
) *Expander {
	return &Expander{
		normalizer:     normalizer,
		contextBuilder: contextBuilder,
		configBuilder:  configBuilder,
	}
}

// ExpandItems expands collection items into child task configurations
func (e *Expander) ExpandItems(
	ctx context.Context,
	config *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*shared.ExpansionResult, error) {
	if err := e.validateInputs(config, workflowState, workflowConfig); err != nil {
		return nil, err
	}
	templateContext := e.contextBuilder.BuildCollectionContext(ctx, workflowState, workflowConfig, config)
	filteredItems, skippedCount, err := e.processCollectionItems(ctx, config, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process collection items: %w", err)
	}
	if len(filteredItems) == 0 {
		return &shared.ExpansionResult{
			ChildConfigs: []*task.Config{},
			ItemCount:    0,
			SkippedCount: skippedCount,
		}, nil
	}
	childConfigs, err := e.createChildConfigs(config, filteredItems, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create child configs: %w", err)
	}
	if err := e.validateChildConfigs(ctx, childConfigs); err != nil {
		return nil, fmt.Errorf("failed to validate child configs: %w", err)
	}
	return &shared.ExpansionResult{
		ChildConfigs: childConfigs,
		ItemCount:    len(filteredItems),
		SkippedCount: skippedCount,
	}, nil
}

// ValidateExpansion validates the expansion result
func (e *Expander) ValidateExpansion(ctx context.Context, result *shared.ExpansionResult) error {
	if result == nil {
		return fmt.Errorf("expansion result cannot be nil")
	}
	if result.ItemCount < 0 {
		return fmt.Errorf("item count cannot be negative: %d", result.ItemCount)
	}
	if result.SkippedCount < 0 {
		return fmt.Errorf("skipped count cannot be negative: %d", result.SkippedCount)
	}
	if len(result.ChildConfigs) != result.ItemCount {
		return fmt.Errorf(
			"child configs count (%d) does not match item count (%d)",
			len(result.ChildConfigs),
			result.ItemCount,
		)
	}
	for i, childConfig := range result.ChildConfigs {
		if childConfig == nil {
			return fmt.Errorf("child config at index %d is nil", i)
		}
		if err := childConfig.Validate(ctx); err != nil {
			return fmt.Errorf("child config at index %d validation failed: %w", i, err)
		}
	}
	return nil
}

// validateInputs validates the inputs for collection expansion
func (e *Expander) validateInputs(
	config *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) error {
	if config == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if config.Type != task.TaskTypeCollection {
		return fmt.Errorf("expected collection task type, got %s", config.Type)
	}
	if workflowState == nil {
		return fmt.Errorf("workflow state cannot be nil")
	}
	if workflowConfig == nil {
		return fmt.Errorf("workflow config cannot be nil")
	}
	return nil
}

// processCollectionItems processes collection items through expansion and filtering pipeline
func (e *Expander) processCollectionItems(
	ctx context.Context,
	config *task.Config,
	templateContext map[string]any,
) ([]any, int, error) {
	items, err := e.normalizer.ExpandCollectionItems(ctx, &config.CollectionConfig, templateContext)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to expand collection items: %w", err)
	}
	filteredItems, err := e.normalizer.FilterCollectionItems(ctx, &config.CollectionConfig, items, templateContext)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to filter collection items: %w", err)
	}
	skippedCount := len(items) - len(filteredItems)
	return filteredItems, skippedCount, nil
}

// createChildConfigs creates child task configurations with collection context injection
func (e *Expander) createChildConfigs(
	config *task.Config,
	filteredItems []any,
	templateContext map[string]any,
) ([]*task.Config, error) {
	childConfigs := make([]*task.Config, len(filteredItems))
	for i, item := range filteredItems {
		itemContext := e.normalizer.CreateItemContext(templateContext, &config.CollectionConfig, item, i)
		childConfig, err := e.configBuilder.BuildTaskConfig(&config.CollectionConfig, config, item, i, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to build child config at index %d: %w", i, err)
		}

		if childConfig.With == nil {
			empty := core.Input{}
			childConfig.With = &empty
		}

		e.injectCollectionContext(childConfig, config, item, i)
		childConfigs[i] = childConfig
	}
	return childConfigs, nil
}

// injectCollectionContext injects collection metadata into child config
func (e *Expander) injectCollectionContext(
	childConfig *task.Config,
	parentConfig *task.Config,
	item any,
	index int,
) {
	if childConfig.With == nil {
		childConfig.With = &core.Input{}
	}
	withMap, err := core.DeepCopy(*childConfig.With) // returns core.Input
	if err != nil {
		withMap = *childConfig.With
	}
	withMap[shared.FieldCollectionItem] = item
	withMap[shared.FieldCollectionIndex] = index
	if parentConfig != nil {
		if iv := parentConfig.GetItemVar(); iv != "" && iv != shared.FieldCollectionItem {
			withMap[iv] = item
		}
		if ix := parentConfig.GetIndexVar(); ix != "" && ix != shared.FieldCollectionIndex {
			withMap[ix] = index
		}
	}
	if parentConfig != nil && parentConfig.With != nil {
		parentMap, err := core.DeepCopy(*parentConfig.With) // returns core.Input
		if err != nil {
			parentMap = *parentConfig.With
		}
		for k, v := range parentMap {
			if _, exists := withMap[k]; !exists {
				withMap[k] = v
			}
		}
	}
	newWith := withMap
	childConfig.With = &newWith
}

// validateChildConfigs validates all child configurations
func (e *Expander) validateChildConfigs(ctx context.Context, childConfigs []*task.Config) error {
	for i, childConfig := range childConfigs {
		if childConfig.ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
		if err := childConfig.Validate(ctx); err != nil {
			return fmt.Errorf("child config at index %d validation failed: %w", i, err)
		}
	}
	return nil
}
