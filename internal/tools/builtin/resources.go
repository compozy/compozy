package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const resourcesKey = "resources"

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

func resourceDescriptors() []toolspkg.Descriptor {
	return resourceTools
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

const resourceFilterInputSchema = `{"type":"object","properties":{"kind":{"type":"string"},"limit":{"type":"integer"},"scope_kind":{"type":"string"},"scope_id":{"type":"string"},"owner_kind":{"type":"string"},"owner_id":{"type":"string"},"source_kind":{"type":"string"},"source_id":{"type":"string"}},"additionalProperties":false}`

const resourceInfoInputSchema = `{"type":"object","required":["kind","id"],"properties":{"kind":{"type":"string"},"id":{"type":"string"}},"additionalProperties":false}`
