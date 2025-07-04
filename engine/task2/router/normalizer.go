package router

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
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
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Call base normalization first
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}
	// Type assert to get the concrete type for router-specific logic
	normCtx, ok := ctx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", ctx)
	}
	// Build template context for router-specific fields
	context := normCtx.BuildTemplateContext()
	// Normalize router-specific fields
	if config.Routes != nil {
		if err := n.normalizeRoutes(config, config.Routes, context); err != nil {
			return fmt.Errorf("failed to normalize routes: %w", err)
		}
	}
	return nil
}

// normalizeRoutes normalizes router routes configuration
func (n *Normalizer) normalizeRoutes(parentConfig *task.Config, routes map[string]any, context map[string]any) error {
	if len(routes) == 0 {
		return nil
	}
	// Process each route in sorted order for deterministic processing
	err := shared.IterateSortedMap(routes, func(routeName string, routeValue any) error {
		// Routes can be either:
		// 1. Simple string (task ID)
		// 2. Map with condition and task_id
		// 3. Inline task configuration (map with type field)
		switch v := routeValue.(type) {
		case string:
			// Simple string - process as task ID template
			processed, err := n.templateEngine.ParseAny(v, context)
			if err != nil {
				return fmt.Errorf("failed to process route %s task ID: %w", routeName, err)
			}
			routes[routeName] = processed
		case map[string]any:
			// Check if this is an inline task configuration (has "type" field)
			if _, hasType := v["type"]; hasType {
				// This is an inline task config - convert to Config for inheritance
				childConfig := &task.Config{}
				if err := childConfig.FromMap(v); err == nil {
					// Apply inheritance from router task to inline task config
					shared.InheritTaskConfig(childConfig, parentConfig)
					// Convert back to map after inheritance
					updatedMap, err := childConfig.AsMap()
					if err != nil {
						return fmt.Errorf("failed to convert inherited config to map: %w", err)
					}
					// Process templates in the inherited config
					processed, err := n.templateEngine.ParseAny(updatedMap, context)
					if err != nil {
						return fmt.Errorf("failed to process route %s: %w", routeName, err)
					}
					routes[routeName] = processed
				} else {
					// Failed to parse as task config, process as regular map
					processedRoute, err := n.templateEngine.ParseAny(v, context)
					if err != nil {
						return fmt.Errorf("failed to process route %s: %w", routeName, err)
					}
					routes[routeName] = processedRoute
				}
			} else {
				// Regular map (condition/task_id structure) - just process templates
				processedRoute, err := n.templateEngine.ParseAny(v, context)
				if err != nil {
					return fmt.Errorf("failed to process route %s: %w", routeName, err)
				}
				routes[routeName] = processedRoute
			}
		default:
			// Leave other types as-is
		}
		return nil
	})
	return err
}
