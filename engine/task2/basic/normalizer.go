package basic

import (
	"fmt"
	"strings"

	enginecore "github.com/compozy/compozy/engine/core"
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
		agentNormalizer: core.NewAgentNormalizer(envMerger),
	}
}

// Normalize applies normalization rules for basic tasks
func (n *Normalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Handle nil config gracefully
	if config == nil {
		return nil
	}

	// Always apply base normalization - it will handle selective processing
	if err := n.normalizeWithSelectiveProcessing(config, ctx); err != nil {
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

// normalizeWithSelectiveProcessing applies normalization but preserves runtime references
func (n *Normalizer) normalizeWithSelectiveProcessing(config *task.Config, ctx contracts.NormalizationContext) error {
	// Type assert to get the concrete type
	normCtx, ok := ctx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", ctx)
	}

	// Detect if runtime already has the .tasks map with completed task outputs
	hasTasksVar := normCtx != nil &&
		normCtx.Variables != nil &&
		normCtx.Variables["tasks"] != nil

	// Preserve With only when it still contains runtime references *and*
	// the tasks variable is NOT yet available (compile-time pass).
	var preservedWith *enginecore.Input
	if config.With != nil && n.containsRuntimeReferences(config.With) && !hasTasksVar {
		preservedWith = config.With
	}

	// Apply base normalization which will process all templates
	if err := n.BaseNormalizer.Normalize(config, normCtx); err != nil {
		return err
	}

	// Restore the preserved With field if it had runtime references
	// that we intentionally skipped during the first pass
	if preservedWith != nil {
		config.With = preservedWith
	}

	return nil
}

// containsRuntimeReferences checks if an input contains runtime-only references
func (n *Normalizer) containsRuntimeReferences(input *enginecore.Input) bool {
	if input == nil {
		return false
	}
	// Check if the input contains .tasks references
	// These should be deferred to runtime when task outputs are available
	inputStr := fmt.Sprintf("%v", *input)
	return strings.Contains(inputStr, ".tasks.")
}
