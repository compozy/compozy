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
	_ context.Context,
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	var filterEval *FilterEvaluator
	var configBuilder *ConfigBuilder
	if templateEngine != nil {
		filterEval = NewFilterEvaluator(templateEngine)
		configBuilder = NewConfigBuilder(templateEngine)
	}
	return &Normalizer{
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
		converter:      NewTypeConverterWithPrecision(),
		filterEval:     filterEval,
		configBuilder:  configBuilder,
	}
}

// Type returns the task type this normalizer handles
func (n *Normalizer) Type() task.Type {
	return task.TaskTypeCollection
}

// Normalize applies collection task-specific normalization rules
func (n *Normalizer) Normalize(
	_ context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	if err := n.validateInputs(config, parentCtx); err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	normCtx, ok := parentCtx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", parentCtx)
	}
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
	context := normCtx.BuildTemplateContext()
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	var childTaskConfig *task.Config
	if config.Task != nil {
		childTaskConfig = config.Task
	}
	if n.templateEngine == nil {
		return fmt.Errorf("template engine is required for normalization")
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, n.shouldSkipField)
	if err != nil {
		return fmt.Errorf("failed to normalize collection task config: %w", err)
	}
	parsed = n.applyPrecisionConversion(parsed)
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	if childTaskConfig != nil {
		config.Task = childTaskConfig
		if err := shared.InheritTaskConfig(config.Task, config); err != nil {
			return fmt.Errorf("failed to inherit task config: %w", err)
		}
	}
	// Note: Collection-specific normalization (items expansion, filtering) happens at runtime
	return nil
}

// shouldSkipField determines if a field should be skipped during normalization
func (n *Normalizer) shouldSkipField(k string) bool {
	return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
		k == "collection" || k == "items" || k == "filter" || k == "task" || k == "tasks"
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
	processedString, err := n.templateEngine.ParseAny(config.Items, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process items expression: %w", err)
	}
	if strValue, ok := processedString.(string); ok {
		var parsedValue any
		decoder := json.NewDecoder(strings.NewReader(strValue))
		decoder.UseNumber() // Preserve numeric precision
		if err := decoder.Decode(&parsedValue); err == nil {
			processedString = parsedValue
		}
	}
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
		return items, nil
	}
	var filteredItems []any
	for i, item := range items {
		filterContext := n.createItemContext(templateContext, config, item, i)
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

// BuildCollectionContext builds context specifically for collection tasks
func (n *Normalizer) BuildCollectionContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	return n.contextBuilder.BuildCollectionContext(
		ctx,
		workflowState,
		workflowConfig,
		taskConfig,
	)
}

// applyPrecisionConversion recursively applies precision conversion to numeric values
func (n *Normalizer) applyPrecisionConversion(value any) any {
	pc := tplengine.NewPrecisionConverter()
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = n.applyPrecisionConversion(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = n.applyPrecisionConversion(val)
		}
		return result
	case string:
		return pc.ConvertWithPrecision(v)
	default:
		return v
	}
}
