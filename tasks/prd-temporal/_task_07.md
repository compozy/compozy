# Task 7.0: CLI & Schema Updates

## status: completed

**Size:** S (half day)  
**Priority:** MEDIUM - CLI support  
**Dependencies:** Task 3.0

## Overview

Add CLI flags for temporal mode configuration and update JSON schema.

## Deliverables

- [x] CLI flag: `--temporal-mode`
- [x] CLI flag: `--temporal-standalone-database`
- [x] CLI flag: `--temporal-standalone-frontend-port`
- [x] CLI flag: `--temporal-standalone-ui-port`
- [x] `schemas/config.json` - Add new fields
- [x] Documentation: `cli/help/global-flags.md`

## Acceptance Criteria

- [x] `--temporal-mode` flag accepts "remote" or "standalone"
- [x] Standalone flags only relevant when mode="standalone"
- [x] Flags override YAML config correctly
- [x] JSON schema updated with new fields
- [x] Schema validation passes
- [x] Help text accurate
- [x] All tests pass

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
