package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/sqlite"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

const (
	driverPostgres           = "postgres"
	driverSQLite             = "sqlite"
	fallbackMigrationTimeout = 2 * time.Minute
)

// Provider exposes repository constructors independent of the backing driver.
type Provider struct {
	driver       string
	authRepo     uc.Repository
	taskRepo     task.Repository
	workflowRepo workflow.Repository
}

// NewProvider creates repository providers for the configured database driver.
func NewProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("repo: database config is required")
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = driverPostgres
	}
	switch driver {
	case driverPostgres:
		return newPostgresProvider(ctx, cfg)
	case driverSQLite:
		return newSQLiteProvider(ctx, cfg)
	default:
		return nil, nil, fmt.Errorf("repo: unsupported database driver %q", driver)
	}
}

func newPostgresProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
	pgCfg := &postgres.Config{
		ConnString:         cfg.ConnString,
		Host:               cfg.Host,
		Port:               cfg.Port,
		User:               cfg.User,
		Password:           cfg.Password,
		DBName:             cfg.DBName,
		SSLMode:            cfg.SSLMode,
		MaxOpenConns:       cfg.MaxOpenConns,
		MaxIdleConns:       cfg.MaxIdleConns,
		ConnMaxLifetime:    cfg.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.ConnMaxIdleTime,
		PingTimeout:        cfg.PingTimeout,
		HealthCheckTimeout: cfg.HealthCheckTimeout,
		HealthCheckPeriod:  cfg.HealthCheckPeriod,
		ConnectTimeout:     cfg.ConnectTimeout,
	}
	store, err := postgres.NewStore(ctx, pgCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("repo: postgres store: %w", err)
	}
	if cfg.AutoMigrate {
		timeout := cfg.MigrationTimeout
		if timeout <= 0 {
			timeout = fallbackMigrationTimeout
		}
		mctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()
		if err := postgres.ApplyMigrationsWithLock(mctx, postgres.DSNFor(pgCfg)); err != nil {
			_ = store.Close(context.WithoutCancel(ctx))
			return nil, nil, fmt.Errorf("repo: postgres migrations: %w", err)
		}
	}
	cleanup := func() {
		_ = store.Close(context.WithoutCancel(ctx))
	}
	provider := &Provider{
		driver:       driverPostgres,
		authRepo:     postgres.NewAuthRepo(store.Pool()),
		taskRepo:     postgres.NewTaskRepo(store.Pool()),
		workflowRepo: postgres.NewWorkflowRepo(store.Pool()),
	}
	return provider, cleanup, nil
}

func newSQLiteProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
	sqliteCfg := &sqlite.Config{
		Path:            cfg.Path,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		BusyTimeout:     cfg.BusyTimeout,
	}
	store, err := sqlite.NewStore(ctx, sqliteCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("repo: sqlite store: %w", err)
	}
	if cfg.AutoMigrate {
		timeout := cfg.MigrationTimeout
		if timeout <= 0 {
			timeout = fallbackMigrationTimeout
		}
		mctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()
		if err := sqlite.ApplyMigrations(mctx, cfg.Path); err != nil {
			_ = store.Close(context.WithoutCancel(ctx))
			return nil, nil, fmt.Errorf("repo: sqlite migrations: %w", err)
		}
	}
	cleanup := func() {
		_ = store.Close(context.WithoutCancel(ctx))
	}
	provider := &Provider{
		driver:       driverSQLite,
		authRepo:     sqlite.NewAuthRepo(store.DB()),
		taskRepo:     sqlite.NewTaskRepo(store.DB()),
		workflowRepo: sqlite.NewWorkflowRepo(store.DB()),
	}
	return provider, cleanup, nil
}

// Driver returns the active database driver identifier.
func (p *Provider) Driver() string {
	return p.driver
}

// NewAuthRepo exposes the configured authentication repository.
func (p *Provider) NewAuthRepo() uc.Repository {
	return p.authRepo
}

// NewTaskRepo exposes the configured task repository.
func (p *Provider) NewTaskRepo() task.Repository {
	return p.taskRepo
}

// NewWorkflowRepo exposes the configured workflow repository.
func (p *Provider) NewWorkflowRepo() workflow.Repository {
	return p.workflowRepo
}
