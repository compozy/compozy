package basic

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// Normalizer handles normalization for basic tasks
type Normalizer struct {
	*shared.BaseNormalizer
	agentNormalizer *core.AgentNormalizer
}

// NewNormalizer creates a new basic task normalizer
func NewNormalizer(templateEngine *tplengine.TemplateEngine) *Normalizer {
	envMerger := core.NewEnvMerger()
	return &Normalizer{
		BaseNormalizer: shared.NewBaseNormalizer(
			templateEngine,
			nil, // Basic normalizer doesn't use contextBuilder
			task.TaskTypeBasic,
			nil, // Use default filter
		),
		agentNormalizer: core.NewAgentNormalizer(templateEngine, envMerger),
	}
}

// Normalize applies normalization rules for basic tasks
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Handle nil config gracefully
	if config == nil {
		return nil
	}

	// First apply base normalization
	if err := n.BaseNormalizer.Normalize(config, ctx); err != nil {
		return err
	}

	// Type assert to get the concrete type
	normCtx, ok := ctx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", ctx)
	}

	// Normalize agent configuration if present
	if config.Agent != nil {
		actionID := config.Action
		if err := n.agentNormalizer.NormalizeAgent(config.Agent, normCtx, actionID); err != nil {
			return fmt.Errorf("failed to normalize agent config: %w", err)
		}
	}

	return nil
}
