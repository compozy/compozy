package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks"
	"github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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
	orchestrator *tasks.ConfigOrchestrator
}

func NewNormalizeConfig(ctx context.Context) (*NormalizeConfig, error) {
	tplEngine := tplengine.NewEngine(tplengine.FormatJSON)
	envMerger := core.NewEnvMerger()
	factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
		TemplateEngine: tplEngine,
		EnvMerger:      envMerger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create normalizer factory: %w", err)
	}
	orchestrator, err := tasks.NewConfigOrchestrator(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("failed to create config orchestrator: %w", err)
	}
	return &NormalizeConfig{
		orchestrator: orchestrator,
	}, nil
}

func (uc *NormalizeConfig) Execute(ctx context.Context, input *NormalizeConfigInput) error {
	taskConfigs := tasks.BuildTaskConfigsMap(input.WorkflowConfig.Tasks)
	taskConfig := input.TaskConfig
	err := uc.orchestrator.NormalizeTask(ctx,
		input.WorkflowState,
		input.WorkflowConfig,
		taskConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to normalize task %s for workflow %s: %w",
			input.TaskConfig.ID, input.WorkflowConfig.ID, err)
	}
	return uc.normalizeComponent(ctx, input, taskConfigs)
}

func (uc *NormalizeConfig) normalizeComponent(
	ctx context.Context,
	input *NormalizeConfigInput,
	taskConfigs map[string]*task.Config,
) error {
	agentConfig := input.TaskConfig.GetAgent()
	toolConfig := input.TaskConfig.GetTool()
	switch {
	case agentConfig != nil:
		return uc.normalizeAgent(ctx, input, agentConfig, taskConfigs)
	case toolConfig != nil:
		return uc.normalizeTool(ctx, input, toolConfig, taskConfigs)
	default:
		return nil
	}
}

func (uc *NormalizeConfig) normalizeAgent(
	ctx context.Context,
	input *NormalizeConfigInput,
	agentConfig *agent.Config,
	taskConfigs map[string]*task.Config,
) error {
	err := uc.orchestrator.NormalizeAgentComponent(
		ctx,
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
	ctx context.Context,
	input *NormalizeConfigInput,
	toolConfig *tool.Config,
	taskConfigs map[string]*task.Config,
) error {
	err := uc.orchestrator.NormalizeToolComponent(
		ctx,
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
