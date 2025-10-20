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
	if err := validateAggregateInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	normalizedConfig, err := a.normalizeAggregateConfig(ctx, input.TaskConfig, workflowState, workflowConfig)
	if err != nil {
		return nil, err
	}
	taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     normalizedConfig,
	})
	if err != nil {
		return nil, err
	}
	output, executionError := a.executeAggregateWithTimeout(ctx, normalizedConfig, workflowState, workflowConfig)
	taskState.Output = output
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeAggregate)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregate response handler: %w", err)
	}
	responseInput := buildAggregateResponseInput(
		normalizedConfig,
		taskState,
		workflowConfig,
		workflowState,
		executionError,
	)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle aggregate response: %w", err)
	}
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
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
	// NOTE: Bound aggregate execution to prevent long-running fan-in routines from stalling workflows.
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// NOTE: Run aggregation in a goroutine so the timeout can abort if templates hang.
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
	emptyOutput := &core.Output{}
	outputTransformer := task2core.NewOutputTransformer(a.templateEngine)
	taskConfigs := task2.BuildTaskConfigsMap(workflowConfig.Tasks)
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normCtx := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	normCtx.TaskConfigs = taskConfigs
	normCtx.CurrentInput = taskConfig.With
	normCtx.MergedEnv = taskConfig.Env
	processedOutput, err := outputTransformer.TransformOutput(
		ctx,
		emptyOutput,
		taskConfig.Outputs,
		normCtx,
		taskConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process aggregate outputs: %w", err)
	}
	if err := taskConfig.ValidateOutput(ctx, processedOutput); err != nil {
		return nil, fmt.Errorf("aggregate output failed schema validation: %w", err)
	}
	return processedOutput, nil
}

// validateAggregateInput ensures the aggregate activity request is well-formed.
func validateAggregateInput(input *ExecuteAggregateInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("task_config cannot be nil")
	}
	if input.TaskConfig.Type != task.TaskTypeAggregate {
		return fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	return nil
}

// normalizeAggregateConfig renders templates and normalizes the aggregate config.
func (a *ExecuteAggregate) normalizeAggregateConfig(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*task.Config, error) {
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeAggregate)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregate normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize aggregate task: %w", err)
	}
	return taskConfig, nil
}

// buildAggregateResponseInput prepares the shared response input for aggregate handlers.
func buildAggregateResponseInput(
	taskConfig *task.Config,
	taskState *task.State,
	workflowConfig *workflow.Config,
	workflowState *workflow.State,
	executionError error,
) *shared.ResponseInput {
	return &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
}
