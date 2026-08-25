package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) loopCoreToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDGoalGet:       {call: n.goalGet, availability: n.goalGetAvailability},
		toolspkg.ToolIDGoalControl:   {call: n.goalControl, availability: n.goalControlAvailability},
		toolspkg.ToolIDGoalReport:    {call: n.goalReport, availability: n.goalReportAvailability},
		toolspkg.ToolIDLoopList:      {call: n.loopList, availability: availability},
		toolspkg.ToolIDLoopInspect:   {call: n.loopInspect, availability: availability},
		toolspkg.ToolIDLoopValidate:  {call: n.loopValidate, availability: availability},
		toolspkg.ToolIDLoopCreate:    {call: n.loopCreate, availability: availability},
		toolspkg.ToolIDLoopRun:       {call: n.loopRun, availability: availability},
		toolspkg.ToolIDLoopStatus:    {call: n.loopStatus, availability: availability},
		toolspkg.ToolIDLoopRuns:      {call: n.loopRuns, availability: availability},
		toolspkg.ToolIDLoopTurns:     {call: n.loopTurns, availability: availability},
		toolspkg.ToolIDLoopPause:     {call: n.loopPause, availability: availability},
		toolspkg.ToolIDLoopResume:    {call: n.loopResume, availability: availability},
		toolspkg.ToolIDLoopConfigure: {call: n.loopConfigure, availability: availability},
	}
}

func (n *daemonNativeTools) loopInteractionToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDLoopApprove:   {call: n.loopApprove, availability: availability},
		toolspkg.ToolIDLoopRequests:  {call: n.loopRequests, availability: availability},
		toolspkg.ToolIDLoopRequest:   {call: n.loopRequest, availability: availability},
		toolspkg.ToolIDLoopRespond:   {call: n.loopRespond, availability: availability},
		toolspkg.ToolIDLoopNodeAmend: {call: n.loopNodeAmend, availability: availability},
		toolspkg.ToolIDLoopDiff:      {call: n.loopDiff, availability: availability},
		toolspkg.ToolIDLoopRerun:     {call: n.loopRerun, availability: availability},
		toolspkg.ToolIDLoopFork:      {call: n.loopFork, availability: availability},
		toolspkg.ToolIDLoopDelete:    {call: n.loopDelete, availability: availability},
	}
}
