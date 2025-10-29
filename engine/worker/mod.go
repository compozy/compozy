package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	stdruntime "runtime"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	memacts "github.com/compozy/compozy/engine/memory/activities"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/services"
	wkacts "github.com/compozy/compozy/engine/worker/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/gosimple/slug"
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
	DispatcherEventChannel        = "event_channel"
	workflowStartTimeoutDefault   = 5 * time.Second
	maxTaskQueueLength            = 200
	maxDispatcherQueueSegment     = 200
	maxDispatcherWorkflowIDLength = 240
	hashSuffixLen                 = 8 // hex-encoded 4 bytes
	dispatcherTakeoverReason      = "terminate stale dispatcher for takeover"
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
	client           *Client
	config           *Config
	activities       *Activities
	worker           worker.Worker
	queueDepthCancel context.CancelFunc
	projectConfig    *project.Config
	workflows        []*wf.Config
	taskQueue        string
	configStore      services.ConfigStore
	redisCache       *cache.Cache
	cacheCleanup     func()
	mcpRegister      *mcp.RegisterService
	dispatcherID     string // Track dispatcher ID for cleanup
	serverID         string // Server ID for this worker instance

	// Memory management
	memoryManager  *memory.Manager
	templateEngine *tplengine.TemplateEngine

	// Lifecycle management
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
}

// registerActivities registers workflows and all activities with explicit labels
// to ensure deterministic name resolution across refactors.
func (o *Worker) registerActivities() {
	o.registerWorkflows()
	o.registerWorkflowActivities()
	o.registerTaskActivities()
	o.registerConfigActivities()
	o.registerDispatcherActivities()
	o.registerMemoryActivities()
}

func (o *Worker) registerWorkflows() {
	o.worker.RegisterWorkflow(CompozyWorkflow)
	o.worker.RegisterWorkflow(DispatcherWorkflow)
}

