---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>flush_strategies</dependencies>
</task_context>

# Task 20.0: Implement Eviction Policies

## Overview

Implement eviction policies to complement the flush strategies from Task 19.0. Create FIFO, LRU, and priority-based eviction policies that can be used independently or in combination with flush strategies, leveraging the same library foundations.

## Subtasks

- [ ] 20.1 Implement FIFO eviction policy
- [ ] 20.2 Implement LRU eviction policy using library patterns
- [ ] 20.3 Implement priority-based eviction policy
- [ ] 20.4 Integrate eviction policies with existing instance management
- [ ] 20.5 Add eviction policy factory and registration
- [ ] 20.6 Add comprehensive tests for each eviction policy

## Implementation Details

### FIFO Eviction Policy

```go
// engine/memory/instance/eviction/fifo_policy.go
package eviction

import (
    "github.com/compozy/compozy/engine/llm"
    "github.com/compozy/compozy/engine/memory/instance"
)

type FIFOEvictionPolicy struct {
    name string
}

func NewFIFOEvictionPolicy() *FIFOEvictionPolicy {
    return &FIFOEvictionPolicy{
        name: "fifo",
    }
}

func (p *FIFOEvictionPolicy) SelectMessagesToEvict(
    messages []llm.Message,
    targetCount int,
) []llm.Message {
    if len(messages) <= targetCount {
        return nil // No eviction needed
    }

    evictCount := len(messages) - targetCount
    return messages[:evictCount] // Remove oldest messages first
}

func (p *FIFOEvictionPolicy) GetType() string {
    return p.name
}
```

### LRU Eviction Policy

```go
// engine/memory/instance/eviction/lru_policy.go
package eviction

import (
    "sync"
    "time"

    "github.com/compozy/compozy/engine/llm"
    "github.com/compozy/compozy/engine/memory/instance"
)

type LRUEvictionPolicy struct {
    accessTimes sync.Map // messageID -> last access time
    name        string
}

func NewLRUEvictionPolicy() *LRUEvictionPolicy {
    return &LRUEvictionPolicy{
        name: "lru",
    }
}

func (p *LRUEvictionPolicy) SelectMessagesToEvict(
    messages []llm.Message,
    targetCount int,
) []llm.Message {
    if len(messages) <= targetCount {
        return nil
    }

    // Sort messages by access time (least recently used first)
    messageAccess := make([]messageWithAccess, len(messages))
    for i, msg := range messages {
        accessTime := p.getLastAccessTime(msg)
        messageAccess[i] = messageWithAccess{
            message:    msg,
            accessTime: accessTime,
            index:      i,
        }
    }

    // Sort by access time (oldest first)
    sort.Slice(messageAccess, func(i, j int) bool {
        return messageAccess[i].accessTime.Before(messageAccess[j].accessTime)
    })

    evictCount := len(messages) - targetCount
    evicted := make([]llm.Message, evictCount)
    for i := 0; i < evictCount; i++ {
        evicted[i] = messageAccess[i].message
    }

    return evicted
}

type messageWithAccess struct {
    message    llm.Message
    accessTime time.Time
    index      int
}

func (p *LRUEvictionPolicy) getLastAccessTime(msg llm.Message) time.Time {
    msgID := p.generateMessageID(msg)
    if accessTime, ok := p.accessTimes.Load(msgID); ok {
        return accessTime.(time.Time)
    }
    return time.Time{} // Never accessed, evict first
}

func (p *LRUEvictionPolicy) UpdateAccess(msg llm.Message) {
    msgID := p.generateMessageID(msg)
    p.accessTimes.Store(msgID, time.Now())
}
```

### Priority-Based Eviction Policy

