package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var marketplaceTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceSearch,
		"marketplace_search",
		"Marketplace Search",
		"Search or browse MCP servers, extensions, and skills through the shared marketplace.",
		marketplaceSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{"marketplace", hooksCatalogKey, "mcp", "extensions", "skills"},
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
		"kind":{"type":"string","enum":["mcp","extension","skill"]},
		"limit":{"type":"integer","minimum":1,"maximum":100},
		"cursor":{"type":"string","description":"Opaque continuation cursor; requires kind"}
	},
	"additionalProperties":false
}`
