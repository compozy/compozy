package config

import "maps"

var roleMutableConfigKinds = map[string]ValueKind{
	"roles":                                               ConfigValueTable,
	"roles.coordinator.enabled":                           ConfigValueBool,
	"roles.coordinator.agent":                             ConfigValueString,
	"roles.coordinator.provider":                          ConfigValueString,
	"roles.coordinator.model":                             ConfigValueString,
	"roles.coordinator.reasoning_effort":                  ConfigValueString,
	"roles.coordinator.ttl":                               ConfigValueDuration,
	"roles.coordinator.max_children":                      ConfigValueInt,
	"roles.coordinator.max_active_sessions_per_workspace": ConfigValueInt,
	"roles.dream.enabled":                                 ConfigValueBool,
	"roles.dream.agent":                                   ConfigValueString,
	"roles.dream.provider":                                ConfigValueString,
	"roles.dream.model":                                   ConfigValueString,
	"roles.dream.reasoning_effort":                        ConfigValueString,
	"roles.checkpoint_summary.enabled":                    ConfigValueBool,
	"roles.checkpoint_summary.agent":                      ConfigValueString,
	"roles.checkpoint_summary.provider":                   ConfigValueString,
	"roles.checkpoint_summary.model":                      ConfigValueString,
	"roles.checkpoint_summary.reasoning_effort":           ConfigValueString,
	"roles.memory_extractor.enabled":                      ConfigValueBool,
	"roles.memory_extractor.agent":                        ConfigValueString,
	"roles.memory_extractor.provider":                     ConfigValueString,
	"roles.memory_extractor.model":                        ConfigValueString,
	"roles.memory_extractor.reasoning_effort":             ConfigValueString,
	"roles.auto_title.enabled":                            ConfigValueBool,
	"roles.auto_title.agent":                              ConfigValueString,
	"roles.auto_title.provider":                           ConfigValueString,
	"roles.auto_title.model":                              ConfigValueString,
	"roles.auto_title.reasoning_effort":                   ConfigValueString,
	"roles.memory_controller.enabled":                     ConfigValueBool,
	"roles.memory_controller.provider":                    ConfigValueString,
	"roles.memory_controller.model":                       ConfigValueString,
	"roles.memory_controller.reasoning_effort":            ConfigValueString,
	"roles.memory_controller.timeout":                     ConfigValueDuration,
	"roles.memory_controller.top_k":                       ConfigValueInt,
	"roles.memory_controller.prompt_version":              ConfigValueString,
	"roles.memory_controller.max_tokens_out":              ConfigValueInt,
}

// RoleMutableConfigKinds returns the config-owned role mutation registry.
func RoleMutableConfigKinds() map[string]ValueKind {
	return maps.Clone(roleMutableConfigKinds)
}
