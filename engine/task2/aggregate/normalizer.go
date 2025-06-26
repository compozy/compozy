package aggregate

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for aggregate tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new aggregate task normalizer
func NewNormalizer(
	templateEngine shared.TemplateEngine,
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
