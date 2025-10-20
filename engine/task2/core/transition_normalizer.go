package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Transition interface for common transition operations
type Transition interface {
	GetWith() *core.Input
	AsMap() (map[string]any, error)
	FromMap(data any) error
}

// TransitionNormalizer handles normalization for any transition type
type TransitionNormalizer[T Transition] struct {
	templateEngine *tplengine.TemplateEngine
}

// NewTransitionNormalizer creates a new generic transition normalizer
func NewTransitionNormalizer[T Transition](templateEngine *tplengine.TemplateEngine) *TransitionNormalizer[T] {
	return &TransitionNormalizer[T]{
		templateEngine: templateEngine,
	}
}

// Normalize normalizes a transition configuration
func (n *TransitionNormalizer[T]) Normalize(
	transition T,
	ctx *shared.NormalizationContext,
) error {
	if isNil(transition) {
		return nil
	}
	if ctx.CurrentInput == nil && transition.GetWith() != nil {
		ctx.CurrentInput = transition.GetWith()
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

// isNil checks if an interface value is nil
func isNil(i any) bool {
	if i == nil {
		return true
	}
	switch v := i.(type) {
	case *core.SuccessTransition:
		return v == nil
	case *core.ErrorTransition:
		return v == nil
	}
	return false
}
