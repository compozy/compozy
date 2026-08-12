package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	extensionsMarketplaceKey = "marketplace"
)

const (
	extensionsExtensionsKey = "extensions"
)

const (
	extensionsAuthoringKey   = "authoring"
	extensionsCatalogKey     = "catalog"
	extensionsDevelopmentKey = "development"
	extensionsMutationKey    = "mutation"
	extensionsStatusKey      = "status"
)

var extensionTools = []toolspkg.Descriptor{
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsInit,
		"extensions_init",
		"Extensions Init",
		"Create an extension project from an embedded SDK template.",
		extensionInitInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, extensionsAuthoringKey, "init"},
		[]string{"create extension project", "scaffold extension"},
	),
	nativeInteractiveExtensionDescriptor(
		toolspkg.ToolIDExtensionsBuild,
		"extensions_build",
		"Extensions Build",
		"Build source into an immutable content-hash extension generation.",
		extensionBuildInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, extensionsAuthoringKey, "build"},
		[]string{"build extension", "compile extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsValidate,
		"extensions_validate",
		"Extensions Validate",
		"Validate an extension bundle without executing extension code.",
		extensionValidateInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsAuthoringKey, descriptorKeywordValidate},
		[]string{"validate extension", "check extension bundle"},
	),
	nativeInteractiveExtensionDescriptor(
		toolspkg.ToolIDExtensionsDev,
		"extensions_dev",
		"Extensions Dev",
		"Link an immutable extension generation to the trusted workspace.",
		extensionDevInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, extensionsDevelopmentKey, extensionsMutationKey},
		[]string{"link development extension", "dev extension"},
	),
	nativeInteractiveExtensionDescriptor(
		toolspkg.ToolIDExtensionsReload,
		"extensions_reload",
		"Extensions Reload",
		"Atomically reload a dev-linked extension from an immutable generation.",
		extensionReloadInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, extensionsDevelopmentKey, "reload"},
		[]string{"reload extension", "restart dev extension"},
	),
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsLogs,
		"extensions_logs",
		"Extensions Logs",
		"Read retained redacted stderr for the trusted extension instance.",
		extensionLogsInputSchema,
		extensionLogsOutputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsDevelopmentKey, "logs"},
		[]string{"extension logs", "extension stderr"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsList,
		"extensions_list",
		"Extensions List",
		"List installed extensions through the daemon extension registry.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, "installed", extensionsCatalogKey},
		[]string{"installed extensions", "extension status"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsInfo,
		"extensions_info",
		"Extensions Info",
		"Read one installed extension status and runtime projection.",
		extensionNameInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsStatusKey},
		[]string{"extension info", "extension status"},
	),
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsInventory,
		"extensions_inventory",
		"Extensions Inventory",
		"Read shipped and live resources for one installed extension.",
		extensionNameInputSchema,
		extensionInventoryOutputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsStatusKey, "inventory", "resources"},
		[]string{"extension inventory", "extension resources"},
	),
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsPreview,
		"extensions_preview",
		"Extensions Preview",
		"Preview enabling one installed extension without changing state.",
		extensionNameInputSchema,
		extensionEnablePreviewOutputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsStatusKey, "preview", "enable"},
		[]string{"preview extension enable", "extension dry run"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsInstall,
		"extensions_install",
		"Extensions Install",
		"Install one extension through the managed local or marketplace lifecycle.",
		extensionInstallInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, "install", extensionsMarketplaceKey, extensionsMutationKey},
		[]string{"install extension", "add extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsUpdate,
		"extensions_update",
		"Extensions Update",
		"Update marketplace-installed extensions through the managed lifecycle.",
		extensionUpdateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, descriptorKeywordUpdate, extensionsMarketplaceKey, extensionsMutationKey},
		[]string{"update extension", "upgrade extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsRemove,
		"extensions_remove",
		"Extensions Remove",
		"Remove one managed installed extension with rollback on reload failure.",
		extensionNameInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{extensionsExtensionsKey, "remove", extensionsMutationKey},
		[]string{"remove extension", "uninstall extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsEnable,
		"extensions_enable",
		"Extensions Enable",
		"Enable one installed extension through the runtime extension lifecycle.",
		extensionEnableInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, "enable", extensionsMutationKey},
		[]string{"enable extension", "activate extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsDisable,
		"extensions_disable",
		"Extensions Disable",
		"Disable one installed extension through the runtime extension lifecycle.",
		extensionNameInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{extensionsExtensionsKey, "disable", extensionsMutationKey},
		[]string{"disable extension", "deactivate extension"},
	),
}

func nativeInteractiveExtensionDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	descriptor := nativeExtensionDescriptor(
		id,
		nativeName,
		title,
		description,
		inputSchema,
		risk,
		readOnly,
		destructive,
		tags,
		searchHints,
	)
	descriptor.RequiresInteraction = true
	return descriptor
}

func extensionDescriptors() []toolspkg.Descriptor {
	items := make([]toolspkg.Descriptor, 0, len(extensionTools)+len(extensionDistributionTools))
	items = append(items, extensionTools...)
	items = append(items, extensionDistributionTools...)
	return items
}

func nativeExtensionDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	return nativeDescriptor(
		id,
		nativeName,
		title,
		description,
		inputSchema,
		risk,
		readOnly,
		destructive,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDExtensions},
		tags,
		searchHints,
	)
}

func nativeExtensionDescriptorWithOutput(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	outputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	descriptor := nativeExtensionDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, destructive, tags, searchHints,
	)
	descriptor.OutputSchema = json.RawMessage(outputSchema)
	return descriptor
}

const extensionNameInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"}
	},
	"additionalProperties":false
}`

const extensionEnableInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"},
		"confirm_network_digest":{"type":"string","pattern":"^[a-f0-9]{64}$"}
	},
	"additionalProperties":false
}`

const extensionInstallInputSchema = `{
	"type":"object",
	"required":["source","ref"],
	"properties":{
		"source":{"type":"string","enum":["curated","github","git","local_path"]},
		"ref":{"type":"string","minLength":1},
		"version":{"type":"string"},
		"asset":{"type":"string"},
		"allow_unverified":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const extensionUpdateInputSchema = `{
	"type":"object",
	"properties":{
		"name":{"type":"string"},
		"all":{"type":"boolean"},
		"check_only":{"type":"boolean"},
		"version":{"type":"string"},
		"allow_unverified":{"type":"boolean"},
		"confirm_network_digest":{"type":"string","pattern":"^[a-f0-9]{64}$"}
	},
	"additionalProperties":false
}`

const extensionInitInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string","minLength":1},
		"template":{"type":"string"},
		"directory":{"type":"string"}
	},
	"additionalProperties":false
}`

const extensionBuildInputSchema = `{
	"type":"object",
	"required":["source_dir"],
	"properties":{
		"source_dir":{"type":"string","minLength":1},
		"output_dir":{"type":"string"},
		"build_command":{"type":"array","items":{"type":"string"}},
		"timeout_ms":{"type":"integer","minimum":1}
	},
	"additionalProperties":false
}`

const extensionValidateInputSchema = `{
	"type":"object",
	"required":["path"],
	"properties":{"path":{"type":"string","minLength":1}},
	"additionalProperties":false
}`

const extensionDevInputSchema = `{
	"type":"object",
	"required":["origin_path","generation_hash"],
	"properties":{
		"origin_path":{"type":"string","minLength":1},
		"generation_hash":{"type":"string","pattern":"^[a-f0-9]{64}$"},
		"confirm_network_digest":{"type":"string","pattern":"^[a-f0-9]{64}$"}
	},
	"additionalProperties":false
}`

const extensionReloadInputSchema = `{
	"type":"object",
	"required":["name","generation_hash"],
	"properties":{
		"name":{"type":"string","minLength":1},
		"generation_hash":{"type":"string","pattern":"^[a-f0-9]{64}$"},
		"confirm_network_digest":{"type":"string","pattern":"^[a-f0-9]{64}$"}
	},
	"additionalProperties":false
}`

const extensionLogsInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string","minLength":1},
		"after":{"type":"integer","minimum":0,"description":"Return entries after this sequence within stream_epoch"},
		"stream_epoch":{"type":"string","minLength":1,"description":` +
	`"Opaque ring identity returned by an earlier log read; required when after is greater than zero"}
	},
	"additionalProperties":false
}`

const extensionLogsOutputSchema = `{
	"type":"object",
	"required":["logs","stream_epoch"],
	"properties":{
		"stream_epoch":{"type":"string","minLength":1},
		"logs":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["sequence","timestamp","message","stream_epoch"],
				"properties":{
					"sequence":{"type":"integer","minimum":1},
					"timestamp":{"type":"string","format":"date-time"},
					"message":{"type":"string"},
					"generation_hash":{"type":"string"},
					"stream_epoch":{"type":"string","minLength":1}
				},
				"additionalProperties":false
			}
		}
	},
	"additionalProperties":false
}`

const extensionInventoryOutputSchema = `{
	"type":"object",
	"required":["extension","enabled","items"],
	"properties":{
		"extension":{"type":"string"},
		"enabled":{"type":"boolean"},
		"items":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["kind","id","name","live"],
				"properties":{
					"kind":{"type":"string"},
					"id":{"type":"string"},
					"name":{"type":"string"},
					"live":{"type":"boolean"}
				},
				"additionalProperties":false
			}
		}
	},
	"additionalProperties":false
}`

const extensionEnablePreviewOutputSchema = `{
	"type":"object",
	"required":[
		"extension","changes","agent_conflicts","missing_env","automation_starting",
		"network_requirement_digest","network_confirmation_required"
	],
	"properties":{
		"extension":{"type":"string"},
		"changes":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["kind","id","name","change"],
				"properties":{
					"kind":{"type":"string"},
					"id":{"type":"string"},
					"name":{"type":"string"},
					"change":{"type":"string","enum":["added","changed","removed"]}
				},
				"additionalProperties":false
			}
		},
		"agent_conflicts":{"type":"array","items":{"type":"string"}},
		"missing_env":{"type":"array","items":{"type":"string"}},
		"automation_starting":{"type":"array","items":{"type":"string"}},
		"network_requirement_digest":{"type":"string","pattern":"^$|^[a-f0-9]{64}$"},
		"network_confirmation_required":{"type":"boolean"}
	},
	"additionalProperties":false
}`
