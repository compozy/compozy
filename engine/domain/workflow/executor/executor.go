package executor

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/pkg/logger"
	pkgnats "github.com/compozy/compozy/pkg/nats"
	commonpb "github.com/compozy/compozy/pkg/pb/common"
	taskpb "github.com/compozy/compozy/pkg/pb/task"
	pb "github.com/compozy/compozy/pkg/pb/workflow"
)

const (
	DefaultWorkflowTimeout = 10 * time.Minute
	DefaultTaskTimeout     = 5 * time.Minute
	ComponentName          = "workflow.Executor"
)

// WorkflowStatus enum values (copied from pb/workflow/events.pb.go)
const (
	WorkflowStatusUnknown   = pb.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED
	WorkflowStatusExecuting = pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING
	WorkflowStatusPaused    = pb.WorkflowStatus_WORKFLOW_STATUS_WAITING
	WorkflowStatusCompleted = pb.WorkflowStatus_WORKFLOW_STATUS_SUCCESS
	WorkflowStatusFailed    = pb.WorkflowStatus_WORKFLOW_STATUS_FAILED
	WorkflowStatusTimedOut  = pb.WorkflowStatus_WORKFLOW_STATUS_TIMED_OUT
	WorkflowStatusCanceled  = pb.WorkflowStatus_WORKFLOW_STATUS_CANCELED
)

// WorkflowConfig is the interface needed for workflow configuration
// This prevents import cycles with the workflow package
type WorkflowConfig interface {
	ValidateParams(map[string]any) error
	GetTasks() []task.Config
	GetID() string
}

type Options struct {
	DefaultTimeout time.Duration
}

type Executor struct {
	natsConn      *nats.Conn
	subscriptions []*nats.Subscription
	timeout       time.Duration
	workflows     []WorkflowConfig
}

// New creates a new workflow executor
func New(natsServer *pkgnats.Server, workflows []WorkflowConfig, opts *Options) (*Executor, error) {
	if opts == nil {
		opts = &Options{
			DefaultTimeout: DefaultWorkflowTimeout,
		}
	}

	if opts.DefaultTimeout <= 0 {
		opts.DefaultTimeout = DefaultWorkflowTimeout
	}

	return &Executor{
		natsConn:  natsServer.Conn,
		timeout:   opts.DefaultTimeout,
		workflows: workflows,
	}, nil
}

// Start initializes the workflow executor and subscribes to execute commands
func (e *Executor) Start(ctx context.Context) error {
	logger.Info("Starting workflow executor")

	// Subscribe to workflow execute commands
	sub, err := e.natsConn.Subscribe("compozy.*.workflow.cmds.*.execute", func(msg *nats.Msg) {
		var cmd pb.WorkflowExecuteCommand
		if err := protojson.Unmarshal(msg.Data, &cmd); err != nil {
			logger.Error("Failed to unmarshal workflow execute command", "error", err)
			return
		}

		execCtx, cancel := context.WithTimeout(ctx, e.timeout)
		defer cancel()

		if err := e.executeWorkflow(execCtx, &cmd); err != nil {
			logger.Error("Failed to execute workflow",
				"error", err,
				"workflow_id", cmd.GetWorkflow().GetId(),
				"exec_id", cmd.GetWorkflow().GetExecId(),
			)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to workflow execute commands: %w", err)
	}
	e.subscriptions = append(e.subscriptions, sub)

	logger.Info("Workflow executor started")
	return nil
}

// Stop unsubscribes from NATS topics and cleans up resources
func (e *Executor) Stop() error {
	logger.Info("Stopping workflow executor")
	for _, sub := range e.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			logger.Error("Failed to unsubscribe", "error", err)
		}
	}
	e.subscriptions = nil
	logger.Info("Workflow executor stopped")
	return nil
}

// executeWorkflow handles the execution of a workflow
func (e *Executor) executeWorkflow(ctx context.Context, cmd *pb.WorkflowExecuteCommand) error {
	workflow := cmd.GetWorkflow()
	logger.Info("Executing workflow",
		"workflow_id", workflow.GetId(),
		"exec_id", workflow.GetExecId(),
	)

	// Emit workflow execution started event
	if err := e.emitExecStarted(ctx, cmd); err != nil {
		return fmt.Errorf("failed to emit workflow execution started event: %w", err)
	}

	// Find workflow configuration and prepare for execution
	workflowCfg, wfContext, currentTask, err := e.prepareWorkflowExecution(ctx, cmd)
	if err != nil {
		return err // Error already handled in the helper function
	}

	// Execute the workflow tasks
	startTime := time.Now()
	finalResult, err := e.executeWorkflowTasks(ctx, cmd, workflowCfg, currentTask, wfContext)
	executionTime := time.Since(startTime)

	// Handle execution result
	if err != nil {
		return e.handleWorkflowExecutionError(ctx, cmd, err)
	}

	// Emit workflow execution success event
	if err := e.emitExecSuccess(ctx, cmd, finalResult, executionTime); err != nil {
		logger.Error("Failed to emit workflow execution success event", "error", err)
	}

	logger.Info("Workflow executed successfully",
		"workflow_id", workflow.GetId(),
		"exec_id", workflow.GetExecId(),
		"duration_ms", executionTime.Milliseconds(),
	)
	return nil
}

