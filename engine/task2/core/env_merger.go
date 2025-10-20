package core

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// EnvMerger handles environment variable merging across workflow, task, and component levels
type EnvMerger struct{}

// NewEnvMerger creates a new environment merger
func NewEnvMerger() *EnvMerger {
	return &EnvMerger{}
}

// mergeEnvMaps is a helper that merges multiple environment maps
// Later maps override earlier ones
func (em *EnvMerger) mergeEnvMaps(envMaps ...*core.EnvMap) *core.EnvMap {
	toMerge := make([]map[string]string, 0, len(envMaps))
	for _, envMap := range envMaps {
		if envMap != nil {
			toMerge = append(toMerge, *envMap)
		}
	}
	merged := core.CopyMaps(toMerge...)
	result := core.EnvMap(merged)
	return &result
}

// MergeWorkflowToTask merges workflow environment variables into task config
func (em *EnvMerger) MergeWorkflowToTask(
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *core.EnvMap {
	var workflowEnv, taskEnv *core.EnvMap
	if workflowConfig != nil && workflowConfig.Opts.Env != nil {
		workflowEnv = workflowConfig.Opts.Env
	}
	if taskConfig != nil && taskConfig.Env != nil {
		taskEnv = taskConfig.Env
	}
	// Task env overrides workflow env
	merged := em.mergeEnvMaps(workflowEnv, taskEnv)
	// Update task config with merged env
	if taskConfig != nil {
		taskConfig.Env = merged
	}
	return merged
}

// MergeThreeLevels merges environment variables across three levels: workflow -> task -> component
func (em *EnvMerger) MergeThreeLevels(
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	componentEnv *core.EnvMap,
) *core.EnvMap {
	var workflowEnv, taskEnv *core.EnvMap
	if workflowConfig != nil && workflowConfig.Opts.Env != nil {
		workflowEnv = workflowConfig.Opts.Env
	}
	if taskConfig != nil && taskConfig.Env != nil {
		taskEnv = taskConfig.Env
	}
	// Component env overrides task env which overrides workflow env
	return em.mergeEnvMaps(workflowEnv, taskEnv, componentEnv)
}

// MergeForComponent merges environment for a specific component type
func (em *EnvMerger) MergeForComponent(
	baseEnv *core.EnvMap,
	componentEnv *core.EnvMap,
) *core.EnvMap {
	// Component env overrides base env
	return em.mergeEnvMaps(baseEnv, componentEnv)
}
