package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultSubagent = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDSubagentCancel:   SingleResultMinimal,
	toolspkg.ToolIDSubagentDelegate: SingleResultMinimal,
	toolspkg.ToolIDSubagentStatus:   SingleResultContent,
}
