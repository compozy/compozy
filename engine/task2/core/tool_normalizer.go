package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/tplengine"
)

const (
	fieldInput  = "input"
	fieldOutput = "output"
)

// ToolNormalizer handles tool component normalization
type ToolNormalizer struct {
	templateEngine *tplengine.TemplateEngine
	envMerger      *EnvMerger
}

// NewToolNormalizer creates a new tool normalizer
func NewToolNormalizer(templateEngine *tplengine.TemplateEngine, envMerger *EnvMerger) *ToolNormalizer {
	return &ToolNormalizer{
		templateEngine: templateEngine,
		envMerger:      envMerger,
	}
}

// NormalizeTool normalizes a tool configuration
func (n *ToolNormalizer) NormalizeTool(
	config *tool.Config,
	ctx *shared.NormalizationContext,
) error {
	if config == nil {
		return nil
	}
	mergedEnv := n.envMerger.MergeThreeLevels(
		ctx.WorkflowConfig,
		ctx.TaskConfig,
		config.Env, // Tool's environment overrides task and workflow
	)
	ctx.MergedEnv = mergedEnv
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	context := ctx.BuildTemplateContext()
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert tool config to map: %w", err)
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == fieldInput || k == fieldOutput
	})
	if err != nil {
		return fmt.Errorf("failed to normalize tool config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update tool config from normalized map: %w", err)
	}
	return nil
}
