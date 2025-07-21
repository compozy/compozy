package worker

import (
	"context"
	"fmt"
	"time"

	"errors"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	memacts "github.com/compozy/compozy/engine/memory/activities"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	wkacts "github.com/compozy/compozy/engine/worker/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	DispatcherEventChannel = "event_channel"
)

// -----------------------------------------------------------------------------
// Temporal-based Worker
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo      func() wf.Repository
	TaskRepo          func() task.Repository
	MonitoringService *monitoring.Service
	ResourceRegistry  *autoload.ConfigRegistry // For memory resource configs
}

type Worker struct {
	client        *Client
	config        *Config
	activities    *Activities
	worker        worker.Worker
	projectConfig *project.Config
	workflows     []*wf.Config
	taskQueue     string
	configStore   services.ConfigStore
	redisCache    *cache.Cache
	mcpRegister   *mcp.RegisterService
	dispatcherID  string // Track dispatcher ID for cleanup
	serverID      string // Server ID for this worker instance

	// Memory management
	memoryManager  *memory.Manager
	templateEngine *tplengine.TemplateEngine

	// Lifecycle management
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
}

func buildWorkerOptions(ctx context.Context, monitoringService *monitoring.Service) *worker.Options {
	log := logger.FromContext(ctx)
	options := &worker.Options{}
	if monitoringService != nil && monitoringService.IsInitialized() {
		interceptor := monitoringService.TemporalInterceptor(ctx)
		if interceptor != nil {
			options.Interceptors = append(options.Interceptors, interceptor)
			log.Info("Added Temporal monitoring interceptor to worker")
		}
	}
	return options
}

func buildRuntimeManager(
	ctx context.Context,
	projectRoot string,
	projectConfig *project.Config,
) (runtime.Runtime, error) {
	log := logger.FromContext(ctx)
	cfg := appconfig.Get()

	// Use factory to create runtime with direct unified config mapping
	factory := runtime.NewDefaultFactory(projectRoot)

	// Create a merged runtime config by applying project-specific overrides
	mergedRuntimeConfig := cfg.Runtime

	// Apply project-specific overrides if specified
	if projectConfig.Runtime.Type != "" {
		mergedRuntimeConfig.RuntimeType = projectConfig.Runtime.Type
		log.Debug("Using project-specific runtime type", "type", projectConfig.Runtime.Type)
	}
	if projectConfig.Runtime.Entrypoint != "" {
		mergedRuntimeConfig.EntrypointPath = projectConfig.Runtime.Entrypoint
		log.Debug("Using project-specific entrypoint", "entrypoint", projectConfig.Runtime.Entrypoint)
	}
	if len(projectConfig.Runtime.Permissions) > 0 {
		mergedRuntimeConfig.BunPermissions = projectConfig.Runtime.Permissions
		log.Debug("Using project-specific permissions", "permissions", projectConfig.Runtime.Permissions)
	}

	// Log final configuration being used for debugging
	log.Debug("Using unified runtime configuration",
		"environment", mergedRuntimeConfig.Environment,
		"runtime_type", mergedRuntimeConfig.RuntimeType,
		"entrypoint_path", mergedRuntimeConfig.EntrypointPath,
		"bun_permissions", mergedRuntimeConfig.BunPermissions,
		"tool_execution_timeout", mergedRuntimeConfig.ToolExecutionTimeout)

	return factory.CreateRuntimeFromAppConfig(ctx, &mergedRuntimeConfig)
}

