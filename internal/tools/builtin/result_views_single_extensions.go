package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultExtensions = map[toolspkg.ToolID]SingleResultReason{
	"compozy__extensions_disable":    SingleResultMinimal,
	"compozy__extensions_enable":     SingleResultMinimal,
	"compozy__extensions_init":       SingleResultMinimal,
	"compozy__extensions_inventory":  SingleResultMinimal,
	"compozy__extensions_logs":       SingleResultContent,
	"compozy__extensions_provenance": SingleResultContent,
	"compozy__extensions_publish":    SingleResultMinimal,
	"compozy__extensions_remove":     SingleResultMinimal,
	"compozy__extensions_search":     SingleResultCallerBounded,
	"compozy__extensions_update":     SingleResultMinimal,
	"compozy__extensions_validate":   SingleResultContent,
}
