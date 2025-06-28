package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteAggregateLabel = "ExecuteAggregateTask"

type ExecuteAggregateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteAggregate struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	taskResponder  *services.TaskResponder
}

func NewExecuteAggregate(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
) (*ExecuteAggregate, error) {
	configManager, err := services.NewConfigManager(configStore, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}
	return &ExecuteAggregate{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configManager),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
	}, nil
}

func (a *ExecuteAggregate) Run(ctx context.Context, input *ExecuteAggregateInput) (*task.MainTaskResponse, error) {
	// Validate input
	if input.TaskConfig == nil {
		return nil, fmt.Errorf("task_config cannot be nil")
	}
	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeAggregate {
		return nil, fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Normalize task config
	normalizer, err := uc.NewNormalizeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create normalizer: %w", err)
	}
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}
	// Create task state
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}
	// Execute aggregate logic with timeout protection
	output, executionError := a.executeAggregateWithTimeout(ctx, input.TaskConfig, workflowState, workflowConfig)
	taskState.Output = output
	// Handle main task response
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if handleErr != nil {
		return nil, handleErr
	}
	// If there was an execution error, the task should be considered failed
	if executionError != nil {
		return response, executionError
	}
	return response, nil
}

func (a *ExecuteAggregate) executeAggregateWithTimeout(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*core.Output, error) {
	// Create timeout context (30 seconds max)
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Execute in goroutine to respect timeout
	resultChan := make(chan struct {
		output *core.Output
		err    error
	}, 1)
	go func() {
		output, err := a.executeAggregate(timeoutCtx, taskConfig, workflowState, workflowConfig)
		select {
		case <-timeoutCtx.Done(): // parent timed-out – abort send
			return
		case resultChan <- struct {
			output *core.Output
			err    error
		}{output, err}:
		}
	}()
	select {
	case result := <-resultChan:
		return result.output, result.err
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("aggregate task execution timed out after 30 seconds")
	}
}

func (a *ExecuteAggregate) executeAggregate(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*core.Output, error) {
	if taskConfig.Outputs == nil || len(*taskConfig.Outputs) == 0 {
		return nil, fmt.Errorf("aggregate task has no outputs defined")
	}
	// For aggregate tasks, we don't have actual task output, just template processing
	// Create an empty output to trigger the transformation
	emptyOutput := &core.Output{}
	// Use task2 output transformer to process outputs with template engine
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	outputTransformer := task2core.NewOutputTransformer(engine)

	// Build task configs map for context
	taskConfigs := task2.BuildTaskConfigsMap(workflowConfig.Tasks)

	// Create normalization context with proper Variables
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normCtx := contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	normCtx.TaskConfigs = taskConfigs
	normCtx.CurrentInput = taskConfig.With
	normCtx.MergedEnv = taskConfig.Env

	processedOutput, err := outputTransformer.TransformOutput(
		emptyOutput,
		taskConfig.Outputs,
		normCtx,
		taskConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process aggregate outputs: %w", err)
	}
	// Validate the processed output against schema
	if err := taskConfig.ValidateOutput(ctx, processedOutput); err != nil {
		return nil, fmt.Errorf("aggregate output failed schema validation: %w", err)
	}
	return processedOutput, nil
}
