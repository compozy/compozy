---
status: pending # Options: pending, in-progress, completed, excluded
parallelizable: false # Whether this task can run in parallel when preconditions are met
blocked_by: ["1.0", "4.0"] # List of task IDs that must be completed first
---

<task_context>
<domain>engine/attachment</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>pkg/tplengine|filesystem</dependencies>
<unblocks>"5.0"</unblocks>
</task_context>

# Task 3.0: Normalization & Template Integration

## Overview

Implement the sophisticated normalization and template integration system that expands pluralized sources (`paths`/`urls`) into individual attachments with glob pattern support, and provides two-phase template resolution to handle runtime dependencies like `.tasks.*` references.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Structural normalization to expand `paths`/`urls` into individual attachments
- Glob pattern support using `github.com/bmatcuk/doublestar/v4` for recursive patterns
- Two-phase template engine integration with `pkg/tplengine`
- Phase 1: Evaluate templates with deferral of unresolved `.tasks.*` references during normalization
- Phase 2: Re-evaluate deferred templates at execution time with full runtime context
- Metadata inheritance from pluralized sources to expanded individual attachments
- Template-enabled fields: `url`, `path`, `urls[]`, `paths[]`, `name`, `mime`, and string values in `meta`
</requirements>

## Subtasks

- [ ] 3.1 Implement structural normalization (`normalize.go`) to expand `paths`/`urls` into individual attachments
- [ ] 3.2 Implement glob pattern support for `paths` using `github.com/bmatcuk/doublestar/v4`
- [ ] 3.3 Design two-phase template engine integration architecture (`context_normalization.go`)
- [ ] 3.4 Implement Phase 1: Template evaluation with deferral of unresolved `.tasks.*` references during normalization
- [ ] 3.5 Implement Phase 2: Re-evaluate deferred templates at execution time with full runtime context
- [ ] 3.6 Add metadata inheritance from pluralized sources to expanded individual attachments
- [ ] 3.7 Unit tests for glob expansion, template deferral logic, and metadata inheritance

## Sequencing

- Blocked by: 1.0 (Domain Model & Core Interfaces), 4.0 (Global Configuration & Schema Integration)
- Unblocks: 5.0 (Execution Wiring & Orchestrator Integration)
- Parallelizable: No (requires both domain model and config integration to be complete)

## Implementation Details

### Structural Normalization Logic

Expand pluralized sources into individual attachments:

- `URLs []string` → multiple individual `URL string` attachments
- `Paths []string` → multiple individual `Path string` attachments (with glob expansion)
- Preserve metadata (`name`, `meta`) from parent to all expanded children
- Maintain order for deterministic results

### Glob Pattern Support

Using `github.com/bmatcuk/doublestar/v4` for enhanced pattern matching:

- Support recursive `**` patterns (e.g., `./assets/**/*.png`)
- Handle edge cases: invalid patterns, permission errors, non-existent paths
- Security: Ensure expanded paths remain within CWD boundaries
- Performance: Stream results for large directory structures

### Two-Phase Template Resolution

Complex template integration with workflow context:

**Phase 1 (Normalization):**

- Evaluate template strings using available context
- **DEFER** any expressions containing `.tasks.*` that cannot be resolved yet
- Keep unresolved templates as-is for Phase 2
- Handle template evaluation errors gracefully with actionable messages

**Phase 2 (Execution):**

- Re-evaluate any remaining template strings with full runtime context
- Include completed task outputs in context (`.tasks.<task_id>.output.*`)
- Final template failures should be reported as execution errors

### Template Context Variables

Available context keys per technical specification:

- `.workflow.id`, `.workflow.exec_id`, `.workflow.input.*`
- `.input.*` (task-local inputs), `.env.*`, `.agents.*`, `.tools.*`, `.trigger.*`
- `.tasks.<task_id>.output.*` for chaining from prior tasks (note plural `.tasks`)

### Relevant Files

- `engine/attachment/normalize.go` - Structural expansion logic
- `engine/attachment/context_normalization.go` - Template integration
- `engine/attachment/normalize_test.go` - Normalization tests

### Dependent Files

- `pkg/tplengine/engine.go` - Template engine integration
- `engine/core/types.go` - For workflow context types
- `github.com/bmatcuk/doublestar/v4` - Glob pattern library

## Success Criteria

- Pluralized sources (`paths`, `urls`) correctly expand to individual attachments
- Glob patterns work with recursive `**` syntax and handle edge cases
- Metadata inheritance preserves parent properties on all expanded children
- Phase 1 template evaluation correctly defers `.tasks.*` references
- Phase 2 template evaluation resolves deferred expressions with runtime context
- Template evaluation errors provide clear, actionable error messages
- Security: All expanded paths remain within CWD boundaries
- Unit tests achieve >90% coverage including error and edge cases
- Performance: Glob expansion handles large directories efficiently
- All linter checks pass (`make lint`)
- All tests pass (`make test`)
