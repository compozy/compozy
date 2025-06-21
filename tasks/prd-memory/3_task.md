---
status: pending
---

<task_context>
<domain>engine/agent</domain>
<type>integration</type>
<scope>configuration</scope>
<complexity>high</complexity>
<dependencies>task_1,task_2</dependencies>
</task_context>

# Task 3.0: Create Fixed Configuration Resolution System

## Overview

Extend the existing `engine/agent/config.go` to support three-tier memory configuration system. This follows the established pattern where each config type (task, workflow, agent) implements the `core.Config` interface. The agent config already exists and implements the interface - we just need to add memory-specific fields and validation.

## Subtasks

- [ ] 3.1 Add memory configuration fields to existing AgentConfig struct
- [ ] 3.2 Implement memory configuration parsing within existing Validate() method
- [ ] 3.3 Add memory validation to agent validators following existing patterns
- [ ] 3.4 Support three-tier configuration resolution through field detection
- [ ] 3.5 Test all configuration levels with existing test patterns

## Implementation Details

Extend the existing `engine/agent/config.go` by adding memory fields:

```go
type Config struct {
    // ... existing fields ...

    // Memory configuration - supports three levels
    Memory     any             `json:"memory,omitempty"     yaml:"memory,omitempty"     mapstructure:"memory,omitempty"`
    Memories   []any           `json:"memories,omitempty"   yaml:"memories,omitempty"   mapstructure:"memories,omitempty"`
    MemoryKey  string          `json:"memory_key,omitempty" yaml:"memory_key,omitempty" mapstructure:"memory_key,omitempty"`

    // ... rest of existing fields ...
}
```

**Configuration Levels**:

- **Level 1**: `memory: "customer-support-context"` (string type)
- **Level 2**: `memory: true` + `memories: ["id1", "id2"]` + optional `memory_key`
- **Level 3**: `memories: [{id: "id1", mode: "append", key: "custom"}]`

The existing `Validate()` method should be extended to:

- Detect configuration level based on field types
- Parse memory configuration into normalized format
- Validate memory IDs exist in the project (via registry lookup)
- Apply defaults (read-write mode for simplified configs)
- Use existing `schema.NewCompositeValidator` pattern

Memory validation should be added to `engine/agent/validators.go` following the existing `ActionsValidator` pattern. This maintains consistency with how other validations are structured in the agent package.

# Relevant Files

## Core Implementation Files

- `engine/agent/config.go` - Extend existing agent config with memory fields
- `engine/agent/validators.go` - Add memory-specific validation rules
- `engine/memory/types.go` - Memory configuration data models
- `engine/core/config.go` - Config interface to implement

## Test Files

- `engine/agent/config_test.go` - Extend with memory configuration tests
- `engine/agent/validators_test.go` - Add memory validation tests

## Configuration Files

- `memories/customer-support.yaml` - Example memory resource file

## Success Criteria

- All three configuration levels work with correct YAML parsing
- Level detection logic properly identifies patterns through field types
- Memory ID validation ensures referenced memories exist in project config
- Smart defaults applied correctly (read-write mode for simplified configs)
- Error handling provides helpful validation messages for configuration issues
- Backward compatibility maintained with existing configurations

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
