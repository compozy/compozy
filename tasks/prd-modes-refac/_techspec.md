# Technical Specification: Mode System Terminology Refactoring

## Status: Planning

## Overview

This technical specification outlines the comprehensive refactoring plan to eliminate legacy "standalone" terminology from the Compozy codebase following the transition from `standalone/distributed` modes to `memory/persistent/distributed` modes. While the core mode system is architecturally sound and functionally correct, inconsistent terminology creates confusion and undermines code clarity.

## Problem Statement

The codebase successfully migrated from two deployment modes (standalone/distributed) to three modes (memory/persistent/distributed), but legacy "standalone" terminology persists throughout:

1. **Configuration Structures**: `Temporal.Standalone`, `Redis.Standalone` fields reference deprecated mode
2. **Documentation**: User-facing docs actively promote "standalone" as valid mode (will fail validation)
3. **Function Names**: `maybeStartStandaloneTemporal`, `validateStandaloneTemporalConfig`, etc.
4. **Comments/Logs**: Mixed terminology ("standalone", "embedded", mode names)
5. **Dead Code**: Unreachable legacy compatibility logic in cache layer
6. **Validation Gaps**: MCPProxy mode validation missing

## Objectives

1. **Eliminate Confusion**: Replace all "standalone" references with accurate terminology
2. **Align Documentation**: Ensure docs match implementation exactly
3. **Improve Maintainability**: Standardize terminology across codebase
4. **Enhance DX**: Provide clear, consistent configuration API
5. **Remove Dead Code**: Clean up unreachable legacy compatibility logic
6. **Complete Validation**: Add missing MCPProxy mode validation

## Scope

### In Scope
- Renaming configuration structs and fields
- Updating all documentation files
- Refactoring function/method names
- Standardizing comments and log messages
- Removing dead code in cache layer
- Adding MCPProxy mode validation
- Updating tests and examples

### Out of Scope
- Changing actual mode behavior (already correct)
- Modifying mode resolution logic (already correct)
- Backward compatibility (alpha project, breaking changes acceptable)
- Schema validation tag changes (keep as-is for now)

## Terminology Standards

### Approved Terms

**"Embedded"** - Preferred term for services running in-process
- Use for: struct names, function names, comments referring to memory/persistent modes collectively
- Examples: `EmbeddedConfig`, `maybeStartEmbeddedTemporal`, "embedded Redis"

**"Memory" / "Persistent"** - Specific mode names
- Use for: actual mode values, mode-specific logic, logs showing current mode
- Examples: `mode := config.ModeMemory`, `log.Info("mode", mode)`

**"Distributed"** - External services mode (unchanged)
- Use for: external service references, production deployment discussions

### Deprecated Terms

**"Standalone"** - Remove entirely (except validation error messages for backward compatibility)

---

## Implementation Plan

## Phase 1: Quick Wins (Immediate - 2 hours)

### 1.1 Remove Dead Code in Cache Layer

**File:** `engine/infra/cache/mod.go`

**Action:** Delete unreachable legacy mode mapping

**Current (Lines 63-71):**
```go
if mode == legacyModeStandalone {
	redisCfg := cacheCfg.RedisConfig
	mappedMode := modeMemory
	if redisCfg != nil && redisCfg.Standalone.Persistence.Enabled {
		mappedMode = modePersistent
	}
	log.Info("Mapping legacy standalone mode", "mapped_mode", mappedMode)
	mode = mappedMode
}
```

**Change:**
1. Delete lines 63-71
2. Remove constant `legacyModeStandalone = "standalone"` (line 16)

**Rationale:** Code is unreachable because `pkg/config/loader.go` already rejects "standalone" mode with hard error before cache setup runs.

**Files to modify:**
- `engine/infra/cache/mod.go`

---

### 1.2 Add MCPProxy Mode Validation

**File:** `pkg/config/loader.go`

**Action:** Add explicit mode validation for MCPProxy component

**Current Function (Lines 637-647):**
```go
func validateMCPProxy(cfg *Config) error {
	mode := cfg.EffectiveMCPProxyMode()
	if isEmbeddedMode(mode) && cfg.MCPProxy.Port == 0 {
		return fmt.Errorf(
			"mcp_proxy.port must be non-zero when mode is %q or %q",
			ModeMemory,
			ModePersistent,
		)
	}
	return nil
}
```

