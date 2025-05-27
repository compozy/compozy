package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	db "github.com/compozy/compozy/engine/infra/store/sqlc"
	"github.com/compozy/compozy/pkg/logger"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	connectionString := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=true&_busy_timeout=5000", dbFilePath)

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
		return fmt.Errorf("failed to create migrate instance (migrationsSourceURL: %s, dbPath: %s): %w", migrationsSourceURL, s.dbPath, err)
	}

	currentVersion, dirty, errVersion := m.Version()
	if errVersion != nil && errVersion != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", errVersion)
	}

	if dirty {
		logger.Error("ERROR: Database is in a dirty migration state. Version: %d. Please resolve manually (e.g., using 'make migrate-force version=%d'). Startup aborted.\n", currentVersion, currentVersion)
		return fmt.Errorf("database dirty (version %d)", currentVersion)
	}

	errUp := m.Up()
	if errUp == nil {
		finalVersion, _, _ := m.Version()
		if finalVersion != currentVersion || (errVersion == migrate.ErrNilVersion && finalVersion != 0) {
			fmt.Printf("Database migrations applied successfully. Current version: %d\n", finalVersion)
		} else {
			fmt.Printf("No new database migrations to apply. Database is up-to-date at version %d.\n", finalVersion)
		}
	} else if errUp == migrate.ErrNoChange {
		fmt.Printf("No database migrations to apply. Database is up-to-date at version %d.\n", currentVersion)
	} else {
		return fmt.Errorf("failed to apply migrations: %w", errUp)
	}

	return nil
}

func (s *Store) UpsertJSON(ctx context.Context, key []byte, value any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
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
	data, ok := exec.Data.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid data type: %T", exec.Data)
	}
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

func (s *Store) GetWorkflowExecutionByExecID(ctx context.Context, workflowExecID string) (*db.Execution, error) {
	exec, err := s.queries.GetWorkflowExecutionByExecID(ctx, workflowExecID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	return &exec, nil
}

func (s *Store) ListWorkflowExecutions(ctx context.Context) ([]db.Execution, error) {
	execs, err := s.queries.ListWorkflowExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	return execs, nil
}

func (s *Store) ListWorkflowExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]db.Execution, error) {
	execs, err := s.queries.ListWorkflowExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions by workflow ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListWorkflowExecutionsByStatus(ctx context.Context, status string) ([]db.Execution, error) {
	execs, err := s.queries.ListWorkflowExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions by status: %w", err)
	}
	return execs, nil
}

func (s *Store) GetTaskExecutionByExecID(ctx context.Context, taskExecID string) (*db.Execution, error) {
	execID := sql.NullString{String: taskExecID, Valid: true}
	exec, err := s.queries.GetTaskExecutionByExecID(ctx, execID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task execution: %w", err)
	}
	return &exec, nil
}

func (s *Store) ListTaskExecutionsByStatus(ctx context.Context, status string) ([]db.Execution, error) {
	execs, err := s.queries.ListTaskExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by status: %w", err)
	}
	return execs, nil
}

func (s *Store) ListTaskExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]db.Execution, error) {
	execs, err := s.queries.ListTaskExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by workflow ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListTaskExecutionsByWorkflowExecID(ctx context.Context, workflowExecID string) ([]db.Execution, error) {
	execs, err := s.queries.ListTaskExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by workflow exec ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListTaskExecutionsByTaskID(ctx context.Context, taskID string) ([]db.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := s.queries.ListTaskExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by task ID: %w", err)
	}
	return execs, nil
}

func (s *Store) GetAgentExecutionByExecID(ctx context.Context, agentExecID string) (*db.Execution, error) {
	execID := sql.NullString{String: agentExecID, Valid: true}
	exec, err := s.queries.GetAgentExecutionByExecID(ctx, execID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent execution: %w", err)
	}
	return &exec, nil
}

func (s *Store) ListAgentExecutionsByStatus(ctx context.Context, status string) ([]db.Execution, error) {
	execs, err := s.queries.ListAgentExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by status: %w", err)
	}
	return execs, nil
}

