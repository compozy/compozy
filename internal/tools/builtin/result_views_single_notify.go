package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultNotify = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDNotify: SingleResultMinimal,
}
