package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/compozy/compozy/pkg/tplengine"
)

const PrepareCollectionLabel = "PrepareCollection"

type PrepareCollectionInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type PrepareCollectionResult struct {
	TaskExecID     core.ID     `json:"task_exec_id"`     // Collection task execution ID
	FilteredCount  int         `json:"filtered_count"`   // Number of items after filtering
	TotalCount     int         `json:"total_count"`      // Original number of items
	BatchCount     int         `json:"batch_count"`      // Number of batches to process
	CollectionState *task.State `json:"collection_state"` // Collection state stored in DB
}

type PrepareCollection struct {
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	handleResponseUC *uc.HandleResponse
}

// NewPrepareCollection creates a new PrepareCollection activity
func NewPrepareCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *PrepareCollection {
	return &PrepareCollection{
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:    uc.NewCreateState(taskRepo),
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
	}
}

func (a *PrepareCollection) Run(ctx context.Context, input *PrepareCollectionInput) (*PrepareCollectionResult, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Note: Do not normalize task config here as it contains item variables
	// that are not available yet. Normalization will happen during item execution.

	// Validate task
	taskConfig := input.TaskConfig
	taskType := taskConfig.Type
	if taskType != task.TaskTypeCollection {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}

	// Evaluate items expression to get collection
	var items []any
	
	// For simple cases like "{{ .input.cities }}", we can directly access the data
	if taskConfig.Items == "{{ .input.cities }}" {
		if workflowState.Input == nil {
			return nil, fmt.Errorf("workflow input is nil")
		}
		citiesVal, ok := (*workflowState.Input)["cities"]
		if !ok {
			return nil, fmt.Errorf("cities field not found in workflow input")
		}
		items, ok = citiesVal.([]any)
		if !ok {
			return nil, fmt.Errorf("cities field must be an array, got %T", citiesVal)
		}
	} else {
		// For more complex expressions, use template engine
		templateEngine := tplengine.NewEngine(tplengine.FormatText)
		context := map[string]any{
			"workflow": workflowState,
			"input":    workflowState.Input,
		}

		result, err := templateEngine.ProcessString(taskConfig.Items, context)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate items expression: %w", err)
		}

		// The result should be a JSON string, parse it
		var itemsData any
		if err := json.Unmarshal([]byte(result.Text), &itemsData); err != nil {
			return nil, fmt.Errorf("failed to parse items expression result as JSON: %w", err)
		}

		var ok bool
		items, ok = itemsData.([]any)
		if !ok {
			return nil, fmt.Errorf("items expression must evaluate to an array, got %T", itemsData)
		}
	}

	totalCount := len(items)
	filteredItems := items

	// Apply filter if provided
	if taskConfig.Filter != "" {
		filteredItems = make([]any, 0)
		for i, item := range items {
			// Set up context for filter evaluation
			workflowMapFilter, err := workflowState.AsMap()
			if err != nil {
				return nil, fmt.Errorf("failed to convert workflow state to map for filter: %w", err)
			}
			filterEvaluator := ref.NewEvaluator(
				ref.WithGlobalScope(map[string]any{
					"workflow":              workflowMapFilter,
					taskConfig.GetItemVar(): item,
					taskConfig.GetIndexVar(): i,
				}),
			)

			shouldInclude, err := filterEvaluator.Eval(taskConfig.Filter)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
			}

			if include, ok := shouldInclude.(bool); ok && include {
				filteredItems = append(filteredItems, item)
			}
		}
	}

	filteredCount := len(filteredItems)
	batchSize := taskConfig.GetBatch()
	batchCount := (filteredCount + batchSize - 1) / batchSize // Ceiling division

	// Create collection state
	collectionState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	})
	if err != nil {
		return nil, err
	}

	// Store filtered items in the collection state
	if collectionState.CollectionState != nil {
		collectionState.CollectionState.Items = filteredItems
		collectionState.CollectionState.ProcessedCount = 0
		collectionState.CollectionState.CompletedCount = 0
		collectionState.CollectionState.FailedCount = 0
		collectionState.CollectionState.SkippedCount = totalCount - filteredCount
	}

	return &PrepareCollectionResult{
		TaskExecID:      collectionState.TaskExecID,
		FilteredCount:   filteredCount,
		TotalCount:      totalCount,
		BatchCount:      batchCount,
		CollectionState: collectionState,
	}, nil
}