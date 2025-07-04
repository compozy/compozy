package signal

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for signal tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new signal task normalizer
func NewNormalizer(
	templateEngine *tplengine.TemplateEngine,
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
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Check for nil config
	if config == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	// Call base normalization first
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}
	// Type assert to get the concrete type for signal-specific logic
	normCtx, ok := ctx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", ctx)
	}
	// Build template context for signal-specific fields
	context := normCtx.BuildTemplateContext()
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
