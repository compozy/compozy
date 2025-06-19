# Memory as a Shared Resource – Technical Specification

**Document Version**: 1.0  
**Last Updated**: {{DATE}}  
**Author**: Tech Spec Creator Agent  
**Status**: Draft  
**Related PRD**: tasks/prd-memory/\_prd.md

---

## Executive Summary

This specification describes how to introduce **project-level, shareable conversation memory** to Compozy.  
The solution adds a new `engine/memory` domain that exposes:

1. A **Memory Registry** that loads `memory` resources from project configuration and provides runtime lookup with enhanced developer experience.
2. A **Memory Manager** that instantiates **Redis-backed, token-managed memory instances** with async operations, priority-based token management, and hybrid flushing strategies.
3. **Distributed Redis locking for cluster-wide safety** combined with async-safe instance management to guarantee safe memory operations.
4. **Enhanced Agent Configuration** supporting three complexity levels from ultra-simple to fully customizable.
5. Minimal changes to the **Agent Runtime** and **LLM Orchestrator** so agents receive a resolved `Memory` interface via dependency injection with async operations.

The design follows Clean Architecture principles, provides progressive complexity for developer experience, and respects existing project standards with async-first operations.

---

## System Architecture

### Domain Placement

| Layer              | Package / Path               | Responsibility                                                 |
| ------------------ | ---------------------------- | -------------------------------------------------------------- |
| **Domain**         | `engine/memory`              | Core memory interfaces, token eviction, priority management    |
| **Infrastructure** | `engine/infra/store`         | Redis client (re-used)                                         |
| **Application**    | `engine/memory/manager`      | Runtime instance management, locking, configuration resolution |
| **Integration**    | `engine/agent`, `engine/llm` | Resolve memory references, enforce modes, async operations     |

### Component Overview

1. **MemoryResourceLoader** – loads & validates memory resources during project bootstrap with priority and flushing configurations.
2. **MemoryRegistry** – stores loaded resources and resolves by ID.
3. **MemoryStore (Redis)** – persistence-agnostic interface with Redis implementation providing async `AppendMessage`, `ReadMessages`, `CountMessages`, `TrimMessages`, `ReplaceMessages` operations.
4. **MemoryInstance** – cluster-safe handle that uses distributed locking, priority-aware token counting, hybrid flushing, and delegates to MemoryStore.
5. **DistributedLock (Redis)** – Redis-based locking using `SET NX EX` pattern for cluster coordination with automatic refresh mechanism.
6. **MemoryManager** – factory that (a) evaluates key templates using `tplengine`, (b) creates or fetches `MemoryInstance`s, (c) enforces read-only mode, (d) resolves simplified agent configurations, (e) returns proper errors instead of panics.
7. **Agent Memory Adapter** – lightweight wrapper injected into `LLM Orchestrator` so prompts include history from resolved memory with async operations.
8. **Configuration Resolver** – handles three levels of agent memory configuration complexity with validation and smart defaults.

Data Flow (high-level):

```
Agent --> LLM Orchestrator --> MemoryManager -> MemoryInstance --> DistributedLock
                                           ^                      |        |
                                           |<---------------------+        v
                                                                       MemoryStore (Redis)
                                                                       [Priority + Hybrid Flushing]
```

---

## Implementation Design

### Core Interfaces

