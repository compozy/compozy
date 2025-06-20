package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

const NormalizeWaitProcessorLabel = "NormalizeWaitProcessor"

type NormalizeWaitProcessorInput struct {
	WorkflowID      string               `json:"workflow_id"`
	WorkflowExecID  core.ID              `json:"workflow_exec_id"`
	ProcessorConfig *task.Config         `json:"processor_config"`
	Signal          *task.SignalEnvelope `json:"signal"`
}

type NormalizeWaitProcessor struct {
	loadWorkflowUC *uc.LoadWorkflow
}

// NewNormalizeWaitProcessor creates a new NormalizeWaitProcessor activity
func NewNormalizeWaitProcessor(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
) *NormalizeWaitProcessor {
	return &NormalizeWaitProcessor{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
	}
}

func (a *NormalizeWaitProcessor) Run(ctx context.Context, input *NormalizeWaitProcessorInput) (*task.Config, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow context: %w", err)
	}

	// Create a copy of the processor config to avoid mutating the original
	normalizedConfig, err := input.ProcessorConfig.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone processor config: %w", err)
	}

	// Build normalization context with full workflow state and config
	norm := normalizer.New()
	normCtx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    normalizer.BuildTaskConfigsMap(workflowConfig.Tasks),
	}

	// Normalize the processor config with signal context
	err = norm.NormalizeTaskConfigWithSignal(normalizedConfig, normCtx, input.Signal)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize processor config with signal context: %w", err)
	}
	// Ensure processor has a type - default to basic if not set
	if normalizedConfig.Type == "" {
		normalizedConfig.Type = task.TaskTypeBasic
	}
	return normalizedConfig, nil
}
