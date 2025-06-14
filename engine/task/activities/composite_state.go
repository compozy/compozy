package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateCompositeStateLabel = "CreateCompositeState"

type CreateCompositeStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type CreateCompositeState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
}

// NewCreateCompositeState creates a new CreateCompositeState activity
func NewCreateCompositeState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
) *CreateCompositeState {
	configManager := services.NewConfigManager(configStore, cwd)
	return &CreateCompositeState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configManager),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configManager),
	}
}

func (a *CreateCompositeState) Run(ctx context.Context, input *CreateCompositeStateInput) (*task.State, error) {
	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeComposite {
		return nil, fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	// Load workflow context
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Create state (ConfigManager handles composite config preparation)
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}
	// Create child tasks
	if err := a.createChildTasksUC.Execute(ctx, &uc.CreateChildTasksInput{
		ParentStateID:  state.TaskExecID,
		WorkflowExecID: input.WorkflowExecID,
		WorkflowID:     input.WorkflowID,
	}); err != nil {
		return nil, fmt.Errorf("failed to create child tasks: %w", err)
	}
	return state, nil
}
