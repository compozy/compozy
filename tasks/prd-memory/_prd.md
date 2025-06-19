# Product Requirements Document (PRD): Memory as a Shared Resource

## Overview

Compozy's current LLM orchestration engine operates in a stateless manner, treating each agent interaction as an isolated request without conversation history or context management. This limitation prevents developers from building sophisticated conversational workflows where agents need to maintain and share context across multiple interactions.

The Memory as a Shared Resource feature transforms memory from an agent-specific configuration to a first-class resource in Compozy's architecture. By defining memories at the project level and allowing agents to reference them by ID, this enhancement enables powerful memory sharing patterns across agents and workflows. This approach provides greater flexibility for complex multi-agent systems while maintaining the simplicity of configuration.

## Goals

- **Enable Memory Sharing**: Allow multiple agents to share conversation context and knowledge through referenced memory resources
- **Maintain Flexibility**: Support diverse memory patterns from agent-specific to globally shared memories
- **Optimize Token Usage**: Implement intelligent context window management with priority-based token allocation and hybrid flushing strategies
- **Provide Resource Management**: Treat memory as a managed resource with lifecycle, versioning, and configuration
- **Simplify Complex Workflows**: Enable sophisticated multi-agent workflows with shared context without complex data passing
- **Enhance Developer Experience**: Provide multiple configuration complexity levels from ultra-simple to fully customizable

## User Stories

### Primary User Stories

1. **As a developer**, I want to define memory resources at the project level so that I can reuse memory configurations across multiple agents without duplication.

2. **As a developer**, I want my customer support intake agent and resolution agent to share the same conversation memory so that customers don't have to repeat information when transferred between agents.

3. **As a developer**, I want to configure different access modes (read-only, read-write) for memories in agents so that I can control which agents can modify shared state.

4. **As a developer**, I want to enable memory for an agent with a single configuration line so that I can quickly prototype without complex setup.

### Secondary User Stories

5. **As a developer**, I want to create specialized memory resources (e.g., "user-preferences", "session-context", "knowledge-base") that different agents can access as needed.

6. **As a developer**, I want memory resources to persist independently of agent lifecycles so that context can survive across workflow executions and agent restarts.

7. **As a developer**, I want to organize memory resources in files similar to how agents and tools are organized for better project structure.

8. **As a developer**, I want critical context (like system prompts and user profiles) to be preserved even when token limits are reached, while less important historical messages are automatically summarized.

## Core Features

### 1. Memory Resource Definition

**What it does**: Establishes memory as a first-class resource type in Compozy, defined at the project level with unique IDs and configurations.

**Why it's important**: Enables reusability, sharing, and centralized management of memory configurations across the entire project.

**How it works**:

- Memory resources defined in project config or separate files
- Each memory has a unique ID and description
- Supports all configuration options from inline memory
- Resources are loaded and validated at project initialization

**Functional Requirements**:

1. The system must support memory resource definitions in project configuration
2. The system must support memory definitions in separate YAML files (e.g., `memories/customer-support.yaml`)
3. Each memory resource must have a unique ID within the project
4. Each memory resource must have a description field for documentation purposes
5. Memory resources must support all configuration options (type, persistence, privacy, etc.)
6. The system must validate memory resource configurations at load time

### 2. Enhanced Developer Experience with Simplified Configuration

**What it does**: Provides multiple levels of configuration complexity to match developer needs, from ultra-simple single-line setup to fully customizable multi-memory configurations.

**Why it's important**: Reduces barriers to adoption while maintaining flexibility for complex use cases.

**How it works**:

- **Level 1**: Ultra-simple single memory reference with automatic defaults
- **Level 2**: Simple multi-memory with shared key template
- **Level 3**: Full control with per-memory configuration
- Automatic validation and smart defaults for simplified configurations

**Functional Requirements**: 7. The system must support direct memory ID reference for single-memory agents 8. The system must support simplified multi-memory configuration with shared key templates 9. The system must provide smart defaults for simplified configurations (read-write mode, token-based type) 10. The system must validate that referenced memory IDs exist at configuration time 11. The system must maintain backward compatibility with full memory reference configurations

