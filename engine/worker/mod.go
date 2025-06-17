package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"errors"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gosimple/slug"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const DispatcherEventChannel = "event_channel"

// -----------------------------------------------------------------------------
// Temporal-based Worker
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo      func() wf.Repository
	TaskRepo          func() task.Repository
	MonitoringService *monitoring.Service
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

	// Lifecycle management
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
}

func buildWorkerOptions(monitoringService *monitoring.Service) *worker.Options {
	options := &worker.Options{}
	if monitoringService != nil && monitoringService.IsInitialized() {
		interceptor := monitoringService.TemporalInterceptor()
		if interceptor != nil {
			options.Interceptors = append(options.Interceptors, interceptor)
			logger.Info("Added Temporal monitoring interceptor to worker")
		}
	}
	return options
}

func NewWorker(
	ctx context.Context,
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
) (*Worker, error) {
	if config == nil {
		return nil, errors.New("worker config cannot be nil")
	}
	client, err := NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}
	taskQueue := slug.Make(projectConfig.Name)
	workerOptions := buildWorkerOptions(config.MonitoringService)
	worker := client.NewWorker(taskQueue, workerOptions)
	projectRoot := projectConfig.GetCWD().PathStr()
	// Build runtime options from project config
	var rtOpts []runtime.Option
	if len(projectConfig.Runtime.Permissions) > 0 {
		rtOpts = append(rtOpts, runtime.WithDenoPermissions(projectConfig.Runtime.Permissions))
	}
	runtime, err := runtime.NewRuntimeManager(projectRoot, rtOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to created execution manager: %w", err)
	}
	redisCache, err := cache.SetupCache(ctx, projectConfig.CacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Redis cache: %w", err)
	}
	// Create Redis-backed ConfigStore with 24h TTL as per PRD
	configStore := services.NewRedisConfigStore(redisCache.Redis, 24*time.Hour)
	configManager := services.NewConfigManager(configStore, nil)
	// Initialize MCP register and register all MCPs from all workflows
	workflowConfigs := make([]mcp.WorkflowConfig, len(workflows))
	for i, wf := range workflows {
		workflowConfigs[i] = wf
	}
	mcpRegister, err := mcp.SetupForWorkflows(ctx, workflowConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP register: %w", err)
	}
	// Create unique dispatcher ID per server instance to avoid conflicts
	serverID := core.MustNewID().String()[:8] // Use first 8 chars for readability
	dispatcherID := fmt.Sprintf("dispatcher-%s-%s", slug.Make(projectConfig.Name), serverID)
	signalDispatcher := NewSignalDispatcher(client, dispatcherID, taskQueue)
	activities := NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
		runtime,
		configStore,
		signalDispatcher,
		configManager,
	)
	// Set configured worker count for monitoring
	interceptor.SetConfiguredWorkerCount(1) // Each worker instance represents 1 configured worker
	// Create lifecycle context for independent operation
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &Worker{
		client:          client,
		config:          config,
		worker:          worker,
		projectConfig:   projectConfig,
		workflows:       workflows,
		activities:      activities,
		taskQueue:       taskQueue,
		configStore:     configStore,
		redisCache:      redisCache,
		mcpRegister:     mcpRegister,
		dispatcherID:    dispatcherID,
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
	}, nil
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
	err := o.worker.Start()
	if err != nil {
		return err
	}
	// Track running worker for monitoring
	interceptor.IncrementRunningWorkers(context.Background())
	// Ensure dispatcher is running with independent lifecycle context
	go o.ensureDispatcherRunning(o.lifecycleCtx)
	return nil
}

