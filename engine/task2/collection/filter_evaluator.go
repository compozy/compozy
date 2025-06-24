package collection

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/task2/shared"
)

// FilterEvaluator handles filter expression evaluation
type FilterEvaluator struct {
	templateEngine shared.TemplateEngine
}

// NewFilterEvaluator creates a new filter evaluator
func NewFilterEvaluator(templateEngine shared.TemplateEngine) *FilterEvaluator {
	return &FilterEvaluator{
		templateEngine: templateEngine,
	}
}

// EvaluateFilter evaluates a filter expression and returns whether the item should be included
func (fe *FilterEvaluator) EvaluateFilter(filterExpr string, context map[string]any) (bool, error) {
	if filterExpr == "" {
		// No filter means include all items
		return true, nil
	}
	// Process the filter expression as a template
	processed, err := fe.templateEngine.Process(filterExpr, context)
	if err != nil {
		return false, fmt.Errorf("failed to process filter expression: %w", err)
	}
	// Trim whitespace
	result := strings.TrimSpace(processed)
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
