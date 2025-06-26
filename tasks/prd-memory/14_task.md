# Task 14: Add Memory-Operation Documentation and Examples

## Overview

Create comprehensive documentation and practical examples for the memory-operation task type to help developers understand and effectively use direct memory management capabilities within their workflows.

## Goals

- Provide clear documentation for all memory operations
- Create practical examples for common use cases
- Include troubleshooting and best practices
- Ensure examples follow existing Compozy patterns
- Support both simple and advanced usage scenarios

## Deliverables

### 1. Update Example Memory Workflow

**File**: `examples/memory/workflow.yaml`

Add memory-operation tasks to the existing memory example:

```yaml
tasks:
    # ... existing tasks ...

    - id: cleanup_old_conversations
      type: memory-operation
      operation: list-keys
      memory_id: user_memory
      list_keys_config:
          pattern: "user-*"
          limit: 100
      outputs:
          conversation_keys: "{{ .result.keys }}"

    - id: clear_specific_conversation
      type: memory-operation
      operation: clear
      memory_id: user_memory
      memory_key: "user-{{ .workflow.input.userId }}"
      clear_config:
          confirm_token: "{{ .workflow.input.confirmClearToken | default('') }}"
      condition: "{{ .workflow.input.clearMemory == true }}"

    - id: get_memory_stats
      type: memory-operation
      operation: get-stats
      memory_id: user_memory
      memory_key: "user-{{ .workflow.input.userId }}"
      outputs:
          memory_stats: "{{ .result.stats }}"

    - id: update_conversation_metadata
      type: memory-operation
      operation: update-metadata
      memory_id: user_memory
      memory_key: "user-{{ .workflow.input.userId }}"
      update_meta_config:
          metadata:
              last_interaction: "{{ .workflow.context.timestamp }}"
              user_satisfaction: "{{ .workflow.input.satisfaction | default('unknown') }}"
              conversation_type: "support"
```

### 2. Create Memory Management Workflow Example

**File**: `examples/memory-management/workflow.yaml`

```yaml
id: memory-management-demo
version: 0.1.0
description: Demonstrates memory management operations

memories:
    - id: session_memory
      resource: memory
      description: "Session-specific memory for demonstrations"
      type: token_based
      max_tokens: 4000
      persistence:
          type: redis
          ttl: "1h"

agents:
    - id: memory_admin
      config:
          $ref: global::models.#(provider=="groq")
          temperature: 0.3
          max_tokens: 200
      instructions: |
          You are a memory administrator. Report on memory operations and provide summaries.

tasks:
    - id: list_all_sessions
      type: memory-operation
      operation: list-keys
      memory_id: session_memory
      list_keys_config:
          pattern: "session-*"
          limit: 50
      outputs:
          session_list: "{{ .result.keys }}"

    - id: analyze_session_count
      type: basic
      agent: memory_admin
      action: |
          I found {{ len .session_list }} active sessions: {{ join .session_list ", " }}

          Please provide a brief analysis of the session activity.
      with:
          session_list: "{{ .outputs.list_all_sessions.session_list }}"

    - id: get_session_stats
      type: memory-operation
      operation: get-stats
      memory_id: session_memory
      memory_key: "session-{{ .workflow.input.sessionId | default('demo-123') }}"
      outputs:
          stats: "{{ .result.stats }}"

    - id: flush_if_needed
      type: memory-operation
      operation: flush
      memory_id: session_memory
      memory_key: "session-{{ .workflow.input.sessionId | default('demo-123') }}"
      flush_config:
          token_target: 2000
          percentage: 0.3
      condition: "{{ .outputs.get_session_stats.stats.token_count > 3500 }}"

    - id: get_session_stats_after_flush
      type: memory-operation
      operation: get-stats
      memory_id: session_memory
      memory_key: "session-{{ .workflow.input.sessionId | default('demo-123') }}"
      outputs:
          stats_after: "{{ .result.stats }}"

    - id: update_session_metadata
      type: memory-operation
      operation: update-metadata
      memory_id: session_memory
      memory_key: "session-{{ .workflow.input.sessionId | default('demo-123') }}"
      update_meta_config:
          metadata:
              last_maintenance: "{{ .workflow.context.timestamp }}"
              operation_type: "automated_flush"
              token_count_before: "{{ .outputs.get_session_stats.stats.token_count }}"
              token_count_after: "{{ .outputs.get_session_stats_after_flush.stats_after.token_count }}"

    - id: report_maintenance
      type: basic
      agent: memory_admin
      action: |
          Memory maintenance completed for session {{ .workflow.input.sessionId | default('demo-123') }}:

          - Token count before: {{ .stats_before.token_count }}
          - Token count after: {{ .stats_after.token_count }}
          - Message count: {{ .stats_after.message_count }}
          - Flush performed: {{ .flush_performed }}

          Provide a summary of the maintenance operation.
      with:
          stats_before: "{{ .outputs.get_session_stats.stats }}"
          stats_after: "{{ .outputs.get_session_stats_after_flush.stats_after }}"
          flush_performed: "{{ .outputs.flush_if_needed != null }}"
```

