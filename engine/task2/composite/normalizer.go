package composite

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for composite tasks
type Normalizer struct {
	*shared.BaseSubTaskNormalizer
}

// NewNormalizer creates a new composite task normalizer
func NewNormalizer(
	templateEngine shared.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	normalizerFactory shared.NormalizerFactory,
) *Normalizer {
	return &Normalizer{
		BaseSubTaskNormalizer: shared.NewBaseSubTaskNormalizer(
			templateEngine,
			contextBuilder,
			normalizerFactory,
			task.TaskTypeComposite,
			"composite",
		),
	}
}
