package compozy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

// Compozy represents an embedded Compozy engine instance that exposes lifecycle
// management and direct workflow execution capabilities for SDK-built
// projects.
type Compozy struct {
	mu            sync.RWMutex
	server        *server.Server
	manager       *config.Manager
	config        *config.Config
	ctx           context.Context
	cancel        context.CancelFunc
	project       *project.Config
	workflowByID  map[string]*workflow.Config
	workflowOrder []string
	store         resources.ResourceStore
	router        *gin.Engine
	httpServer    *http.Server
	httpWG        sync.WaitGroup
	httpErr       error
	httpAddr      string
	started       bool
	stopped       bool
}

// ExecutionResult contains the outcome of a workflow execution triggered
// through the embedded Compozy instance.
type ExecutionResult struct {
	WorkflowID string
	Output     core.Output
}

// Builder constructs embedded Compozy instances from SDK-generated engine
// configurations while collecting validation issues until Build is invoked.
type Builder struct {
	project   *project.Config
	workflows []*workflow.Config
	store     resources.ResourceStore
	errors    []error

	serverHost    string
	serverPort    int
	serverPortSet bool
	corsEnabled   bool
	corsOrigins   []string
	authEnabled   bool

	dbConnString string
	temporalHost string
	temporalNS   string
	redisURL     string

	workingDir string
	configFile string
	envFile    string
	logLevel   string
}

// New creates a builder configured with the provided project configuration.
func New(projectCfg *project.Config) *Builder {
	return &Builder{
		project:   projectCfg,
		workflows: make([]*workflow.Config, 0),
		errors:    make([]error, 0),
	}
}

// WithWorkflows registers one or more workflow configurations that should be
// loaded into the embedded engine.
func (b *Builder) WithWorkflows(workflows ...*workflow.Config) *Builder {
	if b == nil {
		return nil
	}
	b.workflows = append(b.workflows, workflows...)
	return b
}

// WithResourceStore overrides the resource store used for registration. When
// not provided a new in-memory store is created during Build.
func (b *Builder) WithResourceStore(store resources.ResourceStore) *Builder {
	if b == nil {
		return nil
	}
	b.store = store
	return b
}

// WithServerHost configures the HTTP server host binding.
func (b *Builder) WithServerHost(host string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("server host cannot be empty"))
		return b
	}
	b.serverHost = trimmed
	return b
}

// WithServerPort configures the HTTP server port. Port zero selects a random port.
func (b *Builder) WithServerPort(port int) *Builder {
	if b == nil {
		return nil
	}
	if port < 0 || port > 65535 {
		b.errors = append(b.errors, fmt.Errorf("server port must be between 0 and 65535"))
		return b
	}
	b.serverPort = port
	b.serverPortSet = true
	return b
}

// WithCORS enables CORS with the provided origins. Supplying no origins leaves
// the configuration unchanged and records an error.
func (b *Builder) WithCORS(enabled bool, origins ...string) *Builder {
	if b == nil {
		return nil
	}
	b.corsEnabled = enabled
	if !enabled {
		b.corsOrigins = nil
		return b
	}
	if len(origins) == 0 {
		b.errors = append(b.errors, fmt.Errorf("at least one CORS origin must be provided when enabling CORS"))
		return b
	}
	filtered := make([]string, 0, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		b.errors = append(b.errors, fmt.Errorf("CORS origins cannot be empty"))
		return b
	}
	b.corsOrigins = filtered
	return b
}

// WithAuth toggles HTTP authentication for the embedded server.
func (b *Builder) WithAuth(enabled bool) *Builder {
	if b == nil {
		return nil
	}
	b.authEnabled = enabled
	return b
}

// WithDatabase configures the database connection string required by the server.
func (b *Builder) WithDatabase(connString string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(connString)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("database connection string cannot be empty"))
		return b
	}
	b.dbConnString = trimmed
	return b
}

