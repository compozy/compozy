package config

import "strings"

// Deployment modes (shared across components).
const (
	ModeStandalone  = "standalone"
	ModeDistributed = "distributed"
	// Temporal-only normalized mode for remote clusters
	ModeRemoteTemporal = "remote"
)

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
	if cfg != nil && cfg.Mode != "" {
		return cfg.Mode
	}
	return ModeDistributed
}

// EffectiveRedisMode returns the resolved Redis deployment mode.
func (cfg *Config) EffectiveRedisMode() string {
	return ResolveMode(cfg, cfg.Redis.Mode)
}

// EffectiveTemporalMode returns the resolved Temporal deployment mode.
// Normalizes "distributed" → "remote" for Temporal.
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

// EffectiveDatabaseDriver resolves the database driver with global mode fallback.
// Defaults:
//   - Global mode "standalone" -> sqlite (unless overridden)
//   - All other modes -> postgres
func (cfg *Config) EffectiveDatabaseDriver() string {
	if cfg == nil {
		return databaseDriverPostgres
	}
	driver := strings.TrimSpace(cfg.Database.Driver)
	if driver != "" {
		return driver
	}
	if strings.TrimSpace(cfg.Mode) == ModeStandalone {
		return databaseDriverSQLite
	}
	return databaseDriverPostgres
}
