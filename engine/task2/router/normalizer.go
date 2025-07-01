package router

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for router tasks
type Normalizer struct {
	*shared.BaseNormalizer
	templateEngine *tplengine.TemplateEngine
}

// NewNormalizer creates a new router task normalizer
func NewNormalizer(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			contextBuilder,
			task.TaskTypeRouter,
			nil, // Use default filter
		),
		templateEngine: templateEngine,
	}
}

// Normalize applies router task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	// Call base normalization first
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}
	// Build template context for router-specific fields
	context := ctx.BuildTemplateContext()
	// Normalize router-specific fields
	if config.Routes != nil {
		if err := n.normalizeRoutes(config.Routes, context); err != nil {
			return fmt.Errorf("failed to normalize routes: %w", err)
		}
	}
	return nil
}

// normalizeRoutes normalizes router routes configuration
func (n *Normalizer) normalizeRoutes(routes map[string]any, context map[string]any) error {
	if len(routes) == 0 {
		return nil
	}
	// Process each route in sorted order for deterministic processing
	err := shared.IterateSortedMap(routes, func(routeName string, routeValue any) error {
		// Routes can be either:
		// 1. Simple string (task ID)
		// 2. Map with condition and task_id
		switch v := routeValue.(type) {
		case string:
			// Simple string - process as task ID template
			processed, err := n.templateEngine.ParseAny(v, context)
			if err != nil {
				return fmt.Errorf("failed to process route %s task ID: %w", routeName, err)
			}
			routes[routeName] = processed
		case map[string]any:
			// Complex route with condition
			processedRoute, err := n.templateEngine.ParseAny(v, context)
			if err != nil {
				return fmt.Errorf("failed to process route %s: %w", routeName, err)
			}
			routes[routeName] = processedRoute
		default:
			// Leave other types as-is
		}
		return nil
	})
	return err
}
