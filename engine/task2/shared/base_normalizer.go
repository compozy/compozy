package shared

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/pkg/tplengine"
)

// BaseNormalizer provides common normalization functionality for all task types
type BaseNormalizer struct {
	templateEngine *tplengine.TemplateEngine
	contextBuilder *ContextBuilder
	taskType       task.Type
	filterFunc     func(string) bool
}

// NewBaseNormalizer creates a new base normalizer
func NewBaseNormalizer(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *ContextBuilder,
	taskType task.Type,
	filterFunc func(string) bool,
) *BaseNormalizer {
	if filterFunc == nil {
		// Default filter for most task types
		// Filter returns true for fields that should NOT be template processed
		filterFunc = func(k string) bool {
			return k == "agent" || k == "tool" || k == "outputs" || k == "output"
		}
	}
	return &BaseNormalizer{
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
		taskType:       taskType,
		filterFunc:     filterFunc,
	}
}

// Type returns the task type this normalizer handles
func (n *BaseNormalizer) Type() task.Type {
	return n.taskType
}

// Normalize applies common normalization rules across all task types
func (n *BaseNormalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Type assert to get the concrete type
	normCtx, ok := ctx.(*NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *NormalizationContext, got %T", ctx)
	}
	if config == nil {
		return nil
	}
	// Allow empty type for basic tasks
	if n.taskType == task.TaskTypeBasic {
		if config.Type != task.TaskTypeBasic && config.Type != "" {
			return fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskType, config.Type)
		}
	} else if config.Type != n.taskType {
		return fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskType, config.Type)
	}
	// Build template context
	context := normCtx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Preserve existing With values before normalization
	existingWith := config.With
	// Apply template processing with appropriate filters
	if n.templateEngine == nil {
		return fmt.Errorf("template engine is required for normalization")
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, n.filterFunc)
	if err != nil {
		return fmt.Errorf("failed to normalize %s task config: %w", n.taskType, err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	// Merge existing With values back into the normalized config
	if existingWith != nil && config.With != nil {
		mergedWith := make(core.Input)
		maps.Copy(mergedWith, *config.With)
		maps.Copy(mergedWith, *existingWith)
		config.With = &mergedWith
	} else if existingWith != nil {
		config.With = existingWith
	}
	return nil
}

// ProcessTemplateString processes a single string template
func (n *BaseNormalizer) ProcessTemplateString(value string, context map[string]any) (string, error) {
	if n.templateEngine == nil {
		return "", fmt.Errorf("template engine is required for template processing")
	}
	result, err := n.templateEngine.ParseAny(value, context)
	if err != nil {
		return "", err
	}
	value, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", result)
	}
	return value, nil
}

// ProcessTemplateMap processes a map containing templates
func (n *BaseNormalizer) ProcessTemplateMap(value map[string]any, context map[string]any) (map[string]any, error) {
	if n.templateEngine == nil {
		return nil, fmt.Errorf("template engine is required for template processing")
	}
	result, err := n.templateEngine.ParseAny(value, context)
	if err != nil {
		return nil, err
	}
	value, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map[string]any, got %T", result)
	}
	return value, nil
}

// TemplateEngine returns the template engine for custom processing
func (n *BaseNormalizer) TemplateEngine() *tplengine.TemplateEngine {
	return n.templateEngine
}
