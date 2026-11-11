package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultHooks = map[toolspkg.ToolID]SingleResultReason{
	"compozy__hooks_create":  SingleResultMinimal,
	"compozy__hooks_delete":  SingleResultMinimal,
	"compozy__hooks_disable": SingleResultMinimal,
	"compozy__hooks_enable":  SingleResultMinimal,
	"compozy__hooks_events":  SingleResultContent,
	"compozy__hooks_info":    SingleResultMinimal,
	"compozy__hooks_list":    SingleResultMinimal,
	"compozy__hooks_runs":    SingleResultContent,
	"compozy__hooks_update":  SingleResultMinimal,
}
