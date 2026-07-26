package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	extensionsMarketplaceKey = "marketplace"
)

const (
	extensionsExtensionsKey = "extensions"
)

const (
	extensionsCatalogKey  = "catalog"
	extensionsMutationKey = "mutation"
	extensionsStatusKey   = "status"
)

var extensionTools = []toolspkg.Descriptor{
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
		extensionNameInputSchema,
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

func extensionDescriptors() []toolspkg.Descriptor {
	return extensionTools
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

const extensionNameInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"}
	},
	"additionalProperties":false
}`

const extensionInstallInputSchema = `{
	"type":"object",
	"properties":{
		"source":{"type":"string","enum":["local","marketplace"]},
		"path":{"type":"string"},
		"checksum":{"type":"string"},
		"slug":{"type":"string"},
		"registry":{"type":"string"},
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
		"allow_unverified":{"type":"boolean"}
	},
	"additionalProperties":false
}`
