---
status: completed # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/project</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 4.0: Update Configuration Structures

## Overview

Update project and runtime configuration structures to support the new entrypoint-based architecture and runtime selection. Remove all Deno-specific configuration options.

## Subtasks

- [x] 4.1 Update `RuntimeConfig` in `engine/project/config.go` with new fields
- [x] 4.2 Update `engine/runtime/config.go` with runtime-specific options
- [x] 4.3 Remove `execute` property from `engine/tool/config.go`
- [x] 4.4 Add configuration validation for new fields
- [x] 4.5 Update configuration loading and merging logic
- [x] 4.6 Write tests for configuration changes

## Implementation Details

### New RuntimeConfig Structure

```go
type RuntimeConfig struct {
    Type        string   `json:"type,omitempty"        yaml:"type,omitempty"`        // "bun" | "node"
    Entrypoint  string   `json:"entrypoint"            yaml:"entrypoint"`            // Required: path to entrypoint file
    Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}
```

### Runtime-Specific Configuration

```go
type Config struct {
    // Remove Deno fields, add:
    RuntimeType          string        // "bun" or "node"
    EntrypointPath       string        // Path to entrypoint file
    BunPermissions       []string      // Bun-specific permissions
    NodeOptions          []string      // Node.js-specific options
    ToolExecutionTimeout time.Duration // Keep existing timeout
}
```

### Tool Configuration Changes

```go
// Remove Execute field from tool.Config
type Config struct {
    Resource     string         `json:"resource,omitempty"    yaml:"resource,omitempty"`
    ID           string         `json:"id,omitempty"          yaml:"id,omitempty"`
    Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
    // Execute field removed
    Timeout      string         `json:"timeout,omitempty"     yaml:"timeout,omitempty"`
    InputSchema  *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"`
    OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"`
    With         *core.Input    `json:"with,omitempty"        yaml:"with,omitempty"`
    Env          *core.EnvMap   `json:"env,omitempty"         yaml:"env,omitempty"`
}
```

### Validation Requirements

- Entrypoint path must exist when specified
- Runtime type must be valid ("bun", "node", or empty defaults to "bun")
- Permissions array validated based on runtime type

## Success Criteria

- Configuration structures support new architecture
- Validation catches invalid configurations early
- All Deno-specific configuration removed
- Execute property removed from tool configuration
- Tests cover all configuration scenarios
- Documentation updated with examples

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
