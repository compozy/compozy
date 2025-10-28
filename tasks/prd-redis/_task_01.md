## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>pkg/config, engine/infra/cache</domain>
<type>implementation</type>
<scope>core_feature, configuration</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 1.0: Global Mode Configuration & Resolver

## Overview

Implement the global mode configuration system with component inheritance pattern. This establishes the foundation for all standalone mode functionality by adding a top-level `mode` field and per-component mode overrides with a resolver pattern.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `config.FromContext(ctx)` - never store config
- **MUST** use `logger.FromContext(ctx)` - never pass logger as parameter
- **NEVER** use `context.Background()` in tests, use `t.Context()` instead
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Add global `mode` field to Config struct ("standalone" | "distributed")
- Add `RedisConfig.Mode` field with same options (empty string = inherit)
- Create `pkg/config/resolver.go` with mode resolution logic
- Implement `EffectiveRedisMode()` helper method
- Implement `EffectiveTemporalMode()` helper method (normalizes "distributed" → "remote")
- Implement `EffectiveMCPProxyMode()` helper method
- Add validation for mode field values
- Default mode must be "distributed" for backward compatibility
- Support component-level mode overrides
</requirements>

## Subtasks

- [x] 1.1 Add global `mode` field to Config struct in `pkg/config/config.go`
- [x] 1.2 Add `RedisConfig` struct with mode, addr, password, and standalone sections
- [x] 1.3 Add `RedisStandaloneConfig` and `RedisPersistenceConfig` structs
- [x] 1.4 Create `pkg/config/resolver.go` with `ResolveMode()` function
- [x] 1.5 Implement `EffectiveRedisMode()` method on Config
- [x] 1.6 Implement `EffectiveTemporalMode()` method on Config
- [x] 1.7 Implement `EffectiveMCPProxyMode()` method on Config
- [x] 1.8 Add validation rules in `pkg/config/loader.go` for mode fields
- [x] 1.9 Update config tests to verify mode resolution logic

## Implementation Details

### Configuration Schema

Add to `pkg/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // Mode controls global deployment model
    // "distributed" (default): External services required
    // "standalone": Embedded services, single-process
    Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=standalone distributed"`

    // Redis cache configuration
    Redis RedisConfig `koanf:"redis" json:"redis" yaml:"redis" mapstructure:"redis"`
}

type RedisConfig struct {
    // Mode controls Redis deployment model
    // "" (empty): Inherit from global Config.Mode
    // "distributed": Use external Redis (explicit override)
    // "standalone": Use embedded miniredis (explicit override)
    Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=standalone distributed"`

    Addr string `koanf:"addr" json:"addr" yaml:"addr" mapstructure:"addr"`
    Password config.SensitiveString `koanf:"password" json:"password" yaml:"password" mapstructure:"password" sensitive:"true"`

    Standalone RedisStandaloneConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
}

type RedisStandaloneConfig struct {
    Persistence RedisPersistenceConfig `koanf:"persistence" json:"persistence" yaml:"persistence" mapstructure:"persistence"`
}

type RedisPersistenceConfig struct {
    Enabled bool `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled"`
    DataDir string `koanf:"data_dir" json:"data_dir" yaml:"data_dir" mapstructure:"data_dir"`
    SnapshotInterval time.Duration `koanf:"snapshot_interval" json:"snapshot_interval" yaml:"snapshot_interval" mapstructure:"snapshot_interval"`
    SnapshotOnShutdown bool `koanf:"snapshot_on_shutdown" json:"snapshot_on_shutdown" yaml:"snapshot_on_shutdown" mapstructure:"snapshot_on_shutdown"`
    RestoreOnStartup bool `koanf:"restore_on_startup" json:"restore_on_startup" yaml:"restore_on_startup" mapstructure:"restore_on_startup"`
}
```

### Mode Resolver

Create `pkg/config/resolver.go`:

```go
package config

// ResolveMode determines the effective deployment mode for a component.
//
// Resolution priority:
//  1. Component mode (if explicitly set)
//  2. Global mode (if set in Config.Mode)
//  3. Default fallback ("distributed")
func ResolveMode(cfg *Config, componentMode string) string {
    if componentMode != "" {
        return componentMode
    }
    if cfg.Mode != "" {
        return cfg.Mode
    }
    return "distributed"
}

// EffectiveRedisMode returns the resolved Redis deployment mode.
func (cfg *Config) EffectiveRedisMode() string {
    return ResolveMode(cfg, cfg.Redis.Mode)
}