// WithTemporal configures the Temporal host/port and namespace used for workflow execution.
func (b *Builder) WithTemporal(hostPort, namespace string) *Builder {
	if b == nil {
		return nil
	}
	trimmedHost := strings.TrimSpace(hostPort)
	trimmedNS := strings.TrimSpace(namespace)
	if trimmedHost == "" {
		b.errors = append(b.errors, fmt.Errorf("temporal host:port cannot be empty"))
	} else {
		b.temporalHost = trimmedHost
	}
	if trimmedNS == "" {
		b.errors = append(b.errors, fmt.Errorf("temporal namespace cannot be empty"))
	} else {
		b.temporalNS = trimmedNS
	}
	return b
}

// WithRedis configures the Redis connection string required for resource storage.
func (b *Builder) WithRedis(url string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("redis url cannot be empty"))
		return b
	}
	b.redisURL = trimmed
	return b
}

// WithWorkingDirectory sets the working directory for the embedded server.
func (b *Builder) WithWorkingDirectory(cwd string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("working directory cannot be empty"))
		return b
	}
	b.workingDir = trimmed
	return b
}

// WithConfigFile sets the path to the project configuration file used by the embedded server.
func (b *Builder) WithConfigFile(path string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("config file path cannot be empty"))
		return b
	}
	b.configFile = trimmed
	return b
}

// WithEnvFile sets the path to the environment file consumed by the embedded server.
func (b *Builder) WithEnvFile(path string) *Builder {
	if b == nil {
		return nil
	}
	b.envFile = strings.TrimSpace(path)
	return b
}

// WithLogLevel overrides the runtime log level used by the embedded server.
func (b *Builder) WithLogLevel(level string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(level)
	if trimmed == "" {
		return b
	}
	switch strings.ToLower(trimmed) {
	case string(logger.DebugLevel), string(logger.InfoLevel), string(logger.WarnLevel), string(logger.ErrorLevel):
		b.logLevel = strings.ToLower(trimmed)
	default:
		b.errors = append(b.errors, fmt.Errorf("invalid log level: %s", trimmed))
	}
	return b
}

