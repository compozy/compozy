package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultMarketplace = map[toolspkg.ToolID]SingleResultReason{
	"compozy__marketplace_categories": SingleResultMinimal,
}
