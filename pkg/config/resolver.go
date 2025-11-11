package config

import "strings"

// Deployment modes (shared across components).
const (
	ModeMemory      = "memory"      // In-memory SQLite with embedded services
	ModePersistent  = "persistent"  // File-backed SQLite with embedded services
	ModeDistributed = "distributed" // External services (PostgreSQL, Temporal, Redis)
	// Temporal-only normalized mode for remote clusters
	ModeRemoteTemporal = "remote"
)

// ResolveMode determines the effective deployment mode for a component.
//
// Resolution priority:
//  1. Component mode (if explicitly set)
//  2. Global mode (if set in Config.Mode)
//  3. Default fallback ("memory")
func ResolveMode(cfg *Config, componentMode string) string {
	if componentMode != "" {
		return componentMode
	}
	if cfg != nil && cfg.Mode != "" {
		return cfg.Mode
	}
	return ModeMemory
}

// EffectiveRedisMode returns the resolved Redis deployment mode.
func (cfg *Config) EffectiveRedisMode() string {
	return ResolveMode(cfg, cfg.Redis.Mode)
}

// EffectiveTemporalMode returns the resolved Temporal deployment mode.
// Embedded modes (memory, persistent) run Temporal locally; distributed uses remote Temporal clusters.
func (cfg *Config) EffectiveTemporalMode() string {
	mode := ResolveMode(cfg, cfg.Temporal.Mode)
	if mode == ModeDistributed {
		return ModeRemoteTemporal
	}
	return mode
}

// EffectiveMCPProxyMode returns the resolved MCPProxy deployment mode.
func (cfg *Config) EffectiveMCPProxyMode() string {
	return ResolveMode(cfg, cfg.MCPProxy.Mode)
}

// EffectiveDatabaseDriver resolves the database driver with mode-aware defaults.
// Defaults:
//   - Memory and persistent modes -> sqlite (unless overridden)
//   - Distributed mode -> postgres
//   - Unset config -> sqlite (aligns with memory default)
func (cfg *Config) EffectiveDatabaseDriver() string {
	if cfg == nil {
		return databaseDriverSQLite
	}
	driver := strings.TrimSpace(cfg.Database.Driver)
	if driver != "" {
		return driver
	}
	switch strings.TrimSpace(cfg.Mode) {
	case ModeMemory, ModePersistent:
		return databaseDriverSQLite
	case ModeDistributed:
		return databaseDriverPostgres
	}
	return databaseDriverSQLite
}
