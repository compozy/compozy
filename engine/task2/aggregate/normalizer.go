package aggregate

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for aggregate tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new aggregate task normalizer
func NewNormalizer(
	_ context.Context,
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			contextBuilder,
			task.TaskTypeAggregate,
			nil, // Use default filter
		),
	}
}
