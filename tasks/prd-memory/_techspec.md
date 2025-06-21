# Memory as a Shared Resource – Technical Specification

**Document Version**: 2.0  
**Last Updated**: 2025-06-21  
**Author**: Tech Spec Creator Agent  
**Status**: Updated - Aligned with Existing Architecture  
**Related PRD**: tasks/prd-memory/\_prd.md

> **Architecture Alignment Note**: This specification has been updated to maximize reuse of existing Compozy infrastructure. The memory system extends existing components rather than duplicating functionality, using only tiktoken-go as an additional dependency.

---

## Executive Summary

This specification describes how to introduce **project-level, shareable conversation memory** to Compozy.  
The solution adds a new `engine/memory` domain that exposes:

1. **Extends existing Autoload Registry** with memory resource loading and runtime lookup capabilities
2. A **Memory Manager** that instantiates **Redis-backed, token-managed memory instances** using existing Redis infrastructure
3. **Reuses existing LockManager** from `engine/infra/cache` for cluster-wide safety with memory operations
4. **Enhanced Agent Configuration** supporting three complexity levels from ultra-simple to fully customizable
5. Minimal changes to the **Agent Runtime** and **LLM Orchestrator** using Temporal activities for async operations

The design follows Clean Architecture principles, provides progressive complexity for developer experience, and maximally reuses existing infrastructure components.

### Infrastructure Reuse Summary

This memory system is designed to **extend rather than duplicate** existing Compozy infrastructure:

| Component               | Existing Infrastructure Used                         | Benefit                                        |
| ----------------------- | ---------------------------------------------------- | ---------------------------------------------- |
| **Redis Operations**    | `engine/infra/store` Redis pool                      | No new Redis connections or configurations     |
| **Distributed Locking** | `engine/infra/cache/LockManager` (Redlock algorithm) | Proven cluster-safe locking, no new libraries  |
| **Async Processing**    | Temporal activities in `engine/worker`               | Consistent async patterns, no asynq dependency |
| **Resource Management** | `engine/autoload/ConfigRegistry`                     | Unified resource loading, no new registry      |
| **Circuit Breaker**     | Pattern from `engine/worker/dispatcher.go`           | Reuse existing `circuitBreakerDelay` constant  |
| **Error Handling**      | `core.NewError` patterns                             | Consistent error codes and handling            |
| **Template Processing** | Existing `tplengine`                                 | Key template evaluation using existing engine  |

**New Dependencies**: Only `tiktoken-go` for accurate token counting

---

## System Architecture

### Domain Placement

| Layer              | Package / Path               | Responsibility                                           |
| ------------------ | ---------------------------- | -------------------------------------------------------- |
| **Domain**         | `engine/memory`              | Core memory interfaces, token eviction, token management |
| **Infrastructure** | `engine/infra/store`         | Redis client (existing, reused)                          |
| **Infrastructure** | `engine/infra/cache`         | LockManager (existing, reused)                           |
| **Infrastructure** | `engine/autoload`            | ConfigRegistry (existing, extended for memory resources) |
| **Application**    | `engine/memory/manager`      | Runtime instance management, configuration resolution    |
| **Integration**    | `engine/agent`, `engine/llm` | Resolve memory references, enforce modes                 |
| **Integration**    | `engine/worker/activities`   | Temporal activities for async memory operations          |

### Component Overview

1. **MemoryResourceLoader** – loads & validates memory resources during project bootstrap, registers with existing `autoload.ConfigRegistry`
2. **MemoryStore (Redis)** – persistence interface with Redis implementation using existing `infra/store` Redis pool
3. **MemoryInstance** – cluster-safe handle that uses existing `cache.LockManager`, token counting, hybrid flushing
4. **MemoryManager** – factory that (a) evaluates key templates using `tplengine`, (b) creates or fetches `MemoryInstance`s, (c) enforces read-only mode, (d) resolves simplified agent configurations
5. **Memory Activities** – Temporal activities in `worker/activities` for async operations (append, read, flush)
6. **Agent Memory Adapter** – lightweight wrapper injected into `LLM Orchestrator` that uses Temporal activities
7. **Configuration Resolver** – handles three levels of agent memory configuration complexity with validation and smart defaults

Data Flow (high-level):

```
Agent --> LLM Orchestrator --> Temporal Activity --> MemoryManager -> MemoryInstance --> LockManager
                                                                  ^                   |        |
                                                                  |<------------------+        v
                                                                                         MemoryStore (Redis)
                                                                                         [Token Management + Hybrid Flushing]
```

---

## Implementation Design

### Core Interfaces

```go
// Memory abstracts read/write conversation history operations.
type Memory interface {
    // Core operations
    Append(ctx context.Context, msg llm.Message) error
    Read(ctx context.Context) ([]llm.Message, error)
    Len(ctx context.Context) (int, error)

    // Diagnostic operations
    GetTokenCount(ctx context.Context) (int, error)
    GetMemoryHealth(ctx context.Context) (*MemoryHealth, error)
}

// MemoryStore provides persistence-agnostic message storage operations.
type MemoryStore interface {
    AppendMessage(ctx context.Context, key string, msg llm.Message) error
    ReadMessages(ctx context.Context, key string) ([]llm.Message, error)
    CountMessages(ctx context.Context, key string) (int, error)
    TrimMessages(ctx context.Context, key string, keepCount int) error
    ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error
    SetExpiration(ctx context.Context, key string, ttl time.Duration) error
}

// Note: We reuse existing cache.LockManager and cache.Lock interfaces from engine/infra/cache

// MemoryHealth provides diagnostic information about memory state.
type MemoryHealth struct {
    TokenCount      int                    `json:"token_count"`
    MessageCount    int                    `json:"message_count"`
    LastFlush       *time.Time             `json:"last_flush,omitempty"`
    FlushStrategy   string                 `json:"flush_strategy"`
}
```

