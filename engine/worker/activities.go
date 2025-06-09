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

// -----------------------------------------------------------------------------
// Workflow
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Task
// -----------------------------------------------------------------------------

func (a *Activities) ExecuteBasicTask(
	ctx context.Context,
	input *tkfacts.ExecuteBasicInput,
) (*task.Response, error) {
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

func (a *Activities) ExecuteRouterTask(
	ctx context.Context,
	input *tkfacts.ExecuteRouterInput,
) (*task.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewExecuteRouter(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
	)
	return act.Run(ctx, input)
}

func (a *Activities) CreateParallelState(
	ctx context.Context,
	input *tkfacts.CreateParallelStateInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewCreateParallelState(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
	)
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteParallelTask(
	ctx context.Context,
	input *tkfacts.ExecuteParallelTaskInput,
) (*task.SubtaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewExecuteParallelTask(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
	)
	return act.Run(ctx, input)
}

func (a *Activities) GetParallelResponse(
	ctx context.Context,
	input *tkfacts.GetParallelResponseInput,
) (*task.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetParallelResponse(a.workflowRepo, a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) GetProgress(
	ctx context.Context,
	input *tkfacts.GetProgressInput,
) (*task.ProgressInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetProgress(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) UpdateParentStatus(
	ctx context.Context,
	input *tkfacts.UpdateParentStatusInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewUpdateParentStatus(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteCollectionTask(
	ctx context.Context,
	input *tkfacts.ExecuteCollectionInput,
) (*tkfacts.CollectionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewExecuteCollection(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
	)
	return act.Run(ctx, input)
}
