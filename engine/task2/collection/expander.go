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

		// NEW: ensure each child owns an independent With map (prevents pointer sharing)
		if childConfig.With != nil {
			cloned := core.Input(e.deepCopyMap(map[string]any(*childConfig.With)))
			childConfig.With = &cloned
		} else {
			empty := core.Input{}
			childConfig.With = &empty
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

	// Deep copy existing child context to avoid shared pointer mutations
	withMap := e.deepCopyMap(map[string]any(*childConfig.With))

	// Always publish canonical vars
	withMap[shared.FieldCollectionItem] = item
	withMap[shared.FieldCollectionIndex] = index

	// Custom variable naming support (avoid duplicates)
	if parentConfig != nil {
		if iv := parentConfig.GetItemVar(); iv != "" && iv != shared.FieldCollectionItem {
			withMap[iv] = item
		}
		if ix := parentConfig.GetIndexVar(); ix != "" && ix != shared.FieldCollectionIndex {
			withMap[ix] = index
		}
	}

	// Merge inherited parent With after deep-copy to preserve precedence rules
	if parentConfig != nil && parentConfig.With != nil {
		parentMap := e.deepCopyMap(map[string]any(*parentConfig.With))
		for k, v := range parentMap {
			if _, exists := withMap[k]; !exists {
				withMap[k] = v
			}
		}
	}

	newWith := core.Input(withMap)
	childConfig.With = &newWith
}

// deepCopyMap creates a deep copy of a map to avoid shared pointer mutations
func (e *Expander) deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			// Recursively deep copy nested maps
			dst[k] = e.deepCopyMap(val)
		case *map[string]any:
			// Dereference and deep copy pointer to map
			if val != nil {
				dst[k] = e.deepCopyMap(*val)
			} else {
				dst[k] = nil
			}
		case core.Input:
			// Deep copy core.Input (alias for map[string]any)
			dst[k] = e.deepCopyMap(map[string]any(val))
		case *core.Input:
			// Dereference and deep copy pointer to core.Input
			if val != nil {
				dst[k] = e.deepCopyMap(map[string]any(*val))
			} else {
				dst[k] = nil
			}
		case core.Output:
			// Deep copy core.Output (alias for map[string]any)
			dst[k] = e.deepCopyMap(map[string]any(val))
		case *core.Output:
			// Dereference and deep copy pointer to core.Output
			if val != nil {
				dst[k] = e.deepCopyMap(map[string]any(*val))
			} else {
				dst[k] = nil
			}
		case []any:
			// Deep copy slices
			dst[k] = e.deepCopySlice(val)
		case []map[string]any:
			// Deep copy slice of maps
			newSlice := make([]map[string]any, len(val))
			for i, m := range val {
				newSlice[i] = e.deepCopyMap(m)
			}
			dst[k] = newSlice
		default:
			// For primitive types and other values, assign directly
			// This includes string, int, float, bool, etc.
			dst[k] = val
		}
	}
	return dst
}

// deepCopySlice creates a deep copy of a slice
func (e *Expander) deepCopySlice(src []any) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[i] = e.deepCopyMap(val)
		case *map[string]any:
			if val != nil {
				dst[i] = e.deepCopyMap(*val)
			} else {
				dst[i] = nil
			}
		case []any:
			dst[i] = e.deepCopySlice(val)
		default:
			dst[i] = val
		}
	}
	return dst
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
