package config

import "maps"

var roleMutableConfigKinds = map[string]ValueKind{
	"roles":                                               ConfigValueTable,
	"roles.coordinator.enabled":                           ConfigValueBool,
	"roles.coordinator.agent":                             ConfigValueString,
	"roles.coordinator.provider":                          ConfigValueString,
	"roles.coordinator.model":                             ConfigValueString,
	"roles.coordinator.reasoning_effort":                  ConfigValueString,
	"roles.coordinator.speed":                             ConfigValueString,
	"roles.coordinator.acp_options":                       ConfigValueACPOptions,
	"roles.coordinator.ttl":                               ConfigValueDuration,
	"roles.coordinator.max_children":                      ConfigValueInt,
	"roles.coordinator.max_active_sessions_per_workspace": ConfigValueInt,
	"roles.auto_title.enabled":                            ConfigValueBool,
	"roles.auto_title.agent":                              ConfigValueString,
	"roles.auto_title.provider":                           ConfigValueString,
	"roles.auto_title.model":                              ConfigValueString,
	"roles.auto_title.reasoning_effort":                   ConfigValueString,
	"roles.auto_title.speed":                              ConfigValueString,
	"roles.auto_title.acp_options":                        ConfigValueACPOptions,
}

// RoleMutableConfigKinds returns the config-owned role mutation registry.
func RoleMutableConfigKinds() map[string]ValueKind {
	return maps.Clone(roleMutableConfigKinds)
}
