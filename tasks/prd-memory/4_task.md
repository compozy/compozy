---
status: pending
---

<task_context>
<domain>engine/agent</domain>
<type>integration</type>
<scope>configuration</scope>
<complexity>high</complexity>
<dependencies>task_1</dependencies>
</task_context>

# Task 4.0: Create Fixed Configuration Resolution System

## Overview

Implement the three-tier agent memory configuration system with proper YAML parsing and validation. This system provides progressive complexity from ultra-simple single-line setup to fully customizable multi-memory configurations while maintaining clear validation and error messages.

## Subtasks

- [ ] 4.1 Build ConfigurationResolver with ResolveMemoryConfig method
- [ ] 4.2 Implement parseAdvancedMemoryConfig for full MemoryReference objects
- [ ] 4.3 Add smart defaults and memory_key validation
- [ ] 4.4 Create ConfigValidator for memory ID existence checks
- [ ] 4.5 Support AgentConfig with interface{} types for flexible YAML parsing

## Implementation Details

Create `ConfigurationResolver` supporting three configuration levels:

**Level 1**: Direct memory ID (`memory: "customer-support-context"`)
**Level 2**: Simple multi-memory (`memory: true` + `memories: [...]` + `memory_key`)  
**Level 3**: Advanced configuration (`memories: [{id, mode, key}]`)

The resolver must handle YAML parsing correctly by detecting configuration levels through field type examination. Level 2 requires `memory: true` combined with string array in `memories` field. Level 3 uses full reference objects with validation.

Add comprehensive error messages for configuration issues, smart defaults (read-write mode), and required memory_key validation for simplified patterns.

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
