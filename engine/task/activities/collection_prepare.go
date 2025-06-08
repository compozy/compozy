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

const PrepareCollectionLabel = "PrepareCollection"
const EvaluateDynamicItemsLabel = "EvaluateDynamicItems"

type PrepareCollectionInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type PrepareCollectionResult struct {
	TaskExecID      core.ID     `json:"task_exec_id"`     // Collection task execution ID
	FilteredCount   int         `json:"filtered_count"`   // Number of items after filtering
	TotalCount      int         `json:"total_count"`      // Original number of items
	BatchCount      int         `json:"batch_count"`      // Number of batches to process
	CollectionState *task.State `json:"collection_state"` // Collection state stored in DB
}

type EvaluateDynamicItemsInput struct {
	WorkflowID       string       `json:"workflow_id"`
	WorkflowExecID   core.ID      `json:"workflow_exec_id"`
	ItemsExpression  string       `json:"items_expression"`
	FilterExpression string       `json:"filter_expression,omitempty"`
	TaskConfig       *task.Config `json:"task_config"`
}

type PrepareCollection struct {
	loadWorkflowUC    *uc.LoadWorkflow
	prepareCollectionUC *uc.PrepareCollection
}

type EvaluateDynamicItems struct {
	loadWorkflowUC      *uc.LoadWorkflow
	collectionEvaluator *uc.CollectionEvaluator
}

// NewPrepareCollection creates a new PrepareCollection activity
func NewPrepareCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *PrepareCollection {
	return &PrepareCollection{
		loadWorkflowUC:    uc.NewLoadWorkflow(workflows, workflowRepo),
		prepareCollectionUC: uc.NewPrepareCollection(taskRepo),
	}
}

// NewEvaluateDynamicItems creates a new EvaluateDynamicItems activity
func NewEvaluateDynamicItems(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
) *EvaluateDynamicItems {
	return &EvaluateDynamicItems{
		loadWorkflowUC:      uc.NewLoadWorkflow(workflows, workflowRepo),
		collectionEvaluator: uc.NewCollectionEvaluator(),
	}
}

func (a *PrepareCollection) Run(ctx context.Context, input *PrepareCollectionInput) (*PrepareCollectionResult, error) {
	// Load and validate workflow context
	workflowState, workflowConfig, err := a.loadAndValidateWorkflow(ctx, input)
	if err != nil {
		return nil, err
	}

	// Use the prepare collection use case
	result, err := a.prepareCollectionUC.Execute(ctx, &uc.PrepareCollectionInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}

	return &PrepareCollectionResult{
		TaskExecID:      result.TaskExecID,
		FilteredCount:   result.FilteredCount,
		TotalCount:      result.TotalCount,
		BatchCount:      result.BatchCount,
		CollectionState: result.CollectionState,
	}, nil
}

func (a *PrepareCollection) loadAndValidateWorkflow(
	ctx context.Context,
	input *PrepareCollectionInput,
) (*workflow.State, *workflow.Config, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, nil, err
	}

	// Normalize task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, nil, err
	}

	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeCollection {
		return nil, nil, fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}

	return workflowState, workflowConfig, nil
}



func (a *EvaluateDynamicItems) Run(ctx context.Context, input *EvaluateDynamicItemsInput) ([]any, error) {
	// Load current workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}

	// Build evaluation context using normalizer
	contextBuilder := normalizer.NewContextBuilder()
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*input.TaskConfig})

	// Merge environments: workflow -> task
	envMerger := &core.EnvMerger{}
	mergedEnv, err := envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		input.TaskConfig.GetEnv(),
	)
	if err != nil {
		// If merge fails, use task env only
		mergedEnv = input.TaskConfig.GetEnv()
	}

	normCtx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   input.TaskConfig.With,
		MergedEnv:      &mergedEnv,
	}

	evaluationContext := contextBuilder.BuildContext(normCtx)

	// Evaluate items using shared service
	evalInput := &uc.EvaluationInput{
		ItemsExpr:  input.ItemsExpression,
		FilterExpr: input.FilterExpression,
		Context:    evaluationContext,
		ItemVar:    input.TaskConfig.GetItemVar(),
		IndexVar:   input.TaskConfig.GetIndexVar(),
	}

	result, err := a.collectionEvaluator.EvaluateItems(ctx, evalInput)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate items: %w", err)
	}

	return result.Items, nil
}
