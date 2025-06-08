package worker

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
)

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
	var projectOpts *core.GlobalOpts
	if b.ProjectConfig != nil {
		projectOpts = &b.ProjectConfig.Opts.GlobalOpts
	}
	
	var workflowOpts *core.GlobalOpts
	if b.WorkflowConfig != nil {
		workflowOpts = &b.WorkflowConfig.Opts.GlobalOpts
	}
	
	resolved := core.ResolveActivityOptions(
		projectOpts,
		workflowOpts,
		nil,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	activityOptions.WaitForCancellation = true
	return workflow.WithActivityOptions(ctx, activityOptions)
}

func (b *ContextBuilder) BuildTaskContext(ctx workflow.Context, taskID string) workflow.Context {
	if b.WorkflowConfig == nil {
		return ctx
	}
	
	taskConfig, err := task.FindConfig(b.WorkflowConfig.Tasks, taskID)
	if err != nil {
		return ctx
	}
	
	var projectOpts *core.GlobalOpts
	if b.ProjectConfig != nil {
		projectOpts = &b.ProjectConfig.Opts.GlobalOpts
	}
	
	var workflowOpts *core.GlobalOpts
	if b.WorkflowConfig != nil {
		workflowOpts = &b.WorkflowConfig.Opts.GlobalOpts
	}
	
	resolved := core.ResolveActivityOptions(
		projectOpts,
		workflowOpts,
		&taskConfig.Config,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	activityOptions.WaitForCancellation = true
	return workflow.WithActivityOptions(ctx, activityOptions)
}
