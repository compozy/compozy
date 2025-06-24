package signal

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for signal tasks
type Normalizer struct {
	templateEngine shared.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewNormalizer creates a new signal task normalizer
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
	return task.TaskTypeSignal
}

// Normalize applies signal task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	if config == nil {
		return nil
	}
	if config.Type != task.TaskTypeSignal {
		return fmt.Errorf("signal normalizer cannot handle task type: %s", config.Type)
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
	// Signal tasks exclude: agent, tool, outputs, output from template processing
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == "outputs" || k == "output"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize signal task config: %w", err)
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
		processed, err := n.templateEngine.Process(signalConfig.ID, context)
		if err != nil {
			return fmt.Errorf("failed to process signal ID: %w", err)
		}
		signalConfig.ID = processed
	}
	// Process payload if it's a map containing templates
	if signalConfig.Payload != nil {
		processedPayload, err := n.templateEngine.ProcessMap(signalConfig.Payload, context)
		if err != nil {
			return fmt.Errorf("failed to process signal payload: %w", err)
		}
		signalConfig.Payload = processedPayload
	}
	return nil
}