### 3. Agent Memory References

**What it does**: Allows agents to reference one or more memory resources by ID with configurable access modes and user-defined key templates.

**Why it's important**: Provides fine-grained control over how agents interact with shared memories while maintaining clear relationships. The key template is defined in the agent's memory reference to allow a single memory resource to be partitioned in multiple ways (e.g., by user ID or session ID).

**How it works**:

- Agents declare memory references in their configuration using various complexity levels
- Each reference includes the memory ID, access mode, and key template
- Multiple memories can be referenced with different modes
- Agent runtime resolves references to actual memory instances

**Functional Requirements**: 12. Agents must support multiple memory configuration patterns (simple ID, shared template, full references) 13. Each memory reference must specify an ID, mode (read-write, read-only), and key template 14. The system must validate that referenced memory IDs exist at configuration time 15. The system must enforce access modes at runtime (read-only prevents modifications) 16. The system must support agents with no memory references (stateless agents) 17. The system must evaluate key templates using workflow context to create unique memory instances

### 4. Memory Selection Strategy

**What it does**: Provides a simple, explicit mechanism for agents to reference specific memories through static configuration.

**Why it's important**: Ensures predictable behavior and clear relationships between agents and their memory resources.

**How it works**:

- Developers explicitly specify memory ID in agent configuration
- No dynamic selection - agents use only their configured memories
- Clear, deterministic memory access patterns

**Functional Requirements**: 18. The system must support explicit memory selection via agent configuration 19. The system must validate that referenced memory IDs exist at configuration time 20. The system must reject configurations with invalid memory references

### 5. Shared Memory Management with Async Operations

**What it does**: Manages concurrent access, consistency, and lifecycle of shared memory resources across multiple agents using async operations for optimal performance.

**Why it's important**: Ensures data consistency and prevents conflicts when multiple agents access the same memory while maintaining high performance through non-blocking operations.

**How it works**:

- Memory instances are created per workflow execution, based on resource definitions
- The registry acts as a factory, creating isolated instances for each workflow
- Async operations for all memory read/write operations
- Thread-safe operations for concurrent access within a workflow
- Consistent view across all agents in the same workflow
- Lifecycle scoped to workflow execution with configurable persistence

**Functional Requirements**: 21. The system must ensure thread-safe access to shared memory resources 22. The system must provide consistent memory state across agents accessing the same memory instance 23. The system must support memory initialization from persistence on first access 24. The system must use TTL as a fallback cleanup mechanism for abnormal terminations 25. The system must support user-defined memory keys for dynamic memory instance isolation 26. The system must use async operations for all memory read/write operations

### 6. Priority-Based Memory Architecture (Optional)

**What it does**: Allows memory resources to define priority levels for different types of content, ensuring critical information is preserved during token pressure.

**Why it's important**: Provides intelligent context management that preserves the most important information while gracefully handling token limits.

**How it works**:

- Memory resources can optionally define priority blocks
- Priority 0 = Critical (never evicted), Priority 1 = Important (evict last), Priority 2+ = Optional (evict first)
- Token allocation can be distributed across priority levels
- System preserves high-priority content during memory pressure

**Functional Requirements**: 27. The system must support optional priority block configuration in memory resources 28. The system must preserve priority 0 (critical) content regardless of token pressure 29. The system must evict lower priority content before higher priority content 30. The system must support optional token allocation ratios across priority levels 31. The system must provide default behavior when no priority blocks are configured

**Priority and Token Allocation Interaction**:
The `token_allocation` ratios define the overall budget for different memory types (short_term: 70%, long_term: 20%, system: 10%). The `max_tokens` within `priority_blocks` act as a further constraint within that budget, but cannot exceed it. When both are defined, the system enforces the lower of the two values (ratio-based allocation vs. fixed max_tokens) for each priority level. This ensures token budgets are respected while maintaining priority-based preservation.

### 7. Hybrid Memory Flushing Strategy

