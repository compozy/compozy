package embedded

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	serverstate "github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool/builder"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Env wires a minimal end-to-end environment with:
// - Embedded Temporal (memory mode)
// - Postgres-backed repos
// - Real Worker instance
// - Workflows loaded from test fixtures
type Env struct {
	Ctx       context.Context
	Cfg       *config.Config
	DB        *pgxpool.Pool
	Project   *project.Config
	Workflows []*workflow.Config
	Worker    *worker.Worker
	Temporal  *embedded.Server
	Registry  *autoload.ConfigRegistry
	Cleanup   func()
}

// SetupMemoryTestEnv creates a complete environment suitable for exercising
// end-to-end workflow execution in memory mode using embedded Temporal and
// embedded Redis (miniredis via cache.SetupCache in the worker).
//
// The function:
// - Loads default config, enforces memory mode for Redis/Temporal
// - Starts an embedded Temporal server on a free port
// - Creates a project with mock LLM provider for deterministic runs
// - Loads workflows from the provided relative fixture paths
// - Boots a real Worker wired to Postgres test DB and the embedded Temporal
func SetupMemoryTestEnv(t *testing.T, workflowPaths ...string) *Env {
	t.Helper()
	ctx := testhelpers.NewTestContext(t)
	if logger.FromContext(ctx) == nil {
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	}

	// Config + DB
	ctx, cfg, mgr := initConfig(ctx, t)
	pool := initDatabase(t, cfg)
	// Temporal
	_, srv := initTemporal(ctx, t, cfg)
	// Project + workflows
	proj := initProject(t)
	wfs := loadWorkflows(ctx, t, workflowPaths...)
	// Worker
	deps, cleanupProvider := buildBaseDeps(ctx, proj, cfg)
	t.Cleanup(cleanupProvider)
	registry := autoload.NewConfigRegistry()
	w := startWorker(ctx, t, deps, registry, proj, wfs)

	env := &Env{
		Ctx:       ctx,
		Cfg:       cfg,
		DB:        pool,
		Project:   proj,
		Workflows: wfs,
		Worker:    w,
		Temporal:  srv,
		Registry:  registry,
	}
	env.Cleanup = func() {
		stopTemporal(ctx, t, srv)
		stopWorker(ctx, w)
		_ = mgr.Close(ctx)
	}
	t.Cleanup(env.Cleanup)
	return env
}

// --- Smaller helpers to satisfy linter funlen ---

