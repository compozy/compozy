---
title: "Worker Concurrency Hardcoded Limits"
group: "ENGINE_GROUP_4_INFRASTRUCTURE"
category: "performance"
priority: "🟡 MEDIUM"
status: "pending"
source: "tasks/reviews/ENGINE_GROUP_4_INFRASTRUCTURE_PERFORMANCE.md"
issue_index: "4"
sequence: "20"
---

## Worker Concurrency Hardcoded Limits

**Location:** `engine/worker/manager.go:42–47`, `dispatcher.go`

**Severity:** 🟡 MEDIUM

**Issue:**

```go
// Lines 42-47 - Hardcoded timeout and retry values
func (m *Manager) BuildErrHandler(ctx workflow.Context) func(err error) error {
    ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,     // ❌ Hardcoded
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,                     // ❌ Hardcoded
        },
    })
    // ... rest of handler
}
```

**Problems:**

1. **No configuration:** Can't tune for different workloads
2. **30s timeout:** Too short for heavy tasks, too long for light tasks
3. **3 retries:** May be too many (expensive operations) or too few (flaky operations)
4. **Concurrency:** Worker pool size hardcoded in Temporal worker options

**Fix:**

```go
// pkg/config/worker.go (NEW FILE or add to existing)
type WorkerConfig struct {
    // Temporal worker configuration
    MaxConcurrentActivityExecutionSize     int           `yaml:"max_concurrent_activities" mapstructure:"max_concurrent_activities"`
    MaxConcurrentWorkflowExecutionSize     int           `yaml:"max_concurrent_workflows" mapstructure:"max_concurrent_workflows"`
    MaxConcurrentLocalActivityExecutionSize int          `yaml:"max_concurrent_local_activities" mapstructure:"max_concurrent_local_activities"`

    // Activity defaults
    ActivityStartToCloseTimeout time.Duration `yaml:"activity_start_to_close_timeout" mapstructure:"activity_start_to_close_timeout"`
    ActivityHeartbeatTimeout    time.Duration `yaml:"activity_heartbeat_timeout" mapstructure:"activity_heartbeat_timeout"`
    ActivityMaxRetries          int           `yaml:"activity_max_retries" mapstructure:"activity_max_retries"`

    // Error handler specific
    ErrorHandlerTimeout    time.Duration `yaml:"error_handler_timeout" mapstructure:"error_handler_timeout"`
    ErrorHandlerMaxRetries int           `yaml:"error_handler_max_retries" mapstructure:"error_handler_max_retries"`
}

func DefaultWorkerConfig() WorkerConfig {
    numCPU := runtime.NumCPU()
    return WorkerConfig{
        // Default to 2x CPU cores for activities, 1x for workflows
        MaxConcurrentActivityExecutionSize:      numCPU * 2,
        MaxConcurrentWorkflowExecutionSize:      numCPU,
        MaxConcurrentLocalActivityExecutionSize: numCPU * 4,

        ActivityStartToCloseTimeout: 5 * time.Minute,
        ActivityHeartbeatTimeout:    30 * time.Second,
        ActivityMaxRetries:          3,

        ErrorHandlerTimeout:    30 * time.Second,
        ErrorHandlerMaxRetries: 3,
    }
}
```

**Update Manager:**

```go
// engine/worker/manager.go
type Manager struct {
    *ContextBuilder
    *executors.WorkflowExecutor
    *executors.TaskExecutor
    workerConfig *config.WorkerConfig // NEW
}

func NewManager(contextBuilder *ContextBuilder, cfg *config.WorkerConfig) *Manager {
    if cfg == nil {
        defaultCfg := config.DefaultWorkerConfig()
        cfg = &defaultCfg
    }

    executorContextBuilder := executors.NewContextBuilder(
        contextBuilder.Workflows,
        contextBuilder.ProjectConfig,
        contextBuilder.WorkflowConfig,
        contextBuilder.WorkflowInput,
    )
    workflowExecutor := executors.NewWorkflowExecutor(executorContextBuilder)
    taskExecutor := executors.NewTaskExecutor(executorContextBuilder)

    return &Manager{
        ContextBuilder:   contextBuilder,
        WorkflowExecutor: workflowExecutor,
        TaskExecutor:     taskExecutor,
        workerConfig:     cfg,
    }
}

func (m *Manager) BuildErrHandler(ctx workflow.Context) func(err error) error {
    // Use configurable values
    ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: m.workerConfig.ErrorHandlerTimeout,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: int32(m.workerConfig.ErrorHandlerMaxRetries),
        },
    })
    return func(err error) error {
        // ... existing error handling logic ...
    }
}
```

**Update worker initialization:**

```go
// engine/infra/server/worker.go (or wherever Temporal worker is created)
func (s *Server) startTemporalWorker(ctx context.Context) error {
    cfg := config.FromContext(ctx)

    w := worker.New(s.temporalClient, "compozy-task-queue", worker.Options{
        MaxConcurrentActivityExecutionSize:      cfg.Worker.MaxConcurrentActivityExecutionSize,
        MaxConcurrentWorkflowExecutionSize:      cfg.Worker.MaxConcurrentWorkflowExecutionSize,
        MaxConcurrentLocalActivityExecutionSize: cfg.Worker.MaxConcurrentLocalActivityExecutionSize,
    })

    // Register workflows and activities
    // ...

    return w.Start()
}
```

**Configuration example:**

```yaml
# config.yaml
worker:
  max_concurrent_activities: 16 # 2x CPU cores for 8-core machine
  max_concurrent_workflows: 8 # 1x CPU cores
  max_concurrent_local_activities: 32 # 4x CPU cores (local activities are fast)

  activity_start_to_close_timeout: 5m
  activity_heartbeat_timeout: 30s
  activity_max_retries: 3

  error_handler_timeout: 30s
  error_handler_max_retries: 3
```

**Impact:**

- **Tuning flexibility:** Adjust concurrency per deployment
- **Resource utilization:** Better CPU usage on large machines
- **Timeout control:** Shorter timeouts for faster failure detection

**Effort:** M (4h)  
**Risk:** Low - add configuration, keep defaults

## Medium Priority Issues
