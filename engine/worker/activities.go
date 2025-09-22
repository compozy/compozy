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
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/core"
	wkacts "github.com/compozy/compozy/engine/worker/activities"
	"github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

type Activities struct {
	projectConfig    *project.Config
	workflows        []*workflow.Config
	workflowRepo     workflow.Repository
	taskRepo         task.Repository
	runtime          runtime.Runtime
	configStore      services.ConfigStore
	signalDispatcher services.SignalDispatcher
	redisCache       *cache.Cache
	celEvaluator     task.ConditionEvaluator
	memoryManager    *memory.Manager
	memoryActivities *memacts.MemoryActivities
	templateEngine   *tplengine.TemplateEngine
	task2Factory     task2.Factory
	// Cached cache adapter contracts to avoid per-call instantiation
	cacheKV   cache.KV
	cacheKeys cache.KeysProvider
	// Config manager to reattach into Temporal activity contexts
	cfgManager *config.Manager
}

func NewActivities(
	ctx context.Context,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	signalDispatcher services.SignalDispatcher,
	redisCache *cache.Cache,
	memoryManager *memory.Manager,
	templateEngine *tplengine.TemplateEngine,
) (*Activities, error) {
	log := logger.FromContext(ctx)
	ids := make([]string, 0, len(workflows))
	for _, wf := range workflows {
		if wf != nil {
			ids = append(ids, wf.ID)
		}
	}
	const maxIDs = 20
	if len(ids) > maxIDs {
		ids = ids[:maxIDs]
	}
	log.Debug("Initializing activities", "workflow_count", len(workflows), "workflow_ids", ids)
	// Create CEL evaluator once for reuse across all activity executions
	celEvaluator, err := task.NewCELEvaluator()
	if err != nil {
		return nil, fmt.Errorf("activities: create CEL evaluator: %w", err)
	}
	// Create memory activities instance
	// Note: MemoryActivities will use activity.GetLogger(ctx) internally for proper logging
	memoryActivities := memacts.NewMemoryActivities(memoryManager)

	// Create task2 factory
	envMerger := core.NewEnvMerger()
	task2Factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: templateEngine,
		EnvMerger:      envMerger,
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("activities: create task2 factory: %w", err)
	}

	acts := &Activities{
		projectConfig:    projectConfig,
		workflows:        workflows,
		workflowRepo:     workflowRepo,
		taskRepo:         taskRepo,
		runtime:          runtime,
		configStore:      configStore,
		signalDispatcher: signalDispatcher,
		redisCache:       redisCache,
		celEvaluator:     celEvaluator,
		memoryManager:    memoryManager,
		memoryActivities: memoryActivities,
		templateEngine:   templateEngine,
		task2Factory:     task2Factory,
		cfgManager:       config.ManagerFromContext(ctx),
	}
	// Initialize cache adapter contracts once if Redis is available
	if redisCache != nil && redisCache.Redis != nil {
		ad, err := cache.NewRedisAdapter(redisCache.Redis)
		if err != nil {
			return nil, fmt.Errorf("activities: create redis cache adapter: %w", err)
		}
		acts.cacheKV = ad
		acts.cacheKeys = ad
	}
	return acts, nil
}

// withActivityLogger ensures a request-scoped logger is present in the activity context
// with the correct level derived from application configuration (e.g., --debug).
func withActivityLogger(ctx context.Context) context.Context {
	cfg := config.FromContext(ctx)
	if !cfg.CLI.Debug && !cfg.CLI.Quiet {
		return ctx
	}
	level := logger.InfoLevel
	if cfg.CLI.Quiet {
		level = logger.DisabledLevel
	} else if cfg.CLI.Debug {
		level = logger.DebugLevel
	}
	log := logger.NewLogger(&logger.Config{
		Level:      level,
		JSON:       false,
		AddSource:  cfg.CLI.Debug,
		TimeFormat: "15:04:05",
	})
	return logger.ContextWithLogger(ctx, log)
}

// withActivityContext ensures logger and configuration manager are attached to the activity context
func (a *Activities) withActivityContext(ctx context.Context) context.Context {
	c := withActivityLogger(ctx)
	if a != nil && a.cfgManager != nil {
		c = config.ContextWithManager(c, a.cfgManager)
	}
	return c
}

// -----------------------------------------------------------------------------
// Workflow
// -----------------------------------------------------------------------------

