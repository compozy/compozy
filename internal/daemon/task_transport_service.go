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
	query      QueryService
}

var _ apicore.TaskService = (*transportTaskService)(nil)

func newTransportTaskService(
	globalDB *globaldb.GlobalDB,
	runManager *RunManager,
	query ...QueryService,
) *transportTaskService {
	return &transportTaskService{
		globalDB:   globalDB,
		runManager: runManager,
		query:      resolveTransportQueryService(globalDB, runManager, nil, query),
	}
}

func (s *transportTaskService) Dashboard(
	ctx context.Context,
	workspaceRef string,
) (apicore.DashboardPayload, error) {
	if s == nil || s.query == nil {
		return apicore.DashboardPayload{}, taskTransportUnavailable("dashboard read")
	}
	payload, err := s.query.Dashboard(ctx, workspaceRef)
	if err != nil {
		return apicore.DashboardPayload{}, mapQueryTransportError(err)
	}
	return transportDashboard(payload), nil
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
		WorkspaceID:     workspaceRow.ID,
		IncludeArchived: true,
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

func (s *transportTaskService) WorkflowOverview(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.WorkflowOverviewPayload, error) {
	if s == nil || s.query == nil {
		return apicore.WorkflowOverviewPayload{}, taskTransportUnavailable("workflow overview")
	}
	payload, err := s.query.WorkflowOverview(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.WorkflowOverviewPayload{}, mapQueryTransportError(err)
	}
	return transportWorkflowOverview(payload), nil
}

func (*transportTaskService) ListItems(context.Context, string, string) ([]apicore.TaskItem, error) {
	return nil, taskTransportUnavailable("task item listing")
}

func (s *transportTaskService) TaskBoard(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.TaskBoardPayload, error) {
	if s == nil || s.query == nil {
		return apicore.TaskBoardPayload{}, taskTransportUnavailable("task board")
	}
	payload, err := s.query.TaskBoard(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.TaskBoardPayload{}, mapQueryTransportError(err)
	}
	return transportTaskBoard(payload), nil
}

func (s *transportTaskService) WorkflowSpec(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.WorkflowSpecDocument, error) {
	if s == nil || s.query == nil {
		return apicore.WorkflowSpecDocument{}, taskTransportUnavailable("workflow spec")
	}
	payload, err := s.query.WorkflowSpec(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.WorkflowSpecDocument{}, mapQueryTransportError(err)
	}
	return transportWorkflowSpec(payload), nil
}

func (s *transportTaskService) WorkflowMemoryIndex(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.WorkflowMemoryIndex, error) {
	if s == nil || s.query == nil {
		return apicore.WorkflowMemoryIndex{}, taskTransportUnavailable("workflow memory index")
	}
	payload, err := s.query.WorkflowMemoryIndex(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.WorkflowMemoryIndex{}, mapQueryTransportError(err)
	}
	return transportWorkflowMemoryIndex(payload), nil
}

func (s *transportTaskService) WorkflowMemoryFile(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	fileID string,
) (apicore.MarkdownDocument, error) {
	if s == nil || s.query == nil {
		return apicore.MarkdownDocument{}, taskTransportUnavailable("workflow memory file")
	}
	payload, err := s.query.WorkflowMemoryFile(ctx, workspaceRef, workflowSlug, fileID)
	if err != nil {
		return apicore.MarkdownDocument{}, mapQueryTransportError(err)
	}
	return transportMarkdownDocument(payload), nil
}

func (s *transportTaskService) TaskDetail(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	taskID string,
) (apicore.TaskDetailPayload, error) {
	if s == nil || s.query == nil {
		return apicore.TaskDetailPayload{}, taskTransportUnavailable("task detail")
	}
	payload, err := s.query.TaskDetail(ctx, workspaceRef, workflowSlug, taskID)
	if err != nil {
		return apicore.TaskDetailPayload{}, mapQueryTransportError(err)
	}
	return transportTaskDetail(payload), nil
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
	if err := requireWorkspacePathAvailable(workspaceRow); err != nil {
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
