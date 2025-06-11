package normalizer

import (
	"context"
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/tplengine"
)

// CollectionNormalizer handles template evaluation and parsing for collection tasks
type CollectionNormalizer struct {
	engine     *tplengine.TemplateEngine
	textEngine *tplengine.TemplateEngine
	converter  *TypeConverter
	filterEval *FilterEvaluator
}

// NewCollectionNormalizer creates a new collection normalizer
func NewCollectionNormalizer() *CollectionNormalizer {
	return &CollectionNormalizer{
		engine:     tplengine.NewEngine(tplengine.FormatJSON),
		textEngine: tplengine.NewEngine(tplengine.FormatText),
		converter:  NewTypeConverter(),
		filterEval: NewFilterEvaluator(),
	}
}

// ExpandCollectionItems evaluates the 'items' template expression and converts the result
// into a slice of items that can be iterated over
func (cn *CollectionNormalizer) ExpandCollectionItems(
	_ context.Context,
	config *task.CollectionConfig,
	templateContext map[string]any,
) ([]any, error) {
	if config.Items == "" {
		return nil, fmt.Errorf("collection config: items field is required")
	}

	// For simple template expressions, use ParseMap directly
	if tplengine.HasTemplate(config.Items) {
		itemsValue, err := cn.engine.ParseMap(config.Items, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate items expression: %w", err)
		}

		// Convert to a slice of items
		items := cn.converter.ConvertToSlice(itemsValue)
		return items, nil
	}

	// For static JSON arrays/objects, use ProcessString to parse the JSON
	result, err := cn.engine.ProcessString(config.Items, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process items expression: %w", err)
	}

	// Use the JSON result if available, otherwise fall back to text
	var itemsValue any
	if result.JSON != nil {
		itemsValue = result.JSON
	} else {
		itemsValue = result.Text
	}

	// Convert to a slice of items
	items := cn.converter.ConvertToSlice(itemsValue)
	return items, nil
}

