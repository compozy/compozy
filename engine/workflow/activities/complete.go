package activities

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
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
	workflowsMu       sync.RWMutex // Protects workflows map from concurrent access
	outputTransformer *wf.OutputNormalizer
	projectConfig     *project.Config
}

func NewCompleteWorkflow(
	workflowRepo wf.Repository,
	workflows []*wf.Config,
	projectCfg *project.Config,
) *CompleteWorkflow {
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
		projectConfig:     projectCfg,
	}
}

// findWorkflowConfig safely looks up or reloads workflow configuration with race protection
func (a *CompleteWorkflow) findWorkflowConfig(ctx context.Context, workflowID string) *wf.Config {
	// First, try to read under read lock
	a.workflowsMu.RLock()
	if config, exists := a.workflows[workflowID]; exists {
		a.workflowsMu.RUnlock()
		return config
	}
	a.workflowsMu.RUnlock()
	// Attempt to lazily reload workflows from project
	return a.reloadWorkflowsIfNeeded(ctx, workflowID)
}

// reloadWorkflowsIfNeeded performs double-checked locking for workflow reloading
func (a *CompleteWorkflow) reloadWorkflowsIfNeeded(
	ctx context.Context,
	workflowID string,
) *wf.Config {
	if a.projectConfig == nil {
		return nil
	}
	// Acquire write lock for potential reload
	a.workflowsMu.Lock()
	defer a.workflowsMu.Unlock()
	// Double-check existence after acquiring write lock
	if config, exists := a.workflows[workflowID]; exists {
		return config
	}
	// Perform the expensive reload operation
	newWorkflows, err := a.loadWorkflowsFromProject(ctx)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to reload workflows for completion", "error", err)
		return nil
	}
	// Swap the map under write lock
	a.workflows = newWorkflows
	return a.workflows[workflowID]
}

// loadWorkflowsFromProject loads workflows from project configuration
func (a *CompleteWorkflow) loadWorkflowsFromProject(ctx context.Context) (map[string]*wf.Config, error) {
	// Honor cancellation before expensive I/O
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	reloaded, err := wf.WorkflowsFromProject(ctx, a.projectConfig)
	if err != nil {
		return nil, err
	}
	// Build new map off-lock to minimize lock hold time
	newWorkflows := make(map[string]*wf.Config, len(reloaded))
	for _, wcfg := range reloaded {
		newWorkflows[wcfg.ID] = wcfg
	}
	return newWorkflows, nil
}

// createOutputTransformer creates output transformer if workflow has outputs defined
func (a *CompleteWorkflow) createOutputTransformer(
	ctx context.Context,
	config *wf.Config,
) wf.OutputTransformer {
	if config == nil || config.GetOutputs() == nil {
		return nil
	}
	return func(state *wf.State) (*core.Output, error) {
		normCtx := &NormalizationContextAdapter{
			workflowState:  state,
			workflowConfig: config,
		}
		output, err := a.outputTransformer.TransformWorkflowOutput(state, config.GetOutputs(), normCtx)
		if err != nil {
			logger.FromContext(ctx).Error("Output transformation failed",
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

// completeWorkflowWithRetry handles workflow completion with retry logic
func (a *CompleteWorkflow) completeWorkflowWithRetry(ctx context.Context, workflowExecID core.ID,
	transformer wf.OutputTransformer) (*wf.State, error) {
	state, err := a.workflowRepo.CompleteWorkflow(ctx, workflowExecID, transformer)
	if err != nil {
		if errors.Is(err, store.ErrWorkflowNotReady) {
			return nil, temporal.NewApplicationError(
				fmt.Sprintf("workflow %s not ready for completion, tasks still running", workflowExecID),
				"workflow_not_ready",
				err,
			)
		}
		return nil, fmt.Errorf("failed to complete workflow %s: %w", workflowExecID, err)
	}
	return state, nil
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*wf.State, error) {
	// Add heartbeat to ensure activity stays alive during retries
	activity.RecordHeartbeat(ctx, "Attempting to complete workflow")
	log := logger.FromContext(ctx)
	// Find the workflow config with race-safe access
	config := a.findWorkflowConfig(ctx, input.WorkflowID)
	if config == nil {
		log.Warn("Workflow config not found during completion; proceeding without output normalization",
			"workflow_id", input.WorkflowID,
			"workflow_exec_id", input.WorkflowExecID)
	}
	// Create transformer if outputs are defined
	transformer := a.createOutputTransformer(ctx, config)
	// Complete workflow with optional transformer
	return a.completeWorkflowWithRetry(ctx, input.WorkflowExecID, transformer)
}
