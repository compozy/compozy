package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
)

// SuccessTransitionNormalizer handles success transition normalization
type SuccessTransitionNormalizer struct {
	*TransitionNormalizer[*core.SuccessTransition]
}

// NewSuccessTransitionNormalizer creates a new success transition normalizer
func NewSuccessTransitionNormalizer(templateEngine shared.TemplateEngine) *SuccessTransitionNormalizer {
	return &SuccessTransitionNormalizer{
		TransitionNormalizer: NewTransitionNormalizer[*core.SuccessTransition](templateEngine),
	}
}

// Normalize normalizes a success transition configuration
func (n *SuccessTransitionNormalizer) Normalize(
	transition *core.SuccessTransition,
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
