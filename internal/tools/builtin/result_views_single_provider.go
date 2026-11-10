package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultProvider = map[toolspkg.ToolID]SingleResultReason{
	"compozy__provider_models_refresh": SingleResultMinimal,
	"compozy__provider_models_status":  SingleResultMinimal,
	"compozy__provider_usage_list":     SingleResultContent,
	"compozy__provider_usage_refresh":  SingleResultContent,
}
