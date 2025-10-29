## status: pending

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>configuration_guides</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 15.0: Update Configuration Documentation

## Overview

Update mode configuration documentation to reflect new three-mode system. Document mode resolution order, component override capabilities, and provide clear examples for each mode.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4.2)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Update mode-configuration.mdx with new modes (memory/persistent/distributed)
- Document mode resolution order clearly
- Provide component override examples
- Link to mode-specific deployment guides
- All YAML examples must be valid and tested
</requirements>

## Subtasks

- [ ] 15.1 Update mode options section with three modes
- [ ] 15.2 Document mode resolution order (component → global → default)
- [ ] 15.3 Add component override examples
- [ ] 15.4 Link to deployment guides for each mode
- [ ] 15.5 Verify all configuration examples

## Implementation Details

See `tasks/prd-modes/_techspec.md` Section 4.2 for complete implementation details.

**Key Sections to Update:**

**Mode Options:**
- memory (default): SQLite :memory:, embedded services, zero dependencies
- persistent: SQLite file, embedded services with persistence
- distributed: PostgreSQL, external Temporal, external Redis

**Resolution Order:**
1. Component-specific mode (if set)
2. Global mode (if set)
3. Default (memory)

**Component Override Examples:**
- Global memory mode with persistent Temporal
- Mixed mode configurations for hybrid deployments
- Per-component configuration options

### Relevant Files

- `docs/content/docs/configuration/mode-configuration.mdx` (PRIMARY)

### Dependent Files

- `docs/content/docs/deployment/memory-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/persistent-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/distributed-mode.mdx` (Task 14.0)
- `docs/content/docs/examples/memory-mode.mdx` (Task 19.0)
- `docs/content/docs/examples/persistent-mode.mdx` (Task 19.0)
- `docs/content/docs/examples/distributed-mode.mdx` (Task 19.0)

## Deliverables

- [ ] Updated `mode-configuration.mdx` with three-mode system
- [ ] Clear mode resolution order documentation
- [ ] Component override examples
- [ ] Working links to deployment and example pages
- [ ] Valid YAML configuration examples

## Tests

Documentation verification (no automated tests):
- [ ] All YAML examples are syntactically correct
- [ ] Mode resolution order is clearly explained
- [ ] Component override examples work as documented
- [ ] All internal links resolve correctly
- [ ] Default mode (memory) is clearly stated
- [ ] Examples cover common use cases

## Success Criteria

- Configuration documentation accurately reflects new modes
- Mode resolution order is clear and unambiguous
- Component override patterns are well-documented
- All examples are valid and tested
- Links to deployment guides work correctly
- No references to old "standalone" mode (except migration context)