**File**: `examples/memory-management/deno.json`

```json
{
    "imports": {}
}
```

**File**: `examples/memory-management/api.http`

```http
### Create memory management workflow
POST http://localhost:3001/api/v1/workflows
Content-Type: application/json

{
    "workflow_file": "./workflow.yaml",
    "input": {
        "sessionId": "demo-session-1"
    }
}

### List active sessions
POST http://localhost:3001/api/v1/workflows
Content-Type: application/json

{
    "workflow_file": "./workflow.yaml",
    "input": {
        "sessionId": "admin-list-all"
    }
}

### Maintenance with flush
POST http://localhost:3001/api/v1/workflows
Content-Type: application/json

{
    "workflow_file": "./workflow.yaml",
    "input": {
        "sessionId": "heavy-session-456"
    }
}
```

### 3. Update Memory Documentation

**File**: `examples/memory/README.md`

Update the existing README to include memory-operation documentation:

````markdown
# Memory System Examples

This directory contains examples demonstrating Compozy's shared memory system and memory management operations.

## Quick Start

1. Start the Compozy server:
    ```bash
    make dev
    ```
````

2. Run the basic memory example:

    ```bash
    # First conversation - establish memory
    curl -X POST http://localhost:3001/api/v1/workflows \
        -H "Content-Type: application/json" \
        -d '{
        "workflow_file": "./workflow.yaml",
        "input": {
          "inquiry": "My name is John and I live in San Francisco",
          "userId": "user123"
        }
      }'
    
    # Second conversation - recall memory
    curl -X POST http://localhost:3001/api/v1/workflows \
        -H "Content-Type: application/json" \
        -d '{
        "workflow_file": "./workflow.yaml", 
        "input": {
          "inquiry": "Where do I live?",
          "userId": "user123"
        }
      }'
    ```

## Memory Operations

The memory system supports direct memory management through memory-operation tasks:

### Available Operations

1. **clear** - Remove all messages from memory
2. **update-metadata** - Update memory metadata
3. **list-keys** - List memory keys matching a pattern
4. **flush** - Apply flushing strategy to reduce memory size
5. **get-stats** - Get memory statistics

### Clear Memory

```yaml
- id: clear_memory
  type: memory-operation
  operation: clear
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  clear_config:
      confirm_token: "CLEAR_CONFIRMED"
```

**Use Cases:**

- Reset conversation when user requests fresh start
- Clean up after completed support sessions
- Remove sensitive data after processing

### Update Metadata

```yaml
- id: update_metadata
  type: memory-operation
  operation: update-metadata
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  update_meta_config:
      metadata:
          last_interaction: "{{ .workflow.context.timestamp }}"
          user_tier: "premium"
          session_quality: "excellent"
```

**Use Cases:**

- Track conversation quality metrics
- Store user preference updates
- Record session completion status

### List Memory Keys

```yaml
- id: list_sessions
  type: memory-operation
  operation: list-keys
  memory_id: session_memory
  list_keys_config:
      pattern: "session-*"
      limit: 100
```

**Important Security Note**: For security purposes, the `keys` returned by `list-keys` are SHA256 hashes of the actual memory keys. These hashed values are for auditing and monitoring purposes only and cannot be used as `memory_key` values in subsequent memory operations.

**Use Cases:**

- Administrative dashboards and monitoring
- Memory usage analysis and reporting
- Session activity tracking
- Audit trail generation

### Flush Memory

```yaml
- id: flush_memory
  type: memory-operation
  operation: flush
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  flush_config:
      token_target: 2000
      percentage: 0.3
```

