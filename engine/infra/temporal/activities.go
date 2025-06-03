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
	updateWorkflowStatusActivity *wfacts.UpdateWorkflowStatusActivity
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
		updateWorkflowStatusActivity: wfacts.NewUpdateWorkflowStatusActivity(workflowRepo),
	}
}

func (a *Activities) WorkflowTrigger(ctx context.Context, input *wfacts.TriggerInput) (*workflow.State, error) {
	act := wfacts.NewTrigger(a.workflows, a.workflowRepo)
	return act.Run(ctx, input)
}

// UpdateWorkflowStatus executes the activity to update workflow status
func (a *Activities) UpdateWorkflowStatus(ctx context.Context, input *wfacts.UpdateWorkflowStatusInput) error {
	return a.updateWorkflowStatusActivity.Run(ctx, input)
}

// // TaskExecuteActivity wraps the existing task execution use case
// func (a *Activities) TaskExecuteActivity(ctx context.Context, cmd *pb.CmdTaskExecute) error {
// 	// TODO: Implement task execution
// 	return nil
// }

// // AgentExecuteActivity placeholder - to be implemented when agent execution is migrated
// func (a *Activities) AgentExecuteActivity(ctx context.Context, cmd *pb.CmdAgentExecute) error {
// 	// TODO: Implement when agent execution use case is available
// 	return nil
// }

// // ToolExecuteActivity placeholder - to be implemented when tool execution is migrated
// func (a *Activities) ToolExecuteActivity(ctx context.Context, cmd *pb.CmdToolExecute) error {
// 	// TODO: Implement when tool execution use case is available
// 	return nil
// }
