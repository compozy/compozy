## status: pending

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>migration_guides</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 16.0: Create Migration Guide

## Overview

Create comprehensive migration guide covering transitions between modes and migration from alpha versions. Document common issues, data export/import procedures, and provide troubleshooting guidance.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4.3)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Rename existing migration guide to mode-migration-guide.mdx
- Document all migration paths (memory → persistent → distributed)
- Provide alpha version migration instructions (standalone → memory/persistent)
- Include data export/import procedures
- Document common issues and solutions
- Clear step-by-step instructions for each migration path
</requirements>

## Subtasks

- [ ] 16.1 Rename and restructure existing migration guide
- [ ] 16.2 Document alpha version migration (standalone → memory/persistent)
- [ ] 16.3 Add memory → persistent migration path
- [ ] 16.4 Add persistent → distributed migration path
- [ ] 16.5 Document common issues (pgvector, concurrency limits)
- [ ] 16.6 Add data export/import procedures

## Implementation Details

See `tasks/prd-modes/_techspec.md` Section 4.3 for complete implementation details.

**Migration Paths to Document:**

**Alpha Version Migration:**
- Old `standalone` → New `memory` (for testing/ephemeral)
- Old `standalone` → New `persistent` (for development with persistence)
- Old `distributed` → New `distributed` (no changes)

**Mode Transitions:**
- **Memory → Persistent**: Add persistence configuration
- **Persistent → Distributed**: Export data, update config, import data

**Common Issues:**
- pgvector incompatibility with SQLite (solution: use Qdrant/Redis)
- Concurrent workflow limits with SQLite (solution: migrate to distributed)
- Configuration validation errors

### Relevant Files

- `docs/content/docs/guides/migrate-standalone-to-distributed.mdx` → rename to `mode-migration-guide.mdx`

### Dependent Files

- `docs/content/docs/deployment/memory-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/persistent-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/distributed-mode.mdx` (Task 14.0)
- `docs/content/docs/configuration/mode-configuration.mdx` (Task 15.0)

## Deliverables

- [ ] Renamed and updated `mode-migration-guide.mdx`
- [ ] Alpha version migration instructions
- [ ] All migration paths documented with examples
- [ ] Data export/import procedures
- [ ] Common issues and troubleshooting section
- [ ] Working code examples for each migration

## Tests

Documentation verification (no automated tests):
- [ ] All migration commands are valid and tested
- [ ] YAML examples are syntactically correct
- [ ] Data export/import procedures work
- [ ] Common issues have actionable solutions
- [ ] Migration paths are complete and sequential
- [ ] No broken references to old mode names

## Success Criteria

- All migration paths are clearly documented
- Alpha version migration is straightforward
- Data export/import procedures are complete
- Common issues have clear solutions
- Step-by-step instructions are easy to follow
- Examples work and are validated
- No references to outdated mode names (except historical context)
