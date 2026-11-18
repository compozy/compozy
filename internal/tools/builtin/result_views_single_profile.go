package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultProfile = map[toolspkg.ToolID]SingleResultReason{
	"compozy__profile_current": SingleResultMinimal,
	"compozy__profile_list":    SingleResultMinimal,
}
