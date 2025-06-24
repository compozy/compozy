package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/tool"
)

const (
	fieldInput  = "input"
	fieldOutput = "output"
)

// ToolNormalizer handles tool component normalization
type ToolNormalizer struct {
	templateEngine shared.TemplateEngine
	envMerger      *EnvMerger
}

// NewToolNormalizer creates a new tool normalizer
func NewToolNormalizer(templateEngine shared.TemplateEngine, envMerger *EnvMerger) *ToolNormalizer {
	return &ToolNormalizer{
		templateEngine: templateEngine,
		envMerger:      envMerger,
	}
}

// NormalizeTool normalizes a tool configuration
func (n *ToolNormalizer) NormalizeTool(
	config *tool.Config,
	ctx *shared.NormalizationContext,
) error {
	if config == nil {
		return nil
	}
	// Set current input if not already set
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert tool config to map: %w", err)
	}
	// Apply template processing with appropriate filters
	// Skip input and output fields during tool normalization
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == fieldInput || k == fieldOutput
	})
	if err != nil {
		return fmt.Errorf("failed to normalize tool config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update tool config from normalized map: %w", err)
	}
	return nil
}
