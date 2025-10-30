## status: pending

<task_context>
<domain>error_handling</domain>
<type>validation</type>
<scope>user_experience</scope>
<complexity>low</complexity>
<dependencies>all_previous_phases</dependencies>
</task_context>

# Task 25.0: Error Message Validation

## Overview

Validate that all error messages are helpful, clear, and guide users toward correct configuration. Test error scenarios to ensure quality user experience.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 6.4 before start
- **DEPENDENCIES:** Tasks 1.0-23.0 must be completed
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about error handling:
- check existing error patterns in the codebase
- verify error messages follow Go best practices
</research>

<requirements>
- Invalid mode errors show valid options
- pgvector + SQLite error provides clear guidance
- SQLite concurrency warnings are informative
- Migration hints for "standalone" mode users
- All error messages are actionable
</requirements>

## Subtasks

- [ ] 25.1 Test invalid mode error message
- [ ] 25.2 Test pgvector + SQLite incompatibility error
- [ ] 25.3 Test SQLite concurrency warning
- [ ] 25.4 Test "standalone" migration hint
- [ ] 25.5 Verify all error messages are clear and actionable

## Implementation Details

See `_techspec.md` Phase 6.4 for complete implementation details.

### Error Scenarios to Test

**Invalid mode:**
```bash
compozy start --mode invalid
# Expected error:
# Error: invalid mode "invalid". Valid modes: memory, persistent, distributed
```

**pgvector + SQLite:**
```bash
cat > test-config.yaml <<EOF
mode: memory
knowledge:
  vector_dbs:
    - provider: pgvector
EOF

compozy start --config test-config.yaml
# Expected error:
# Error: pgvector provider is incompatible with SQLite driver.
# SQLite requires an external vector database.
# Configure one of: Qdrant, Redis, or Filesystem.
# See documentation: docs/database/sqlite.md#vector-database-requirement
```

**SQLite concurrency warning:**
```bash
cat > test-config.yaml <<EOF
mode: memory
worker:
  max_concurrent_workflow_execution_size: 50
EOF

compozy start --config test-config.yaml
# Expected warning:
# WARN SQLite has concurrency limitations max_concurrent_workflows=50 recommended_max=10
# note="Consider using mode: distributed for high-concurrency workloads"
```

**Standalone migration hint:**
```bash
cat > test-config.yaml <<EOF
mode: standalone
EOF

compozy start --config test-config.yaml
# Expected error:
# Error: invalid mode "standalone"
# Hint: The "standalone" mode has been replaced with:
#   - mode: memory (for ephemeral testing)
#   - mode: persistent (for file-based storage)
# See migration guide: docs/guides/mode-migration-guide.mdx
```

### Relevant Files

**Error handling code:**
- `pkg/config/config.go` (validation)
- `pkg/config/loader.go` (custom validation)
- `engine/infra/server/dependencies.go` (validateDatabaseConfig)

### Dependent Files

- Validation logic from Tasks 1.2, 2.2

## Deliverables

- All error scenarios tested and documented
- Error messages are clear and actionable
- Migration hints help users upgrade from alpha
- Validation errors provide next steps
- Error documentation is complete

## Tests

- Error message validation:
  - [ ] Invalid mode shows valid options
  - [ ] Invalid mode includes migration hint if "standalone"
  - [ ] pgvector + SQLite shows clear incompatibility error
  - [ ] pgvector error suggests alternative vector DBs
  - [ ] SQLite concurrency warning recommends distributed mode
  - [ ] All errors include actionable guidance
  - [ ] Error messages reference documentation
  - [ ] Errors are formatted clearly and consistently

## Success Criteria

- ✅ All error messages tested and validated
- ✅ Invalid mode errors are helpful and clear
- ✅ pgvector + SQLite error guides users to solutions
- ✅ Concurrency warnings are informative
- ✅ Migration hints help users upgrade configs
- ✅ All errors reference relevant documentation
- ✅ No confusing or misleading error messages
