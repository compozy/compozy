package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func withLoopResultViews(d toolspkg.Descriptor) toolspkg.Descriptor {
	var compact string
	switch d.ID {
	case toolspkg.ToolIDLoopConfigure:
		compact = loopconfigureCompactSchema
	case toolspkg.ToolIDLoopCreate:
		compact = loopcreateCompactSchema
	case toolspkg.ToolIDLoopFork:
		compact = loopforkCompactSchema
	case toolspkg.ToolIDLoopInspect:
		compact = loopinspectCompactSchema
	case toolspkg.ToolIDLoopList:
		compact = looplistCompactSchema
	case toolspkg.ToolIDLoopNodeAmend:
		compact = loopnodeamendCompactSchema
	case toolspkg.ToolIDLoopNodes:
		compact = loopnodesCompactSchema
	case toolspkg.ToolIDLoopRequests:
		compact = looprequestsCompactSchema
	case toolspkg.ToolIDLoopRun:
		compact = looprunCompactSchema
	case toolspkg.ToolIDLoopRuns:
		compact = looprunsCompactSchema
	case toolspkg.ToolIDLoopStatus:
		compact = loopstatusCompactSchema
	default:
		return d
	}
	return withResultViews(d, compact, string(d.OutputSchema))
}
