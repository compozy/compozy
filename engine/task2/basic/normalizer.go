package basic

import (
	"context"
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

// created environment merger.
func NewNormalizer(_ context.Context, templateEngine *tplengine.TemplateEngine) *Normalizer {
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
func (n *Normalizer) Normalize(
	ctx context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	// Handle nil config gracefully
	if config == nil {
		return nil
	}

	// Always apply base normalization - it will handle selective processing
	if err := n.normalizeWithSelectiveProcessing(ctx, config, parentCtx); err != nil {
		return err
	}

	// Type assert to get the concrete type
	normCtx, ok := parentCtx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", parentCtx)
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
func (n *Normalizer) normalizeWithSelectiveProcessing(
	ctx context.Context,
	config *task.Config,
	parentCtx contracts.NormalizationContext,
) error {
	// Type assert to get the concrete type
	normCtx, ok := parentCtx.(*shared.NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *shared.NormalizationContext, got %T", parentCtx)
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
	if err := n.BaseNormalizer.Normalize(ctx, config, normCtx); err != nil {
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
	// Recursively check the input map for template expressions with runtime references
	return n.checkMapForRuntimeReferences(*input)
}

// checkMapForRuntimeReferences recursively checks a map for runtime template references
func (n *Normalizer) checkMapForRuntimeReferences(m map[string]any) bool {
	for _, v := range m {
		switch val := v.(type) {
		case string:
			if tplengine.HasTemplate(val) && strings.Contains(val, ".tasks.") {
				return true
			}
		case map[string]any:
			if n.checkMapForRuntimeReferences(val) {
				return true
			}
		case []any:
			// Handle slices of any type.
			for _, item := range val {
				switch itemVal := item.(type) {
				case map[string]any:
					if n.checkMapForRuntimeReferences(itemVal) {
						return true
					}
				case string:
					if tplengine.HasTemplate(itemVal) && strings.Contains(itemVal, ".tasks.") {
						return true
					}
				}
			}
		}
	}
	return false
}
