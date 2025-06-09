package normalizer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/tplengine"
)

// CollectionNormalizer handles template evaluation and parsing for collection tasks
type CollectionNormalizer struct {
	engine     *tplengine.TemplateEngine
	textEngine *tplengine.TemplateEngine
}

// NewCollectionNormalizer creates a new collection normalizer
func NewCollectionNormalizer() *CollectionNormalizer {
	return &CollectionNormalizer{
		engine:     tplengine.NewEngine(tplengine.FormatJSON),
		textEngine: tplengine.NewEngine(tplengine.FormatText),
	}
}

// ExpandCollectionItems evaluates the 'items' template expression and converts the result
// into a slice of items that can be iterated over
func (cn *CollectionNormalizer) ExpandCollectionItems(
	_ context.Context,
	config *task.CollectionConfig,
	templateContext map[string]any,
) ([]any, error) {
	if config.Items == "" {
		return nil, fmt.Errorf("collection config: items field is required")
	}

	// For simple template expressions, use ParseMap directly
	if tplengine.HasTemplate(config.Items) {
		itemsValue, err := cn.engine.ParseMap(config.Items, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate items expression: %w", err)
		}

		// Convert to a slice of items
		items := cn.convertToSlice(itemsValue)
		return items, nil
	}

	// For static JSON arrays/objects, use ProcessString to parse the JSON
	result, err := cn.engine.ProcessString(config.Items, templateContext)
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
	items := cn.convertToSlice(itemsValue)
	return items, nil
}

// FilterCollectionItems filters items based on the filter expression
func (cn *CollectionNormalizer) FilterCollectionItems(
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
		filterContext := cn.CreateItemContext(templateContext, config, item, i)

		// Evaluate filter expression using RenderString to properly handle template functions
		filterResult, err := cn.textEngine.RenderString(config.Filter, filterContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
		}

		// Check if result is truthy
		if cn.isTruthy(filterResult) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

// CreateItemContext creates a template context for a specific collection item
func (cn *CollectionNormalizer) CreateItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	itemContext := make(map[string]any)

	// Copy base context
	for k, v := range baseContext {
		itemContext[k] = v
	}

	// Add item-specific variables
	itemContext[config.GetItemVar()] = item
	itemContext[config.GetIndexVar()] = index

	return itemContext
}

// CreateProgressContext creates a template context enriched with progress information
func (cn *CollectionNormalizer) CreateProgressContext(
	baseContext map[string]any,
	progressInfo *task.ProgressInfo,
) map[string]any {
	contextWithProgress := make(map[string]any)

	// Copy base context
	for k, v := range baseContext {
		contextWithProgress[k] = v
	}

	// Add progress info
	contextWithProgress["progress"] = map[string]any{
		"total_children":  progressInfo.TotalChildren,
		"completed_count": progressInfo.CompletedCount,
		"failed_count":    progressInfo.FailedCount,
		"running_count":   progressInfo.RunningCount,
		"pending_count":   progressInfo.PendingCount,
		"completion_rate": progressInfo.CompletionRate,
		"failure_rate":    progressInfo.FailureRate,
		"overall_status":  string(progressInfo.OverallStatus),
		"status_counts":   progressInfo.StatusCounts,
		"has_failures":    progressInfo.HasFailures(),
		"is_all_complete": progressInfo.IsAllComplete(),
	}

	// Add summary alias for backward compatibility
	contextWithProgress["summary"] = contextWithProgress["progress"]

	return contextWithProgress
}