func (o *Worker) registerWorkflowActivities() {
	o.worker.RegisterActivityWithOptions(
		o.activities.GetWorkflowData,
		activity.RegisterOptions{Name: wfacts.GetDataLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.TriggerWorkflow,
		activity.RegisterOptions{Name: wfacts.TriggerLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.UpdateWorkflowState,
		activity.RegisterOptions{Name: wfacts.UpdateStateLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.CompleteWorkflow,
		activity.RegisterOptions{Name: wfacts.CompleteWorkflowLabel},
	)
}

func (o *Worker) registerTaskActivities() {
	o.registerActivityBatch([]activityRegistration{
		{o.activities.ExecuteBasicTask, tkacts.ExecuteBasicLabel},
		{o.activities.ExecuteRouterTask, tkacts.ExecuteRouterLabel},
		{o.activities.ExecuteAggregateTask, tkacts.ExecuteAggregateLabel},
		{o.activities.ExecuteSignalTask, tkacts.ExecuteSignalLabel},
		{o.activities.ExecuteWaitTask, tkacts.ExecuteWaitLabel},
		{o.activities.ExecuteMemoryTask, tkacts.ExecuteMemoryLabel},
		{o.activities.NormalizeWaitProcessor, tkacts.NormalizeWaitProcessorLabel},
		{o.activities.EvaluateCondition, tkacts.EvaluateConditionLabel},
		{o.activities.ExecuteSubtask, tkacts.ExecuteSubtaskLabel},
		{o.activities.CreateParallelState, tkacts.CreateParallelStateLabel},
		{o.activities.GetParallelResponse, tkacts.GetParallelResponseLabel},
		{o.activities.CreateCollectionState, tkacts.CreateCollectionStateLabel},
		{o.activities.GetCollectionResponse, tkacts.GetCollectionResponseLabel},
		{o.activities.CreateCompositeState, tkacts.CreateCompositeStateLabel},
		{o.activities.GetCompositeResponse, tkacts.GetCompositeResponseLabel},
		{o.activities.GetProgress, tkacts.GetProgressLabel},
		{o.activities.UpdateParentStatus, tkacts.UpdateParentStatusLabel},
		{o.activities.UpdateChildState, tkacts.UpdateChildStateLabel},
		{o.activities.ListChildStates, tkacts.ListChildStatesLabel},
	})
}

type activityRegistration struct {
	handler any
	name    string
}

// registerActivities registers a batch of activity handlers with their labels.
func (o *Worker) registerActivityBatch(regs []activityRegistration) {
	for _, reg := range regs {
		o.worker.RegisterActivityWithOptions(reg.handler, activity.RegisterOptions{Name: reg.name})
	}
}

func (o *Worker) registerConfigActivities() {
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
}

func (o *Worker) registerDispatcherActivities() {
	o.worker.RegisterActivityWithOptions(
		o.activities.DispatcherHeartbeat,
		activity.RegisterOptions{Name: wkacts.DispatcherHeartbeatLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.ListActiveDispatchers,
		activity.RegisterOptions{Name: wkacts.ListActiveDispatchersLabel},
	)
}

func (o *Worker) registerMemoryActivities() {
	if o.memoryManager == nil {
		return
	}
	o.worker.RegisterActivityWithOptions(
		o.activities.FlushMemory,
		activity.RegisterOptions{Name: memacts.FlushMemoryLabel},
	)
	o.worker.RegisterActivityWithOptions(
		o.activities.ClearFlushPendingFlag,
		activity.RegisterOptions{Name: memacts.ClearFlushPendingLabel},
	)
}

func buildWorkerOptions(ctx context.Context, monitoringService *monitoring.Service) *worker.Options {
	log := logger.FromContext(ctx)
	options := &worker.Options{}
	if cfg := appconfig.FromContext(ctx); cfg != nil {
		autoActivity := stdruntime.NumCPU() * 2
		autoWorkflow := stdruntime.NumCPU()
		autoLocal := stdruntime.NumCPU() * 4
		options.MaxConcurrentActivityExecutionSize = positiveOrDefault(
			cfg.Worker.MaxConcurrentActivityExecutionSize,
			autoActivity,
		)
		options.MaxConcurrentWorkflowTaskExecutionSize = positiveOrDefault(
			cfg.Worker.MaxConcurrentWorkflowExecutionSize,
			autoWorkflow,
		)
		options.MaxConcurrentLocalActivityExecutionSize = positiveOrDefault(
			cfg.Worker.MaxConcurrentLocalActivityExecutionSize,
			autoLocal,
		)
	}
	setWorkerConcurrencyLimits(
		options.MaxConcurrentActivityExecutionSize,
		options.MaxConcurrentWorkflowTaskExecutionSize,
	)
	if monitoringService != nil && monitoringService.IsInitialized() {
		interceptor := monitoringService.TemporalInterceptor(ctx)
		if interceptor != nil {
			options.Interceptors = append(options.Interceptors, interceptor)
			log.Info("Added Temporal monitoring interceptor to worker")
		}
	}
	options.Interceptors = append(options.Interceptors, newWorkerMetricsInterceptor(ctx))
	return options
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func buildRuntimeManager(
	ctx context.Context,
	projectRoot string,
	projectConfig *project.Config,
) (runtime.Runtime, error) {
	log := logger.FromContext(ctx)
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("config manager not found in context")
	}
	factory := runtime.NewDefaultFactory(projectRoot)
	mergedRuntimeConfig := cfg.Runtime
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
	if projectConfig.Runtime.ToolExecutionTimeout > 0 {
		mergedRuntimeConfig.ToolExecutionTimeout = projectConfig.Runtime.ToolExecutionTimeout
		log.Debug(
			"Using project-specific tool execution timeout",
			"timeout",
			projectConfig.Runtime.ToolExecutionTimeout,
		)
	}
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
	toolEnv toolenv.Environment,
) (*Worker, error) {
	log := logger.FromContext(ctx)
	workerStart := time.Now()
	if config == nil {
		return nil, errors.New("worker config cannot be nil")
	}
	if err := validateToolEnvironment(toolEnv); err != nil {
		return nil, err
	}
	initResult, err := initializeWorkerComponents(ctx, config, clientConfig, projectConfig, workflows, toolEnv)
	if err != nil {
		return nil, err
	}
	interceptor.SetConfiguredWorkerCount(1)
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.WithoutCancel(ctx))
	log.Debug("Worker initialization completed", "total_duration", time.Since(workerStart))
	return &Worker{
		client:          initResult.client,
		config:          config,
		worker:          initResult.core.worker,
		projectConfig:   projectConfig,
		workflows:       workflows,
		activities:      initResult.activities,
		taskQueue:       initResult.core.taskQueue,
		configStore:     initResult.core.configStore,
		redisCache:      initResult.core.redisCache,
		cacheCleanup:    initResult.core.cacheCleanup,
		mcpRegister:     initResult.mcpRegister,
		dispatcherID:    initResult.dispatcher.dispatcherID,
		serverID:        initResult.dispatcher.serverID,
		memoryManager:   initResult.memoryManager,
		templateEngine:  initResult.templateEngine,
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
	}, nil
}

type workerInitResult struct {
	client         *Client
	core           *workerCoreComponents
	mcpRegister    *mcp.RegisterService
	templateEngine *tplengine.TemplateEngine
	memoryManager  *memory.Manager
	dispatcher     *dispatcherComponents
	activities     *Activities
}

// initializeWorkerComponents wires all worker dependencies prior to instantiation.
func initializeWorkerComponents(
	ctx context.Context,
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
	toolEnv toolenv.Environment,
) (*workerInitResult, error) {
	deps, err := buildWorkerDependencies(ctx, config, clientConfig, projectConfig, workflows)
	if err != nil {
		return nil, err
	}
	projectName := ""
	if projectConfig != nil {
		projectName = projectConfig.Name
	}
	dispatcher := createDispatcher(deps.core.taskQueue, projectName, deps.client)
	activities, err := prepareWorkerActivities(
		ctx,
		config,
		projectConfig,
		workflows,
		deps.core,
		dispatcher,
		deps.memoryManager,
		deps.templateEngine,
		toolEnv,
	)
	if err != nil {
		return nil, err
	}
	return &workerInitResult{
		client:         deps.client,
		core:           deps.core,
		mcpRegister:    deps.mcpRegister,
		templateEngine: deps.templateEngine,
		memoryManager:  deps.memoryManager,
		dispatcher:     dispatcher,
		activities:     activities,
	}, nil
}

type workerDependencies struct {
	client         *Client
	core           *workerCoreComponents
	mcpRegister    *mcp.RegisterService
	templateEngine *tplengine.TemplateEngine
	memoryManager  *memory.Manager
}

// buildWorkerDependencies creates the foundational services required by the worker.
func buildWorkerDependencies(
	ctx context.Context,
	config *Config,
	clientConfig *TemporalConfig,
	projectConfig *project.Config,
	workflows []*wf.Config,
) (*workerDependencies, error) {
	client, err := createTemporalClient(ctx, clientConfig)
	if err != nil {
		return nil, err
	}
	workerCore, err := setupWorkerCore(ctx, config, projectConfig, client)
	if err != nil {
		return nil, err
	}
	mcpRegister, err := setupMCPRegister(ctx, workflows)
	if err != nil {
		return nil, err
	}
	templateEngine, memoryManager, err := buildWorkerResources(ctx, config, projectConfig, workerCore, client)
	if err != nil {
		return nil, err
	}
	return &workerDependencies{
		client:         client,
		core:           workerCore,
		mcpRegister:    mcpRegister,
		templateEngine: templateEngine,
		memoryManager:  memoryManager,
	}, nil
}

func validateToolEnvironment(env toolenv.Environment) error {
	if env == nil {
		return errors.New("tool environment cannot be nil")
	}
	return nil
}

func buildWorkerResources(
	ctx context.Context,
	config *Config,
	projectConfig *project.Config,
	workerCore *workerCoreComponents,
	client *Client,
) (*tplengine.TemplateEngine, *memory.Manager, error) {
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	memoryManager, err := setupMemoryManager(
		ctx,
		config,
		templateEngine,
		workerCore.redisCache,
		client,
		workerCore.taskQueue,
		projectConfig,
	)
	if err != nil {
		return nil, nil, err
	}
	return templateEngine, memoryManager, nil
}

// workerCoreComponents holds the core components needed for a worker
type workerCoreComponents struct {
	worker       worker.Worker
	taskQueue    string
	rtManager    runtime.Runtime
	redisCache   *cache.Cache
	cacheCleanup func()
	configStore  services.ConfigStore
}

// dispatcherComponents holds dispatcher-related components
type dispatcherComponents struct {
	dispatcherID     string
	serverID         string
	signalDispatcher services.SignalDispatcher
}

// createTemporalClient creates and validates the Temporal client
func createTemporalClient(ctx context.Context, clientConfig *TemporalConfig) (*Client, error) {
	log := logger.FromContext(ctx)
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
	taskQueue := deriveTaskQueue(ctx, client.config.TaskQueue, projectConfig)
	client.config.TaskQueue = taskQueue
	log := logger.FromContext(ctx)
	log.Debug("derived task queue for worker", "task_queue", taskQueue)
	workerOptions := buildWorkerOptions(ctx, config.MonitoringService)
	worker := client.NewWorker(taskQueue, workerOptions)
	projectRoot := projectConfig.GetCWD().PathStr()
	rtManager, err := buildRuntimeManager(ctx, projectRoot, projectConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution manager: %w", err)
	}
	redisCache, cacheCleanup, configStore, err := setupRedisAndConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &workerCoreComponents{
		worker:       worker,
		taskQueue:    taskQueue,
		rtManager:    rtManager,
		redisCache:   redisCache,
		cacheCleanup: cacheCleanup,
		configStore:  configStore,
	}, nil
}

func deriveTaskQueue(ctx context.Context, configuredQueue string, projectConfig *project.Config) string {
	segments := make([]string, 0, 4)
	base := strings.TrimSpace(configuredQueue)
	if base == "" {
		base = "compozy-tasks"
	}
	segments = append(segments, sanitizeQueueSegment(base))
	if projectConfig != nil && projectConfig.Name != "" {
		segments = append(segments, sanitizeQueueSegment(projectConfig.Name))
	}
	segments = append(segments, queueScopeParts(ctx)...)
	queue := strings.Join(segments, "-")
	return truncateWithHash(queue, maxTaskQueueLength)
}

func queueScopeParts(ctx context.Context) []string {
	parts := make([]string, 0, 3)
	if scoped := strings.TrimSpace(os.Getenv("COMPOZY_TASK_QUEUE_SCOPE")); scoped != "" {
		parts = append(parts, sanitizeQueueSegment(scoped))
		return parts
	}
	cfg := appconfig.FromContext(ctx)
	if cfg != nil && cfg.Runtime.Environment != "" {
		parts = append(parts, sanitizeQueueSegment(cfg.Runtime.Environment))
	}
	if os.Getenv("CI") != "" {
		ciHints := []string{
			os.Getenv("CI_RUN_ID"),
			os.Getenv("GITHUB_RUN_ID"),
			os.Getenv("BUILD_ID"),
			os.Getenv("BUILD_NUMBER"),
		}
		for _, hint := range ciHints {
			if hint != "" {
				parts = append(parts, sanitizeQueueSegment(hint))
				break
			}
		}
		return dedupeQueueSegments(parts)
	}
	user := strings.TrimSpace(os.Getenv("USER"))
	if user == "" {
		user = strings.TrimSpace(os.Getenv("USERNAME"))
	}
	if user != "" {
		parts = append(parts, sanitizeQueueSegment(user))
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		parts = append(parts, sanitizeQueueSegment(host))
	}
	return dedupeQueueSegments(parts)
}

func sanitizeQueueSegment(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	slugged := slug.Make(value)
	if slugged == "" {
		return ""
	}
	return slugged
}

func dedupeQueueSegments(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}
	seen := make(map[string]struct{}, len(segments))
	result := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if _, ok := seen[segment]; ok {
			continue
		}
		seen[segment] = struct{}{}
		result = append(result, segment)
	}
	return result
}

func truncateWithHash(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	if limit <= hashSuffixLen {
		return value[:limit]
	}
	hash := sha256.Sum256([]byte(value))
	suffix := hex.EncodeToString(hash[:4])
	cut := limit - hashSuffixLen
	return value[:cut] + suffix
}

// setupRedisAndConfig sets up Redis cache and configuration management
func setupRedisAndConfig(
	ctx context.Context,
) (*cache.Cache, func(), services.ConfigStore, error) {
	log := logger.FromContext(ctx)
	cacheStart := time.Now()
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return nil, nil, nil, fmt.Errorf("config manager not found in context")
	}
	redisCache, cleanup, err := cache.SetupCache(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup Redis cache: %w", err)
	}
	log.Debug("Redis cache connected", "duration", time.Since(cacheStart))
	configStore := services.NewRedisConfigStore(redisCache.Redis, cfg.Worker.ConfigStoreTTL)
	return redisCache, cleanup, configStore, nil
}