### Enhanced Data Models

```go
// MemoryResource loaded from project config with enhanced features
type MemoryResource struct {
    ID          string  `yaml:"id"`
    Description string  `yaml:"description"`
    Type        string  `yaml:"type"` // token_based (only option for v1)
    MaxTokens   int     `yaml:"max_tokens,omitempty"`
    MaxRatio    float64 `yaml:"max_context_ratio,omitempty"`

    // Token allocation ratios (optional)
    TokenAllocation *TokenAllocation `yaml:"token_allocation,omitempty"`

    // Flushing strategy (defaults to hybrid)
    FlushingStrategy *FlushingStrategy `yaml:"flushing_strategy,omitempty"`

    Persistence struct {
        Type string        `yaml:"type"` // redis
        TTL  time.Duration `yaml:"ttl"`
    } `yaml:"persistence"`
}

// TokenAllocation defines how tokens are distributed across memory types
type TokenAllocation struct {
    ShortTerm float64 `yaml:"short_term"` // 0.7 = 70%
    LongTerm  float64 `yaml:"long_term"`  // 0.2 = 20%
    System    float64 `yaml:"system"`     // 0.1 = 10%
}

// FlushingStrategy defines how memory handles token pressure
type FlushingStrategy struct {
    Type                    string  `yaml:"type"`                      // hybrid_summary (default)
    SummarizeThreshold      float64 `yaml:"summarize_threshold"`       // 0.8 = 80%
    SummaryTokens           int     `yaml:"summary_tokens"`            // tokens for summaries
    SummarizeOldestPercent  int     `yaml:"summarize_oldest_percent"`  // 30%
}

// Enhanced AgentConfig supporting three configuration levels correctly
type AgentConfig struct {
    // Existing fields...

    // Level 1: Direct memory ID reference
    // Example: memory: "customer-support-context"
    // Level 2: Simple multi-memory enablement
    // Example: memory: true + memories: [...] + memory_key: "..."
    // Level 3: Advanced configuration
    // Example: memories: [{id: "...", mode: "...", key: "..."}]
    Memory    interface{}       `yaml:"memory,omitempty"`     // string (Level 1) | bool (Level 2)
    Memories  interface{}       `yaml:"memories,omitempty"`   // []string (Level 2) | []MemoryReference (Level 3)
    MemoryKey string            `yaml:"memory_key,omitempty"` // For Level 1 and Level 2
}

// MemoryReference for advanced agent configuration (Level 3)
type MemoryReference struct {
    ID   string `yaml:"id"`
    Mode string `yaml:"mode"` // read-write | read-only
    Key  string `yaml:"key"`  // template string
}
```

### Fixed Configuration Resolution System

```go
// ConfigurationResolver handles three levels of agent memory configuration correctly
type ConfigurationResolver struct {
    registry  *MemoryRegistry
    validator *ConfigValidator
}

func (r *ConfigurationResolver) ResolveMemoryConfig(config *AgentConfig) ([]MemoryReference, error) {
    var memoryRefs []MemoryReference

    // Check if Level 3 (Advanced) configuration is used first
    if config.Memories != nil {
        if memoriesSlice, ok := config.Memories.([]interface{}); ok {
            // Level 3: Advanced configuration with full MemoryReference objects
            return r.parseAdvancedMemoryConfig(memoriesSlice)
        } else if memoriesStringSlice, ok := config.Memories.([]string); ok {
            // Level 2: Simple multi-memory with string array
            // Must be used with memory: true for Level 2 pattern
            if config.Memory == nil {
                return nil, fmt.Errorf("Level 2 configuration requires 'memory: true' when using 'memories' array")
            }
            if memoryBool, ok := config.Memory.(bool); !ok || !memoryBool {
                return nil, fmt.Errorf("Level 2 configuration requires 'memory: true' when using 'memories' array")
            }
            if config.MemoryKey == "" {
                return nil, fmt.Errorf("memory_key is required for Level 2 configuration")
            }
            for _, memID := range memoriesStringSlice {
                memoryRefs = append(memoryRefs, MemoryReference{
                    ID:   memID,
                    Mode: "read-write", // default for Level 2
                    Key:  config.MemoryKey,
                })
            }
        } else {
            return nil, fmt.Errorf("invalid memories configuration type: %T", config.Memories)
        }
    } else if config.Memory != nil {
        // Handle Level 1 via memory field
        switch v := config.Memory.(type) {
        case string:
            // Level 1: Direct memory ID reference
            if config.MemoryKey == "" {
                return nil, fmt.Errorf("memory_key is required for Level 1 configuration")
            }
            memoryRefs = append(memoryRefs, MemoryReference{
                ID:   v,
                Mode: "read-write", // default for Level 1
                Key:  config.MemoryKey,
            })

        case bool:
            if v {
                // Level 2: memory: true requires memories array of strings
                return nil, fmt.Errorf("when memory: true, the 'memories' field must be a non-empty array of memory IDs")
            }
            // memory: false means no memory configuration
            return []MemoryReference{}, nil

        default:
            return nil, fmt.Errorf("invalid memory configuration type: %T", v)
        }
    } else {
        // No memory configuration - stateless agent
        return []MemoryReference{}, nil
    }

    // Validate all referenced memory IDs exist
    for _, ref := range memoryRefs {
        if !r.registry.Exists(ref.ID) {
            return nil, fmt.Errorf("memory resource not found: %s", ref.ID)
        }
    }

    return memoryRefs, nil
}

func (r *ConfigurationResolver) parseAdvancedMemoryConfig(memoriesInterface []interface{}) ([]MemoryReference, error) {
    var memoryRefs []MemoryReference

    for i, memInterface := range memoriesInterface {
        memMap, ok := memInterface.(map[string]interface{})
        if !ok {
            return nil, fmt.Errorf("advanced memory config item %d must be an object", i)
        }

        id, ok := memMap["id"].(string)
        if !ok {
            return nil, fmt.Errorf("advanced memory config item %d missing required 'id' field", i)
        }

        mode, ok := memMap["mode"].(string)
        if !ok {
            mode = "read-write" // default
        }

        key, ok := memMap["key"].(string)
        if !ok {
            return nil, fmt.Errorf("advanced memory config item %d missing required 'key' field", i)
        }

        memoryRefs = append(memoryRefs, MemoryReference{
            ID:   id,
            Mode: mode,
            Key:  key,
        })
    }

    return memoryRefs, nil
}
```