**What it does**: Implements intelligent memory management that summarizes and flushes old content while preserving summaries for context continuity using rule-based summarization.

**Why it's important**: Maintains conversation continuity while effectively managing token budgets through deterministic summarization that ensures predictable costs and performance.

**How it works**:

- Default strategy: hybrid rule-based summarization with configurable parameters
- When token limits are reached, oldest messages are summarized using deterministic rules and flushed
- Summaries are kept in memory to maintain context
- Rule-based approach combines first message and most recent messages for continuity

**Functional Requirements**: 32. The system must implement hybrid rule-based summarization as the default flushing strategy 33. The system must summarize oldest messages when token limits are exceeded using deterministic rules 34. The system must preserve summaries in memory for context continuity 35. The system must support configurable summarization parameters (trigger thresholds, summary size) 36. The system may support alternative flushing strategies for advanced use cases

**Summarization Strategy Decision**: For the initial release, the system uses a rule-based summarization strategy to ensure performance and cost predictability. This strategy combines the first message and the N most recent messages to form the summary, providing context continuity while avoiding LLM costs. LLM-based summarization will be considered for a future release.

### 8. Token-Based Memory Strategy (Enhanced)

**What it does**: Implements a token-first approach that maximizes message retention within model token limits with intelligent priority management.

**Why it's important**: Directly addresses the core problem of context overflow while maximizing useful conversation history through smart prioritization.

**How it works**:

- Tracks conversation history as an ordered list of messages with priorities
- Counts tokens for each message using model-specific tokenizers
- Uses token budget as primary constraint with priority-aware eviction
- Always preserves system prompts and critical content
- Optional message count as secondary limit

**Functional Requirements**: 37. The system must use token-based eviction as the primary strategy 38. The system must calculate token usage for the entire conversation 39. The system must preserve system prompts regardless of token pressure 40. The system must support configurable max_tokens or max_context_ratio 41. The system must support optional message_limit as a secondary constraint 42. The system must support priority-aware token eviction when priority blocks are configured

### 9. Privacy and Data Protection (Inherited from Original PRD)

**What it does**: Provides controls for handling sensitive data in conversations.

**Why it's important**: Ensures compliance with privacy regulations and protects sensitive information.

**How it works**:

- Message-level privacy flags
- Synchronous redaction before data leaves process boundaries
- Configurable redaction policies at memory resource level
- Selective persistence controls

**Functional Requirements**: 43. The system must support marking messages as non-persistable 44. The system must honor privacy flags in the persistence layer 45. The system must log when sensitive data is excluded from persistence

### 10. Simplified Design for Alpha

**What it does**: Implements a simplified, robust approach for the alpha version focusing on reliability over advanced features.

**Why it's important**: Reduces complexity, ensures stability, and provides a solid foundation for future enhancements.

**How it works**:

- Append-only memory design (no updates or deletes)
- Async operations as primary interface (no sync versions)
- Static memory selection via configuration
- Basic performance metrics (reads, writes)
- Project-level isolation for multi-tenancy

**Functional Requirements**: 46. The system must enforce append-only operations on memory 47. The system must use async operations for all memory interactions 48. The system must provide basic operational metrics 49. The system must isolate memories at the project level

## User Experience

### Developer Configuration Experience

#### Memory Resource Definition

Developers define memory resources in the project configuration:

