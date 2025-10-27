# Task 3.0: Configuration System Extension

**Size:** M (2 days)  
**Priority:** HIGH - Required for integration  
**Dependencies:** Task 1.0 (needs embedded.Config type)

## Overview

Extend `pkg/config` to support `TemporalConfig.Mode` and `StandaloneConfig`, add registry entries, defaults, and validation.

## Deliverables

- [ ] `pkg/config/config.go` - Add Mode and StandaloneConfig fields
- [ ] `pkg/config/definition/schema.go` - Registry entries
- [ ] `pkg/config/provider.go` - Default values
- [ ] `pkg/config/config_test.go` - Config validation tests

## Acceptance Criteria

- [ ] `TemporalConfig.Mode` field added (values: "remote", "standalone")
- [ ] `TemporalConfig.Standalone` field added (type: StandaloneConfig)
- [ ] StandaloneConfig matches embedded.Config structure
- [ ] Registry entries defined for all new fields
- [ ] Defaults applied: Mode="remote", Standalone.FrontendPort=7233, etc.
- [ ] Validation ensures valid mode values
- [ ] Validation ensures valid standalone config when mode="standalone"
- [ ] All tests pass
- [ ] No linter errors

## Implementation Approach

See `_techspec.md` "Configuration Extension" section for field structure.

**Changes to pkg/config/config.go:**
```go
type TemporalConfig struct {
    HostPort  string
    Namespace string
    TaskQueue string
    Mode      string            // NEW: "remote" or "standalone"
    Standalone StandaloneConfig // NEW: standalone settings
}

type StandaloneConfig struct {
    DatabaseFile string
    FrontendPort int
    BindIP       string
    Namespace    string
    EnableUI     bool
    UIPort       int
    LogLevel     string
}
```

**Registry Entries (definition/schema.go):**
- `temporal.mode` → Mode
- `temporal.standalone.*` → All StandaloneConfig fields

**Defaults (provider.go):**
- Mode: "remote"
- Standalone.FrontendPort: 7233
- Standalone.BindIP: "127.0.0.1"
- Standalone.EnableUI: true
- Standalone.UIPort: 8233
- Standalone.LogLevel: "warn"

## Tests (from _tests.md)

**config_test.go:**
- Should validate Mode field (only "remote" or "standalone")
- Should apply standalone defaults when Mode="standalone"
- Should validate standalone config fields
- Should allow HostPort override in standalone mode

## Files to Modify

- `pkg/config/config.go`
- `pkg/config/definition/schema.go`
- `pkg/config/provider.go`
- `pkg/config/config_test.go`

## Notes

- Keep TemporalConfig.HostPort - it gets overridden at runtime in standalone mode
- StandaloneConfig.Namespace defaults to TemporalConfig.Namespace
- Use validation tags where applicable

## Validation

```bash
gotestsum --format pkgname -- -race -parallel=4 ./pkg/config
golangci-lint run --fix --allow-parallel-runners ./pkg/config/...
```
