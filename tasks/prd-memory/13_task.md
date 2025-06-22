---
status: pending
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_1,task_5,task_6</dependencies>
</task_context>

# Task 13.0: Implement Memory Task Type for Direct Memory Management

## Overview

Implement a new task type `memory` that enables direct memory management operations outside of agent scope. This allows workflows to perform operations like read, write, append, delete, flush, health checks, clear, and stats on memory resources programmatically.

## Subtasks

- [ ] 13.1 Add task type enums and executor registration
- [ ] 13.2 Create config structs with performance validation
- [ ] 13.3 Implement basic operations (read, write, append, delete)
- [ ] 13.4 Add error handling and safety limits
- [ ] 13.5 Implement administrative operations (flush, health, stats)
- [ ] 13.6 Add batch processing for bulk operations
- [ ] 13.7 Implement destructive operations (clear) with safety guards
- [ ] 13.8 Add monitoring and basic performance metrics
- [ ] 13.9 Create comprehensive unit and integration tests
- [ ] 13.10 Implement performance testing with safety limits
- [ ] 13.11 Add documentation and usage examples

## Implementation Details

### Core Configuration Schema

Following the pattern of other task types (SignalTask, WaitTask), add to `engine/task/config.go`:

```go
// -----------------------------------------------------------------------------
// Memory Task
// -----------------------------------------------------------------------------

type MemoryTask struct {
    Operation    MemoryOpType    `json:"operation"     yaml:"operation"     mapstructure:"operation"`
    MemoryRef    string          `json:"memory_ref"    yaml:"memory_ref"    mapstructure:"memory_ref"`
    KeyTemplate  string          `json:"key_template"  yaml:"key_template"  mapstructure:"key_template"`
    Payload      any             `json:"payload"       yaml:"payload"       mapstructure:"payload,omitempty"`

    // Performance controls
    BatchSize    int             `json:"batch_size"    yaml:"batch_size"    mapstructure:"batch_size,omitempty"`
    MaxKeys      int             `json:"max_keys"      yaml:"max_keys"      mapstructure:"max_keys,omitempty"`

    // Operation-specific configs
    FlushConfig  *FlushConfig    `json:"flush_config"  yaml:"flush_config"  mapstructure:"flush_config,omitempty"`
    HealthConfig *HealthConfig   `json:"health_config" yaml:"health_config" mapstructure:"health_config,omitempty"`
    StatsConfig  *StatsConfig    `json:"stats_config"  yaml:"stats_config"  mapstructure:"stats_config,omitempty"`
    ClearConfig  *ClearConfig    `json:"clear_config"  yaml:"clear_config"  mapstructure:"clear_config,omitempty"`
}

type MemoryOpType string

const (
    MemoryOpRead   MemoryOpType = "read"
    MemoryOpWrite  MemoryOpType = "write"
    MemoryOpAppend MemoryOpType = "append"
    MemoryOpDelete MemoryOpType = "delete"
    MemoryOpFlush  MemoryOpType = "flush"
    MemoryOpHealth MemoryOpType = "health"
    MemoryOpClear  MemoryOpType = "clear"
    MemoryOpStats  MemoryOpType = "stats"
)

// Add task type constant:
const (
    // ... existing constants ...
    TaskTypeMemory Type = "memory"
)

// Update GetExecType() method to handle memory tasks:
func (t *Config) GetExecType() ExecutionType {
    // ... existing cases ...
    case TaskTypeMemory:
        executionType = ExecutionBasic  // Memory ops use basic execution
    // ...
}

// Update Validate() method to handle memory task validation:
func (t *Config) Validate() error {
    // ... existing validation ...

    // Validate memory task specific fields
    if t.Type == TaskTypeMemory {
        if err := t.validateMemoryTask(); err != nil {
            return fmt.Errorf("invalid memory task '%s': %w", t.ID, err)
        }
    }
    return nil
}

// Add to Config struct embedding:
type Config struct {
    BasicTask        `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    RouterTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    ParallelTask     `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    CollectionConfig `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    SignalTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    WaitTask         `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    MemoryTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"` // NEW
    BaseConfig       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
    Tasks            []Config `json:"tasks"          yaml:"tasks"          mapstructure:"tasks"`
    Task             *Config  `json:"task,omitempty" yaml:"task,omitempty" mapstructure:"task,omitempty"`
}
```

### Memory Operations

#### 1. READ Operation

```yaml
- id: load_user_context
  type: memory
  operation: read
  memory_ref: user-context
  key_template: "user:{{ .workflow.input.user_id }}"
```

Returns: Memory content as messages array

#### 2. WRITE Operation

```yaml
- id: save_user_preferences
  type: memory
  operation: write
  memory_ref: user-prefs
  key_template: "prefs:{{ .workflow.input.user_id }}"
  payload: "{{ .workflow.input.preferences }}"
