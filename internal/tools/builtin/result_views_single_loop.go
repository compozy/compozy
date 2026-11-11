package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultLoop = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDLoopApprove:     SingleResultMinimal,
	toolspkg.ToolIDLoopCancel:      SingleResultMinimal,
	toolspkg.ToolIDLoopDelete:      SingleResultMinimal,
	toolspkg.ToolIDLoopDiff:        SingleResultContent,
	toolspkg.ToolIDLoopNodeCancel:  SingleResultMinimal,
	toolspkg.ToolIDLoopNodePause:   SingleResultMinimal,
	toolspkg.ToolIDLoopNodeRequeue: SingleResultMinimal,
	toolspkg.ToolIDLoopNodeResume:  SingleResultMinimal,
	toolspkg.ToolIDLoopPause:       SingleResultMinimal,
	toolspkg.ToolIDLoopRequest:     SingleResultContent,
	toolspkg.ToolIDLoopRerun:       SingleResultMinimal,
	toolspkg.ToolIDLoopRespond:     SingleResultMinimal,
	toolspkg.ToolIDLoopResume:      SingleResultMinimal,
	toolspkg.ToolIDLoopTurns:       SingleResultContent,
	toolspkg.ToolIDLoopValidate:    SingleResultMinimal,
}