// setupMCPRegister initializes MCP registration for workflows
func setupMCPRegister(ctx context.Context, workflows []*wf.Config) (*mcp.RegisterService, error) {
	log := logger.FromContext(ctx)
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
	ctx context.Context,
	config *Config,
	templateEngine *tplengine.TemplateEngine,
	redisCache *cache.Cache,
	client *Client,
	taskQueue string,
	projectConfig *project.Config,
) (*memory.Manager, error) {
	log := logger.FromContext(ctx)
	if config.ResourceRegistry == nil {
		log.Warn("Resource registry not provided, memory features will be disabled")
		return nil, nil
	}
	privacyManager := privacy.NewManager()
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

// createDispatcher creates dispatcher components with deterministic workflow IDs
func buildDispatcherWorkflowID(projectName string, taskQueue string) string {
	queueSegment := sanitizeQueueSegment(taskQueue)
	queueSegment = truncateWithHash(queueSegment, maxDispatcherQueueSegment)
	parts := []string{"dispatcher"}
	if projectSegment := sanitizeQueueSegment(projectName); projectSegment != "" {
		parts = append(parts, projectSegment)
	}
	parts = append(parts, queueSegment)
	dispatcherID := strings.Join(parts, "-")
	return truncateWithHash(dispatcherID, maxDispatcherWorkflowIDLength)
}

func createDispatcher(taskQueue string, projectName string, client *Client) *dispatcherComponents {
	serverID := core.MustNewID().String()
	dispatcherID := buildDispatcherWorkflowID(projectName, taskQueue)
	signalDispatcher := NewSignalDispatcher(client, dispatcherID, taskQueue, serverID)
	return &dispatcherComponents{
		dispatcherID:     dispatcherID,
		serverID:         serverID,
		signalDispatcher: signalDispatcher,
	}
}

func prepareWorkerActivities(
	ctx context.Context,
	config *Config,
	projectConfig *project.Config,
	workflows []*wf.Config,
	workerCore *workerCoreComponents,
	dispatcher *dispatcherComponents,
	memoryManager *memory.Manager,
	templateEngine *tplengine.TemplateEngine,
	toolEnv toolenv.Environment,
) (*Activities, error) {
	var usageMetrics usage.Metrics
	providerMetrics := providermetrics.Nop()
	if config.MonitoringService != nil && config.MonitoringService.IsInitialized() {
		usageMetrics = config.MonitoringService.LLMUsageMetrics()
		providerMetrics = config.MonitoringService.LLMProviderMetrics()
	}
	return NewActivities(
		ctx,
		projectConfig,
		workflows,
		config.WorkflowRepo(),
		config.TaskRepo(),
		usageMetrics,
		providerMetrics,
		workerCore.rtManager,
		workerCore.configStore,
		dispatcher.signalDispatcher,
		workerCore.redisCache,
		memoryManager,
		templateEngine,
		toolEnv,
	)
}

func (o *Worker) Setup(ctx context.Context) error {
	o.registerActivities()
	err := o.worker.Start()
	if err != nil {
		return err
	}
	if o.config != nil && o.config.MonitoringService != nil && o.config.MonitoringService.IsInitialized() {
		o.startQueueDepthMonitor(ctx)
	}
	interceptor.IncrementRunningWorkers(ctx)
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("config manager not found in context")
	}
	monitoring.RegisterDispatcher(
		ctx,
		o.dispatcherID,
		cfg.Runtime.DispatcherStaleThreshold,
	)
	go o.ensureDispatcherRunning(o.lifecycleCtx)
	return nil
}