### Token Management Integration

```go
// TokenMemoryManager handles token counting and FIFO eviction
type TokenMemoryManager struct {
    resource      *MemoryResource
    tokenCounter  TokenCounter  // Uses tiktoken-go for accurate counting
}

func (t *TokenMemoryManager) EvictMessages(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
    // Calculate total tokens using tiktoken-go
    totalTokens := 0
    tokenCounts := make([]int, len(messages))

    for i, msg := range messages {
        count, err := t.tokenCounter.CountTokens(msg.Content)
        if err != nil {
            return nil, fmt.Errorf("failed to count tokens: %w", err)
        }
        tokenCounts[i] = count
        totalTokens += count
    }

    // Check if eviction is needed
    maxTokens := t.getMaxTokens()
    if totalTokens <= maxTokens {
        return messages, nil
    }

    // FIFO eviction - remove oldest messages until under limit
    for totalTokens > maxTokens && len(messages) > 0 {
        totalTokens -= tokenCounts[0]
        messages = messages[1:]
        tokenCounts = tokenCounts[1:]
    }

    return messages, nil
}

func (t *TokenMemoryManager) getMaxTokens() int {
    if t.resource.MaxTokens > 0 {
        return t.resource.MaxTokens
    }
    if t.resource.MaxRatio > 0 {
        // Calculate based on context ratio (would need model context size)
        return int(float64(4096) * t.resource.MaxRatio) // default 4K context
    }
    return 4000 // default
}

// TokenCounter interface for tiktoken-go integration
type TokenCounter interface {
    CountTokens(text string) (int, error)
    GetEncoding() string
}

// TiktokenCounter implements TokenCounter using tiktoken-go
type TiktokenCounter struct {
    encoding *tiktoken.Encoding
}

func NewTiktokenCounter(model string) (*TiktokenCounter, error) {
    encoding, err := tiktoken.EncodingForModel(model)
    if err != nil {
        // Fall back to a default encoding
        encoding, err = tiktoken.GetEncoding("cl100k_base")
        if err != nil {
            return nil, err
        }
    }
    return &TiktokenCounter{encoding: encoding}, nil
}

func (tc *TiktokenCounter) CountTokens(text string) (int, error) {
    tokens := tc.encoding.Encode(text, nil, nil)
    return len(tokens), nil
}
```

### Circuit Breaker for Memory Operations

```go
// MemoryCircuitBreaker protects memory operations from cascading failures
type MemoryCircuitBreaker struct {
    manager         *MemoryManager
    failureCount    atomic.Int32
    lastFailureTime atomic.Int64
    state           atomic.Int32 // 0=closed, 1=open, 2=half-open
}

const (
    circuitClosed    int32 = 0
    circuitOpen      int32 = 1
    circuitHalfOpen  int32 = 2

    maxFailures      = 5
    circuitBreakerDelay = 5 * time.Second // Reuse existing pattern from dispatcher.go
)

func (cb *MemoryCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
    state := cb.state.Load()

    switch state {
    case circuitOpen:
        // Check if we should transition to half-open
        lastFailure := time.Unix(0, cb.lastFailureTime.Load())
        if time.Since(lastFailure) > circuitBreakerDelay {
            cb.state.CompareAndSwap(circuitOpen, circuitHalfOpen)
            cb.failureCount.Store(0)
        } else {
            return ErrCircuitBreakerOpen
        }
    }

    // Execute the function
    err := fn()

    if err != nil {
        failures := cb.failureCount.Add(1)
        cb.lastFailureTime.Store(time.Now().UnixNano())

        if failures >= maxFailures {
            cb.state.Store(circuitOpen)
            log.Warn("Circuit breaker opened", "failures", failures)
        }
        return err
    }

    // Success - reset failure count
    if state == circuitHalfOpen {
        cb.state.Store(circuitClosed)
        log.Info("Circuit breaker closed after recovery")
    }
    cb.failureCount.Store(0)

    return nil
}

// Integration with MemoryInstance
func (m *MemoryInstance) AppendWithCircuitBreaker(ctx context.Context, msg llm.Message) error {
    return m.circuitBreaker.Execute(ctx, func() error {
        return m.Append(ctx, msg)
    })
}
```

### Priority-Based Eviction Implementation

