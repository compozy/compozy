package normalizer

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// ConfigNormalizer handles the normalization of configurations along with environment merging.
type ConfigNormalizer struct {
	normalizer *Normalizer
	envMerger  *core.EnvMerger
}

// NewConfigNormalizer creates a new configuration normalizer.
func NewConfigNormalizer() *ConfigNormalizer {
	return &ConfigNormalizer{
		normalizer: New(),
		envMerger:  &core.EnvMerger{},
	}
}

// BuildTaskConfigsMap converts a slice of task.Config into a map keyed by task ID.
func BuildTaskConfigsMap(taskConfigSlice []task.Config) map[string]*task.Config {
	configs := make(map[string]*task.Config)
	for i := range taskConfigSlice {
		tc := &taskConfigSlice[i]
		configs[tc.ID] = tc
	}
	return configs
}

// NormalizeTask normalizes a task configuration after merging environments (workflow -> task).
// It modifies taskConfig in place and returns the base merged environment.
func (n *ConfigNormalizer) NormalizeTask(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	baseEnv, err := n.envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		taskConfig.GetEnv(),
	)
	if err != nil {
		return fmt.Errorf("failed to merge base environments for task %s: %w", taskConfig.ID, err)
	}
	allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    allTaskConfigsMap,
		ParentConfig: map[string]any{
			"id":     workflowState.WorkflowID,
			"input":  workflowState.Input,
			"output": workflowState.Output,
		},
		MergedEnv: &baseEnv,
	}
	taskConfig.Env = &baseEnv
	if err := n.normalizer.NormalizeTaskConfig(taskConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize task config for %s: %w", taskConfig.ID, err)
	}
	return nil
}

// NormalizeAgentComponent normalizes an agent component configuration after
// merging environments (workflow -> task -> agent).
// It modifies agentConfig in place and returns the fully merged environment.
func (n *ConfigNormalizer) NormalizeAgentComponent(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	agentConfig *agent.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	mergedEnv, err := n.envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		taskConfig.GetEnv(),
		agentConfig.GetEnv(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to merge environments for agent %s in task %s: %w",
			agentConfig.ID,
			taskConfig.ID,
			err,
		)
	}

	// Build complete parent context with all task config properties
	parentConfig, err := core.AsMapDefault(taskConfig)
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}

	// Add runtime state if available
	if workflowState.Tasks != nil {
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			parentConfig["input"] = taskState.Input
			parentConfig["output"] = taskState.Output
		}
	}

	mergedInput, err := taskConfig.GetInput().Merge(agentConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for agent %s in task %s: %w", agentConfig.ID, taskConfig.ID, err)
	}
	agentConfig.With = mergedInput
	agentConfig.Env = &mergedEnv
	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    allTaskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   agentConfig.With,
		MergedEnv:      &mergedEnv,
	}
	if err := n.normalizer.NormalizeAgentConfig(agentConfig, normCtx, taskConfig.Action); err != nil {
		return fmt.Errorf("failed to normalize agent config for %s: %w", agentConfig.ID, err)
	}

	// Process memory configuration for agents loaded via references
	// This ensures memory references are created during normalization phase
	if err := agentConfig.NormalizeAndValidateMemoryConfig(); err != nil {
		return fmt.Errorf("failed to process memory config for agent %s: %w", agentConfig.ID, err)
	}

	return nil
}

// NormalizeToolComponent normalizes a tool component configuration after
// merging environments (workflow -> task -> tool).
// It modifies toolConfig in place and returns the fully merged environment.
func (n *ConfigNormalizer) NormalizeToolComponent(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	toolConfig *tool.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	mergedEnv, err := n.envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		taskConfig.GetEnv(),
		toolConfig.GetEnv(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to merge environments for tool %s in task %s: %w",
			toolConfig.ID,
			taskConfig.ID,
			err,
		)
	}
	parentConfig, err := core.AsMapDefault(taskConfig)
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	if workflowState.Tasks != nil {
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			parentConfig["input"] = taskState.Input
			parentConfig["output"] = taskState.Output
		}
	}

	mergedInput, err := taskConfig.GetInput().Merge(toolConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for tool %s in task %s: %w", toolConfig.ID, taskConfig.ID, err)
	}
	toolConfig.With = mergedInput
	toolConfig.Env = &mergedEnv
	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    allTaskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   toolConfig.With,
		MergedEnv:      &mergedEnv,
	}
	if err := n.normalizer.NormalizeToolConfig(toolConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize tool config for %s: %w", toolConfig.ID, err)
	}
	return nil
}

// NormalizeSuccessTransition normalizes a success transition configuration.
func (n *ConfigNormalizer) NormalizeSuccessTransition(
	transition *core.SuccessTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}

	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}

	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output

	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    allTaskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   transition.With,
		MergedEnv:      mergedEnv,
	}

	if err := n.normalizer.NormalizeTransition(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize success transition: %w", err)
	}
	return nil
}

