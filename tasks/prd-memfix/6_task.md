---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>project_wide</scope>
<complexity>low</complexity>
<dependencies>task_1.0,task_2.0,task_3.0,task_4.0,task_5.0</dependencies>
</task_context>

# Task 6.0: Documentation and Production Deployment

## Overview

Create comprehensive documentation for memory task functionality, update release notes, and prepare for production deployment. Ensure all changes are properly documented for users and maintainers.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [x] 6.1 Create memory task usage documentation with examples ✅ COMPLETED
- [x] 6.2 Update API documentation for memory task endpoints ✅ COMPLETED

## Implementation Details

### Memory Task Usage Documentation

**File: `docs/features/memory-tasks.md`**

````markdown
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
````

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

````

### API Documentation Update

**File: `docs/api/memory-endpoints.md`**

```markdown
# Memory Task API Endpoints

## Execute Memory Task

### Request
````

POST /workflows/{workflowId}/tasks/{taskId}/execute
Content-Type: application/json

{
"type": "memory",
"operation": "write",
"memory_ref": "conversation_memory",
"key_template": "conv:{{.user_id}}:{{.session_id}}",
"payload": {
"role": "user",
"content": "Hello"
},
"input": {
"user_id": "user123",
"session_id": "sess456"
}
}

````

### Response
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "write"
  }
}
````

````

### Changelog Update

**File: `CHANGELOG.md`**

```markdown
# Changelog

## [Unreleased]

### Added
- Memory task integration for conversation persistence
- Support for 8 memory operations: read, write, append, delete, flush, health, clear, stats
- Redis-based memory persistence with distributed locking
- Token-based memory management with configurable limits
- Privacy protection with pattern-based redaction
- Comprehensive memory resource configuration via autoload

### Fixed
- Fixed Task2 factory missing support for memory task type
- Added missing normalizer and response handler for memory tasks
- Resolved "unsupported task type: memory" errors

### Changed
- Task2 factory now includes memory task cases in CreateNormalizer and CreateResponseHandler
````

### Migration Guide

**File: `docs/migration/memory-tasks-migration.md`**

````markdown
# Memory Tasks Migration Guide

## Overview

This guide helps you migrate existing projects to use the new memory task functionality.

## Prerequisites

- Compozy v0.4.0 or later
- Redis instance available
- Updated compozy.yaml with autoload configuration

## Migration Steps

### 1. Enable Autoload

Update your `compozy.yaml`:

```yaml
autoload:
    enabled: true
    include:
        - "memory/*.yaml"
```
````

### 2. Create Memory Resources

Create memory resource definitions in `memory/` directory:

```yaml
# memory/conversation.yaml
resource: memory
id: conversation_memory
# ... configuration
```

### 3. Update Workflows

Replace custom state management with memory tasks:

```yaml
# Before (custom implementation)
tasks:
  - id: save_state
    type: basic
    # Custom Redis logic

# After (memory task)
tasks:
  - id: save_state
    type: memory
    operation: write
    memory_ref: conversation_memory
```

### 4. Test Integration

Run integration tests to verify memory functionality:

```bash
make test-integration
```

````

### Deployment Checklist

**File: `docs/deployment/memory-tasks-rollout.md`**

```markdown
# Memory Tasks Production Deployment Checklist

## Pre-Deployment

- [ ] All tests passing (`make test`)
- [ ] Performance benchmarks meet targets
- [ ] Documentation reviewed and updated
- [ ] Redis infrastructure provisioned
- [ ] Memory resource configurations tested
- [ ] Rollback plan prepared

## Deployment Steps

1. **Deploy Redis Infrastructure**
   - Ensure Redis cluster is available
   - Configure connection pooling
   - Set up monitoring

2. **Deploy Application**
   - Deploy with feature flag disabled
   - Verify basic functionality
   - Enable memory features progressively

3. **Monitor**
   - Watch error rates
   - Monitor Redis memory usage
   - Check operation latencies

## Post-Deployment

- [ ] Verify memory operations in production
- [ ] Monitor performance metrics
- [ ] Update status page
- [ ] Notify users of new functionality
````

### Relevant Files

> Files that this task will create/modify:

- `docs/features/memory-tasks.md` - User documentation
- `docs/api/memory-endpoints.md` - API documentation
- `docs/migration/memory-tasks-migration.md` - Migration guide
- `docs/deployment/memory-tasks-rollout.md` - Deployment checklist
- `CHANGELOG.md` - Release notes
- `examples/memory-workflow/` - Example configurations

### Dependent Files

> Files that should be referenced in documentation:

- All implementation files from tasks 1-5
- `compozy.yaml` - Project configuration
- Memory resource YAML files

## Success Criteria

- [ ] User documentation covers all 8 memory operations with examples
- [ ] API documentation includes request/response formats
- [ ] Migration guide helps users adopt memory tasks
- [ ] Changelog accurately reflects all changes
- [ ] Deployment checklist ensures safe rollout
- [ ] Examples demonstrate real-world usage patterns
- [ ] Documentation is clear, accurate, and follows project standards
- [ ] All links and references are valid