**Use Cases:**

- Proactive memory management
- Performance optimization
- Cost control for long conversations

### Get Statistics

```yaml
- id: get_stats
  type: memory-operation
  operation: get-stats
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
```

**Use Cases:**

- Performance monitoring
- Memory usage analytics
- Debugging memory issues

## Common Patterns

### Memory Maintenance Workflow

Combine multiple operations for comprehensive memory management:

```yaml
- id: maintenance_check
  type: memory-operation
  operation: get-stats
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"

- id: conditional_flush
  type: memory-operation
  operation: flush
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  condition: "{{ .outputs.maintenance_check.stats.token_count > 4000 }}"

- id: update_maintenance_log
  type: memory-operation
  operation: update-metadata
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  update_meta_config:
      metadata:
          last_maintenance: "{{ .workflow.context.timestamp }}"
          maintenance_reason: "token_limit_exceeded"
```

### Memory Monitoring and Analysis

**Important Note**: The `list-keys` operation returns hashed keys for security purposes. These hashed keys are for auditing and monitoring only and cannot be used as `memory_key` values in subsequent operations.

```yaml
- id: monitor_sessions
  type: memory-operation
  operation: list-keys
  memory_id: session_memory
  list_keys_config:
      pattern: "session-*"
      limit: 100
  outputs:
      session_count: "{{ len .result.keys }}"
      session_hashes: "{{ .result.keys }}"

- id: analyze_session_activity
  type: basic
  agent: memory_analyst
  action: |
      Found {{ .session_count }} active sessions.
      Session activity analysis:

      {{ range .session_hashes }}
      - Session hash: {{ . }}
      {{ end }}

      Provide insights on session usage patterns.
  with:
      session_count: "{{ .outputs.monitor_sessions.session_count }}"
      session_hashes: "{{ .outputs.monitor_sessions.session_hashes }}"
```

## Best Practices

### 1. Use Confirmation Tokens for Destructive Operations

Always require confirmation tokens for clear operations:

```yaml
clear_config:
    confirm_token: "{{ .workflow.input.confirmClearToken | default('') }}"
```

### 2. Check Statistics Before Operations

Get memory statistics before performing operations:

```yaml
- id: check_before_flush
  type: memory-operation
  operation: get-stats
  # ... config ...

- id: conditional_flush
  type: memory-operation
  operation: flush
  condition: "{{ .outputs.check_before_flush.stats.token_count > 3000 }}"
  # ... config ...
```

### 3. Update Metadata for Tracking

Record operation history in metadata:

```yaml
update_meta_config:
    metadata:
        last_operation: "{{ .operation }}"
        operation_timestamp: "{{ .workflow.context.timestamp }}"
        operation_user: "{{ .workflow.input.userId }}"
```

### 4. Use Patterns for Bulk Operations

Use specific patterns when listing keys:

```yaml
list_keys_config:
  pattern: "user-{{ .workflow.input.userId }}-*"  # User-specific
  pattern: "session-{{ .workflow.context.date }}-*"   # Today's sessions
  pattern: "temp-*"                                # Temporary memories
```

## Error Handling

Memory operations can fail for various reasons. Handle errors gracefully:

```yaml
- id: safe_clear
  type: memory-operation
  operation: clear
  memory_id: user_memory
  memory_key: "user-{{ .workflow.input.userId }}"
  clear_config:
      confirm_token: "CLEAR_CONFIRMED"
  on_error:
      transition: "handle_clear_error"

- id: handle_clear_error
  type: basic
  agent: error_handler
  action: |
      Failed to clear memory: {{ .error.message }}
      Please try again or contact support.
```

## Troubleshooting

### Common Issues

1. **Memory Key Not Found**

    - Ensure the memory key template resolves correctly
    - Check that the memory instance exists

2. **Operation Validation Errors**

    - Verify required configuration fields are present
    - Check that operation type is supported

3. **Permission Errors**
    - Ensure memory resource exists in project configuration
    - Verify access permissions for the operation

### Debugging Tips

1. Use get-stats to inspect memory state:

    ```yaml
    - id: debug_memory
      type: memory-operation
      operation: get-stats
      memory_id: problematic_memory
      memory_key: "debug-key"
    ```

2. List keys to verify memory instances:
    ```yaml
    - id: list_debug
      type: memory-operation
      operation: list-keys
      memory_id: problematic_memory
      list_keys_config:
          pattern: "*"
          limit: 10
    ```

