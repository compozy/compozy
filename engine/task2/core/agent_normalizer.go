package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task2/shared"
)

// AgentNormalizer handles agent component normalization
type AgentNormalizer struct {
	templateEngine shared.TemplateEngine
	envMerger      *EnvMerger
}

// NewAgentNormalizer creates a new agent normalizer
func NewAgentNormalizer(templateEngine shared.TemplateEngine, envMerger *EnvMerger) *AgentNormalizer {
	return &AgentNormalizer{
		templateEngine: templateEngine,
		envMerger:      envMerger,
	}
}

// NormalizeAgent normalizes an agent configuration
func (n *AgentNormalizer) NormalizeAgent(
	config *agent.Config,
	ctx *shared.NormalizationContext,
	actionID string,
) error {
	if config == nil {
		return nil
	}
	// Set current input if not already set
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert agent config to map: %w", err)
	}
	// Apply template processing with appropriate filters
	// Skip actions, tools, input, and output fields during general normalization
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "actions" || k == "tools" || k == "input" || k == "output"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize agent config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update agent config from normalized map: %w", err)
	}
	// Normalize agent actions if actionID is provided
	if err := n.normalizeAgentActions(config, ctx, actionID); err != nil {
		return fmt.Errorf("failed to normalize agent actions: %w", err)
	}
	// Tools normalization happens separately via tool normalizer
	return nil
}

// normalizeAgentActions normalizes agent actions
func (n *AgentNormalizer) normalizeAgentActions(
	config *agent.Config,
	ctx *shared.NormalizationContext,
	actionID string,
) error {
	// Normalize agent actions (only if actionID is provided and actions exist)
	if actionID != "" && len(config.Actions) > 0 {
		aConfig, err := agent.FindActionConfig(config.Actions, actionID)
		if err != nil {
			return fmt.Errorf("failed to find action config: %w", err)
		}
		if aConfig == nil {
			return fmt.Errorf("agent action %s not found in agent config %s", actionID, config.ID)
		}
		// Merge input from agent and action
		mergedInput, err := config.GetInput().Merge(aConfig.GetInput())
		if err != nil {
			return fmt.Errorf("failed to merge input for agent action %s: %w", aConfig.ID, err)
		}
		aConfig.With = mergedInput
		// Create action context with agent as parent
		actionCtx := &shared.NormalizationContext{
			WorkflowState:  ctx.WorkflowState,
			WorkflowConfig: ctx.WorkflowConfig,
			TaskConfigs:    ctx.TaskConfigs,
			ParentConfig: map[string]any{
				"id":           config.ID,
				"input":        config.With,
				"instructions": config.Instructions,
				"config":       config.Config,
			},
			CurrentInput:  aConfig.With,
			MergedEnv:     ctx.MergedEnv,
			ChildrenIndex: ctx.ChildrenIndex,
		}
		// Normalize the action config
		if err := n.normalizeAgentActionConfig(aConfig, actionCtx); err != nil {
			return fmt.Errorf("failed to normalize agent action config: %w", err)
		}
	}
	return nil
}

// normalizeAgentActionConfig normalizes an agent action configuration
func (n *AgentNormalizer) normalizeAgentActionConfig(
	config *agent.ActionConfig,
	ctx *shared.NormalizationContext,
) error {
	if config == nil {
		return nil
	}
	// Set current input if not already set
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	// Build template context
	context := ctx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert action config to map: %w", err)
	}
	// Apply template processing with appropriate filters
	// Skip input and output fields during action normalization
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "input" || k == "output"
	})
	if err != nil {
		return fmt.Errorf("failed to normalize action config: %w", err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update action config from normalized map: %w", err)
	}
	return nil
}
