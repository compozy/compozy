package bootstrap

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/compozy/compozy/engine/auth/bootstrap"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/pkg/config"
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
	dbCfg := cfg.Database
	dbCfg.Driver = "postgres"
	provider, providerCleanup, err := repo.NewProvider(dbCtx, &dbCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	authRepo := provider.NewAuthRepo()
	cleanup := func() {
		providerCleanup()
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
