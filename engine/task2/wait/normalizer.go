package wait

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for wait tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new wait task normalizer
func NewNormalizer(
	_ context.Context,
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			contextBuilder,
			task.TaskTypeWait,
			func(k string) bool {
				return k == "agent" || k == "tool" || k == "outputs" || k == "output" || k == "processor"
			},
		),
	}
}

// Normalize applies wait task-specific normalization rules
func (n *Normalizer) Normalize(ctx context.Context, config *task.Config, normCtx contracts.NormalizationContext) error {
	if err := n.BaseNormalizer.Normalize(ctx, config, normCtx); err != nil {
		return err
	}
	if config != nil && config.Processor != nil {
		if err := shared.InheritTaskConfig(config.Processor, config); err != nil {
			return fmt.Errorf("failed to inherit task config: %w", err)
		}
	}
	return nil
}

// NormalizeWithSignal normalizes a wait task config with signal context
// This is specifically for wait task processors that need signal data during normalization
func (n *Normalizer) NormalizeWithSignal(
	_ context.Context,
	config *task.Config,
	normCtx *shared.NormalizationContext,
	signal any,
) error {
	if config == nil {
		return nil
	}
	context, err := n.buildContextWithSignal(normCtx, signal)
	if err != nil {
		return err
	}
	existingWith := config.With
	parsed, err := n.parseConfigTemplates(config, context)
	if err != nil {
		return err
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	n.restoreWithValues(config, existingWith)
	return nil
}

// buildContextWithSignal enriches the normalization context with optional signal data
func (n *Normalizer) buildContextWithSignal(
	normCtx *shared.NormalizationContext,
	signal any,
) (map[string]any, error) {
	context := normCtx.BuildTemplateContext()
	if signal == nil {
		context["signal"] = nil
		return context, nil
	}
	signalMap, err := core.AsMapDefault(signal)
	if err != nil {
		return nil, fmt.Errorf("failed to convert signal to map: %w", err)
	}
	context["signal"] = signalMap
	return context, nil
}

// parseConfigTemplates converts the config to a map and applies template parsing
func (n *Normalizer) parseConfigTemplates(
	config *task.Config,
	context map[string]any,
) (map[string]any, error) {
	configMap, err := config.AsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert task config to map: %w", err)
	}
	parsed, err := n.TemplateEngine().ParseAny(configMap, context)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize task config with signal context: %w", err)
	}
	parsedMap, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected normalized task config type %T", parsed)
	}
	return parsedMap, nil
}

// restoreWithValues merges the original With values back into the normalized config
func (n *Normalizer) restoreWithValues(config *task.Config, existingWith *core.Input) {
	if existingWith == nil {
		return
	}
	if config.With != nil {
		merged := core.CopyMaps(*existingWith, *config.With)
		mergedWith := core.Input(merged)
		config.With = &mergedWith
		return
	}
	cloned, err := core.DeepCopy(*existingWith)
	if err == nil {
		config.With = &cloned
		return
	}
	config.With = existingWith
}
