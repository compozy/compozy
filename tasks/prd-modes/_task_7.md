## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 7.0: Update Server Logging

## Overview

Update server initialization logging in `engine/infra/server/server.go` to use actual mode values instead of hardcoded "standalone" strings, ensuring clear visibility into runtime configuration.

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
- Find and replace any hardcoded "standalone" strings in server logging
- Use `cfg.Mode` to dynamically log the actual mode value
- Ensure all mode-related logging uses structured fields
- Maintain consistent log format across server initialization
- No functional changes - logging updates only
</requirements>

## Subtasks

- [ ] 7.1 Search for hardcoded "standalone" strings in server.go logging
- [ ] 7.2 Replace with dynamic mode values from config
- [ ] 7.3 Verify structured logging format consistency
- [ ] 7.4 Test logging output in each mode

## Implementation Details

See **Phase 2.3: Update Server Logging** in `_techspec.md` (lines 756-783).

**Key Change Pattern:**
```go
// BEFORE:
log.Info("Starting in standalone mode", ...)

// AFTER:
log.Info("Starting server", "mode", cfg.Mode, ...)
```

**Search Strategy:**
```bash
grep -n "standalone" engine/infra/server/server.go
```

**Logging Best Practices:**
- Use structured fields (`"mode", cfg.Mode`)
- Include relevant context (database driver, temporal mode, redis mode)
- Keep messages concise and actionable
- Use consistent terminology (memory/persistent/distributed)

### Relevant Files

- `engine/infra/server/server.go` - Server initialization and logging

### Dependent Files

- `pkg/config/config.go` - Configuration structure with Mode field
- Logger package (`logger.FromContext(ctx)`)

## Deliverables

- All hardcoded "standalone" strings removed from server logging
- Mode-specific logging using dynamic config values
- Consistent structured logging format
- Clear log output showing active mode during server startup

## Tests

Manual validation tests:
- [ ] Run `compozy start --mode memory` and verify logs show "mode=memory"
- [ ] Run `compozy start --mode persistent` and verify logs show "mode=persistent"
- [ ] Run `compozy start --mode distributed` and verify logs show "mode=distributed"
- [ ] Verify no "standalone" strings appear in log output
- [ ] Verify structured log fields are consistent across modes
- [ ] Check log clarity and usefulness for debugging

## Success Criteria

- `make lint` passes with no errors
- No hardcoded "standalone" strings remain in server.go (except comments referencing old behavior)
- Server startup logs clearly indicate active mode for all three modes
- Log format is consistent and uses structured fields
- Manual testing confirms correct mode values in logs
- No functional changes to server behavior - purely logging updates