func (a *Activities) GetWorkflowData(ctx context.Context, input *wfacts.GetDataInput) (*wfacts.GetData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := config.FromContext(ctx)
	act := wfacts.NewGetData(a.projectConfig, a.workflows, cfg)
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
	act := wfacts.NewCompleteWorkflow(a.workflowRepo, a.workflows, a.projectConfig)
	return act.Run(ctx, input)
}

// -----------------------------------------------------------------------------
// Task
// -----------------------------------------------------------------------------

func (a *Activities) ExecuteBasicTask(
	ctx context.Context,
	input *tkfacts.ExecuteBasicInput,
) (*task.MainTaskResponse, error) {
	// Ensure logger and configuration manager are attached to the activity context
	// so downstream code (use-cases, LLM orchestrator) can emit logs and see app config.
	ctx = a.withActivityContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := config.FromContext(ctx)
	act, err := tkfacts.NewExecuteBasic(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
		a.configStore,
		memcore.ManagerInterface(a.memoryManager),
		a.templateEngine,
		a.projectConfig,
		a.task2Factory,
		cfg,
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
		a.task2Factory,
		a.templateEngine,
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
		a.task2Factory,
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
	// Ensure logger is attached to the activity context so downstream code
	// (use-cases, LLM orchestrator) can emit debug logs when enabled.
	ctx = withActivityLogger(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := config.FromContext(ctx)
	act := tkfacts.NewExecuteSubtask(
		a.workflows,
		a.workflowRepo,
		a.taskRepo,
		a.runtime,
		a.configStore,
		a.task2Factory,
		a.templateEngine,
		cfg,
		a.projectConfig,
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
	act := tkfacts.NewGetParallelResponse(
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.task2Factory,
		a.projectConfig.CWD,
	)
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
		a.task2Factory,
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
	act := tkfacts.NewGetCollectionResponse(
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.task2Factory,
		a.projectConfig.CWD,
	)
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
		a.task2Factory,
		a.templateEngine,
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
		a.task2Factory,
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
	act := tkfacts.NewGetCompositeResponse(
		a.workflowRepo,
		a.taskRepo,
		a.configStore,
		a.task2Factory,
		a.projectConfig.CWD,
	)
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
	// Create task config repository from factory
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.projectConfig.CWD)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config repository: %w", err)
	}
	act := tkfacts.NewLoadCompositeConfigs(configRepo)
	return act.Run(ctx, input)
}

func (a *Activities) LoadCollectionConfigsActivity(
	ctx context.Context,
	input *tkfacts.LoadCollectionConfigsInput,
) (map[string]*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Create task config repository from factory
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.projectConfig.CWD)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config repository: %w", err)
	}
	act := tkfacts.NewLoadCollectionConfigs(configRepo)
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
		a.task2Factory,
		a.templateEngine,
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
		a.task2Factory,
		a.templateEngine,
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
		a.projectConfig,
		a.task2Factory,
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
		a.taskRepo,
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
	ctx = a.withActivityContext(ctx)
	if a.cacheKV == nil {
		return wkacts.DispatcherHeartbeat(ctx, nil, input)
	}
	return wkacts.DispatcherHeartbeat(ctx, a.cacheKV, input)
}

func (a *Activities) ListActiveDispatchers(
	ctx context.Context,
	input *wkacts.ListActiveDispatchersInput,
) (*wkacts.ListActiveDispatchersOutput, error) {
	ctx = a.withActivityContext(ctx)
	if a.cacheKV == nil || a.cacheKeys == nil {
		return wkacts.ListActiveDispatchers(ctx, nil, input)
	}
	// Compose minimal contract interface
	type contracts interface {
		cache.KV
		cache.KeysProvider
	}
	return wkacts.ListActiveDispatchers(ctx, struct{ contracts }{contracts: struct {
		cache.KV
		cache.KeysProvider
	}{a.cacheKV, a.cacheKeys}}, input)
}

func (a *Activities) RemoveDispatcherHeartbeat(ctx context.Context, dispatcherID string) error {
	ctx = a.withActivityContext(ctx)
	if a.cacheKV == nil {
		return wkacts.RemoveDispatcherHeartbeat(ctx, nil, dispatcherID)
	}
	return wkacts.RemoveDispatcherHeartbeat(ctx, a.cacheKV, dispatcherID)
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