func NewWorker(
	ctx context.Context,
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
) (*Worker, error) {
	log := logger.FromContext(ctx)
	workerStart := time.Now()
	if config == nil {
		return nil, errors.New("worker config cannot be nil")
	}
	client, err := createTemporalClient(ctx, clientConfig, log)
	if err != nil {
		return nil, err
	}
	workerCore, err := setupWorkerCore(ctx, config, projectConfig, client)
	if err != nil {
		return nil, err
	}
	mcpRegister, err := setupMCPRegister(ctx, workflows, log)
	if err != nil {
		return nil, err
	}
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	memoryManager, err := setupMemoryManager(
		config,
		templateEngine,
		workerCore.redisCache,
		client,
		workerCore.taskQueue,
		projectConfig,
		log,
	)
	if err != nil {
		return nil, err
	}
	dispatcher := createDispatcher(projectConfig, workerCore.taskQueue, client)
	activities := NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
		workerCore.rtManager,
		workerCore.configStore,
		dispatcher.signalDispatcher,
		workerCore.redisCache,
		memoryManager,
		templateEngine,
	)
	interceptor.SetConfiguredWorkerCount(1)
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	log.Debug("Worker initialization completed", "total_duration", time.Since(workerStart))
	return &Worker{
		client:          client,
		config:          config,
		worker:          workerCore.worker,
		projectConfig:   projectConfig,
		workflows:       workflows,
		activities:      activities,
		taskQueue:       workerCore.taskQueue,
		configStore:     workerCore.configStore,
		redisCache:      workerCore.redisCache,
		mcpRegister:     mcpRegister,
		dispatcherID:    dispatcher.dispatcherID,
		serverID:        dispatcher.serverID,
		memoryManager:   memoryManager,
		templateEngine:  templateEngine,
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
	}, nil
}

// workerCoreComponents holds the core components needed for a worker
type workerCoreComponents struct {
	worker      worker.Worker
	taskQueue   string
	rtManager   runtime.Runtime
	redisCache  *cache.Cache
	configStore services.ConfigStore
}

// dispatcherComponents holds dispatcher-related components
type dispatcherComponents struct {
	dispatcherID     string
	serverID         string
	signalDispatcher services.SignalDispatcher
}

// createTemporalClient creates and validates the Temporal client
func createTemporalClient(ctx context.Context, clientConfig *TemporalConfig, log logger.Logger) (*Client, error) {
	clientStart := time.Now()
	client, err := NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}
	log.Debug("Temporal client created", "duration", time.Since(clientStart))
	return client, nil
}

// setupWorkerCore creates the core worker components including Temporal worker, cache, and runtime manager
func setupWorkerCore(
	ctx context.Context,
	config *Config,
	projectConfig *project.Config,
	client *Client,
) (*workerCoreComponents, error) {
	// Use TaskQueue from client config if provided, otherwise generate from project name
	taskQueue := client.config.TaskQueue
	if taskQueue == "" {
		taskQueue = GetTaskQueue(projectConfig.Name)
	}
	workerOptions := buildWorkerOptions(ctx, config.MonitoringService)
	worker := client.NewWorker(taskQueue, workerOptions)
	projectRoot := projectConfig.GetCWD().PathStr()
	rtManager, err := buildRuntimeManager(ctx, projectRoot, projectConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to created execution manager: %w", err)
	}
	redisCache, configStore, err := setupRedisAndConfig(ctx, projectConfig)
	if err != nil {
		return nil, err
	}
	return &workerCoreComponents{
		worker:      worker,
		taskQueue:   taskQueue,
		rtManager:   rtManager,
		redisCache:  redisCache,
		configStore: configStore,
	}, nil
}

// setupRedisAndConfig sets up Redis cache and configuration management
func setupRedisAndConfig(
	ctx context.Context,
	_ *project.Config,
) (*cache.Cache, services.ConfigStore, error) {
	log := logger.FromContext(ctx)
	cacheStart := time.Now()
	cfg := appconfig.Get()
	// Build cache config from centralized Redis and cache config
	cacheConfig := cache.FromAppConfig(cfg)

	redisCache, err := cache.SetupCache(ctx, cacheConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup Redis cache: %w", err)
	}
	log.Debug("Redis cache connected", "duration", time.Since(cacheStart))
	configStore := services.NewRedisConfigStore(redisCache.Redis, cfg.Worker.ConfigStoreTTL)
	return redisCache, configStore, nil
}

