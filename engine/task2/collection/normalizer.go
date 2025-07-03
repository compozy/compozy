package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for collection tasks
type Normalizer struct {
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
	converter      *TypeConverter
	filterEval     *FilterEvaluator
	configBuilder  *ConfigBuilder
}

// NewNormalizer creates a new collection task normalizer
func NewNormalizer(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	// Enable precision preservation in the template engine for collection tasks
	if templateEngine != nil {
		templateEngine.WithPrecisionPreservation(true)
	}

	return &Normalizer{
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
		converter:      NewTypeConverterWithPrecision(),
		filterEval:     NewFilterEvaluator(templateEngine),
		configBuilder:  NewConfigBuilder(templateEngine),
	}
}

// Type returns the task type this normalizer handles
func (n *Normalizer) Type() task.Type {
	return task.TaskTypeCollection
}

// Normalize applies collection task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Validate inputs
	if err := n.validateInputs(config, ctx); err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	// Type assert to get the concrete type
	normCtx, ok := ctx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", ctx)
	}
	// Normalize the config
	return n.normalizeConfig(config, normCtx)
}

// validateInputs validates the input parameters
func (n *Normalizer) validateInputs(config *task.Config, _ contracts.NormalizationContext) error {
	if config != nil && config.Type != task.TaskTypeCollection {
		return fmt.Errorf("collection normalizer cannot handle task type: %s", config.Type)
	}
	return nil
}

// normalizeConfig performs the actual normalization
func (n *Normalizer) normalizeConfig(config *task.Config, normCtx *shared.NormalizationContext) error {
	// Build template context
	context := normCtx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Store the task field before normalization to preserve it
	var childTaskConfig *task.Config
	if config.Task != nil {
		childTaskConfig = config.Task
	}
	// Normalize the collection task fields (excluding collection-specific fields)
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, n.shouldSkipField)
	if err != nil {
		return fmt.Errorf("failed to normalize collection task config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	// Restore the task field to ensure it's not modified during normalization
	// The task field contains templates with {{ .item }} references that
	// should only be processed at runtime when collection items are available
	if childTaskConfig != nil {
		config.Task = childTaskConfig
	}
	// Note: Collection-specific normalization (items expansion, filtering) happens at runtime
	// during task execution, not during config normalization phase
	return nil
}

// shouldSkipField determines if a field should be skipped during normalization
func (n *Normalizer) shouldSkipField(k string) bool {
	// Skip fields that need special handling
	return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
		k == "collection" || k == "items" || k == "filter" || k == "task"
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
	// First, process any templates in the items expression
	processedString, err := n.templateEngine.ParseAny(config.Items, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process items expression: %w", err)
	}

	// If the result is a string, try to parse it as JSON with precision handling
	if strValue, ok := processedString.(string); ok {
		var parsedValue any
		decoder := json.NewDecoder(strings.NewReader(strValue))
		decoder.UseNumber() // Preserve numeric precision
		if err := decoder.Decode(&parsedValue); err == nil {
			// Successfully parsed JSON, use the parsed value
			processedString = parsedValue
		}
		// If JSON parsing fails, keep the string as is (for range expressions like "1..3")
	}

	// Convert to a slice of items
	items := n.converter.ConvertToSlice(processedString)
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
