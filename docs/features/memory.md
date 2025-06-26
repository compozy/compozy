# Memory System

The Compozy Memory System provides a sophisticated, privacy-aware state management solution for AI workflows. It enables persistent context retention, intelligent memory management, and seamless integration with LLM conversations while ensuring data privacy and optimal performance.

## Overview

The memory system is designed to solve critical challenges in AI applications:

- **Context Window Management**: Intelligently manage limited LLM context windows
- **State Persistence**: Maintain conversation history and user context across sessions
- **Privacy Compliance**: Built-in data protection and redaction capabilities
- **Performance Optimization**: Efficient token counting and caching mechanisms
- **Distributed Operations**: Redis-backed storage with atomic operations

## Architecture

The memory system consists of several key components:

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Memory Manager │────▶│ Memory Instance  │────▶│   Store Layer   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
         │                       │                         │
         │                       ├── Strategies            ├── Redis Store
         │                       ├── Token Counter         └── In-Memory Store
         │                       └── Privacy Manager
         │
         └── Resource Registry
```

### Core Components

1. **Memory Manager**: Central orchestrator that manages memory instances, handles template evaluation, and integrates with the workflow system
2. **Memory Instance**: Individual memory containers with specific configurations and strategies
3. **Store Layer**: Abstraction for persistence backends with atomic operations
4. **Strategies**: Sophisticated algorithms for memory eviction and flushing
5. **Privacy Layer**: Comprehensive data protection and redaction system

## Memory Types

### TokenBasedMemory

Manages memory based on token counts, ideal for LLM context window management:

```yaml
resource: memory
id: conversation_memory
type: token_based
max_tokens: 4000
max_context_ratio: 0.8 # Use 80% of model's context window

model: gpt-4-turbo
model_context_size: 128000
```

### MessageCountBasedMemory

Simple message count-based management for fixed-size conversation windows:

```yaml
resource: memory
id: chat_history
type: message_count_based
max_messages: 50
```

### BufferMemory

Basic storage with size limits and simple FIFO behavior:

```yaml
resource: memory
id: temporary_buffer
type: buffer
max_messages: 10
```

## Configuration

### Basic Configuration

```yaml
resource: memory
id: user_memory
description: User conversation history and preferences
version: 1.0.0

# Dynamic key generation using Go templates
key: "user:{{.user_id}}:{{.session_id}}"

# Memory type and limits
type: token_based
max_tokens: 4000
max_messages: 100

# Persistence configuration
persistence:
    type: redis # or 'in_memory'
    ttl: 24h
    circuit_breaker:
        enabled: true
        max_failures: 5
        reset_timeout: 30s
```

### Advanced Configuration

```yaml
resource: memory
id: advanced_memory

# Token counting configuration
token_provider:
    provider: openai # anthropic, google, cohere, deepseek
    model: gpt-4
    api_key_env: OPENAI_API_KEY
    settings:
        timeout: 30s
        retry_config:
            max_retries: 3
            initial_delay: 1s

# Eviction policy configuration
eviction_policy:
    type: priority # fifo, lru, priority
    priority_keywords: ["important", "critical", "error"]
    protected_roles: ["system"]

# Flushing strategy
flushing:
    type: token_aware_lru # simple_fifo, lru, token_aware_lru
    summarize_threshold: 0.8
    target_capacity_percent: 0.5

# Locking configuration
locking:
    append_ttl: 15s
    clear_ttl: 30s
    flush_ttl: 2m

# Privacy policy
privacy_policy:
    redact_patterns:
        - '\b\d{3}-\d{2}-\d{4}\b' # SSN
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b' # Email
        - '\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{3,6}\b' # Credit cards
    non_persistable_message_types: ["system", "tool"]
    default_redaction_string: "[REDACTED]"
