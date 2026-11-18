package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultTask = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDTaskBlock:                  SingleResultMinimal,
	toolspkg.ToolIDTaskBlocks:                 SingleResultContent,
	toolspkg.ToolIDTaskExecutionProfileDelete: SingleResultMinimal,
	toolspkg.ToolIDTaskExecutionProfileGet:    SingleResultContent,
	toolspkg.ToolIDTaskExecutionProfileSet:    SingleResultMinimal,
	toolspkg.ToolIDTaskRunClaimNext:           SingleResultMinimal,
	toolspkg.ToolIDTaskRunComplete:            SingleResultMinimal,
	toolspkg.ToolIDTaskRunFail:                SingleResultMinimal,
	toolspkg.ToolIDTaskRunHeartbeat:           SingleResultMinimal,
	toolspkg.ToolIDTaskRunRelease:             SingleResultMinimal,
	toolspkg.ToolIDTaskRunResult:              SingleResultContent,
	toolspkg.ToolIDTaskRunReviewShow:          SingleResultContent,
	toolspkg.ToolIDTaskUnblock:                SingleResultMinimal,
	toolspkg.ToolIDTaskWorktreePolicySet:      SingleResultMinimal,
}
