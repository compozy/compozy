package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ErrorTransitionNormalizer handles error transition normalization
type ErrorTransitionNormalizer struct {
	*TransitionNormalizer[*core.ErrorTransition]
}

// NewErrorTransitionNormalizer creates a new error transition normalizer
func NewErrorTransitionNormalizer(templateEngine *tplengine.TemplateEngine) *ErrorTransitionNormalizer {
	return &ErrorTransitionNormalizer{
		TransitionNormalizer: NewTransitionNormalizer[*core.ErrorTransition](templateEngine),
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
	if ctx.CurrentInput == nil && transition.With != nil {
		ctx.CurrentInput = transition.With
	}
	context := ctx.BuildTemplateContext()
	configMap, err := transition.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert transition to map: %w", err)
	}
	parsed, err := n.templateEngine.ParseAny(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize transition: %w", err)
	}
	if err := transition.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update transition from normalized map: %w", err)
	}
	return nil
}
