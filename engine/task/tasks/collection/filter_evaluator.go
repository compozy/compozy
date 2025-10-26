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
		return true, nil
	}
	processed, err := fe.templateEngine.ParseAny(filterExpr, context)
	if err != nil {
		return false, fmt.Errorf("failed to process filter expression: %w", err)
	}
	var result string
	if processed != nil {
		result = fmt.Sprintf("%v", processed)
	}
	result = strings.TrimSpace(result)
	switch strings.ToLower(result) {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off", "":
		return false, nil
	default:
		return result != "", nil
	}
}