## See Also

- [Memory Management Examples](../memory-management/) - Advanced memory management patterns
- [Memory System Documentation](../../docs/memory-system.md) - Technical implementation details
- [Memory Migration Guide](../../docs/memory-migration.md) - Upgrading from previous versions

````

### 4. Create Memory Management README

**File**: `examples/memory-management/README.md`

```markdown
# Memory Management Examples

This directory demonstrates advanced memory management patterns using memory-operation tasks in Compozy workflows.

## Overview

Memory-operation tasks provide direct control over memory resources, enabling:

- Proactive memory maintenance
- Administrative operations
- Performance optimization
- Session lifecycle management

## Examples

### 1. Basic Memory Management

The main workflow demonstrates:
- Listing active memory sessions
- Getting memory statistics
- Conditional memory flushing
- Metadata updates for tracking

Run the example:
```bash
curl -X POST http://localhost:3001/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_file": "./workflow.yaml",
    "input": {
      "sessionId": "demo-session-1"
    }
  }'
````

### 2. Memory Monitoring Dashboard

Create a monitoring workflow that checks memory health:

```yaml
- id: system_health_check
  type: memory-operation
  operation: list-keys
  memory_id: session_memory
  list_keys_config:
      pattern: "*"
      limit: 1000

- id: analyze_memory_usage
  type: basic
  agent: memory_analyst
  action: |
      Found {{ len .session_keys }} active memory instances.
      Analyzing memory health and usage patterns...
  with:
      session_keys: "{{ .outputs.system_health_check.keys }}"
```

### 3. Automated Cleanup

Implement scheduled cleanup workflows:

```yaml
- id: daily_cleanup
  type: collection
  items: "{{ .old_sessions }}"
  item_var: session_key
  mode: parallel
  max_workers: 5
  task:
      id: cleanup_session
      type: memory-operation
      operation: clear
      memory_id: session_memory
      memory_key: "{{ .session_key }}"
      clear_config:
          confirm_token: "AUTOMATED_CLEANUP"
```

## Best Practices

1. **Monitor Before Acting**: Always check memory statistics before performing operations
2. **Use Confirmation Tokens**: Protect destructive operations with confirmation tokens
3. **Batch Operations**: Use collection tasks for bulk memory operations
4. **Update Metadata**: Track operations in memory metadata for auditing
5. **Handle Errors**: Implement proper error handling for failed operations

## Use Cases

- **Session Cleanup**: Remove old or completed conversation memories
- **Performance Maintenance**: Proactively flush memories approaching token limits
- **Administrative Tools**: Build dashboards for memory usage monitoring
- **Data Lifecycle**: Implement retention policies for memory data
- **Quality Assurance**: Track memory usage patterns and optimize configurations

## Integration with Existing Systems

Memory-operation tasks integrate seamlessly with:

- Agent conversations (get stats before/after interactions)
- Workflow orchestration (conditional memory management)
- External APIs (trigger cleanup from external events)
- Monitoring systems (export memory metrics)

```

## Implementation Requirements

### Documentation Standards
- Follow existing Compozy documentation patterns
- Provide practical, runnable examples
- Include troubleshooting and best practices
- Use consistent YAML formatting and style

### Example Quality
- Examples should be immediately runnable
- Cover common real-world use cases
- Demonstrate both simple and advanced patterns
- Include proper error handling

### Integration
- Extend existing memory examples rather than replace
- Maintain consistency with other Compozy examples
- Follow established naming conventions
- Use appropriate file organization

## Success Criteria

1. **Completeness**: Documentation covers all memory operations with practical examples
2. **Clarity**: Examples are easy to understand and follow
3. **Usability**: Developers can copy/paste examples and run them successfully
4. **Consistency**: Documentation follows Compozy standards and patterns
5. **Coverage**: Common use cases and troubleshooting scenarios are addressed

## Dependencies

- Task 13 (Memory-Operation Task Type implementation)
- Existing memory system examples
- Documentation standards and templates

## Estimated Effort

- **Example Creation**: 1-2 days
- **Documentation Writing**: 1-2 days
- **Review and Polish**: 1 day
- **Total**: 3-5 days

This task ensures developers have comprehensive resources to effectively use memory-operation tasks in their Compozy workflows.
```
