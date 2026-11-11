package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultVault = map[toolspkg.ToolID]SingleResultReason{
	"compozy__vault_list": SingleResultMinimal,
}
