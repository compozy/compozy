# `worker` – _Temporal-Based Workflow Execution Engine_

> **The worker package provides a distributed, fault-tolerant workflow execution engine built on Temporal, enabling scalable orchestration of AI-powered workflows with advanced features like signal handling, event-driven execution, and comprehensive monitoring.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `worker` package is the distributed execution engine that powers Compozy's workflow orchestration. Built on top of Temporal, it provides robust, scalable, and fault-tolerant execution of AI workflows with advanced features like signal handling, event-driven processing, and comprehensive monitoring.

**Key Features:**

- 🔄 **Temporal Integration**: Built on proven Temporal workflow engine for reliability
- 📡 **Event-Driven Architecture**: Signal-based triggers and external event processing
- 🔧 **MCP Integration**: Seamless Model Context Protocol server management
- 📊 **Advanced Monitoring**: Comprehensive metrics, health checks, and observability
- 🛡️ **Fault Tolerance**: Automatic retries, error handling, and recovery mechanisms
- 🚀 **Scalable Execution**: Distributed processing with load balancing
- 💾 **State Management**: Persistent workflow state with Redis caching
- 🔐 **Memory Management**: Secure memory handling with privacy controls

---

## 💡 Motivation

- **Reliable Execution**: Ensure workflows complete successfully even in face of failures
- **Scalable Architecture**: Support horizontal scaling for high-throughput scenarios
- **Event-Driven Processing**: Enable reactive workflows responding to external events
- **Operational Excellence**: Provide comprehensive monitoring and health management
- **Developer Experience**: Simplify complex distributed system patterns

---

## ⚡ Design Highlights

### Temporal-Based Architecture

Built on Temporal's proven workflow engine, providing durable execution, automatic retries, and workflow versioning. Workflows can survive process restarts and system failures.

### Event-Driven Signal Processing

Advanced signal handling system that allows workflows to respond to external events, webhooks, and system signals in real-time while maintaining execution state.

### Distributed Dispatcher System

Each worker instance manages its own dispatcher for handling external events, ensuring no single point of failure and enabling horizontal scaling.

### Comprehensive Monitoring

Built-in monitoring with metrics collection, health checks, and observability features that provide deep insights into workflow execution and system performance.

### Memory and Privacy Management

Sophisticated memory management system with privacy controls, ensuring sensitive data is handled securely throughout the workflow lifecycle.

---

## 🚀 Getting Started

### Basic Worker Setup

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/worker"
    "github.com/compozy/compozy/engine/project"
    "github.com/compozy/compozy/engine/workflow"
)

