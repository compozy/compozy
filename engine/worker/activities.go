package worker

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/memory"
	memacts "github.com/compozy/compozy/engine/memory/activities"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	tkfacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	wkacts "github.com/compozy/compozy/engine/worker/activities"
	"github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

type Activities struct {
	projectConfig    *project.Config
	workflows        []*workflow.Config
	workflowRepo     workflow.Repository
	taskRepo         task.Repository
	runtime          *runtime.Manager
	configStore      services.ConfigStore
	signalDispatcher services.SignalDispatcher
	configManager    *services.ConfigManager
	redisCache       *cache.Cache
	celEvaluator     task.ConditionEvaluator
	memoryManager    *memory.Manager
	memoryActivities *memacts.MemoryActivities
	templateEngine   *tplengine.TemplateEngine
	logger           logger.Logger
}

func NewActivities(
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
	configStore services.ConfigStore,
	signalDispatcher services.SignalDispatcher,
	configManager *services.ConfigManager,
	redisCache *cache.Cache,
	memoryManager *memory.Manager,
	templateEngine *tplengine.TemplateEngine,
) *Activities {
	// Create CEL evaluator once for reuse across all activity executions
	celEvaluator, err := task.NewCELEvaluator()
	if err != nil {
		// This is a critical initialization error
		panic(fmt.Sprintf("failed to create CEL evaluator: %v", err))
	}
	// Create logger for activities
	log := logger.NewForTests() // TODO: Use proper logger from config
	// Create memory activities instance
	memoryActivities := memacts.NewMemoryActivities(memoryManager, log)
	return &Activities{
		projectConfig:    projectConfig,
		workflows:        workflows,
		workflowRepo:     workflowRepo,
		taskRepo:         taskRepo,
		runtime:          runtime,
		configStore:      configStore,
		signalDispatcher: signalDispatcher,
		configManager:    configManager,
		redisCache:       redisCache,
		celEvaluator:     celEvaluator,
		memoryManager:    memoryManager,
		memoryActivities: memoryActivities,
		templateEngine:   templateEngine,
		logger:           log,
	}
}

// -----------------------------------------------------------------------------
// Workflow
// -----------------------------------------------------------------------------

func (a *Activities) GetWorkflowData(ctx context.Context, input *wfacts.GetDataInput) (*wfacts.GetData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewGetData(a.projectConfig, a.workflows)
	return act.Run(ctx, input)
}

// TriggerWorkflow executes the activity to trigger the workflow
func (a *Activities) TriggerWorkflow(ctx context.Context, input *wfacts.TriggerInput) (*workflow.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewTrigger(a.workflows, a.workflowRepo)
	return act.Run(ctx, input)
}

// UpdateWorkflowState executes the activity to update workflow status
func (a *Activities) UpdateWorkflowState(ctx context.Context, input *wfacts.UpdateStateInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	act := wfacts.NewUpdateState(a.workflowRepo, a.taskRepo)
	return act.Run(ctx, input)
}

// CompleteWorkflow executes the activity to complete workflow with task outputs
func (a *Activities) CompleteWorkflow(
	ctx context.Context,
	input *wfacts.CompleteWorkflowInput,
) (*workflow.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := wfacts.NewCompleteWorkflow(a.workflowRepo, a.workflows)
	return act.Run(ctx, input)
}

// -----------------------------------------------------------------------------
// Task
// -----------------------------------------------------------------------------