```yaml
# In compozy.yaml or separate memory files
memories:
    - id: customer-support-context
      resource: memory
      description: "Shared conversation history for customer support workflows including intake, troubleshooting, and resolution"
      type: token_based
      max_context_ratio: 0.8

      # Optional: Priority-based token management
      priority_blocks:
          - priority: 0 # Critical - never evicted
            content_types: [system_prompt, user_profile]
            max_tokens: 500
          - priority: 1 # Important - evict last
            content_types: [recent_context]
            max_tokens: 2000
          - priority: 2 # Optional - evict first
            content_types: [historical_messages]

      # Optional: Token allocation ratios (interact with priority blocks)
      token_allocation:
          short_term: 0.7 # 70% for recent messages
          long_term: 0.2 # 20% for summaries/important context
          system: 0.1 # 10% reserved for prompts

      # Optional: Flushing strategy (defaults to rule-based hybrid)
      flushing_strategy:
          type: hybrid_summary # rule-based default strategy
          summarize_threshold: 0.8 # Summarize when 80% full
          summary_tokens: 500 # Tokens reserved for summaries
          summarize_oldest_percent: 30 # Summarize oldest 30% of messages

      persistence:
          type: redis
          ttl: "24h"
      privacy_policy:
          redact_patterns:
              - '\b\d{3}-\d{2}-\d{4}\b' # SSN

    - id: user-preferences
      resource: memory
      description: "Long-term user preferences and settings that persist across sessions"
      type: token_based
      max_tokens: 2000
      persistence:
          type: redis
          ttl: "30d"

    - id: research-findings
      resource: memory
      description: "Accumulated research findings and facts discovered during investigation workflows"
      type: token_based
      message_limit: 100
      persistence:
          type: redis
```

#### Enhanced Agent Configuration Options

**Level 1: Ultra-Simple (Single Memory)**

```yaml
agents:
    - id: simple-agent
      config:
          provider: openai
          model: gpt-4-turbo
      # Direct memory ID reference - uses smart defaults
      memory: customer-support-context
      memory_key: "support-{{ .workflow.input.conversationId }}"
      instructions: |
          You are a customer support specialist...
```

**Level 2: Simple Multi-Memory**

```yaml
agents:
    - id: multi-memory-agent
      config:
          provider: openai
          model: gpt-4-turbo
      # Enable memory feature with defaults
      memory: true
      # Array of memory IDs to access
      memories:
          - customer-support-context
          - user-preferences
      # Single key template applied to all memories
      memory_key: "support-{{ .workflow.input.conversationId }}"
      instructions: |
          You are a customer support specialist...
```

**Level 3: Full Control (Advanced)**

```yaml
agents:
    - id: advanced-agent
      config:
          provider: openai
          model: gpt-4-turbo
      # Full memory reference configuration
      memories:
          - id: customer-support-context
            mode: read-write
            key: "support-{{ .workflow.input.conversationId }}"
          - id: user-preferences
            mode: read-only
            key: "prefs-{{ .workflow.input.userId }}"
          - id: research-findings
            mode: read-write
            key: "research-{{ .workflow.id }}"
      instructions: |
          You are a customer support resolution specialist...
```

### Configuration Resolution Logic

The system resolves agent memory configuration as follows:

1. **Direct Memory ID** (`memory: "memory-id"`): Creates single memory reference with read-write mode and provided key template
2. **Simple Multi-Memory** (`memory: true` + `memories: [...]` as string array): Creates memory references for each ID with read-write mode and shared key template
3. **Advanced Configuration** (`memories: [...]` with full reference objects): Uses provided configuration as-is
4. **Validation**: All referenced memory IDs must exist in project configuration
5. **Defaults**: Mode defaults to `read-write`, key template is required for simplified configurations

**Level Detection Logic**: The system identifies configuration levels by examining the `memories` field type first. Level 2 is detected when `memories` is a string array combined with `memory: true`. Level 3 is detected when `memories` contains full reference objects. Level 1 is identified by a direct string value in the `memory` field.

### Memory Resource Files

For better organization, memories can be defined in separate files:

```yaml
# memories/customer-support.yaml
resource: memory
id: customer-support-context
description: "Shared conversation history for customer support workflows"
version: 1.0.0

type: token_based
max_context_ratio: 0.8

# Optional priority configuration
priority_blocks:
    - priority: 0
      content_types: [system_prompt, user_profile]
      max_tokens: 500
    - priority: 1
      content_types: [recent_context, important_facts]
      max_tokens: 2000

# Default rule-based hybrid flushing strategy
flushing_strategy:
    type: hybrid_summary
    summarize_threshold: 0.8
    summary_tokens: 500

persistence:
    type: redis
    ttl: "24h"
    circuit_breaker:
        timeout: "100ms"
        max_failures: 3
        reset_timeout: "30s"

privacy_policy:
    redact_patterns:
        - '\b\d{3}-\d{2}-\d{4}\b' # SSN
        - '\b\d{4}-\d{4}-\d{4}-\d{4}\b' # Credit card
```