func main() {
    ctx := context.Background()

    // Load project configuration
    projectConfig, err := project.Load("./compozy.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Load workflow configurations
    workflows, err := workflow.LoadAllFromProject(projectConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Configure Temporal connection
    temporalConfig := &worker.TemporalConfig{
        HostPort:  "localhost:7233",
        Namespace: "default",
        TaskQueue: "compozy-tasks",
    }

    // Create worker configuration
    workerConfig := &worker.Config{
        WorkflowRepo:      func() workflow.Repository { return myWorkflowRepo },
        TaskRepo:          func() task.Repository { return myTaskRepo },
        MonitoringService: monitoringService,
        ResourceRegistry:  resourceRegistry,
        AppConfig:         appConfig,
    }

    // Create and start worker
    w, err := worker.NewWorker(ctx, workerConfig, temporalConfig, projectConfig, workflows)
    if err != nil {
        log.Fatal(err)
    }

    if err := w.Setup(ctx); err != nil {
        log.Fatal(err)
    }

    // Worker is now running and processing workflows
    log.Println("Worker started successfully")
}
```

### Quick Workflow Execution

```go
// Trigger a workflow
workflowInput, err := w.TriggerWorkflow(ctx, "my-workflow", &core.Input{
    "user_id": "12345",
    "action": "process_order",
}, "initial-task")

if err != nil {
    log.Fatal(err)
}

log.Printf("Workflow started: %s", workflowInput.WorkflowExecID)
```

---

## 📖 Usage

### Library

#### Worker Lifecycle Management

```go
// Create worker with full configuration
worker, err := worker.NewWorker(ctx, &worker.Config{
    WorkflowRepo: func() workflow.Repository {
        return myWorkflowRepo
    },
    TaskRepo: func() task.Repository {
        return myTaskRepo
    },
    MonitoringService: monitoringService,
    ResourceRegistry:  resourceRegistry,
    AppConfig:         appConfig,
}, temporalConfig, projectConfig, workflows)

if err != nil {
    return fmt.Errorf("failed to create worker: %w", err)
}

// Setup and start worker
if err := worker.Setup(ctx); err != nil {
    return fmt.Errorf("failed to setup worker: %w", err)
}

// Graceful shutdown
defer worker.Stop(ctx)
```

#### Workflow Execution

```go
// Start workflow with input validation
input := &core.Input{
    "customer_email": "user@example.com",
    "issue_type": "billing",
    "priority": "high",
}

workflowInput, err := worker.TriggerWorkflow(ctx, "support-workflow", input, "analyze-issue")
if err != nil {
    return fmt.Errorf("failed to trigger workflow: %w", err)
}

// Monitor workflow execution
workflowID := workflowInput.WorkflowID
execID := workflowInput.WorkflowExecID
log.Printf("Workflow %s started with execution ID: %s", workflowID, execID)
```

#### Signal Handling

```go
// Send signal to running workflow
signal := worker.EventSignal{
    WorkflowID: "user-registration",
    EventType:  "email_verified",
    Data: map[string]any{
        "user_id": "12345",
        "verified_at": time.Now(),
    },
}

err := worker.SendSignal(ctx, signal)
if err != nil {
    return fmt.Errorf("failed to send signal: %w", err)
}
```

#### Health Monitoring

```go
// Perform health check
if err := worker.HealthCheck(ctx); err != nil {
    log.Printf("Health check failed: %v", err)
    // Handle unhealthy state
}

// Get worker status
client := worker.GetClient()
dispatcherID := worker.GetDispatcherID()
taskQueue := worker.GetTaskQueue()

log.Printf("Worker info - Dispatcher: %s, Queue: %s", dispatcherID, taskQueue)
```

---

## 🔧 Configuration

### Worker Configuration

```go
type Config struct {
    // Repository factories
    WorkflowRepo      func() workflow.Repository
    TaskRepo          func() task.Repository

    // Monitoring and observability
    MonitoringService *monitoring.Service

    // Resource management
    ResourceRegistry  *autoload.ConfigRegistry

    // Application configuration
    AppConfig         *config.Config
}
```

### Temporal Configuration

```go
type TemporalConfig struct {
    HostPort  string  // Temporal server address
    Namespace string  // Temporal namespace
    TaskQueue string  // Task queue name
}
```

### Environment Variables

```bash
# Temporal Configuration
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=compozy-tasks

# Tool Execution
TOOL_EXECUTION_TIMEOUT=60s

# MCP Integration
MCP_PROXY_URL=http://localhost:5001
MCP_PROXY_ADMIN_TOKEN=admin-token-123

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Monitoring
MONITORING_ENABLED=true
METRICS_PORT=5001
```

---

## 🎨 Examples

### Complete Worker Setup

```go
func setupWorker() (*worker.Worker, error) {
    ctx := context.Background()

    // Load project configuration
    projectConfig, err := project.Load("./compozy.yaml")
    if err != nil {
        return nil, fmt.Errorf("failed to load project: %w", err)
    }

    // Load workflows
    workflows, err := workflow.WorkflowsFromProject(projectConfig, evaluator)
    if err != nil {
        return nil, fmt.Errorf("failed to load workflows: %w", err)
    }

    // Setup repositories
    workflowRepo := setupWorkflowRepository()
    taskRepo := setupTaskRepository()

    // Setup monitoring
    monitoringService, err := monitoring.NewService(&monitoring.Config{
        Enabled:     true,
        MetricsPort: 5001,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to setup monitoring: %w", err)
    }

    // Setup resource registry for memory management
    resourceRegistry := autoload.NewConfigRegistry()

    // Create worker
    w, err := worker.NewWorker(ctx, &worker.Config{
        WorkflowRepo:      func() workflow.Repository { return workflowRepo },
        TaskRepo:          func() task.Repository { return taskRepo },
        MonitoringService: monitoringService,
        ResourceRegistry:  resourceRegistry,
        AppConfig:         appConfig,
    }, &worker.TemporalConfig{
        HostPort:  "localhost:7233",
        Namespace: "default",
        TaskQueue: "compozy-tasks",
    }, projectConfig, workflows)

    if err != nil {
        return nil, fmt.Errorf("failed to create worker: %w", err)
    }

    return w, nil
}
```

### Event-Driven Workflow

```go
// Setup workflow with signal triggers
func setupEventDrivenWorkflow() error {
    // Create worker
    w, err := setupWorker()
    if err != nil {
        return err
    }

    // Start worker
    if err := w.Setup(context.Background()); err != nil {
        return err
    }

    // The worker automatically handles signals defined in workflow configs
    // Example workflow config with signal trigger:
    /*
    triggers:
      - type: signal
        name: user_registered
        schema:
          type: object
          properties:
            user_id:
              type: string
            email:
              type: string
              format: email
    */

    return nil
}

// Send event to trigger workflow
func sendUserRegistrationEvent(userID, email string) error {
    signal := worker.EventSignal{
        WorkflowID: "user-onboarding",
        EventType:  "user_registered",
        Data: map[string]any{
            "user_id": userID,
            "email":   email,
        },
    }

    return worker.SendSignal(context.Background(), signal)
}
```

### Batch Processing Workflow

```go
// Setup batch processing with parallel execution
func setupBatchProcessor() error {
    // Workflow configuration for batch processing
    workflowConfig := &workflow.Config{
        ID:          "batch-processor",
        Description: "Process large datasets in parallel",
        Tasks: []task.Config{
            {
                ID:   "split-data",
                Type: "basic",
                Use:  "tool(data-splitter)",
            },
            {
                ID:   "process-batch",
                Type: "parallel",
                Use:  "agent(data-processor)",
                With: map[string]any{
                    "batch_size": 100,
                    "max_workers": 10,
                },
            },
            {
                ID:   "aggregate-results",
                Type: "basic",
                Use:  "tool(result-aggregator)",
            },
        },
    }

    // Worker will handle parallel execution automatically
    return nil
}
```

### Monitoring Integration

```go
// Setup worker with comprehensive monitoring
func setupMonitoredWorker() (*worker.Worker, error) {
    // Create monitoring service
    monitoringService, err := monitoring.NewService(&monitoring.Config{
        Enabled:     true,
        MetricsPort: 5001,
        Tracing: &monitoring.TracingConfig{
            Enabled:  true,
            Endpoint: "http://jaeger:14268/api/traces",
        },
        Metrics: &monitoring.MetricsConfig{
            Enabled: true,
            Prefix:  "compozy_worker",
        },
    })
    if err != nil {
        return nil, err
    }

    // Worker automatically integrates with monitoring
    w, err := worker.NewWorker(ctx, &worker.Config{
        MonitoringService: monitoringService,
        // ... other config
    }, temporalConfig, projectConfig, workflows)

    if err != nil {
        return nil, err
    }

    // Setup health check endpoint
    go func() {
        http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
            if err := worker.HealthCheck(r.Context()); err != nil {
                http.Error(w, err.Error(), http.StatusServiceUnavailable)
                return
            }
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("healthy"))
        })
        log.Fatal(http.ListenAndServe(":8081", nil))
    }()

    return w, nil
}
```

---

## 📚 API Reference

### Core Types

#### `Worker`

Main worker instance that manages workflow execution.

```go
type Worker struct {
    // Temporal client for workflow operations
    client        *Client

    // Worker configuration
    config        *Config

    // Activity implementations
    activities    *Activities

    // Project and workflow configurations
    projectConfig *project.Config
    workflows     []*workflow.Config

    // Resource management
    memoryManager  *memory.Manager
    templateEngine *tplengine.TemplateEngine
}
```

**Key Methods:**

- `Setup(ctx context.Context) error` - Initialize and start worker
- `Stop(ctx context.Context)` - Gracefully shutdown worker
- `TriggerWorkflow(ctx context.Context, workflowID string, input *core.Input, initTaskID string) (*WorkflowInput, error)` - Start workflow execution
- `CancelWorkflow(ctx context.Context, workflowID string, workflowExecID core.ID) error` - Cancel running workflow
- `HealthCheck(ctx context.Context) error` - Perform health check
- `GetClient() client.Client` - Get Temporal client
- `GetDispatcherID() string` - Get unique dispatcher ID
- `GetTaskQueue() string` - Get task queue name

#### `Client`

Temporal client wrapper with additional functionality.

```go
type Client struct {
    client.Client
    config *TemporalConfig
}
```

**Key Methods:**

- `NewWorker(taskQueue string, options *worker.Options) worker.Worker` - Create Temporal worker
- `Config() *TemporalConfig` - Get client configuration
- `Close()` - Close client connection

#### `Activities`

Container for all workflow activities.

```go
type Activities struct {
    // Core workflow activities
    GetWorkflowData    func(context.Context, *GetWorkflowDataInput) (*GetWorkflowDataOutput, error)
    TriggerWorkflow    func(context.Context, *TriggerWorkflowInput) (*TriggerWorkflowOutput, error)

    // Task execution activities
    ExecuteBasicTask     func(context.Context, *ExecuteBasicTaskInput) (*ExecuteBasicTaskOutput, error)
    ExecuteRouterTask    func(context.Context, *ExecuteRouterTaskInput) (*ExecuteRouterTaskOutput, error)
    ExecuteAggregateTask func(context.Context, *ExecuteAggregateTaskInput) (*ExecuteAggregateTaskOutput, error)

    // Signal and event activities
    ExecuteSignalTask func(context.Context, *ExecuteSignalTaskInput) (*ExecuteSignalTaskOutput, error)

    // Memory management activities
    ExecuteMemoryTask func(context.Context, *ExecuteMemoryTaskInput) (*ExecuteMemoryTaskOutput, error)

    // Monitoring activities
    DispatcherHeartbeat func(context.Context, *DispatcherHeartbeatInput) (*DispatcherHeartbeatOutput, error)
}
```

### Functions

#### `NewWorker(ctx context.Context, config *Config, clientConfig *TemporalConfig, projectConfig *project.Config, workflows []*workflow.Config) (*Worker, error)`

Creates a new worker instance.

```go
w, err := worker.NewWorker(ctx, &worker.Config{
    WorkflowRepo:      workflowRepoFactory,
    TaskRepo:          taskRepoFactory,
    MonitoringService: monitoringService,
    ResourceRegistry:  resourceRegistry,
    AppConfig:         appConfig,
}, temporalConfig, projectConfig, workflows)
```

#### `NewClient(ctx context.Context, cfg *TemporalConfig) (*Client, error)`

Creates a new Temporal client.

```go
client, err := worker.NewClient(ctx, &worker.TemporalConfig{
    HostPort:  "localhost:7233",
    Namespace: "default",
    TaskQueue: "compozy-tasks",
})
```

#### `GetTaskQueue(projectName string) string`

Generates task queue name from project name.

```go
taskQueue := worker.GetTaskQueue("my-project")
// Returns: "compozy-my-project"
```

### Workflow Functions

#### `CompozyWorkflow(ctx workflow.Context, input WorkflowInput) (*workflow.State, error)`

Main workflow execution function.

#### `DispatcherWorkflow(ctx workflow.Context, projectName string, serverID string) error`

Dispatcher workflow for handling external events.

### Signal Processing

#### `ProcessEventSignal(ctx workflow.Context, signal EventSignal, signalMap map[string]*CompiledTrigger) bool`

Process incoming event signals.

#### `BuildSignalRoutingMap(ctx workflow.Context, data *activities.GetData) (map[string]*CompiledTrigger, error)`

Build routing map for signal processing.

---

## 🧪 Testing

### Unit Tests

```go
func TestWorker_TriggerWorkflow(t *testing.T) {
    tests := []struct {
        name        string
        workflowID  string
        input       *core.Input
        initTaskID  string
        wantErr     bool
    }{
        {
            name:       "Should trigger valid workflow",
            workflowID: "test-workflow",
            input: &core.Input{
                "param1": "value1",
                "param2": "value2",
            },
            initTaskID: "start-task",
            wantErr:    false,
        },
        {
            name:       "Should reject invalid workflow ID",
            workflowID: "nonexistent-workflow",
            input:      &core.Input{},
            initTaskID: "start-task",
            wantErr:    true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            w := setupTestWorker(t)

            result, err := w.TriggerWorkflow(context.Background(),
                tt.workflowID, tt.input, tt.initTaskID)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, result)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
                assert.Equal(t, tt.workflowID, result.WorkflowID)
            }
        })
    }
}
```

### Integration Tests

```go
func TestWorker_Integration(t *testing.T) {
    // Setup test environment
    temporalTestServer := setupTemporalTestServer(t)
    defer temporalTestServer.Stop()

    // Create test worker
    w := setupTestWorker(t)

    // Setup worker
    ctx := context.Background()
    err := w.Setup(ctx)
    require.NoError(t, err)

    // Test workflow execution
    input := &core.Input{
        "test_param": "test_value",
    }

    result, err := w.TriggerWorkflow(ctx, "test-workflow", input, "start")
    require.NoError(t, err)

    // Verify workflow execution
    assert.NotEmpty(t, result.WorkflowExecID)
    assert.Equal(t, "test-workflow", result.WorkflowID)

    // Cleanup
    w.Stop(ctx)
}
```

### Activity Tests

```go
func TestActivities_ExecuteBasicTask(t *testing.T) {
    activities := setupTestActivities(t)

    input := &ExecuteBasicTaskInput{
        TaskConfig: &task.Config{
            ID:   "test-task",
            Type: "basic",
            Use:  "tool(test-tool)",
        },
        TaskInput: &core.Input{
            "param": "value",
        },
    }

    output, err := activities.ExecuteBasicTask(context.Background(), input)
    require.NoError(t, err)

    assert.NotNil(t, output)
    assert.Equal(t, "test-task", output.TaskID)
}
```

### Best Practices

1. **Use test containers** for integration tests with Temporal
2. **Mock external dependencies** like Redis and databases
3. **Test error scenarios** and timeout conditions
4. **Verify signal handling** and event processing
5. **Test workflow state management** and persistence
6. **Use table-driven tests** for multiple scenarios
7. **Test health checks** and monitoring endpoints

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE)