func (s *Store) ListAgentExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]db.Execution, error) {
	execs, err := s.queries.ListAgentExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by workflow ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListAgentExecutionsByWorkflowExecID(ctx context.Context, workflowExecID string) ([]db.Execution, error) {
	execs, err := s.queries.ListAgentExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by workflow exec ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListAgentExecutionsByTaskID(ctx context.Context, taskID string) ([]db.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := s.queries.ListAgentExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by task ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListAgentExecutionsByTaskExecID(ctx context.Context, taskExecID string) ([]db.Execution, error) {
	execID := sql.NullString{String: taskExecID, Valid: true}
	execs, err := s.queries.ListAgentExecutionsByTaskExecID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by task exec ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListAgentExecutionsByAgentID(ctx context.Context, agentID string) ([]db.Execution, error) {
	execID := sql.NullString{String: agentID, Valid: true}
	execs, err := s.queries.ListAgentExecutionsByAgentID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by agent ID: %w", err)
	}
	return execs, nil
}

func (s *Store) GetToolExecutionByExecID(ctx context.Context, toolExecID string) (*db.Execution, error) {
	execID := sql.NullString{String: toolExecID, Valid: true}
	exec, err := s.queries.GetToolExecutionByExecID(ctx, execID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tool execution: %w", err)
	}
	return &exec, nil
}

func (s *Store) ListToolExecutionsByStatus(ctx context.Context, status string) ([]db.Execution, error) {
	execs, err := s.queries.ListToolExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by status: %w", err)
	}
	return execs, nil
}

func (s *Store) ListToolExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]db.Execution, error) {
	execs, err := s.queries.ListToolExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by workflow ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListToolExecutionsByWorkflowExecID(ctx context.Context, workflowExecID string) ([]db.Execution, error) {
	execs, err := s.queries.ListToolExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by workflow exec ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListToolExecutionsByTaskID(ctx context.Context, taskID string) ([]db.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := s.queries.ListToolExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by task ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListToolExecutionsByTaskExecID(ctx context.Context, taskExecID string) ([]db.Execution, error) {
	execID := sql.NullString{String: taskExecID, Valid: true}
	execs, err := s.queries.ListToolExecutionsByTaskExecID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by task exec ID: %w", err)
	}
	return execs, nil
}

func (s *Store) ListToolExecutionsByToolID(ctx context.Context, toolID string) ([]db.Execution, error) {
	execID := sql.NullString{String: toolID, Valid: true}
	execs, err := s.queries.ListToolExecutionsByToolID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by tool ID: %w", err)
	}
	return execs, nil
}

type ExtractedMetadata struct {
	Status         string
	WorkflowID     string
	WorkflowExecID string
	ComponentType  string
	TaskID         sql.NullString
	TaskExecID     sql.NullString
	AgentID        sql.NullString
	AgentExecID    sql.NullString
	ToolID         sql.NullString
	ToolExecID     sql.NullString
}

func extractMetadata(value any) (*ExtractedMetadata, error) {
	switch v := value.(type) {
	case *workflow.Execution:
		return &ExtractedMetadata{
			ComponentType:  "workflow",
			Status:         string(v.Status),
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: string(v.WorkflowExecID),
		}, nil
	case *agent.Execution:
		return &ExtractedMetadata{
			ComponentType:  "agent",
			Status:         string(v.Status),
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: string(v.WorkflowExecID),
			TaskID:         sql.NullString{String: string(v.TaskID), Valid: true},
			TaskExecID:     sql.NullString{String: string(v.TaskExecID), Valid: true},
			AgentID:        sql.NullString{String: string(v.AgentID), Valid: true},
			AgentExecID:    sql.NullString{String: string(v.AgentExecID), Valid: true},
		}, nil
	case *task.Execution:
		return &ExtractedMetadata{
			ComponentType:  "task",
			Status:         string(v.Status),
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: string(v.WorkflowExecID),
			TaskID:         sql.NullString{String: string(v.TaskID), Valid: true},
			TaskExecID:     sql.NullString{String: string(v.TaskExecID), Valid: true},
		}, nil
	case *tool.Execution:
		return &ExtractedMetadata{
			ComponentType:  "tool",
			Status:         string(v.Status),
			WorkflowID:     v.WorkflowID,
			WorkflowExecID: string(v.WorkflowExecID),
			ToolID:         sql.NullString{String: string(v.ToolID), Valid: true},
			ToolExecID:     sql.NullString{String: string(v.ToolExecID), Valid: true},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", v)
	}
}

func unmarshalExecution[T any](exec db.Execution) (T, error) {
	var result T
	data, ok := exec.Data.([]byte)
	if !ok {
		return result, fmt.Errorf("invalid data type: %T", exec.Data)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return result, nil
}

func unmarshalExecutions[T any](execs []db.Execution) ([]T, error) {
	results := make([]T, len(execs))
	for i, exec := range execs {
		result, err := unmarshalExecution[T](exec)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal execution %d: %w", i, err)
		}
		results[i] = result
	}
	return results, nil
}