// FilterCollectionItems filters items based on the filter expression
func (cn *CollectionNormalizer) FilterCollectionItems(
	_ context.Context,
	config *task.CollectionConfig,
	items []any,
	templateContext map[string]any,
) ([]any, error) {
	if config.Filter == "" {
		// No filter, return all items
		return items, nil
	}
	var filteredItems []any
	for i, item := range items {
		// Create context with item and index variables
		filterContext := cn.CreateItemContext(templateContext, config, item, i)
		// Evaluate filter expression
		include, err := cn.filterEval.EvaluateFilter(config.Filter, filterContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
		}
		if include {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems, nil
}

// CreateItemContext creates a template context for a specific collection item
func (cn *CollectionNormalizer) CreateItemContext(
	baseContext map[string]any,
	config *task.CollectionConfig,
	item any,
	index int,
) map[string]any {
	itemContext := make(map[string]any)

	// Copy base context
	for k, v := range baseContext {
		itemContext[k] = v
	}

	// Add item-specific variables
	itemVar := config.GetItemVar()
	if itemVar == "" {
		itemVar = "item" // Default fallback
	}
	indexVar := config.GetIndexVar()
	if indexVar == "" {
		indexVar = "index" // Default fallback
	}

	itemContext[itemVar] = item
	itemContext[indexVar] = index

	return itemContext
}

// CreateProgressContext creates a template context enriched with progress information
func (cn *CollectionNormalizer) CreateProgressContext(
	baseContext map[string]any,
	progressInfo *task.ProgressInfo,
) map[string]any {
	contextWithProgress := make(map[string]any)

	// Copy base context
	maps.Copy(contextWithProgress, baseContext)

	// Add progress info
	contextWithProgress["progress"] = map[string]any{
		"total_children":  progressInfo.TotalChildren,
		"completed_count": progressInfo.CompletedCount,
		"failed_count":    progressInfo.FailedCount,
		"running_count":   progressInfo.RunningCount,
		"pending_count":   progressInfo.PendingCount,
		"completion_rate": progressInfo.CompletionRate,
		"failure_rate":    progressInfo.FailureRate,
		"overall_status":  string(progressInfo.OverallStatus),
		"status_counts":   progressInfo.StatusCounts,
		"has_failures":    progressInfo.HasFailures(),
		"is_all_complete": progressInfo.IsAllComplete(),
	}

	// Add summary alias for backward compatibility
	contextWithProgress["summary"] = contextWithProgress["progress"]

	return contextWithProgress
}

// ApplyTemplateToConfig applies item-specific context to a task configuration and returns a new config
func (cn *CollectionNormalizer) ApplyTemplateToConfig(
	config *task.Config,
	itemContext map[string]any,
) (*task.Config, error) {
	// Create a deep copy to avoid mutating the original config
	newConfig := cn.deepCopyConfig(config)

	// Use the template engine to process the configuration
	engine := tplengine.NewEngine(tplengine.FormatText)

	// Apply templates to different parts of the configuration
	if err := cn.applyActionTemplate(newConfig, itemContext, engine); err != nil {
		return nil, err
	}

	if err := cn.applyWithTemplate(newConfig, itemContext, engine); err != nil {
		return nil, err
	}

	if err := cn.applyEnvTemplate(newConfig, itemContext, engine); err != nil {
		return nil, err
	}

	if err := cn.applyAgentTemplate(newConfig, itemContext, engine); err != nil {
		return nil, err
	}

	if err := cn.applyToolTemplate(newConfig, itemContext, engine); err != nil {
		return nil, err
	}

	return newConfig, nil
}

// applyActionTemplate applies template to the action field
func (cn *CollectionNormalizer) applyActionTemplate(
	config *task.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if config.Action != "" {
		processedAction, err := engine.RenderString(config.Action, itemContext)
		if err != nil {
			return fmt.Errorf("failed to apply template to action: %w", err)
		}
		config.Action = processedAction
	}
	return nil
}

// applyWithTemplate applies templates to the 'with' input parameters
func (cn *CollectionNormalizer) applyWithTemplate(
	config *task.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if config.With == nil {
		return nil
	}

	processedWith := make(map[string]any)
	for k, v := range *config.With {
		if strVal, ok := v.(string); ok {
			// Apply template to string values
			renderedVal, err := engine.RenderString(strVal, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to with parameter '%s': %w", k, err)
			}
			processedWith[k] = renderedVal
		} else {
			// For non-string values, use ParseMap to handle nested structures
			processedVal, err := engine.ParseMap(v, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to with parameter '%s': %w", k, err)
			}
			processedWith[k] = processedVal
		}
	}
	*config.With = processedWith
	return nil
}

// applyEnvTemplate applies templates to environment variables
func (cn *CollectionNormalizer) applyEnvTemplate(
	config *task.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if config.Env == nil {
		return nil
	}

	processedEnv, err := engine.ParseMap(*config.Env, itemContext)
	if err != nil {
		return fmt.Errorf("failed to apply template to env variables: %w", err)
	}

	if envMap, ok := processedEnv.(map[string]any); ok {
		envStrMap := make(map[string]string)
		for k, v := range envMap {
			if strVal, ok := v.(string); ok {
				envStrMap[k] = strVal
			} else {
				envStrMap[k] = fmt.Sprintf("%v", v)
			}
		}
		envMapPtr := core.EnvMap(envStrMap)
		config.Env = &envMapPtr
	}
	return nil
}

// applyAgentTemplate applies templates to agent configuration
func (cn *CollectionNormalizer) applyAgentTemplate(
	config *task.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if config.Agent != nil {
		err := cn.applyTemplateToAgent(config.Agent, itemContext, engine)
		if err != nil {
			return fmt.Errorf("failed to apply template to agent: %w", err)
		}
	}
	return nil
}

// applyToolTemplate applies templates to tool configuration
func (cn *CollectionNormalizer) applyToolTemplate(
	config *task.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if config.Tool != nil {
		err := cn.applyTemplateToTool(config.Tool, itemContext, engine)
		if err != nil {
			return fmt.Errorf("failed to apply template to tool: %w", err)
		}
	}
	return nil
}

// applyTemplateToAgent applies templates to agent configuration
func (cn *CollectionNormalizer) applyTemplateToAgent(
	agentConfig any,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	// Handle agent config through reflection since it could be a pointer or embedded
	agentPtr, ok := agentConfig.(*agent.Config)
	if !ok {
		return fmt.Errorf("agent config is not of expected type")
	}

	// Apply templates to different parts of agent configuration
	if err := cn.applyAgentInstructionsTemplate(agentPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyAgentActionsTemplate(agentPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyAgentWithTemplate(agentPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyAgentEnvTemplate(agentPtr, itemContext, engine); err != nil {
		return err
	}

	return nil
}

// applyAgentInstructionsTemplate applies template to agent instructions
func (cn *CollectionNormalizer) applyAgentInstructionsTemplate(
	agentPtr *agent.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if agentPtr.Instructions != "" {
		processedInstructions, err := engine.RenderString(agentPtr.Instructions, itemContext)
		if err != nil {
			return fmt.Errorf("failed to apply template to agent instructions: %w", err)
		}
		agentPtr.Instructions = processedInstructions
	}
	return nil
}

// applyAgentActionsTemplate applies templates to agent actions
func (cn *CollectionNormalizer) applyAgentActionsTemplate(
	agentPtr *agent.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	for i, action := range agentPtr.Actions {
		if err := cn.applyActionPromptTemplate(action, itemContext, engine, i); err != nil {
			return err
		}

		if err := cn.applyActionWithTemplate(action, itemContext, engine, i); err != nil {
			return err
		}
	}
	return nil
}

// applyActionPromptTemplate applies template to action prompt
func (cn *CollectionNormalizer) applyActionPromptTemplate(
	action *agent.ActionConfig,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
	actionIndex int,
) error {
	if action.Prompt != "" {
		processedPrompt, err := engine.RenderString(action.Prompt, itemContext)
		if err != nil {
			return fmt.Errorf("failed to apply template to action %d prompt: %w", actionIndex, err)
		}
		action.Prompt = processedPrompt
	}
	return nil
}

// applyActionWithTemplate applies templates to action's 'with' parameters
func (cn *CollectionNormalizer) applyActionWithTemplate(
	action *agent.ActionConfig,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
	actionIndex int,
) error {
	if action.With == nil {
		return nil
	}

	processedWith := make(map[string]any)
	for k, v := range *action.With {
		if strVal, ok := v.(string); ok {
			renderedVal, err := engine.RenderString(strVal, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to action %d with parameter '%s': %w", actionIndex, k, err)
			}
			processedWith[k] = renderedVal
		} else {
			processedVal, err := engine.ParseMap(v, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to action %d with parameter '%s': %w", actionIndex, k, err)
			}
			processedWith[k] = processedVal
		}
	}
	*action.With = processedWith
	return nil
}

// applyAgentWithTemplate applies templates to agent's 'with' parameters
func (cn *CollectionNormalizer) applyAgentWithTemplate(
	agentPtr *agent.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if agentPtr.With == nil {
		return nil
	}

	processedWith := make(map[string]any)
	for k, v := range *agentPtr.With {
		if strVal, ok := v.(string); ok {
			renderedVal, err := engine.RenderString(strVal, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to agent with parameter '%s': %w", k, err)
			}
			processedWith[k] = renderedVal
		} else {
			processedVal, err := engine.ParseMap(v, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to agent with parameter '%s': %w", k, err)
			}
			processedWith[k] = processedVal
		}
	}
	*agentPtr.With = processedWith
	return nil
}

// applyAgentEnvTemplate applies templates to agent's environment variables
func (cn *CollectionNormalizer) applyAgentEnvTemplate(
	agentPtr *agent.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if agentPtr.Env == nil {
		return nil
	}

	processedEnv, err := engine.ParseMap(*agentPtr.Env, itemContext)
	if err != nil {
		return fmt.Errorf("failed to apply template to agent env variables: %w", err)
	}

	if envMap, ok := processedEnv.(map[string]any); ok {
		envStrMap := make(map[string]string)
		for k, v := range envMap {
			if strVal, ok := v.(string); ok {
				envStrMap[k] = strVal
			} else {
				envStrMap[k] = fmt.Sprintf("%v", v)
			}
		}
		envMapPtr := core.EnvMap(envStrMap)
		agentPtr.Env = &envMapPtr
	}
	return nil
}

// applyTemplateToTool applies templates to tool configuration
func (cn *CollectionNormalizer) applyTemplateToTool(
	toolConfig any,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	// Handle tool config through reflection since it could be a pointer or embedded
	toolPtr, ok := toolConfig.(*tool.Config)
	if !ok {
		return fmt.Errorf("tool config is not of expected type")
	}

	// Apply templates to different parts of tool configuration
	if err := cn.applyToolDescriptionTemplate(toolPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyToolExecuteTemplate(toolPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyToolWithTemplate(toolPtr, itemContext, engine); err != nil {
		return err
	}

	if err := cn.applyToolEnvTemplate(toolPtr, itemContext, engine); err != nil {
		return err
	}

	return nil
}

// applyToolDescriptionTemplate applies template to tool description
func (cn *CollectionNormalizer) applyToolDescriptionTemplate(
	toolPtr *tool.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if toolPtr.Description != "" {
		processedDescription, err := engine.RenderString(toolPtr.Description, itemContext)
		if err != nil {
			return fmt.Errorf("failed to apply template to tool description: %w", err)
		}
		toolPtr.Description = processedDescription
	}
	return nil
}

// applyToolExecuteTemplate applies template to tool execute command
func (cn *CollectionNormalizer) applyToolExecuteTemplate(
	toolPtr *tool.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if toolPtr.Execute != "" {
		processedExecute, err := engine.RenderString(toolPtr.Execute, itemContext)
		if err != nil {
			return fmt.Errorf("failed to apply template to tool execute: %w", err)
		}
		toolPtr.Execute = processedExecute
	}
	return nil
}

// applyToolWithTemplate applies templates to tool's 'with' parameters
func (cn *CollectionNormalizer) applyToolWithTemplate(
	toolPtr *tool.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if toolPtr.With == nil {
		return nil
	}

	processedWith := make(map[string]any)
	for k, v := range *toolPtr.With {
		if strVal, ok := v.(string); ok {
			renderedVal, err := engine.RenderString(strVal, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to tool with parameter '%s': %w", k, err)
			}
			processedWith[k] = renderedVal
		} else {
			processedVal, err := engine.ParseMap(v, itemContext)
			if err != nil {
				return fmt.Errorf("failed to apply template to tool with parameter '%s': %w", k, err)
			}
			processedWith[k] = processedVal
		}
	}
	*toolPtr.With = processedWith
	return nil
}

// applyToolEnvTemplate applies templates to tool's environment variables
func (cn *CollectionNormalizer) applyToolEnvTemplate(
	toolPtr *tool.Config,
	itemContext map[string]any,
	engine *tplengine.TemplateEngine,
) error {
	if toolPtr.Env == nil {
		return nil
	}

	processedEnv, err := engine.ParseMap(*toolPtr.Env, itemContext)
	if err != nil {
		return fmt.Errorf("failed to apply template to tool env variables: %w", err)
	}

	if envMap, ok := processedEnv.(map[string]any); ok {
		envStrMap := make(map[string]string)
		for k, v := range envMap {
			if strVal, ok := v.(string); ok {
				envStrMap[k] = strVal
			} else {
				envStrMap[k] = fmt.Sprintf("%v", v)
			}
		}
		envMapPtr := core.EnvMap(envStrMap)
		toolPtr.Env = &envMapPtr
	}
	return nil
}

// deepCopyConfig creates a deep copy of a task configuration
func (cn *CollectionNormalizer) deepCopyConfig(config *task.Config) *task.Config {
	newConfig := *config // shallow copy

	// Deep copy With map if it exists
	if config.With != nil {
		withCopy := make(core.Input)
		for k, v := range *config.With {
			withCopy[k] = v
		}
		newConfig.With = &withCopy
	}

	// Deep copy Env map if it exists
	if config.Env != nil {
		envCopy := make(core.EnvMap)
		for k, v := range *config.Env {
			envCopy[k] = v
		}
		newConfig.Env = &envCopy
	}

	return &newConfig
}