func (o *Worker) Stop(ctx context.Context) {
	o.stopQueueDepthMonitor()
	interceptor.DecrementRunningWorkers(ctx)
	o.stopDispatcherMonitoring(ctx)
	o.cancelLifecycle()
	o.terminateDispatcher(ctx)
	o.worker.Stop()
	o.client.Close()
	o.shutdownMCPs(ctx)
	o.closeStores(ctx)
}

func (o *Worker) stopDispatcherMonitoring(ctx context.Context) {
	if o.dispatcherID != "" {
		interceptor.StopDispatcher(ctx, o.dispatcherID)
		monitoring.UnregisterDispatcher(ctx, o.dispatcherID)
	}
}

func (o *Worker) cancelLifecycle() {
	if o.lifecycleCancel != nil {
		o.lifecycleCancel()
	}
}

func (o *Worker) terminateDispatcher(ctx context.Context) {
	log := logger.FromContext(ctx)
	if o.dispatcherID != "" {
		log.Info("Terminating instance dispatcher", "dispatcher_id", o.dispatcherID)
		if err := o.client.TerminateWorkflow(ctx, o.dispatcherID, "", "server shutdown"); err != nil {
			log.Error("Failed to terminate dispatcher", "error", err, "dispatcher_id", o.dispatcherID)
		}
		o.cleanupDispatcherHeartbeat(ctx)
	}
}

