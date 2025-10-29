## status: pending

<task_context>
<domain>examples</domain>
<type>validation</type>
<scope>user_documentation</scope>
<complexity>medium</complexity>
<dependencies>all_previous_phases</dependencies>
</task_context>

# Task 23.0: Validate Examples

## Overview

Test all example configurations in each mode to ensure they work correctly and demonstrate proper usage patterns for users.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 6.2 before start
- **DEPENDENCIES:** Tasks 1.0-21.0 must be completed
</critical>

<research>
# When you need information about example validation:
- check existing example patterns in examples/
- verify example configs are complete and runnable
</research>

<requirements>
- Memory mode example works and starts instantly
- Persistent mode example creates .compozy/ directory structure
- Distributed mode example connects to external services
- All examples are complete and runnable
- Example READMEs are clear and accurate
</requirements>

## Subtasks

- [ ] 23.1 Test memory mode example
- [ ] 23.2 Test persistent mode example
- [ ] 23.3 Test distributed mode example
- [ ] 23.4 Verify example directory structure
- [ ] 23.5 Validate example documentation

## Implementation Details

See `_techspec.md` Phase 6.2 for complete implementation details.

### Example Testing

**Memory mode:**
```bash
cd examples/memory-mode
compozy start
# Expected: Instant startup, no .compozy/ directory
```

**Persistent mode:**
```bash
cd examples/persistent-mode
compozy start
ls -la .compozy/
# Expected: compozy.db, temporal.db, redis/ created
# Restart test
compozy stop
compozy start
# Expected: Previous state persists
```

**Distributed mode:**
```bash
cd examples/distributed-mode
docker-compose up -d
compozy start
# Expected: Connects to postgres, redis, temporal
```

### Relevant Files

**Example directories:**
- `examples/memory-mode/` (renamed from standalone)
- `examples/persistent-mode/` (new)
- `examples/distributed-mode/` (updated)
- `examples/README.md`

**Example configs:**
- `examples/memory-mode/compozy.yaml`
- `examples/persistent-mode/compozy.yaml`
- `examples/distributed-mode/compozy.yaml`

### Dependent Files

- `pkg/config/resolver.go`
- `engine/infra/server/server.go`

## Deliverables

- All example configs tested and working
- Memory mode example demonstrates instant startup
- Persistent mode example shows file structure creation
- Distributed mode example connects to external services
- Example README files are accurate and complete

## Tests

- Example validation:
  - [ ] Memory mode example starts instantly
  - [ ] Memory mode example runs workflows successfully
  - [ ] Persistent mode example creates .compozy/ directory
  - [ ] Persistent mode example persists state across restarts
  - [ ] Distributed mode example connects to external services
  - [ ] All example configs are valid YAML
  - [ ] Example READMEs are clear and complete
  - [ ] Examples demonstrate best practices

## Success Criteria

- ✅ All examples work in their respective modes
- ✅ Memory mode: instant startup, no persistence
- ✅ Persistent mode: .compozy/ directory created with db files
- ✅ Distributed mode: successful external service connections
- ✅ Example documentation is clear and accurate
- ✅ Examples demonstrate proper mode usage patterns
