package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var marketplaceTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceCategories,
		"marketplace_categories",
		"Marketplace Categories",
		"List the Marketplace taxonomy with counts within an optional query.",
		`{"type":"object","properties":{"query":{"type":"string"}},"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{extensionsMarketplaceKey, "categories"},
		[]string{"marketplace categories"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMarketplaceSources,
		"marketplace_sources",
		"Marketplace Sources",
		"List configured plugin marketplace sources, enablement, counts, and diagnostics. Stability: experimental.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMarketplace},
		[]string{
			extensionsMarketplaceKey,
			"sources",
			"plugins",
		},
		[]string{"marketplace sources", "list plugin sources"},
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
		[]string{extensionsMarketplaceKey, descriptorKeywordCatalog, "mcp", "extensions", "skills"},
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
 "category":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":100},
		"cursor":{"type":"string","description":"Opaque catalog continuation cursor"}
	},
	"additionalProperties":false
}`