```go
// MessagePriority defines priority levels for messages
type MessagePriority int

const (
    PriorityLow    MessagePriority = 0
    PriorityNormal MessagePriority = 1
    PriorityHigh   MessagePriority = 2
    PrioritySystem MessagePriority = 3 // System messages never evicted
)

// PriorityEvictionManager handles priority-based message eviction
type PriorityEvictionManager struct {
    tokenCounter TokenCounter
    maxTokens    int
}

func (p *PriorityEvictionManager) EvictByPriority(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
    // Group messages by priority
    priorityGroups := make(map[MessagePriority][]llm.Message)
    tokensByPriority := make(map[MessagePriority]int)

    for _, msg := range messages {
        priority := extractPriority(msg)
        priorityGroups[priority] = append(priorityGroups[priority], msg)

        tokens, err := p.tokenCounter.CountTokens(msg.Content)
        if err != nil {
            return nil, fmt.Errorf("failed to count tokens: %w", err)
        }
        tokensByPriority[priority] += tokens
    }

    totalTokens := 0
    for _, tokens := range tokensByPriority {
        totalTokens += tokens
    }

    // If under limit, return all messages
    if totalTokens <= p.maxTokens {
        return messages, nil
    }

    // Evict starting from lowest priority
    result := make([]llm.Message, 0, len(messages))
    currentTokens := totalTokens

    // Always keep system messages
    result = append(result, priorityGroups[PrioritySystem]...)
    currentTokens -= tokensByPriority[PrioritySystem]

    // Add messages by priority until we exceed limit
    for priority := PriorityHigh; priority >= PriorityLow; priority-- {
        groupMsgs := priorityGroups[priority]
        groupTokens := tokensByPriority[priority]

        if currentTokens <= p.maxTokens {
            // We have room for all messages at this priority
            result = append(result, groupMsgs...)
        } else if currentTokens - groupTokens < p.maxTokens {
            // We need to partially include this priority group
            // Use FIFO within the priority group
            for i := len(groupMsgs) - 1; i >= 0; i-- {
                msgTokens, _ := p.tokenCounter.CountTokens(groupMsgs[i].Content)
                if currentTokens - msgTokens >= p.maxTokens {
                    currentTokens -= msgTokens
                } else {
                    result = append(result, groupMsgs[i])
                }
            }
        } else {
            // Skip entire priority group
            currentTokens -= groupTokens
        }
    }

    return result, nil
}

func extractPriority(msg llm.Message) MessagePriority {
    // Check for priority markers in message metadata
    if priority, ok := msg.Metadata["priority"].(int); ok {
        return MessagePriority(priority)
    }

    // System messages get highest priority
    if msg.Role == "system" {
        return PrioritySystem
    }

    // Default to normal priority
    return PriorityNormal
}
```

### Optimized Hybrid Flushing Strategy

```go
// HybridFlushingStrategy implements intelligent memory management with rule-based summarization
type HybridFlushingStrategy struct {
    config       *FlushingStrategy
    summarizer   MessageSummarizer
    tokenCounter TokenCounter
}

func (h *HybridFlushingStrategy) ShouldFlush(ctx context.Context, messageCount int, estimatedTokens int, maxTokens int) (bool, error) {
    // Optimized check - use count and estimates instead of reading all messages
    if estimatedTokens == 0 {
        // Fallback to reading messages if no estimate available
        return h.shouldFlushWithFullRead(ctx, messageCount, maxTokens)
    }

    threshold := float64(maxTokens) * h.config.SummarizeThreshold
    return float64(estimatedTokens) >= threshold, nil
}

func (h *HybridFlushingStrategy) ShouldCheckFlush(ctx context.Context, messageCount int) bool {
    // Quick check based on message count to avoid expensive token counting
    return messageCount > 10 // Only check flush when we have substantial conversation
}

func (h *HybridFlushingStrategy) FlushMessages(ctx context.Context, messages []llm.Message) (*FlushResult, error) {
    // Calculate how many messages to summarize
    summarizeCount := len(messages) * h.config.SummarizeOldestPercent / 100
    if summarizeCount <= 1 {
        // Don't summarize if there's nothing meaningful to condense
        return &FlushResult{Messages: messages}, nil
    }

    // Split messages into groups
    toSummarize := messages[:summarizeCount]
    toKeep := messages[summarizeCount:]

    // Generate summary using rule-based approach for v1
    summary, err := h.summarizer.SummarizeMessages(ctx, toSummarize, h.config.SummaryTokens)
    if err != nil {
        return nil, fmt.Errorf("failed to summarize messages: %w", err)
    }

    // Check if summarization produced a meaningful result
    if summary.Content == "" {
        // Summarizer indicated no summarization was needed/possible
        return &FlushResult{Messages: messages}, nil
    }

    // Combine summary with kept messages
    result := []llm.Message{summary}
    result = append(result, toKeep...)

    return &FlushResult{
        Messages:         result,
        SummarizedCount:  summarizeCount,
        Summary:          summary,
        TokensSaved:      h.calculateTokensSaved(toSummarize, summary),
    }, nil
}

// MessageSummarizer interface with rule-based implementation for v1
type MessageSummarizer interface {
    SummarizeMessages(ctx context.Context, messages []llm.Message, maxTokens int) (llm.Message, error)
}

// RuleBasedSummarizer implements deterministic summarization for v1
type RuleBasedSummarizer struct {
    tokenCounter TokenCounter
}

func (r *RuleBasedSummarizer) SummarizeMessages(ctx context.Context, messages []llm.Message, maxTokens int) (llm.Message, error) {
    if len(messages) < 3 {
        // Not enough messages to apply N-recent summarization strategy
        // Return empty message and nil error to indicate no summarization needed
        return llm.Message{}, nil
    }

    // Rule-based strategy: combine first message and N most recent messages
    // Keep first message and last 2 messages as per PRD description
    const keepLastN = 2
    firstMsg := messages[0]
    lastNMsgs := messages[len(messages)-keepLastN:]

    var contentBuilder strings.Builder

    // Always include first message (usually contains important context)
    contentBuilder.WriteString(fmt.Sprintf("Initial context: %s", firstMsg.Content))

    // Add summary of truncated middle section
    truncatedCount := len(messages) - 1 - keepLastN
    if truncatedCount > 0 {
        contentBuilder.WriteString(fmt.Sprintf(" ... [%d messages exchanged] ... ", truncatedCount))
    }

    // Include N most recent messages for continuity
    contentBuilder.WriteString("Recent context: ")
    for i, msg := range lastNMsgs {
        if i > 0 {
            contentBuilder.WriteString(" ")
        }
        contentBuilder.WriteString(fmt.Sprintf("%s: %s", msg.Role, msg.Content))
    }

    summaryContent := contentBuilder.String()

    // Truncate to maxTokens if necessary (simplified token estimation)
    if r.tokenCounter != nil {
        estimatedTokens := r.estimateTokens(summaryContent)
        if estimatedTokens > maxTokens {
            // Simple truncation - real implementation would be more sophisticated
            targetLength := len(summaryContent) * maxTokens / estimatedTokens
            if targetLength < len(summaryContent) {
                summaryContent = summaryContent[:targetLength] + "..."
            }
        }
    }

    summary := llm.Message{
        Role:    "system",
        Content: fmt.Sprintf("CONVERSATION_SUMMARY: %s", summaryContent),
    }

    return summary, nil
}

func (r *RuleBasedSummarizer) estimateTokens(content string) int {
    // Simple estimation: roughly 4 characters per token
    return len(content) / 4
}

type FlushResult struct {
    Messages         []llm.Message `json:"messages"`
    SummarizedCount  int           `json:"summarized_count"`
    Summary          llm.Message   `json:"summary"`
    TokensSaved      int           `json:"tokens_saved"`
}
```

