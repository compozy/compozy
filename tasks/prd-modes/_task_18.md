## status: completed

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>cli_help</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 18.0: Update CLI Help

## Overview

Update CLI help documentation and inline help text to reflect new mode system. Ensure --mode flag description accurately describes all three modes and default behavior.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4.5)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Update global flags help documentation
- Update inline CLI help for --mode flag
- Ensure environment variable documentation is accurate
- Brief but clear description of each mode
- Default mode (memory) is clearly stated
</requirements>

## Subtasks

- [x] 18.1 Update cli/help/global-flags.md
- [x] 18.2 Verify CLI flag help text matches documentation
- [x] 18.3 Update environment variable documentation (COMPOZY_MODE)
- [x] 18.4 Ensure consistency across all help outputs

## Implementation Details

See `tasks/prd-modes/_techspec.md` Section 4.5 for complete implementation details.

**Updated --mode Flag Description:**

```markdown
### --mode

Deployment mode: memory (default), persistent, or distributed

- **memory**: In-memory SQLite, embedded services (fastest)
- **persistent**: File-based SQLite, embedded services (local dev)
- **distributed**: PostgreSQL, external services (production)

**Default:** memory

**Environment:** COMPOZY_MODE
```

**Key Points:**
- Clear one-line summary for each mode
- Default mode explicitly stated
- Use cases in parentheses
- Environment variable documented

### Relevant Files

- `cli/help/global-flags.md` (PRIMARY)
- CLI flag definitions (verify inline help matches docs)

### Dependent Files

- `pkg/config/resolver.go` (ensure default matches documentation)
- `docs/content/docs/configuration/mode-configuration.mdx` (Task 15.0)

## Deliverables

- [ ] Updated `cli/help/global-flags.md` with new mode descriptions
- [ ] Verified inline CLI help matches documentation
- [ ] Updated environment variable documentation
- [ ] Consistent help text across all CLI commands

## Tests

Documentation verification (no automated tests):
- [ ] `compozy --help` shows correct mode description
- [ ] `compozy start --help` shows correct mode flag
- [ ] Environment variable (COMPOZY_MODE) is documented
- [ ] Default mode is stated as "memory"
- [ ] All three modes are described
- [ ] Help text is concise and clear

## Success Criteria

- CLI help documentation is accurate and complete
- Inline help matches written documentation
- Mode descriptions are brief but informative
- Default mode is clearly stated
- Environment variable usage is documented
- No references to old "standalone" mode
