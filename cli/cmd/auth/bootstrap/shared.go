package bootstrap

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/compozy/compozy/engine/auth/bootstrap"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/infra/repo"
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
	// Always use Postgres for auth bootstrap
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
	provider := repo.NewProvider(drv.Pool())
	authRepo := provider.NewAuthRepo()
	cleanup := func() {
		if err := drv.Close(ctx); err != nil {
			logger.FromContext(ctx).Error("Failed to close database", "error", err)
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