### Memory Operations with Existing Infrastructure

**Reused Components**:

- **cache.LockManager** from `engine/infra/cache` for distributed locking (already uses Redlock algorithm)
- **Temporal Activities** for async operations instead of custom async implementations
- **tiktoken-go** for accurate token counting (only new library needed)

```go
// MemoryInstance wraps memory operations with distributed locking
type MemoryInstance struct {
    store         MemoryStore
    lockManager   cache.LockManager     // Reuse existing LockManager
    key           string
    resource      *MemoryResource
    tokenMgr      *TokenMemoryManager   // Uses tiktoken-go
    flushStrategy *HybridFlushingStrategy
    temporalClient client.Client        // For scheduling flush activities
}

func (m *MemoryInstance) Append(ctx context.Context, msg llm.Message) error {
    // Using existing LockManager for distributed locking
    lock, err := m.lockManager.Acquire(ctx, fmt.Sprintf("memory:%s", m.key), 30*time.Second)
    if err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer lock.Release(ctx)

    // Append message
    if err := m.store.AppendMessage(ctx, m.key, msg); err != nil {
        return fmt.Errorf("failed to append message: %w", err)
    }

    // Optimized flush check - use count instead of reading all messages
    messageCount, err := m.store.CountMessages(ctx, m.key)
    if err != nil {
        return fmt.Errorf("failed to get message count for flush check: %w", err)
    }

    // Quick check if flush might be needed
    if m.flushStrategy.ShouldCheckFlush(ctx, messageCount) {
        // Schedule flush via Temporal activity
        wo := client.StartWorkflowOptions{
            ID:        fmt.Sprintf("memory-flush-%s-%d", m.key, time.Now().Unix()),
            TaskQueue: "memory-operations",
        }

        _, err := m.temporalClient.ExecuteWorkflow(ctx, wo, "MemoryFlushWorkflow", m.key, m.resource.ID)
        if err != nil {
            // Log error but don't fail the append operation
            log.Error("failed to schedule flush workflow", "error", err)
        }
    }

    return nil
}

func (m *AsyncSafeMemoryInstance) performFlushAsync(ctx context.Context, messages []llm.Message) error {
    // Apply token-based FIFO eviction
    evictedMessages, err := m.tokenMgr.EvictMessages(ctx, messages)
    if err != nil {
        return fmt.Errorf("token eviction failed: %w", err)
    }
    messages = evictedMessages

    // Apply hybrid flushing strategy
    flushResult, err := m.flushStrategy.FlushMessages(ctx, messages)
    if err != nil {
        return fmt.Errorf("hybrid flush failed: %w", err)
    }

    // Store the flushed messages back
    return m.store.ReplaceMessagesAsync(ctx, m.key, flushResult.Messages)
}

// MemoryFlushActivity processes memory flush tasks as a Temporal activity
type MemoryFlushActivity struct {
    manager     *MemoryManager
    lockManager cache.LockManager
}

func (a *MemoryFlushActivity) FlushMemory(ctx context.Context, key string, resourceID string) error {
    // Acquire lock for flush operation
    lock, err := a.lockManager.Acquire(ctx, fmt.Sprintf("memory-flush:%s", key), 5*time.Minute)
    if err != nil {
        return temporal.NewApplicationError("failed to acquire flush lock", "LOCK_FAILED", err)
    }
    defer lock.Release(ctx)

    // Get memory instance and perform flush
    instance, err := a.manager.GetInstance(ctx, resourceID, key)
    if err != nil {
        return fmt.Errorf("failed to get memory instance: %w", err)
    }

    // Read messages and perform flush
    messages, err := instance.store.ReadMessages(ctx, key)
    if err != nil {
        return fmt.Errorf("failed to read messages for flush: %w", err)
    }

    // Apply token eviction and hybrid flushing
    return instance.performFlush(ctx, messages)
}

// Register activity with Temporal worker
func RegisterMemoryActivities(w worker.Worker, manager *MemoryManager, lockManager cache.LockManager) {
    activity := &MemoryFlushActivity{
        manager:     manager,
        lockManager: lockManager,
    }
    w.RegisterActivity(activity.FlushMemory)
}
```

