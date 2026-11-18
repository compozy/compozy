package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultMcp = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDMCPList:        SingleResultCallerBounded,
	toolspkg.ToolIDMCPAllow:       SingleResultMinimal,
	toolspkg.ToolIDMCPImport:      SingleResultMinimal,
	toolspkg.ToolIDSessionMCPList: SingleResultCallerBounded,
	"compozy__mcp_auth_status":    SingleResultMinimal,
	"compozy__mcp_status":         SingleResultMinimal,
}
