package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const ExecuteParallelTaskLabel = "ExecuteParallelTask"

type ExecuteParallelTaskInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	ParentState    *task.State  `json:"parent_state"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteParallelTask struct {
	loadWorkflowUC *uc.LoadWorkflow
	executeTaskUC  *uc.ExecuteTask
	taskRepo       task.Repository
}

// NewExecuteParallelTask creates a new ExecuteParallelTask activity
func NewExecuteParallelTask(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *ExecuteParallelTask {
	return &ExecuteParallelTask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC:  uc.NewExecuteTask(runtime),
		taskRepo:       taskRepo,
	}
}

func (a *ExecuteParallelTask) Run(ctx context.Context, input *ExecuteParallelTaskInput) (*task.SubtaskResponse, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Normalize task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	taskConfig := input.TaskConfig
	taskType := taskConfig.Type
	// TODO: we need to support parallel task execution here too
	if taskType != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
	output, err := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig: taskConfig,
	})
	if err != nil {
		return &task.SubtaskResponse{
			TaskID: taskConfig.ID,
			Output: nil,
			Status: core.StatusFailed,
			Error:  core.NewError(err, "subtask_execution_failed", nil),
		}, err
	}
	return &task.SubtaskResponse{
		TaskID: taskConfig.ID,
		Output: output,
		Status: core.StatusSuccess,
		Error:  nil,
	}, nil
}