func (o *Worker) cleanupDispatcherHeartbeat(ctx context.Context) {
	log := logger.FromContext(ctx)
	if o.activities != nil && o.redisCache != nil {
		cfg := appconfig.FromContext(ctx)
		if cfg == nil {
			log.Error(
				"config manager not found in context, skipping heartbeat cleanup",
				"dispatcher_id",
				o.dispatcherID,
			)
		} else {
			cleanupCtx, cancel := context.WithTimeout(ctx, cfg.Worker.HeartbeatCleanupTimeout)
			defer cancel()
			if err := o.activities.RemoveDispatcherHeartbeat(cleanupCtx, o.dispatcherID); err != nil {
				log.Error("Failed to remove dispatcher heartbeat", "error", err, "dispatcher_id", o.dispatcherID)
			}
		}
	}
}

func (o *Worker) shutdownMCPs(ctx context.Context) {
	log := logger.FromContext(ctx)
	if o.mcpRegister != nil {
		cfg := appconfig.FromContext(ctx)
		if cfg == nil {
			log.Error("config manager not found in context, skipping MCP shutdown", "dispatcher_id", o.dispatcherID)
		} else {
			shutdownCtx, cancel := context.WithTimeout(ctx, cfg.Worker.MCPShutdownTimeout)
			defer cancel()
			if err := o.mcpRegister.Shutdown(shutdownCtx); err != nil {
				log.Error("Failed to shutdown MCP register", "error", err)
			}
		}
	}
}

