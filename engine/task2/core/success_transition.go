package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// SuccessTransitionNormalizer handles success transition normalization
type SuccessTransitionNormalizer struct {
	*TransitionNormalizer[*core.SuccessTransition]
}

// NewSuccessTransitionNormalizer creates a new success transition normalizer
func NewSuccessTransitionNormalizer(templateEngine *tplengine.TemplateEngine) *SuccessTransitionNormalizer {
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
