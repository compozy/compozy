# `engine/memory` – _Advanced Memory Management for AI Workflows_

> **Provides persistent, token-aware memory for AI agents with automatic summarization and smart flushing strategies to optimize performance and cost.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Manager-Instance Pattern](#manager-instance-pattern)
  - [Token Management](#token-management)
  - [Flushing Strategies](#flushing-strategies)
  - [Health Monitoring](#health-monitoring)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `engine/memory` package provides sophisticated memory management for AI agents in Compozy workflows. It implements a manager-instance pattern where a `Manager` creates and manages multiple `Memory` instances, each with dedicated storage, token counting, and privacy controls.

This package is designed to handle conversation histories, context management, and automatic memory optimization through intelligent flushing strategies that balance performance, cost, and context preservation.

---

## 💡 Motivation

- **Token Efficiency**: Automatically manages token counts and implements smart flushing to stay within LLM context limits
- **Performance Optimization**: Provides O(1) metadata operations and paginated access for large conversation histories
- **Privacy-First**: Built-in privacy controls with redaction and selective persistence for sensitive data
- **Resilient Operations**: Implements retry logic, health monitoring, and graceful degradation for production reliability

---

## ⚡ Design Highlights

- **Manager-Instance Pattern**: Centralized resource management with isolated memory instances
- **Token-Aware Operations**: Real-time token counting and monitoring with configurable thresholds
- **Hybrid Flushing Strategy**: Combines rule-based and intelligent summarization for optimal context preservation
- **Privacy by Design**: Comprehensive privacy controls with metadata-driven redaction
- **Health Monitoring**: Built-in diagnostics and health checks for operational visibility
- **Temporal Integration**: Async operations and workflow scheduling for scalable processing

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/memory"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/llm"
)

func main() {
    ctx := context.Background()

    // Create a memory manager
    manager, err := memory.NewManager(&memory.ManagerOptions{
        ResourceRegistry:  registry,    // autoload.ConfigRegistry
        TplEngine:        tplEngine,    // Template engine for key resolution
        BaseLockManager:  lockManager,  // Redis-based locking
        BaseRedisClient:  redisClient,  // Redis client for storage
        TemporalClient:   temporal,     // Temporal client for async ops
        Logger:           logger,       // Structured logger
    })
    if err != nil {
        log.Fatal(err)
    }

    // Get a memory instance
    memoryRef := core.MemoryReference{
        ID:  "conversation-memory",
        Key: "user:{{ .user_id }}:conversation",
    }

    workflowContext := map[string]any{
        "user_id": "user123",
    }

    memoryInstance, err := manager.GetInstance(ctx, memoryRef, workflowContext)
    if err != nil {
        log.Fatal(err)
    }

    // Use the memory
    message := llm.Message{
        Role:    "user",
        Content: "Hello, I need help with my account.",
    }

    err = memoryInstance.Append(ctx, message)
    if err != nil {
        log.Fatal(err)
    }

    // Read conversation history
    messages, err := memoryInstance.Read(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Conversation has %d messages", len(messages))
}
```

---

## 📖 Usage

### Library

The memory package provides three main components:

#### 1. Manager

Central coordinator that creates and manages memory instances:

```go
manager, err := memory.NewManager(&memory.ManagerOptions{
    ResourceRegistry:        registry,
    TplEngine:              tplEngine,
    BaseLockManager:        lockManager,
    BaseRedisClient:        redisClient,
    TemporalClient:         temporal,
    TemporalTaskQueue:      "memory-tasks",
    PrivacyManager:         privacyManager,
    FallbackProjectID:      "default-project",
    Logger:                 logger,
})
```

#### 2. Memory Instances

Individual memory stores with conversation history:

```go
// Append messages
err = memory.Append(ctx, llm.Message{
    Role:    "assistant",
    Content: "I can help you with your account. What do you need?",
})

// Read with pagination
messages, total, err := memory.ReadPaginated(ctx, 0, 10)

// Get current token count
tokenCount, err := memory.GetTokenCount(ctx)
```

#### 3. Health Monitoring

Built-in diagnostics and monitoring:

```go
// Create health service
healthService := memory.NewHealthService(ctx, manager)

// Check memory health
health, err := memory.GetMemoryHealth(ctx)
fmt.Printf("Token count: %d, Message count: %d", health.TokenCount, health.MessageCount)
```

### Manager-Instance Pattern

The package uses a manager-instance pattern where:

- **Manager**: Creates and configures memory instances
- **Instance**: Handles actual memory operations
- **Resource**: Defines memory configuration and policies

```go
// Manager creates instances based on configuration
memoryRef := core.MemoryReference{
    ID:  "support-memory",
    Key: "support:{{ .ticket_id }}",
}

instance, err := manager.GetInstance(ctx, memoryRef, workflowContext)
```

### Token Management

Advanced token counting and management:

```go
// Get token counter
counter, err := manager.GetTokenCounter(ctx)

// Count tokens in text
count, err := counter.CountTokens(ctx, "Hello world")

// Token-aware memory manager
tokenManager := memory.NewTokenMemoryManager(
    resourceConfig,
    counter,
    logger,
)
```

### Flushing Strategies

Smart memory flushing to optimize context:

```go
// Rule-based summarizer
summarizer := memory.NewRuleBasedSummarizer(
    counter,
    5,  // Keep first 5 messages
    10, // Keep last 10 messages
)

// Hybrid flushing strategy
strategy, err := memory.NewHybridFlushingStrategy(
    flushConfig,
    summarizer,
    tokenManager,
)

// Perform flush
flushOutput, err := strategy.PerformFlush(ctx, messages, resourceConfig)
```

### Health Monitoring

Comprehensive health monitoring and diagnostics:

```go
// Global health service
healthService := memory.InitializeGlobalHealthService(ctx, manager)

// Register instance for monitoring
memory.RegisterInstanceGlobally("memory-123")

// Check system health
systemHealth, err := healthService.GetSystemHealth(ctx)
```

---

## 🔧 Configuration

Memory resources are configured through YAML files:

```yaml
# memory/conversation.yaml
id: conversation-memory
description: "User conversation history"
type: memory

# Storage configuration
storage:
  type: redis
  key_prefix: "memory:conversation:"

# Token management
token_management:
  max_tokens: 4000
  warning_threshold: 3200
  counter_model: "gpt-4"

# Flushing strategy
flushing:
  strategy: "hybrid"
  max_tokens_before_flush: 3500
  keep_first_messages: 3
  keep_last_messages: 10

# Privacy controls
privacy:
  redaction_enabled: true
  sensitive_fields: ["email", "phone", "ssn"]
  retention_days: 30

# Health monitoring
health:
  check_interval: "30s"
  token_usage_alert_threshold: 0.8
```

---

## 🎨 Examples

### Basic Conversation Memory

```go
func setupConversationMemory() {
    ctx := context.Background()

    // Create manager
    manager, err := memory.NewManager(&memory.ManagerOptions{
        ResourceRegistry:  registry,
        TplEngine:        tplEngine,
        BaseLockManager:  lockManager,
        BaseRedisClient:  redisClient,
        TemporalClient:   temporal,
        Logger:           logger,
    })
    if err != nil {
        panic(err)
    }

    // Get memory instance
    memoryRef := core.MemoryReference{
        ID:  "conversation",
        Key: "user:{{ .user_id }}:conversation",
    }

    memory, err := manager.GetInstance(ctx, memoryRef, map[string]any{
        "user_id": "user123",
    })
    if err != nil {
        panic(err)
    }

    // Store conversation
    messages := []llm.Message{
        {Role: "user", Content: "Hi, I need help"},
        {Role: "assistant", Content: "I'm here to help!"},
        {Role: "user", Content: "What's my account balance?"},
    }

    for _, msg := range messages {
        err = memory.Append(ctx, msg)
        if err != nil {
            panic(err)
        }
    }

    // Read back conversation
    history, err := memory.Read(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Conversation has %d messages\n", len(history))
}
```

### Privacy-Aware Memory

```go
func privacyAwareMemory() {
    ctx := context.Background()

    // Create memory with privacy controls
    memory, err := getMemoryInstance(ctx, "sensitive-data-memory")
    if err != nil {
        panic(err)
    }

    // Message with sensitive data
    message := llm.Message{
        Role:    "user",
        Content: "My email is john@example.com and phone is 555-1234",
    }

    // Privacy metadata
    privacyMetadata := memcore.PrivacyMetadata{
        SensitiveFields: []string{"email", "phone"},
        PrivacyLevel:    "confidential",
        RedactionApplied: false,
    }

    // Store with privacy controls
    err = memory.AppendWithPrivacy(ctx, message, privacyMetadata)
    if err != nil {
        panic(err)
    }

    // Message will be automatically redacted based on privacy rules
}
```

### Health Monitoring Setup

```go
func setupHealthMonitoring() {
    ctx := context.Background()

    // Initialize global health service
    healthService := memory.InitializeGlobalHealthService(ctx, manager)

    // Start background monitoring
    memory.StartGlobalHealthService(ctx)

    // Register memory instances
    memory.RegisterInstanceGlobally("user-123-conversation")
    memory.RegisterInstanceGlobally("support-ticket-456")

    // Set up HTTP health endpoints
    router := gin.New()
    memory.RegisterMemoryHealthRoutes(router, healthService)

    // Custom health checks
    go func() {
        for {
            systemHealth, err := healthService.GetSystemHealth(ctx)
            if err != nil {
                log.Printf("Health check failed: %v", err)
                continue
            }

            if systemHealth.TokenUsage > 0.8 {
                log.Printf("High token usage: %.2f%%", systemHealth.TokenUsage*100)
            }

            time.Sleep(30 * time.Second)
        }
    }()
}
```

### Advanced Flushing Strategy

```go
func setupAdvancedFlushing() {
    ctx := context.Background()

    // Create token counter
    counter, err := tokens.NewTiktokenCounter("gpt-4")
    if err != nil {
        panic(err)
    }

    // Create summarizer with custom rules
    summarizer := memory.NewRuleBasedSummarizerWithOptions(
        counter,
        3,  // Keep first 3 messages
        5,  // Keep last 5 messages
        50, // Fallback ratio 50%
    )

    // Create token manager
    tokenManager, err := memory.NewTokenMemoryManager(
        resourceConfig,
        counter,
        logger,
    )
    if err != nil {
        panic(err)
    }

    // Create hybrid flushing strategy
    strategy, err := memory.NewHybridFlushingStrategy(
        &memcore.FlushingStrategyConfig{
            MaxTokensBeforeFlush: 3500,
            KeepFirstMessages:    3,
            KeepLastMessages:     5,
            Strategy:             "hybrid",
        },
        summarizer,
        tokenManager,
    )
    if err != nil {
        panic(err)
    }

    // Use strategy in memory operations
    memory := &flushableMemory{
        baseMemory: baseMemory,
        strategy:   strategy,
    }

    // Automatic flushing when thresholds are reached
    err = memory.Append(ctx, llm.Message{
        Role:    "user",
        Content: "This message might trigger a flush if token limit is reached",
    })
}
```

---

## 📚 API Reference

### Manager

```go
// NewManager creates a new memory manager
func NewManager(opts *ManagerOptions) (*Manager, error)

// GetInstance retrieves or creates a memory instance
func (m *Manager) GetInstance(
    ctx context.Context,
    agentMemoryRef core.MemoryReference,
    workflowContextData map[string]any,
) (memcore.Memory, error)

// GetTokenCounter returns a token counter
func (m *Manager) GetTokenCounter(ctx context.Context) (memcore.TokenCounter, error)

// GetMemoryConfig retrieves memory configuration
func (m *Manager) GetMemoryConfig(memoryID string) (*memcore.Resource, error)
```

### Memory Interface

```go
// Core memory operations
type Memory interface {
    Append(ctx context.Context, msg llm.Message) error
    Read(ctx context.Context) ([]llm.Message, error)
    ReadPaginated(ctx context.Context, offset, limit int) ([]llm.Message, int, error)
    Len(ctx context.Context) (int, error)
    GetTokenCount(ctx context.Context) (int, error)
    GetMemoryHealth(ctx context.Context) (*Health, error)
    Clear(ctx context.Context) error
    GetID() string
    AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata PrivacyMetadata) error
}
```

### Token Management

```go
// NewTokenMemoryManager creates a token-aware memory manager
func NewTokenMemoryManager(
    config *memcore.Resource,
    counter memcore.TokenCounter,
    log logger.Logger,
) (*TokenMemoryManager, error)

// Token counter interface
type TokenCounter interface {
    CountTokens(ctx context.Context, text string) (int, error)
    GetEncoding() string
}
```

### Flushing Strategies

```go
// NewHybridFlushingStrategy creates a hybrid flushing strategy
func NewHybridFlushingStrategy(
    config *memcore.FlushingStrategyConfig,
    summarizer MessageSummarizer,
    tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error)

// NewRuleBasedSummarizer creates a rule-based summarizer
func NewRuleBasedSummarizer(
    counter memcore.TokenCounter,
    keepFirstN int,
    keepLastN int,
) *RuleBasedSummarizer
```

### Health Monitoring

```go
// NewHealthService creates a health service
func NewHealthService(ctx context.Context, manager *Manager) *HealthService

// Global health service management
func InitializeGlobalHealthService(ctx context.Context, manager *Manager) *HealthService
func StartGlobalHealthService(ctx context.Context)
func StopGlobalHealthService()
func RegisterInstanceGlobally(memoryID string)
func UnregisterInstanceGlobally(memoryID string)
```

---

## 🧪 Testing

Run the test suite:

```bash
# Run all memory tests
go test ./engine/memory/...

# Run specific test
go test -v ./engine/memory -run TestManager_GetInstance

# Run tests with coverage
go test -cover ./engine/memory/...

# Run integration tests
go test -tags=integration ./engine/memory/...
```

### Test Examples

```go
func TestMemoryBasicOperations(t *testing.T) {
    t.Run("Should append and read messages", func(t *testing.T) {
        ctx := context.Background()
        memory := setupTestMemory(t)

        message := llm.Message{
            Role:    "user",
            Content: "Test message",
        }

        err := memory.Append(ctx, message)
        require.NoError(t, err)

        messages, err := memory.Read(ctx)
        require.NoError(t, err)
        assert.Len(t, messages, 1)
        assert.Equal(t, "Test message", messages[0].Content)
    })
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
