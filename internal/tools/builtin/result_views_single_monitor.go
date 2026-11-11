package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultMonitor = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDMonitorCancel: SingleResultMinimal,
	toolspkg.ToolIDMonitorCreate: SingleResultMinimal,
	toolspkg.ToolIDMonitorList:   SingleResultMinimal,
}
