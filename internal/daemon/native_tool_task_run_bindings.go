package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) taskRunToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTaskRunList: {
			call:         n.taskRunList,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunResult: {
			call:         n.taskRunResult,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewRequest: {
			call:         n.taskRunReviewRequest,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewList: {
			call:         n.taskRunReviewList,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewShow: {
			call:         n.taskRunReviewShow,
			availability: availability,
		},
	}
}