### Minimal Agent Integration Changes

```go
// agent/runtime.go - Minimal changes to existing agent runtime
type Runtime struct {
    // Existing fields...
    memory Memory // Add memory interface
}

// Small update to NewRuntime to accept memory
func NewRuntime(config *Config, memory Memory) *Runtime {
    return &Runtime{
        // Existing initialization...
        memory: memory,
    }
}

// agent/router.go - Add memory resolution to existing router
func (r *Router) resolveMemory(ctx context.Context, agentConfig *Config) (Memory, error) {
    // Use existing ConfigurationResolver
    resolver := &ConfigurationResolver{
        registry: r.autoloadRegistry, // Reuse existing registry
    }

    memoryRefs, err := resolver.ResolveMemoryConfig(agentConfig)
    if err != nil {
        return nil, err
    }

    if len(memoryRefs) == 0 {
        return nil, nil // No memory configured
    }

    // Get memory from manager
    return r.memoryManager.GetMemory(ctx, memoryRefs[0])
}
```

### Minimal Orchestrator Integration Changes

```go
// llm/orchestrator.go - Minimal changes to use memory
type Orchestrator struct {
    // Existing fields...
    memoryActivity MemoryActivity // Add memory activity client
}

// Small update to prompt building to include memory
func (o *Orchestrator) buildPrompt(ctx context.Context, agent *agent.Runtime) (string, error) {
    var prompt strings.Builder

    // Existing prompt building...

    // Add memory context if available
    if agent.memory != nil {
        // Use Temporal activity for async read
        var messages []llm.Message
        err := workflow.ExecuteActivity(ctx, o.memoryActivity.ReadMemory, agent.memory.Key()).Get(ctx, &messages)
        if err != nil {
            log.Warn("Failed to read memory", "error", err)
            // Continue without memory - non-blocking
        } else if len(messages) > 0 {
            prompt.WriteString("\n\nConversation History:\n")
            for _, msg := range messages {
                prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
            }
        }
    }

    return prompt.String(), nil
}

// After getting response, save to memory
func (o *Orchestrator) processResponse(ctx context.Context, agent *agent.Runtime, response llm.Message) error {
    // Existing response processing...

    // Save to memory if available (non-blocking)
    if agent.memory != nil {
        // Use Temporal activity for async append
        err := workflow.ExecuteActivity(ctx, o.memoryActivity.AppendMemory, agent.memory.Key(), response).Get(ctx, nil)
        if err != nil {
            log.Warn("Failed to save to memory", "error", err)
            // Continue - memory failure doesn't block agent
        }
    }

    return nil
}
```

### Default Configurations

```go
// Default configurations applied when not specified
var DefaultFlushingStrategy = &FlushingStrategy{
    Type:                   "hybrid_summary",
    SummarizeThreshold:     0.8,  // 80%
    SummaryTokens:          500,
    SummarizeOldestPercent: 30,   // 30%
}

var DefaultTokenAllocation = &TokenAllocation{
    ShortTerm: 0.7,  // 70%
    LongTerm:  0.2,  // 20%
    System:    0.1,  // 10%
}

// Applied when no priority blocks are configured
var DefaultPriorityBehavior = "standard_fifo_eviction"

// MemoryCleanupManager handles resource cleanup and expiration
type MemoryCleanupManager struct {
    store       MemoryStore
    lockManager cache.LockManager
    temporal    client.Client
}

// StartCleanupWorker runs periodic cleanup tasks
func (m *MemoryCleanupManager) StartCleanupWorker(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := m.performCleanup(ctx); err != nil {
                log.Error("Cleanup failed", "error", err)
            }
        }
    }
}

func (m *MemoryCleanupManager) performCleanup(ctx context.Context) error {
    // Schedule cleanup via Temporal for reliability
    wo := client.StartWorkflowOptions{
        ID:        fmt.Sprintf("memory-cleanup-%d", time.Now().Unix()),
        TaskQueue: "memory-operations",
    }

    _, err := m.temporal.ExecuteWorkflow(ctx, wo, "MemoryCleanupWorkflow")
    return err
}

// MemoryCleanupActivity performs actual cleanup as a Temporal activity
type MemoryCleanupActivity struct {
    store       MemoryStore
    lockManager cache.LockManager
}

func (a *MemoryCleanupActivity) CleanupExpiredMemories(ctx context.Context) error {
    // This would scan Redis for expired memory keys and clean them up
    // Implementation depends on Redis key patterns used

    pattern := "compozy:*:memory:*"
    keys, err := a.store.ScanKeys(ctx, pattern)
    if err != nil {
        return fmt.Errorf("failed to scan keys: %w", err)
    }

    cleaned := 0
    for _, key := range keys {
        // Check if key has TTL set and is close to expiration
        ttl, err := a.store.GetTTL(ctx, key)
        if err != nil {
            log.Warn("Failed to get TTL", "key", key, "error", err)
            continue
        }

        // If TTL is negative, key has no expiration - check last access time
        if ttl < 0 {
            lastAccess, err := a.store.GetLastAccess(ctx, key)
            if err != nil {
                continue
            }

            // Clean up memories not accessed in 30 days
            if time.Since(lastAccess) > 30*24*time.Hour {
                if err := a.store.Delete(ctx, key); err == nil {
                    cleaned++
                }
            }
        }
    }

    log.Info("Memory cleanup completed", "cleaned", cleaned)
    return nil
}

// Key sanitization rules for Redis compatibility and security
func SanitizeMemoryKey(userKey string, projectID string) string {
    // Character whitelist: [a-zA-Z0-9-_.:]
    sanitized := regexp.MustCompile(`[^a-zA-Z0-9\-_\.:]+`).ReplaceAllString(userKey, "_")

    // Limit length to 512 characters
    if len(sanitized) > 512 {
        sanitized = sanitized[:512]
    }

    // Apply automatic namespacing by project
    return fmt.Sprintf("compozy:%s:memory:%s", projectID, sanitized)
}
```