```go
// engine/memory/instance/eviction/priority_policy.go
package eviction

import (
    "sort"

    "github.com/compozy/compozy/engine/llm"
    "github.com/compozy/compozy/engine/memory/instance"
    "github.com/compozy/compozy/engine/memory/token_manager"
)

type PriorityEvictionPolicy struct {
    name string
}

func NewPriorityEvictionPolicy() *PriorityEvictionPolicy {
    return &PriorityEvictionPolicy{
        name: "priority",
    }
}

func (p *PriorityEvictionPolicy) SelectMessagesToEvict(
    messages []llm.Message,
    targetCount int,
) []llm.Message {
    if len(messages) <= targetCount {
        return nil
    }

    // Convert to priority messages
    priorityMessages := make([]token_manager.MessageWithPriorityAndTokens, len(messages))
    for i, msg := range messages {
        priority := p.extractPriority(msg)
        priorityMessages[i] = token_manager.MessageWithPriorityAndTokens{
            MessageWithTokens: core.MessageWithTokens{
                Message:    msg,
                TokenCount: p.estimateTokens(msg),
            },
            Priority:      priority,
            OriginalIndex: i,
        }
    }

    // Sort by eviction preference (lowest priority first, then oldest)
    sort.SliceStable(priorityMessages, func(i, j int) bool {
        if priorityMessages[i].Priority != priorityMessages[j].Priority {
            return priorityMessages[i].Priority > priorityMessages[j].Priority // Higher number = lower priority
        }
        return priorityMessages[i].OriginalIndex < priorityMessages[j].OriginalIndex
    })

    evictCount := len(messages) - targetCount
    evicted := make([]llm.Message, evictCount)
    for i := 0; i < evictCount; i++ {
        evicted[i] = priorityMessages[i].MessageWithTokens.Message.(llm.Message)
    }

    return evicted
}

func (p *PriorityEvictionPolicy) extractPriority(msg llm.Message) int {
    // Priority extraction logic based on message role and metadata
    switch msg.Role {
    case "system":
        return 0 // Highest priority - never evict
    case "assistant":
        return 1 // High priority
    case "user":
        return 2 // Medium priority
    case "tool":
        return 3 // Lower priority
    default:
        return 4 // Lowest priority
    }
}
```

### Eviction Policy Factory

```go
// engine/memory/instance/eviction/factory.go
package eviction

import (
    "fmt"

    "github.com/compozy/compozy/engine/memory/instance"
)

type PolicyFactory struct {
    policies map[string]func() instance.EvictionPolicy
}

func NewPolicyFactory() *PolicyFactory {
    factory := &PolicyFactory{
        policies: make(map[string]func() instance.EvictionPolicy),
    }

    // Register built-in policies
    factory.Register("fifo", func() instance.EvictionPolicy {
        return NewFIFOEvictionPolicy()
    })

    factory.Register("lru", func() instance.EvictionPolicy {
        return NewLRUEvictionPolicy()
    })

    factory.Register("priority", func() instance.EvictionPolicy {
        return NewPriorityEvictionPolicy()
    })

    return factory
}

func (f *PolicyFactory) Register(name string, creator func() instance.EvictionPolicy) {
    f.policies[name] = creator
}

func (f *PolicyFactory) Create(policyType string) (instance.EvictionPolicy, error) {
    creator, exists := f.policies[policyType]
    if !exists {
        return nil, fmt.Errorf("unknown eviction policy type: %s", policyType)
    }

    return creator(), nil
}

func (f *PolicyFactory) ListAvailable() []string {
    policies := make([]string, 0, len(f.policies))
    for name := range f.policies {
        policies = append(policies, name)
    }
    return policies
}
```

**Key Implementation Notes:**

- Leverages patterns from library-based flush strategies
- Thread-safe access tracking for LRU policy
- Priority-based eviction respects message importance
- Factory pattern enables dynamic policy selection
- Integration with existing token management

## Success Criteria

- ✅ FIFO eviction policy removes oldest messages first
- ✅ LRU eviction policy tracks and evicts least recently used messages
- ✅ Priority-based eviction preserves high-priority messages (system, assistant)
- ✅ Eviction policies integrate seamlessly with flush strategies
- ✅ Factory enables dynamic policy selection and registration
- ✅ Comprehensive tests validate each policy's eviction behavior
- ✅ Thread-safe operation in concurrent environments
- ✅ Performance is optimized for high-frequency operations

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** implement the `EvictionPolicy` interface defined in `instance/interfaces.go`
- **MUST** use established patterns from flush strategy implementations
- **MUST** ensure thread-safe operation for concurrent access
- **MUST** include comprehensive test coverage for each policy
- **MUST** integrate with existing token management and priority systems
- **MUST** follow established factory and registration patterns
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