```go
// Memory abstracts async read/write conversation history operations.
type Memory interface {
    // Async operations (primary interface)
    AppendAsync(ctx context.Context, msg llm.Message) error
    ReadAsync(ctx context.Context) ([]llm.Message, error)
    LenAsync(ctx context.Context) (int, error)

    // Diagnostic operations
    GetTokenCountAsync(ctx context.Context) (int, error)
    GetMemoryHealthAsync(ctx context.Context) (*MemoryHealth, error)
}

// MemoryStore provides persistence-agnostic async message storage operations.
type MemoryStore interface {
    AppendMessageAsync(ctx context.Context, key string, msg llm.Message) error
    ReadMessagesAsync(ctx context.Context, key string) ([]llm.Message, error)
    CountMessagesAsync(ctx context.Context, key string) (int, error)
    TrimMessagesAsync(ctx context.Context, key string, keepCount int) error
    ReplaceMessagesAsync(ctx context.Context, key string, messages []llm.Message) error
    SetExpirationAsync(ctx context.Context, key string, ttl time.Duration) error

    // Hybrid flushing support
    SummarizeAndFlushAsync(ctx context.Context, key string, config FlushConfig) (*FlushResult, error)
}

// DistributedLock provides cluster-safe locking for memory operations.
type DistributedLock interface {
    Acquire(ctx context.Context, key string, ttl time.Duration) (LockHandle, error)
}

// LockHandle represents an acquired distributed lock with refresh capability.
type LockHandle interface {
    Release(ctx context.Context) error
    Refresh(ctx context.Context, ttl time.Duration) error
}

// MemoryHealth provides diagnostic information about memory state.
type MemoryHealth struct {
    TokenCount      int                    `json:"token_count"`
    MessageCount    int                    `json:"message_count"`
    PriorityBreakdown map[int]int          `json:"priority_breakdown,omitempty"`
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

    // Priority-based token management (optional)
    PriorityBlocks []PriorityBlock `yaml:"priority_blocks,omitempty"`

    // Token allocation ratios (optional)
    TokenAllocation *TokenAllocation `yaml:"token_allocation,omitempty"`

    // Flushing strategy (defaults to hybrid)
    FlushingStrategy *FlushingStrategy `yaml:"flushing_strategy,omitempty"`

    Persistence struct {
        Type string        `yaml:"type"` // redis
        TTL  time.Duration `yaml:"ttl"`
    } `yaml:"persistence"`
}

// PriorityBlock defines content types and their priority levels
type PriorityBlock struct {
    Priority     int      `yaml:"priority"`     // 0=critical, 1=important, 2+=optional
    ContentTypes []string `yaml:"content_types"`
    MaxTokens    int      `yaml:"max_tokens,omitempty"`
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

### Priority and Token Management Integration

```go
// PriorityMemoryManager handles token eviction with priority awareness
type PriorityMemoryManager struct {
    resource      *MemoryResource
    tokenCounter  TokenCounter
}

func (p *PriorityMemoryManager) EvictWithPriority(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
    if len(p.resource.PriorityBlocks) == 0 {
        // No priority configuration - use standard token eviction
        return p.standardEviction(ctx, messages)
    }

    // Calculate effective token limits (enforces lower of ratio-based vs fixed limits)
    effectiveLimits := p.calculateEffectiveTokenLimits()

    // Group messages by priority
    priorityGroups := p.groupMessagesByPriority(messages)

    // Evict from lowest priority first, respecting effective token limits
    for priority := p.getMaxPriority(); priority >= 2; priority-- {
        if p.isUnderTokenLimit(messages) {
            break
        }
        maxTokensForPriority := effectiveLimits[priority]
        messages = p.evictFromPriorityWithLimits(messages, priority, maxTokensForPriority)
    }

    return messages, nil
}

// calculateEffectiveTokenLimits implements the PRD rule: "enforce the lower of the two values"
func (p *PriorityMemoryManager) calculateEffectiveTokenLimits() map[int]int {
    totalTokens := p.resource.MaxTokens
    if totalTokens == 0 && p.resource.MaxRatio > 0 {
        // Calculate based on context ratio (would need model context size)
        totalTokens = int(float64(4096) * p.resource.MaxRatio) // default 4K context
    }

    effectiveLimits := make(map[int]int)

    // If no token allocation configured, use priority block max_tokens directly
    if p.resource.TokenAllocation == nil {
        for _, block := range p.resource.PriorityBlocks {
            if block.MaxTokens > 0 {
                effectiveLimits[block.Priority] = block.MaxTokens
            } else {
                // No limit specified for this priority
                effectiveLimits[block.Priority] = totalTokens
            }
        }
        return effectiveLimits
    }

    // Calculate budgets from token_allocation ratios
    allocation := p.resource.TokenAllocation
    shortTermBudget := int(float64(totalTokens) * allocation.ShortTerm)
    longTermBudget := int(float64(totalTokens) * allocation.LongTerm)
    systemBudget := int(float64(totalTokens) * allocation.System)

    // Map allocation categories to priorities (simplified mapping for example)
    allocationBudgets := map[string]int{
        "short_term": shortTermBudget,
        "long_term":  longTermBudget,
        "system":     systemBudget,
    }

    // For each priority block, enforce the lower of allocation budget vs max_tokens
    for _, block := range p.resource.PriorityBlocks {
        // Map content types to allocation categories (implementation would be more sophisticated)
        allocationCategory := p.mapContentTypesToAllocation(block.ContentTypes)
        ratioBudget := allocationBudgets[allocationCategory]

        // Enforce the lower of the two values, as per PRD requirement
        if block.MaxTokens > 0 && block.MaxTokens < ratioBudget {
            effectiveLimits[block.Priority] = block.MaxTokens
        } else {
            effectiveLimits[block.Priority] = ratioBudget
        }
    }

    return effectiveLimits
}

