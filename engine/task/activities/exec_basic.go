package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
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
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	executeUC      *uc.ExecuteTask
	task2Factory   task2.Factory
	workflowRepo   workflow.Repository
	taskRepo       task.Repository
	memoryManager  memcore.ManagerInterface
	templateEngine *tplengine.TemplateEngine
	projectConfig  *project.Config
	appConfig      *config.Config
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
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
	projectConfig *project.Config,
	task2Factory task2.Factory,
	appConfig *config.Config,
) (*ExecuteBasic, error) {
	return &ExecuteBasic{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configStore),
		executeUC:      uc.NewExecuteTask(runtime, workflowRepo, memoryManager, templateEngine, appConfig, nil),
		task2Factory:   task2Factory,
		workflowRepo:   workflowRepo,
		taskRepo:       taskRepo,
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
		projectConfig:  projectConfig,
		appConfig:      appConfig,
	}, nil
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.MainTaskResponse, error) {
	// Validate input
	if input == nil || input.TaskConfig == nil {
		return nil, fmt.Errorf("invalid ExecuteBasic input: task_config is required")
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 normalizer for basic tasks
	normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeBasic)
	if err != nil {
		return nil, fmt.Errorf("failed to create basic normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(workflowState, workflowConfig, input.TaskConfig)
	// Don't inject raw TaskConfig.With before normalization - this causes circular dependency
	// The workflow-level .input should be preserved for template processing

	// Normalize the task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize basic task: %w", err)
	}
	// AFTER normalization - add rendered with: as .input for downstream use
	// This makes the normalized (template-processed) values available to agents/sub-tasks
	if normalizedConfig.With != nil {
		normContext.CurrentInput = normalizedConfig.With
		contextBuilder.VariableBuilder.AddCurrentInputToVariables(normContext.Variables, normalizedConfig.With)
	}
	// Validate task type
	if normalizedConfig.Type != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", normalizedConfig.Type)
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
	// Execute component
	output, executionError := a.executeUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:     normalizedConfig,
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		ProjectConfig:  a.projectConfig,
	})
	taskState.Output = output
	// Use task2 ResponseHandler for basic type
	handler, err := a.task2Factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
	if err != nil {
		return nil, fmt.Errorf("failed to create basic response handler: %w", err)
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
		return nil, fmt.Errorf("failed to handle basic response: %w", err)
	}
	// Convert shared.ResponseOutput to task.MainTaskResponse
	mainTaskResponse := a.convertToMainTaskResponse(result)
	// If there was an execution error, return it
	if executionError != nil {
		return mainTaskResponse, executionError
	}
	return mainTaskResponse, nil
}

// convertToMainTaskResponse converts shared.ResponseOutput to task.MainTaskResponse
func (a *ExecuteBasic) convertToMainTaskResponse(result *shared.ResponseOutput) *task.MainTaskResponse {
	converter := NewResponseConverter()
	return converter.ConvertToMainTaskResponse(result)
}