// setupMCPRegister initializes MCP registration for workflows
func setupMCPRegister(ctx context.Context, workflows []*wf.Config, log logger.Logger) (*mcp.RegisterService, error) {
	// Initialize MCP register and register all MCPs from all workflows
	mcpStart := time.Now()
	workflowConfigs := make([]mcp.WorkflowConfig, len(workflows))
	for i, wf := range workflows {
		workflowConfigs[i] = wf
	}
	mcpRegister, err := mcp.SetupForWorkflows(ctx, workflowConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP register: %w", err)
	}
	log.Debug("MCP registration scheduled", "setup_duration", time.Since(mcpStart))
	return mcpRegister, nil
}

// setupMemoryManager creates the memory manager if resource registry is available
func setupMemoryManager(
	config *Config,
	templateEngine *tplengine.TemplateEngine,
	redisCache *cache.Cache,
	client *Client,
	taskQueue string,
	projectConfig *project.Config,
	log logger.Logger,
) (*memory.Manager, error) {
	if config.ResourceRegistry == nil {
		log.Warn("Resource registry not provided, memory features will be disabled")
		return nil, nil
	}
	privacyManager := privacy.NewManager()

	// Extract project ID for consistent namespace resolution
	fallbackProjectID := ""
	if projectConfig != nil {
		fallbackProjectID = projectConfig.Name
	}

	memoryManagerOpts := &memory.ManagerOptions{
		ResourceRegistry:  config.ResourceRegistry,
		TplEngine:         templateEngine,
		BaseLockManager:   redisCache.LockManager,
		BaseRedisClient:   redisCache.Redis,
		TemporalClient:    client,
		TemporalTaskQueue: taskQueue,
		PrivacyManager:    privacyManager,
		FallbackProjectID: fallbackProjectID,
	}
	memoryManager, err := memory.NewManager(memoryManagerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory manager: %w", err)
	}
	log.Info("Memory manager initialized successfully")
	return memoryManager, nil
}

// createDispatcher creates dispatcher components with unique IDs
func createDispatcher(projectConfig *project.Config, taskQueue string, client *Client) *dispatcherComponents {
	serverID := core.MustNewID().String()[:8]
	dispatcherID := fmt.Sprintf("dispatcher-%s-%s", GetTaskQueue(projectConfig.Name), serverID)
	signalDispatcher := NewSignalDispatcher(client, dispatcherID, taskQueue, serverID)
	return &dispatcherComponents{
		dispatcherID:     dispatcherID,
		serverID:         serverID,
		signalDispatcher: signalDispatcher,
	}
}