```

## Memory Operations

### Available Operations

| Operation | Description                       | Usage                            |
| --------- | --------------------------------- | -------------------------------- |
| `read`    | Retrieve all messages from memory | Reading conversation history     |
| `write`   | Replace all messages in memory    | Initializing or resetting memory |
| `append`  | Add new messages to memory        | Adding conversation turns        |
| `delete`  | Remove all messages               | Clearing memory                  |
| `flush`   | Execute flushing strategy         | Manual memory optimization       |
| `health`  | Check memory health status        | Monitoring and diagnostics       |
| `clear`   | Clear with confirmation           | Safe memory clearing             |
| `stats`   | Get memory statistics             | Usage analytics                  |

### Using Memory in Tasks

#### Reading Memory

```yaml
tasks:
    - id: load_context
      type: memory
      operation: read
      memory_ref: user_memory
      key_template: "user:{{ .workflow.input.user_id }}"
```

Output:

```json
{
    "messages": [
        { "role": "system", "content": "User preferences..." },
        { "role": "user", "content": "Hello" },
        { "role": "assistant", "content": "Welcome back!" }
    ],
    "count": 3,
    "key": "user:123"
}
```

#### Appending to Memory

```yaml
tasks:
    - id: save_conversation
      type: memory
      operation: append
      memory_ref: conversation_memory
      key_template: "user:{{ .workflow.input.user_id }}:chat"
      payload:
          - role: "user"
            content: "{{ .workflow.input.message }}"
          - role: "assistant"
            content: "{{ .tasks.ai_response.output.text }}"
```

#### Batch Operations

```yaml
tasks:
    - id: get_all_sessions
      type: memory
      operation: stats
      memory_ref: session_memory
      key_template: "session:*" # Wildcard pattern
      stats_config:
          include_flush_info: true
          include_token_count: true
          max_keys: 50
```

## Flushing Strategies

### SimpleFIFO

Basic first-in-first-out eviction when threshold is reached:

```yaml
flushing:
    type: simple_fifo
    summarize_threshold: 0.8 # Flush at 80% capacity
```

### LRU (Least Recently Used)

Evicts least recently used messages using ARC algorithm:

```yaml
flushing:
    type: lru
    summarize_threshold: 0.8
    settings:
        cache_size: 1000
        target_capacity_percent: 0.5
```

### TokenAwareLRU

Sophisticated strategy that considers token costs:

```yaml
flushing:
    type: token_aware_lru
    summarize_threshold: 0.8
    settings:
        target_capacity_percent: 0.5
        max_tokens: 4000
```

## Eviction Policies

### Priority-Based Eviction

Protects important messages based on role and keywords:

```yaml
eviction_policy:
    type: priority
    priority_keywords: ["critical", "important", "error", "warning"]
    role_priorities:
        system: 10 # Highest priority
        assistant: 5
        user: 3
        tool: 1 # Lowest priority
    recent_threshold: 0.8 # Protect recent 80% of messages
```

### FIFO Eviction

Simple chronological eviction:

```yaml
eviction_policy:
    type: fifo
```

### LRU Eviction

Evicts based on access patterns:

```yaml
eviction_policy:
    type: lru
```

## Privacy Features

### Data Redaction

Built-in patterns for sensitive data:

```yaml
privacy_policy:
    redact_patterns:
        # Built-in patterns
        - '\b\d{3}-\d{2}-\d{4}\b' # SSN
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b' # Email
        - '\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{3,6}\b' # Credit card
        - '\b\d{3}[-.]?\d{3}[-.]?\d{4}\b' # Phone
        - '\b(?:\d{1,3}\.){3}\d{1,3}\b' # IPv4
        - "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}" # UUID

        # Custom patterns
        - '\b[A-Z]{2}\d{6}\b' # Custom ID format
```

### Selective Persistence

Control which messages are stored:

```yaml
privacy_policy:
    non_persistable_message_types:
        - system # System prompts
        - tool # Tool responses

    # Privacy metadata support
    privacy_metadata:
        do_not_persist: true
        sensitive_fields: ["password", "api_key"]
        privacy_level: "confidential"