**New Function (Insert before line 637):**
```go
func validateMCPProxyMode(cfg *Config) error {
	switch mode := strings.TrimSpace(cfg.MCPProxy.Mode); mode {
	case "", ModeMemory, ModePersistent, ModeDistributed:
		return nil
	case deprecatedModeStandalone:
		return fmt.Errorf(
			"mcp_proxy.mode %q is no longer supported; use %q (in-memory) or %q (persistent) for embedded MCP proxy",
			deprecatedModeStandalone,
			ModeMemory,
			ModePersistent,
		)
	default:
		return fmt.Errorf(
			"mcp_proxy.mode must be one of [%s %s %s] or empty for inheritance, got %q",
			ModeMemory,
			ModePersistent,
			ModeDistributed,
			mode,
		)
	}
}
```

**Update validateMCPProxy (Lines 637-647):**
```go
func validateMCPProxy(cfg *Config) error {
	// Validate mode value first
	if err := validateMCPProxyMode(cfg); err != nil {
		return err
	}
	// Then validate port requirements
	mode := cfg.EffectiveMCPProxyMode()
	if isEmbeddedMode(mode) && cfg.MCPProxy.Port == 0 {
		return fmt.Errorf(
			"mcp_proxy.port must be non-zero when mode is %q or %q",
			ModeMemory,
			ModePersistent,
		)
	}
	return nil
}
```

**Files to modify:**
- `pkg/config/loader.go`

**Tests to add:**
- `pkg/config/loader_test.go` - Add test case for MCPProxy mode validation

---

### 1.3 Fix Critical Documentation

**Files to update:**

1. **`docs/content/docs/configuration/redis.mdx`**

**Current (Lines 7-16):**
```markdown
Compozy's cache layer supports two modes:

- `distributed`: connect to an external Redis instance (production/staging)
- `standalone`: run an embedded, Redis‑compatible server with optional snapshots (development/CI)

## Configuration Structure

```yaml title="compozy.yaml"
redis:
  mode: distributed | standalone
```

**Replace with:**
```markdown
Compozy's cache layer supports three modes:

- `memory`: run embedded, in-memory Redis (fastest, no persistence)
- `persistent`: run embedded Redis with snapshot persistence (local development)
- `distributed`: connect to an external Redis instance (production/staging)

## Configuration Structure

```yaml title="compozy.yaml"
redis:
  mode: memory | persistent | distributed
```

**Additional changes in same file:**
- Line 34: Change default from `distributed` to `memory`
- Lines 37-49: Update "Distributed Mode" section (keep as-is, just verify)
- Lines 60-82: Replace "Standalone Mode" section with "Embedded Modes"

**New "Embedded Modes" section:**
```markdown
## Embedded Modes (Memory & Persistent)

Runs a Redis‑compatible server inside the Compozy process.

### Memory Mode

Fastest option with no persistence:

```yaml
redis:
  mode: memory
```

### Persistent Mode

Embedded Redis with periodic snapshots:

```yaml
redis:
  mode: persistent
  standalone:  # Note: field name is "standalone" but mode is "persistent"
    persistence:
      enabled: true
      dir: ./.compozy/redis
      interval: 30s
```

### Persistence Options

- `enabled`: turn on periodic snapshots
- `dir`: where snapshots are stored; ensure the process can write here
- `interval`: snapshot frequency (`time.Duration` syntax)

<Callout type="warning">
Embedded modes (memory, persistent) are single‑process and not HA. Use them only for dev/CI.
</Callout>
```

2. **`docs/content/docs/architecture/embedded-temporal.mdx`**

**Current (Lines 1-9):**
```markdown
---
title: "Embedded Temporal"
description: "Deep dive into the embedded Temporal server that powers standalone mode."
icon: Layers
---

<Callout type="info">
Standalone mode embeds the official Temporal server inside the Compozy process...
</Callout>
```

**Replace with:**
```markdown
---
title: "Embedded Temporal"
description: "Deep dive into the embedded Temporal server that powers memory and persistent modes."
icon: Layers
---

<Callout type="info">
Memory and persistent modes embed the official Temporal server inside the Compozy process. They spin up the same four microservices you deploy in production—just scoped to the developer machine.
</Callout>
```

**Additional changes:**
- Search and replace all instances of "standalone mode" with "embedded mode" or "memory/persistent modes"
- Update YAML examples showing `mode: standalone` to `mode: memory` or `mode: persistent`

3. **`docs/content/docs/deployment/temporal-modes.mdx`** (if exists)

**Search for:** All references to "standalone"
**Replace with:** Appropriate references to "memory", "persistent", or "embedded"

4. **`docs/content/docs/cli/compozy-start.mdx`** (if exists)

**Update mode examples** to show `memory`, `persistent`, `distributed`

**Files to modify:**
- `docs/content/docs/configuration/redis.mdx`
- `docs/content/docs/architecture/embedded-temporal.mdx`
- `docs/content/docs/deployment/temporal-modes.mdx` (if exists)
- `docs/content/docs/cli/compozy-start.mdx` (if exists)