// prepareWorkflowExecution handles preparation steps before workflow execution
func (e *Executor) prepareWorkflowExecution(
	ctx context.Context,
	cmd *pb.WorkflowExecuteCommand,
) (WorkflowConfig, *structpb.Struct, *task.Config, error) {
	// Find workflow configuration from already loaded workflows
	workflowCfg, err := e.findWorkflowConfig(cmd.GetWorkflow().GetId())
	if err != nil {
		e.handleError(ctx, cmd, "Failed to find workflow configuration", err, nil)
		return nil, nil, nil, fmt.Errorf("failed to find workflow configuration: %w", err)
	}

	// Initialize workflow context and execution state
	wfContext, err := e.initContext(cmd)
	if err != nil {
		e.handleError(ctx, cmd, "Failed to initialize workflow context", err, nil)
		return nil, nil, nil, fmt.Errorf("failed to initialize workflow context: %w", err)
	}

	// Validate workflow parameters against schema
	if cmd.GetPayload() != nil && cmd.GetPayload().GetContext() != nil {
		if err := e.validateWorkflowParams(cmd, workflowCfg); err != nil {
			e.handleError(ctx, cmd, "Invalid workflow input parameters", err, ptr("INVALID_PARAMETERS"))
			return nil, nil, nil, fmt.Errorf("invalid workflow input parameters: %w", err)
		}
	}

	// Find initial task to execute
	currentTask, err := e.findInitialTask(workflowCfg)
	if err != nil {
		e.handleError(ctx, cmd, "Failed to find initial task", err, nil)
		return nil, nil, nil, fmt.Errorf("failed to find initial task: %w", err)
	}

	return workflowCfg, wfContext, currentTask, nil
}

// handleWorkflowExecutionError processes errors from workflow execution
func (e *Executor) handleWorkflowExecutionError(
	ctx context.Context,
	cmd *pb.WorkflowExecuteCommand,
	err error,
) error {
	// Check if it's a timeout error
	if ctx.Err() == context.DeadlineExceeded {
		if emitErr := e.emitExecTimedOut(ctx, cmd); emitErr != nil {
			logger.Error("Failed to emit workflow execution timed out event", "error", emitErr)
		}
		return fmt.Errorf("workflow execution timed out: %w", err)
	}

	// It's another type of error
	errResult := &commonpb.Result{
		Error: &commonpb.ErrorResult{
			Message: fmt.Sprintf("Workflow execution failed: %s", err.Error()),
			Code:    nil,
			Details: nil,
		},
	}
	if emitErr := e.emitExecFailed(ctx, cmd, errResult); emitErr != nil {
		logger.Error("Failed to emit workflow execution failed event", "error", emitErr)
	}
	return fmt.Errorf("workflow execution failed: %w", err)
}

// handleError is a helper to create an error result and emit the execution failed event
func (e *Executor) handleError(
	ctx context.Context,
	cmd *pb.WorkflowExecuteCommand,
	message string,
	err error,
	code *string,
) {
	errResult := &commonpb.Result{
		Error: &commonpb.ErrorResult{
			Message: fmt.Sprintf("%s: %s", message, err.Error()),
			Code:    code,
			Details: nil,
		},
	}
	if emitErr := e.emitExecFailed(ctx, cmd, errResult); emitErr != nil {
		logger.Error("Failed to emit workflow execution failed event", "error", emitErr)
	}
}

