package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task2/shared"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// TemplateEngineAdapter adapts tplengine to workflow.TemplateEngine interface
type TemplateEngineAdapter struct {
	engine *tplengine.TemplateEngine
}

func (tea *TemplateEngineAdapter) ParseMap(value any, context map[string]any) (any, error) {
	return tea.engine.ParseAny(value, context)
}

// NormalizationContextAdapter creates a normalization context from workflow state
type NormalizationContextAdapter struct {
	workflowState  *wf.State
	workflowConfig *wf.Config
}

func (nca *NormalizationContextAdapter) BuildTemplateContext() map[string]any {
	// Use the shared context builder to build a proper context with children support
	contextBuilder := shared.NewBaseContextBuilder()
	normContext := contextBuilder.BuildContext(nca.workflowState, nca.workflowConfig, nil)

	// Get the variables which contain the properly built context
	if normContext.Variables != nil {
		// The variables already contain workflow, tasks with children, env, etc.
		// Add additional top-level fields for backward compatibility
		normContext.Variables["workflow_id"] = nca.workflowState.WorkflowID
		normContext.Variables["workflow_exec_id"] = nca.workflowState.WorkflowExecID
		normContext.Variables["status"] = nca.workflowState.Status
		normContext.Variables["config"] = nca.workflowConfig

		return normContext.Variables
	}

	// Fallback to original implementation if context builder fails
	workflowMap := map[string]any{
		"id":      nca.workflowState.WorkflowID,
		"exec_id": nca.workflowState.WorkflowExecID,
		"status":  nca.workflowState.Status,
	}

	// Add input if available
	if nca.workflowState.Input != nil {
		workflowMap["input"] = *nca.workflowState.Input
	}

	// Add output if available
	if nca.workflowState.Output != nil {
		workflowMap["output"] = *nca.workflowState.Output
	}

	context := map[string]any{
		"env":              make(map[string]any),
		"workflow":         workflowMap,
		"config":           nca.workflowConfig,
		"tasks":            nca.workflowState.Tasks,
		"workflow_id":      nca.workflowState.WorkflowID,
		"workflow_exec_id": nca.workflowState.WorkflowExecID,
		"status":           nca.workflowState.Status,
	}

	// Add workflow environment if available
	if envMap := nca.workflowConfig.GetEnv(); envMap != nil {
		context["env"] = envMap.AsMap()
	}
	return context
}

const CompleteWorkflowLabel = "CompleteWorkflow"

type CompleteWorkflowInput struct {
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

type CompleteWorkflow struct {
	workflowRepo      wf.Repository
	workflows         map[string]*wf.Config
	outputTransformer *wf.OutputNormalizer
}

func NewCompleteWorkflow(workflowRepo wf.Repository, workflows []*wf.Config) *CompleteWorkflow {
	workflowMap := make(map[string]*wf.Config, len(workflows))
	for _, wf := range workflows {
		workflowMap[wf.ID] = wf
	}
	// Create template engine for output transformation
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	return &CompleteWorkflow{
		workflowRepo:      workflowRepo,
		workflows:         workflowMap,
		outputTransformer: wf.NewOutputNormalizer(engine),
	}
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*wf.State, error) {
	// Add heartbeat to ensure activity stays alive during retries
	activity.RecordHeartbeat(ctx, "Attempting to complete workflow")
	log := logger.FromContext(ctx)

	// Find the workflow config
	config, exists := a.workflows[input.WorkflowID]
	if !exists {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unknown workflow ID: %s", input.WorkflowID),
			"unknown_workflow_id",
			nil,
		)
	}

	// Create transformer if outputs are defined
	var transformer wf.OutputTransformer
	if config.GetOutputs() != nil {
		transformer = func(state *wf.State) (*core.Output, error) {
			// Create normalization context
			normCtx := &NormalizationContextAdapter{
				workflowState:  state,
				workflowConfig: config,
			}
			output, err := a.outputTransformer.TransformWorkflowOutput(state, config.GetOutputs(), normCtx)
			if err != nil {
				log.Error("Output transformation failed",
					"workflow_id", state.WorkflowID,
					"workflow_exec_id", state.WorkflowExecID,
					"error", err)
				return nil, temporal.NewNonRetryableApplicationError(
					fmt.Sprintf("failed to normalize workflow output: %v", err),
					"normalization_failed",
					err,
				)
			}
			return output, nil
		}
	}

	// Complete workflow with optional transformer
	state, err := a.workflowRepo.CompleteWorkflow(ctx, input.WorkflowExecID, transformer)
	if err != nil {
		// Check if this is the specific error indicating tasks are not ready for completion
		if err == store.ErrWorkflowNotReady {
			// Create a retryable application error to trigger Temporal's retry mechanism
			return nil, temporal.NewApplicationError(
				fmt.Sprintf("workflow %s not ready for completion, tasks still running", input.WorkflowExecID),
				"workflow_not_ready",
				err,
			)
		}
		return nil, fmt.Errorf("failed to complete workflow %s: %w", input.WorkflowExecID, err)
	}
	return state, nil
}
