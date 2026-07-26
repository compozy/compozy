package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	configSettingsKey = "settings"
)

const (
	configConfigKey   = "config"
	configMutationKey = "mutation"
)

var configTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDConfigShow,
		"config_show",
		"Config Show",
		"Show the redacted effective config for the selected scope.",
		configReadInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey},
		[]string{"effective config", "configuration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigList,
		"config_list",
		"Config List",
		"List redacted effective config entries for the selected scope.",
		configReadInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey},
		[]string{"config entries", "configuration values"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigGet,
		"config_get",
		"Config Get",
		"Read one redacted effective config value by path.",
		configGetInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey},
		[]string{"config path", "configuration value"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigSet,
		"config_set",
		"Config Set",
		"Set one validated non-secret, non-trust-root config overlay value.",
		configSetInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey, configMutationKey},
		[]string{"set config", "write configuration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigUnset,
		"config_unset",
		"Config Unset",
		"Remove one validated non-secret, non-trust-root config overlay value.",
		configUnsetInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey, configMutationKey},
		[]string{"unset config", "delete configuration value"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigDiff,
		"config_diff",
		"Config Diff",
		"Compare redacted effective config entries against defaults or the global scope.",
		configReadInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey, "diff"},
		[]string{"config diff", "configuration changes"},
	),
	nativeDescriptor(
		toolspkg.ToolIDConfigPath,
		"config_path",
		"Config Path",
		"Report resolved global and workspace config paths.",
		configPathInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDConfig},
		[]string{configConfigKey, configSettingsKey, "paths"},
		[]string{"config file path", "workspace config path"},
	),
}

func configDescriptors() []toolspkg.Descriptor {
	return configTools
}

const configReadInputSchema = `{
	"type":"object",
	"properties":{
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`

const configGetInputSchema = `{
	"type":"object",
	"required":["path"],
	"properties":{
		"path":{"type":"string"},
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`

const configSetInputSchema = `{
	"type":"object",
	"required":["path","value"],
	"properties":{
		"path":{"type":"string"},
		"value":{},
		"scope":{"type":"string"},
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`

const configUnsetInputSchema = `{
	"type":"object",
	"required":["path"],
	"properties":{
		"path":{"type":"string"},
		"scope":{"type":"string"},
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`

const configPathInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string"},
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`
