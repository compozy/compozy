package daemon

import (
	"context"
	"net/http"

	apicore "github.com/compozy/compozy/internal/api/core"
)

type transportTaskService struct {
	runManager *RunManager
}

var _ apicore.TaskService = (*transportTaskService)(nil)

func newTransportTaskService(runManager *RunManager) *transportTaskService {
	return &transportTaskService{runManager: runManager}
}

func (*transportTaskService) ListWorkflows(context.Context, string) ([]apicore.WorkflowSummary, error) {
	return nil, taskTransportUnavailable("workflow listing")
}

func (*transportTaskService) GetWorkflow(context.Context, string, string) (apicore.WorkflowSummary, error) {
	return apicore.WorkflowSummary{}, taskTransportUnavailable("workflow lookup")
}

func (*transportTaskService) ListItems(context.Context, string, string) ([]apicore.TaskItem, error) {
	return nil, taskTransportUnavailable("task item listing")
}

func (*transportTaskService) Validate(context.Context, string, string) (apicore.ValidationSuccess, error) {
	return apicore.ValidationSuccess{}, taskTransportUnavailable("task validation")
}

func (s *transportTaskService) StartRun(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.TaskRunRequest,
) (apicore.Run, error) {
	if s == nil || s.runManager == nil {
		return apicore.Run{}, taskTransportUnavailable("task runs")
	}
	return s.runManager.StartTaskRun(ctx, workspaceRef, workflowSlug, req)
}

func (*transportTaskService) Archive(context.Context, string, string) (apicore.ArchiveResult, error) {
	return apicore.ArchiveResult{}, taskTransportUnavailable("task archiving")
}

func taskTransportUnavailable(action string) error {
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"task_service_unavailable",
		action+" is not available in this daemon build",
		nil,
		nil,
	)
}
