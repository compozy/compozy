package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
	"github.com/compozy/compozy/pkg/tplengine"
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
	loadWorkflowUC   *uc.LoadWorkflow
	createStateUC    *uc.CreateState
	handleResponseUC *uc.HandleResponse
	taskRepo         task.Repository
	normalizer       *normalizer.Normalizer
}

type EvaluateDynamicItems struct {
	loadWorkflowUC *uc.LoadWorkflow
	normalizer     *normalizer.Normalizer
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
		taskRepo:         taskRepo,
		normalizer:       normalizer.New(),
	}
}

// NewEvaluateDynamicItems creates a new EvaluateDynamicItems activity
func NewEvaluateDynamicItems(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
) *EvaluateDynamicItems {
	return &EvaluateDynamicItems{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		normalizer:     normalizer.New(),
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

	// Normalize task config
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	}
	err = normalizer.Execute(ctx, normalizeInput)
	if err != nil {
		return nil, err
	}

	// Validate task type
	taskConfig := input.TaskConfig
	if taskConfig.Type != task.TaskTypeCollection {
		return nil, fmt.Errorf("unsupported task type: %s", taskConfig.Type)
	}

	// Evaluate collection items - now handles both static and dynamic
	items, isDynamic, err := a.evaluateCollectionItemsWithDynamic(taskConfig, workflowState)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate collection items: %w", err)
	}

	var filteredItems []any
	var batchCount int

	if isDynamic && len(items) == 0 {
		// Dynamic items couldn't be resolved yet - create placeholder
		filteredItems = []any{}
		batchCount = 1
	} else {
		// Apply filter if present (static items or resolved dynamic items)
		filteredItems, err = a.applyFilter(items, taskConfig, workflowState)
		if err != nil {
			return nil, fmt.Errorf("failed to apply filter: %w", err)
		}

		// Calculate batch information
		mode := taskConfig.GetMode()
		batchSize := taskConfig.GetBatch()
		batchCount = a.calculateBatchCount(len(filteredItems), batchSize, mode)
	}

	// Create collection state
	collectionState, err := a.createCollectionState(
		workflowState,
		workflowConfig,
		taskConfig,
		filteredItems,
		isDynamic && len(items) == 0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection state: %w", err)
	}

	return &PrepareCollectionResult{
		TaskExecID:      collectionState.TaskExecID,
		FilteredCount:   len(filteredItems),
		TotalCount:      len(items),
		BatchCount:      batchCount,
		CollectionState: collectionState,
	}, nil
}

// evaluateCollectionItemsWithDynamic attempts to evaluate both static and dynamic items
func (a *PrepareCollection) evaluateCollectionItemsWithDynamic(
	taskConfig *task.Config,
	workflowState *workflow.State,
) ([]any, bool, error) {
	itemsExpr := taskConfig.Items
	if itemsExpr == "" {
		return nil, false, fmt.Errorf("items expression is empty")
	}

	// Check if items expression references previous task outputs (dynamic items)
	if a.isDynamicItemsExpression(itemsExpr) {
		// Try to evaluate dynamic items first
		items, err := a.evaluateDynamicItems(taskConfig, workflowState, itemsExpr)
		if err != nil {
			// If evaluation fails, return empty items with dynamic flag
			// This will be resolved during execution
			return []any{}, true, nil
		}
		return items, true, nil
	}

	// Static items - evaluate immediately using existing logic
	items, err := a.evaluateStaticItems(taskConfig, workflowState, itemsExpr)
	if err != nil {
		return nil, false, err
	}
	return items, false, nil
}

// evaluateDynamicItems attempts to evaluate dynamic items expressions
func (a *PrepareCollection) evaluateDynamicItems(
	taskConfig *task.Config,
	workflowState *workflow.State,
	itemsExpr string,
) ([]any, error) {
	// Create template evaluation context using existing normalizer patterns
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})
	ctx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: nil, // Will be set by caller if needed
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
	}

	// Build template context
	context := a.normalizer.BuildContext(ctx)

	// Use template engine to evaluate the items expression
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	result, err := engine.ParseMap(itemsExpr, context)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate dynamic items expression '%s': %w", itemsExpr, err)
	}

	// Convert result to []any
	switch items := result.(type) {
	case []any:
		return items, nil
	case string:
		// If result is a string, try to parse it as JSON
		if items == "" {
			return []any{}, nil
		}
		return nil, fmt.Errorf("dynamic items expression evaluated to string instead of array: %s", items)
	default:
		return nil, fmt.Errorf("dynamic items expression must evaluate to an array, got %T", result)
	}
}

