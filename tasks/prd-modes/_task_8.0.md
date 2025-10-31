## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database|temporal|redis</dependencies>
</task_context>

# Task 8.0: Manual Runtime Validation

## Overview

Perform comprehensive manual validation of runtime infrastructure behavior across all three modes to ensure correct component initialization, state persistence, and mode-specific behavior.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technicals docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Test server startup in each mode (memory/persistent/distributed)
- Verify correct infrastructure components activate per mode
- Validate state persistence behavior in persistent mode
- Verify ephemeral behavior in memory mode
- Confirm no regressions in distributed mode
- Validate error messages and warnings are clear and helpful
- Confirm default mode (memory) works without any configuration
</requirements>

## Subtasks

- [ ] 8.1 Manual validation: memory mode
- [ ] 8.2 Manual validation: persistent mode
- [ ] 8.3 Manual validation: distributed mode
- [ ] 8.4 Verify error handling and validation messages
- [ ] 8.5 Test default mode behavior (no config)
- [ ] 8.6 Document validation results and any issues

## Implementation Details

See **Phase 2.3: Update Server Logging** in `_techspec.md` (lines 756-783) for validation approach.

### Memory Mode Validation

```bash
# Start in memory mode (default)
compozy start

# Or explicitly
compozy start --mode memory

# Verify:
# - Server starts in <1 second
# - Logs show "mode=memory"
# - Database: SQLite :memory:
# - Temporal: embedded :memory:
# - Redis: Miniredis (no persistence)
# - No .compozy/ directory created
```

### Persistent Mode Validation

```bash
# Start in persistent mode
compozy start --mode persistent

# Verify:
# - Server starts in <2 seconds
# - Logs show "mode=persistent"
# - Database: SQLite file at ./.compozy/compozy.db
# - Temporal: file at ./.compozy/temporal.db
# - Redis: BadgerDB at ./.compozy/redis/
# - .compozy/ directory created with db files

# Test persistence:
# 1. Run a workflow
# 2. Stop server
# 3. Restart server
# 4. Verify workflow history persists
```

### Distributed Mode Validation

```bash
# Requires external services
docker-compose up -d postgres redis temporal

# Start in distributed mode
compozy start --mode distributed

# Verify:
# - Server starts in 5-15 seconds
# - Logs show "mode=distributed"
# - Database: PostgreSQL external
# - Temporal: external cluster
# - Redis: external cluster
# - No embedded services started
```

### Relevant Files

- `engine/infra/server/server.go` - Server initialization
- `engine/infra/server/dependencies.go` - Component startup
- `engine/infra/cache/mod.go` - Cache initialization
- Examples:
  - `examples/hello-world.yaml` - Simple workflow for testing

### Dependent Files

- All Phase 2 implementation files (Tasks 5.0-7.0)

## Deliverables

- Documented validation results for all three modes
- List of any issues or unexpected behaviors discovered
- Confirmation that infrastructure behaves correctly per mode
- Verification of logging clarity and helpfulness
- Evidence of state persistence in persistent mode
- Evidence of ephemeral behavior in memory mode

## Tests

Manual validation checklist:
- [ ] Memory mode: server starts <1s
- [ ] Memory mode: no persistence files created
- [ ] Memory mode: data lost on restart
- [ ] Memory mode: correct logging output
- [ ] Persistent mode: server starts <2s
- [ ] Persistent mode: .compozy/ directory created
- [ ] Persistent mode: all db files present
- [ ] Persistent mode: state persists across restarts
- [ ] Persistent mode: correct logging output
- [ ] Distributed mode: connects to external services
- [ ] Distributed mode: no embedded services started
- [ ] Distributed mode: correct logging output
- [ ] Default behavior: memory mode without config
- [ ] Error messages: clear and helpful
- [ ] Warnings: appropriate context with mode info

## Success Criteria

- Server successfully starts in all three modes
- Infrastructure components activate correctly per mode:
  - Memory: embedded SQLite :memory:, embedded Temporal :memory:, Miniredis ephemeral
  - Persistent: embedded SQLite file, embedded Temporal file, Miniredis + BadgerDB
  - Distributed: external Postgres, external Temporal, external Redis
- State persistence verified in persistent mode
- Ephemeral behavior verified in memory mode
- No regressions in distributed mode behavior
- Logging clearly indicates active mode and component configuration
- Default mode (memory) works without any configuration
- Error messages and warnings are clear and actionable
- All issues documented and resolved or tracked
