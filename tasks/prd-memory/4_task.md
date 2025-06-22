---
status: completed
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>task_1,task_2,task_3</dependencies>
</task_context>

# Task 4.0: Build Memory Registry and Resource Loading System

## Overview

Extend the existing `engine/autoload/registry.go` to support memory resources as first-class project entities. The ConfigRegistry already provides thread-safe storage, duplicate detection, and integration with AutoLoader - we just need to add memory resource support following the established patterns.

## Subtasks

- [ ] 4.1 Add ConfigMemory constant to core.ConfigType enum
- [ ] 4.2 Update registry's extractResourceType to recognize memory configs
- [ ] 4.3 Create memory.Config struct implementing Configurable interface
- [ ] 4.4 Update AutoLoader to scan memories/ directory
- [ ] 4.5 Add memory-specific validation following existing patterns

## Implementation Details

**1. Add to `engine/core/config.go`:**

```go
const (
    // ... existing types ...
    ConfigMemory ConfigType = "memory"
)
```

**2. Update `engine/autoload/registry.go` extractResourceType():**

```go
resourceTypeMap := map[string]string{
    // ... existing mappings ...
    "*memory.Config": string(core.ConfigMemory),
    "memory.Config":  string(core.ConfigMemory),
}
```

**3. Create `engine/memory/config.go` implementing Configurable:**

```go
type Config struct {
    Resource    string                `json:"resource,omitempty" yaml:"resource,omitempty"`
    ID          string                `json:"id" yaml:"id" validate:"required"`
    Type        string                `json:"type" yaml:"type" validate:"required,oneof=context summary"`
    Allocations []AllocationConfig    `json:"allocations,omitempty" yaml:"allocations,omitempty"`
    Flushing    *FlushingConfig       `json:"flushing,omitempty" yaml:"flushing,omitempty"`
    Version     string                `json:"version,omitempty" yaml:"version,omitempty"`
    Description string                `json:"description,omitempty" yaml:"description,omitempty"`
}

// Implement Configurable interface
func (c *Config) GetResource() string { return "memory" }
func (c *Config) GetID() string { return c.ID }
```

The existing ConfigRegistry will automatically:

- Handle thread-safe storage and retrieval
- Detect and prevent duplicate memory IDs
- Integrate with AutoLoader for memories/ directory scanning
- Provide case-insensitive lookups

Key sanitization and tenant isolation should be implemented in the memory package's business logic, not in the registry, maintaining proper separation of concerns.

# Relevant Files

## Core Implementation Files

- `engine/autoload/registry.go` - Extend existing ConfigRegistry
- `engine/autoload/loader.go` - Integrate memory loading with AutoLoader
- `engine/memory/config.go` - Memory resource config implementing Configurable
- `engine/memory/types.go` - Memory resource data models
- `engine/core/config.go` - Add ConfigMemory type constant

## Test Files

- `engine/autoload/registry_test.go` - Extend with memory resource tests
- `engine/memory/config_test.go` - Memory configuration tests

## Configuration Files

- `memories/customer-support.yaml` - Example memory resource file

## Success Criteria

- Memory resources load correctly from both project config and separate files
- Complex memory configurations validate properly (allocations, flushing)
- Resource versioning and descriptions support project documentation
- Key sanitization ensures Redis compatibility and multi-tenant security
- Project-level isolation prevents cross-project memory access
- Integration with project configuration system works seamlessly

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