```

## Token Counting

### Multi-Provider Support

```yaml
token_provider:
    provider: openai # Options: openai, anthropic, google, cohere, deepseek
    model: gpt-4
    api_key_env: OPENAI_API_KEY

    # API configuration
    settings:
        base_url: "https://api.openai.com/v1"
        timeout: 30s
        retry_config:
            max_retries: 3
            initial_delay: 1s
            max_delay: 10s
```

### Fallback Strategies

When API counting fails, character-based estimation:

```yaml
token_estimation:
    strategy: english # Options: english, unicode, chinese, conservative

    # Custom ratios
    custom_ratios:
        english: 4 # 4 characters per token
        unicode: 0.5 # Rune count / 2
        chinese: 0.67 # (Rune count * 2) / 3
        conservative: 3 # 3 characters per token
```

## Integration Patterns

### Agent-Based Memory

Automatic memory management for agents:

```yaml
agents:
    - id: support_agent
      config:
          $ref: global::models.#(provider=="openai")
      memory:
          - id: user_memory # Auto-loaded before execution
          - id: session_memory # Auto-saved after execution
      instructions: |
          You have access to user preferences and conversation history.
          Provide personalized assistance based on past interactions.
```

### Direct Task Integration

Explicit memory operations in workflows:

```yaml
workflow:
    tasks:
        # Initialize user profile
        - id: init_profile
          type: memory
          operation: write
          memory_ref: user_memory
          key_template: "user:{{ .workflow.input.user_id }}:profile"
          payload:
              role: "system"
              content: |
                  User Profile:
                  Name: {{ .workflow.input.name }}
                  Preferences: {{ .workflow.input.preferences }}

        # Load and use context
        - id: load_context
          type: memory
          operation: read
          memory_ref: user_memory
          key_template: "user:{{ .workflow.input.user_id }}:profile"

        - id: personalized_response
          type: agent
          agent_id: support_agent
          input:
              context: "{{ .tasks.load_context.output.messages }}"
              query: "{{ .workflow.input.query }}"
```

### Tool-Based Access

Memory operations through custom tools:

```yaml
tools:
    - id: memory_tool
      type: function
      function:
          name: manage_memory
          description: Read or write to user memory
          parameters:
              operation:
                  type: string
                  enum: ["read", "append"]
              key:
                  type: string
              message:
                  type: string
```

## Performance Optimization

### Metadata Caching

O(1) performance for common operations:

```yaml
# Cached metadata includes:
# - Token count
# - Message count
# - Last flush timestamp
# - TTL information

persistence:
    metadata_cache:
        enabled: true
        ttl: 5m
```

### Atomic Operations

Redis Lua scripts ensure consistency:

```lua
-- Atomic append with metadata update
local key = KEYS[1]
local meta_key = KEYS[2]
local message = ARGV[1]
local token_delta = ARGV[2]

redis.call('RPUSH', key, message)
redis.call('HINCRBY', meta_key, 'token_count', token_delta)
redis.call('HINCRBY', meta_key, 'message_count', 1)
```

### Circuit Breaker

Resilience for persistence operations:

```yaml
persistence:
    circuit_breaker:
        enabled: true
        max_failures: 5
        reset_timeout: 30s
        half_open_requests: 3
```

## Monitoring and Health

### Health Checks

```yaml
tasks:
    - id: check_memory_health
      type: memory
      operation: health
      memory_ref: user_memory
      key_template: "user:{{ .workflow.input.user_id }}"
      health_config:
          check_flush_threshold: true
          check_token_limit: true