---

## Phase 2: Configuration Structure Refactoring (1 day)

### 2.1 Rename Configuration Structs

**Objective:** Replace `StandaloneConfig` with `EmbeddedConfig` throughout codebase

#### 2.1.1 Core Config Types

**File:** `pkg/config/config.go`

**Changes:**

1. **Rename `StandaloneConfig` → `EmbeddedTemporalConfig` (Lines 588-624)**

**Current:**
```go
// StandaloneConfig configures the embedded Temporal server that powers memory and persistent modes.
type StandaloneConfig struct {
	DatabaseFile string `koanf:"database_file" env:"TEMPORAL_STANDALONE_DATABASE_FILE" ...`
	// ... other fields
}
```

**New:**
```go
// EmbeddedTemporalConfig configures the embedded Temporal server that powers memory and persistent modes.
//
// This configuration only applies when temporal.mode is "memory" or "persistent".
// In distributed mode, these settings are ignored.
type EmbeddedTemporalConfig struct {
	DatabaseFile string `koanf:"database_file" env:"TEMPORAL_EMBEDDED_DATABASE_FILE" ...`
	FrontendPort int    `koanf:"frontend_port" env:"TEMPORAL_EMBEDDED_FRONTEND_PORT" ...`
	BindIP       string `koanf:"bind_ip"       env:"TEMPORAL_EMBEDDED_BIND_IP"       ...`
	Namespace    string `koanf:"namespace"     env:"TEMPORAL_EMBEDDED_NAMESPACE"     ...`
	ClusterName  string `koanf:"cluster_name"  env:"TEMPORAL_EMBEDDED_CLUSTER_NAME"  ...`
	EnableUI     bool   `koanf:"enable_ui"     env:"TEMPORAL_EMBEDDED_ENABLE_UI"     ...`
	RequireUI    bool   `koanf:"require_ui"    env:"TEMPORAL_EMBEDDED_REQUIRE_UI"    ...`
	UIPort       int    `koanf:"ui_port"       env:"TEMPORAL_EMBEDDED_UI_PORT"       ...`
	LogLevel     string `koanf:"log_level"     env:"TEMPORAL_EMBEDDED_LOG_LEVEL"     ...`
	StartTimeout time.Duration `koanf:"start_timeout" env:"TEMPORAL_EMBEDDED_START_TIMEOUT" ...`
}
```

**Note:** Keep koanf tag as "standalone" for backward compatibility with existing YAML files. Only change the Go type name and environment variable prefix.

2. **Update TemporalConfig field (Line 585)**

**Current:**
```go
Standalone StandaloneConfig `koanf:"standalone" env_prefix:"TEMPORAL_STANDALONE" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
```

**New:**
```go
// Standalone configures the embedded Temporal server for memory and persistent modes.
// YAML path remains "temporal.standalone" for backward compatibility.
Standalone EmbeddedTemporalConfig `koanf:"standalone" env_prefix:"TEMPORAL_EMBEDDED" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
```

3. **Rename `RedisStandaloneConfig` → `EmbeddedRedisConfig` (Lines 1436-1449)**

**Current:**
```go
type RedisStandaloneConfig struct {
	Persistence RedisPersistenceConfig `koanf:"persistence" json:"persistence" yaml:"persistence" mapstructure:"persistence"`
}
```

**New:**
```go
// EmbeddedRedisConfig defines options for the embedded Redis used by memory and persistent modes.
//
// This configuration only applies when redis.mode is "memory" or "persistent".
// In distributed mode, these settings are ignored.
type EmbeddedRedisConfig struct {
	Persistence RedisPersistenceConfig `koanf:"persistence" json:"persistence" yaml:"persistence" mapstructure:"persistence"`
}
```

4. **Update RedisConfig field (Line 1433)**

**Current:**
```go
Standalone RedisStandaloneConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
```

**New:**
```go
// Standalone configures the embedded Redis for memory and persistent modes.
// YAML path remains "redis.standalone" for backward compatibility.
Standalone EmbeddedRedisConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
```

**Files to modify:**
- `pkg/config/config.go`

---

#### 2.1.2 Update Builder Functions

**File:** `pkg/config/config.go`

**Functions to update:**

1. **`buildTemporalConfig` (around line 2396)**

Update type references:
```go
Standalone: EmbeddedTemporalConfig{
	DatabaseFile: getString(registry, "temporal.standalone.database_file"),
	// ...
}
```

2. **`buildRedisConfig` (around line 2728)**

