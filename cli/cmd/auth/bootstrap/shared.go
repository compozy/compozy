package bootstrap

import (
	"context"
	"fmt"
	"regexp"

	"github.com/compozy/compozy/engine/auth/bootstrap"
	authrepo "github.com/compozy/compozy/engine/auth/infra/postgres"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/store"
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
	cfg := config.Get()

	dbCfg := &store.Config{
		ConnString: cfg.Database.ConnString,
		Host:       cfg.Database.Host,
		Port:       cfg.Database.Port,
		User:       cfg.Database.User,
		Password:   cfg.Database.Password,
		DBName:     cfg.Database.DBName,
		SSLMode:    cfg.Database.SSLMode,
	}

	db, err := store.NewDB(ctx, dbCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	repo := authrepo.NewRepository(db)
	factory := authuc.NewFactory(repo)
	service := bootstrap.NewService(factory)

	cleanup := func() {
		if err := db.Close(ctx); err != nil {
			logger.FromContext(ctx).Error("Failed to close database", "error", err)
		}
	}

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
