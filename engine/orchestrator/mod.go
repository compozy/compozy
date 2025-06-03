package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/temporal"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Temporal-based Orchestrator
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo func() workflow.Repository
	TaskRepo     func() task.Repository
}

type Orchestrator struct {
	config        *Config
	tc            *temporal.Client
	activities    *temporal.Activities
	worker        worker.Worker
	projectConfig *project.Config
	workflows     []*workflow.Config
}

func NewOrchestrator(
	tc *temporal.Client,
	config *Config,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) (*Orchestrator, error) {
	worker := tc.NewWorker(tc.Config().TaskQueue)
	activities := temporal.NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
	)
	return &Orchestrator{
		tc:            tc,
		config:        config,
		worker:        worker,
		projectConfig: projectConfig,
		workflows:     workflows,
		activities:    activities,
	}, nil
}

func (o *Orchestrator) Setup(_ context.Context) error {
	o.tc.RegisterWorker(o.worker, o.activities)
	return o.worker.Start()
}

func (o *Orchestrator) Stop() {
	o.worker.Stop()
}

func (o *Orchestrator) WorkflowRepo() workflow.Repository {
	return o.config.WorkflowRepo()
}

func (o *Orchestrator) TaskRepo() task.Repository {
	return o.config.TaskRepo()
}

// -----------------------------------------------------------------------------
// Workflow Operations
// -----------------------------------------------------------------------------

func (o *Orchestrator) TriggerWorkflow(
	ctx context.Context,
	workflowID string,
	input *core.Input,
) (*workflow.StateID, error) {
	// Start workflow
	workflowExecID := core.MustNewID()
	workflowInput := temporal.WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          input,
	}

	options := client.StartWorkflowOptions{
		ID:        workflowExecID.String(),
		TaskQueue: o.tc.Config().TaskQueue,
	}

	_, err := o.tc.ExecuteWorkflow(
		ctx,
		options,
		temporal.CompozyWorkflow,
		workflowInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	return &workflow.StateID{
		WorkflowID:   workflowID,
		WorkflowExec: workflowExecID,
	}, nil
}

func (o *Orchestrator) PauseWorkflow(ctx context.Context, workflowExecID string) error {
	return o.tc.SignalWorkflow(ctx, workflowExecID, "", temporal.SignalPause, nil)
}

func (o *Orchestrator) ResumeWorkflow(ctx context.Context, workflowExecID string) error {
	return o.tc.SignalWorkflow(ctx, workflowExecID, "", temporal.SignalResume, nil)
}

func (o *Orchestrator) CancelWorkflow(ctx context.Context, workflowExecID string) error {
	return o.tc.SignalWorkflow(ctx, workflowExecID, "", temporal.SignalCancel, nil)
}
