package signal

import (
	"context"
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
	_ context.Context,
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
func (n *Normalizer) Normalize(
	ctx context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	if config == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if err := n.BaseNormalizer.Normalize(ctx, config, parentCtx); err != nil {
		return err
	}
	normCtx, ok := parentCtx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", parentCtx)
	}
	context := normCtx.BuildTemplateContext()
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
	if signalConfig.ID != "" {
		processed, err := n.ProcessTemplateString(signalConfig.ID, context)
		if err != nil {
			return fmt.Errorf("failed to process signal ID: %w", err)
		}
		signalConfig.ID = processed
	}
	if signalConfig.Payload != nil {
		processedPayload, err := n.ProcessTemplateMap(signalConfig.Payload, context)
		if err != nil {
			return fmt.Errorf("failed to process signal payload: %w", err)
		}
		signalConfig.Payload = processedPayload
	}
	return nil
}