// NormalizeErrorTransition normalizes an error transition configuration.
func (n *ConfigNormalizer) NormalizeErrorTransition(
	transition *core.ErrorTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}

	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}

	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output

	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    allTaskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   transition.With,
		MergedEnv:      mergedEnv,
	}

	if err := n.normalizer.NormalizeErrorTransition(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize error transition: %w", err)
	}
	return nil
}

// NormalizeTaskOutput applies output transformation to task output based on the outputs configuration.
func (n *ConfigNormalizer) NormalizeTaskOutput(
	taskOutput *core.Output,
	outputsConfig *core.Input,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*core.Output, error) {
	if outputsConfig == nil || taskOutput == nil {
		return taskOutput, nil
	}

	// Build context for template evaluation
	taskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

	// Build transformation context
	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
	}

	// Create context with current output available
	transformCtx := n.normalizer.BuildContext(normCtx)
	// Special handling for collection/parallel tasks
	if taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel {
		// Look for the nested outputs map
		if nestedOutputs, ok := (*taskOutput)["outputs"]; ok {
			// Use child outputs map as .output in template context
			transformCtx["output"] = nestedOutputs
		} else {
			// Outputs not yet aggregated, use empty map
			transformCtx["output"] = make(map[string]any)
		}
		// For parent tasks, also add children context at the top level
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			if taskState.CanHaveChildren() && normCtx.ChildrenIndex != nil {
				transformCtx["children"] = n.normalizer.BuildChildrenContext(taskState, normCtx, 0)
			}
		}
	} else {
		// For regular tasks, use the full output
		transformCtx["output"] = taskOutput
	}
	// Apply output transformation using the normalizer's template engine
	transformedOutput, err := n.transformOutputFields(*outputsConfig, transformCtx, "task")
	if err != nil {
		return nil, err
	}
	result := core.Output(transformedOutput)
	return &result, nil
}

// transformOutputFields applies template transformation to output fields using the normalizer's engine
func (n *ConfigNormalizer) transformOutputFields(
	outputsConfig map[string]any,
	transformCtx map[string]any,
	contextName string,
) (map[string]any, error) {
	// Sort keys to ensure deterministic iteration order for Temporal workflows
	keys := make([]string, 0, len(outputsConfig))
	for k := range outputsConfig {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make(map[string]any)
	for _, key := range keys {
		value := outputsConfig[key]
		transformed, err := n.normalizer.engine.ParseMap(value, transformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to transform %s output field %s: %w", contextName, key, err)
		}
		result[key] = transformed
	}
	return result, nil
}

// NormalizeTaskEnvironment only merges environments without processing task config templates
// This is used when we only need environment merging (e.g., for transition normalization)
func (n *ConfigNormalizer) NormalizeTaskEnvironment(
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	baseEnv, err := n.envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		taskConfig.GetEnv(),
	)
	if err != nil {
		return fmt.Errorf("failed to merge base environments for task %s: %w", taskConfig.ID, err)
	}
	taskConfig.Env = &baseEnv
	return nil
}

// NormalizeWorkflowOutput transforms the workflow output using the outputs configuration
func (n *ConfigNormalizer) NormalizeWorkflowOutput(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	outputsConfig *core.Output,
) (*core.Output, error) {
	if outputsConfig == nil {
		return nil, nil
	}
	log := logger.FromContext(ctx)
	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	taskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   workflowState.Input,
		MergedEnv:      &[]core.EnvMap{workflowConfig.GetEnv()}[0],
	}
	transformCtx := n.normalizer.BuildContext(normCtx)
	transformCtx["status"] = workflowState.Status
	transformCtx["workflow_id"] = workflowState.WorkflowID
	transformCtx["workflow_exec_id"] = workflowState.WorkflowExecID
	if workflowState.Error != nil {
		transformCtx["error"] = workflowState.Error
	}

	// Log template processing start at debug level
	log.Debug("Starting workflow output template processing",
		"workflow_id", workflowState.WorkflowID,
		"workflow_exec_id", workflowState.WorkflowExecID,
		"task_count", len(workflowState.Tasks),
		"output_fields", len(*outputsConfig))
	// Apply output transformation using the normalizer's template engine
	if len(*outputsConfig) == 0 {
		return &core.Output{}, nil
	}
	transformedOutput, err := n.transformOutputFields(outputsConfig.AsMap(), transformCtx, "workflow")
	if err != nil {
		log.Error("Failed to transform workflow output",
			"workflow_id", workflowState.WorkflowID,
			"error", err)
		return nil, err
	}
	log.Debug("Successfully transformed workflow output",
		"workflow_id", workflowState.WorkflowID,
		"fields_count", len(transformedOutput))
	finalOutput := core.Output(transformedOutput)
	return &finalOutput, nil
}
