# `cache` – _Redis-based caching, distributed locking, and pub/sub notifications_

> **A unified cache infrastructure providing Redis client wrapper, distributed locking with Redlock algorithm, and pub/sub notification system for workflow orchestration.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Basic Cache Operations](#basic-cache-operations)
  - [Distributed Locking](#distributed-locking)
  - [Pub/Sub Notifications](#pubsub-notifications)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `cache` package provides a comprehensive Redis-based caching and coordination layer for the Compozy workflow orchestration engine. It combines Redis client operations, distributed locking using the Redlock algorithm, and pub/sub notifications into a unified interface.

This package is designed to handle:

- **Distributed caching** for workflow state and temporary data
- **Distributed locking** for coordinating concurrent workflow execution
- **Real-time notifications** for workflow events and task updates

---

## 💡 Motivation

- **Unified Interface**: Single package providing all Redis-based infrastructure needs
- **Distributed Coordination**: Redlock algorithm ensures reliable distributed locking across multiple instances
- **Event-Driven Architecture**: Pub/sub system enables real-time workflow and task notifications
- **Production Ready**: Comprehensive configuration, metrics, and health checking capabilities

---

## ⚡ Design Highlights

- **Redlock Algorithm**: Implements Redis distributed locking with automatic renewal and safe release
- **Type-Safe Events**: Strongly typed event structures for workflow and task notifications
- **Graceful Degradation**: Fallback mechanisms and comprehensive error handling
- **Metrics Integration**: Built-in metrics for monitoring lock operations and notification performance
- **Configuration Validation**: Extensive validation with environment variable support

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "time"

    "github.com/compozy/compozy/engine/infra/cache"
)

func main() {
    ctx := context.Background()

    // Setup cache with default configuration
    cache, err := cache.SetupCache(ctx, nil)
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    // Basic cache operation
    err = cache.Redis.Set(ctx, "key", "value", time.Hour).Err()
    if err != nil {
        panic(err)
    }

    // Distributed locking
    lock, err := cache.LockManager.Acquire(ctx, "resource", time.Minute)
    if err != nil {
        panic(err)
    }
    defer lock.Release(ctx)

    // Work with exclusive access to resource
    // ...
}
```

---

## 📖 Usage

### Basic Cache Operations

```go
// Set with expiration
err := cache.Redis.Set(ctx, "workflow:123", workflowData, time.Hour).Err()

// Get value
val, err := cache.Redis.Get(ctx, "workflow:123").Result()

// Check existence
exists, err := cache.Redis.Exists(ctx, "workflow:123").Result()

// Delete
err = cache.Redis.Del(ctx, "workflow:123").Err()
```

### Distributed Locking

```go
// Acquire lock with TTL
lock, err := cache.LockManager.Acquire(ctx, "workflow:123:execution", time.Minute)
if err != nil {
    return fmt.Errorf("failed to acquire lock: %w", err)
}
defer lock.Release(ctx)

// Check if lock is still held
if lock.IsHeld() {
    // Perform critical section work
    processWorkflow()
}

// Manually refresh lock if needed
err = lock.Refresh(ctx)
```

### Pub/Sub Notifications

```go
// Subscribe to workflow events
subscriber := cache.Notification.Subscribe(ctx, "workflow:events")
defer subscriber.Close()

// Publish workflow event
event := &cache.WorkflowEvent{
    ID:     "workflow-123",
    Status: "running",
    Data:   workflowData,
}
err = cache.Notification.PublishWorkflowEvent(ctx, event)

// Listen for events
for {
    select {
    case msg := <-subscriber.Messages():
        // Handle message
        handleWorkflowEvent(msg)
    case <-ctx.Done():
        return
    }
}
```

---

## 🔧 Configuration

### Environment Variables

```bash
# Redis Connection
REDIS_URL=redis://localhost:6379/0
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=secret
REDIS_DB=0

# Connection Pool
REDIS_POOL_SIZE=10
REDIS_MAX_IDLE_CONNS=5
REDIS_MIN_IDLE_CONNS=2

# Timeouts
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s
REDIS_POOL_TIMEOUT=4s
REDIS_PING_TIMEOUT=1s

# TLS
REDIS_TLS=false

# Retry Configuration
REDIS_MAX_RETRIES=3
REDIS_MIN_RETRY_BACKOFF=8ms
REDIS_MAX_RETRY_BACKOFF=512ms

# Notifications
REDIS_NOTIFICATION_BUFFER_SIZE=100
```

### Programmatic Configuration

```go
config := &cache.Config{
    Host:     "localhost",
    Port:     "6379",
    Password: "secret",
    DB:       0,
    PoolSize: 10,

    // Timeouts
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
    PoolTimeout:  4 * time.Second,
    PingTimeout:  1 * time.Second,

    // TLS
    TLSEnabled: false,

    // Retry configuration
    MaxRetries:      3,
    MinRetryBackoff: 8 * time.Millisecond,
    MaxRetryBackoff: 512 * time.Millisecond,

    // Pool configuration
    MaxIdleConns: 5,
    MinIdleConns: 2,

    // Notifications
    NotificationBufferSize: 100,
}

cache, err := cache.SetupCache(ctx, config)
```

---

## 🎨 Examples

### Workflow State Caching

```go
// Cache workflow state
workflowState := &WorkflowState{
    ID:     "workflow-123",
    Status: "running",
    Step:   3,
    Data:   stateData,
}

key := fmt.Sprintf("workflow:state:%s", workflowState.ID)
data, _ := json.Marshal(workflowState)
err := cache.Redis.Set(ctx, key, data, time.Hour).Err()
```

### Coordinated Task Processing

```go
func processTask(cache *cache.Cache, taskID string) error {
    // Acquire exclusive lock for task processing
    lockKey := fmt.Sprintf("task:processing:%s", taskID)
    lock, err := cache.LockManager.Acquire(ctx, lockKey, 5*time.Minute)
    if err != nil {
        return fmt.Errorf("task already being processed: %w", err)
    }
    defer lock.Release(ctx)

    // Process task exclusively
    return processTaskExclusively(taskID)
}
```

### Real-time Workflow Notifications

```go
func startWorkflowNotifications(cache *cache.Cache) {
    // Subscribe to workflow events
    subscriber := cache.Notification.Subscribe(ctx, "workflow:*")
    defer subscriber.Close()

    // Handle workflow events
    for {
        select {
        case msg := <-subscriber.Messages():
            var event cache.WorkflowEvent
            if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
                log.Error("Failed to unmarshal workflow event", "error", err)
                continue
            }

            // Process workflow event
            handleWorkflowEvent(&event)

        case <-ctx.Done():
            return
        }
    }
}
```

### Integration with Monitoring

```go
// Get lock metrics for monitoring
func getLockMetrics(cache *cache.Cache) *cache.LockMetrics {
    if lockManager, ok := cache.LockManager.(*cache.RedisLockManager); ok {
        return lockManager.GetMetrics()
    }
    return nil
}

// Get notification metrics
func getNotificationMetrics(cache *cache.Cache) *cache.NotificationMetrics {
    if notificationSystem, ok := cache.Notification.(*cache.RedisNotificationSystem); ok {
        return notificationSystem.GetMetrics()
    }
    return nil
}
```

---

## 📚 API Reference

### Core Types

#### `Cache`

Main cache wrapper combining Redis, LockManager, and NotificationSystem.

```go
type Cache struct {
    Redis        *Redis
    LockManager  LockManager
    Notification NotificationSystem
}

func SetupCache(ctx context.Context, config *Config) (*Cache, error)
func (c *Cache) Close() error
func (c *Cache) HealthCheck(ctx context.Context) error
```

#### `Redis`

Redis client wrapper implementing RedisInterface.

```go
type Redis struct {
    client redis.UniversalClient
    config *Config
}

func NewRedis(ctx context.Context, cfg *Config) (*Redis, error)
func (r *Redis) HealthCheck(ctx context.Context) error
```

#### `LockManager`

Distributed locking interface.

```go
type LockManager interface {
    Acquire(ctx context.Context, resource string, ttl time.Duration) (Lock, error)
}

type Lock interface {
    Release(ctx context.Context) error
    Refresh(ctx context.Context) error
    Resource() string
    IsHeld() bool
}
```

#### `NotificationSystem`

Pub/sub notification system interface.

```go
type NotificationSystem interface {
    PublishWorkflowEvent(ctx context.Context, event *WorkflowEvent) error
    PublishTaskEvent(ctx context.Context, event *TaskEvent) error
    Subscribe(ctx context.Context, pattern string) Subscriber
    Close() error
}

type Subscriber interface {
    Messages() <-chan *Message
    Close() error
}
```

### Configuration

#### `Config`

Configuration struct with validation.

```go
type Config struct {
    URL      string        `json:"url,omitempty"`
    Host     string        `json:"host,omitempty"`
    Port     string        `json:"port,omitempty"`
    Password string        `json:"password,omitempty"`
    DB       int           `json:"db,omitempty"`
    PoolSize int           `json:"pool_size,omitempty"`

    // TLS Configuration
    TLSEnabled bool        `json:"tls_enabled,omitempty"`
    TLSConfig  *tls.Config `json:"-"`

    // Timeout Configuration
    DialTimeout  time.Duration `json:"dial_timeout,omitempty"`
    ReadTimeout  time.Duration `json:"read_timeout,omitempty"`
    WriteTimeout time.Duration `json:"write_timeout,omitempty"`
    PoolTimeout  time.Duration `json:"pool_timeout,omitempty"`
    PingTimeout  time.Duration `json:"ping_timeout,omitempty"`

    // Pool Configuration
    MaxRetries      int           `json:"max_retries,omitempty"`
    MinRetryBackoff time.Duration `json:"min_retry_backoff,omitempty"`
    MaxRetryBackoff time.Duration `json:"max_retry_backoff,omitempty"`
    MaxIdleConns    int           `json:"max_idle_conns,omitempty"`
    MinIdleConns    int           `json:"min_idle_conns,omitempty"`

    // Notification Configuration
    NotificationBufferSize int `json:"notification_buffer_size,omitempty"`
}

func (c *Config) Validate() error
```

### Event Types

#### `WorkflowEvent`

Workflow notification event structure.

```go
type WorkflowEvent struct {
    ID     string      `json:"id"`
    Status string      `json:"status"`
    Data   interface{} `json:"data,omitempty"`
}
```

#### `TaskEvent`

Task notification event structure.

```go
type TaskEvent struct {
    ID         string      `json:"id"`
    WorkflowID string      `json:"workflow_id"`
    Status     string      `json:"status"`
    Data       interface{} `json:"data,omitempty"`
}
```

### Utility Functions

```go
func getEnvOrDefault(value, defaultValue string) string
```

---

## 🧪 Testing

### Unit Tests

```go
func TestCache(t *testing.T) {
    ctx := context.Background()

    // Setup test cache
    cache, err := cache.SetupCache(ctx, &cache.Config{
        Host: "localhost",
        Port: "6379",
        DB:   1, // Use separate DB for testing
    })
    require.NoError(t, err)
    defer cache.Close()

    t.Run("Should set and get values", func(t *testing.T) {
        err := cache.Redis.Set(ctx, "test:key", "test:value", time.Minute).Err()
        require.NoError(t, err)

        val, err := cache.Redis.Get(ctx, "test:key").Result()
        require.NoError(t, err)
        assert.Equal(t, "test:value", val)
    })
}
```

### Integration Tests

```go
func TestDistributedLocking(t *testing.T) {
    ctx := context.Background()

    // Setup two cache instances
    cache1, err := cache.SetupCache(ctx, testConfig())
    require.NoError(t, err)
    defer cache1.Close()

    cache2, err := cache.SetupCache(ctx, testConfig())
    require.NoError(t, err)
    defer cache2.Close()

    t.Run("Should prevent concurrent access", func(t *testing.T) {
        // Acquire lock from first instance
        lock1, err := cache1.LockManager.Acquire(ctx, "resource", time.Minute)
        require.NoError(t, err)
        defer lock1.Release(ctx)

        // Try to acquire same lock from second instance
        _, err = cache2.LockManager.Acquire(ctx, "resource", time.Second)
        assert.Error(t, err, "Should fail to acquire already held lock")
    })
}
```

### Test Utilities

```go
func testConfig() *cache.Config {
    return &cache.Config{
        Host:                   "localhost",
        Port:                   "6379",
        DB:                     1,
        PoolSize:               5,
        NotificationBufferSize: 10,
    }
}

func setupTestCache(t *testing.T) *cache.Cache {
    ctx := context.Background()
    cache, err := cache.SetupCache(ctx, testConfig())
    require.NoError(t, err)

    // Cleanup function
    t.Cleanup(func() {
        cache.Close()
    })

    return cache
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../../LICENSE)