```

Output:

```json
{
    "healthy": true,
    "key": "user:123",
    "token_count": 3200,
    "message_count": 45,
    "max_tokens": 4000,
    "flush_threshold_exceeded": false,
    "last_flush": "2024-01-15T10:30:00Z"
}
```

### Statistics

```yaml
tasks:
    - id: memory_analytics
      type: memory
      operation: stats
      memory_ref: user_memory
      key_template: "user:*"
      stats_config:
          include_flush_info: true
          include_token_count: true
          include_message_distribution: true
```

## Best Practices

### 1. Key Design

Use hierarchical keys for better organization:

```yaml
# Good: Hierarchical and specific
key_template: "org:{{ .org_id }}:user:{{ .user_id }}:{{ .context_type }}"
# Examples:
# org:acme:user:123:preferences
# org:acme:user:123:conversation:456
# org:acme:user:123:session:current
```

### 2. Memory Sizing

Set appropriate limits based on your use case:

```yaml
# Short conversations (customer support)
max_tokens: 2000
max_messages: 50
ttl: 24h

# Long-term memory (user profiles)
max_tokens: 8000
max_messages: 200
ttl: 720h  # 30 days

# Real-time chat
max_tokens: 4000
max_messages: 100
ttl: 1h
```

### 3. Error Handling

Always handle memory operation failures gracefully:

```yaml
tasks:
    - id: safe_memory_read
      type: memory
      operation: read
      memory_ref: user_memory
      key_template: "user:{{ .workflow.input.user_id }}"
      on_error:
          continue: true
          default_output:
              messages: []
              count: 0
```

### 4. Privacy Compliance

Configure comprehensive privacy policies:

```yaml
privacy_policy:
    # Data protection
    encrypt_at_rest: true
    retention_days: 30

    # Automatic redaction
    redact_patterns:
        - '\b\d{3}-\d{2}-\d{4}\b' # SSN
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b' # Email

    # Audit logging
    audit_operations: ["read", "write", "delete"]
    audit_retention_days: 90
```

### 5. Performance Tuning

Optimize for your workload:

```yaml
# High-throughput configuration
persistence:
    type: redis
    connection_pool:
        max_connections: 100
        min_idle: 10

    circuit_breaker:
        enabled: true
        max_failures: 10
        reset_timeout: 60s

# Token counting optimization
token_provider:
    cache_tokens: true
    cache_ttl: 1h
    batch_size: 100
```

## Troubleshooting

### Common Issues

1. **Token Count Exceeds Limit**

    - Check flushing strategy configuration
    - Verify threshold settings (usually 0.8)
    - Consider more aggressive eviction policy

2. **Memory Not Persisting**

    - Verify Redis connection
    - Check TTL configuration
    - Ensure circuit breaker isn't open

3. **Slow Performance**

    - Enable metadata caching
    - Check Redis latency
    - Consider batch operations

4. **Privacy Violations**
    - Review redaction patterns
    - Check non-persistable message types
    - Verify privacy metadata handling

### Debug Mode

Enable detailed logging:

```yaml
debug:
    log_operations: true
    log_token_counts: true
    log_flush_decisions: true
    trace_redis_commands: true
```

## Migration Guide

### From Simple Storage

If migrating from basic key-value storage:

1. Define memory resources with appropriate types
2. Configure flushing strategies based on usage patterns
3. Set up privacy policies for sensitive data
4. Implement gradual rollout with feature flags

### From Other Memory Systems

Key considerations:

- Map existing eviction policies to Compozy strategies
- Convert storage keys to template format
- Implement data migration scripts
- Test token counting accuracy

## Future Enhancements

Planned features for the memory system:

1. **Vector Embeddings**: Semantic search and retrieval
2. **Compression**: Message compression for larger contexts
3. **Summarization**: AI-powered message summarization
4. **Multi-Model Support**: Different strategies per model
5. **Analytics Dashboard**: Memory usage visualization
6. **Export/Import**: Memory backup and restoration

The Compozy Memory System provides a production-ready solution for managing conversational context in AI applications, with sophisticated features for performance, privacy, and reliability.