// EffectiveTemporalMode returns the resolved Temporal deployment mode.
// Normalizes "distributed" → "remote" for Temporal.
func (cfg *Config) EffectiveTemporalMode() string {
    mode := ResolveMode(cfg, cfg.Temporal.Mode)
    if mode == "distributed" {
        return "remote"
    }
    return mode
}

// EffectiveMCPProxyMode returns the resolved MCPProxy deployment mode.
func (cfg *Config) EffectiveMCPProxyMode() string {
    return ResolveMode(cfg, cfg.MCPProxy.Mode)
}
```

### Relevant Files

- `pkg/config/config.go` - Add Config structs
- `pkg/config/resolver.go` - NEW - Mode resolution logic
- `pkg/config/loader.go` - Add validation rules

### Dependent Files

None - this is the foundation task with no dependencies

## Deliverables

- [x] Global `mode` field added to Config struct
- [x] RedisConfig struct with mode and standalone sections
- [x] RedisStandaloneConfig with persistence configuration
- [x] RedisPersistenceConfig with all snapshot settings
- [x] `pkg/config/resolver.go` created with ResolveMode function
- [x] EffectiveRedisMode() method implemented
- [x] EffectiveTemporalMode() method implemented with "distributed" → "remote" normalization
- [x] EffectiveMCPProxyMode() method implemented
- [x] Validation rules for mode fields
- [x] Default mode is "distributed" for backward compatibility

## Tests

Unit tests mapped from `_tests.md` for this feature:

### pkg/config/resolver_test.go (NEW)

- [ ] Should return component mode when explicitly set
  ```go
  func TestResolveMode_ExplicitComponentMode(t *testing.T) {
      t.Run("Should return component mode when explicitly set", func(t *testing.T) {
          cfg := &Config{
              Mode: "standalone",
              Redis: RedisConfig{Mode: "distributed"},
          }
          result := cfg.EffectiveRedisMode()
          assert.Equal(t, "distributed", result)
      })
  }
  ```

- [ ] Should inherit from global mode when component mode is empty
  ```go
  t.Run("Should inherit from global mode", func(t *testing.T) {
      cfg := &Config{
          Mode: "standalone",
          Redis: RedisConfig{Mode: ""},
      }
      result := cfg.EffectiveRedisMode()
      assert.Equal(t, "standalone", result)
  })
  ```

- [ ] Should default to "distributed" when both modes are empty
  ```go
  t.Run("Should default to distributed", func(t *testing.T) {
      cfg := &Config{
          Mode: "",
          Redis: RedisConfig{Mode: ""},
      }
      result := cfg.EffectiveRedisMode()
      assert.Equal(t, "distributed", result)
  })
  ```

- [ ] Should normalize "distributed" to "remote" for Temporal
  ```go
  func TestEffectiveTemporalMode_Normalization(t *testing.T) {
      t.Run("Should normalize distributed to remote for Temporal", func(t *testing.T) {
          cfg := &Config{Mode: "distributed"}
          result := cfg.EffectiveTemporalMode()
          assert.Equal(t, "remote", result)
      })

      t.Run("Should pass through standalone for Temporal", func(t *testing.T) {
          cfg := &Config{Mode: "standalone"}
          result := cfg.EffectiveTemporalMode()
          assert.Equal(t, "standalone", result)
      })
  }
  ```

- [ ] Should validate mode values against allowed enums
- [ ] Should handle mixed mode configurations correctly
- [ ] Should resolve effective modes for all components (Redis, Temporal, MCPProxy)

### pkg/config/loader_test.go (UPDATE)

- [ ] Should validate global mode field (standalone | distributed)
- [ ] Should validate component mode fields
- [ ] Should reject invalid mode values
- [ ] Should allow empty mode values (inheritance)
- [ ] Should validate Redis persistence configuration

## Success Criteria

- [ ] All resolver tests pass (`go test ./pkg/config/...`)
- [ ] Mode resolution logic handles all scenarios (explicit, inherit, default)
- [ ] Temporal mode normalization works correctly
- [ ] Configuration validation rejects invalid mode values
- [ ] Default mode is "distributed" for backward compatibility
- [ ] All helper methods (EffectiveRedisMode, EffectiveTemporalMode, EffectiveMCPProxyMode) work correctly
- [ ] `make lint` passes with zero warnings
- [ ] Code follows project standards (context patterns, error handling)
