package worker

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Temporal-based Worker
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo func() workflow.Repository
	TaskRepo     func() task.Repository
}

type Worker struct {
	client        *Client
	config        *Config
	activities    *Activities
	worker        worker.Worker
	projectConfig *project.Config
	workflows     []*workflow.Config
}

func NewWorker(
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) (*Worker, error) {
	client, err := NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}
	worker := client.NewWorker(client.Config().TaskQueue)
	activities := NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
	)
	return &Worker{
		client:        client,
		config:        config,
		worker:        worker,
		projectConfig: projectConfig,
		workflows:     workflows,
		activities:    activities,
	}, nil
}

func (o *Worker) Setup(_ context.Context) error {
	o.worker.RegisterWorkflow(CompozyWorkflow)
	o.worker.RegisterActivity(o.activities.TriggerWorkflow)
	o.worker.RegisterActivity(o.activities.UpdateWorkflowState)
	o.worker.RegisterActivity(o.activities.DispatchTask)
	o.worker.RegisterActivity(o.activities.ExecuteBasicTask)
	return o.worker.Start()
}

func (o *Worker) Stop() {
	o.client.Close()
	o.worker.Stop()
}

func (o *Worker) WorkflowRepo() workflow.Repository {
	return o.config.WorkflowRepo()
}

func (o *Worker) TaskRepo() task.Repository {
	return o.config.TaskRepo()
}

// -----------------------------------------------------------------------------
// Workflow Operations
// -----------------------------------------------------------------------------

func (o *Worker) TriggerWorkflow(
	ctx context.Context,
	workflowID string,
	input *core.Input,
	initTaskID string,
) (*WorkflowInput, error) {
	// Start workflow
	workflowExecID := core.MustNewID()
	workflowInput := WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          input,
		InitialTaskID:  initTaskID,
	}

	options := client.StartWorkflowOptions{
		ID:        workflowExecID.String(),
		TaskQueue: o.client.Config().TaskQueue,
	}
	workflowConfig, err := workflow.FindConfig(o.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	if err := workflowConfig.ValidateParams(ctx, input); err != nil {
		return nil, fmt.Errorf("failed to validate workflow params: %w", err)
	}

	_, err = o.client.ExecuteWorkflow(
		ctx,
		options,
		CompozyWorkflow,
		workflowInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}
	return &workflowInput, nil
}

func (o *Worker) PauseWorkflow(ctx context.Context, workflowExecID core.ID) error {
	id := workflowExecID.String()
	return o.client.SignalWorkflow(ctx, id, "", SignalPause, nil)
}

func (o *Worker) ResumeWorkflow(ctx context.Context, workflowExecID core.ID) error {
	id := workflowExecID.String()
	return o.client.SignalWorkflow(ctx, id, "", SignalResume, nil)
}

func (o *Worker) CancelWorkflow(ctx context.Context, workflowExecID core.ID) error {
	id := workflowExecID.String()
	return o.client.SignalWorkflow(ctx, id, "", SignalCancel, nil)
}
