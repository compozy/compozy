package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/usage"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteBasicLabel = "ExecuteBasicTask"

type ExecuteBasicInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// ExecuteBasic handles basic task execution with task2 integration
type ExecuteBasic struct {
	loadWorkflowUC  *uc.LoadWorkflow
	createStateUC   *uc.CreateState
	executeUC       *uc.ExecuteTask
	task2Factory    task2.Factory
	workflowRepo    workflow.Repository
	taskRepo        task.Repository
	usageMetrics    usage.Metrics
	providerMetrics providermetrics.Recorder
	memoryManager   memcore.ManagerInterface
	templateEngine  *tplengine.TemplateEngine
	projectConfig   *project.Config
	streamPublisher services.StreamPublisher
}

// NewExecuteBasic creates and returns a configured ExecuteBasic activity.
//
// The constructed ExecuteBasic wires the provided repositories, runtime, memory
// manager, template engine, project and app configs, and task2 factory into
// its internal use-cases. It initializes use-cases for loading workflows,
// creating task state, and executing tasks (ExecuteTask is constructed with the
// workflow repository), and returns the ready-to-use ExecuteBasic or an error.
func NewExecuteBasic(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	usageMetrics usage.Metrics,
	providerMetrics providermetrics.Recorder,
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
	projectConfig *project.Config,
	task2Factory task2.Factory,
	toolEnvironment toolenv.Environment,
	streamPublisher services.StreamPublisher,
) (*ExecuteBasic, error) {
	if toolEnvironment == nil {
		return nil, fmt.Errorf("tool environment is required for execute basic activity")
	}
	return &ExecuteBasic{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		executeUC: uc.NewExecuteTask(
			runtime,
			workflowRepo,
			memoryManager,
			templateEngine,
			nil,
			providerMetrics,
			toolEnvironment,
		),
		task2Factory:    task2Factory,
		workflowRepo:    workflowRepo,
		taskRepo:        taskRepo,
		usageMetrics:    usageMetrics,
		providerMetrics: providerMetrics,
		memoryManager:   memoryManager,
		templateEngine:  templateEngine,
		projectConfig:   projectConfig,
		streamPublisher: streamPublisher,
	}, nil
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.MainTaskResponse, error) {
	if err := validateExecuteBasicInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	normalizedConfig, err := a.normalizeBasicConfig(ctx, workflowState, workflowConfig, input.TaskConfig)
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
	result, err := a.executeBasicWithResponse(ctx, normalizedConfig, workflowState, workflowConfig, taskState)
	if err != nil {
		return nil, err
	}
	if result.executionErr != nil {
		return result.response, result.executionErr
	}
	return result.response, nil
}

// basicExecutionResult captures the outcome of a basic task execution.
type basicExecutionResult struct {
	response     *task.MainTaskResponse
	executionErr error
}

// executeBasicWithResponse runs the task execution and builds the main task response.
func (a *ExecuteBasic) executeBasicWithResponse(
	ctx context.Context,
	normalizedConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskState *task.State,
) (*basicExecutionResult, error) {
	ctx, finalizeUsage := a.attachUsageCollector(ctx, taskState)
	status := core.StatusFailed
	defer func() {
		finalizeUsage(status)
	}()
	output, executionError := a.executeUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:     normalizedConfig,
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		ProjectConfig:  a.projectConfig,
	})
	if executionError == nil {
		status = core.StatusSuccess
	}
	taskState.Output = output
	if executionError == nil {
		a.publishTextChunks(ctx, normalizedConfig, taskState)
	}
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
	if err != nil {
		return nil, fmt.Errorf("failed to create basic response handler: %w", err)
	}
	responseInput := buildBasicResponseInput(normalizedConfig, taskState, workflowConfig, workflowState, executionError)
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle basic response: %w", err)
	}
	if taskState.Status != "" {
		status = taskState.Status
	}
	return &basicExecutionResult{
		response:     a.convertToMainTaskResponse(result),
		executionErr: executionError,
	}, nil
}

func (a *ExecuteBasic) publishTextChunks(ctx context.Context, cfg *task.Config, state *task.State) {
	if a == nil || a.streamPublisher == nil {
		return
	}
	a.streamPublisher.Publish(ctx, cfg, state)
}

func (a *ExecuteBasic) attachUsageCollector(
	ctx context.Context,
	state *task.State,
) (context.Context, func(core.StatusType)) {
	if a == nil || state == nil {
		return ctx, func(core.StatusType) {}
	}
	collector := usage.NewCollector(a.usageMetrics, usage.Metadata{
		Component:      state.Component,
		WorkflowExecID: state.WorkflowExecID,
		TaskExecID:     state.TaskExecID,
		AgentID:        state.AgentID,
	})
	collectorCtx := usage.ContextWithCollector(ctx, collector)
	return collectorCtx, func(status core.StatusType) {
		finalized, err := collector.Finalize(collectorCtx, status)
		if err != nil {
			logger.FromContext(collectorCtx).Warn(
				"Failed to aggregate usage for task execution",
				"error", err,
				"task_exec_id", state.TaskExecID.String(),
			)
			return
		}
		a.persistUsageSummary(collectorCtx, state, finalized)
	}
}

func (a *ExecuteBasic) persistUsageSummary(ctx context.Context, state *task.State, finalized *usage.Finalized) {
	if a == nil || state == nil || finalized == nil || finalized.Summary == nil {
		return
	}
	taskSummary := finalized.Summary.CloneWithSource(usage.SourceTask)
	if taskSummary == nil || len(taskSummary.Entries) == 0 {
		return
	}
	log := logger.FromContext(ctx)
	if err := a.taskRepo.MergeUsage(ctx, state.TaskExecID, taskSummary); err != nil {
		log.Warn(
			"Failed to merge task usage", "task_exec_id", state.TaskExecID.String(), "error", err,
		)
	}
	if a.workflowRepo != nil && !state.WorkflowExecID.IsZero() {
		workflowSummary := finalized.Summary.CloneWithSource(usage.SourceWorkflow)
		if err := a.workflowRepo.MergeUsage(ctx, state.WorkflowExecID, workflowSummary); err != nil {
			log.Warn(
				"Failed to merge workflow usage", "workflow_exec_id", state.WorkflowExecID.String(), "error", err,
			)
		}
	}
}

// convertToMainTaskResponse converts shared.ResponseOutput to task.MainTaskResponse
func (a *ExecuteBasic) convertToMainTaskResponse(result *shared.ResponseOutput) *task.MainTaskResponse {
	converter := NewResponseConverter()
	return converter.ConvertToMainTaskResponse(result)
}

// validateExecuteBasicInput verifies required ExecuteBasic parameters.
func validateExecuteBasicInput(input *ExecuteBasicInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("invalid ExecuteBasic input: task_config is required")
	}
	return nil
}

// normalizeBasicConfig renders templates and validates the normalized basic configuration.
func (a *ExecuteBasic) normalizeBasicConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.Config, error) {
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeBasic)
	if err != nil {
		return nil, fmt.Errorf("failed to create basic normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize basic task: %w", err)
	}
	if taskConfig.With != nil {
		normContext.CurrentInput = taskConfig.With
		contextBuilder.VariableBuilder.AddCurrentInputToVariables(normContext.Variables, taskConfig.With)
	}
	if taskConfig.Type != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskConfig.Type)
	}
	return taskConfig, nil
}

// buildBasicResponseInput prepares the shared response input for basic tasks.
func buildBasicResponseInput(
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