Update type references:
```go
Standalone: EmbeddedRedisConfig{
	Persistence: RedisPersistenceConfig{
		// ...
	},
}
```

**Files to modify:**
- `pkg/config/config.go`

---

#### 2.1.3 Update Validation Functions

**File:** `pkg/config/loader.go`

**Functions to rename:**

1. **`validateStandaloneTemporalConfig` → `validateEmbeddedTemporalConfig` (Line 456)**

```go
func validateEmbeddedTemporalConfig(cfg *Config) error {
	embedded := &cfg.Temporal.Standalone
	if err := validateEmbeddedTemporalDatabase(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalPorts(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalNetwork(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalMetadata(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalLogLevel(embedded); err != nil {
		return err
	}
	return validateEmbeddedTemporalStartTimeout(embedded)
}
```

2. **Rename all `validateStandalone*` helper functions:**

- `validateStandaloneDatabase` → `validateEmbeddedTemporalDatabase` (Line 476)
- `validateStandalonePorts` → `validateEmbeddedTemporalPorts` (Line 483)
- `validateStandaloneNetwork` → `validateEmbeddedTemporalNetwork` (Line 505)
- `validateStandaloneMetadata` → `validateEmbeddedTemporalMetadata` (Line 517)
- `validateStandaloneLogLevel` → `validateEmbeddedTemporalLogLevel` (Line 530)
- `validateStandaloneStartTimeout` → `validateEmbeddedTemporalStartTimeout` (Line 538)

3. **Update function signatures to use `EmbeddedTemporalConfig`:**

```go
func validateEmbeddedTemporalDatabase(embedded *EmbeddedTemporalConfig) error {
	if embedded.DatabaseFile == "" {
		return fmt.Errorf("temporal.standalone.database_file is required when using embedded Temporal")
	}
	return nil
}
```

**Update call site in `validateTemporal` (Line 437):**
```go
case ModeMemory, ModePersistent:
	return validateEmbeddedTemporalConfig(cfg)
```

**Files to modify:**
- `pkg/config/loader.go`

---

### 2.2 Update Embedded Temporal Package

**File:** `engine/worker/embedded/config.go`

**Changes:**

1. **Rename type alias or update imports**

If there's a type alias, update it:
```go
// Config wraps the embedded Temporal server configuration for memory and persistent modes.
type Config = pkg/config.EmbeddedTemporalConfig
```

Or if it's a separate struct, ensure it references the correct type.

2. **Update comments** to replace "standalone" with "embedded"

**Files to modify:**
- `engine/worker/embedded/config.go`
- `engine/worker/embedded/server.go` (update comments)
- `engine/worker/embedded/builder.go` (update comments)

---

### 2.3 Update Server Dependency Functions

**File:** `engine/infra/server/dependencies.go`

**Functions to rename:**

1. **`maybeStartStandaloneTemporal` → `maybeStartEmbeddedTemporal` (Line 384)**

```go
func maybeStartEmbeddedTemporal(ctx context.Context) (func(), error) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required to start Temporal")
	}
	mode := cfg.EffectiveTemporalMode()
	if mode != config.ModeMemory && mode != config.ModePersistent {
		return nil, nil
	}
	embeddedCfg := embeddedTemporalConfig(cfg)
	log := logger.FromContext(ctx)
	log.Info(
		"Starting embedded Temporal",
		"mode", mode,
		"database", embeddedCfg.DatabaseFile,
		// ...
	)
	// ... rest of function
}
```

2. **`standaloneEmbeddedConfig` → `embeddedTemporalConfig` (Line 428)**

```go
func embeddedTemporalConfig(cfg *config.Config) *embedded.Config {
	embedded := cfg.Temporal.Standalone
	mode := cfg.EffectiveTemporalMode()
	dbFile := strings.TrimSpace(embedded.DatabaseFile)
	switch mode {
	case config.ModePersistent:
		if dbFile == "" || dbFile == ":memory:" {
			dbFile = "./.compozy/temporal.db"
		}
	case config.ModeMemory:
		if dbFile == "" {
			dbFile = ":memory:"
		}
	// ... rest
	}
	return &embedded.Config{
		DatabaseFile: dbFile,
		FrontendPort: embedded.FrontendPort,
		BindIP:       embedded.BindIP,
		Namespace:    embedded.Namespace,
		ClusterName:  embedded.ClusterName,
		EnableUI:     embedded.EnableUI,
		RequireUI:    embedded.RequireUI,
		UIPort:       embedded.UIPort,
		LogLevel:     embedded.LogLevel,
		StartTimeout: embedded.StartTimeout,
	}
}
```

3. **`standaloneTemporalCleanup` → `embeddedTemporalCleanup` (Line 460)**

