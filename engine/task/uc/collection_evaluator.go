package uc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/tplengine"
)

const (
	// Use constants from task package
	DefaultItemVar  = task.DefaultItemVariable
	DefaultIndexVar = task.DefaultIndexVariable
)

// Pool for reusing evaluation context maps to reduce allocations in hot paths
var contextMapPool = sync.Pool{
	New: func() any {
		return make(map[string]any)
	},
}

// getContextMap gets a clean map from the pool
func getContextMap() map[string]any {
	if v, ok := contextMapPool.Get().(map[string]any); ok {
		return v
	}
	// Fallback: create new map if type assertion fails (should never happen)
	return make(map[string]any)
}

// putContextMap clears and returns a map to the pool
func putContextMap(m map[string]any) {
	// Clear the map before returning to pool
	for k := range m {
		delete(m, k)
	}
	contextMapPool.Put(m)
}

// copyContextToMap efficiently copies context to a pooled map
func copyContextToMap(dst map[string]any, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

type CollectionEvaluator struct {
	engine *tplengine.TemplateEngine
}

func NewCollectionEvaluator() *CollectionEvaluator {
	return &CollectionEvaluator{
		engine: tplengine.NewEngine(tplengine.FormatJSON),
	}
}

type EvaluationInput struct {
	ItemsExpr  string
	FilterExpr string
	Context    map[string]any
	ItemVar    string
	IndexVar   string
}

type EvaluationResult struct {
	Items         []any
	TotalCount    int
	FilteredCount int
}

func (ce *CollectionEvaluator) EvaluateItems(_ context.Context, input *EvaluationInput) (*EvaluationResult, error) {
	if input.ItemsExpr == "" {
		return nil, fmt.Errorf("items expression is required")
	}

	if input.ItemVar == "" {
		input.ItemVar = DefaultItemVar
	}
	if input.IndexVar == "" {
		input.IndexVar = DefaultIndexVar
	}

	// Sanitize input
	if err := ce.validateInput(input); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	var items []any
	var err error

	if ce.isDynamicExpression(input.ItemsExpr) {
		items, err = ce.evaluateDynamicItems(input.ItemsExpr, input.Context)
	} else {
		items, err = ce.evaluateStaticItems(input.ItemsExpr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to evaluate items: %w", err)
	}

	totalCount := len(items)

	// Validate collection size for security
	if totalCount > task.DefaultMaxCollectionItems {
		return nil, fmt.Errorf(
			"collection size %d exceeds maximum allowed %d items",
			totalCount,
			task.DefaultMaxCollectionItems,
		)
	}

	// Apply filter if provided
	if input.FilterExpr != "" {
		items, err = ce.applyFilter(items, input.FilterExpr, input.ItemVar, input.IndexVar, input.Context)
		if err != nil {
			return nil, fmt.Errorf("failed to apply filter: %w", err)
		}
	}

	return &EvaluationResult{
		Items:         items,
		TotalCount:    totalCount,
		FilteredCount: len(items),
	}, nil
}

func (ce *CollectionEvaluator) validateInput(input *EvaluationInput) error {
	// Check for potential template injection
	if strings.Contains(input.ItemsExpr, "{{") && strings.Contains(input.ItemsExpr, "}}") {
		if strings.Contains(input.ItemsExpr, "exec") || strings.Contains(input.ItemsExpr, "system") {
			return fmt.Errorf("potential security risk detected in items expression")
		}
	}

	if input.FilterExpr != "" {
		if strings.Contains(input.FilterExpr, "exec") || strings.Contains(input.FilterExpr, "system") {
			return fmt.Errorf("potential security risk detected in filter expression")
		}
	}

	// Validate variable names
	if !isValidVariableName(input.ItemVar) {
		return fmt.Errorf("invalid item variable name: %s", input.ItemVar)
	}
	if !isValidVariableName(input.IndexVar) {
		return fmt.Errorf("invalid index variable name: %s", input.IndexVar)
	}

	return nil
}

func (ce *CollectionEvaluator) isDynamicExpression(expr string) bool {
	return strings.Contains(expr, "{{") && strings.Contains(expr, "}}")
}

func (ce *CollectionEvaluator) evaluateDynamicItems(itemsExpr string, context map[string]any) ([]any, error) {
	result, err := ce.engine.ParseMap(itemsExpr, context)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate dynamic expression: %w", err)
	}

	return ce.convertToSlice(result)
}

func (ce *CollectionEvaluator) evaluateStaticItems(itemsExpr string) ([]any, error) {
	// For static expressions, try to parse as a simple list
	if strings.HasPrefix(itemsExpr, "[") && strings.HasSuffix(itemsExpr, "]") {
		// Simple array notation - delegate to template engine
		result, err := ce.engine.ParseMap(itemsExpr, map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("failed to parse static array: %w", err)
		}
		return ce.convertToSlice(result)
	}

	// Single item
	return []any{itemsExpr}, nil
}

func (ce *CollectionEvaluator) convertToSlice(result any) ([]any, error) {
	if result == nil {
		return []any{}, nil
	}

	// Handle slice types
	v := reflect.ValueOf(result)
	if v.Kind() == reflect.Slice {
		items := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			items[i] = v.Index(i).Interface()
		}
		return items, nil
	}

	// Handle array types
	if v.Kind() == reflect.Array {
		items := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			items[i] = v.Index(i).Interface()
		}
		return items, nil
	}

	// Single item
	return []any{result}, nil
}

