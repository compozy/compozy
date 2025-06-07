package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// -----------------------------------------------------------------------------
// NormalizeConfig
// -----------------------------------------------------------------------------

type NormalizeConfigInput struct {
	WorkflowState  *workflow.State  `json:"workflow_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type NormalizeConfig struct {
	normalizer *normalizer.ConfigNormalizer
}

func NewNormalizeConfig() *NormalizeConfig {
	return &NormalizeConfig{
		normalizer: normalizer.NewConfigNormalizer(),
	}
}

func (uc *NormalizeConfig) Execute(_ context.Context, input *NormalizeConfigInput) error {
	taskConfigs := normalizer.BuildTaskConfigsMap(input.WorkflowConfig.Tasks)
	taskConfig := input.TaskConfig
	err := uc.normalizer.NormalizeTask(
		input.WorkflowState,
		input.WorkflowConfig,
		taskConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to normalize task %s for workflow %s: %w",
			input.TaskConfig.ID, input.WorkflowConfig.ID, err)
	}
	return uc.normalizeComponent(input, taskConfigs)
}

func (uc *NormalizeConfig) normalizeComponent(
	input *NormalizeConfigInput,
	taskConfigs map[string]*task.Config,
) error {
	agentConfig := input.TaskConfig.GetAgent()
	toolConfig := input.TaskConfig.GetTool()
	switch {
	case agentConfig != nil:
		return uc.normalizeAgent(input, agentConfig, taskConfigs)
	case toolConfig != nil:
		return uc.normalizeTool(input, toolConfig, taskConfigs)
	default:
		return nil
	}
}

func (uc *NormalizeConfig) normalizeAgent(
	input *NormalizeConfigInput,
	agentConfig *agent.Config,
	taskConfigs map[string]*task.Config,
) error {
	err := uc.normalizer.NormalizeAgentComponent(
		input.WorkflowState,
		input.WorkflowConfig,
		input.TaskConfig,
		agentConfig,
		taskConfigs,
	)
	if err != nil {
		return fmt.Errorf("failed to normalize agent component for task %s: %w",
			input.TaskConfig.ID, err)
	}
	return nil
}

func (uc *NormalizeConfig) normalizeTool(
	input *NormalizeConfigInput,
	toolConfig *tool.Config,
	taskConfigs map[string]*task.Config,
) error {
	err := uc.normalizer.NormalizeToolComponent(
		input.WorkflowState,
		input.WorkflowConfig,
		input.TaskConfig,
		toolConfig,
		taskConfigs,
	)
	if err != nil {
		return fmt.Errorf("failed to normalize tool component for task %s: %w",
			input.TaskConfig.ID, err)
	}
	return nil
}
