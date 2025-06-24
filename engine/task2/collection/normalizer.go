package collection

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for collection tasks
type Normalizer struct {
	templateEngine shared.TemplateEngine
	contextBuilder *shared.ContextBuilder
	converter      *TypeConverter
	filterEval     *FilterEvaluator
	configBuilder  *ConfigBuilder
}

// NewNormalizer creates a new collection task normalizer
func NewNormalizer(
	templateEngine shared.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
		converter:      NewTypeConverter(),
		filterEval:     NewFilterEvaluator(templateEngine),
		configBuilder:  NewConfigBuilder(templateEngine),
	}
}

// Type returns the task type this normalizer handles
func (n *Normalizer) Type() task.Type {
	return task.TaskTypeCollection
}

// Normalize applies collection task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	if config == nil {
		return nil
	}
	if config.Type != task.TaskTypeCollection {
		return fmt.Errorf("collection normalizer cannot handle task type: %s", config.Type)
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Normalize the collection task fields (excluding collection-specific fields)
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		// Skip fields that need special handling
		return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
			k == "collection" || k == "items" || k == "filter" || k == "task"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize collection task config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	// Note: Collection-specific normalization (items expansion, filtering) happens at runtime
	// during task execution, not during config normalization phase
	return nil
}

// ExpandCollectionItems evaluates the 'items' template expression and converts the result
// into a slice of items that can be iterated over
func (n *Normalizer) ExpandCollectionItems(
	_ context.Context,
	config *task.CollectionConfig,
	templateContext map[string]any,
) ([]any, error) {
	if config.Items == "" {
		return nil, fmt.Errorf("collection config: items field is required")
	}
	// For simple template expressions, process the template string first
	if tplengine.HasTemplate(config.Items) {
		processed, err := n.templateEngine.Process(config.Items, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate items expression: %w", err)
		}
		// Parse the processed result to get the actual value
		result, err := n.templateEngine.ProcessString(processed, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to parse items result: %w", err)
		}
		// Use the JSON result if available
		var itemsValue any
		if result.JSON != nil {
			itemsValue = result.JSON
		} else {
			itemsValue = result.Text
		}
		// Convert to a slice of items
		items := n.converter.ConvertToSlice(itemsValue)
		return items, nil
	}
	// For static JSON arrays/objects, use ProcessString to parse the JSON
	result, err := n.templateEngine.ProcessString(config.Items, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process items expression: %w", err)
	}
	// Use the JSON result if available, otherwise fall back to text
	var itemsValue any
	if result.JSON != nil {
		itemsValue = result.JSON
	} else {
		itemsValue = result.Text
	}
	// Convert to a slice of items
	items := n.converter.ConvertToSlice(itemsValue)
	return items, nil
}

// FilterCollectionItems filters items based on the filter expression
func (n *Normalizer) FilterCollectionItems(
	_ context.Context,
	config *task.CollectionConfig,
	items []any,
	templateContext map[string]any,
) ([]any, error) {
	if config.Filter == "" {
		// No filter, return all items
		return items, nil
	}
	var filteredItems []any
	for i, item := range items {
		// Create context with item and index variables
		filterContext := n.createItemContext(templateContext, config, item, i)
		// Evaluate filter expression
		include, err := n.filterEval.EvaluateFilter(config.Filter, filterContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
		}
		if include {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems, nil
}

// CreateItemContext creates a context for a collection item
func (n *Normalizer) CreateItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	return n.createItemContext(baseContext, config, item, index)
}

// createItemContext creates a context for a collection item (internal helper)
func (n *Normalizer) createItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	// Clone base context
	itemContext := make(map[string]any)
	for k, v := range baseContext {
		itemContext[k] = v
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

// BuildCollectionContext builds context specifically for collection tasks
func (n *Normalizer) BuildCollectionContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	// Use the context builder's collection context method
	return n.contextBuilder.BuildCollectionContext(
		workflowState,
		workflowConfig,
		taskConfig,
	)
}