// isDynamicItemsExpression checks if the items expression references previous task outputs
func (a *PrepareCollection) isDynamicItemsExpression(itemsExpr string) bool {
	// Check for patterns that reference previous task outputs
	// This covers expressions like "{{ .tasks.some_task.output.results }}"
	return len(itemsExpr) > 6 && itemsExpr[0:2] == "{{" &&
		(itemsExpr[3:9] == ".tasks" || (len(itemsExpr) > 10 && itemsExpr[4:10] == ".tasks"))
}

// evaluateStaticItems evaluates static items expressions (non-dynamic)
func (a *PrepareCollection) evaluateStaticItems(
	taskConfig *task.Config,
	workflowState *workflow.State,
	itemsExpr string,
) ([]any, error) {
	// Create template evaluation context using existing normalizer patterns
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})
	ctx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: nil, // Will be set by caller if needed
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
	}

	// Build template context
	context := a.normalizer.BuildContext(ctx)

	// Use template engine to evaluate the items expression
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	result, err := engine.ParseMap(itemsExpr, context)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate items expression '%s': %w", itemsExpr, err)
	}

	// Convert result to []any
	switch items := result.(type) {
	case []any:
		return items, nil
	case string:
		// If result is a string, try to parse it as JSON
		if items == "" {
			return []any{}, nil
		}
		return nil, fmt.Errorf("items expression evaluated to string instead of array: %s", items)
	default:
		return nil, fmt.Errorf("items expression must evaluate to an array, got %T", result)
	}
}

func (a *PrepareCollection) applyFilter(
	items []any,
	taskConfig *task.Config,
	workflowState *workflow.State,
) ([]any, error) {
	filter := taskConfig.Filter
	if filter == "" {
		return items, nil // No filter, return all items
	}

	// Create template evaluation context
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})
	engine := tplengine.NewEngine(tplengine.FormatJSON)

	var filteredItems []any
	itemVar := taskConfig.GetItemVar()
	indexVar := taskConfig.GetIndexVar()

	for i, item := range items {
		// Create context for this specific item
		ctx := &normalizer.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: nil,
			TaskConfigs:    taskConfigs,
			ParentConfig:   nil,
			CurrentInput:   taskConfig.With,
			MergedEnv:      taskConfig.Env,
		}

		// Build base context and add item-specific variables
		context := a.normalizer.BuildContext(ctx)
		context[itemVar] = item
		context[indexVar] = i

		// Evaluate filter expression for this item
		result, err := engine.ParseMap(filter, context)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
		}

		// Check if result is truthy
		include := false
		switch filterResult := result.(type) {
		case bool:
			include = filterResult
		case string:
			include = filterResult == "true"
		case int:
			include = filterResult != 0
		case float64:
			include = filterResult != 0
		default:
			include = filterResult != nil
		}

		if include {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

func (a *PrepareCollection) calculateBatchCount(itemCount, batchSize int, mode task.CollectionMode) int {
	if mode == task.CollectionModeSequential && batchSize > 0 {
		return (itemCount + batchSize - 1) / batchSize // Ceiling division
	}
	return 1 // For parallel mode, treat as single batch
}

func (a *PrepareCollection) createCollectionState(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	filteredItems []any,
	isDynamic bool,
) (*task.State, error) {
	collectionTask := &taskConfig.CollectionTask

	var partialState *task.PartialState

	if isDynamic {
		// Create dynamic collection state that will be evaluated at execution time
		partialState = task.CreateDynamicCollectionPartialState(
			taskConfig.Items, // Store the unevaluated expression
			collectionTask.Filter,
			string(collectionTask.GetMode()),
			collectionTask.GetBatch(),
			collectionTask.ContinueOnError,
			collectionTask.GetItemVar(),
			collectionTask.GetIndexVar(),
			nil, // ParallelConfig will be set if needed
			taskConfig.Env,
		)
	} else {
		// Create static collection state with evaluated items
		partialState = task.CreateCollectionPartialState(
			filteredItems,
			collectionTask.Filter,
			string(collectionTask.GetMode()),
			collectionTask.GetBatch(),
			collectionTask.ContinueOnError,
			collectionTask.GetItemVar(),
			collectionTask.GetIndexVar(),
			nil, // ParallelConfig will be set if needed
			taskConfig.Env,
		)
	}

	// Create and persist the collection state
	taskExecID := core.MustNewID()
	stateInput := &task.CreateStateInput{
		WorkflowID:     workflowConfig.ID,
		WorkflowExecID: workflowState.WorkflowExecID,
		TaskID:         taskConfig.ID,
		TaskExecID:     taskExecID,
	}

	collectionState := task.CreateCollectionState(stateInput, partialState)

	// Persist the collection state to the database
	if err := a.taskRepo.UpsertState(context.Background(), collectionState); err != nil {
		return nil, fmt.Errorf("failed to persist collection state: %w", err)
	}

	return collectionState, nil
}

func (a *EvaluateDynamicItems) Run(ctx context.Context, input *EvaluateDynamicItemsInput) ([]any, error) {
	// Load current workflow state
	workflowState, _, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}

	// Evaluate the items expression with current workflow state
	items, err := a.evaluateDynamicItems(input.TaskConfig, workflowState, input.ItemsExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate items expression: %w", err)
	}

	// Apply filter if present
	if input.FilterExpression != "" {
		filteredItems, err := a.applyFilter(items, input.TaskConfig, workflowState, input.FilterExpression)
		if err != nil {
			return nil, fmt.Errorf("failed to apply filter: %w", err)
		}
		return filteredItems, nil
	}

	return items, nil
}

