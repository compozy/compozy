package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

// extensionSetupTools read or complete the inputs of one installed extension in the caller's trusted
// workspace and profile. A call without inputs is a read; agents never overwrite inputs that are set (D7).
var extensionSetupTools = []toolspkg.Descriptor{
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsSetup,
		"extensions_setup",
		"Extensions Setup",
		"Read input state of one installed extension, or write inputs that are missing, invalid or "+
			"daemon_env in the caller's workspace and profile. Inputs already set are operator-only "+
			"(extension_input_overwrite_denied); values are never returned.",
		extensionSetupInputSchema,
		extensionSetupOutputSchema,
		toolspkg.RiskMutating,
		false,
		[]string{extensionsExtensionsKey, "setup", inputsTagKey, extensionsMutationKey},
		[]string{"extension setup", "configure extension inputs", "missing extension inputs"},
	),
}

const extensionSetupInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"owner":{"type":"string","pattern":"` + extensionOwnerPattern + `",
			"description":"Definition owner; must equal extension:<name>."},
		"name":{"type":"string","minLength":1},
		"inputs":` + extensionValuesInputSchema + `
	},
	"additionalProperties":false
}`

const extensionSetupOutputSchema = `{
	"type":"object",
	"required":["extension","workspace_id","profile","enabled","inputs","missing","invalid","daemon_env",
		"missing_env","mcp_auth_servers"],
	"properties":{
		"extension":{"type":"string"},
		"workspace_id":{"type":"string"},
		"profile":{"type":"string"},
		"enabled":{"type":"boolean"},
		"inputs":{"type":"array","items":{
			"type":"object",
			"required":["id","prompt","type","required","binding","targets","state"],
			"properties":{
				"id":{"type":"string"},
				"prompt":{"type":"string"},
				"type":{"type":"string"},
				"required":{"type":"boolean"},
				"binding":{"type":"object","required":["type","name"],
					"properties":{"type":{"type":"string"},"name":{"type":"string"}},"additionalProperties":false},
				"default":{"type":["string","boolean"]},
				"targets":{"type":"array","items":{"type":"string"}},
				"state":{"type":"string","enum":["set","missing","invalid","daemon_env"]}
			},
			"additionalProperties":false
		}},
		"missing":{"type":"array","items":{"type":"string"}},
		"invalid":{"type":"array","items":{"type":"string"}},
		"daemon_env":{"type":"array","items":{"type":"string"}},
		"missing_env":{"type":"array","items":{"type":"string"}},
		"mcp_auth_servers":{"type":"array","items":{"type":"string"}},
		"changed":{"type":"array","items":{"type":"string"}},
		"reloaded":{"type":"boolean"}
	},
	"additionalProperties":false
}`
