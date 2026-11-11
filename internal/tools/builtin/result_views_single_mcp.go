package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultMcp = map[toolspkg.ToolID]SingleResultReason{
	"compozy__mcp_auth_status": SingleResultMinimal,
	"compozy__mcp_status":      SingleResultMinimal,
}
