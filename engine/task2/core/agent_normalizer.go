package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task2/shared"
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
	// Add config-specific fields based on type
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

	// Build fresh template context with up-to-date workflow state
	tplCtx := ctx.BuildTemplateContext()

	// Re-parse agent-level With
	parsedWith, err := n.parseInputTemplates(agentCfg.With, tplCtx)
	if err != nil {
		return fmt.Errorf("runtime template parse (agent.With) failed: %w", err)
	}
	agentCfg.With = parsedWith

	// Re-parse action-level With and Prompt
	// If actionID is specified, only reparse that specific action
	// Otherwise, reparse all actions (backward compatibility)
	if len(agentCfg.Actions) > 0 {
		for _, action := range agentCfg.Actions {
			if action == nil {
				continue
			}

			// Skip if we're targeting a specific action and this isn't it
			if actionID != "" && action.ID != actionID {
				continue
			}

			// Re-parse action.With
			parsedActionWith, err := n.parseInputTemplates(action.With, tplCtx)
			if err != nil {
				return fmt.Errorf(
					"runtime template parse (action.With, action %s) failed: %w",
					action.ID, err,
				)
			}
			action.With = parsedActionWith

			// Re-parse action.Prompt to resolve collection variables
			if action.Prompt != "" {
				parsedPromptAny, err := n.templateEngine.ParseAny(action.Prompt, tplCtx)
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
			}
		}
	}

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
	// Merge environment variables across workflow -> task -> agent levels
	mergedEnv := n.envMerger.MergeThreeLevels(
		ctx.WorkflowConfig,
		ctx.TaskConfig,
		config.Env, // Agent's environment overrides task and workflow
	)

	// Update context with merged environment for template processing
	ctx.MergedEnv = mergedEnv

	// Set current input if not already set
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}

	// Refresh `.task` so that templates inside an agent (or its actions)
	// still see the expected workflow-level variables (.workflow, .task)
	// and that `.task` now points to *this* agent instead of its parent.
	if ctx != nil {
		vars := ctx.GetVariables()
		vars["task"] = n.buildTaskMetadata(config.ID, "agent", config)
	}

	// Build template context *after* the injection above
	context := ctx.BuildTemplateContext()

	// Lazily parse templates inside agent-level With block
	parsedWith, err := n.parseInputTemplates(config.With, context)
	if err != nil {
		return fmt.Errorf("failed to parse agent input templates: %w", err)
	}
	config.With = parsedWith

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
				// Keep key name `config` for backward template compatibility,
				// but now it carries the resolved provider configuration.
				"config": config.Model.Config,
			},
			CurrentInput:  aConfig.With,
			MergedEnv:     ctx.MergedEnv,
			ChildrenIndex: ctx.ChildrenIndex,
			Variables:     ctx.Variables,  // Copy variables to preserve workflow context
			ParentTask:    ctx.ParentTask, // Preserve parent task to maintain collection context
		}
		// Normalize the action config
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
	// Ensure template context has a stable `.input` map for prompts:
	// 1) Seed from ctx.CurrentInput (task-level With) when available
	// 2) Overlay action-level With when provided
	vb := shared.NewVariableBuilder()
	if ctx.CurrentInput != nil {
		vb.AddCurrentInputToVariables(ctx.GetVariables(), ctx.CurrentInput)
	}
	if config.With != nil {
		// Also set/override from action-level With
		if ctx.CurrentInput == nil {
			ctx.CurrentInput = config.With
		}
		vb.AddCurrentInputToVariables(ctx.GetVariables(), config.With)
	}

	// Make sure `.task` inside the action refers to *this* action.
	if ctx != nil {
		vars := ctx.GetVariables()
		vars["task"] = n.buildTaskMetadata(config.ID, "agent_action", config)
	}

	// Build template context
	context := ctx.BuildTemplateContext()

	// Do NOT parse action-level With at config time.
	// Action inputs may reference runtime-only variables (e.g., .item, .index, .output, .tasks).
	// They are re-parsed at runtime in ExecuteTask.reparseTaskWith.

	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert action config to map: %w", err)
	}
	// Apply template processing with appropriate filters
	// Skip input, output, with, and prompt fields during action normalization
	// Action prompts must ALWAYS be parsed at runtime via ReparseInput because:
	// 1. They may reference .input which comes from the task's resolved With block
	// 2. They may reference .item/.index in collection contexts
	// 3. They may reference .tasks.* which is only available at runtime
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		// Always skip runtime-only fields
		if k == "input" || k == "output" || k == "with" || k == "prompt" {
			return true
		}
		return false
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