func (p *PriorityMemoryManager) mapContentTypesToAllocation(contentTypes []string) string {
    // Simplified mapping - real implementation would be more sophisticated
    for _, contentType := range contentTypes {
        switch contentType {
        case "system_prompt", "user_profile":
            return "system"
        case "recent_context", "important_facts":
            return "short_term"
        case "historical_messages":
            return "long_term"
        }
    }
    return "short_term" // default
}

func (p *PriorityMemoryManager) evictFromPriorityWithLimits(messages []llm.Message, priority int, maxTokensForPriority int) []llm.Message {
    // Evict from this priority level while respecting effective token limits
    return p.evictMessagesFromPriority(messages, priority, maxTokensForPriority)
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

### Optimized Async-Safe Memory Operations

```go
// AsyncSafeMemoryInstance wraps memory operations with distributed locking and optimization
type AsyncSafeMemoryInstance struct {
    store        MemoryStore
    lock         DistributedLock
    key          string
    resource     *MemoryResource
    priorityMgr  *PriorityMemoryManager
    flushStrategy *HybridFlushingStrategy
}

func (m *AsyncSafeMemoryInstance) AppendAsync(ctx context.Context, msg llm.Message) error {
    lockHandle, err := m.lock.Acquire(ctx, m.key, 30*time.Second)
    if err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer lockHandle.Release(ctx)

    // Use lock refresh for potentially long operations
    return m.withLockRefresh(ctx, lockHandle, func() error {
        // Append message
        if err := m.store.AppendMessageAsync(ctx, m.key, msg); err != nil {
            return fmt.Errorf("failed to append message: %w", err)
        }

        // Optimized flush check - use count instead of reading all messages
        messageCount, err := m.store.CountMessagesAsync(ctx, m.key)
        if err != nil {
            return fmt.Errorf("failed to get message count for flush check: %w", err)
        }

        // Quick check if flush might be needed
        if m.flushStrategy.ShouldCheckFlush(ctx, messageCount) {
            // Only read messages when likely near threshold
            messages, err := m.store.ReadMessagesAsync(ctx, m.key)
            if err != nil {
                return fmt.Errorf("failed to read messages for flush check: %w", err)
            }

            shouldFlush, err := m.flushStrategy.ShouldFlush(ctx, len(messages), 0, m.getMaxTokens())
            if err != nil {
                return fmt.Errorf("failed to check flush condition: %w", err)
            }

            if shouldFlush {
                return m.performFlushAsync(ctx, messages)
            }
        }

        return nil
    })
}

func (m *AsyncSafeMemoryInstance) performFlushAsync(ctx context.Context, messages []llm.Message) error {
    // Apply priority-based eviction if configured
    if len(m.resource.PriorityBlocks) > 0 {
        evictedMessages, err := m.priorityMgr.EvictWithPriority(ctx, messages)
        if err != nil {
            return fmt.Errorf("priority eviction failed: %w", err)
        }
        messages = evictedMessages
    }

    // Apply hybrid flushing strategy
    flushResult, err := m.flushStrategy.FlushMessages(ctx, messages)
    if err != nil {
        return fmt.Errorf("hybrid flush failed: %w", err)
    }

    // Store the flushed messages back
    return m.store.ReplaceMessagesAsync(ctx, m.key, flushResult.Messages)
}

// withLockRefresh spawns a goroutine that refreshes the lock TTL every (TTL/2) seconds
func (m *AsyncSafeMemoryInstance) withLockRefresh(ctx context.Context, lockHandle LockHandle, operation func() error) error {
    // Create a context for the refresh goroutine
    refreshCtx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Start lock refresh goroutine
    refreshDone := make(chan struct{})
    go func() {
        defer close(refreshDone)
        ticker := time.NewTicker(15 * time.Second) // refresh every 15s for 30s TTL
        defer ticker.Stop()

        for {
            select {
            case <-refreshCtx.Done():
                return
            case <-ticker.C:
                if err := lockHandle.Refresh(refreshCtx, 30*time.Second); err != nil {
                    // Log refresh failure but don't interrupt main operation
                    // The operation might complete before lock expires
                    return
                }
            }
        }
    }()

    // Execute main operation
    err := operation()

    // Cancel refresh and wait for cleanup
    cancel()
    <-refreshDone

    return err
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
| Redis       | Message persistence & locking  | Use existing `infra/store` Redis pool with `memory:` namespace prefix, async operations         |
| tplengine   | Key template evaluation        | Injected into `MemoryManager` for configuration resolution                                      |
| LLM Models  | Token counting & summarization | Integrate with existing model registry for token calculations and rule-based summary generation |

---

## Impact Analysis

| Affected Component        | Impact Type           | Description & Risk                                    | Action                                          |
| ------------------------- | --------------------- | ----------------------------------------------------- | ----------------------------------------------- |
| `engine/agent`            | API Change (internal) | Agents now accept enhanced memory config patterns     | Update constructor, add config resolver & tests |
| `engine/llm/orchestrator` | Code Change           | Must load history from async `Memory` interface       | Inject dependency, convert to async operations  |
| Redis cluster             | Performance           | Additional operations for priorities, flushing, locks | Monitor ops/sec, add TTL, capacity planning     |
| Project Configuration     | Schema Change         | Enhanced memory resource schema with new fields       | Update validation, migration guide              |

Risk level is **Medium** – async changes require careful integration, but architectural impact is contained.

---

## Testing Approach

### Unit

- `MemoryInstance.AppendAsync/ReadAsync/LenAsync` with in-memory fake store and mock distributed lock
- Priority-based token eviction with various priority configurations and token allocation constraints
- Hybrid flushing strategy with rule-based summarization and different message distributions
- Configuration resolution for all three complexity levels with correct YAML parsing
- Key template evaluation edge cases with sanitization
- Read-only enforcement returns errors (not panics)
- Distributed lock acquire/release/retry logic with refresh mechanism
- Optimized flush checking with message count-based triggers

### Integration

- Redis backed tests in `test/integration/memory` behind build tag `integration`
- End-to-end agent workflow using memory with all features enabled
- Multi-agent concurrent access with priorities and flushing
- Configuration migration testing for all three configuration patterns
- Performance testing with async operations under load
- Lock contention and refresh mechanism testing

### Performance

- Async operation latency benchmarks
- Priority processing overhead measurement with token allocation
- Hybrid flushing performance impact with rule-based summarization
- Lock contention analysis under concurrent load with refresh mechanism
- Message count vs. full read optimization validation

Coverage goals: 85% for `engine/memory` with focus on configuration resolution and async operations.

---

## Development Sequencing

1. **Enhanced Memory Domain Foundation**  
   a. Define enhanced `Memory`, `MemoryStore`, `DistributedLock` interfaces with async operations (fixed interface without contradictory methods)
   b. Implement `redisStore` with async methods in `engine/infra/store` (reuse existing pool) including `ReplaceMessagesAsync`
   c. Implement `redisDistributedLock` using `SET NX EX` pattern with lock refresh mechanism
   d. Create priority management and flushing strategy components with rule-based summarization

2. **Fixed Configuration Resolution System**  
   a. Implement corrected three-tier configuration resolver with proper YAML parsing
   b. Add enhanced data models for priority blocks and flushing strategies
   c. Create smart defaults and configuration migration logic
   d. Add comprehensive unit tests for all configuration patterns with correct mappings

3. **Enhanced Memory Implementation**  
   a. Integrate priority-based token management with token allocation constraints into `MemoryInstance.AppendAsync`
   b. Implement hybrid flushing strategy with rule-based summarization
   c. Add async-safe operations with distributed locking and refresh mechanism
   d. Create diagnostic and health monitoring capabilities
   e. Implement optimized flush checking to avoid performance bottlenecks

4. **Agent & Orchestrator Integration**  
   a. Extend agent router to resolve enhanced memory configurations correctly
   b. Update LLM orchestrator to use async memory operations
   c. Pass enhanced `Memory` handle with priority and flushing support
   d. Proper error handling throughout async integration points

5. **Advanced Features & Performance**  
   a. Implement token allocation ratios and advanced priority features with proper interaction rules
   b. Add performance optimizations for async operations
   c. Create comprehensive monitoring and observability
   d. Performance benchmarking and optimization

6. **Documentation & Examples** – Complete documentation with examples for all configuration levels, async patterns, and cluster deployment considerations.

---

## Monitoring & Observability

Enhanced Metrics (Prometheus):

- `memory_messages_total{memory_id, priority}` – total messages appended by priority level
- `memory_trim_total{memory_id, strategy}` – total trim operations by strategy (priority/hybrid)
- `memory_flush_total{memory_id, type}` – total flush operations (summary/eviction)
- `memory_operation_latency_seconds{operation, memory_id}` – async operation latency distribution
- `memory_lock_acquire_total{memory_id}` – distributed lock acquisitions
- `memory_lock_contention_total{memory_id}` – lock acquisition failures/retries
- `memory_lock_duration_seconds{memory_id}` – lock hold time distribution
- `memory_lock_refresh_total{memory_id}` – lock refresh operations
- `memory_tokens_saved_total{memory_id, strategy}` – tokens saved through flushing strategies
- `memory_priority_distribution{memory_id, priority}` – message distribution across priority levels
- `memory_config_resolution_total{pattern}` – configuration pattern usage (simple/multi/advanced)
- `memory_flush_optimization_hits_total{memory_id}` – count-based flush check optimizations

Logs: structured via `pkg/logger` with async operation tracing.  
Grafana: enhanced `memory` dashboard with priority and flushing visualizations in `cluster/grafana/dashboards`.

---

## Technical Considerations

### Key Decisions

- **Async-first operations** eliminate blocking during memory access, improving concurrent agent performance.
- **Priority-based token management** with token allocation constraints ensures critical content preservation while maintaining intelligent eviction.
- **Hybrid flushing with rule-based summarization** maintains context continuity while managing token budgets effectively and ensuring predictable behavior.
- **Corrected three-tier configuration system** provides progressive complexity from simple to advanced use cases with proper YAML parsing.
- **Distributed Redis locks with refresh mechanism** ensure cluster-safe operations with automatic TTL extension for long operations.
- **Optimized flush checking** uses message count estimates to avoid performance bottlenecks on every append.
- **Rule-based summarization for v1** provides deterministic, cost-effective context preservation.

### Known Risks

- **Async complexity** in integration points → mitigate with comprehensive testing and clear async patterns.
- **Priority processing overhead** → mitigate with efficient grouping algorithms and optional features.
- **Configuration confusion** with multiple patterns → mitigate with corrected parsing logic, clear documentation and validation messages.
- **Lock contention** under high concurrency → mitigate with exponential backoff, reasonable lock TTLs, and refresh mechanism.
- **Redis traffic increase** from enhanced features → monitor with metrics and implement optimized flush checking for high-volume scenarios.
- **Token allocation conflicts** with priority blocks → resolved by defining clear precedence rules (lower of ratio-based vs. fixed limits).

### Standards Compliance

- Follows Clean Architecture; enhanced `engine/memory` domain maintains separation of concerns.
- Uses required libraries (`go-redis/v9`, `go-playground/validator/v10`) with async patterns.
- Error handling via wrapped errors with context (no panics).
- Testing strategy aligns with `testing-standards.mdc` with async operation coverage.
- Distributed design supports multi-replica deployments with enhanced concurrent safety.
- Progressive complexity supports both simple and advanced use cases without overwhelming developers.
- Key sanitization ensures Redis compatibility and multi-tenant security.

---
