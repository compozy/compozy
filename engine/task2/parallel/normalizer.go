package parallel

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for parallel tasks
type Normalizer struct {
	*shared.BaseSubTaskNormalizer
}

// NewNormalizer creates a new parallel task normalizer
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
			task.TaskTypeParallel,
			"parallel",
		),
	}
}