```go
func embeddedTemporalCleanup(
	ctx context.Context,
	server *embedded.Server,
	shutdownTimeout time.Duration,
) func() {
	return func() {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := server.Stop(stopCtx); err != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded Temporal server", "error", err)
		}
	}
}
```

4. **Update call sites:**

**In `Server.setupDependencies` (Line 238):**
```go
temporalCleanup, err := maybeStartEmbeddedTemporal(s.ctx)
```

**In `maybeStartEmbeddedTemporal` (Line 425):**
```go
return embeddedTemporalCleanup(ctx, server, shutdownTimeout), nil
```

**Files to modify:**
- `engine/infra/server/dependencies.go`

---

### 2.4 Update Cache Setup Functions

**File:** `engine/infra/cache/mod.go`

**Changes:**

1. **Update function names (Lines 89-126):**

```go
func setupMemoryCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	redisCfg := cacheCfg.RedisConfig
	if redisCfg == nil {
		return nil, nil, fmt.Errorf("missing redis configuration for memory mode")
	}
	persistence := &redisCfg.Standalone.Persistence
	previouslyEnabled := persistence.Enabled
	persistence.Enabled = false
	log := logger.FromContext(ctx)
	log.Info("Cache in memory mode",
		"persistence_enabled", persistence.Enabled,
		"previously_enabled", previouslyEnabled,
	)
	return setupEmbeddedCache(ctx, cacheCfg, modeMemory)
}

func setupPersistentCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	// ... same logic, call setupEmbeddedCache
}

// setupEmbeddedCache creates embedded miniredis backend and wraps it with Redis facade.
func setupEmbeddedCache(ctx context.Context, cacheCfg *Config, mode string) (*Cache, func(), error) {
	// ... existing logic
}
```

2. **Rename `setupStandaloneCache` → `setupEmbeddedCache` (Line 129)**

3. **Update comments:**

**Lines 42-44:**
```go
// embedded holds the in-process miniredis server when running in
// memory or persistent mode. It remains nil when using an external (distributed) Redis backend.
embedded *MiniredisStandalone
```

**Files to modify:**
- `engine/infra/cache/mod.go`

---

### 2.5 Update MiniredisStandalone References

**File:** `engine/infra/cache/miniredis_standalone.go`

**Consider renaming file:** `miniredis_standalone.go` → `miniredis_embedded.go`

**Type name:** `MiniredisStandalone` → `MiniredisEmbedded`

**Changes:**

```go
// MiniredisEmbedded wraps a miniredis server for embedded (memory/persistent) modes.
type MiniredisEmbedded struct {
	server *miniredis.Miniredis
	client *redis.Client
	config *Config
}

// NewMiniredisEmbedded creates and starts an embedded miniredis server.
func NewMiniredisEmbedded(ctx context.Context) (*MiniredisEmbedded, error) {
	// ...
}
```

**Update all references:**
- `engine/infra/cache/mod.go` - Update `embedded *MiniredisStandalone` field
- `engine/infra/cache/miniredis_standalone.go` - Rename type and constructor

**Files to modify:**
- `engine/infra/cache/miniredis_standalone.go` (consider renaming file)
- `engine/infra/cache/mod.go`

---

## Phase 3: Function Names and Comments (1 day)

### 3.1 Update Test Helper Functions

**File:** `test/integration/temporal/standalone_test.go`

**Consider renaming file:** `standalone_test.go` → `embedded_test.go`

**Functions to update:**

1. **`startStandaloneServer` → `startEmbeddedServer`**
2. **`TestStandaloneMemoryMode` → `TestEmbeddedMemoryMode`**
3. **`TestStandaloneFileMode` → `TestEmbeddedFileMode`**
4. **`TestStandaloneCustomPorts` → `TestEmbeddedCustomPorts`**
5. **`TestStandaloneWorkflowExecution` → `TestEmbeddedWorkflowExecution`**

**Files to modify:**
- `test/integration/temporal/standalone_test.go` (rename to `embedded_test.go`)
- `test/integration/temporal/startup_lifecycle_test.go`
- `test/integration/temporal/persistence_test.go`
- `test/integration/temporal/errors_test.go`

---

### 3.2 Update Standalone Test Package

**Directory:** `test/integration/standalone/`

**Consider renaming directory:** `standalone/` → `embedded/` or `memory-mode/`

**Files in directory:**
- `workflow_test.go`
- `streaming_test.go`
- `resource_store_test.go`
- `persistence_test.go`
- `helpers.go`

**Package declaration:** Update from `package standalone` to `package embedded`

