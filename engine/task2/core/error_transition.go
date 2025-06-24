package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
)

// ErrorTransitionNormalizer handles error transition normalization
type ErrorTransitionNormalizer struct {
	templateEngine shared.TemplateEngine
}

// NewErrorTransitionNormalizer creates a new error transition normalizer
func NewErrorTransitionNormalizer(templateEngine shared.TemplateEngine) *ErrorTransitionNormalizer {
	return &ErrorTransitionNormalizer{
		templateEngine: templateEngine,
	}
}

// Normalize normalizes an error transition configuration
func (n *ErrorTransitionNormalizer) Normalize(
	transition *core.ErrorTransition,
	ctx *shared.NormalizationContext,
) error {
	if transition == nil {
		return nil
	}
	// Set current input if not already set
	if ctx.CurrentInput == nil && transition.With != nil {
		ctx.CurrentInput = transition.With
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert transition to map for template processing
	configMap, err := transition.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert transition to map: %w", err)
	}
	// Apply template processing to all fields
	parsed, err := n.templateEngine.ParseMap(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize transition: %w", err)
	}
	// Update transition from normalized map
	if err := transition.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update transition from normalized map: %w", err)
	}
	return nil
}
