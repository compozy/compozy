# `store` – _PostgreSQL-based data layer for Compozy workflow orchestration_

> **Database abstraction layer providing persistent storage for workflows and tasks with PostgreSQL-optimized repositories and connection management.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Store Setup](#store-setup)
  - [Database Connection](#database-connection)
  - [Task Repository](#task-repository)
  - [Workflow Repository](#workflow-repository)
  - [Transactions](#transactions)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
  - [Basic Task Operations](#basic-task-operations)
  - [Workflow Management](#workflow-management)
  - [Transaction Usage](#transaction-usage)
- [📚 API Reference](#-api-reference)
  - [Store](#store)
  - [Config](#config)
  - [DB](#db)
  - [TaskRepo](#taskrepo)
  - [WorkflowRepo](#workflowrepo)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `store` package provides a clean, production-ready data access layer for Compozy's workflow orchestration engine. It implements the repository pattern with PostgreSQL-specific optimizations for storing and retrieving workflow execution states, task states, and their relationships.

Key capabilities include:

- **Workflow State Management**: Store and track workflow execution lifecycle
- **Task State Management**: Persist task execution states with hierarchical relationships
- **Transactional Operations**: Ensure data consistency across complex operations
- **Connection Pooling**: Efficient PostgreSQL connection management with pgx
- **Type Safety**: Strongly typed interfaces with generic JSON handling

---

## 💡 Motivation

- **Workflow Persistence**: Compozy needs reliable storage for long-running workflow executions that may span hours or days
- **Task Hierarchy**: Complex workflows create parent-child task relationships that require efficient querying and updates
- **Concurrency Safety**: Multiple agents executing tasks simultaneously need proper isolation and locking mechanisms
- **Performance**: High-throughput task execution requires optimized database operations and connection pooling

---

## ⚡ Design Highlights

- **Repository Pattern**: Clean separation between data access and business logic with mockable interfaces
- **PostgreSQL Optimization**: Leverages PostgreSQL-specific features like JSONB, CTEs, and row-level locking
- **Connection Management**: pgx-based connection pooling with health checks and automatic reconnection
- **Type-Safe JSON**: Generic helper functions for JSONB serialization with proper nil handling
- **Transaction Support**: Comprehensive transaction management with proper rollback and error handling
- **Concurrent Safety**: Row-level locking and atomic operations for parallel task execution

---

## 🚀 Getting Started

### Prerequisites

- PostgreSQL 12+ database
- Go 1.21+ with generics support
- Database migrations applied (see `migrations/` directory)

### Quick Setup

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/infra/store"
    "github.com/compozy/compozy/pkg/config"
)

func main() {
    ctx := context.Background()

    // Option 1: Using app configuration
    appConfig := &config.Config{
        Database: config.DatabaseConfig{
            Host:     "localhost",
            Port:     "5432",
            User:     "compozy",
            Password: config.SecretValue("password"),
            DBName:   "compozy_dev",
            SSLMode:  "disable",
        },
    }

    store, err := store.SetupStoreWithConfig(ctx, appConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Create repository instances
    taskRepo := store.NewTaskRepo()
    workflowRepo := store.NewWorkflowRepo()

    // Start using repositories...
}
```

---

## 📖 Usage

### Store Setup

The `Store` acts as a factory for repository instances:

```go
// Using direct configuration
storeConfig := &store.Config{
    Host:     "localhost",
    Port:     "5432",
    User:     "compozy",
    Password: "password",
    DBName:   "compozy_dev",
    SSLMode:  "disable",
}

store, err := store.SetupStore(ctx, storeConfig)
if err != nil {
    return err
}
defer store.DB.Close(ctx)
```

### Database Connection

The `DB` type provides connection pooling and implements the `DBInterface`:

```go
db, err := store.NewDB(ctx, config)
if err != nil {
    return err
}

// Direct database operations
result, err := db.Exec(ctx, "UPDATE tasks SET status = $1 WHERE id = $2", "completed", taskID)
if err != nil {
    return err
}
```

### Task Repository

The `TaskRepo` handles all task-related database operations:

```go
taskRepo := store.NewTaskRepo()

// Create a task state
taskState := &task.State{
    TaskExecID:      taskExecID,
    TaskID:          "my-task",
    WorkflowExecID:  workflowExecID,
    WorkflowID:      "my-workflow",
    Status:          core.StatusRunning,
    Input:           &core.Input{"key": "value"},
}

// Save task state
err = taskRepo.UpsertState(ctx, taskState)
if err != nil {
    return err
}

// Retrieve task state
retrievedState, err := taskRepo.GetState(ctx, taskExecID)
if err != nil {
    return err
}
```

### Workflow Repository

The `WorkflowRepo` manages workflow execution data:

```go
workflowRepo := store.NewWorkflowRepo()

// Create workflow execution
workflowExec := &workflow.Execution{
    ExecID:     execID,
    WorkflowID: "my-workflow",
    Status:     core.StatusRunning,
    Input:      &core.Input{"param": "value"},
}

err = workflowRepo.UpsertExecution(ctx, workflowExec)
if err != nil {
    return err
}
```

### Transactions

Use transactions for operations that require consistency:

```go
err = taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
    // Get task with lock
    taskState, err := taskRepo.GetStateForUpdate(ctx, tx, taskExecID)
    if err != nil {
        return err
    }

    // Update task status
    taskState.Status = core.StatusCompleted
    taskState.Output = &core.Output{"result": "success"}

    // Save within transaction
    return taskRepo.UpsertStateWithTx(ctx, tx, taskState)
})
```

---

## 🔧 Configuration

### Database Configuration

```go
type Config struct {
    ConnString string // Full connection string (overrides individual fields)
    Host       string // Database host
    Port       string // Database port
    User       string // Database user
    Password   string // Database password
    DBName     string // Database name
    SSLMode    string // SSL mode (disable, require, verify-ca, verify-full)
}
```

### Connection Pool Settings

The database connection pool is configured with sensible defaults:

```go
config.MaxConns = 20                        // Maximum connections
config.MinConns = 2                         // Minimum connections
config.HealthCheckPeriod = 30 * time.Second // Health check frequency
config.ConnectTimeout = 5 * time.Second     // Connection timeout
```

---

## 🎨 Examples

### Basic Task Operations

```go
func ExampleTaskOperations() {
    ctx := context.Background()
    store, _ := setupStore(ctx)
    taskRepo := store.NewTaskRepo()

    // Create task execution ID
    taskExecID, _ := core.NewID()

    // Create and save task state
    taskState := &task.State{
        TaskExecID:      taskExecID,
        TaskID:          "data-processor",
        WorkflowExecID:  workflowExecID,
        WorkflowID:      "etl-pipeline",
        Status:          core.StatusRunning,
        ExecutionType:   core.ExecutionTypeTask,
        Input:           &core.Input{"source": "database"},
    }

    err := taskRepo.UpsertState(ctx, taskState)
    if err != nil {
        log.Fatal(err)
    }

    // Update task with results
    taskState.Status = core.StatusCompleted
    taskState.Output = &core.Output{
        "processed_rows": 1000,
        "duration_ms":    2500,
    }

    err = taskRepo.UpsertState(ctx, taskState)
    if err != nil {
        log.Fatal(err)
    }

    // Query tasks by status
    completedTasks, err := taskRepo.ListTasksByStatus(ctx, workflowExecID, core.StatusCompleted)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Completed tasks: %d\n", len(completedTasks))
}
```

### Workflow Management

```go
func ExampleWorkflowManagement() {
    ctx := context.Background()
    store, _ := setupStore(ctx)
    workflowRepo := store.NewWorkflowRepo()

    // Create workflow execution
    execID, _ := core.NewID()
    workflowExec := &workflow.Execution{
        ExecID:     execID,
        WorkflowID: "data-pipeline",
        Status:     core.StatusRunning,
        Input:      &core.Input{"environment": "production"},
    }

    err := workflowRepo.UpsertExecution(ctx, workflowExec)
    if err != nil {
        log.Fatal(err)
    }

    // Get workflow with related tasks
    execution, err := workflowRepo.GetExecution(ctx, execID)
    if err != nil {
        log.Fatal(err)
    }

    // Update workflow status
    execution.Status = core.StatusCompleted
    execution.Output = &core.Output{"total_processed": 5000}

    err = workflowRepo.UpsertExecution(ctx, execution)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Transaction Usage

```go
func ExampleTransactionUsage() {
    ctx := context.Background()
    store, _ := setupStore(ctx)
    taskRepo := store.NewTaskRepo()

    parentTaskID, _ := core.NewID()

    // Create multiple child tasks atomically
    childStates := []*task.State{
        {
            TaskExecID:      mustNewID(),
            TaskID:          "child-1",
            WorkflowExecID:  workflowExecID,
            WorkflowID:      "parent-workflow",
            Status:          core.StatusPending,
            ExecutionType:   core.ExecutionTypeTask,
            ParentStateID:   &parentTaskID,
        },
        {
            TaskExecID:      mustNewID(),
            TaskID:          "child-2",
            WorkflowExecID:  workflowExecID,
            WorkflowID:      "parent-workflow",
            Status:          core.StatusPending,
            ExecutionType:   core.ExecutionTypeTask,
            ParentStateID:   &parentTaskID,
        },
    }

    err := taskRepo.CreateChildStatesInTransaction(ctx, parentTaskID, childStates)
    if err != nil {
        log.Fatal(err)
    }

    // Get progress information
    progress, err := taskRepo.GetProgressInfo(ctx, parentTaskID)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Total children: %d, Pending: %d\n",
        progress.TotalChildren, progress.PendingCount)
}
```

---

## 📚 API Reference

### Store

```go
type Store struct {
    DB *DB
}

func SetupStore(ctx context.Context, storeConfig *Config) (*Store, error)
func SetupStoreWithConfig(ctx context.Context, appConfig *config.Config) (*Store, error)
func (s *Store) NewTaskRepo() *TaskRepo
func (s *Store) NewWorkflowRepo() *WorkflowRepo
```

### Config

```go
type Config struct {
    ConnString string // Full PostgreSQL connection string
    Host       string // Database host
    Port       string // Database port
    User       string // Database username
    Password   string // Database password
    DBName     string // Database name
    SSLMode    string // SSL mode configuration
}
```

### DB

```go
type DB struct {
    // Contains pgxpool.Pool for connection management
}

func NewDB(ctx context.Context, cfg *Config) (*DB, error)
func (db *DB) Close(ctx context.Context) error
func (db *DB) Pool() *pgxpool.Pool
func (db *DB) WithTx(ctx context.Context, fn func(pgx.Tx) error) error

// DBInterface methods
func (db *DB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
func (db *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
func (db *DB) Begin(ctx context.Context) (pgx.Tx, error)
```

### TaskRepo

```go
type TaskRepo struct {
    // Task state repository with full CRUD operations
}

func NewTaskRepo(db DBInterface) *TaskRepo

// Core operations
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error)
func (r *TaskRepo) GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error)
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error)

// Workflow-specific queries
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error)
func (r *TaskRepo) ListTasksByStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) ([]*task.State, error)
func (r *TaskRepo) ListTasksByAgent(ctx context.Context, workflowExecID core.ID, agentID string) ([]*task.State, error)
func (r *TaskRepo) ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*task.State, error)

// Hierarchy operations
func (r *TaskRepo) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error)
func (r *TaskRepo) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error)
func (r *TaskRepo) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error)
func (r *TaskRepo) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error)
func (r *TaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error)

// Transaction operations
func (r *TaskRepo) WithTx(ctx context.Context, fn func(pgx.Tx) error) error
func (r *TaskRepo) CreateChildStatesInTransaction(ctx context.Context, parentStateID core.ID, childStates []*task.State) error
func (r *TaskRepo) UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error
```

### WorkflowRepo

```go
type WorkflowRepo struct {
    // Workflow execution repository
}

func NewWorkflowRepo(db DBInterface) *WorkflowRepo

// Core workflow operations
func (r *WorkflowRepo) UpsertExecution(ctx context.Context, execution *workflow.Execution) error
func (r *WorkflowRepo) GetExecution(ctx context.Context, execID core.ID) (*workflow.Execution, error)
func (r *WorkflowRepo) ListExecutions(ctx context.Context, filter *workflow.ExecutionFilter) ([]*workflow.Execution, error)
```

### JSON Helpers

```go
func ToJSONB(v any) ([]byte, error)
func FromJSONB[T any](src []byte, dst **T) error
```

---

## 🧪 Testing

The package includes comprehensive mocks for testing:

```go
// Use mock repositories in tests
mockTaskRepo := &store.MockTaskRepo{}
mockWorkflowRepo := &store.MockWorkflowRepo{}

// Set up expectations
mockTaskRepo.On("UpsertState", mock.Anything, mock.AnythingOfType("*task.State")).
    Return(nil)

mockTaskRepo.On("GetState", mock.Anything, taskExecID).
    Return(expectedState, nil)

// Test your business logic
result, err := myService.ProcessTask(ctx, taskExecID)
assert.NoError(t, err)
assert.Equal(t, expectedResult, result)

// Verify expectations
mockTaskRepo.AssertExpectations(t)
```

### Running Tests

```bash
# Run all tests
go test ./engine/infra/store/...

# Run tests with coverage
go test -cover ./engine/infra/store/...

# Run specific test
go test -v -run TestTaskRepo_UpsertState ./engine/infra/store/
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../../CONTRIBUTING.md) for development guidelines.

---

## 📄 License

MIT License - see [LICENSE](../../../LICENSE) for details.
