package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var marketplaceTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceSources, "marketplace_sources", "Marketplace Sources",
		"List configured plugin marketplace sources, enablement, counts, and diagnostics. Stability: experimental.",
		emptyInputSchema,
		toolspkg.RiskRead, true, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{"marketplace", "sources", "plugins"}, []string{"marketplace sources", "list plugin sources"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceSearch,
		"marketplace_search",
		"Marketplace Search",
		"Search or browse the Marketplace extension catalog.",
		marketplaceSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{"marketplace", descriptorKeywordCatalog, "mcp", "extensions", "skills"},
		[]string{"marketplace search", "discover capabilities", "find extensions", "find skills"},
	),
}

func marketplaceDescriptors() []toolspkg.Descriptor {
	return marketplaceTools
}

const marketplaceSearchInputSchema = `{
	"type":"object",
	"properties":{
		"query":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":100},
		"cursor":{"type":"string","description":"Opaque catalog continuation cursor"}
	},
	"additionalProperties":false
}`
