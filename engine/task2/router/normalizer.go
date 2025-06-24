package router

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for router tasks
type Normalizer struct {
	templateEngine shared.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewNormalizer creates a new router task normalizer
func NewNormalizer(
	templateEngine shared.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
	}
}

// Type returns the task type this normalizer handles
func (n *Normalizer) Type() task.Type {
	return task.TaskTypeRouter
}

// Normalize applies router task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	if config == nil {
		return nil
	}
	if config.Type != task.TaskTypeRouter {
		return fmt.Errorf("router normalizer cannot handle task type: %s", config.Type)
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Preserve existing With values before normalization
	existingWith := config.With
	// Apply template processing with appropriate filters
	// Router tasks process condition fields and route names
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == "outputs" || k == "output"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize router task config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	// Merge existing With values back into the normalized config
	if existingWith != nil && config.With != nil {
		// Check for aliasing to prevent concurrent map iteration and write panic
		if existingWith != config.With {
			// Create a new map with normalized values first, then overwrite with existing values
			mergedWith := make(core.Input)
			// Copy normalized values first
			maps.Copy(mergedWith, *config.With)
			// Then copy existing values (will overwrite for same keys)
			maps.Copy(mergedWith, *existingWith)
			config.With = &mergedWith
		}
	} else if existingWith != nil {
		// If normalization cleared With but we had existing values, restore them
		config.With = existingWith
	}
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
	// Process each route in the map
	for routeName, routeValue := range routes {
		// Routes can be either:
		// 1. Simple string (task ID)
		// 2. Map with condition and task_id
		switch v := routeValue.(type) {
		case string:
			// Simple string - process as task ID template
			processed, err := n.templateEngine.Process(v, context)
			if err != nil {
				return fmt.Errorf("failed to process route %s task ID: %w", routeName, err)
			}
			routes[routeName] = processed
		case map[string]any:
			// Complex route with condition
			processedRoute, err := n.templateEngine.ProcessMap(v, context)
			if err != nil {
				return fmt.Errorf("failed to process route %s: %w", routeName, err)
			}
			routes[routeName] = processedRoute
		default:
			// Leave other types as-is
		}
	}
	return nil
}
