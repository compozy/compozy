package executors

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput

// -----------------------------------------------------------------------------
// Activity Context Builder
// -----------------------------------------------------------------------------

type ContextBuilder struct {
	Workflows      []*wf.Config
	ProjectConfig  *project.Config
	WorkflowConfig *wf.Config
	*WorkflowInput
}

func NewContextBuilder(
	workflows []*wf.Config,
	projectConfig *project.Config,
	workflowConfig *wf.Config,
	workflowInput *WorkflowInput,
) *ContextBuilder {
	return &ContextBuilder{
		Workflows:      workflows,
		ProjectConfig:  projectConfig,
		WorkflowConfig: workflowConfig,
		WorkflowInput:  workflowInput,
	}
}

func (b *ContextBuilder) BuildBaseContext(ctx workflow.Context) workflow.Context {
	resolved := core.ResolveActivityOptions(
		&b.ProjectConfig.Opts.GlobalOpts,
		&b.WorkflowConfig.Opts.GlobalOpts,
		nil,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	activityOptions.WaitForCancellation = true
	return workflow.WithActivityOptions(ctx, activityOptions)
}

func (b *ContextBuilder) BuildTaskContext(ctx workflow.Context, taskID string) workflow.Context {
	taskConfig, err := task.FindConfig(b.WorkflowConfig.Tasks, taskID)
	if err != nil {
		return ctx
	}
	resolved := core.ResolveActivityOptions(
		&b.ProjectConfig.Opts.GlobalOpts,
		&b.WorkflowConfig.Opts.GlobalOpts,
		&taskConfig.Config,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	activityOptions.WaitForCancellation = true
	return workflow.WithActivityOptions(ctx, activityOptions)
}
