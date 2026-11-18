package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultLogs = map[toolspkg.ToolID]SingleResultReason{
	"compozy__logs": SingleResultContent,
}
