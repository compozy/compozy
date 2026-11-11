package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func withTaskResultViews(d toolspkg.Descriptor) toolspkg.Descriptor {
	var compact string
	switch d.ID {
	case toolspkg.ToolIDTaskCancel:
		compact = taskcancelCompactSchema
	case toolspkg.ToolIDTaskChildCreate:
		compact = taskchildcreateCompactSchema
	case toolspkg.ToolIDTaskCreate:
		compact = taskcreateCompactSchema
	case toolspkg.ToolIDTaskFanOutRuns:
		compact = taskfanoutrunsCompactSchema
	case toolspkg.ToolIDTaskList:
		compact = tasklistCompactSchema
	case toolspkg.ToolIDTaskRead:
		compact = taskreadCompactSchema
	case toolspkg.ToolIDTaskRecover:
		compact = taskrecoverCompactSchema
	case toolspkg.ToolIDTaskRunList:
		compact = taskrunlistCompactSchema
	case toolspkg.ToolIDTaskRunReviewList:
		compact = taskrunreviewlistCompactSchema
	case toolspkg.ToolIDTaskRunReviewRequest:
		compact = taskrunreviewrequestCompactSchema
	case toolspkg.ToolIDTaskRunReviewSubmit:
		compact = taskrunreviewsubmitCompactSchema
	case toolspkg.ToolIDTaskUpdate:
		compact = taskupdateCompactSchema
	default:
		return d
	}
	return withResultViews(d, compact, string(d.OutputSchema))
}
