## status: pending

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 19.0: Create/Update Examples

## Overview

Create or update example configurations for each mode (memory, persistent, distributed). Ensure examples demonstrate common patterns, best practices, and mode-specific features.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Create/update example for memory mode
- Create/update example for persistent mode
- Create/update example for distributed mode
- Examples must demonstrate mode-specific features
- All examples must be tested and working
- Clear comments explaining mode-specific configuration
</requirements>

## Subtasks

- [ ] 19.1 Create/update memory mode example
- [ ] 19.2 Create/update persistent mode example
- [ ] 19.3 Create/update distributed mode example
- [ ] 19.4 Add inline comments explaining mode-specific features
- [ ] 19.5 Test all examples to ensure they work
- [ ] 19.6 Update examples index/navigation

## Implementation Details

Examples should demonstrate:

**Memory Mode Example:**
- Minimal configuration (mode can be omitted as it's default)
- Fast startup for testing/development
- No external dependencies
- Typical use case: testing or quick experimentation

**Persistent Mode Example:**
- File-based storage configuration
- Custom paths for database and Temporal
- Redis persistence settings
- Typical use case: local development with state preservation

**Distributed Mode Example:**
- External PostgreSQL configuration
- External Temporal cluster
- External Redis cluster
- Production-ready settings
- Typical use case: production deployment

**Example Structure:**
```yaml
name: example-workflow
mode: [memory|persistent|distributed]  # Explicit mode

# Mode-specific configuration
database:
  # ... mode-appropriate settings

temporal:
  # ... mode-appropriate settings

redis:
  # ... mode-appropriate settings

# Common workflow configuration
models:
  - provider: openai
    model: gpt-4o-mini
    api_key: "${OPENAI_API_KEY}"

tasks:
  # ... example tasks
```

### Relevant Files

- `docs/content/docs/examples/memory-mode.mdx` (CREATE or UPDATE)
- `docs/content/docs/examples/persistent-mode.mdx` (CREATE or UPDATE)
- `docs/content/docs/examples/distributed-mode.mdx` (CREATE or UPDATE)
- `examples/configs/memory-mode.yaml` (CREATE)
- `examples/configs/persistent-mode.yaml` (CREATE)
- `examples/configs/distributed-mode.yaml` (CREATE)

### Dependent Files

- `docs/content/docs/configuration/mode-configuration.mdx` (Task 15.0)
- `docs/content/docs/deployment/*.mdx` (Task 14.0)

## Deliverables

- [ ] Working memory mode example with documentation
- [ ] Working persistent mode example with documentation
- [ ] Working distributed mode example with documentation
- [ ] Inline comments explaining mode-specific settings
- [ ] Updated examples navigation/index
- [ ] All examples tested and validated

## Tests

Documentation verification (no automated tests):
- [ ] Memory mode example runs successfully
- [ ] Persistent mode example runs successfully
- [ ] Distributed mode example runs successfully (with required services)
- [ ] All YAML is syntactically valid
- [ ] Comments are clear and helpful
- [ ] Examples demonstrate mode-specific features
- [ ] Configuration follows best practices

## Success Criteria

- Three complete mode examples exist
- Each example demonstrates mode-specific features
- Examples are well-documented with inline comments
- All examples have been tested and work
- Clear use case guidance in each example
- Examples follow configuration best practices
- No references to old "standalone" mode