// ApplyTemplateToConfig applies item-specific context to a task configuration and returns a new config
func (cn *CollectionNormalizer) ApplyTemplateToConfig(
	config *task.Config,
	itemContext map[string]any,
) (*task.Config, error) {
	// Create a deep copy to avoid mutating the original config
	newConfig := cn.deepCopyConfig(config)

	// Use the template engine to process the configuration
	engine := tplengine.NewEngine(tplengine.FormatText)

	// Apply template to action field
	if newConfig.Action != "" {
		processedAction, err := engine.RenderString(newConfig.Action, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to apply template to action: %w", err)
		}
		newConfig.Action = processedAction
	}

	// Apply templates to the 'with' input parameters
	if newConfig.With != nil {
		processedWith := make(map[string]any)
		for k, v := range *newConfig.With {
			if strVal, ok := v.(string); ok {
				// Apply template to string values
				renderedVal, err := engine.RenderString(strVal, itemContext)
				if err != nil {
					return nil, fmt.Errorf("failed to apply template to with parameter '%s': %w", k, err)
				}
				processedWith[k] = renderedVal
			} else {
				// For non-string values, use ParseMap to handle nested structures
				processedVal, err := engine.ParseMap(v, itemContext)
				if err != nil {
					return nil, fmt.Errorf("failed to apply template to with parameter '%s': %w", k, err)
				}
				processedWith[k] = processedVal
			}
		}
		*newConfig.With = processedWith
	}

	// Apply templates to environment variables
	if newConfig.Env != nil {
		processedEnv, err := engine.ParseMap(*newConfig.Env, itemContext)
		if err != nil {
			return nil, fmt.Errorf("failed to apply template to env variables: %w", err)
		}
		if envMap, ok := processedEnv.(map[string]any); ok {
			envStrMap := make(map[string]string)
			for k, v := range envMap {
				if strVal, ok := v.(string); ok {
					envStrMap[k] = strVal
				} else {
					envStrMap[k] = fmt.Sprintf("%v", v)
				}
			}
			envMapPtr := core.EnvMap(envStrMap)
			newConfig.Env = &envMapPtr
		}
	}

	return newConfig, nil
}

// deepCopyConfig creates a deep copy of a task configuration
func (cn *CollectionNormalizer) deepCopyConfig(config *task.Config) *task.Config {
	newConfig := *config // shallow copy

	// Deep copy With map if it exists
	if config.With != nil {
		withCopy := make(core.Input)
		for k, v := range *config.With {
			withCopy[k] = v
		}
		newConfig.With = &withCopy
	}

	// Deep copy Env map if it exists
	if config.Env != nil {
		envCopy := make(core.EnvMap)
		for k, v := range *config.Env {
			envCopy[k] = v
		}
		newConfig.Env = &envCopy
	}

	return &newConfig
}

// convertToSlice converts various types to a slice of interfaces
func (cn *CollectionNormalizer) convertToSlice(value any) []any {
	if value == nil {
		return []any{}
	}

	switch v := value.(type) {
	case []any:
		return v
	case []string:
		return cn.convertStringSlice(v)
	case []int:
		return cn.convertIntSlice(v)
	case []float64:
		return cn.convertFloatSlice(v)
	case map[string]any:
		return cn.convertMapToSlice(v)
	case string, int, int32, int64, float32, float64, bool:
		return []any{v}
	default:
		return cn.convertReflectionSlice(value)
	}
}

// convertStringSlice converts []string to []any
func (cn *CollectionNormalizer) convertStringSlice(v []string) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertIntSlice converts []int to []any
func (cn *CollectionNormalizer) convertIntSlice(v []int) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertFloatSlice converts []float64 to []any
func (cn *CollectionNormalizer) convertFloatSlice(v []float64) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertMapToSlice converts map to slice of key-value pairs
func (cn *CollectionNormalizer) convertMapToSlice(v map[string]any) []any {
	result := make([]any, 0, len(v))
	for key, val := range v {
		result = append(result, map[string]any{
			"key":   key,
			"value": val,
		})
	}
	return result
}

// convertReflectionSlice handles any slice/array type using reflection
func (cn *CollectionNormalizer) convertReflectionSlice(value any) []any {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		result := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = rv.Index(i).Interface()
		}
		return result
	}
	// If it's not a slice/array/map, treat as single item
	return []any{value}
}

// isTruthy checks if a value is considered truthy for filtering
func (cn *CollectionNormalizer) isTruthy(value any) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return cn.isStringTruthy(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v != 0
	case float32, float64:
		return v != 0.0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return cn.isDefaultTruthy(v)
	}
}

// isStringTruthy checks if a string value is truthy
func (cn *CollectionNormalizer) isStringTruthy(v string) bool {
	if v == "true" || v == `"true"` {
		return true
	}
	if v == "false" || v == `"false"` || v == "" {
		return false
	}
	// Any other non-empty string is truthy
	return v != ""
}

// isDefaultTruthy handles default case for truthy evaluation
func (cn *CollectionNormalizer) isDefaultTruthy(v any) bool {
	str := fmt.Sprintf("%v", v)
	if str == "true" || str == `"true"` {
		return true
	}
	if str == "false" || str == `"false"` || str == "0" || str == "" {
		return false
	}
	return true
}
