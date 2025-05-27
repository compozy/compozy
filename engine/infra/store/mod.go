package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/engine/core"
	db "github.com/compozy/compozy/engine/infra/store/sqlc"
	"github.com/compozy/compozy/pkg/logger"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file" // Required for file:// migration source URLs
	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	queries *db.Queries
	dbPath  string
}

// findProjectRoot finds the project root by looking for go.mod file
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found in any parent directory")
}

func NewStore(dbFilePath string) (*Store, error) {
	dir := filepath.Dir(dbFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	// Configure SQLite connection string for concurrent access
	// Enable WAL mode, foreign keys, and set timeouts
	connectionString := fmt.Sprintf(
		"%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=true&_busy_timeout=5000",
		dbFilePath,
	)

	dbConn, err := sql.Open("sqlite", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database at %s: %w", dbFilePath, err)
	}

	// Configure connection pool for concurrent access
	dbConn.SetMaxOpenConns(25)                 // Allow up to 25 concurrent connections
	dbConn.SetMaxIdleConns(5)                  // Keep 5 idle connections
	dbConn.SetConnMaxLifetime(5 * time.Minute) // Recycle connections every 5 minutes

	if err = dbConn.Ping(); err != nil {
		dbConn.Close()
		return nil, fmt.Errorf("failed to ping SQLite database at %s: %w", dbFilePath, err)
	}

	// Enable WAL mode explicitly (in case connection string doesn't work)
	if _, err := dbConn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		dbConn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set additional pragmas for better concurrency
	pragmas := []string{
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA cache_size=1000;",
		"PRAGMA foreign_keys=true;",
		"PRAGMA temp_store=memory;",
		"PRAGMA busy_timeout=5000;",
	}

	for _, pragma := range pragmas {
		if _, err := dbConn.Exec(pragma); err != nil {
			dbConn.Close()
			return nil, fmt.Errorf("failed to execute pragma %s: %w", pragma, err)
		}
	}

	queries := db.New(dbConn)
	return &Store{db: dbConn, queries: queries, dbPath: dbFilePath}, nil
}

func (s *Store) Setup() error {
	// Find project root and construct absolute migration path
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	migrationsPath := filepath.Join(projectRoot, "engine", "infra", "store", "migrations")
	migrationsSourceURL := "file://" + migrationsPath
	logger.Debug("Applying database migrations...", "source", migrationsSourceURL)
	if err := s.MigrateDB(migrationsSourceURL); err != nil {
		logger.Error("Failed to apply database migrations", "error", err)
		return fmt.Errorf("failed to apply database migrations: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) MigrateDB(migrationsSourceURL string) error {
	if s.dbPath == "" {
		return fmt.Errorf("dbPath is not set in Store, cannot run migrations")
	}
	if s.db == nil {
		return fmt.Errorf("database connection (s.db) is nil, cannot run migrations")
	}

	driver, err := sqlite.WithInstance(s.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create sqlite migrate driver instance: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsSourceURL,
		"sqlite",
		driver)
	if err != nil {
		return fmt.Errorf(
			"failed to create migrate instance (migrationsSourceURL: %s, dbPath: %s): %w",
			migrationsSourceURL,
			s.dbPath,
			err,
		)
	}

	currentVersion, dirty, errVersion := m.Version()
	if errVersion != nil && errVersion != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", errVersion)
	}

	if dirty {
		logger.Error(
			"ERROR: Database is in a dirty migration state. Version: %d. "+
				"Please resolve manually (e.g., using 'make migrate-force version=%d'). "+
				"Startup aborted.\n",
			currentVersion,
			currentVersion,
		)
		return fmt.Errorf("database dirty (version %d)", currentVersion)
	}

	errUp := m.Up()
	switch errUp {
	case nil:
		finalVersion, _, errFinalVersion := m.Version()
		if errFinalVersion != nil {
			return fmt.Errorf("failed to get final migration version: %w", errFinalVersion)
		}
		if finalVersion != currentVersion || (errVersion == migrate.ErrNilVersion && finalVersion != 0) {
			fmt.Printf("Database migrations applied successfully. Current version: %d\n", finalVersion)
		} else {
			fmt.Printf("No new database migrations to apply. Database is up-to-date at version %d.\n", finalVersion)
		}
	case migrate.ErrNoChange:
		fmt.Printf("No database migrations to apply. Database is up-to-date at version %d.\n", currentVersion)
	default:
		return fmt.Errorf("failed to apply migrations: %w", errUp)
	}

	return nil
}

func (s *Store) UpsertJSON(ctx context.Context, key []byte, value any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			// Log rollback error but don't override the main error
			fmt.Printf("Warning: failed to rollback transaction: %v\n", rollbackErr)
		}
	}()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	metadata, err := extractMetadata(value)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}
	err = s.queries.WithTx(tx).UpsertExecution(ctx, db.UpsertExecutionParams{
		ComponentType:  metadata.ComponentType,
		WorkflowID:     metadata.WorkflowID,
		WorkflowExecID: metadata.WorkflowExecID,
		TaskID:         metadata.TaskID,
		TaskExecID:     metadata.TaskExecID,
		AgentID:        metadata.AgentID,
		AgentExecID:    metadata.AgentExecID,
		ToolID:         metadata.ToolID,
		ToolExecID:     metadata.ToolExecID,
		Key:            string(key),
		Status:         metadata.Status,
		Data:           data,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert execution: %w", err)
	}
	return tx.Commit()
}

