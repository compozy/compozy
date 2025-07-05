# Memory Tasks

Memory tasks enable conversation persistence and context management across workflow executions in Compozy.

## Overview

Memory tasks provide a way to store, retrieve, and manage conversational context using Redis as the persistence layer. This is essential for building AI agents that need to maintain state across multiple interactions.

## Task Configuration

Memory tasks use the `task: memory` type with various operations:

```yaml
tasks:
    - id: write_conversation
      type: memory
      operation: write
      memory_ref: conversation_memory
      key_template: "conv:{{.user_id}}:{{.session_id}}"
      payload:
          role: user
          content: "{{.user_message}}"
          timestamp: "{{.timestamp}}"
```

## Available Operations

### 1. Write Operation

Stores a new message in memory:

```yaml
operation: write
payload:
    role: user
    content: "Hello AI"
    metadata:
        user_id: "{{.user_id}}"
```

### 2. Read Operation

Retrieves messages from memory:

```yaml
operation: read
# Optional filters
read_config:
    limit: 10
    offset: 0
```

### 3. Append Operation

Adds messages to existing memory:

```yaml
operation: append
payload:
    role: assistant
    content: "{{.ai_response}}"
```

### 4. Delete Operation

Removes specific memory:

```yaml
operation: delete
```

### 5. Flush Operation

Archives old messages when limits are reached:

```yaml
operation: flush
flush_config:
    dry_run: false
    force: false
```

### 6. Health Operation

Checks memory system health:

```yaml
operation: health
health_config:
    include_stats: true
```

### 7. Clear Operation

Removes all messages from memory:

```yaml
operation: clear
clear_config:
    confirm: true
    backup: true
```

### 8. Stats Operation

Gets memory usage statistics:

```yaml
operation: stats
stats_config:
    include_content: false
```

## Memory Resource Configuration

Memory resources are defined in separate YAML files and loaded via autoload:

```yaml
# memory/conversation.yaml
resource: memory
id: conversation_memory
description: Conversation history with token management

key: "conv:{{.user_id}}:{{.session_id}}"
type: token_based
max_tokens: 4000
max_messages: 100

persistence:
    type: redis
    ttl: 24h

token_provider:
    provider: openai
    model: gpt-3.5-turbo

privacy_policy:
    redact_patterns:
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'
    non_persistable_message_types:
        - system
```

## Complete Example

```yaml
name: chatbot-workflow
version: 1.0.0

tasks:
    # Read previous conversation
    - id: read_history
      type: memory
      operation: read
      memory_ref: conversation_memory
      key_template: "conv:{{.user_id}}:{{.session_id}}"
      output: conversation_history

    # Process with AI
    - id: generate_response
      type: basic
      provider: openai
      model: gpt-3.5-turbo
      messages:
          - role: system
            content: "You are a helpful assistant."
          - role: assistant
            content: "{{.conversation_history}}"
          - role: user
            content: "{{.user_message}}"
      output: ai_response

    # Save to memory
    - id: save_interaction
      type: memory
      operation: append
      memory_ref: conversation_memory
      key_template: "conv:{{.user_id}}:{{.session_id}}"
      payload:
          - role: user
            content: "{{.user_message}}"
            timestamp: "{{.timestamp}}"
          - role: assistant
            content: "{{.ai_response.content}}"
            timestamp: "{{.timestamp}}"
```

## Best Practices

1. **Key Design**: Use hierarchical keys for easy management
2. **Token Limits**: Set appropriate limits to control costs
3. **Privacy**: Configure redaction patterns for sensitive data
4. **TTL**: Set reasonable TTL values to manage Redis memory
5. **Error Handling**: Always handle memory operation failures gracefully

## Troubleshooting

### Common Issues

1. **"Memory resource not found"**
    - Verify memory_ref matches resource id
    - Check autoload configuration
    - Ensure memory YAML files are valid

2. **"Resource registry not provided"**
    - Ensure compozy.yaml has autoload enabled
    - Check worker configuration

3. **Redis Connection Errors**
    - Verify Redis is running and accessible
    - Check connection configuration