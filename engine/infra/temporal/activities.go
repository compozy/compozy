package temporal

import (
	"context"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type Activities struct {
	projectConfig                *project.Config
	workflows                    []*workflow.Config
	workflowRepo                 workflow.Repository
	taskRepo                     task.Repository
	updateWorkflowStatusActivity *wfacts.UpdateStatus
}

func NewActivities(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *Activities {
	return &Activities{
		projectConfig:                projectConfig,
		workflows:                    workflows,
		workflowRepo:                 workflowRepo,
		taskRepo:                     taskRepo,
		updateWorkflowStatusActivity: wfacts.NewUpdateStatus(workflowRepo),
	}
}

func (a *Activities) WorkflowTrigger(ctx context.Context, input *wfacts.TriggerInput) (*workflow.State, error) {
	act := wfacts.NewTrigger(a.workflows, a.workflowRepo)
	return act.Run(ctx, input)
}

// UpdateWorkflowStatus executes the activity to update workflow status
func (a *Activities) UpdateWorkflowStatus(ctx context.Context, input *wfacts.UpdateStatusInput) error {
	act := wfacts.NewUpdateStatus(a.workflowRepo)
	return act.Run(ctx, input)
}
