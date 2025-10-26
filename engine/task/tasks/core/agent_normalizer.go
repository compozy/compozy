package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// AgentNormalizer handles agent component normalization
type AgentNormalizer struct {
	templateEngine *tplengine.TemplateEngine
	envMerger      *EnvMerger
}

// buildTaskMetadata creates task metadata for template context
func (n *AgentNormalizer) buildTaskMetadata(id, taskType string, config any) map[string]any {
	metadata := map[string]any{
		"id":   id,
		"type": taskType,
	}
	switch taskType {
	case "agent":
		if agentCfg, ok := config.(*agent.Config); ok {
			metadata["instructions"] = agentCfg.Instructions
			metadata["with"] = agentCfg.With
			metadata["env"] = agentCfg.Env
		}
	case "agent_action":
		if actionCfg, ok := config.(*agent.ActionConfig); ok {
			metadata["prompt"] = actionCfg.Prompt
			metadata["with"] = actionCfg.With
		}
	}
	return metadata
}

// parseInputTemplates resolves any template expressions contained in the
// provided core.Input using the supplied template context. It returns a new
// *enginecore.Input with the parsed values or the original pointer if no changes are
// necessary.
func (n *AgentNormalizer) parseInputTemplates(
	input *enginecore.Input,
	templateCtx map[string]any,
) (*enginecore.Input, error) {
	if input == nil || *input == nil || len(*input) == 0 {
		return input, nil
	}
	parsedAny, err := n.templateEngine.ParseAny(map[string]any(*input), templateCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input templates: %w", err)
	}
	parsedMap, ok := parsedAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parsed input is not a map (got %T)", parsedAny)
	}
	parsedInput := enginecore.Input(parsedMap)
	return &parsedInput, nil
}

// NewAgentNormalizer creates and returns an AgentNormalizer.
// The returned normalizer uses a JSON-format template engine and the provided
// EnvMerger to merge environment variables across workflow/task/agent levels.
func NewAgentNormalizer(envMerger *EnvMerger) *AgentNormalizer {
	return &AgentNormalizer{
		templateEngine: tplengine.NewEngine(tplengine.FormatJSON),
		envMerger:      envMerger,
	}
}

// ReparseInput re-evaluates template expressions inside the With blocks of the
// given agent configuration *at runtime*, when the full template context
// (including `.tasks.*` data) is finally available.  It only mutates the `With`
// fields of the agent and its actions; all other fields remain untouched.
// If actionID is provided, only that specific action is reparsed.
func (n *AgentNormalizer) ReparseInput(
	agentCfg *agent.Config,
	ctx *shared.NormalizationContext,
	actionID string,
) error {
	if agentCfg == nil || ctx == nil {
		return nil
	}
	templateCtx := ctx.BuildTemplateContext()
	if err := n.reparseAgentWithBlock(agentCfg, templateCtx); err != nil {
		return err
	}
	return n.reparseAgentActions(agentCfg, actionID, templateCtx)
}

// reparseAgentWithBlock updates the agent-level With block with runtime context.
func (n *AgentNormalizer) reparseAgentWithBlock(
	agentCfg *agent.Config,
	templateCtx map[string]any,
) error {
	parsedWith, err := n.parseInputTemplates(agentCfg.With, templateCtx)
	if err != nil {
		return fmt.Errorf("runtime template parse (agent.With) failed: %w", err)
	}
	agentCfg.With = parsedWith
	return nil
}

// reparseAgentActions refreshes action inputs and prompts for runtime execution.
func (n *AgentNormalizer) reparseAgentActions(
	agentCfg *agent.Config,
	actionID string,
	templateCtx map[string]any,
) error {
	if len(agentCfg.Actions) == 0 {
		return nil
	}
	for _, action := range agentCfg.Actions {
		if action == nil || (actionID != "" && action.ID != actionID) {
			continue
		}
		if err := n.reparseSingleAction(action, templateCtx); err != nil {
			return err
		}
	}
	return nil
}

// reparseSingleAction reparses a specific action's With and Prompt sections.
func (n *AgentNormalizer) reparseSingleAction(
	action *agent.ActionConfig,
	templateCtx map[string]any,
) error {
	parsedWith, err := n.parseInputTemplates(action.With, templateCtx)
	if err != nil {
		return fmt.Errorf(
			"runtime template parse (action.With, action %s) failed: %w",
			action.ID, err,
		)
	}
	action.With = parsedWith
	if action.Prompt == "" {
		return nil
	}
	parsedPromptAny, err := n.templateEngine.ParseAny(action.Prompt, templateCtx)
	if err != nil {
		return fmt.Errorf(
			"runtime template parse (action.Prompt, action %s) failed: %w",
			action.ID, err,
		)
	}
	promptStr, ok := parsedPromptAny.(string)
	if !ok {
		return fmt.Errorf("parsed prompt is not a string (action %s)", action.ID)
	}
	action.Prompt = promptStr
	return nil
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
	n.mergeAgentEnvironment(ctx, config)
	n.ensureAgentCurrentInput(ctx, config)
	n.updateAgentTaskVariables(ctx, config)
	templateCtx := ctx.BuildTemplateContext()
	if err := n.normalizeAgentWithBlock(config, templateCtx); err != nil {
		return err
	}
	if err := n.normalizeAgentConfigMap(config, templateCtx); err != nil {
		return err
	}
	if err := n.normalizeAgentActions(config, ctx, actionID); err != nil {
		return fmt.Errorf("failed to normalize agent actions: %w", err)
	}
	return nil
}