### Template Variables for Memory Keys

The memory key templates support the following variables from the workflow context:

- `{{ .workflow.id }}` - Unique workflow execution ID
- `{{ .workflow.input.* }}` - Any input parameter passed to the workflow
- `{{ .user.id }}` - User identifier (if available)
- `{{ .session.id }}` - Session identifier (if available)
- `{{ .project.id }}` - Project identifier
- `{{ .agent.id }}` - Current agent ID
- `{{ .timestamp }}` - Current timestamp

Keys are sanitized to ensure multi-tenant safety and Redis compatibility using a character whitelist (`[a-zA-Z0-9-_.:]`), maximum length of 512 characters, and automatic namespacing by project (`compozy:{project_id}:memory:{user_defined_key}`).

**Key Benefits**:

- **Dynamic Isolation**: Same memory resource serves multiple isolated instances
- **User Control**: Developers define exactly how memory is partitioned
- **Natural Multi-tenancy**: Different conversations/users automatically separated
- **Flexible Scoping**: No predefined scopes - use any workflow context for partitioning
- **Progressive Complexity**: Start simple, add complexity as needed

### Message Flow with Shared Memory

1. Workflow starts with multiple agents configured
2. First agent (intake) receives user message
3. Memory resource is initialized (or loaded from persistence) asynchronously
4. Agent adds messages to shared memory using async operations
5. Second agent (resolution) accesses same memory resource asynchronously
6. Both agents see consistent conversation history
7. Memory persists according to resource configuration with hybrid flushing as needed

## High-Level Technical Constraints

### Required Integrations

- **Redis Integration**: Memory persistence requires Redis connectivity with circuit breaker patterns for resilience
- **Project Configuration System**: Memory resources must be loadable from project configuration files
- **Agent Runtime Integration**: Agents must be able to receive and use external memory instances
- **Template Engine**: Support for evaluating template variables in memory keys
- **Async Runtime**: All memory operations must support async/await patterns

### Performance Requirements

- **Memory Operations**: < 50ms overhead for async memory read/write operations
- **Concurrency**: Support concurrent agent access to shared memory instances with async operations
- **Token Management**: Efficient token counting and eviction for conversation history with priority awareness

### Security and Privacy

- **Multi-tenant Isolation**: Memory instances must be isolated based on user-defined keys
- **Privacy Controls**: Support for marking messages as non-persistable
- **Key Sanitization**: Memory keys must be sanitized for Redis compatibility and security

### Scalability Considerations

- **Memory Efficiency**: < 10MB overhead per shared memory instance
- **Redis Storage**: TTL-based cleanup for memory instances
- **Thread Safety**: Safe concurrent access to memory instances within workflows using async operations

## Non-Goals (Out of Scope)

- **Memory Versioning**: No support for multiple versions of same memory resource in v1
- **Access Control Lists**: No fine-grained permissions beyond read/read-write modes
- **Memory Composition**: No support for combining multiple memories into one
- **Dynamic Memory Creation**: Agents cannot create new memory resources at runtime
- **Memory Transfer**: No tools for transferring memory data between different resource definitions
- **Conflict Resolution**: No sophisticated merge strategies for concurrent modifications
- **Memory Inheritance**: No support for memory resources extending other memories
- **Cross-Project Memory**: No sharing of memory resources across different projects
- **Synchronous Operations**: No synchronous memory interface (async-only for performance)
- **LLM-Based Summarization**: Not included in v1 - rule-based summarization only

## Development Roadmap

### Phase 1: Memory Resource Framework (Week 1-2)

- Create memory resource data models and interfaces with async support
- Implement resource loader for memory definitions
- Build global memory registry
- Create memory factory for instantiation
- Add resource validation including priority and flushing configurations
- Unit tests for resource management

