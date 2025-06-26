package basic

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Normalizer handles normalization for basic tasks
type Normalizer struct {
	*shared.BaseNormalizer
}

// NewNormalizer creates a new basic task normalizer
func NewNormalizer(templateEngine shared.TemplateEngine) *Normalizer {
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			nil, // Basic normalizer doesn't use contextBuilder
			task.TaskTypeBasic,
			nil, // Use default filter
		),
	}
}