func (a *EvaluateDynamicItems) evaluateDynamicItems(
	taskConfig *task.Config,
	workflowState *workflow.State,
	itemsExpr string,
) ([]any, error) {
	// Create template evaluation context using existing normalizer patterns
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})
	ctx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: nil,
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
	}

	// Build template context
	context := a.normalizer.BuildContext(ctx)

	// Use template engine to evaluate the items expression
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	result, err := engine.ParseMap(itemsExpr, context)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate dynamic items expression '%s': %w", itemsExpr, err)
	}

	// Convert result to []any
	switch items := result.(type) {
	case []any:
		return items, nil
	case string:
		if items == "" {
			return []any{}, nil
		}
		return nil, fmt.Errorf("dynamic items expression evaluated to string instead of array: %s", items)
	default:
		return nil, fmt.Errorf("dynamic items expression must evaluate to an array, got %T", result)
	}
}

func (a *EvaluateDynamicItems) applyFilter(
	items []any,
	taskConfig *task.Config,
	workflowState *workflow.State,
	filter string,
) ([]any, error) {
	if filter == "" {
		return items, nil
	}

	// Create template evaluation context
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})
	engine := tplengine.NewEngine(tplengine.FormatJSON)

	var filteredItems []any
	itemVar := taskConfig.GetItemVar()
	indexVar := taskConfig.GetIndexVar()

	for i, item := range items {
		// Create context for this specific item
		ctx := &normalizer.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: nil,
			TaskConfigs:    taskConfigs,
			ParentConfig:   nil,
			CurrentInput:   taskConfig.With,
			MergedEnv:      taskConfig.Env,
		}

		// Build base context and add item-specific variables
		context := a.normalizer.BuildContext(ctx)
		context[itemVar] = item
		context[indexVar] = i

		// Evaluate filter expression for this item
		result, err := engine.ParseMap(filter, context)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filter expression for item %d: %w", i, err)
		}

		// Check if result is truthy
		include := false
		switch filterResult := result.(type) {
		case bool:
			include = filterResult
		case string:
			include = filterResult == "true"
		case int:
			include = filterResult != 0
		case float64:
			include = filterResult != 0
		default:
			include = filterResult != nil
		}

		if include {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}