**Success Criteria**: Memory resources can be defined, loaded, and validated from configuration with all new features

### Phase 2: Core Memory Implementation (Week 2-3)

- Port token-based memory from original design with async operations
- Implement priority-based token management with token allocation constraints (optional feature)
- Implement hybrid flushing strategy with rule-based summarization
- Add memory sharing and singleton management
- Add thread-safe access controls with async operations
- Integration tests and performance benchmarks

**Success Criteria**: Shared memory instances work correctly with token management, priorities, and hybrid flushing

### Phase 3: Enhanced Developer Experience (Week 3-4)

- Implement simplified agent configuration patterns (3 levels) with corrected parsing logic
- Add configuration resolution logic with validation
- Extend agent config for memory references
- Add access mode enforcement
- Update orchestrator for async memory operations
- Integration tests with all configuration patterns

**Success Criteria**: All three levels of agent configuration work correctly with proper validation

### Phase 4: Persistence & Privacy (Week 4-5)

- Implement Redis storage with circuit breakers and async operations
- Add privacy controls and redaction
- Build persistence lifecycle management
- Create memory initialization from storage
- Add configuration interpolation
- Failure scenario testing

**Success Criteria**: Memories persist reliably with privacy controls and async operations

### Phase 5: Testing & Polish (Week 5-6)

- End-to-end multi-agent workflows with all features
- Load testing with concurrent async operations
- Memory sharing examples for all configuration levels
- Documentation and best practices
- Performance optimization
- Comprehensive examples and documentation

**Success Criteria**: Feature is production-ready with comprehensive documentation and examples

## Logical Dependency Chain

1. **Foundation**: Memory resource models and registry with enhanced configuration support
2. **Resource Loading**: Depends on models, enables configuration parsing with all new features
3. **Memory Implementation**: Enhanced implementation with priorities, async operations, and hybrid flushing
4. **Agent Integration**: Requires registry and enhanced memory implementation
5. **Orchestrator Updates**: Depends on agent integration with async support
6. **Persistence**: Enhanced with async operations after memory implementation
7. **Testing**: Requires all components complete with new features

Critical path: Enhanced resource models → Registry → Enhanced memory implementation → Agent integration

## Success Metrics

### User Engagement Metrics

- **Adoption Rate**: % of projects using shared memory resources within 3 months
- **Sharing Ratio**: Average number of agents sharing each memory resource
- **Configuration Success**: % of memory configurations that work without errors
- **Simplification Usage**: % of users using simplified configuration patterns vs. full configuration

### Technical Quality Metrics

- **Performance Impact**: < 50ms overhead for async memory operations
- **Concurrency**: Support 10+ concurrent agents with async operations
- **Memory Efficiency**: < 10MB overhead per shared memory instance with priority and flushing features
- **Test Coverage**: > 85% code coverage with unit and integration tests

### Business Impact

- **Workflow Complexity**: Average reduction in workflow configuration size
- **Developer Productivity**: Time saved through simplified configuration and memory reuse
- **Support Tickets**: Reduction in issues related to context management

## Risks and Mitigations

### Technical Risks

1. **Resource Loading Complexity**

    - Risk: Complex dependency resolution between memories and agents with new configuration patterns
    - Mitigation: Simple flat structure, validate all references at startup, clear configuration resolution logic

2. **Concurrent Access Conflicts**

    - Risk: Race conditions and data loss when multiple agents modify shared memory with async operations
    - Mitigation: Implement a pessimistic distributed lock using Redis to serialize append operations, async-safe locking patterns, comprehensive concurrency testing

3. **Memory Leaks**

    - Risk: Shared memory instances not properly cleaned up with enhanced features
    - Mitigation: Workflow-scoped lifecycle, automatic cleanup, memory monitoring, TTL for all features

4. **Performance Degradation with Enhanced Features**

    - Risk: Priority processing and hybrid flushing impact performance
    - Mitigation: Efficient async operations, performance benchmarks, optional feature flags

