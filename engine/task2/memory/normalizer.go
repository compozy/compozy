package memory

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for memory tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new memory task normalizer
func NewNormalizer(_ context.Context, templateEngine *tplengine.TemplateEngine) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			nil, // Memory normalizer doesn't use contextBuilder
			task.TaskTypeMemory,
			nil, // Use default filter
		),
	}
}

// Normalize applies normalization rules for memory tasks
func (n *Normalizer) Normalize(
	ctx context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	// Handle nil config gracefully
	if config == nil {
		return nil
	}
	// Apply base normalization
	// Memory task constraints are already validated in task.TypeValidator.validateMemoryTask()
	if err := n.BaseNormalizer.Normalize(ctx, config, parentCtx); err != nil {
		return err
	}
	// Memory tasks are simple and don't need additional normalization
	// beyond what BaseNormalizer provides
	return nil
}
