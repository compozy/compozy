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
	// Build template context for collection processing
	templateContext := e.contextBuilder.BuildCollectionContext(workflowState, workflowConfig, config)
	// Process collection items through expansion and filtering pipeline
	filteredItems, skippedCount, err := e.processCollectionItems(ctx, config, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process collection items: %w", err)
	}
	// Handle empty collection case
	if len(filteredItems) == 0 {
		return &shared.ExpansionResult{
			ChildConfigs: []*task.Config{},
			ItemCount:    0,
			SkippedCount: skippedCount,
		}, nil
	}
	// Create child configurations with collection context injection
	childConfigs, err := e.createChildConfigs(config, filteredItems, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create child configs: %w", err)
	}
	// Validate all child configurations
	if err := e.validateChildConfigs(childConfigs); err != nil {
		return nil, fmt.Errorf("child config validation failed: %w", err)
	}
	return &shared.ExpansionResult{
		ChildConfigs: childConfigs,
		ItemCount:    len(filteredItems),
		SkippedCount: skippedCount,
	}, nil
}

// ValidateExpansion validates the expansion result
func (e *Expander) ValidateExpansion(result *shared.ExpansionResult) error {
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
		if err := childConfig.Validate(); err != nil {
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
	// Stage 1: Expand collection items from template expressions
	items, err := e.normalizer.ExpandCollectionItems(ctx, &config.CollectionConfig, templateContext)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to expand collection items: %w", err)
	}
	// Stage 2: Filter items based on filter expressions
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
		// Create item-specific context for template processing
		itemContext := e.normalizer.CreateItemContext(templateContext, &config.CollectionConfig, item, i)
		// Build base child config from template with item context
		childConfig, err := e.configBuilder.BuildTaskConfig(&config.CollectionConfig, config, item, i, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to build child config at index %d: %w", i, err)
		}
		// Inject collection context metadata into child config
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
	// Ensure With field exists
	if childConfig.With == nil {
		childConfig.With = &core.Input{}
	}
	withMap := map[string]any(*childConfig.With)
	// Standard collection variables
	withMap[shared.FieldCollectionItem] = item
	withMap[shared.FieldCollectionIndex] = index
	// Custom variable naming support
	if itemVar := parentConfig.GetItemVar(); itemVar != "" {
		withMap[itemVar] = item
	}
	if indexVar := parentConfig.GetIndexVar(); indexVar != "" {
		withMap[indexVar] = index
	}
	*childConfig.With = core.Input(withMap)
}

// validateChildConfigs validates all child configurations
func (e *Expander) validateChildConfigs(childConfigs []*task.Config) error {
	for i, childConfig := range childConfigs {
		if childConfig.ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
		if err := childConfig.Validate(); err != nil {
			return fmt.Errorf("child config at index %d validation failed: %w", i, err)
		}
	}
	return nil
}