### API Endpoints

_No new HTTP endpoints required_. Memory metrics will be exposed via the existing Prometheus middleware with enhanced metrics for priorities and flushing.

---

## Integration Points

| Integration | Purpose                        | Approach                                                                                        |
| ----------- | ------------------------------ | ----------------------------------------------------------------------------------------------- |
| Redis       | Message persistence            | Use existing `infra/store` Redis pool with `memory:` namespace prefix                           |
| LockManager | Distributed locking            | Use existing `cache.LockManager` for all memory locking needs                                   |
| Autoload    | Resource registration          | Extend existing `autoload.ConfigRegistry` to register memory resources                          |
| Temporal    | Async operations               | Use Temporal activities for async memory operations (append, read, flush)                       |
| tplengine   | Key template evaluation        | Injected into `MemoryManager` for configuration resolution                                      |
| LLM Models  | Token counting & summarization | Integrate with existing model registry for token calculations and rule-based summary generation |

---

## Impact Analysis

| Affected Component        | Impact Type           | Description & Risk                                     | Action                                          |
| ------------------------- | --------------------- | ------------------------------------------------------ | ----------------------------------------------- |
| `engine/agent`            | API Change (internal) | Agents now accept enhanced memory config patterns      | Update constructor, add config resolver & tests |
| `engine/llm/orchestrator` | Code Change           | Must load history from `Memory` interface via activity | Inject dependency, use Temporal activities      |
| `engine/worker`           | New Activities        | Add memory-related Temporal activities                 | Create new activities, register with worker     |
| `engine/autoload`         | Extension             | Registry extended to handle memory resources           | Add memory resource type to registry            |
| Redis cluster             | Performance           | Additional operations for memory storage               | Monitor ops/sec, add TTL, capacity planning     |
| Project Configuration     | Schema Change         | Enhanced memory resource schema with new fields        | Update validation, migration guide              |

Risk level is **Low** – leveraging existing infrastructure minimizes integration complexity.

---

## Testing Approach

### Unit

- `MemoryInstance.Append/Read/Len` with in-memory fake store and mock LockManager
- Token-based FIFO eviction with tiktoken-go integration
- Hybrid flushing strategy with rule-based summarization and different message distributions
- Configuration resolution for all three complexity levels with correct YAML parsing
- Key template evaluation edge cases with sanitization
- Read-only enforcement returns errors (not panics)
- LockManager integration with existing cache.Lock interface
- Temporal activity tests for memory operations
- Optimized flush checking with message count-based triggers

### Integration

- Redis backed tests in `test/integration/memory` behind build tag `integration`
- End-to-end agent workflow using memory with all features enabled via Temporal
- Multi-agent concurrent access with token management and flushing
- Configuration migration testing for all three configuration patterns
- Temporal activity integration tests with memory operations
- Lock contention testing with existing LockManager
- ConfigRegistry integration tests for memory resources

### Performance

- Memory operation latency benchmarks via Temporal activities
- Token counting performance with tiktoken-go
- Hybrid flushing performance impact with rule-based summarization
- Lock contention analysis under concurrent load with LockManager
- Temporal activity throughput benchmarks
- Message count vs. full read optimization validation

Coverage goals: 85% for `engine/memory` with focus on configuration resolution and async operations.

---

## Development Sequencing

### Phase 1: Memory Domain Foundation (Reuse Existing Infrastructure)

1. **Core Interfaces**  
   a. Define `Memory` and `MemoryStore` interfaces
   b. Implement `redisStore` using existing `engine/infra/store` Redis pool
   c. Create token management and flushing strategy components with rule-based summarization
   d. Integrate tiktoken-go for accurate token counting (only new dependency)

### Phase 2: Infrastructure Integration (Extend, Don't Duplicate)

1. **Registry Extension**  
   a. Extend existing `autoload.ConfigRegistry` to support memory resource type
   b. Create MemoryResourceLoader that registers with existing ConfigRegistry
2. **Manager Implementation**  
   a. Implement MemoryManager that uses existing `cache.LockManager`
   b. Integrate with existing `tplengine` for key template evaluation
3. **Async Operations**  
   a. Create Temporal activities for async memory operations (follow existing patterns)
   b. Register activities with existing Temporal worker infrastructure

