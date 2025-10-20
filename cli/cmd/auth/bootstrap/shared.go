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

// defaultAuthBootstrapDBTimeout is the bounded timeout for bootstrap DB operations.
const defaultAuthBootstrapDBTimeout = 10 * time.Second

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
	// Apply a bounded timeout for the DB connect.
	dbCtx, cancel := context.WithTimeout(ctx, defaultAuthBootstrapDBTimeout)
	defer cancel()
	// NOTE: Auth bootstrap always uses Postgres to enforce consistent credential handling.
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
		closeCtx, cancelClose := context.WithTimeout(context.WithoutCancel(ctx), defaultAuthBootstrapDBTimeout)
		defer cancelClose()
		if err := drv.Close(closeCtx); err != nil {
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
