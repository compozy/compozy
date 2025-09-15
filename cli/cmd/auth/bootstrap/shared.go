package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/compozy/compozy/engine/auth/bootstrap"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/repo"
	sqli "github.com/compozy/compozy/engine/infra/sqlite"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// ServiceFactory interface for dependency injection
type ServiceFactory interface {
	CreateService(ctx context.Context) (*bootstrap.Service, func(), error)
}

// DefaultServiceFactory implements ServiceFactory with database connection
type DefaultServiceFactory struct{}

// CreateService creates a bootstrap service with direct DB access
func (f *DefaultServiceFactory) CreateService(ctx context.Context) (*bootstrap.Service, func(), error) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, nil, fmt.Errorf("config manager not found in context")
	}

	// Add timeout for database connection to prevent indefinite hanging
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		cleanup  func()
		authRepo authuc.Repository
	)

	if cfg.Mode == "standalone" {
		// Use embedded SQLite for standalone auth bootstrap
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		stateDir := filepath.Join(wd, ".compozy", "state")
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create state dir: %w", err)
		}
		dbPath := filepath.Join(stateDir, "compozy.sqlite")
		sq, err := sqli.NewStore(dbCtx, dbPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open sqlite store: %w", err)
		}
		if err := sqli.ApplyMigrations(dbCtx, sq.DB()); err != nil {
			_ = sq.Close(ctx)
			return nil, nil, fmt.Errorf("failed to apply sqlite migrations: %w", err)
		}
		provider := repo.NewProvider("standalone", nil, sq)
		authRepo = provider.NewAuthRepo()
		cleanup = func() { _ = sq.Close(ctx) }
	} else {
		// Distributed/auth bootstrap uses Postgres
		dbCfg := &postgres.Config{
			ConnString: cfg.Database.ConnString,
			Host:       cfg.Database.Host,
			Port:       cfg.Database.Port,
			User:       cfg.Database.User,
			Password:   cfg.Database.Password,
			DBName:     cfg.Database.DBName,
			SSLMode:    cfg.Database.SSLMode,
		}
		drv, err := postgres.NewStore(dbCtx, dbCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		provider := repo.NewProvider("distributed", drv.Pool(), nil)
		authRepo = provider.NewAuthRepo()
		cleanup = func() {
			if err := drv.Close(ctx); err != nil {
				logger.FromContext(ctx).Error("Failed to close database", "error", err)
			}
		}
	}

	factory := authuc.NewFactory(authRepo)
	service := bootstrap.NewService(factory)
	return service, cleanup, nil
}

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Basic email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// MaskAPIKey masks an API key for safe logging
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}
