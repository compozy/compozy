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
	if cfg != nil && cfg.Mode != "" {
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