**Function names to update:**
- `SetupStandaloneStreaming` → `SetupEmbeddedStreaming`
- `SetupStandaloneResourceStore` → `SetupEmbeddedResourceStore`
- `SetupStandaloneWithPersistence` → `SetupEmbeddedWithPersistence`

**Files to modify:**
- All files in `test/integration/standalone/` directory
- Consider renaming directory to `test/integration/embedded/`

---

### 3.3 Update Test Helpers

**File:** `test/helpers/server/server.go`

**Update comments and function documentation** that reference "standalone"

**Files to modify:**
- `test/helpers/server/server.go`
- Other helper files referencing "standalone"

---

### 3.4 Standardize Comments and Log Messages

**Search Pattern:** `grep -r "standalone" --include="*.go" --exclude-dir=vendor`

**For each occurrence:**

1. **In comments:** Replace with "embedded" or specific mode names
2. **In log messages:** Use actual mode value, not "standalone"
3. **In function docs:** Use "embedded", "memory mode", or "persistent mode"

**Example transformations:**

**Before:**
```go
// Start standalone Temporal server
log.Info("Starting standalone server")
```

**After:**
```go
// Start embedded Temporal server for memory/persistent modes
log.Info("Starting embedded Temporal", "mode", mode)
```

**Files to search and update:**
- All `.go` files in `engine/`
- All `.go` files in `pkg/`
- All `.go` files in `cli/`
- All `.go` files in `test/`

---

### 3.5 Update CLI Help Text

**File:** `cli/cmd/start/start.go`

**Search for:** Mode descriptions and examples

**Update:** References to "standalone" in help text

**Files to modify:**
- `cli/cmd/start/start.go`
- `cli/helpers/mode.go` (if exists)
- Any other CLI command files mentioning modes

---

## Phase 4: Test Updates (0.5 days)

### 4.1 Update Test Fixtures

**Directory:** `test/fixtures/standalone/`

**Consider renaming:** `standalone/` → `embedded/`

**Files to check:**
- YAML workflow fixtures
- Any configuration files

**Files to modify:**
- Update paths in test code that reference `test/fixtures/standalone/`

---

### 4.2 Add New Test Cases

**File:** `pkg/config/loader_test.go`

**Add test for MCPProxy mode validation:**

```go
t.Run("MCPProxy mode validation", func(t *testing.T) {
	cases := []struct {
		name           string
		mode           string
		wantErr        bool
		wantSubstrings []string
	}{
		{name: "empty inherits", mode: ""},
		{name: "memory valid", mode: ModeMemory},
		{name: "persistent valid", mode: ModePersistent},
		{name: "distributed valid", mode: ModeDistributed},
		{
			name:           "standalone invalid",
			mode:           "standalone",
			wantErr:        true,
			wantSubstrings: []string{"standalone", "no longer supported", ModeMemory, ModePersistent},
		},
		{name: "invalid value", mode: "invalid", wantErr: true, wantSubstrings: []string{"must be one of"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService()
			cfg := Default()
			cfg.MCPProxy.Mode = tc.mode
			err := svc.Validate(cfg)
			if tc.wantErr {
				require.Error(t, err)
				for _, sub := range tc.wantSubstrings {
					assert.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
		})
	}
})
```

**Files to modify:**
- `pkg/config/loader_test.go`

---

### 4.3 Update Existing Tests

**Files to review and update:**

1. **`pkg/config/config_test.go`**
   - Update test names referencing "standalone"
   - Update test assertions

2. **`pkg/config/resolver_test.go`**
   - Verify mode resolution tests still pass
   - Update comments

3. **`engine/infra/cache/mod_test.go`**
   - Update test function names
   - Update assertions

**Files to modify:**
- `pkg/config/config_test.go`
- `pkg/config/resolver_test.go`
- `engine/infra/cache/mod_test.go`

---

## Phase 5: Schema and Documentation (0.5 days)

### 5.1 Update JSON Schemas

**Files to check:**

1. **`schemas/config-temporal.json`**
   - Update descriptions
   - Keep property names as-is (backward compatibility)

2. **`schemas/config.json`**
   - Update mode enum descriptions
   - Update embedded config descriptions

**Files to modify:**
- `schemas/config-temporal.json`
- `schemas/config.json`

---

### 5.2 Update Remaining Documentation

**Files to audit:**

1. **`README.md`**
2. **`docs/content/docs/configuration/mode-configuration.mdx`**
3. **`docs/content/docs/deployment/*.mdx`**
4. **`CONTRIBUTING.md`** (if exists)
5. **`examples/*/README.md`**

**Search for:** All references to "standalone" mode

**Replace with:** Appropriate references to memory/persistent/embedded

**Files to modify:**
- All documentation files containing "standalone" mode references

