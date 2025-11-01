## status: completed

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>deployment_guides</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 14.0: Update Deployment Documentation

## Overview

Rename and update deployment documentation to reflect new three-mode system (memory/persistent/distributed). This involves creating new mode-specific guides and updating existing distributed mode documentation with comparison tables.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4.1)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Rename standalone-mode.mdx to memory-mode.mdx with updated content
- Create new persistent-mode.mdx documentation
- Update distributed-mode.mdx with mode comparison table
- All examples must be working and accurate
- Clear use case guidance for each mode
- Consistent structure across all mode documentation
</requirements>

## Subtasks

- [x] 14.1 Rename and update standalone-mode.mdx to memory-mode.mdx
- [x] 14.2 Create new persistent-mode.mdx documentation
- [x] 14.3 Update distributed-mode.mdx with comparison section
- [x] 14.4 Verify all cross-references between mode docs

## Implementation Details

See `tasks/prd-modes/_techspec.md` Section 4.1 for complete implementation details.

**Key Changes:**

**Memory Mode (renamed from standalone):**
- Document in-memory SQLite, embedded Temporal, embedded Redis
- Use cases: testing, rapid development, CI/CD pipelines
- Characteristics: instant startup, no persistence, fastest execution
- Limitations: no pgvector support, write concurrency limits

**Persistent Mode (NEW):**
- Document file-based SQLite with persistence
- Use cases: local development, debugging, small teams
- Default paths: `./.compozy/` directory structure
- Backup and recovery procedures
- Same limitations as memory mode

**Distributed Mode (updated):**
- Add mode comparison table at top
- Highlight production readiness
- Clear guidance on when to use distributed vs other modes

### Relevant Files

- `docs/content/docs/deployment/standalone-mode.mdx` → rename to `memory-mode.mdx`
- `docs/content/docs/deployment/persistent-mode.mdx` (NEW)
- `docs/content/docs/deployment/distributed-mode.mdx` (UPDATE)

### Dependent Files

- `docs/content/docs/configuration/mode-configuration.mdx` (Task 15.0)
- `docs/content/docs/guides/mode-migration-guide.mdx` (Task 16.0)

## Deliverables

- [x] `memory-mode.mdx` with updated content and use cases
- [x] `persistent-mode.mdx` with complete configuration examples
- [x] `distributed-mode.mdx` with mode comparison table
- [x] All internal links updated and working
- [x] Consistent MDX formatting and structure

## Tests

Documentation verification (no automated tests):
- [x] All code examples are syntactically valid YAML
- [x] All cross-references resolve correctly
- [x] Mode comparison table is accurate
- [x] Use case guidance is clear and actionable
- [x] No references to old "standalone" naming (except in migration context)

## Success Criteria

- All three mode documentation pages exist and are complete
- Mode comparison table accurately reflects differences
- Clear guidance on when to use each mode
- Examples work and follow best practices
- No broken internal links
- Consistent structure and formatting across all mode docs
