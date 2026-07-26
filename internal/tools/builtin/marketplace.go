package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

var marketplaceTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceSearch,
		"marketplace_search",
		"Marketplace Search",
		"Search or browse MCP servers, extensions, skills, and bundles through the shared marketplace.",
		marketplaceSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{"marketplace", "catalog", "mcp", "extensions", "skills", "bundles"},
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
		"kind":{"type":"string","enum":["mcp","extension","skill","bundle"]},
		"limit":{"type":"integer","minimum":1,"maximum":100},
		"cursor":{"type":"string","description":"Opaque continuation cursor; requires kind"}
	},
	"additionalProperties":false
}`
