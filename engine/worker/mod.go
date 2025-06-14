package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gosimple/slug"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Temporal-based Worker
// -----------------------------------------------------------------------------

type Config struct {
	WorkflowRepo func() wf.Repository
	TaskRepo     func() task.Repository
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
}

func NewWorker(
	ctx context.Context,
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
) (*Worker, error) {
	client, err := NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}
	taskQueue := slug.Make(projectConfig.Name)
	worker := client.NewWorker(taskQueue)
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

	// Initialize MCP register and register all MCPs from all workflows
	workflowConfigs := make([]mcp.WorkflowConfig, len(workflows))
	for i, wf := range workflows {
		workflowConfigs[i] = wf
	}
	mcpRegister, err := mcp.SetupForWorkflows(ctx, workflowConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP register: %w", err)
	}

	activities := NewActivities(
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
		runtime,
		configStore,
	)
	return &Worker{
		client:        client,
		config:        config,
		worker:        worker,
		projectConfig: projectConfig,
		workflows:     workflows,
		activities:    activities,
		taskQueue:     taskQueue,
		configStore:   configStore,
		redisCache:    redisCache,
		mcpRegister:   mcpRegister,
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
	o.worker.RegisterActivity(o.activities.ExecuteSubtask)
	o.worker.RegisterActivity(o.activities.CreateParallelState)
	o.worker.RegisterActivity(o.activities.GetParallelResponse)
	o.worker.RegisterActivity(o.activities.CreateCollectionState)
	o.worker.RegisterActivity(o.activities.GetCollectionResponse)
	o.worker.RegisterActivity(o.activities.CreateCompositeState)
	o.worker.RegisterActivity(o.activities.GetCompositeResponse)
	o.worker.RegisterActivity(o.activities.GetProgress)
	o.worker.RegisterActivity(o.activities.UpdateParentStatus)
	o.worker.RegisterActivity(o.activities.ListChildStates)
	err := o.worker.Start()
	if err != nil {
		return err
	}
	// Ensure dispatcher is running
	go o.ensureDispatcherRunning(context.Background())
	return nil
}

func (o *Worker) Stop(ctx context.Context) {
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

func (o *Worker) ensureDispatcherRunning(ctx context.Context) {
	dispatcherID := "dispatcher-" + slug.Make(o.projectConfig.Name)
	_, err := o.client.SignalWithStartWorkflow(
		ctx,
		dispatcherID,
		"event_channel",
		nil,
		client.StartWorkflowOptions{
			ID:        dispatcherID,
			TaskQueue: o.taskQueue,
		},
		DispatcherWorkflow,
		o.projectConfig.Name,
	)
	if err != nil {
		// Use the typed error check for robustness
		if _, ok := err.(*serviceerror.WorkflowExecutionAlreadyStarted); !ok {
			logger.Error("Failed to start dispatcher", "error", err)
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
		ID:        workflowExecID.String(),
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

func (o *Worker) CancelWorkflow(ctx context.Context, workflowExecID core.ID) error {
	id := workflowExecID.String()
	return o.client.CancelWorkflow(ctx, id, "")
}
