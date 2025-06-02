package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/temporal"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Temporal-based Orchestrator
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepoFactory func() workflow.Repository
	TaskRepoFactory     func() task.Repository
	AgentRepoFactory    func() agent.Repository
	ToolRepoFactory     func() tool.Repository
}

type Orchestrator struct {
	tc            *temporal.Client
	activities    *temporal.Activities
	worker        worker.Worker
	config        Config
	projectConfig *project.Config
	workflows     []*workflow.Config
}

func NewOrchestrator(
	tc *temporal.Client,
	config Config,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *Orchestrator {
	worker := tc.NewWorker(tc.Config().TaskQueue)
	activities := temporal.NewActivities(
		config.TaskRepoFactory(),
		config.AgentRepoFactory(),
		config.ToolRepoFactory(),
	)
	return &Orchestrator{
		tc:            tc,
		worker:        worker,
		config:        config,
		projectConfig: projectConfig,
		workflows:     workflows,
		activities:    activities,
	}
}

func (o *Orchestrator) Config() *Config {
	return &o.config
}

func (o *Orchestrator) Setup(ctx context.Context) error {
	// Register workflows
	o.worker.RegisterWorkflow(temporal.CompozyWorkflow)

	// Register activities
	o.worker.RegisterActivity(o.activities.TaskExecuteActivity)
	o.worker.RegisterActivity(o.activities.AgentExecuteActivity)
	o.worker.RegisterActivity(o.activities.ToolExecuteActivity)

	// Start the worker
	return o.worker.Start()
}

func (o *Orchestrator) Stop() {
	o.worker.Stop()
}

// -----------------------------------------------------------------------------
// Workflow Operations
// -----------------------------------------------------------------------------

func (o *Orchestrator) TriggerWorkflow(ctx context.Context, workflowID string, input core.Input) (string, error) {
	// Find workflow config
	var workflowConfig *workflow.Config
	for _, wf := range o.workflows {
		if wf.ID == workflowID {
			workflowConfig = wf
			break
		}
	}

	if workflowConfig == nil {
		return "", fmt.Errorf("workflow %s not found", workflowID)
	}

	// Create workflow metadata
	metadata := &pb.WorkflowMetadata{
		WorkflowExecId: generateWorkflowExecID(workflowID),
		WorkflowId:     workflowID,
	}

	// Start workflow
	workflowInput := temporal.WorkflowInput{
		Metadata: metadata,
		Config:   workflowConfig,
		Input:    input,
	}

	options := client.StartWorkflowOptions{
		ID:        metadata.WorkflowExecId,
		TaskQueue: o.tc.Config().TaskQueue,
	}

	workflowRun, err := o.tc.ExecuteWorkflow(
		ctx,
		options,
		temporal.CompozyWorkflow,
		workflowInput,
	)
	if err != nil {
		return "", fmt.Errorf("failed to start workflow: %w", err)
	}

	return workflowRun.GetID(), nil
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

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func generateWorkflowExecID(workflowID string) string {
	return fmt.Sprintf("%s-%d", workflowID, time.Now().UnixNano())
}
