package shared

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
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
func (n *BaseNormalizer) Normalize(
	_ context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	normCtx, err := n.extractNormalizationContext(parentCtx)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	if err := n.validateConfigType(config); err != nil {
		return err
	}
	context := normCtx.BuildTemplateContext()
	parsed, existingWith, err := n.parseTaskConfigMap(config, context)
	if err != nil {
		return err
	}
	return n.applyNormalizedConfig(config, parsed, existingWith)
}

// extractNormalizationContext ensures the provided normalization context is valid.
func (n *BaseNormalizer) extractNormalizationContext(
	parentCtx contracts.NormalizationContext,
) (*NormalizationContext, error) {
	normCtx, ok := parentCtx.(*NormalizationContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type: expected *NormalizationContext, got %T", parentCtx)
	}
	return normCtx, nil
}

// validateConfigType confirms the normalizer handles the provided task type.
func (n *BaseNormalizer) validateConfigType(config *task.Config) error {
	if n.taskType == task.TaskTypeBasic {
		if config.Type != task.TaskTypeBasic && config.Type != "" && config.Type != task.TaskTypeMemory {
			return fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskType, config.Type)
		}
		return nil
	}
	if config.Type != n.taskType {
		return fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskType, config.Type)
	}
	return nil
}

// parseTaskConfigMap prepares the configuration map for template processing.
func (n *BaseNormalizer) parseTaskConfigMap(
	config *task.Config,
	context map[string]any,
) (map[string]any, *core.Input, error) {
	configMap, err := config.AsMap()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert task config to map: %w", err)
	}
	existingWith := config.With
	if n.templateEngine == nil {
		return nil, nil, fmt.Errorf("template engine is required for normalization")
	}
	parsedAny, err := n.templateEngine.ParseMapWithFilter(configMap, context, n.filterFunc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize %s task config: %w", n.taskType, err)
	}
	parsed, ok := parsedAny.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("normalized %s task config is not a map", n.taskType)
	}
	return parsed, existingWith, nil
}

// applyNormalizedConfig updates the config with parsed data and restores With values.
func (n *BaseNormalizer) applyNormalizedConfig(
	config *task.Config,
	parsed map[string]any,
	existingWith *core.Input,
) error {
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	if existingWith != nil && config.With != nil {
		merged := core.CopyMaps(*existingWith, *config.With)
		mergedWith := core.Input(merged)
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