// Build validates inputs, clones configurations, seeds the resource store, and
// returns a ready-to-use embedded Compozy engine instance.
func (b *Builder) Build(ctx context.Context) (*Compozy, error) {
	if b == nil {
		return nil, fmt.Errorf("builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	logger.FromContext(ctx).Debug("building embedded Compozy instance")

	workflows, wfErrs := b.cloneWorkflows()
	if errs := b.collectBuildErrors(workflows, wfErrs); len(errs) > 0 {
		return nil, &sdkerrors.BuildError{Errors: errs}
	}

	projectClone, err := core.DeepCopy(b.project)
	if err != nil {
		return nil, fmt.Errorf("failed to clone project config: %w", err)
	}
	if err := b.ensureProjectCWD(projectClone); err != nil {
		return nil, err
	}

	manager, cfg, srvCtx, cancel, err := b.initConfigManager(ctx)
	if err != nil {
		return nil, err
	}
	instance, err := server.NewServer(srvCtx, cfg.CLI.CWD, cfg.CLI.ConfigFile, b.envFile)
	if err != nil {
		cancel()
		_ = manager.Close(srvCtx)
		return nil, fmt.Errorf("failed to create embedded server: %w", err)
	}

	comp := b.assembleCompozy(srvCtx, cancel, manager, cfg, projectClone, workflows, instance)
	if err := comp.loadProjectIntoEngine(srvCtx, projectClone); err != nil {
		cancel()
		_ = manager.Close(srvCtx)
		return nil, err
	}

	return comp, nil
}

func (b *Builder) collectBuildErrors(workflows []*workflow.Config, wfErrs []error) []error {
	collected := append([]error{}, b.errors...)
	collected = append(collected, wfErrs...)
	if b.project == nil {
		collected = append(collected, fmt.Errorf("project config is required"))
	}
	if strings.TrimSpace(b.dbConnString) == "" {
		collected = append(collected, fmt.Errorf("database connection string is required"))
	}
	if strings.TrimSpace(b.temporalHost) == "" {
		collected = append(collected, fmt.Errorf("temporal host:port is required"))
	}
	if strings.TrimSpace(b.temporalNS) == "" {
		collected = append(collected, fmt.Errorf("temporal namespace is required"))
	}
	if strings.TrimSpace(b.redisURL) == "" {
		collected = append(collected, fmt.Errorf("redis url is required"))
	}
	if len(workflows) == 0 {
		collected = append(collected, fmt.Errorf("at least one workflow must be provided"))
	}
	if b.project != nil && b.workingDir == "" {
		if b.project.GetCWD() == nil {
			collected = append(collected, fmt.Errorf("project working directory must be configured"))
		}
	}
	return filterErrors(collected)
}

func (b *Builder) initConfigManager(
	ctx context.Context,
) (*config.Manager, *config.Config, context.Context, context.CancelFunc, error) {
	baseCtx, cancel := context.WithCancel(ctx)
	manager := config.NewManager(baseCtx, config.NewService())
	if _, err := manager.Load(baseCtx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("failed to load base configuration: %w", err)
	}
	cfg := manager.Get()
	if err := b.applyServerConfig(cfg); err != nil {
		cancel()
		_ = manager.Close(baseCtx)
		return nil, nil, nil, nil, err
	}
	srvCtx := config.ContextWithManager(baseCtx, manager)
	return manager, cfg, srvCtx, cancel, nil
}

func (b *Builder) assembleCompozy(
	ctx context.Context,
	cancel context.CancelFunc,
	manager *config.Manager,
	cfg *config.Config,
	projectClone *project.Config,
	workflows []*workflow.Config,
	instance *server.Server,
) *Compozy {
	store := b.resolveStore()
	comp := &Compozy{
		server:        instance,
		manager:       manager,
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
		project:       projectClone,
		workflowByID:  make(map[string]*workflow.Config, len(workflows)),
		workflowOrder: make([]string, len(workflows)),
		store:         store,
	}
	for idx, wf := range workflows {
		id := strings.TrimSpace(wf.ID)
		comp.workflowByID[id] = wf
		comp.workflowOrder[idx] = id
	}
	return comp
}

func (b *Builder) resolveStore() resources.ResourceStore {
	if b.store != nil {
		return b.store
	}
	return resources.NewMemoryResourceStore()
}

// Start launches the embedded HTTP server. Multiple invocations are safe.
func (c *Compozy) Start() error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if c.ctx == nil {
		return fmt.Errorf("compozy instance is not properly initialized")
	}

	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	router := newRouter()
	host := c.config.Server.Host
	port := c.config.Server.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	httpSrv := &http.Server{
		Handler: router,
		BaseContext: func(net.Listener) context.Context {
			return c.ctx
		},
	}
	c.router = router
	c.httpServer = httpSrv
	c.httpAddr = listener.Addr().String()
	c.started = true
	c.mu.Unlock()

	log := logger.FromContext(c.ctx)
	log.Info("embedded Compozy server starting", "address", c.httpAddr)

	c.httpWG.Add(1)
	go func() {
		defer c.httpWG.Done()
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("embedded Compozy server failed", "error", err)
			c.mu.Lock()
			c.httpErr = err
			c.mu.Unlock()
		}
	}()

	return nil
}

// Stop gracefully shuts down the embedded HTTP server using the provided context.
func (c *Compozy) Stop(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	serverRef := c.httpServer
	manager := c.manager
	c.stopped = true
	c.mu.Unlock()

	if serverRef != nil {
		if err := serverRef.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("failed to shutdown embedded server: %w", err)
		}
	}
	if c.cancel != nil {
		c.cancel()
	}
	if manager != nil {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Debug("failed to close config manager", "error", err)
		}
	}
	return nil
}

// Wait blocks until the embedded server stops and returns any runtime error.
func (c *Compozy) Wait() error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	c.httpWG.Wait()
	c.mu.RLock()
	err := c.httpErr
	c.mu.RUnlock()
	return err
}