---

### 5.3 Update Example Configurations

**Directory:** `examples/`

**Search for:** YAML files with `mode: standalone`

**Update to:** `mode: memory` or `mode: persistent`

**Files to modify:**
- All example YAML configuration files

---

## Testing & Validation

### Pre-Refactoring Checklist

- [ ] Run full test suite: `make test`
- [ ] Run linter: `make lint`
- [ ] Document current behavior
- [ ] Create feature branch: `git checkout -b refactor/mode-terminology`

### During Refactoring

After each phase:

- [ ] Run affected tests: `go test ./path/to/modified/...`
- [ ] Run linter on modified files: `golangci-lint run ./path/...`
- [ ] Verify no compilation errors: `go build ./...`

### Post-Refactoring Validation

1. **Unit Tests**
   ```bash
   make test
   ```
   Expected: All tests pass

2. **Linting**
   ```bash
   make lint
   ```
   Expected: No linting errors

3. **Integration Tests**
   ```bash
   make test-all
   ```
   Expected: All integration tests pass

4. **Build Verification**
   ```bash
   make build
   ```
   Expected: Clean build with no errors

5. **Documentation Build**
   ```bash
   cd docs && npm run build
   ```
   Expected: Documentation builds without errors

6. **Manual Testing Scenarios**

   **Test 1: Memory Mode**
   ```yaml
   # compozy.yaml
   mode: memory
   ```
   ```bash
   compozy start
   ```
   Expected: Starts successfully with embedded services

   **Test 2: Persistent Mode**
   ```yaml
   # compozy.yaml
   mode: persistent
   redis:
     standalone:
       persistence:
         enabled: true
         dir: ./.compozy/redis
   ```
   ```bash
   compozy start
   ```
   Expected: Starts with persistence, creates snapshot directory

   **Test 3: Component Override**
   ```yaml
   mode: distributed
   temporal:
     mode: memory  # Override just Temporal
   ```
   Expected: External Redis/Postgres, embedded Temporal

   **Test 4: Validation**
   ```yaml
   mode: standalone  # Invalid
   ```
   ```bash
   compozy start
   ```
   Expected: Clear error message suggesting memory/persistent

   **Test 5: Config Validation**
   ```bash
   compozy config show
   ```
   Expected: Shows configuration without errors

   **Test 6: MCPProxy Mode Validation**
   ```yaml
   mcp_proxy:
     mode: standalone  # Should fail
   ```
   Expected: Validation error with helpful message

7. **Grep Verification**
   ```bash
   # Should find NO occurrences (except in validation error messages)
   grep -r "standalone" pkg/ engine/ --include="*.go" | grep -v "deprecatedModeStandalone" | grep -v "test" | grep -v "comment"
   
   # Documentation should not reference standalone as valid mode
   grep -r "mode: standalone" docs/ examples/
   ```

---

## Migration Guide for Users

### Breaking Changes

**Configuration struct field names changed in Go API:**
- `config.StandaloneConfig` → `config.EmbeddedTemporalConfig`
- `config.RedisStandaloneConfig` → `config.EmbeddedRedisConfig`

**Environment variable prefixes changed:**
- `TEMPORAL_STANDALONE_*` → `TEMPORAL_EMBEDDED_*`

**YAML structure unchanged** (backward compatible):
```yaml
temporal:
  standalone:  # YAML key remains the same
    database_file: ":memory:"
```

### No User Action Required

- Existing YAML configurations continue to work
- Mode validation unchanged (already rejected "standalone")
- Only internal naming and documentation changed

---

## Rollout Plan

### Phase-by-Phase Approach

**Week 1: Quick Wins**
- Phase 1.1: Remove dead code (merge immediately)
- Phase 1.2: Add MCPProxy validation (merge immediately)
- Phase 1.3: Fix critical documentation (merge immediately)

**Week 2: Core Refactoring**
- Phase 2: Configuration structures
- Phase 3: Function names and comments
- Merge as single PR with comprehensive tests

**Week 3: Cleanup**
- Phase 4: Test updates
- Phase 5: Documentation and schemas
- Final validation and merge

### Git Strategy

**Commits:**
1. `refactor(config): remove dead code from cache layer`
2. `feat(config): add MCPProxy mode validation`
3. `docs: update Redis and Temporal documentation for new modes`
4. `refactor(config): rename StandaloneConfig to EmbeddedConfig`
5. `refactor(server): rename standalone functions to embedded`
6. `refactor(cache): rename MiniredisStandalone to MiniredisEmbedded`
7. `refactor(tests): update test names and fixtures`
8. `docs: comprehensive mode terminology update`

