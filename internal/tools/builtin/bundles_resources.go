package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	bundlesKey   = "bundles"
	resourcesKey = "resources"
	mcpKey       = "mcp"
)

var bundleTools = []toolspkg.Descriptor{
	nativeBundleDescriptor(
		toolspkg.ToolIDBundlesList,
		"bundles_list",
		"Bundles List",
		"List the extension bundle catalog and active bundle records.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{bundlesKey, descriptorKeywordCatalog, descriptorKeywordStatus},
		[]string{"bundle catalog", "active bundles"},
	),
	nativeBundleDescriptor(
		toolspkg.ToolIDBundlesInfo,
		"bundles_info",
		"Bundles Info",
		"Read one active bundle record.",
		bundleInfoInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{bundlesKey, descriptorKeywordStatus},
		[]string{"bundle activation", "bundle status"},
	),
	nativeBundleDescriptor(
		toolspkg.ToolIDBundlesActivate,
		"bundles_activate",
		"Bundles Activate",
		"Activate one extension bundle profile through the daemon bundle service.",
		bundleActivateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{bundlesKey, "activate", "mutation"},
		[]string{"activate bundle", "bundle profile"},
	),
	nativeBundleDescriptor(
		toolspkg.ToolIDBundlesDeactivate,
		"bundles_deactivate",
		"Bundles Deactivate",
		"Deactivate one bundle activation through the daemon bundle service.",
		bundleInfoInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{bundlesKey, "deactivate", "mutation"},
		[]string{"deactivate bundle", "remove bundle activation"},
	),
	nativeBundleDescriptor(
		toolspkg.ToolIDBundlesStatus,
		"bundles_status",
		"Bundles Status",
		"Report bundle catalog, activation, and network-default status.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{bundlesKey, descriptorKeywordStatus, networkNetworkKey},
		[]string{"bundle status", "bundle network defaults"},
	),
}

var resourceTools = []toolspkg.Descriptor{
	nativeResourceDescriptor(
		toolspkg.ToolIDResourcesList,
		"resources_list",
		"Resources List",
		"List desired-state resource records through the daemon resource service.",
		resourceFilterInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{resourcesKey, descriptorKeywordCatalog},
		[]string{"list resources", "desired state"},
	),
	nativeResourceDescriptor(
		toolspkg.ToolIDResourcesInfo,
		"resources_info",
		"Resources Info",
		"Read one desired-state resource record.",
		resourceInfoInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{resourcesKey, descriptorKeywordStatus},
		[]string{"resource info", "desired-state record"},
	),
	nativeResourceDescriptor(
		toolspkg.ToolIDResourcesSnapshot,
		"resources_snapshot",
		"Resources Snapshot",
		"Read a filtered desired-state resource snapshot.",
		resourceFilterInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{resourcesKey, descriptorKeywordSnapshot},
		[]string{"resource snapshot", "desired-state snapshot"},
	),
}

var mcpTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMCPStatus,
		"mcp_status",
		"MCP Status",
		"Probe one configured MCP server without exposing login or logout as tool calls.",
		mcpAuthStatusInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, "status", "probe"},
		[]string{"mcp status", "mcp probe", "mcp server health"},
	),
}

func bundleDescriptors() []toolspkg.Descriptor {
	return bundleTools
}

func resourceDescriptors() []toolspkg.Descriptor {
	return resourceTools
}

func mcpDescriptors() []toolspkg.Descriptor {
	return mcpTools
}

func nativeBundleDescriptor(
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
	descriptor := nativeDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, destructive, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBundles}, tags, searchHints,
	)
	if readOnly {
		return withRequiredCapabilities(descriptor, "bundles.read")
	}
	return withRequiredCapabilities(descriptor, "bundles.write")
}

func nativeResourceDescriptor(
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
	descriptor := nativeDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, destructive, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDResources}, tags, searchHints,
	)
	if readOnly {
		return withRequiredCapabilities(descriptor, "resources.read")
	}
	return withRequiredCapabilities(descriptor, "resources.write")
}

const bundleInfoInputSchema = `{"type":"object","required":["id"],"properties":{"id":{"type":"string"}},"additionalProperties":false}`

const bundleActivateInputSchema = `{"type":"object","required":["extension_name","bundle_name"],"properties":{"extension_name":{"type":"string"},"bundle_name":{"type":"string"},"profile_name":{"type":"string"},"scope":{"type":"string"},"workspace":{"type":"string"},"confirm_network_requirement":{"type":"boolean"}},"additionalProperties":false}`

const resourceFilterInputSchema = `{"type":"object","properties":{"kind":{"type":"string"},"limit":{"type":"integer"},"scope_kind":{"type":"string"},"scope_id":{"type":"string"},"owner_kind":{"type":"string"},"owner_id":{"type":"string"},"source_kind":{"type":"string"},"source_id":{"type":"string"}},"additionalProperties":false}`

const resourceInfoInputSchema = `{"type":"object","required":["kind","id"],"properties":{"kind":{"type":"string"},"id":{"type":"string"}},"additionalProperties":false}`