func (o *Worker) Setup(_ context.Context) error {
	o.worker.RegisterWorkflow(CompozyWorkflow)
	o.worker.RegisterWorkflow(DispatcherWorkflow)
	o.worker.RegisterActivity(o.activities.GetWorkflowData)
	o.worker.RegisterActivity(o.activities.TriggerWorkflow)
	o.worker.RegisterActivity(o.activities.UpdateWorkflowState)
	o.worker.RegisterActivity(o.activities.CompleteWorkflow)
	o.worker.RegisterActivity(o.activities.ExecuteBasicTask)
	o.worker.RegisterActivity(o.activities.ExecuteRouterTask)
	o.worker.RegisterActivity(o.activities.ExecuteAggregateTask)
	o.worker.RegisterActivity(o.activities.ExecuteSignalTask)
	o.worker.RegisterActivity(o.activities.ExecuteWaitTask)
	o.worker.RegisterActivity(o.activities.ExecuteMemoryTask)
	o.worker.RegisterActivityWithOptions(
		o.activities.NormalizeWaitProcessor,
		activity.RegisterOptions{Name: tkacts.NormalizeWaitProcessorLabel},
	)
	o.worker.RegisterActivity(o.activities.EvaluateCondition)
	o.worker.RegisterActivity(o.activities.ExecuteSubtask)
	o.worker.RegisterActivity(o.activities.CreateParallelState)
	o.worker.RegisterActivity(o.activities.GetParallelResponse)
	o.worker.RegisterActivity(o.activities.CreateCollectionState)
	o.worker.RegisterActivity(o.activities.GetCollectionResponse)
	o.worker.RegisterActivity(o.activities.CreateCompositeState)
	o.worker.RegisterActivity(o.activities.GetCompositeResponse)
	o.worker.RegisterActivity(o.activities.GetProgress)
	o.worker.RegisterActivity(o.activities.UpdateParentStatus)
	o.worker.RegisterActivity(o.activities.UpdateChildState)
	o.worker.RegisterActivity(o.activities.ListChildStates)
	o.worker.RegisterActivityWithOptions(
		o.activities.LoadTaskConfigActivity,
		activity.RegisterOptions{Name: tkacts.LoadTaskConfigLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.LoadBatchConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadBatchConfigsLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.LoadCompositeConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadCompositeConfigsLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.LoadCollectionConfigsActivity,
		activity.RegisterOptions{Name: tkacts.LoadCollectionConfigsLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.DispatcherHeartbeat,
		activity.RegisterOptions{Name: wkacts.DispatcherHeartbeatLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.ListActiveDispatchers,
		activity.RegisterOptions{Name: wkacts.ListActiveDispatchersLabel},
	)
	// Register memory activities if memory manager is available
	if o.memoryManager != nil {
		o.worker.RegisterActivityWithOptions(
			o.activities.FlushMemory,
			activity.RegisterOptions{Name: memacts.FlushMemoryLabel},
		)
		o.worker.RegisterActivityWithOptions(
			o.activities.ClearFlushPendingFlag,
			activity.RegisterOptions{Name: memacts.ClearFlushPendingLabel},
		)
	}
	err := o.worker.Start()
	if err != nil {
		return err
	}
	// Track running worker for monitoring
	interceptor.IncrementRunningWorkers(context.Background())
	// Register dispatcher for health monitoring
	cfg := appconfig.Get()
	monitoring.RegisterDispatcher(
		context.Background(),
		o.dispatcherID,
		cfg.Runtime.DispatcherStaleThreshold,
	)
	// Ensure dispatcher is running with independent lifecycle context
	go o.ensureDispatcherRunning(o.lifecycleCtx)
	return nil
}

func (o *Worker) Stop(ctx context.Context) {
	log := logger.FromContext(ctx)
	// Track worker stopping for monitoring
	interceptor.DecrementRunningWorkers(ctx)
	// Record dispatcher stop event for monitoring
	if o.dispatcherID != "" {
		interceptor.StopDispatcher(ctx, o.dispatcherID)
		monitoring.UnregisterDispatcher(ctx, o.dispatcherID)
	}
	// Cancel lifecycle context to stop background operations
	if o.lifecycleCancel != nil {
		o.lifecycleCancel()
	}
	// Terminate this instance's dispatcher since each server has its own
	if o.dispatcherID != "" {
		log.Info("Terminating instance dispatcher", "dispatcher_id", o.dispatcherID)
		if err := o.client.TerminateWorkflow(ctx, o.dispatcherID, "", "server shutdown"); err != nil {
			log.Error("Failed to terminate dispatcher", "error", err, "dispatcher_id", o.dispatcherID)
		}
		// Clean up heartbeat entry with background context to ensure completion
		if o.activities != nil && o.redisCache != nil {
			cfg := appconfig.Get()
			cleanupCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.HeartbeatCleanupTimeout)
			defer cancel()
			if err := o.activities.RemoveDispatcherHeartbeat(cleanupCtx, o.dispatcherID); err != nil {
				log.Error("Failed to remove dispatcher heartbeat", "error", err, "dispatcher_id", o.dispatcherID)
			}
		}
	}
	o.worker.Stop()
	o.client.Close()
	// Deregister all MCPs from proxy on shutdown
	if o.mcpRegister != nil {
		cfg := appconfig.Get()
		ctx, cancel := context.WithTimeout(ctx, cfg.Worker.MCPShutdownTimeout)
		defer cancel()
		if err := o.mcpRegister.Shutdown(ctx); err != nil {
			log.Error("Failed to shutdown MCP register", "error", err)
		}
	}
	if o.configStore != nil {
		if err := o.configStore.Close(); err != nil {
			log.Error("Failed to close config store", "error", err)
		}
	}
	if o.redisCache != nil {
		if err := o.redisCache.Close(); err != nil {
			log.Error("Failed to close Redis cache", "error", err)
		}
	}
}

func (o *Worker) WorkflowRepo() wf.Repository {
	return o.config.WorkflowRepo()
}

func (o *Worker) TaskRepo() task.Repository {
	return o.config.TaskRepo()
}

// GetClient exposes the Temporal client for signal operations
func (o *Worker) GetClient() client.Client {
	return o.client
}

// GetWorkerClient exposes the wrapped worker client
func (o *Worker) GetWorkerClient() *Client {
	return o.client
}

// GetDispatcherID returns this worker's unique dispatcher ID
func (o *Worker) GetDispatcherID() string {
	return o.dispatcherID
}

// GetTaskQueue returns this worker's task queue name
func (o *Worker) GetTaskQueue() string {
	return o.taskQueue
}

// GetMemoryManager returns the memory manager instance
func (o *Worker) GetMemoryManager() *memory.Manager {
	return o.memoryManager
}

// TerminateDispatcher explicitly terminates the dispatcher workflow
// Use this only when you want to force cleanup (e.g., CLI cleanup command)
func (o *Worker) TerminateDispatcher(ctx context.Context, reason string) error {
	log := logger.FromContext(ctx)
	if o.dispatcherID == "" {
		return fmt.Errorf("no dispatcher ID available")
	}
	log.Info("Terminating dispatcher workflow", "dispatcher_id", o.dispatcherID, "reason", reason)
	return o.client.TerminateWorkflow(ctx, o.dispatcherID, "", reason)
}

func (o *Worker) ensureDispatcherRunning(ctx context.Context) {
	log := logger.FromContext(ctx)
	cfg := appconfig.Get()
	maxRetries := cfg.Worker.DispatcherMaxRetries
	var safeMaxRetries uint64
	if maxRetries <= 0 {
		safeMaxRetries = 0
	} else {
		safeMaxRetries = uint64(maxRetries)
	}
	err := retry.Do(
		ctx,
		retry.WithMaxRetries(
			safeMaxRetries,
			retry.NewExponential(cfg.Worker.DispatcherRetryDelay),
		),
		func(ctx context.Context) error {
			_, err := o.client.SignalWithStartWorkflow(
				ctx,
				o.dispatcherID,
				DispatcherEventChannel,
				nil,
				client.StartWorkflowOptions{
					ID:                    o.dispatcherID,
					TaskQueue:             o.taskQueue,
					WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
				},
				DispatcherWorkflow,
				o.projectConfig.Name,
				o.serverID,
			)
			if err != nil {
				if _, ok := err.(*serviceerror.WorkflowExecutionAlreadyStarted); ok {
					log.Debug(
						"Dispatcher already running",
						"dispatcher_id",
						o.dispatcherID,
						"project",
						o.projectConfig.Name,
					)
					// Dispatcher is already running, no need to record start metrics
					return nil
				}
				log.Warn("Failed to start dispatcher, retrying", "error", err, "dispatcher_id", o.dispatcherID)
				return retry.RetryableError(err)
			}
			// Successfully started new dispatcher
			interceptor.StartDispatcher(ctx, o.dispatcherID)
			return nil
		},
	)
	if err != nil {
		log.Error("Failed to start dispatcher after all retries", "error", err, "dispatcher_id", o.dispatcherID)
	} else {
		log.Info("Started new dispatcher", "dispatcher_id", o.dispatcherID, "project", o.projectConfig.Name)
	}
}

// HealthCheck performs a comprehensive health check including cache connectivity
func (o *Worker) HealthCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	cfg := appconfig.Get()
	// Check Redis cache health
	if o.redisCache != nil {
		if err := o.redisCache.HealthCheck(ctx); err != nil {
			return fmt.Errorf("redis cache health check failed: %w", err)
		}
	}
	// Check dispatcher health by verifying recent heartbeat
	if o.dispatcherID != "" && o.activities != nil {
		input := &wkacts.ListActiveDispatchersInput{
			StaleThreshold: cfg.Runtime.DispatcherStaleThreshold,
		}
		output, err := o.activities.ListActiveDispatchers(ctx, input)
		if err != nil {
			log.Warn("Failed to check dispatcher health", "error", err)
		} else {
			// Check if our dispatcher is in the list and not stale
			found := false
			for _, dispatcher := range output.Dispatchers {
				if dispatcher.DispatcherID == o.dispatcherID {
					found = true
					if dispatcher.IsStale {
						log.Warn("Dispatcher is stale",
							"dispatcher_id", o.dispatcherID,
							"stale_duration", dispatcher.StaleDuration)
					}
					break
				}
			}
			if !found {
				log.Warn("Dispatcher not found in active list", "dispatcher_id", o.dispatcherID)
			}
		}
	}
	// Check MCP proxy health if configured
	if err := o.checkMCPProxyHealth(ctx); err != nil {
		// Log error but don't fail health check since MCP proxy is optional
		log.Warn("MCP proxy health check failed", "error", err)
	}
	return nil
}

// checkMCPProxyHealth checks if MCP proxy is healthy when configured
func (o *Worker) checkMCPProxyHealth(ctx context.Context) error {
	cfg := appconfig.Get()
	if cfg.LLM.ProxyURL == "" {
		return nil // No proxy configured
	}
	client := mcp.NewProxyClient(cfg.LLM.ProxyURL, string(cfg.LLM.AdminToken), cfg.Worker.MCPProxyHealthCheckTimeout)
	defer client.Close()
	return client.Health(ctx)
}

// -----------------------------------------------------------------------------
// Workflow ID Management
// -----------------------------------------------------------------------------

// buildWorkflowID creates a consistent Temporal workflow ID from workflowID and execID
func buildWorkflowID(workflowID string, workflowExecID core.ID) string {
	return workflowID + "-" + workflowExecID.String()
}

// -----------------------------------------------------------------------------
// Workflow Operations
// -----------------------------------------------------------------------------

func (o *Worker) TriggerWorkflow(
	ctx context.Context,
	workflowID string,
	input *core.Input,
	initTaskID string,
) (*WorkflowInput, error) {
	// Start workflow
	workflowExecID := core.MustNewID()
	workflowConfig, err := wf.FindConfig(o.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	// Apply schema defaults to input before validation and execution
	mergedInput, err := workflowConfig.ApplyInputDefaults(input)
	if err != nil {
		return nil, fmt.Errorf("failed to apply input defaults: %w", err)
	}
	// Validate the merged input (with defaults applied)
	if validationErr := workflowConfig.ValidateInput(ctx, mergedInput); validationErr != nil {
		return nil, fmt.Errorf("failed to validate workflow params: %w", validationErr)
	}
	workflowInput := WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          mergedInput, // Use merged input with defaults
		InitialTaskID:  initTaskID,
	}
	options := client.StartWorkflowOptions{
		ID:        buildWorkflowID(workflowID, workflowInput.WorkflowExecID),
		TaskQueue: o.taskQueue,
	}
	// MCPs are already registered at server startup, no need to register per workflow
	_, err = o.client.ExecuteWorkflow(
		ctx,
		options,
		CompozyWorkflow,
		workflowInput,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}
	return &workflowInput, nil
}

func (o *Worker) CancelWorkflow(ctx context.Context, workflowID string, workflowExecID core.ID) error {
	temporalID := buildWorkflowID(workflowID, workflowExecID)
	return o.client.CancelWorkflow(ctx, temporalID, "")
}
