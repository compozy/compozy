package wait

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for wait tasks
type Normalizer struct {
	templateEngine shared.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewNormalizer creates a new wait task normalizer
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
	return task.TaskTypeWait
}

// Normalize applies wait task-specific normalization rules
func (n *Normalizer) Normalize(config *task.Config, ctx *shared.NormalizationContext) error {
	if config == nil {
		return nil
	}
	if config.Type != task.TaskTypeWait {
		return fmt.Errorf("wait normalizer cannot handle task type: %s", config.Type)
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
	// Wait tasks skip "processor" field - it contains signal templates that need deferred evaluation
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == "outputs" || k == "output" || k == "processor"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize wait task config: %w", err)
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
	parsed, err := n.templateEngine.ParseMap(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize task config with signal context: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	// Merge existing With values back into the normalized config
	if existingWith != nil && config.With != nil {
		// Check for aliasing to prevent concurrent map iteration and write panic
		if existingWith != config.With {
			// Merge existing values into normalized values (existing takes precedence)
			maps.Copy((*config.With), *existingWith)
		}
	} else if existingWith != nil {
		// If normalization cleared With but we had existing values, restore them
		config.With = existingWith
	}
	return nil
}
