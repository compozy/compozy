package signal

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for signal tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new signal task normalizer
func NewNormalizer(
	templateEngine shared.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			contextBuilder,
			task.TaskTypeSignal,
			nil, // Use default filter
		),
	}
}

// Normalize applies signal task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	// Call base normalization first
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}
	// Build template context for signal-specific fields
	context := ctx.BuildTemplateContext()
	// Normalize signal-specific fields
	if config.Signal != nil {
		if err := n.normalizeSignalConfig(config.Signal, context); err != nil {
			return fmt.Errorf("failed to normalize signal config: %w", err)
		}
	}
	return nil
}

// normalizeSignalConfig normalizes signal-specific configuration
func (n *Normalizer) normalizeSignalConfig(signalConfig *task.SignalConfig, context map[string]any) error {
	if signalConfig == nil {
		return nil
	}
	// Process signal ID if it contains templates
	if signalConfig.ID != "" {
		processed, err := n.ProcessTemplateString(signalConfig.ID, context)
		if err != nil {
			return fmt.Errorf("failed to process signal ID: %w", err)
		}
		signalConfig.ID = processed
	}
	// Process payload if it's a map containing templates
	if signalConfig.Payload != nil {
		processedPayload, err := n.ProcessTemplateMap(signalConfig.Payload, context)
		if err != nil {
			return fmt.Errorf("failed to process signal payload: %w", err)
		}
		signalConfig.Payload = processedPayload
	}
	return nil
}
