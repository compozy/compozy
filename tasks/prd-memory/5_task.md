---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>task_1,task_4</dependencies>
</task_context>

# Task 5.0: Build Memory Registry and Resource Loading System

## Overview

Create project-level memory resource management with loading, validation, and lookup capabilities. This system manages memory resources as first-class project entities with support for separate YAML files, versioning, and multi-tenant safety.

## Subtasks

- [ ] 5.1 Implement MemoryRegistry with resource storage and ID-based lookup
- [ ] 5.2 Create MemoryResourceLoader for parsing memory definitions
- [ ] 5.3 Add resource validation for priority blocks, token allocation, and flushing strategies
- [ ] 5.4 Support memory resource files in memories/ directory
- [ ] 5.5 Add project-level isolation and key sanitization

## Implementation Details

Implement `MemoryRegistry` as the central repository for memory resources with ID-based lookup and validation. Create `MemoryResourceLoader` that can parse memory definitions from:

- Project configuration (`compozy.yaml` memories section)
- Separate YAML files (`memories/customer-support.yaml`)

Add comprehensive validation for enhanced memory configurations including priority blocks, token allocation ratios, and flushing strategies. Support resource versioning and description fields for documentation.

Implement multi-tenant safety with key sanitization using character whitelist `[a-zA-Z0-9-_\.:]`, length limits (max 512 chars), and automatic namespacing pattern: `compozy:{project_id}:memory:{user_key}`.

## Success Criteria

- Memory resources load correctly from both project config and separate files
- Complex memory configurations validate properly (priorities, allocations, flushing)
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