### Phase 3: Configuration Resolution System (Leverage Existing Patterns)

1. **Configuration System**  
   a. Implement three-tier configuration resolver with proper YAML parsing
   b. Add enhanced data models for token allocation and flushing strategies
   c. Create smart defaults and configuration migration logic
   d. Add comprehensive unit tests for all configuration patterns

### Phase 4: Memory Implementation (Integrate with Existing Infrastructure)

1. **Core Memory Features**  
   a. Integrate token-based FIFO management with tiktoken-go into `MemoryInstance`
   b. Implement hybrid flushing strategy with rule-based summarization
   c. Use existing `cache.LockManager` for distributed locking
   d. Create diagnostic and health monitoring capabilities
2. **Optimizations**  
   a. Implement circuit breaker using existing pattern from `dispatcher.go`
   b. Implement optimized flush checking with existing Temporal workflows
   c. Add priority-based eviction with system message protection

### Phase 5: Agent & Orchestrator Integration (Minimal Changes)

1. **Integration Points**  
   a. Extend agent router to resolve enhanced memory configurations
   b. Update LLM orchestrator to use memory via Temporal activities
   c. Pass `Memory` handle with token management and flushing support
   d. Proper error handling using existing `core.NewError` patterns

### Phase 6: Documentation & Monitoring (Extend Existing Systems)

1. **Documentation** – Complete documentation with examples for all configuration levels, Temporal patterns, and cluster deployment considerations
2. **Monitoring** – Extend existing Grafana dashboard and metrics collection

---

## Monitoring & Observability

Enhanced Metrics (Prometheus):

- `memory_messages_total{memory_id}` – total messages appended
- `memory_trim_total{memory_id, strategy}` – total trim operations by strategy (fifo/hybrid)
- `memory_flush_total{memory_id, type}` – total flush operations (summary/eviction)
- `memory_operation_latency_seconds{operation, memory_id}` – operation latency distribution
- `memory_tokens_total{memory_id}` – current token count in memory
- `memory_tokens_saved_total{memory_id, strategy}` – tokens saved through flushing strategies
- `memory_temporal_activity_total{activity, status}` – Temporal activity metrics
- `memory_config_resolution_total{pattern}` – configuration pattern usage (simple/multi/advanced)
- `memory_flush_optimization_hits_total{memory_id}` – count-based flush check optimizations

**Infrastructure Reuse Notes**:

- Lock metrics are already tracked by existing LockManager implementation
- Temporal activity metrics are automatically collected by existing Temporal infrastructure
- Redis metrics are already monitored via existing monitoring stack

Logs: structured via `pkg/logger` with async operation tracing (reuses existing logging infrastructure).  
Grafana: memory metrics will be added to existing `compozy-monitoring.json` dashboard rather than creating a separate dashboard.

---

## Technical Considerations

### Key Decisions

- **Reuse existing infrastructure** maximizes code reuse and minimizes new dependencies
- **Temporal activities for async** leverages existing async patterns instead of custom implementations
- **Existing LockManager** provides proven distributed locking without new libraries
- **Extend ConfigRegistry** rather than creating new registry reduces code duplication
- **Token-based FIFO management** with tiktoken-go ensures accurate token counting and predictable eviction behavior
- **Hybrid flushing with rule-based summarization** maintains context continuity while managing token budgets effectively
- **Three-tier configuration system** provides progressive complexity from simple to advanced use cases
- **Optimized flush checking** uses message count estimates to avoid performance bottlenecks
- **Rule-based summarization for v1** provides deterministic, cost-effective context preservation

### Known Risks

- **Temporal activity complexity** → mitigate with comprehensive testing and clear activity patterns
- **Configuration confusion** with multiple patterns → mitigate with clear documentation and validation messages
- **Lock contention** under high concurrency → mitigate with existing LockManager retry mechanisms and proven Redlock algorithm
- **Redis traffic increase** from memory features → monitor with metrics and implement optimized flush checking
- **Token counting accuracy** → tiktoken-go provides OpenAI-compatible counting but may need updates for new models

### Risk Mitigation Through Infrastructure Reuse

By reusing existing infrastructure components, we've significantly reduced implementation risks:

- **No new Redis setup** - leverages battle-tested `infra/store` configuration
- **No new async patterns** - uses proven Temporal activity patterns from existing workers
- **No new locking implementation** - relies on existing LockManager with Redlock algorithm
- **No new registry system** - extends existing ConfigRegistry with established patterns

### Standards Compliance

- Follows Clean Architecture; `engine/memory` domain maintains separation of concerns
- Maximally reuses existing infrastructure (ConfigRegistry, LockManager, Redis pool, Temporal)
- Uses required libraries (`go-redis/v9`, `go-playground/validator/v10`)
- Error handling via wrapped errors with context (no panics)
- Testing strategy aligns with `testing-standards.mdc`
- Distributed design supports multi-replica deployments using existing patterns
- Progressive complexity supports both simple and advanced use cases
- Key sanitization ensures Redis compatibility and multi-tenant security

### Library Dependencies

The implementation requires only ONE additional Go library:

- **tiktoken-go** (`github.com/pkoukk/tiktoken-go`) - Accurate token counting for OpenAI models

All other functionality leverages existing infrastructure:

- Distributed locking: existing `cache.LockManager`
- Async operations: existing Temporal infrastructure
- Redis operations: existing `infra/store` Redis pool
- Configuration management: existing `autoload.ConfigRegistry`

---