```

Returns: Success confirmation with metadata

#### 3. APPEND Operation

```yaml
- id: add_conversation_message
  type: memory
  operation: append
  memory_ref: chat-history
  key_template: "chat:{{ .workflow.input.session_id }}"
  payload:
      role: "user"
      content: "{{ .workflow.input.message }}"
```

Returns: Success confirmation with new total size

#### 4. DELETE Operation

```yaml
- id: remove_session
  type: memory
  operation: delete
  memory_ref: session-store
  key_template: "session:{{ .workflow.input.session_id }}"
```

Returns: Success confirmation

#### 5. FLUSH Operation

```yaml
- id: cleanup_old_memories
  type: memory
  operation: flush
  memory_ref: chat-history
  key_template: "chat:{{ .workflow.input.user_id }}:*"
  max_keys: 5000
  flush_config:
      strategy: fifo
      max_keys: 1000
      dry_run: false
      threshold: 0.8
```

Returns: Flush statistics (keys affected, space reclaimed)

#### 6. HEALTH Operation

```yaml
- id: check_memory_status
  type: memory
  operation: health
  memory_ref: main-memory
  key_template: "{{ .workflow.input.key_pattern | default('*') }}"
  health_config:
      include_stats: true
      check_connectivity: true
```

Returns: Health status, connectivity, and optional statistics

#### 7. CLEAR Operation

```yaml
- id: reset_user_data
  type: memory
  operation: clear
  memory_ref: user-data
  key_template: "user:{{ .workflow.input.user_id }}:*"
  max_keys: 1000
  clear_config:
      confirm: true
      backup: true
```

Returns: Clear confirmation and backup info

#### 8. STATS Operation

```yaml
- id: analyze_memory_usage
  type: memory
  operation: stats
  memory_ref: analytics-store
  key_template: "analytics:{{ .workflow.input.project_id }}:*"
  batch_size: 500
  stats_config:
      include_content: true
      group_by: "project"
```

Returns: Detailed usage statistics

### Performance Controls

**Safety Limits**:

- `max_keys`: Default 10,000 key limit for wildcard operations
- `batch_size`: Default 1,000 items per batch for bulk operations
- Maximum allowed: 50,000 keys, 10,000 batch size

**Performance Validation**:

```go
func (c *Config) validatePerformance() error {
    if c.MaxKeys > 50000 {
        return errors.New("max_keys cannot exceed 50,000 for safety")
    }
    if c.BatchSize > 10000 {
        return errors.New("batch_size cannot exceed 10,000")
    }
    return nil
}
```

### Error Handling

**Error Types**:

- `quota_exceeded`: Memory limits reached
- `not_found`: Key/memory not found
- `conflict`: Concurrent modification conflict
- `unauthorized`: Access denied
- `invalid_operation`: Unsupported operation for memory type
- `template_error`: Key template evaluation failed
- `timeout`: Operation timeout exceeded
- `safety_limit_exceeded`: Max keys or batch size exceeded

### Security & Resource Binding

**Required Resource Declaration**:

```yaml
resources:
    memories:
        - id: chat-history
          type: memory
        - id: user-prefs
          type: memory

tasks:
    - id: cleanup_task
      type: memory
      memory_ref: chat-history # Must be in resources.memories
```

**Access Control**:

- Same resource binding validation as agents
- Memory-ref must be declared in workflow resources
- Operation permissions based on memory resource configuration
- Template evaluation using existing ref.Evaluator

### Implementation Architecture

**Integration with Existing Systems**:

- Use existing `memory.Manager` interface for memory access
- Leverage `ref.Evaluator` for template evaluation
- Follow existing task executor patterns
- Reuse error handling patterns from other task types
- Integrate with existing monitoring service

**Key Implementation Files**:

- `engine/task/config.go` - Add TaskTypeMemory constant and MemoryTask struct
- `engine/task/domain.go` - Add ExecutionMemory constant if needed
- `engine/task/activities/exec_memory.go` - Main memory task executor
- `engine/task/activities/memory_operations.go` - Individual operation handlers
- `engine/task/validators.go` - Add memory task validation
- `engine/task/activities/exec_memory_test.go` - Memory task tests

## Success Criteria

1. **Functional Requirements**:

    - All 8 operations work correctly with proper validation
    - Resource binding enforced for all memory references
    - Template evaluation works for dynamic key generation
    - Error handling provides clear, actionable messages

2. **Performance Requirements**:

    - Operations complete within 50ms overhead
    - Batch operations handle up to 10,000 items efficiently
    - Safety limits prevent resource exhaustion
    - Monitoring tracks operation duration and resource usage

3. **Security Requirements**:

    - Memory access follows project-level permissions
    - Destructive operations require explicit confirmation
    - Template evaluation prevents injection attacks
    - All operations logged for audit trail

4. **Testing Requirements**:

    - Unit tests for each operation type
    - Integration tests with Redis backend
    - Performance tests validating safety limits
    - Error scenario testing for all error types

5. **Documentation Requirements**:
    - Complete API documentation for all operations
    - YAML configuration examples for each operation
    - Performance tuning guide
    - Troubleshooting guide for common issues

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