func (s *Store) GetJSON(ctx context.Context, key []byte) (any, error) {
	exec, err := s.queries.GetExecutionByKey(ctx, string(key))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}
	var result any
	data := exec.Data.Bytes()
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return result, nil
}

func (s *Store) DeleteJSON(ctx context.Context, key []byte) error {
	err := s.queries.DeleteExecution(ctx, string(key))
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to delete execution: %w", err)
	}
	return nil
}

type ExtractedMetadata struct {
	Status         core.StatusType
	WorkflowID     string
	WorkflowExecID core.ID
	ComponentType  core.ComponentType
	TaskID         sql.NullString
	TaskExecID     core.ID
	AgentID        sql.NullString
	AgentExecID    core.ID
	ToolID         sql.NullString
	ToolExecID     core.ID
}

func extractMetadata(value any) (*ExtractedMetadata, error) {
	switch v := value.(type) {
	case *workflow.Execution:
		return &ExtractedMetadata{
			ComponentType:  core.ComponentWorkflow,
			Status:         v.Status,
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: v.WorkflowExecID,
		}, nil
	case *agent.Execution:
		return &ExtractedMetadata{
			ComponentType:  core.ComponentAgent,
			Status:         v.Status,
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: v.WorkflowExecID,
			TaskID:         sql.NullString{String: v.TaskID, Valid: true},
			TaskExecID:     v.TaskExecID,
			AgentID:        sql.NullString{String: v.AgentID, Valid: true},
			AgentExecID:    v.AgentExecID,
		}, nil
	case *task.Execution:
		return &ExtractedMetadata{
			ComponentType:  core.ComponentTask,
			Status:         v.Status,
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: v.WorkflowExecID,
			TaskID:         sql.NullString{String: v.TaskID, Valid: true},
			TaskExecID:     v.TaskExecID,
		}, nil
	case *tool.Execution:
		return &ExtractedMetadata{
			ComponentType:  core.ComponentTool,
			Status:         v.Status,
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: v.WorkflowExecID,
			TaskID:         sql.NullString{String: v.TaskID, Valid: true},
			TaskExecID:     v.TaskExecID,
			ToolID:         sql.NullString{String: v.ToolID, Valid: true},
			ToolExecID:     v.ToolExecID,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", v)
	}
}

func UnmarshalExecutions[T any](data []db.Execution) ([]T, error) {
	results := make([]T, len(data))
	for i := range data {
		result, err := core.UnmarshalExecution[T](data[i].Data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal execution %d: %w", i, err)
		}
		results[i] = result
	}
	return results, nil
}
