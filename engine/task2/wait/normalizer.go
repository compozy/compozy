package wait

import (
	"fmt"
	"maps"

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
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			contextBuilder,
			task.TaskTypeWait,
			func(k string) bool {
				// Wait tasks skip "processor" field - it contains signal templates that need deferred evaluation
				return k == "agent" || k == "tool" || k == "outputs" || k == "output" || k == "processor"
			},
		),
	}
}

// Normalize applies wait task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Call base normalization first
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}
	// Apply inheritance to processor if present
	if config != nil && config.Processor != nil {
		shared.InheritTaskConfig(config.Processor, config)
	}
	return nil
}

// NormalizeWithSignal normalizes a wait task config with signal context
// This is specifically for wait task processors that need signal data during normalization
func (n *Normalizer) NormalizeWithSignal(
	config *task.Config,
	ctx *shared.NormalizationContext,
	signal any,
) error {
	if config == nil {
		return nil
	}
	// Build the full normalization context
	context := ctx.BuildTemplateContext()
	// Add signal data to the context (convert to map for template engine)
	if signal != nil {
		signalMap, err := core.AsMapDefault(signal)
		if err != nil {
			return fmt.Errorf("failed to convert signal to map: %w", err)
		}
		context["signal"] = signalMap
	} else {
		context["signal"] = nil
	}
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Preserve existing With values before normalization
	existingWith := config.With
	// Parse all templates with the signal-augmented context
	parsed, err := n.TemplateEngine().ParseAny(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize task config with signal context: %w", err)
	}
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