func (o *Worker) closeStores(ctx context.Context) {
	log := logger.FromContext(ctx)
	if o.cacheCleanup != nil {
		o.cacheCleanup()
	}
	if o.configStore != nil {
		if err := o.configStore.Close(); err != nil {
			log.Error("Failed to close config store", "error", err)
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

// GetServerID returns the unique identifier for this worker instance
func (o *Worker) GetServerID() string {
	return o.serverID
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
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		log.Error("config manager not found in context, cannot start dispatcher")
		return
	}
	maxRetries := cfg.Worker.DispatcherMaxRetries
	backoff := retry.NewExponential(cfg.Worker.DispatcherRetryDelay)
	if maxRetries > 0 {
		backoff = retry.WithMaxRetries(uint64(maxRetries), backoff)
	}
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		return o.startDispatcherWithTakeover(ctx, cfg)
	})
	if err != nil {
		log.Error("Failed to start dispatcher after all retries", "error", err, "dispatcher_id", o.dispatcherID)
	} else {
		log.Info("Started new dispatcher", "dispatcher_id", o.dispatcherID, "project", o.projectConfig.Name)
	}
}

func (o *Worker) startDispatcherWithTakeover(ctx context.Context, cfg *appconfig.Config) error {
	attemptStart := time.Now()
	attemptTimeout := workflowStartTimeoutDefault
	if cfg.Worker.StartWorkflowTimeout > 0 {
		attemptTimeout = cfg.Worker.StartWorkflowTimeout
	}
	actx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	_, err := o.client.SignalWithStartWorkflow(
		actx,
		o.dispatcherID,
		DispatcherEventChannel,
		nil,
		client.StartWorkflowOptions{
			ID:                    o.dispatcherID,
			TaskQueue:             o.taskQueue,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
		},
		DispatcherWorkflow,
		o.projectConfig.Name,
		o.serverID,
	)
	if err == nil {
		o.recordDispatcherStart(ctx, attemptStart)
		return nil
	}
	return o.handleDispatcherStartError(ctx, attemptStart, attemptTimeout, err)
}

func (o *Worker) recordDispatcherStart(ctx context.Context, startedAt time.Time) {
	monitoring.RecordDispatcherTakeover(ctx, o.dispatcherID, time.Since(startedAt), monitoring.TakeoverOutcomeStarted)
	interceptor.StartDispatcher(ctx, o.dispatcherID)
}

