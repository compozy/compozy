package normalizer

import (
	"fmt"

	"github.com/compozy/compozy/pkg/tplengine"
)

// FilterEvaluator handles filtering logic for collection items
type FilterEvaluator struct {
	textEngine *tplengine.TemplateEngine
}

// NewFilterEvaluator creates a new filter evaluator
func NewFilterEvaluator() *FilterEvaluator {
	return &FilterEvaluator{
		textEngine: tplengine.NewEngine(tplengine.FormatText),
	}
}

// IsTruthy checks if a value is considered truthy for filtering
func (fe *FilterEvaluator) IsTruthy(value any) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return fe.isStringTruthy(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v != 0
	case float32, float64:
		return v != 0.0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return fe.isDefaultTruthy(v)
	}
}

// isStringTruthy checks if a string value is truthy
func (fe *FilterEvaluator) isStringTruthy(v string) bool {
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
func (fe *FilterEvaluator) isDefaultTruthy(v any) bool {
	str := fmt.Sprintf("%v", v)
	if str == "true" || str == `"true"` {
		return true
	}
	if str == "false" || str == `"false"` || str == "0" || str == "" {
		return false
	}
	return true
}

// EvaluateFilter evaluates a filter expression against an item context
func (fe *FilterEvaluator) EvaluateFilter(
	filterExpression string,
	itemContext map[string]any,
) (bool, error) {
	if filterExpression == "" {
		return true, nil
	}

	// Evaluate filter expression using RenderString to properly handle template functions
	filterResult, err := fe.textEngine.RenderString(filterExpression, itemContext)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate filter expression: %w", err)
	}

	// Check if result is truthy
	return fe.IsTruthy(filterResult), nil
}