// mergeAgentEnvironment populates the normalization context with merged env vars.
func (n *AgentNormalizer) mergeAgentEnvironment(ctx *shared.NormalizationContext, config *agent.Config) {
	if ctx == nil {
		return
	}
	mergedEnv := n.envMerger.MergeThreeLevels(
		ctx.WorkflowConfig,
		ctx.TaskConfig,
		config.Env,
	)
	ctx.MergedEnv = mergedEnv
}

// ensureAgentCurrentInput ensures the context exposes the agent With block as input.
func (n *AgentNormalizer) ensureAgentCurrentInput(ctx *shared.NormalizationContext, config *agent.Config) {
	if ctx == nil {
		return
	}
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
}

// updateAgentTaskVariables refreshes `.task` metadata to describe the current agent.
func (n *AgentNormalizer) updateAgentTaskVariables(ctx *shared.NormalizationContext, config *agent.Config) {
	if ctx == nil {
		return
	}
	vars := ctx.GetVariables()
	vars["task"] = n.buildTaskMetadata(config.ID, "agent", config)
}

// normalizeAgentWithBlock parses agent-level inputs during configuration normalization.
func (n *AgentNormalizer) normalizeAgentWithBlock(
	config *agent.Config,
	templateCtx map[string]any,
) error {
	parsedWith, err := n.parseInputTemplates(config.With, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to parse agent input templates: %w", err)
	}
	config.With = parsedWith
	return nil
}

// normalizeAgentConfigMap applies template parsing to general agent configuration fields.
func (n *AgentNormalizer) normalizeAgentConfigMap(
	config *agent.Config,
	templateCtx map[string]any,
) error {
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert agent config to map: %w", err)
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, templateCtx, n.skipAgentFieldDuringParse)
	if err != nil {
		return fmt.Errorf("failed to normalize agent config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update agent config from normalized map: %w", err)
	}
	return nil
}

// skipAgentFieldDuringParse identifies agent fields that should not be template processed.
func (n *AgentNormalizer) skipAgentFieldDuringParse(key string) bool {
	return key == "actions" || key == "tools" || key == "input" || key == "output"
}

// normalizeAgentActions normalizes agent actions
func (n *AgentNormalizer) normalizeAgentActions(
	config *agent.Config,
	ctx *shared.NormalizationContext,
	actionID string,
) error {
	if actionID != "" && len(config.Actions) > 0 {
		aConfig, err := agent.FindActionConfig(config.Actions, actionID)
		if err != nil {
			return fmt.Errorf("failed to find action config: %w", err)
		}
		if aConfig == nil {
			return fmt.Errorf("agent action %s not found in agent config %s", actionID, config.ID)
		}
		mergedInput, err := config.GetInput().Merge(aConfig.GetInput())
		if err != nil {
			return fmt.Errorf("failed to merge input for agent action %s: %w", aConfig.ID, err)
		}
		aConfig.With = mergedInput
		actionCtx := &shared.NormalizationContext{
			WorkflowState:  ctx.WorkflowState,
			WorkflowConfig: ctx.WorkflowConfig,
			TaskConfigs:    ctx.TaskConfigs,
			ParentConfig: map[string]any{
				"id":           config.ID,
				"input":        config.With,
				"instructions": config.Instructions,
				"config":       config.Model.Config,
			},
			CurrentInput:  aConfig.With,
			MergedEnv:     ctx.MergedEnv,
			ChildrenIndex: ctx.ChildrenIndex,
			Variables:     ctx.Variables,  // Copy variables to preserve workflow context
			ParentTask:    ctx.ParentTask, // Preserve parent task to maintain collection context
		}
		if err := n.normalizeAgentActionConfig(aConfig, actionCtx); err != nil {
			return fmt.Errorf("failed to normalize agent action config: %w", err)
		}
	}
	return nil
}

// normalizeAgentActionConfig normalizes an individual agent action configuration
func (n *AgentNormalizer) normalizeAgentActionConfig(
	config *agent.ActionConfig,
	ctx *shared.NormalizationContext,
) error {
	if config == nil {
		return nil
	}
	n.prepareActionVariables(ctx, config)
	return n.applyActionTemplateNormalization(config, ctx)
}

// prepareActionVariables seeds normalization context variables for agent actions.
func (n *AgentNormalizer) prepareActionVariables(
	ctx *shared.NormalizationContext,
	config *agent.ActionConfig,
) {
	if ctx == nil {
		return
	}
	vb := shared.NewVariableBuilder()
	if ctx.CurrentInput != nil {
		vb.AddCurrentInputToVariables(ctx.GetVariables(), ctx.CurrentInput)
	}
	if config.With != nil {
		if ctx.CurrentInput == nil {
			ctx.CurrentInput = config.With
		}
		vb.AddCurrentInputToVariables(ctx.GetVariables(), config.With)
	}
	vars := ctx.GetVariables()
	vars["task"] = n.buildTaskMetadata(config.ID, "agent_action", config)
}

// applyActionTemplateNormalization parses non-runtime fields of an action config.
func (n *AgentNormalizer) applyActionTemplateNormalization(
	config *agent.ActionConfig,
	ctx *shared.NormalizationContext,
) error {
	context := ctx.BuildTemplateContext()
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert action config to map: %w", err)
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, skipActionRuntimeField)
	if err != nil {
		return fmt.Errorf("failed to normalize action config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update action config from normalized map: %w", err)
	}
	return nil
}

// skipActionRuntimeField reports whether an action field must avoid template parsing.
func skipActionRuntimeField(key string) bool {
	return key == "input" || key == "output" || key == "with" || key == "prompt"
}
