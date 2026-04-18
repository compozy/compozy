package daemon

import (
	"context"
	"net/http"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type transportTaskService struct {
	globalDB   *globaldb.GlobalDB
	runManager *RunManager
}

var _ apicore.TaskService = (*transportTaskService)(nil)

func newTransportTaskService(globalDB *globaldb.GlobalDB, runManager *RunManager) *transportTaskService {
	return &transportTaskService{
		globalDB:   globalDB,
		runManager: runManager,
	}
}

func (s *transportTaskService) ListWorkflows(
	ctx context.Context,
	workspaceRef string,
) ([]apicore.WorkflowSummary, error) {
	if s == nil || s.globalDB == nil {
		return nil, taskTransportUnavailable("workflow listing")
	}

	workspaceRow, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.globalDB.ListWorkflows(ctx, globaldb.ListWorkflowsOptions{
		WorkspaceID: workspaceRow.ID,
	})
	if err != nil {
		return nil, err
	}

	workflows := make([]apicore.WorkflowSummary, 0, len(rows))
	for _, row := range rows {
		workflows = append(workflows, transportWorkflowSummary(row))
	}
	return workflows, nil
}

func (s *transportTaskService) GetWorkflow(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.WorkflowSummary, error) {
	if s == nil || s.globalDB == nil {
		return apicore.WorkflowSummary{}, taskTransportUnavailable("workflow lookup")
	}

	workspaceRow, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	row, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspaceRow.ID, workflowSlug)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	return transportWorkflowSummary(row), nil
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

func (s *transportTaskService) Archive(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.ArchiveResult, error) {
	if s == nil || s.globalDB == nil {
		return apicore.ArchiveResult{}, taskTransportUnavailable("task archiving")
	}

	workspaceRow, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return apicore.ArchiveResult{}, err
	}
	result, err := corepkg.ArchiveDirect(ctx, corepkg.ArchiveConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Name:          strings.TrimSpace(workflowSlug),
	})
	if err != nil {
		return apicore.ArchiveResult{}, err
	}
	return apicore.ArchiveResult{Archived: result != nil && result.Archived > 0}, nil
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