func (o *Worker) handleDispatcherStartError(
	ctx context.Context,
	attemptStart time.Time,
	attemptTimeout time.Duration,
	startErr error,
) error {
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(startErr, &alreadyStarted) {
		o.logStaleDispatcher(ctx)
		return o.terminateStaleDispatcher(ctx, attemptStart, attemptTimeout, startErr)
	}
	logger.FromContext(ctx).Warn(
		"Failed to start dispatcher, retrying",
		"error",
		startErr,
		"dispatcher_id",
		o.dispatcherID,
	)
	monitoring.RecordDispatcherTakeover(ctx, o.dispatcherID, time.Since(attemptStart), monitoring.TakeoverOutcomeError)
	return retry.RetryableError(startErr)
}

func (o *Worker) logStaleDispatcher(ctx context.Context) {
	logger.FromContext(ctx).Info(
		"Dispatcher already running, terminating stale execution",
		"dispatcher_id",
		o.dispatcherID,
		"project",
		o.projectConfig.Name,
	)
}

func (o *Worker) terminateStaleDispatcher(
	ctx context.Context,
	attemptStart time.Time,
	attemptTimeout time.Duration,
	startErr error,
) error {
	terminateCtx, terminateCancel := context.WithTimeout(ctx, attemptTimeout)
	defer terminateCancel()
	terminateErr := o.client.TerminateWorkflow(terminateCtx, o.dispatcherID, "", dispatcherTakeoverReason)
	duration := time.Since(attemptStart)
	if terminateErr != nil {
		if _, ok := terminateErr.(*serviceerror.NotFound); ok {
			monitoring.RecordDispatcherTakeover(ctx, o.dispatcherID, duration, monitoring.TakeoverOutcomeTerminated)
			return retry.RetryableError(startErr)
		}
		logger.FromContext(ctx).Warn(
			"Failed to terminate stale dispatcher",
			"error",
			terminateErr,
			"dispatcher_id",
			o.dispatcherID,
		)
		monitoring.RecordDispatcherTakeover(ctx, o.dispatcherID, duration, monitoring.TakeoverOutcomeTerminateError)
		return retry.RetryableError(terminateErr)
	}
	monitoring.RecordDispatcherTakeover(ctx, o.dispatcherID, duration, monitoring.TakeoverOutcomeTerminated)
	return retry.RetryableError(startErr)
}

// HealthCheck performs a comprehensive health check including cache connectivity
func (o *Worker) HealthCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("config manager not found in context")
	}
	if o.redisCache != nil {
		if err := o.redisCache.HealthCheck(ctx); err != nil {
			return fmt.Errorf("redis cache health check failed: %w", err)
		}
	}
	if o.dispatcherID != "" && o.activities != nil {
		input := &wkacts.ListActiveDispatchersInput{
			StaleThreshold: cfg.Runtime.DispatcherStaleThreshold,
		}
		output, err := o.activities.ListActiveDispatchers(ctx, input)
		if err != nil {
			log.Warn("Failed to check dispatcher health", "error", err)
		} else {
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
	if err := o.checkMCPProxyHealth(ctx); err != nil {
		log.Warn("MCP proxy health check failed", "error", err)
	}
	return nil
}

// checkMCPProxyHealth checks if MCP proxy is healthy when configured
func (o *Worker) checkMCPProxyHealth(ctx context.Context) error {
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return nil // treat as "no proxy configured"
	}
	if cfg.LLM.ProxyURL == "" {
		return nil // No proxy configured
	}
	client := mcp.NewProxyClient(ctx, cfg.LLM.ProxyURL, cfg.Worker.MCPProxyHealthCheckTimeout)
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
	workflowExecID := core.MustNewID()
	log := logger.FromContext(ctx)
	log.Debug("TriggerWorkflow requested", "workflow_id", workflowID, "registered_workflows", len(o.workflows))
	workflowConfig, err := wf.FindConfig(o.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	workflowInput, mergedInput, err := o.prepareWorkflowInput(
		ctx,
		workflowConfig,
		workflowID,
		workflowExecID,
		input,
		initTaskID,
	)
	if err != nil {
		return nil, err
	}
	if err := o.persistPendingState(ctx, workflowID, workflowExecID, mergedInput); err != nil {
		return nil, err
	}
	options := client.StartWorkflowOptions{
		ID:        buildWorkflowID(workflowID, workflowInput.WorkflowExecID),
		TaskQueue: o.taskQueue,
	}
	if err := o.startWorkflowExecution(ctx, &options, workflowInput, mergedInput); err != nil {
		return nil, err
	}
	o.transitionWorkflowToRunning(ctx, workflowID, workflowExecID)
	return &workflowInput, nil
}

// prepareWorkflowInput merges defaults, validates payloads, and builds workflow input.
func (o *Worker) prepareWorkflowInput(
	ctx context.Context,
	workflowConfig *wf.Config,
	workflowID string,
	workflowExecID core.ID,
	input *core.Input,
	initTaskID string,
) (WorkflowInput, *core.Input, error) {
	mergedInput, err := workflowConfig.ApplyInputDefaults(input)
	if err != nil {
		return WorkflowInput{}, nil, fmt.Errorf("failed to apply input defaults: %w", err)
	}
	if validationErr := workflowConfig.ValidateInput(ctx, mergedInput); validationErr != nil {
		return WorkflowInput{}, nil, fmt.Errorf("failed to validate workflow params: %w", validationErr)
	}
	workflowInput := WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          mergedInput,
		InitialTaskID:  initTaskID,
	}
	if cfg := appconfig.FromContext(ctx); cfg != nil {
		workflowInput.ErrorHandlerTimeout = cfg.Worker.ErrorHandlerTimeout
		workflowInput.ErrorHandlerMaxRetries = cfg.Worker.ErrorHandlerMaxRetries
	}
	return workflowInput, mergedInput, nil
}

