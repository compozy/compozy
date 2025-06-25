package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
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

func (tea *TemplateEngineAdapter) Process(template string, context map[string]any) (any, error) {
	result, err := tea.engine.ProcessString(template, context)
	if err != nil {
		return nil, err
	}
	// Return JSON parsed value for JSON format, otherwise return text
	if result.JSON != nil {
		return result.JSON, nil
	}
	return result.Text, nil
}

func (tea *TemplateEngineAdapter) ProcessMap(
	templateMap map[string]any,
	context map[string]any,
) (map[string]any, error) {
	result, err := tea.engine.ParseMap(templateMap, context)
	if err != nil {
		return nil, err
	}
	// ParseMap returns any, need to type assert to map[string]any
	if resultMap, ok := result.(map[string]any); ok {
		return resultMap, nil
	}
	return nil, fmt.Errorf("ParseMap result is not a map[string]any, got %T", result)
}

// NormalizationContextAdapter creates a normalization context from workflow state
type NormalizationContextAdapter struct {
	workflowState  *wf.State
	workflowConfig *wf.Config
}

func (nca *NormalizationContextAdapter) BuildTemplateContext() map[string]any {
	context := map[string]any{
		"env":              make(map[string]any),
		"workflow":         nca.workflowState,
		"config":           nca.workflowConfig,
		"tasks":            nca.workflowState.Tasks,
		"workflow_id":      nca.workflowState.WorkflowID,
		"workflow_exec_id": nca.workflowState.WorkflowExecID,
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
	templateEngineAdapter := &TemplateEngineAdapter{engine: engine}
	return &CompleteWorkflow{
		workflowRepo:      workflowRepo,
		workflows:         workflowMap,
		outputTransformer: wf.NewOutputNormalizer(templateEngineAdapter),
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
