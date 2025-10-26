package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks"
	taskscore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const NormalizeWaitProcessorLabel = "NormalizeWaitProcessor"

type NormalizeWaitProcessorInput struct {
	WorkflowID       string               `json:"workflow_id"`
	WorkflowExecID   core.ID              `json:"workflow_exec_id"`
	ProcessorConfig  *task.Config         `json:"processor_config"`
	ParentTaskConfig *task.Config         `json:"parent_task_config"` // NEW: Enable inheritance
	Signal           *task.SignalEnvelope `json:"signal"`
}

type NormalizeWaitProcessor struct {
	loadWorkflowUC *uc.LoadWorkflow
	workflowRepo   workflow.Repository
	taskRepo       task.Repository
}

// NewNormalizeWaitProcessor creates a new NormalizeWaitProcessor activity
func NewNormalizeWaitProcessor(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *NormalizeWaitProcessor {
	return &NormalizeWaitProcessor{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		workflowRepo:   workflowRepo,
		taskRepo:       taskRepo,
	}
}

func (a *NormalizeWaitProcessor) Run(ctx context.Context, input *NormalizeWaitProcessorInput) (*task.Config, error) {
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow context: %w", err)
	}
	normalizedConfig, err := input.ProcessorConfig.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone processor config: %w", err)
	}
	if err := shared.InheritTaskConfig(normalizedConfig, input.ParentTaskConfig); err != nil {
		return nil, fmt.Errorf("failed to inherit task config: %w", err)
	}
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	envMerger := taskscore.NewEnvMerger()
	factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
		TemplateEngine: engine,
		EnvMerger:      envMerger,
		WorkflowRepo:   a.workflowRepo,
		TaskRepo:       a.taskRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create normalizer factory: %w", err)
	}
	orchestrator, err := tasks.NewConfigOrchestrator(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("failed to create config orchestrator: %w", err)
	}
	err = orchestrator.NormalizeTaskWithSignal(ctx, normalizedConfig, workflowState, workflowConfig, input.Signal)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize processor config with signal context: %w", err)
	}
	if normalizedConfig.Type == "" {
		normalizedConfig.Type = task.TaskTypeBasic
	}
	return normalizedConfig, nil
}
