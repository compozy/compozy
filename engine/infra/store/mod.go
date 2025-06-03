package store

import (
	"context"
	"os"
)

type Store struct {
	DB *DB
}

func SetupStore(ctx context.Context) (*Store, error) {
	dbConfig := Config{
		ConnString: os.Getenv("DB_CONN_STRING"),
		Host:       os.Getenv("DB_HOST"),
		Port:       os.Getenv("DB_PORT"),
		User:       os.Getenv("DB_USER"),
		Password:   os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		SSLMode:    os.Getenv("DB_SSL_MODE"),
	}
	db, err := NewDB(ctx, &dbConfig)
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