func (a *Activities) ExecuteBasicTask(
	ctx context.Context,
	input *tkfacts.ExecuteBasicInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteBasic(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
		a.configStore,
		a.projectConfig.CWD,
		a.memoryManager,
		a.templateEngine,
		a.projectConfig,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteRouterTask(
	ctx context.Context,
	input *tkfacts.ExecuteRouterInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteRouter(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) CreateParallelState(
	ctx context.Context,
	input *tkfacts.CreateParallelStateInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewCreateParallelState(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteSubtask(
	ctx context.Context,
	input *tkfacts.ExecuteSubtaskInput,
) (*task.SubtaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewExecuteSubtask(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
		a.configStore,
	)
	return act.Run(ctx, input)
}

func (a *Activities) GetParallelResponse(
	ctx context.Context,
	input *tkfacts.GetParallelResponseInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetParallelResponse(a.workflowRepo, a.taskRepo, a.configStore)
	return act.Run(ctx, input)
}

func (a *Activities) GetProgress(
	ctx context.Context,
	input *tkfacts.GetProgressInput,
) (*task.ProgressInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetProgress(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) UpdateParentStatus(
	ctx context.Context,
	input *tkfacts.UpdateParentStatusInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewUpdateParentStatus(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) UpdateChildState(
	ctx context.Context,
	input map[string]any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	act := tkfacts.NewUpdateChildState(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) CreateCollectionState(
	ctx context.Context,
	input *tkfacts.CreateCollectionStateInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewCreateCollectionState(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) GetCollectionResponse(
	ctx context.Context,
	input *tkfacts.GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetCollectionResponse(a.workflowRepo, a.taskRepo, a.configStore)
	return act.Run(ctx, input)
}

func (a *Activities) ListChildStates(
	ctx context.Context,
	input *tkfacts.ListChildStatesInput,
) ([]*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewListChildStates(a.taskRepo)
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteAggregateTask(
	ctx context.Context,
	input *tkfacts.ExecuteAggregateInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteAggregate(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) CreateCompositeState(
	ctx context.Context,
	input *tkfacts.CreateCompositeStateInput,
) (*task.State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewCreateCompositeState(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) GetCompositeResponse(
	ctx context.Context,
	input *tkfacts.GetCompositeResponseInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewGetCompositeResponse(a.workflowRepo, a.taskRepo, a.configStore)
	return act.Run(ctx, input)
}

func (a *Activities) LoadTaskConfigActivity(
	ctx context.Context,
	input *tkfacts.LoadTaskConfigInput,
) (*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewLoadTaskConfig(a.workflows)
	return act.Run(ctx, input)
}

func (a *Activities) LoadBatchConfigsActivity(
	ctx context.Context,
	input *tkfacts.LoadBatchConfigsInput,
) (map[string]*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewLoadBatchConfigs(a.workflows)
	return act.Run(ctx, input)
}

func (a *Activities) LoadCompositeConfigsActivity(
	ctx context.Context,
	input *tkfacts.LoadCompositeConfigsInput,
) (map[string]*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewLoadCompositeConfigs(a.configManager)
	return act.Run(ctx, input)
}

func (a *Activities) LoadCollectionConfigsActivity(
	ctx context.Context,
	input *tkfacts.LoadCollectionConfigsInput,
) (map[string]*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewLoadCollectionConfigs(a.configManager)
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteSignalTask(
	ctx context.Context,
	input *tkfacts.ExecuteSignalInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteSignal(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.signalDispatcher,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteWaitTask(
	ctx context.Context,
	input *tkfacts.ExecuteWaitInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteWait(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.projectConfig.CWD,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) ExecuteMemoryTask(
	ctx context.Context,
	input *tkfacts.ExecuteMemoryInput,
) (*task.MainTaskResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act, err := tkfacts.NewExecuteMemory(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.memoryManager,
		a.projectConfig.CWD,
		a.templateEngine,
	)
	if err != nil {
		return nil, err
	}
	return act.Run(ctx, input)
}

func (a *Activities) NormalizeWaitProcessor(
	ctx context.Context,
	input *tkfacts.NormalizeWaitProcessorInput,
) (*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	act := tkfacts.NewNormalizeWaitProcessor(
		a.workflows,
		a.workflowRepo,
	)
	return act.Run(ctx, input)
}

func (a *Activities) EvaluateCondition(
	ctx context.Context,
	input *tkfacts.EvaluateConditionInput,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// Use the shared CEL evaluator instance
	act := tkfacts.NewEvaluateCondition(a.celEvaluator)
	return act.Run(ctx, input)
}

// -----------------------------------------------------------------------------
// Dispatcher Activities
// -----------------------------------------------------------------------------

func (a *Activities) DispatcherHeartbeat(ctx context.Context, input *wkacts.DispatcherHeartbeatInput) error {
	return wkacts.DispatcherHeartbeat(ctx, a.redisCache, input)
}

func (a *Activities) ListActiveDispatchers(
	ctx context.Context,
	input *wkacts.ListActiveDispatchersInput,
) (*wkacts.ListActiveDispatchersOutput, error) {
	return wkacts.ListActiveDispatchers(ctx, a.redisCache, input)
}

func (a *Activities) RemoveDispatcherHeartbeat(ctx context.Context, dispatcherID string) error {
	return wkacts.RemoveDispatcherHeartbeat(ctx, a.redisCache, dispatcherID)
}

// -----------------------------------------------------------------------------
// Memory Activities
// -----------------------------------------------------------------------------

func (a *Activities) FlushMemory(
	ctx context.Context,
	input memcore.FlushMemoryActivityInput,
) (*memcore.FlushMemoryActivityOutput, error) {
	// Delegate to the memory activities implementation
	return a.memoryActivities.FlushMemory(ctx, input)
}

func (a *Activities) ClearFlushPendingFlag(
	ctx context.Context,
	input memcore.ClearFlushPendingFlagInput,
) error {
	// Delegate to the memory activities implementation
	return a.memoryActivities.ClearFlushPendingFlag(ctx, input)
}
