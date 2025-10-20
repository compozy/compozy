package serverhelpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	lgmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/logger"
	serverrouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
	ctxhelpers "github.com/compozy/compozy/test/helpers/ctx"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type ServerOptions struct {
	ProjectName string
}

type Option func(*ServerOptions)

func WithProjectName(name string) Option {
	return func(o *ServerOptions) {
		o.ProjectName = name
	}
}

type ServerHarness struct {
	Engine        *gin.Engine
	State         *appstate.State
	Ctx           context.Context
	Config        *config.Config
	ResourceStore resources.ResourceStore
	Project       *project.Config
	Server        *server.Server
	DB            *pgxpool.Pool
}

func NewServerHarness(t *testing.T, opts ...Option) *ServerHarness {
	t.Helper()

	ctx := ctxhelpers.TestContext(t)
	options := resolveServerOptions(t, opts)

	ctx, cfg := loadTestConfig(ctx, t)
	applyServerDefaults(cfg)

	proj, projectFile, projectDir := prepareProject(t, options.ProjectName)
	pool := prepareDatabase(t, cfg)
	state, store := buildApplicationState(t, proj, pool)
	engine, srv := buildGinComponents(ctx, t, cfg, state, projectDir, projectFile)

	return &ServerHarness{
		Engine:        engine,
		State:         state,
		Ctx:           ctx,
		Config:        cfg,
		ResourceStore: store,
		Project:       proj,
		Server:        srv,
		DB:            pool,
	}
}

// resolveServerOptions applies functional options and ensures defaults.
func resolveServerOptions(t *testing.T, opts []Option) ServerOptions {
	options := ServerOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.ProjectName == "" {
		options.ProjectName = sanitizeProjectName(t.Name())
	}
	return options
}

// loadTestConfig loads the default configuration and binds it to the context.
func loadTestConfig(ctx context.Context, t *testing.T) (context.Context, *config.Config) {
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	ctx = config.ContextWithManager(ctx, manager)
	return ctx, manager.Get()
}

// applyServerDefaults tunes server configuration for isolated tests.
func applyServerDefaults(cfg *config.Config) {
	cfg.Server.Auth.Enabled = false
	cfg.Server.CORSEnabled = false
	cfg.Server.SourceOfTruth = "repo"
	cfg.Server.Timeouts.StartProbeDelay = time.Millisecond
	cfg.RateLimit.GlobalRate.Limit = 0
	cfg.RateLimit.APIKeyRate.Limit = 0
	cfg.Webhooks.DefaultMaxBody = 1 << 20
	cfg.Webhooks.DefaultDedupeTTL = time.Minute
}

// prepareProject scaffolds a temporary project configuration for tests.
func prepareProject(t *testing.T, projectName string) (*project.Config, string, string) {
	tempDir := t.TempDir()
	projFile := filepath.Join(tempDir, "compozy.yaml")
	yamlContent := fmt.Sprintf("name: %s\nversion: 1.0.0\n", projectName)
	require.NoError(t, os.WriteFile(projFile, []byte(yamlContent), 0o600))

	proj := &project.Config{Name: projectName, Version: "1.0.0"}
	require.NoError(t, proj.SetCWD(tempDir))
	proj.SetFilePath(projFile)
	return proj, projFile, tempDir
}

// prepareDatabase provisions a shared test database connection pool.
func prepareDatabase(t *testing.T, cfg *config.Config) *pgxpool.Pool {
	pool, cleanup := helpers.GetSharedPostgresDB(t)
	t.Cleanup(cleanup)
	require.NoError(t, helpers.EnsureTablesExistForTest(pool))

	cfg.Database.ConnString = pool.Config().ConnString()
	cfg.Database.AutoMigrate = false
	return pool
}

// buildApplicationState constructs application state and resource store.
func buildApplicationState(
	t *testing.T,
	proj *project.Config,
	pool *pgxpool.Pool,
) (*appstate.State, resources.ResourceStore) {
	deps := appstate.NewBaseDeps(proj, nil, repo.NewProvider(pool), nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)

	store := resources.NewMemoryResourceStore()
	state.SetResourceStore(store)
	return state, store
}

// buildGinComponents wires Gin, middleware, and server routes for tests.
func buildGinComponents(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	state *appstate.State,
	projectDir, projectFile string,
) (*gin.Engine, *server.Server) {
	ginmode.EnsureGinTestMode()

	srv, err := server.NewServer(ctx, projectDir, projectFile, "")
	require.NoError(t, err)

	engine := gin.New()
	engine.Use(gin.Recovery())

	authFactory := authuc.NewFactory(state.Store.NewAuthRepo())
	authManager := authmw.NewManager(authFactory, cfg)
	engine.Use(authManager.Middleware())
	engine.Use(lgmiddleware.Middleware(ctx))
	engine.Use(appstate.StateMiddleware(state))
	engine.Use(serverrouter.ErrorHandler())

	require.NoError(t, server.RegisterRoutes(ctx, engine, state, srv))
	return engine, srv
}

func sanitizeProjectName(name string) string {
	clean := strings.ToLower(name)
	clean = strings.ReplaceAll(clean, " ", "-")
	clean = strings.ReplaceAll(clean, "/", "-")
	clean = strings.ReplaceAll(clean, "\\", "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = fmt.Sprintf("project-%d", time.Now().UnixNano())
	}
	return clean
}