// persistPendingState stores an initial pending state prior to workflow start.
func (o *Worker) persistPendingState(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	input *core.Input,
) error {
	pendingState := wf.NewState(workflowID, workflowExecID, input).WithStatus(core.StatusPending)
	if err := o.config.WorkflowRepo().UpsertState(ctx, pendingState); err != nil {
		return fmt.Errorf("failed to persist initial (pending) workflow state: %w", err)
	}
	return nil
}

// startWorkflowExecution starts the Temporal workflow and persists failure states when necessary.
func (o *Worker) startWorkflowExecution(
	ctx context.Context,
	options *client.StartWorkflowOptions,
	workflowInput WorkflowInput,
	mergedInput *core.Input,
) error {
	timeout := workflowStartTimeoutDefault
	if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.Worker.StartWorkflowTimeout > 0 {
		timeout = cfg.Worker.StartWorkflowTimeout
	}
	startCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := o.client.ExecuteWorkflow(startCtx, *options, CompozyWorkflow, workflowInput); err != nil {
		o.persistFailedStartState(ctx, workflowInput.WorkflowID, workflowInput.WorkflowExecID, mergedInput, err)
		return fmt.Errorf("failed to start workflow: %w", err)
	}
	return nil
}

// persistFailedStartState best-effort persists a failed state when Temporal start fails.
func (o *Worker) persistFailedStartState(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	input *core.Input,
	startErr error,
) {
	repo := o.config.WorkflowRepo()
	failed := wf.NewState(workflowID, workflowExecID, input).
		WithStatus(core.StatusFailed).
		WithError(core.NewError(startErr, "WORKFLOW_START_FAILED", map[string]any{
			"workflow_id": workflowID,
			"exec_id":     workflowExecID.String(),
		}))
	if err := repo.UpsertState(ctx, failed); err != nil {
		logger.FromContext(ctx).Error(
			"Failed to persist failed workflow state after start error",
			"error", err, "workflow_id", workflowID, "exec_id", workflowExecID,
		)
	}
}

// transitionWorkflowToRunning logs errors but does not propagate failures during state promotion.
func (o *Worker) transitionWorkflowToRunning(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
) {
	if err := o.config.WorkflowRepo().UpdateStatus(ctx, workflowExecID, core.StatusRunning); err != nil {
		logger.FromContext(ctx).Error(
			"Failed to transition workflow state to RUNNING after start",
			"error", err, "workflow_id", workflowID, "exec_id", workflowExecID,
		)
	}
}

func (o *Worker) CancelWorkflow(ctx context.Context, workflowID string, workflowExecID core.ID) error {
	temporalID := buildWorkflowID(workflowID, workflowExecID)
	return o.client.CancelWorkflow(ctx, temporalID, "")
}
