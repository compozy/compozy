package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	DB *DB
}

func SetupStore(ctx context.Context, storeConfig *Config) (*Store, error) {
	if storeConfig == nil {
		return nil, fmt.Errorf("store configuration is required")
	}
	db, err := NewDB(ctx, storeConfig)
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

// SetupStoreWithConfig creates a store using the provided app configuration
func SetupStoreWithConfig(ctx context.Context) (*Store, error) {
	cfg := config.Get()
	// Debug logging
	log := logger.FromContext(ctx)
	log.Debug("Database configuration loaded",
		"has_password", cfg.Database.Password != "",
		"host", cfg.Database.Host)

	storeConfig := &Config{
		ConnString: cfg.Database.ConnString,
		Host:       cfg.Database.Host,
		Port:       cfg.Database.Port,
		User:       cfg.Database.User,
		Password:   cfg.Database.Password,
		DBName:     cfg.Database.DBName,
		SSLMode:    cfg.Database.SSLMode,
	}
	log.Debug("Database connection configured",
		"host", storeConfig.Host,
		"port", storeConfig.Port,
		"user", storeConfig.User,
		"dbname", storeConfig.DBName,
		"has_password", storeConfig.Password != "",
		"ssl_mode", storeConfig.SSLMode)

	db, err := NewDB(ctx, storeConfig)
	if err != nil {
		return nil, err
	}

	// Run migrations if auto-migration is enabled
	if cfg.Database.AutoMigrate {
		log.Info("Running database migrations...")

		// Convert pgxpool to sql.DB using stdlib
		sqlDB := stdlib.OpenDBFromPool(db.Pool())
		defer sqlDB.Close()

		if err := runEmbeddedMigrations(ctx, sqlDB); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Info("Database migrations completed successfully")
	}

	return &Store{DB: db}, nil
}

// NewTaskRepo creates a new TaskRepo.
func (s *Store) NewTaskRepo() *TaskRepo {
	return &TaskRepo{db: s.DB.Pool()}
}

func (s *Store) NewWorkflowRepo() *WorkflowRepo {
	return &WorkflowRepo{db: s.DB.Pool(), taskRepo: s.NewTaskRepo()}
}

// NewAuthRepo creates a new AuthRepo.
func (s *Store) NewAuthRepo() *AuthRepo {
	return NewAuthRepo(s.DB.Pool())
}