func (ce *CollectionEvaluator) applyFilter(
	items []any,
	filterExpr, itemVar, indexVar string,
	context map[string]any,
) ([]any, error) {
	var filteredItems []any

	for i, item := range items {
		// Get a clean map from the pool
		itemContext := getContextMap()

		// Copy base context and add item-specific variables
		copyContextToMap(itemContext, context)
		itemContext[itemVar] = item
		itemContext[indexVar] = i

		// Evaluate filter expression
		result, err := ce.engine.ParseMap(filterExpr, itemContext)

		// Return map to pool before handling error/continuing
		putContextMap(itemContext)

		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter for item %d: %w", i, err)
		}

		// Check if result is truthy
		if ce.isTruthy(result) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

func (ce *CollectionEvaluator) isTruthy(value any) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0
	default:
		// For complex types, check if they're non-nil
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
			return rv.Len() > 0
		case reflect.Ptr, reflect.Interface:
			return !rv.IsNil()
		default:
			return true
		}
	}
}

func isValidVariableName(name string) bool {
	if name == "" {
		return false
	}

	// Check if it starts with a letter or underscore
	first := rune(name[0])
	if (first < 'a' || first > 'z') && (first < 'A' || first > 'Z') && first != '_' {
		return false
	}

	// Check remaining characters
	for _, r := range name[1:] {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}

	return true
}

type TaskTemplateEvaluator struct {
	engine *tplengine.TemplateEngine
}

func NewTaskTemplateEvaluator() *TaskTemplateEvaluator {
	return &TaskTemplateEvaluator{
		engine: tplengine.NewEngine(tplengine.FormatJSON),
	}
}

func (tte *TaskTemplateEvaluator) EvaluateTaskTemplate(
	template *task.Config,
	item any,
	index int,
	itemVar, indexVar string,
	context map[string]any,
) (*task.Config, error) {
	// Get a clean map from the pool for evaluation context
	evalContext := getContextMap()
	defer putContextMap(evalContext)

	// Copy base context and add item-specific variables
	copyContextToMap(evalContext, context)
	evalContext[itemVar] = item
	evalContext[indexVar] = index

	// Convert template to map for processing
	templateMap, err := template.AsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert template to map: %w", err)
	}

	// Process template with evaluation context, excluding only the outputs field
	// The outputs field should not be processed during template evaluation since the task hasn't executed
	// Collection items should have full access to workflow state (tasks, workflow context, etc.)
	parsed, err := tte.engine.ParseMapWithFilter(templateMap, evalContext, func(key string) bool {
		// Exclude outputs field - will be processed after execution
		return key == "outputs"
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process task template: %w", err)
	}

	// Create new task config from parsed map
	result := &task.Config{}
	if err := result.FromMap(parsed); err != nil {
		return nil, fmt.Errorf("failed to create task config from template: %w", err)
	}

	return result, nil
}
