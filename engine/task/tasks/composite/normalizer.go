package composite

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for composite tasks
type Normalizer struct {
	*shared.BaseSubTaskNormalizer
}

// NewNormalizer creates a new composite task normalizer
func NewNormalizer(
	_ context.Context,
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	normalizerFactory contracts.NormalizerFactory,
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