**PR Title:** `refactor: eliminate legacy "standalone" terminology from mode system`

**PR Description Template:**
```markdown
## Overview
Comprehensive refactoring to eliminate legacy "standalone" terminology following the mode system migration (standalone/distributed → memory/persistent/distributed).

## Changes
- Renamed configuration structs: `StandaloneConfig` → `EmbeddedConfig`
- Renamed functions: `maybeStartStandaloneTemporal` → `maybeStartEmbeddedTemporal`
- Updated all documentation to reference memory/persistent modes
- Added MCPProxy mode validation
- Removed unreachable legacy compatibility code
- Standardized comments and log messages

## Breaking Changes
⚠️ Go API changes (type names only):
- `config.StandaloneConfig` → `config.EmbeddedTemporalConfig`
- `config.RedisStandaloneConfig` → `config.EmbeddedRedisConfig`
- Environment variable prefix: `TEMPORAL_STANDALONE_*` → `TEMPORAL_EMBEDDED_*`

✅ YAML configuration unchanged (backward compatible)

## Testing
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Manual testing of memory/persistent/distributed modes
- [ ] Documentation builds successfully
- [ ] Validation errors tested

## Checklist
- [ ] Code follows project standards
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Linter passes
- [ ] No grep results for inappropriate "standalone" usage
```

---

## Success Criteria

### Code Quality
- [ ] Zero references to "standalone" in comments (except deprecation errors)
- [ ] Zero references to "standalone" in function names
- [ ] Zero references to "standalone" in log messages (use actual mode)
- [ ] All config structs use "Embedded" terminology
- [ ] Consistent terminology across codebase

### Testing
- [ ] 100% test pass rate
- [ ] New tests for MCPProxy validation
- [ ] Updated test names reflect new terminology
- [ ] Integration tests verify embedded modes work

### Documentation
- [ ] User docs reference memory/persistent/distributed only
- [ ] API docs updated
- [ ] Examples use correct modes
- [ ] Migration guide provided

### Validation
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make build` succeeds
- [ ] Manual smoke tests pass
- [ ] No grep hits for invalid "standalone" usage

---

## Risk Assessment

### Low Risk
- Dead code removal (unreachable)
- Documentation updates (no functional change)
- Comment updates (no functional change)

### Medium Risk
- Function renaming (extensive but mechanical)
- Test updates (comprehensive coverage exists)

### High Risk (Requires Careful Review)
- Configuration struct renaming (affects public API)
- Environment variable prefix changes (user-facing)

### Mitigation Strategies

1. **Comprehensive Testing**
   - Run full test suite after each phase
   - Manual testing of all three modes
   - Integration test coverage

2. **Backward Compatibility**
   - Keep YAML keys unchanged
   - Validation already enforces new modes
   - Type aliases if needed for transition

3. **Clear Communication**
   - Document breaking changes
   - Provide migration guide
   - Update changelog

4. **Rollback Plan**
   - Each phase is independently revertable
   - Feature branch for safety
   - Comprehensive commit messages

---

## Timeline Estimate

| Phase | Effort | Duration |
|-------|--------|----------|
| Phase 1: Quick Wins | 2 hours | Day 1 AM |
| Phase 2: Config Refactoring | 1 day | Day 1 PM - Day 2 |
| Phase 3: Functions & Comments | 1 day | Day 3 |
| Phase 4: Tests | 0.5 days | Day 4 AM |
| Phase 5: Documentation | 0.5 days | Day 4 PM |
| Testing & Validation | 0.5 days | Day 5 AM |
| **Total** | **3.5 days** | **5 days with buffer** |

---

## Notes

- **Backward Compatibility:** YAML configuration keys remain unchanged (`temporal.standalone`, `redis.standalone`) to avoid breaking existing configurations
- **Environment Variables:** Prefix changes from `TEMPORAL_STANDALONE_*` to `TEMPORAL_EMBEDDED_*` is a breaking change but acceptable for alpha project
- **Dead Code:** The cache layer's legacy mapping (lines 63-71) is provably unreachable due to loader validation
- **Validation Gap:** MCPProxy lacks mode validation unlike Redis/Temporal - this inconsistency should be fixed
- **Test Coverage:** Existing test coverage is good; updates are mostly mechanical renames

---

## References

**Related Files:**
- Mode system PRD: `tasks/prd-modes/*.md`
- Original mode migration: `tasks/prd-modes/_techspec.md`

**Key Commits:**
- Mode system implementation (reference commit hash if available)

**Documentation:**
- Mode configuration guide: `docs/content/docs/configuration/mode-configuration.mdx`
- Architecture guide: `docs/content/docs/architecture/embedded-temporal.mdx`

