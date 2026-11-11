package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultAutomation = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDAutomationJobsDelete:      SingleResultMinimal,
	toolspkg.ToolIDAutomationJobsHistory:     SingleResultCallerBounded,
	toolspkg.ToolIDAutomationJobsTrigger:     SingleResultMinimal,
	toolspkg.ToolIDAutomationRunsGet:         SingleResultMinimal,
	toolspkg.ToolIDAutomationRunsList:        SingleResultCallerBounded,
	toolspkg.ToolIDAutomationTriggersDelete:  SingleResultMinimal,
	toolspkg.ToolIDAutomationTriggersHistory: SingleResultCallerBounded,
}