// findWorkflowConfig retrieves a workflow configuration by ID
func (e *Executor) findWorkflowConfig(workflowID string) (WorkflowConfig, error) {
	for _, wf := range e.workflows {
		if wf.GetID() == workflowID {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow with ID %s not found", workflowID)
}

// findInitialTask determines the first task to execute in a workflow
func (e *Executor) findInitialTask(workflow WorkflowConfig) (*task.Config, error) {
	tasks := workflow.GetTasks()
	if len(tasks) == 0 {
		return nil, fmt.Errorf("workflow contains no tasks")
	}

	// In a real implementation, this would analyze the workflow definition to find the initial task
	// For now, we'll use a simple approach and take the first task in the list
	return &tasks[0], nil
}

// executeWorkflowTasks handles the execution of all tasks in a workflow
func (e *Executor) executeWorkflowTasks(
	ctx context.Context,
	workflowCmd *pb.WorkflowExecuteCommand,
	workflow WorkflowConfig,
	currentTask *task.Config,
	wfContext *structpb.Struct,
) (*commonpb.Result, error) {
	var result *commonpb.Result

	// Loop through tasks until we reach the end or encounter an error
	for currentTask != nil {
		// Check if context is done (timeout or cancellation)
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Execute the current task and get its result
		taskResult, err := e.executeTaskWithContext(ctx, workflowCmd, currentTask, wfContext)
		if err != nil {
			return nil, err
		}

		// Store the latest result
		result = taskResult

		// If this is a final task, we're done
		if currentTask.Final {
			break
		}

		// Determine and find the next task to execute
		nextTask, err := e.determineNextTask(workflow, currentTask, taskResult)
		if err != nil {
			return taskResult, err
		}

		// Update the current task
		currentTask = nextTask

		// Update workflow context with task result
		if err := e.updateWorkflowContext(taskResult, &wfContext); err != nil {
			return nil, err
		}
	}

	// If we get here with no result, create a default success result
	if result == nil {
		result = &commonpb.Result{
			Output: wfContext,
		}
	}

	return result, nil
}

// executeTaskWithContext executes a single task with the given context
func (e *Executor) executeTaskWithContext(
	ctx context.Context,
	workflowCmd *pb.WorkflowExecuteCommand,
	taskConfig *task.Config,
	wfContext *structpb.Struct,
) (*commonpb.Result, error) {
	taskExecID := common.GenerateExecID()
	taskResult, err := e.executeTask(ctx, workflowCmd, taskConfig, taskExecID, wfContext)
	if err != nil {
		return nil, fmt.Errorf("task execution failed: %w", err)
	}
	return taskResult, nil
}

// determineNextTask determines the next task to execute based on the current task's result
func (e *Executor) determineNextTask(
	workflow WorkflowConfig,
	currentTask *task.Config,
	taskResult *commonpb.Result,
) (*task.Config, error) {
	var nextTaskID string

	if taskResult.GetError() != nil {
		// Handle error transition
		nextTaskID, err := e.getErrorTransition(currentTask)
		if err != nil {
			return nil, err
		}
		return e.findTaskByID(workflow, nextTaskID)
	}

	// Handle success transition
	nextTaskID = e.getSuccessTransition(currentTask)

	if nextTaskID == "" {
		// No next task defined, this is the end of the workflow
		return nil, nil
	}

	return e.findTaskByID(workflow, nextTaskID)
}

// getErrorTransition gets the next task ID for an error transition
func (e *Executor) getErrorTransition(currentTask *task.Config) (string, error) {
	if currentTask.OnError != nil && currentTask.OnError.Next != nil {
		return *currentTask.OnError.Next, nil
	}
	return "", fmt.Errorf("task failed with no error transition defined")
}

// getSuccessTransition gets the next task ID for a success transition
func (e *Executor) getSuccessTransition(currentTask *task.Config) string {
	if currentTask.OnSuccess != nil && currentTask.OnSuccess.Next != nil {
		return *currentTask.OnSuccess.Next
	}
	return "" // No next task is okay for success case
}

// updateWorkflowContext updates the workflow context with task result output
func (e *Executor) updateWorkflowContext(
	taskResult *commonpb.Result,
	wfContextPtr **structpb.Struct,
) error {
	wfContext := *wfContextPtr

	if taskResult.GetOutput() == nil {
		return nil
	}

	// Merge task output into workflow context
	contextMap := wfContext.AsMap()
	maps.Copy(contextMap, taskResult.GetOutput().AsMap())

	// Update the workflow context
	newContext, err := structpb.NewStruct(contextMap)
	if err != nil {
		return fmt.Errorf("failed to update workflow context: %w", err)
	}

	*wfContextPtr = newContext
	return nil
}

// findTaskByID finds a task in a workflow by its ID
func (e *Executor) findTaskByID(workflow WorkflowConfig, taskID string) (*task.Config, error) {
	tasks := workflow.GetTasks()
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task with ID %s not found in workflow", taskID)
}

// executeTask executes a single task
func (e *Executor) executeTask(
	ctx context.Context,
	workflowCmd *pb.WorkflowExecuteCommand,
	taskConfig *task.Config,
	taskExecID string,
	wfContext *structpb.Struct,
) (*commonpb.Result, error) {
	logger.Info("Executing task",
		"workflow_id", workflowCmd.GetWorkflow().GetId(),
		"workflow_exec_id", workflowCmd.GetWorkflow().GetExecId(),
		"task_id", taskConfig.ID,
		"task_exec_id", taskExecID,
	)

	// Create and publish task command
	taskCmd := e.createTaskCommand(workflowCmd, taskConfig, taskExecID, wfContext)
	if err := e.publishTaskCommand(taskCmd); err != nil {
		return nil, err
	}

	// Wait for and process the task result
	return e.waitForTaskResult(ctx, taskCmd)
}

// createTaskCommand creates a task execute command from workflow and task information
func (e *Executor) createTaskCommand(
	workflowCmd *pb.WorkflowExecuteCommand,
	taskConfig *task.Config,
	taskExecID string,
	wfContext *structpb.Struct,
) *taskpb.TaskExecuteCommand {
	return &taskpb.TaskExecuteCommand{
		Metadata: &commonpb.Metadata{
			CorrelationId:   workflowCmd.GetMetadata().GetCorrelationId(),
			RequestId:       common.GenerateRequestID(),
			SourceComponent: ComponentName,
			EventTimestamp:  timestamppb.Now(),
		},
		Workflow: workflowCmd.GetWorkflow(),
		Task: &commonpb.TaskInfo{
			Id:     taskConfig.ID,
			ExecId: taskExecID,
		},
		Payload: &taskpb.TaskExecuteCommand_Payload{
			Context: wfContext,
		},
	}
}

// publishTaskCommand marshals and publishes a task command to NATS
func (e *Executor) publishTaskCommand(taskCmd *taskpb.TaskExecuteCommand) error {
	taskData, err := protojson.Marshal(taskCmd)
	if err != nil {
		return fmt.Errorf("failed to marshal task execute command: %w", err)
	}

	subject := taskCmd.ToSubject()
	if err := e.natsConn.Publish(subject, taskData); err != nil {
		return fmt.Errorf("failed to publish task execute command: %w", err)
	}

	return nil
}

// waitForTaskResult sets up a subscription and waits for the task result
func (e *Executor) waitForTaskResult(
	ctx context.Context,
	_ *taskpb.TaskExecuteCommand, // Not using this parameter directly
) (*commonpb.Result, error) {
	// Create a reply inbox for the task execution result
	inbox := nats.NewInbox()
	sub, err := e.natsConn.SubscribeSync(inbox)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription for task result: %w", err)
	}
	defer func() {
		if unsubErr := sub.Unsubscribe(); unsubErr != nil {
			logger.Error("Failed to unsubscribe from inbox", "error", unsubErr)
		}
	}()

	// Set up a timeout for task execution
	taskCtx, cancel := context.WithTimeout(ctx, DefaultTaskTimeout)
	defer cancel()

	// Wait for the task execution result
	select {
	case <-taskCtx.Done():
		return nil, fmt.Errorf("task execution timed out: %w", taskCtx.Err())
	default:
		return e.processTaskResponse(sub)
	}
}

// processTaskResponse waits for and processes the task response message
func (e *Executor) processTaskResponse(sub *nats.Subscription) (*commonpb.Result, error) {
	// Wait for reply with timeout
	msg, err := sub.NextMsg(DefaultTaskTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to receive task execution result: %w", err)
	}

	return e.parseTaskResult(msg.Data)
}

// parseTaskResult unmarshals the task result from the message data
func (e *Executor) parseTaskResult(msgData []byte) (*commonpb.Result, error) {
	// Try to unmarshal as success event first
	var successEvent taskpb.TaskExecutionSuccessEvent
	if err := protojson.Unmarshal(msgData, &successEvent); err == nil {
		return successEvent.GetPayload().GetResult(), nil
	}

	// Try to unmarshal as failure event
	var failedEvent taskpb.TaskExecutionFailedEvent
	if err := protojson.Unmarshal(msgData, &failedEvent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task execution result: %w", err)
	}

	// Return the error result
	return failedEvent.GetPayload().GetResult(), nil
}

// validateWorkflowParams validates input parameters against the workflow's schema
func (e *Executor) validateWorkflowParams(cmd *pb.WorkflowExecuteCommand, workflowCfg WorkflowConfig) error {
	if cmd.GetPayload() == nil || cmd.GetPayload().GetContext() == nil {
		return nil
	}

	contextMap := cmd.GetPayload().GetContext().AsMap()
	inputVal, exists := contextMap["input"]
	if !exists {
		return nil
	}

	inputMap, ok := inputVal.(map[string]any)
	if !ok {
		return fmt.Errorf("input is not a valid object")
	}

	return workflowCfg.ValidateParams(inputMap)
}
