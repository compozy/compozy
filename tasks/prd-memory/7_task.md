---
status: completed
---

<task_context>
<domain>engine/agent</domain>
<type>integration</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>task_3,task_6</dependencies>
</task_context>

# Task 7.0: Integrate Enhanced Memory System with Agent Runtime

## Overview

Extend the existing agent configuration structure (`engine/agent/config.go`) to support memory references with minimal changes. This enables agents to leverage shared memory resources within the existing framework.

## Subtasks

- [ ] 7.1 Add memory configuration fields to existing `agent.Config` struct
- [ ] 7.2 Update existing `Validate()` method to include memory reference validation
- [ ] 7.3 Create lightweight memory resolver following existing patterns
- [ ] 7.4 Add memory interface resolution in agent initialization
- [ ] 7.5 Ensure backward compatibility - agents without memory continue working

## Implementation Details

Extend the existing `engine/agent/config.go` with minimal additive changes:

```go
// Add these fields to the existing Config struct (after line 29)
type Config struct {
    // ... existing fields (lines 17-29) ...
    MaxIterations int           `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty" mapstructure:"max_iterations,omitempty"`
    JSONMode      bool          `json:"json_mode"                yaml:"json_mode"                mapstructure:"json_mode"`

    // Memory configuration (optional) - NEW FIELDS
    Memory    interface{} `json:"memory,omitempty"     yaml:"memory,omitempty"     mapstructure:"memory,omitempty"`
    Memories  interface{} `json:"memories,omitempty"   yaml:"memories,omitempty"   mapstructure:"memories,omitempty"`
    MemoryKey string      `json:"memory_key,omitempty" yaml:"memory_key,omitempty" mapstructure:"memory_key,omitempty"`

    filePath string
    CWD      *core.PathCWD
}
```

Extend the existing `Validate()` method (minimal change to existing method):

```go
func (a *Config) Validate() error {
    v := schema.NewCompositeValidator(
        schema.NewCWDValidator(a.CWD, a.ID),
        NewActionsValidator(a.Actions),
        schema.NewStructValidator(a),
    )
    if err := v.Validate(); err != nil {
        return err
    }
    var mcpErrors []error
    for i := range a.MCPs {
        if err := a.MCPs[i].Validate(); err != nil {
            mcpErrors = append(mcpErrors, fmt.Errorf("mcp validation error: %w", err))
        }
    }
    // NEW: Add memory validation following the same pattern as MCP validation
    if a.Memory != nil || a.Memories != nil {
        if err := a.validateMemoryConfig(); err != nil {
            return fmt.Errorf("memory validation error: %w", err)
        }
    }
    if len(mcpErrors) > 0 {
        return errors.Join(mcpErrors...)
    }
    return nil
}

// NEW: Add this private method following existing validation patterns
func (a *Config) validateMemoryConfig() error {
    // Simple validation - ensure memory references are strings or valid config maps
    if a.Memory != nil {
        switch v := a.Memory.(type) {
        case string:
            if v == "" {
                return fmt.Errorf("memory reference cannot be empty")
            }
        case map[string]interface{}:
            // Valid memory config map
        default:
            return fmt.Errorf("memory must be a string reference or config map")
        }
    }
    // Similar validation for Memories field if needed
    return nil
}
```

Key principles:

- Changes are purely additive - no modification to existing fields or behavior
- Memory fields are optional with `omitempty` tags
- Validation follows existing patterns (similar to MCP validation)
- Uses existing error handling patterns (fmt.Errorf, errors.Join)
- No changes to existing methods except adding memory validation to `Validate()`

The changes maintain full backward compatibility - agents without memory configuration continue to work exactly as before.

# Relevant Files

## Core Implementation Files

- `engine/agent/config.go` - EXISTING file - add memory fields to Config struct
- `engine/agent/memory_resolver.go` - NEW file - lightweight memory configuration resolver
- `engine/memory/interfaces.go` - Memory interfaces for agent integration

## Test Files

- `engine/agent/config_test.go` - Update existing tests to cover memory configuration
- `engine/agent/memory_resolver_test.go` - Tests for memory configuration resolution

## Success Criteria

- Agent configuration resolution works for all three complexity levels
- Memory references validate correctly at agent startup
- Multiple memory instances per agent work with different access modes
- Dependency injection provides Memory interfaces to LLM orchestrator
- Configuration migration maintains backward compatibility
- Error propagation from memory operations works correctly

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
