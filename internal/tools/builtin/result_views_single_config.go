package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultConfig = map[toolspkg.ToolID]SingleResultReason{
	"compozy__config_diff":  SingleResultContent,
	"compozy__config_get":   SingleResultContent,
	"compozy__config_list":  SingleResultContent,
	"compozy__config_path":  SingleResultMinimal,
	"compozy__config_set":   SingleResultMinimal,
	"compozy__config_show":  SingleResultContent,
	"compozy__config_unset": SingleResultMinimal,
}