func (o *Worker) Stop(ctx context.Context) {
	// Track worker stopping for monitoring
	interceptor.DecrementRunningWorkers(ctx)
	// Cancel lifecycle context to stop background operations
	if o.lifecycleCancel != nil {
		o.lifecycleCancel()
	}
	// Terminate this instance's dispatcher since each server has its own
	if o.dispatcherID != "" {
		logger.Info("terminating instance dispatcher", "dispatcher_id", o.dispatcherID)
		if err := o.client.TerminateWorkflow(ctx, o.dispatcherID, "", "server shutdown"); err != nil {
			logger.Error("failed to terminate dispatcher", "error", err, "dispatcher_id", o.dispatcherID)
		}
	}
	o.worker.Stop()
	o.client.Close()
	// Deregister all MCPs from proxy on shutdown
	if o.mcpRegister != nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := o.mcpRegister.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown MCP register", "error", err)
		}
	}
	if o.configStore != nil {
		if err := o.configStore.Close(); err != nil {
			logger.Error("failed to close config store", "error", err)
		}
	}
	if o.redisCache != nil {
		if err := o.redisCache.Close(); err != nil {
			logger.Error("failed to close Redis cache", "error", err)
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

// GetDispatcherID returns this worker's unique dispatcher ID
func (o *Worker) GetDispatcherID() string {
	return o.dispatcherID
}

// GetTaskQueue returns this worker's task queue name
func (o *Worker) GetTaskQueue() string {
	return o.taskQueue
}

// TerminateDispatcher explicitly terminates the dispatcher workflow
// Use this only when you want to force cleanup (e.g., CLI cleanup command)
func (o *Worker) TerminateDispatcher(ctx context.Context, reason string) error {
	if o.dispatcherID == "" {
		return fmt.Errorf("no dispatcher ID available")
	}
	logger.Info("terminating dispatcher workflow", "dispatcher_id", o.dispatcherID, "reason", reason)
	return o.client.TerminateWorkflow(ctx, o.dispatcherID, "", reason)
}

func (o *Worker) ensureDispatcherRunning(ctx context.Context) {
	maxRetries := 5
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			logger.Info("context canceled, stopping dispatcher startup attempts", "dispatcher_id", o.dispatcherID)
			return
		default:
		}
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
		)
		if err != nil {
			if _, ok := err.(*serviceerror.WorkflowExecutionAlreadyStarted); ok {
				logger.Info(
					"dispatcher already running",
					"dispatcher_id",
					o.dispatcherID,
					"project",
					o.projectConfig.Name,
				)
				return
			}
			if attempt < maxRetries-1 {
				delay := time.Duration(1<<attempt) * baseDelay
				if delay > maxDelay {
					delay = maxDelay
				}
				logger.Error(
					"failed to start dispatcher, retrying",
					"error",
					err,
					"dispatcher_id",
					o.dispatcherID,
					"attempt",
					attempt+1,
					"retry_in",
					delay,
				)
				select {
				case <-ctx.Done():
					logger.Info("context canceled during retry delay", "dispatcher_id", o.dispatcherID)
					return
				case <-time.After(delay):
				}
			} else {
				logger.Error("failed to start dispatcher after all retries",
					"error", err, "dispatcher_id", o.dispatcherID, "attempts", maxRetries)
			}
		} else {
			logger.Info("started new dispatcher", "dispatcher_id", o.dispatcherID, "project", o.projectConfig.Name)
			return
		}
	}
}

// HealthCheck performs a comprehensive health check including cache connectivity
func (o *Worker) HealthCheck(ctx context.Context) error {
	// Check Redis cache health
	if o.redisCache != nil {
		if err := o.redisCache.HealthCheck(ctx); err != nil {
			return fmt.Errorf("redis cache health check failed: %w", err)
		}
	}
	// Check MCP proxy health if configured
	if err := o.checkMCPProxyHealth(ctx); err != nil {
		// Log error but don't fail health check since MCP proxy is optional
		fmt.Printf("Warning: MCP proxy health check failed: %v\n", err)
	}
	return nil
}

// checkMCPProxyHealth checks if MCP proxy is healthy when configured
func (o *Worker) checkMCPProxyHealth(ctx context.Context) error {
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		return nil // No proxy configured
	}
	adminToken := os.Getenv("MCP_PROXY_ADMIN_TOKEN")
	client := mcp.NewProxyClient(proxyURL, adminToken, 10*time.Second)
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
	workflowInput := WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          input,
		InitialTaskID:  initTaskID,
	}
	options := client.StartWorkflowOptions{
		ID:        buildWorkflowID(workflowID, workflowInput.WorkflowExecID),
		TaskQueue: o.taskQueue,
	}
	workflowConfig, err := wf.FindConfig(o.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	if err := workflowConfig.ValidateInput(ctx, input); err != nil {
		return nil, fmt.Errorf("failed to validate workflow params: %w", err)
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
