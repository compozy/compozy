package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	catalogToolsKey = "tools"
)

const (
	catalogRegistryKey = "registry"
)

const (
	catalogCatalogKey            = "catalog"
	toolListMaxResultBytes int64 = 1 << 20
)

var catalogTools = []toolspkg.Descriptor{
	toolListDescriptor(),
	nativeDescriptor(
		toolspkg.ToolIDToolSearch,
		"tool_search",
		"Tool Search",
		"Search the caller's tool catalog with effective callable or denied diagnostics.",
		toolSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBootstrap, toolspkg.ToolsetIDCatalog},
		[]string{catalogToolsKey, catalogRegistryKey, catalogCatalogKey},
		[]string{"find tools", "tool registry search"},
	),
	nativeDescriptor(
		toolspkg.ToolIDToolInfo,
		"tool_info",
		"Tool Info",
		"Read one known tool descriptor and effective callable or denied diagnostics view.",
		toolInfoInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBootstrap, toolspkg.ToolsetIDCatalog},
		[]string{catalogToolsKey, catalogRegistryKey, descriptorKeywordDiagnostics},
		[]string{"tool descriptor", "tool policy diagnostics"},
	),
}

func toolListDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDToolList,
		"tool_list",
		"Tool List",
		"List tools currently callable in the caller's effective registry projection.",
		toolListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBootstrap, toolspkg.ToolsetIDCatalog},
		[]string{catalogToolsKey, catalogRegistryKey, catalogCatalogKey},
		[]string{"available tools", "tool registry"},
	)
	descriptor.MaxResultBytes = toolListMaxResultBytes
	return descriptor
}

func catalogDescriptors() []toolspkg.Descriptor {
	return catalogTools
}

const toolListInputSchema = `{
	"type":"object",
	"properties":{
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const toolSearchInputSchema = `{
	"type":"object",
	"required":["query"],
	"properties":{
		"query":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const toolInfoInputSchema = `{
	"type":"object",
	"required":["tool_id"],
	"properties":{
		"tool_id":{"type":"string"}
	},
	"additionalProperties":false
}`
