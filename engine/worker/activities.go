package worker

import (
	"context"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	tkfacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type Activities struct {
	projectConfig *project.Config
	workflows     []*workflow.Config
	workflowRepo  workflow.Repository
	taskRepo      task.Repository
	runtime       *runtime.Manager
}

func NewActivities(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
) *Activities {
	return &Activities{
		projectConfig: projectConfig,
		workflows:     workflows,
		workflowRepo:  workflowRepo,
		taskRepo:      taskRepo,
		runtime:       runtime,
	}
}

func (a *Activities) GetWorkflowData(ctx context.Context, input *wfacts.GetDataInput) (*wfacts.GetData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewGetData(a.projectConfig, a.workflows)
	return act.Run(ctx, input)
}

// TriggerWorkflow executes the activity to trigger the workflow
func (a *Activities) TriggerWorkflow(ctx context.Context, input *wfacts.TriggerInput) (*workflow.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewTrigger(a.workflows, a.workflowRepo)
	return act.Run(ctx, input)
}

// UpdateWorkflowState executes the activity to update workflow status
func (a *Activities) UpdateWorkflowState(ctx context.Context, input *wfacts.UpdateStateInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	act := wfacts.NewUpdateState(a.workflowRepo, a.taskRepo)
	return act.Run(ctx, input)
}

// CompleteWorkflow executes the activity to complete workflow with task outputs
func (a *Activities) CompleteWorkflow(
	ctx context.Context,
	input *wfacts.CompleteWorkflowInput,
) (*workflow.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewCompleteWorkflow(a.workflowRepo)
	return act.Run(ctx, input)
}

func (a *Activities) DispatchTask(
	ctx context.Context,
	input *tkfacts.DispatchInput,
) (*tkfacts.DispatchOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewDispatch(a.workflows, a.workflowRepo, a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteBasicTask(ctx context.Context, input *tkfacts.ExecuteBasicInput) (*task.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewExecuteBasic(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
	)
	return act.Run(ctx, input)
}