func initConfig(ctx context.Context, t *testing.T) (context.Context, *config.Config, *config.Manager) {
	mgr := config.NewManager(ctx, config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := mgr.Get()
	cfg.Mode = config.ModeMemory
	cfg.Redis.Mode = config.ModeMemory
	cfg.Temporal.Mode = config.ModeMemory
	cfg.Server.Auth.Enabled = false
	cfg.Server.SourceOfTruth = "repo"
	cfg.Server.Timeouts.StartProbeDelay = time.Millisecond
	ctx = config.ContextWithManager(ctx, mgr)
	return ctx, cfg, mgr
}

func initDatabase(t *testing.T, cfg *config.Config) *pgxpool.Pool {
	pool, cleanup := testhelpers.GetSharedPostgresDB(t)
	t.Cleanup(cleanup)
	require.NoError(t, testhelpers.EnsureTablesExistForTest(pool))
	cfg.Database.ConnString = pool.Config().ConnString()
	cfg.Database.AutoMigrate = false
	cfg.Database.Driver = "postgres"
	return pool
}

func initTemporal(ctx context.Context, t *testing.T, cfg *config.Config) (*embedded.Config, *embedded.Server) {
	e := newEmbeddedConfig(t)
	e.FrontendPort = findFreePortRange(ctx, t, 4)
	srv := startTemporal(ctx, t, e)
	cfg.Temporal.HostPort = srv.FrontendAddress()
	cfg.Temporal.Namespace = e.Namespace
	return e, srv
}

func initProject(t *testing.T) *project.Config {
	proj := &project.Config{
		Name:    sanitizeProjectName(t.Name()),
		Version: "1.0.0",
		Models:  []*core.ProviderConfig{{Provider: "mock", Model: "mock-model", Default: true}},
	}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	return proj
}

func loadWorkflows(ctx context.Context, t *testing.T, relPaths ...string) []*workflow.Config {
	cwd := projectCWDFromRepoRoot(t)
	var out []*workflow.Config
	for _, rel := range relPaths {
		wf, err := workflow.Load(ctx, cwd, filepath.ToSlash(rel))
		require.NoErrorf(t, err, "load workflow fixture: %s", rel)
		out = append(out, wf)
	}
	return out
}

func buildBaseDeps(
	ctx context.Context,
	proj *project.Config,
	cfg *config.Config,
) (serverstate.BaseDeps, func()) {
	provider, cleanup, err := repo.NewProvider(ctx, &cfg.Database)
	if err != nil {
		panic(fmt.Errorf("buildBaseDeps: failed to create repo provider: %w", err))
	}
	deps := serverstate.NewBaseDeps(proj, nil, provider, &worker.TemporalConfig{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
		TaskQueue: cfg.Temporal.TaskQueue,
	})
	return deps, cleanup
}

func startWorker(
	ctx context.Context,
	t *testing.T,
	deps serverstate.BaseDeps,
	reg *autoload.ConfigRegistry,
	proj *project.Config,
	wfs []*workflow.Config,
) *worker.Worker {
	w, err := buildWorker(ctx, deps, reg, proj, wfs)
	require.NoError(t, err)
	require.NoError(t, w.Setup(ctx))
	return w
}

// buildWorker mirrors the server worker setup to produce a real Worker instance.
func buildWorker(
	ctx context.Context,
	deps serverstate.BaseDeps,
	reg *autoload.ConfigRegistry,
	proj *project.Config,
	wfs []*workflow.Config,
) (*worker.Worker, error) {
	toolEnv, err := resolveToolEnv(ctx, proj, wfs, deps)
	if err != nil {
		return nil, err
	}
	wcfg := &worker.Config{
		WorkflowRepo:     func() workflow.Repository { return deps.Store.NewWorkflowRepo() },
		TaskRepo:         func() task.Repository { return deps.Store.NewTaskRepo() },
		ResourceRegistry: reg,
	}
	return worker.NewWorker(ctx, wcfg, deps.ClientConfig, proj, wfs, toolEnv)
}

// resolveToolEnv builds a basic tool environment by indexing resources from project+workflows.
func resolveToolEnv(
	ctx context.Context,
	proj *project.Config,
	wfs []*workflow.Config,
	deps serverstate.BaseDeps,
) (toolenv.Environment, error) {
	// Index project+workflows into a fresh in-memory resource store
	store := resources.NewMemoryResourceStore()
	if err := proj.IndexToResourceStore(ctx, store); err != nil {
		return nil, err
	}
	for _, wf := range wfs {
		if err := wf.IndexToResourceStore(ctx, proj.Name, store); err != nil {
			return nil, err
		}
	}
	// Build environment the same way server does
	wfRepo := deps.Store.NewWorkflowRepo()
	tkRepo := deps.Store.NewTaskRepo()
	return builder.Build(proj, wfs, wfRepo, tkRepo, store)
}

// ---- Temporal helpers ----

func newEmbeddedConfig(t *testing.T) *embedded.Config {
	defaults := config.Default().Temporal.Standalone
	memDir := t.TempDir()
	return &embedded.Config{
		DatabaseFile: filepath.Join(memDir, "temporal.db"),
		FrontendPort: defaults.FrontendPort,
		BindIP:       defaults.BindIP,
		Namespace:    defaults.Namespace,
		ClusterName:  defaults.ClusterName,
		EnableUI:     false,
		RequireUI:    false,
		UIPort:       defaults.UIPort,
		LogLevel:     defaults.LogLevel,
		StartTimeout: defaults.StartTimeout,
	}
}

func startTemporal(ctx context.Context, t *testing.T, cfg *embedded.Config) *embedded.Server {
	t.Helper()
	srv, err := embedded.NewServer(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, srv.Start(ctx))
	return srv
}

func stopTemporal(ctx context.Context, t *testing.T, srv *embedded.Server) {
	t.Helper()
	if srv == nil {
		return
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	require.NoError(t, srv.Stop(sctx))
}

func stopWorker(ctx context.Context, w *worker.Worker) {
	if w == nil {
		return
	}
	w.Stop(context.WithoutCancel(ctx))
}

// findFreePortRange searches for a contiguous port range starting at an available port.
func findFreePortRange(ctx context.Context, t *testing.T, size int) int {
	t.Helper()
	for port := 35000; port < 45000; port++ {
		ok := true
		for i := 0; i < size; i++ {
			if !portAvailable(ctx, port+i) {
				ok = false
				break
			}
		}
		// Check UI offset as well for parity with embedded server (even if UI disabled)
		if ok && portAvailable(ctx, port+1000) {
			return port
		}
	}
	t.Fatalf("no available port range found for size=%d", size)
	return 0
}

func portAvailable(ctx context.Context, port int) bool {
	dctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	ln, err := (&net.ListenConfig{}).Listen(dctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func projectCWDFromRepoRoot(t *testing.T) *core.PathCWD {
	t.Helper()
	root, err := testhelpers.FindProjectRoot()
	require.NoError(t, err)
	cwd, err := core.CWDFromPath(root)
	require.NoError(t, err)
	return cwd
}

func sanitizeProjectName(name string) string {
	if name == "" {
		return fmt.Sprintf("proj-%d", time.Now().UnixNano())
	}
	return name
}