// ExecuteWorkflow retrieves the requested workflow from the resource store and
// produces a synthetic execution result using configured workflow outputs.
func (c *Compozy) ExecuteWorkflow(
	ctx context.Context,
	workflowID string,
	input map[string]any,
) (*ExecutionResult, error) {
	if c == nil {
		return nil, fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	id := strings.TrimSpace(workflowID)
	if id == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return nil, fmt.Errorf("resource store is not configured")
	}
	log := logger.FromContext(ctx)
	log.Info("executing workflow", "workflow", id)
	c.mu.RLock()
	wf, ok := c.workflowByID[id]
	projectName := ""
	if c.project != nil {
		projectName = c.project.Name
	}
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("workflow %s not registered", id)
	}
	key := resources.ResourceKey{Project: projectName, Type: resources.ResourceWorkflow, ID: id}
	if _, _, err := store.Get(ctx, key); err != nil {
		return nil, fmt.Errorf("workflow %s not registered in store: %w", id, err)
	}
	output := core.Output{}
	if wf.Outputs != nil {
		cloned, err := core.DeepCopy(*wf.Outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to copy workflow outputs: %w", err)
		}
		output = cloned
	} else if len(input) > 0 {
		cloned := core.CloneMap(input)
		output = core.Output(cloned)
	}
	log.Info("workflow executed", "workflow", id)
	return &ExecutionResult{WorkflowID: id, Output: output}, nil
}

// Server exposes the underlying engine server instance for advanced usage.
func (c *Compozy) Server() *server.Server {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.server
}

// Router returns the HTTP router used by the embedded server after Start.
func (c *Compozy) Router() *gin.Engine {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.router
}

// Config returns the application configuration bound to the embedded server.
func (c *Compozy) Config() *config.Config {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// ResourceStore exposes the underlying resource store used by the embedded
// engine for registration and lookup.
func (c *Compozy) ResourceStore() resources.ResourceStore {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store
}

func (b *Builder) cloneWorkflows() ([]*workflow.Config, []error) {
	if len(b.workflows) == 0 {
		return nil, nil
	}
	result := make([]*workflow.Config, 0, len(b.workflows))
	errs := make([]error, 0, len(b.workflows))
	seen := make(map[string]struct{}, len(b.workflows))
	for _, wf := range b.workflows {
		if wf == nil {
			errs = append(errs, fmt.Errorf("workflow cannot be nil"))
			continue
		}
		id := strings.TrimSpace(wf.ID)
		if id == "" {
			errs = append(errs, fmt.Errorf("workflow id is required"))
			continue
		}
		if _, exists := seen[id]; exists {
			errs = append(errs, fmt.Errorf("duplicate workflow id '%s'", id))
			continue
		}
		clone, err := core.DeepCopy(wf)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to clone workflow %s: %w", id, err))
			continue
		}
		result = append(result, clone)
		seen[id] = struct{}{}
	}
	return result, errs
}

func (b *Builder) applyServerConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is required")
	}
	if b.serverHost != "" {
		cfg.Server.Host = b.serverHost
	}
	if b.serverPortSet {
		cfg.Server.Port = b.serverPort
	}
	cfg.Server.CORSEnabled = b.corsEnabled
	if len(b.corsOrigins) > 0 {
		cfg.Server.CORS.AllowedOrigins = append([]string(nil), b.corsOrigins...)
	}
	cfg.Server.Auth.Enabled = b.authEnabled
	cfg.Server.SourceOfTruth = "builder"
	cfg.Database.ConnString = b.dbConnString
	cfg.Temporal.HostPort = b.temporalHost
	cfg.Temporal.Namespace = b.temporalNS
	cfg.Redis.URL = b.redisURL
	if b.workingDir != "" {
		cfg.CLI.CWD = b.workingDir
	}
	if b.configFile != "" {
		cfg.CLI.ConfigFile = b.configFile
	} else if cfg.CLI.CWD != "" && cfg.CLI.ConfigFile == "" {
		cfg.CLI.ConfigFile = filepath.Join(cfg.CLI.CWD, "compozy.yaml")
	}
	if b.envFile != "" {
		cfg.CLI.EnvFile = b.envFile
	}
	if b.logLevel != "" {
		cfg.Runtime.LogLevel = b.logLevel
	}
	return nil
}

func (b *Builder) ensureProjectCWD(proj *project.Config) error {
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	if b.workingDir != "" {
		return proj.SetCWD(b.workingDir)
	}
	if proj.GetCWD() != nil {
		return nil
	}
	return fmt.Errorf("project working directory must be configured")
}

func newRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	return r
}

func filterErrors(errs []error) []error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
