package normalizer

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
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
	transformCtx["output"] = taskOutput

	// Apply output transformation using the normalizer's template engine
	transformedOutput := make(core.Output)
	for key, value := range *outputsConfig {
		result, err := n.normalizer.engine.ParseMap(value, transformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to transform output field %s: %w", key, err)
		}
		transformedOutput[key] = result
	}

	return &transformedOutput, nil
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
