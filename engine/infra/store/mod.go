package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
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
func SetupStoreWithConfig(ctx context.Context, appConfig *config.Config) (*Store, error) {
	// Debug logging
	log := logger.FromContext(ctx)
	log.Debug("AppConfig Database Password Debug",
		"raw_value", fmt.Sprintf("%#v", appConfig.Database.Password),
		"value_method", appConfig.Database.Password.Value(),
		"is_empty", appConfig.Database.Password.Value() == "")

	storeConfig := &Config{
		ConnString: appConfig.Database.ConnString,
		Host:       appConfig.Database.Host,
		Port:       appConfig.Database.Port,
		User:       appConfig.Database.User,
		Password:   appConfig.Database.Password.Value(),
		DBName:     appConfig.Database.DBName,
		SSLMode:    appConfig.Database.SSLMode,
	}
	log.Debug("Database config values",
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
	return &Store{DB: db}, nil
}

// NewTaskRepo creates a new TaskRepo.
func (s *Store) NewTaskRepo() *TaskRepo {
	return &TaskRepo{db: s.DB.Pool()}
}

func (s *Store) NewWorkflowRepo() *WorkflowRepo {
	return &WorkflowRepo{db: s.DB.Pool(), taskRepo: s.NewTaskRepo()}
}
