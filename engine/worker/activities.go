package worker

import (
	"context"

	"github.com/compozy/compozy/engine/project"
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
}

func NewActivities(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *Activities {
	return &Activities{
		projectConfig: projectConfig,
		workflows:     workflows,
		workflowRepo:  workflowRepo,
		taskRepo:      taskRepo,
	}
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
	act := wfacts.NewUpdateState(a.workflowRepo)
	return act.Run(ctx, input)
}

func (a *Activities) DispatchTask(ctx context.Context, input *tkfacts.DispatchInput) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewDispatch(a.workflows, a.workflowRepo, a.taskRepo)
	return act.Run(ctx, input)
}
