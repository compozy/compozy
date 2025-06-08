package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/ref"
)

const AggregateCollectionLabel = "AggregateCollection"

type AggregateCollectionInput struct {
	ParentTaskExecID core.ID                       `json:"parent_task_exec_id"` // Collection task ID
	ItemResults      []ExecuteCollectionItemResult `json:"item_results"`        // Individual results
	TaskConfig       *task.Config                  `json:"task_config"`
	WorkflowID       string                        `json:"workflow_id"`
	WorkflowExecID   core.ID                       `json:"workflow_exec_id"`
}

type AggregateCollection struct {
	loadWorkflowUC   *uc.LoadWorkflow
	handleResponseUC *uc.HandleResponse
	taskRepo         task.Repository
}

// NewAggregateCollection creates a new AggregateCollection activity
func NewAggregateCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *AggregateCollection {
	return &AggregateCollection{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
		taskRepo:         taskRepo,
	}
}

func (a *AggregateCollection) Run(ctx context.Context, input *AggregateCollectionInput) (*task.Response, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Load the collection task state
	collectionState, err := a.taskRepo.GetState(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load collection state: %w", err)
	}

	if !collectionState.IsCollection() || collectionState.CollectionState == nil {
		return nil, fmt.Errorf("task is not a collection task")
	}

	// Aggregate results
	results := make([]map[string]any, 0, len(input.ItemResults))
	summary := map[string]any{
		"total_items":    collectionState.CollectionState.SkippedCount + len(input.ItemResults),
		"filtered_items": len(input.ItemResults),
		"completed":      0,
		"failed":         0,
		"skipped":        collectionState.CollectionState.SkippedCount,
		"mode":           string(collectionState.CollectionState.Mode),
		"strategy":       string(collectionState.CollectionState.Strategy),
	}

	// Process each item result
	for _, itemResult := range input.ItemResults {
		// Get the item data from collection state
		var item any
		if itemResult.ItemIndex < len(collectionState.CollectionState.Items) {
			item = collectionState.CollectionState.Items[itemResult.ItemIndex]
		}

		result := map[string]any{
			"index":  itemResult.ItemIndex,
			"item":   item,
			"status": string(itemResult.Status),
		}

		if itemResult.Output != nil {
			result["output"] = *itemResult.Output
		}

		if itemResult.Error != nil {
			result["error"] = itemResult.Error.Message
		}

		results = append(results, result)

		// Update summary counts
		switch itemResult.Status {
		case core.StatusSuccess:
			summary["completed"] = summary["completed"].(int) + 1
		case core.StatusFailed:
			summary["failed"] = summary["failed"].(int) + 1
		}
	}

	// Check stop condition if provided
	shouldStop := false
	if input.TaskConfig.StopCondition != "" {
		workflowMap, err := workflowState.AsMap()
		if err != nil {
			return nil, fmt.Errorf("failed to convert workflow state to map: %w", err)
		}
		evaluator := ref.NewEvaluator(
			ref.WithGlobalScope(map[string]any{
				"workflow": workflowMap,
				"summary":  summary,
				"results":  results,
			}),
		)

		stopResult, err := evaluator.Eval(input.TaskConfig.StopCondition)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate stop condition: %w", err)
		}

		if stop, ok := stopResult.(bool); ok {
			shouldStop = stop
		}
	}

	// Create final output
	output := &core.Output{
		"results": results,
		"summary": summary,
	}

	// Determine final status based on strategy and results
	finalStatus := a.determineFinalStatus(
		collectionState.CollectionState.Strategy,
		summary["completed"].(int),
		summary["failed"].(int),
		len(input.ItemResults),
		shouldStop,
	)

	// Update collection state
	collectionState.Status = finalStatus
	collectionState.Output = output
	collectionState.CollectionState.CompletedCount = summary["completed"].(int)
	collectionState.CollectionState.FailedCount = summary["failed"].(int)
	collectionState.CollectionState.ProcessedCount = len(input.ItemResults)

	// Store item result IDs
	itemResultIDs := make([]string, len(input.ItemResults))
	for i, result := range input.ItemResults {
		itemResultIDs[i] = string(result.TaskExecID)
	}
	collectionState.CollectionState.ItemResults = itemResultIDs

	// Handle response and return
	response, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      collectionState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
		ExecutionError: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}

	return response, nil
}

func (a *AggregateCollection) determineFinalStatus(
	strategy task.ParallelStrategy,
	completed, failed, total int,
	shouldStop bool,
) core.StatusType {
	if shouldStop {
		return core.StatusFailed
	}

	switch strategy {
	case task.StrategyWaitAll:
		if failed > 0 {
			return core.StatusFailed
		}
		if completed == total {
			return core.StatusSuccess
		}
		return core.StatusRunning

	case task.StrategyFailFast:
		if failed > 0 {
			return core.StatusFailed
		}
		if completed == total {
			return core.StatusSuccess
		}
		return core.StatusRunning

	case task.StrategyBestEffort:
		if (completed + failed) == total {
			if completed > 0 {
				return core.StatusSuccess
			}
			return core.StatusFailed
		}
		return core.StatusRunning

	case task.StrategyRace:
		if completed > 0 {
			return core.StatusSuccess
		}
		if failed == total {
			return core.StatusFailed
		}
		return core.StatusRunning

	default:
		// Default to wait_all behavior
		if failed > 0 {
			return core.StatusFailed
		}
		if completed == total {
			return core.StatusSuccess
		}
		return core.StatusRunning
	}
}
