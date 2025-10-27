# Task 7.0: CLI & Schema Updates

**Size:** S (half day)  
**Priority:** MEDIUM - CLI support  
**Dependencies:** Task 3.0

## Overview

Add CLI flags for temporal mode configuration and update JSON schema.

## Deliverables

- [ ] CLI flag: `--temporal-mode`
- [ ] CLI flag: `--temporal-standalone-database`
- [ ] CLI flag: `--temporal-standalone-frontend-port`
- [ ] CLI flag: `--temporal-standalone-ui-port`
- [ ] `schemas/config.json` - Add new fields
- [ ] Documentation: `cli/help/global-flags.md`

## Acceptance Criteria

- [ ] `--temporal-mode` flag accepts "remote" or "standalone"
- [ ] Standalone flags only relevant when mode="standalone"
- [ ] Flags override YAML config correctly
- [ ] JSON schema updated with new fields
- [ ] Schema validation passes
- [ ] Help text accurate
- [ ] All tests pass

## Implementation Approach

See `_techspec.md` "CLI Extension" and `_docs.md` "CLI Documentation" sections.

**Add to root.go or global flags:**
```go
--temporal-mode string
    Temporal server mode (remote or standalone)
    
--temporal-standalone-database string
    SQLite database file path (use :memory: for ephemeral)
    
--temporal-standalone-frontend-port int
    Frontend service port (default: 7233)
    
--temporal-standalone-ui-port int
    Web UI port (default: 8233)
```

**Schema Updates:**
Add to `schemas/config.json` under `temporal` object:
- `mode` (string, enum: ["remote", "standalone"])
- `standalone` (object with all StandaloneConfig fields)

## Files to Modify

- `cli/root.go` - Add global flags
- `cli/helpers/global.go` - Wire flags to config
- `schemas/config.json` - Add schema definitions
- `cli/help/global-flags.md` - Document new flags

## Tests

- Verify flag precedence: CLI > Env > YAML > Default
- Test: `compozy start --temporal-mode=standalone`
- Test: `compozy start --temporal-mode=standalone --temporal-standalone-database=:memory:`

## Validation

```bash
# Test CLI flags
compozy start --help | grep temporal

# Verify schema
make validate-schemas

# Integration test
compozy start --temporal-mode=standalone --temporal-standalone-database=:memory:
```
