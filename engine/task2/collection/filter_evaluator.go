package collection

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/pkg/tplengine"
)

// FilterEvaluator handles filter expression evaluation
type FilterEvaluator struct {
	templateEngine *tplengine.TemplateEngine
}

// NewFilterEvaluator creates a new filter evaluator
func NewFilterEvaluator(templateEngine *tplengine.TemplateEngine) *FilterEvaluator {
	if templateEngine == nil {
		panic("templateEngine cannot be nil")
	}
	return &FilterEvaluator{
		templateEngine: templateEngine,
	}
}

// EvaluateFilter evaluates a filter expression and returns whether the item should be included
func (fe *FilterEvaluator) EvaluateFilter(filterExpr string, context map[string]any) (bool, error) {
	if context == nil {
		context = make(map[string]any)
	}
	if filterExpr == "" {
		// No filter means include all items
		return true, nil
	}
	// Process the filter expression as a template
	processed, err := fe.templateEngine.ParseAny(filterExpr, context)
	if err != nil {
		return false, fmt.Errorf("failed to process filter expression: %w", err)
	}
	// Convert result to string for evaluation
	var result string
	if processed != nil {
		result = fmt.Sprintf("%v", processed)
	}
	// Trim whitespace
	result = strings.TrimSpace(result)
	// Check for boolean-like values
	switch strings.ToLower(result) {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off", "":
		return false, nil
	default:
		// If the result is not a clear boolean, treat non-empty as true
		// This allows expressions like {{ if .item.active }}active{{ end }}
		return result != "", nil
	}
}
