package config

func callsToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		"calls.max_depth":                      ConfigValueInt,
		callsMaxBatchPath:                      ConfigValueInt,
		"calls.max_children":                   ConfigValueInt,
		"calls.max_active_per_root":            ConfigValueInt,
		"calls.idle_ttl":                       ConfigValueString,
		"calls.results.default_budget":         ConfigValueString,
		"calls.results.max_budget":             ConfigValueString,
		"calls.results.overflow":               ConfigValueString,
		"calls.messages.rate_limit_per_minute": ConfigValueInt,
		"calls.messages.dedup_window":          ConfigValueString,
		"calls.messages.pending_cap":           ConfigValueInt,
		"calls.messages.max_bytes":             ConfigValueString,
	}
}