5. **Configuration Complexity**

    - Risk: Multiple configuration levels create confusion
    - Mitigation: Clear documentation, validation with helpful error messages, progressive disclosure

6. **Key Injection Attacks**
    - Risk: User-supplied data in persistence keys could cause collisions or security issues
    - Mitigation: Strict key sanitization rules, length limits (max 512 chars), character whitelist (`[a-zA-Z0-9-_.:]`), automatic namespacing by project

### Adoption Risks

7. **Configuration Migration**

    - Risk: Developers struggle to choose appropriate configuration level
    - Mitigation: Clear migration guide, examples for each pattern, validation with suggestions

8. **Feature Complexity**
    - Risk: Advanced features (priorities, flushing) add unwanted complexity
    - Mitigation: Smart defaults, optional configurations, comprehensive examples

## Cross-Workflow Memory Sharing

To achieve memory persistence that survives across workflow executions (User Story #6), developers should use key templates that reference stable identifiers from the workflow input rather than the workflow ID itself:

**Examples**:

- For user-specific memories: `"prefs-{{ .workflow.input.userId }}"`
- For conversation-specific memories: `"support-{{ .workflow.input.conversationId }}"`
- For project-specific memories: `"research-{{ .workflow.input.projectId }}"`

This pattern allows multiple workflows to access the same memory instance by providing the same identifier in their input parameters.

## Open Questions

| Question                       | Description                                                                    | Decision Needed By | Owner           |
| ------------------------------ | ------------------------------------------------------------------------------ | ------------------ | --------------- |
| **Resource File Organization** | Should memory files be in `memories/` directory or mixed with other resources? | Phase 1            | Tech Lead       |
| **Priority Block Defaults**    | What should be the default priority blocks when none are specified?            | Phase 2            | Product Manager |
| **Configuration Validation**   | How strict should validation be for simplified configurations?                 | Phase 3            | Tech Lead       |
| **Monitoring**                 | What metrics to expose for shared memory usage with new features?              | Phase 5            | DevOps Lead     |

## Appendix

### Configuration Examples

#### Simple Project Setup

```yaml
# Minimal memory resource
memories:
    - id: chat-memory
      resource: memory
      description: "Simple chat history"
      type: token_based
      max_tokens: 4000
      persistence:
          type: redis
          ttl: "1h"

# Ultra-simple agent
agents:
    - id: chat-agent
      memory: chat-memory
      memory_key: "chat-{{ .workflow.input.sessionId }}"
```

#### Advanced Project Setup with All Features

```yaml
# Full-featured memory resource
memories:
    - id: advanced-memory
      resource: memory
      description: "Advanced memory with all features"
      type: token_based
      max_context_ratio: 0.8

      # Priority configuration
      priority_blocks:
          - priority: 0
            content_types: [system_prompt, user_profile]
            max_tokens: 500
          - priority: 1
            content_types: [recent_context, important_facts]
            max_tokens: 2000
          - priority: 2
            content_types: [historical_messages]

      # Token allocation (works with priority blocks)
      token_allocation:
          short_term: 0.7
          long_term: 0.2
          system: 0.1

      # Rule-based flushing strategy
      flushing_strategy:
          type: hybrid_summary
          summarize_threshold: 0.8
          summary_tokens: 500
          summarize_oldest_percent: 30

      persistence:
          type: redis
          ttl: "24h"

# Advanced agent configuration
agents:
    - id: advanced-agent
      memories:
          - id: advanced-memory
            mode: read-write
            key: "session-{{ .workflow.input.sessionId }}"
```

### Technical References

- Memory resource pattern inspired by Kubernetes ConfigMaps and Secrets
- Singleton pattern for shared memory instances
- Circuit breaker pattern for persistence layer (Hystrix-style)
- Priority-based eviction patterns from LlamaIndex
- Async operation patterns from LangChain/LangGraph
- Rule-based summarization approach for predictable costs and performance
- All token management and persistence features from original PRD retained and enhanced
