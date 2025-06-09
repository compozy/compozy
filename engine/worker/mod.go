package worker

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/gosimple/slug"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Temporal-based Worker
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo func() wf.Repository
	TaskRepo     func() task.Repository
}

type Worker struct {
	client        *Client
	config        *Config
	activities    *Activities
	worker        worker.Worker
	projectConfig *project.Config
	workflows     []*wf.Config
	taskQueue     string
}

func NewWorker(
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
) (*Worker, error) {
	client, err := NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}
	taskQueue := slug.Make(projectConfig.Name)
	worker := client.NewWorker(taskQueue)
	projectRoot := projectConfig.GetCWD().PathStr()

	// Build runtime options from project config
	var rtOpts []runtime.Option
	if len(projectConfig.Runtime.Permissions) > 0 {
		rtOpts = append(rtOpts, runtime.WithDenoPermissions(projectConfig.Runtime.Permissions))
	}
	runtime, err := runtime.NewRuntimeManager(projectRoot, rtOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to created execution manager: %w", err)
	}
	activities := NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
		runtime,
	)
	return &Worker{
		client:        client,
		config:        config,
		worker:        worker,
		projectConfig: projectConfig,
		workflows:     workflows,
		activities:    activities,
		taskQueue:     taskQueue,
	}, nil
}

func (o *Worker) Setup(_ context.Context) error {
	o.worker.RegisterWorkflow(CompozyWorkflow)
	o.worker.RegisterActivity(o.activities.GetWorkflowData)
	o.worker.RegisterActivity(o.activities.TriggerWorkflow)
	o.worker.RegisterActivity(o.activities.UpdateWorkflowState)
	o.worker.RegisterActivity(o.activities.CompleteWorkflow)
	o.worker.RegisterActivity(o.activities.ExecuteBasicTask)
	o.worker.RegisterActivity(o.activities.ExecuteRouterTask)
	o.worker.RegisterActivity(o.activities.ExecuteParallelTask)
	o.worker.RegisterActivity(o.activities.CreateParallelState)
	o.worker.RegisterActivity(o.activities.GetParallelResponse)
	o.worker.RegisterActivity(o.activities.GetProgress)
	o.worker.RegisterActivity(o.activities.UpdateParentStatus)
	return o.worker.Start()
}

func (o *Worker) Stop() {
	o.worker.Stop()
	o.client.Close()
}

func (o *Worker) WorkflowRepo() wf.Repository {
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
		TaskQueue: o.taskQueue,
	}
	workflowConfig, err := wf.FindConfig(o.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	if err := workflowConfig.ValidateInput(ctx, input); err != nil {
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

func (o *Worker) CancelWorkflow(ctx context.Context, workflowExecID core.ID) error {
	id := workflowExecID.String()
	return o.client.CancelWorkflow(ctx, id, "")
}
