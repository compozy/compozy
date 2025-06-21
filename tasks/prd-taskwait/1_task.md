---
status: pending
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>baseconfig</dependencies>
</task_context>

# Task 1.0: Implement WaitTaskConfig Structure

## Overview

Create the WaitTaskConfig struct extending BaseConfig with wait-specific fields following established Compozy patterns. This foundational task establishes the configuration structure that enables workflows to pause execution until receiving specific external signals.

## Subtasks

- [ ] 1.1 Create WaitTaskConfig struct in engine/task/ with proper YAML/JSON tags
- [ ] 1.2 Implement ProcessorSpec struct for optional signal processing
- [ ] 1.3 Add TaskTypeWait to the Type enum
- [ ] 1.4 Ensure proper mapstructure support for configuration parsing
- [ ] 1.5 Add validation tags and struct documentation

## Implementation Details

The WaitTaskConfig extends BaseConfig following established Compozy patterns:

```go
type WaitTaskConfig struct {
    BaseConfig                                    // Standard task configuration
    WaitFor      string         `yaml:"wait_for"`      // REQUIRED: Signal name to wait for
    Condition    string         `yaml:"condition"`     // REQUIRED: CEL expression
    Processor    *ProcessorSpec `yaml:"processor"`     // Optional: Signal processing task
    OnTimeout    string         `yaml:"on_timeout"`    // Optional: Timeout routing
}

type ProcessorSpec struct {
    BaseConfig                                    // Inherit timeout, retries, etc.
    ID       string            `yaml:"id"`
    Type     string            `yaml:"type"`       // basic, docker, wasm
    Use      string            `yaml:"$use"`       // Tool reference
    With     map[string]any    `yaml:"with"`       // Input parameters
}
```

Key requirements:

- Extend BaseConfig to inherit standard task fields (id, type, timeout, on_success, on_error)
- Support YAML/JSON marshaling with proper tags
- Include mapstructure tags for configuration parsing
- Follow established naming conventions and struct patterns

## Success Criteria

- [ ] WaitTaskConfig struct properly extends BaseConfig
- [ ] All required fields have appropriate YAML/JSON/mapstructure tags
- [ ] TaskTypeWait is added to Type enum and properly integrated
- [ ] Struct validates correctly with existing configuration framework
- [ ] Code follows Go coding standards and architecture patterns
- [ ] Unit tests pass for struct validation and YAML parsing

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
