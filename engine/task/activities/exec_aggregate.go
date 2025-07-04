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
	task2Factory   task2.Factory
	templateEngine *tplengine.TemplateEngine
}

func NewExecuteAggregate(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	_ *core.PathCWD,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
) (*ExecuteAggregate, error) {
	return &ExecuteAggregate{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
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
	// Use task2 normalizer for aggregate tasks
	normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeAggregate)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregate normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(workflowState, workflowConfig, input.TaskConfig)
	// Normalize the task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize aggregate task: %w", err)
	}
	// Create task state
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     normalizedConfig,
	})
	if err != nil {
		return nil, err
	}
	// Execute aggregate logic with timeout protection
	output, executionError := a.executeAggregateWithTimeout(ctx, normalizedConfig, workflowState, workflowConfig)
	taskState.Output = output
	// Use task2 ResponseHandler for aggregate type
	handler, err := a.task2Factory.CreateResponseHandler(task.TaskTypeAggregate)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregate response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     normalizedConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle aggregate response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	// If there was an execution error, return it
	if executionError != nil {
		return mainTaskResponse, executionError
	}
	return mainTaskResponse, nil
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
	outputTransformer := task2core.NewOutputTransformer(a.templateEngine)

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
