package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/normalizer"
	"github.com/compozy/compozy/pkg/tplengine"
)

type ExecuteTaskInput struct {
	TaskConfig *task.Config `json:"task_config"`
}

type ExecuteTask struct {
	runtime *runtime.Manager
}

func NewExecuteTask(runtime *runtime.Manager) *ExecuteTask {
	return &ExecuteTask{runtime: runtime}
}

func (uc *ExecuteTask) Execute(ctx context.Context, input *ExecuteTaskInput) (*core.Output, error) {
	agentConfig := input.TaskConfig.Agent
	toolConfig := input.TaskConfig.Tool
	var result *core.Output
	var err error
	switch {
	case agentConfig != nil:
		actionID := input.TaskConfig.Action
		// TODO: remove this when do automatically selection for action
		if actionID == "" {
			return nil, fmt.Errorf("action ID is required for agent")
		}
		result, err = uc.executeAgent(ctx, agentConfig, actionID, input.TaskConfig.With)
		if err != nil {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
		return result, nil
	case toolConfig != nil:
		result, err = uc.executeTool(ctx, input.TaskConfig, toolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("no component specified for execution")
	}
}

func (uc *ExecuteTask) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
	taskInput *core.Input,
) (*core.Output, error) {
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}

	// CRITICAL FIX: Normalize agent action with task input context
	// This resolves templates like {{ .city }} in agent action prompts (already processed by TaskTemplateEvaluator)
	if taskInput != nil {
		// Filter out unresolved workflow-level references that contain template syntax
		// These should not be passed to agent action normalization as they cause parsing errors
		filteredInput := uc.filterWorkflowReferences(taskInput)

		// Only process agent action templates if all referenced input fields are available
		// This prevents template processing errors when the action references filtered-out fields
		processable := uc.actionPromptIsProcessable(actionConfig.Prompt, filteredInput)

		if processable {
			norm := normalizer.New()

			// Create normalization context with the filtered task input
			// The TaskTemplateEvaluator has already converted {{ .input.city }} to {{ .city }}
			// So we need to provide the input fields at the top level of the context
			normCtx := &normalizer.NormalizationContext{
				CurrentInput: filteredInput,
			}

			// Build context and add task input fields to top level
			context := norm.BuildContext(normCtx)

			// Add all filtered task input fields to the top level so {{ .city }} resolves correctly
			for key, value := range *filteredInput {
				context[key] = value
			}

			// Process the action prompt template directly with our enhanced context
			templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
			processedPrompt, err := templateEngine.ParseMap(actionConfig.Prompt, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process action prompt template: %w", err)
			}

			// Update the action config with the processed prompt
			if promptStr, ok := processedPrompt.(string); ok {
				actionConfig.Prompt = promptStr
			}
		}
	}

	llmService := llm.NewService(uc.runtime, agentConfig, actionConfig)
	result, err := llmService.GenerateContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return result, nil
}

// filterWorkflowReferences filters out unresolved workflow-level template references
// that contain template syntax which would cause parsing errors in agent action normalization
func (uc *ExecuteTask) filterWorkflowReferences(input *core.Input) *core.Input {
	if input == nil {
		return &core.Input{}
	}

	filtered := make(core.Input)
	for key, value := range *input {
		if !uc.containsUnresolvedTemplateReferences(value) {
			filtered[key] = value
		}
	}

	return &filtered
}

// containsUnresolvedTemplateReferences checks if a value contains unresolved template syntax
// that should not be processed during agent action normalization
func (uc *ExecuteTask) containsUnresolvedTemplateReferences(value any) bool {
	switch v := value.(type) {
	case string:
		// Check if the string contains unresolved template references
		return strings.Contains(v, "{{") && strings.Contains(v, "}}")
	case map[string]any:
		// Recursively check map values
		for _, mapValue := range v {
			if uc.containsUnresolvedTemplateReferences(mapValue) {
				return true
			}
		}
	case []any:
		// Recursively check slice values
		for _, sliceValue := range v {
			if uc.containsUnresolvedTemplateReferences(sliceValue) {
				return true
			}
		}
	}
	return false
}

// actionPromptIsProcessable checks if the action prompt can be safely processed
// without causing template parsing errors due to unresolved references
func (uc *ExecuteTask) actionPromptIsProcessable(prompt string, filteredInput *core.Input) bool {
	// If there are no template references, it's always processable
	if !strings.Contains(prompt, "{{") {
		return true
	}

	// For now, allow processing of simple field references like {{ .city }}
	// We'll let the template engine handle any errors during processing
	// The main concern is complex workflow references with brackets like [0] which cause parsing errors

	// Check if the prompt contains problematic bracket syntax that causes parsing errors
	return !strings.Contains(prompt, "[") && !strings.Contains(prompt, "]")
}

func (uc *ExecuteTask) executeTool(
	ctx context.Context,
	taskConfig *task.Config,
	toolConfig *tool.Config,
) (*core.Output, error) {
	tool := llm.NewTool(toolConfig, toolConfig.Env, uc.runtime)
	output, err := tool.Call(ctx, taskConfig.With)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	return output, nil
}
