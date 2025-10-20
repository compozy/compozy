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
	options := ServerOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.ProjectName == "" {
		options.ProjectName = sanitizeProjectName(t.Name())
	}
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	ctx = config.ContextWithManager(ctx, manager)
	cfg := manager.Get()
	cfg.Server.Auth.Enabled = false
	cfg.Server.CORSEnabled = false
	cfg.Server.SourceOfTruth = "repo"
	cfg.Server.Timeouts.StartProbeDelay = time.Millisecond
	cfg.RateLimit.GlobalRate.Limit = 0
	cfg.RateLimit.APIKeyRate.Limit = 0
	cfg.Webhooks.DefaultMaxBody = 1 << 20
	cfg.Webhooks.DefaultDedupeTTL = time.Minute
	tempDir := t.TempDir()
	projFile := filepath.Join(tempDir, "compozy.yaml")
	yamlContent := fmt.Sprintf("name: %s\nversion: 1.0.0\n", options.ProjectName)
	require.NoError(t, os.WriteFile(projFile, []byte(yamlContent), 0o600))
	proj := &project.Config{Name: options.ProjectName, Version: "1.0.0"}
	require.NoError(t, proj.SetCWD(tempDir))
	proj.SetFilePath(projFile)
	pool, cleanup := helpers.GetSharedPostgresDB(t)
	t.Cleanup(cleanup)
	require.NoError(t, helpers.EnsureTablesExistForTest(pool))
	cfg.Database.ConnString = pool.Config().ConnString()
	cfg.Database.AutoMigrate = false
	deps := appstate.NewBaseDeps(proj, nil, repo.NewProvider(pool), nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	store := resources.NewMemoryResourceStore()
	state.SetResourceStore(store)
	ginmode.EnsureGinTestMode()
	srv, err := server.NewServer(ctx, tempDir, projFile, "")
	require.NoError(t, err)
	r := gin.New()
	r.Use(gin.Recovery())
	authFactory := authuc.NewFactory(state.Store.NewAuthRepo())
	authManager := authmw.NewManager(authFactory, cfg)
	r.Use(authManager.Middleware())
	r.Use(lgmiddleware.Middleware(ctx))
	r.Use(appstate.StateMiddleware(state))
	r.Use(serverrouter.ErrorHandler())
	require.NoError(t, server.RegisterRoutes(ctx, r, state, srv))
	return &ServerHarness{
		Engine:        r,
		State:         state,
		Ctx:           ctx,
		Config:        cfg,
		ResourceStore: store,
		Project:       proj,
		Server:        srv,
		DB:            pool,
	}
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
