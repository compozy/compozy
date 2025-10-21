package router

import (
	"context"
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
	_ context.Context,
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
func (n *Normalizer) Normalize(
	ctx context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	if err := n.BaseNormalizer.Normalize(ctx, config, parentCtx); err != nil {
		return err
	}
	normCtx, ok := parentCtx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", parentCtx)
	}
	context := normCtx.BuildTemplateContext()
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
	return shared.IterateSortedMap(routes, func(routeName string, routeValue any) error {
		normalizedValue, err := n.normalizeRouteValue(parentConfig, routeName, routeValue, context)
		if err != nil {
			return err
		}
		routes[routeName] = normalizedValue
		return nil
	})
}

// normalizeRouteValue normalizes an individual route entry.
func (n *Normalizer) normalizeRouteValue(
	parentConfig *task.Config,
	routeName string,
	routeValue any,
	context map[string]any,
) (any, error) {
	switch v := routeValue.(type) {
	case string:
		return n.processRouteString(routeName, v, context)
	case map[string]any:
		return n.processRouteMap(parentConfig, routeName, v, context)
	default:
		return routeValue, nil
	}
}

// processRouteString resolves templates for string-based routes.
func (n *Normalizer) processRouteString(routeName string, value string, context map[string]any) (any, error) {
	processed, err := n.templateEngine.ParseAny(value, context)
	if err != nil {
		return nil, fmt.Errorf("failed to process route %s task ID: %w", routeName, err)
	}
	return processed, nil
}

// processRouteMap normalizes map-based route definitions.
func (n *Normalizer) processRouteMap(
	parentConfig *task.Config,
	routeName string,
	value map[string]any,
	context map[string]any,
) (any, error) {
	if _, hasType := value["type"]; hasType {
		return n.processInlineTaskRoute(parentConfig, routeName, value, context)
	}
	processedRoute, err := n.templateEngine.ParseAny(value, context)
	if err != nil {
		return nil, fmt.Errorf("failed to process route %s: %w", routeName, err)
	}
	return processedRoute, nil
}

// processInlineTaskRoute inherits and normalizes inline task configurations.
func (n *Normalizer) processInlineTaskRoute(
	parentConfig *task.Config,
	routeName string,
	value map[string]any,
	context map[string]any,
) (any, error) {
	childConfig := &task.Config{}
	if err := childConfig.FromMap(value); err != nil {
		return nil, fmt.Errorf("invalid inline task config for route %q: %w", routeName, err)
	}
	if err := shared.InheritTaskConfig(childConfig, parentConfig); err != nil {
		return nil, fmt.Errorf("failed to inherit task config: %w", err)
	}
	updatedMap, err := childConfig.AsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert inherited config to map: %w", err)
	}
	processed, err := n.templateEngine.ParseAny(updatedMap, context)
	if err != nil {
		return nil, fmt.Errorf("failed to process route %s: %w", routeName, err)
	}
	return processed, nil
}
