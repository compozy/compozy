package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
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

	workflowIDs := make([]string, 0, len(rows))
	for rowIndex := range rows {
		workflowIDs = append(workflowIDs, rows[rowIndex].ID)
	}
	taskCountsByWorkflowID, err := s.globalDB.TaskCountsByWorkflowIDs(ctx, workflowIDs)
	if err != nil {
		return nil, err
	}
	archiveEligibilityByWorkflowID, err := s.globalDB.WorkflowArchiveEligibilityByIDs(ctx, rows)
	if err != nil {
		return nil, err
	}

	childrenByParentID := make(map[string][]*globaldb.Workflow)
	for rowIndex := range rows {
		row := &rows[rowIndex]
		if row.ParentWorkflowID == "" {
			continue
		}
		childrenByParentID[row.ParentWorkflowID] = append(childrenByParentID[row.ParentWorkflowID], row)
	}

	workflows := make([]apicore.WorkflowSummary, 0, len(rows))
	for rowIndex := range rows {
		row := &rows[rowIndex]
		if row.ParentWorkflowID != "" {
			continue
		}
		summary, summaryErr := workflowListSummary(
			ctx,
			s.globalDB,
			row,
			taskCountsByWorkflowID,
			archiveEligibilityByWorkflowID,
			childrenByParentID[row.ID],
		)
		if summaryErr != nil {
			return nil, summaryErr
		}
		workflows = append(workflows, summary)
	}
	return workflows, nil
}

func workflowListSummary(
	ctx context.Context,
	db *globaldb.GlobalDB,
	row *globaldb.Workflow,
	taskCountsByWorkflowID map[string]globaldb.WorkflowTaskCountsRow,
	archiveEligibilityByWorkflowID map[string]globaldb.WorkflowArchiveEligibility,
	children []*globaldb.Workflow,
) (apicore.WorkflowSummary, error) {
	taskCounts := taskCountsByWorkflowID[row.ID]
	summary := transportWorkflowSummaryWithTaskCounts(*row, WorkflowTaskCounts{
		Total:     taskCounts.Total,
		Completed: taskCounts.Completed,
		Pending:   taskCounts.Pending,
	})
	if row.ArchivedAt != nil {
		archiveEligible := false
		summary.ArchiveEligible = &archiveEligible
		summary.ArchiveReason = workflowArchiveReasonArchived
	} else {
		archiveEligible, archiveReason, err := workflowArchiveAction(ctx, db, *row)
		if err != nil {
			return apicore.WorkflowSummary{}, err
		}
		summary.ArchiveEligible = &archiveEligible
		summary.ArchiveReason = archiveReason
	}
	if row.Kind != globaldb.WorkflowKindInitiative {
		return summary, nil
	}
	readinessByTaskGroupID, err := projectTaskGroupReadiness(children)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	for childIndex := range children {
		child := children[childIndex]
		childCounts := taskCountsByWorkflowID[child.ID]
		childEligibility := archiveEligibilityByWorkflowID[child.ID]
		childReview, reviewErr := latestTaskGroupReviewSummary(ctx, db, *child)
		if reviewErr != nil {
			return apicore.WorkflowSummary{}, reviewErr
		}
		summary.TaskGroups = append(summary.TaskGroups, transportTaskGroupSummary(
			*child,
			WorkflowTaskCounts{
				Total:     childCounts.Total,
				Completed: childCounts.Completed,
				Pending:   childCounts.Pending,
			},
			childEligibility,
			readinessByTaskGroupID[child.TaskGroupID],
			childReview,
		))
	}
	return summary, nil
}

// latestTaskGroupReviewSummary projects a task group's most recent review round
// into the read model so inventory cards can link to it. A task group without any
// review round contributes no identity (nil), not an error.
func latestTaskGroupReviewSummary(
	ctx context.Context,
	db *globaldb.GlobalDB,
	child globaldb.Workflow,
) (*apicore.ReviewSummary, error) {
	round, err := db.GetLatestReviewRound(ctx, child.ID)
	if err != nil {
		if errors.Is(err, globaldb.ErrReviewRoundNotFound) {
			return nil, nil
		}
		return nil, err
	}
	summary := transportReviewSummary(child.Slug, round)
	return &summary, nil
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
	if err := validateTaskGroupTransportReference(
		ctx,
		s.globalDB,
		workspaceRow.ID,
		workspaceRow.RootDir,
		workflowSlug,
	); err != nil {
		return apicore.WorkflowSummary{}, err
	}
	row, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspaceRow.ID, workflowSlug)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	taskRows, err := s.globalDB.ListTaskItems(ctx, row.ID)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	summary := transportWorkflowSummaryWithTaskCounts(row, summarizeTaskRows(taskRows))
	if err := attachWorkflowArchiveEligibility(ctx, s.globalDB, row, &summary); err != nil {
		return apicore.WorkflowSummary{}, err
	}
	return summary, nil
}

func (s *transportTaskService) WorkflowOverview(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.WorkflowOverviewPayload, error) {
	if s == nil || s.query == nil {
		return apicore.WorkflowOverviewPayload{}, taskTransportUnavailable("workflow overview")
	}
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.WorkflowOverviewPayload{}, err
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
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.TaskBoardPayload{}, err
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
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.WorkflowSpecDocument{}, err
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
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.WorkflowMemoryIndex{}, err
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
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.MarkdownDocument{}, err
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
	if err := s.validateTaskGroupReference(ctx, workspaceRef, workflowSlug); err != nil {
		return apicore.TaskDetailPayload{}, err
	}
	payload, err := s.query.TaskDetail(ctx, workspaceRef, workflowSlug, taskID)
	if err != nil {
		return apicore.TaskDetailPayload{}, mapQueryTransportError(err)
	}
	return transportTaskDetail(payload), nil
}

func (s *transportTaskService) validateTaskGroupReference(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) error {
	if s == nil || s.globalDB == nil {
		return taskTransportUnavailable("task group selection")
	}
	workspace, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return err
	}
	return validateTaskGroupTransportReference(ctx, s.globalDB, workspace.ID, workspace.RootDir, workflowSlug)
}

// validateTaskGroupTransportReference resolves a task group reference against the
// current plan before query paths touch the durable workflow catalog. This
// keeps public task group failures typed even when the catalog has not yet been
// synced for an unknown or stale selection. Durable archived rows are read-only
// history whose active plan no longer exists on disk, so once the catalog
// confirms the selection was archived it bypasses active-plan resolution; live
// selections keep typed validation against the on-disk plan.
func validateTaskGroupTransportReference(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	workspaceRoot string,
	workflowSlug string,
) error {
	if !strings.Contains(strings.TrimSpace(workflowSlug), "/") {
		return nil
	}
	ref, err := taskgroups.ParseTaskGroupRef(strings.TrimSpace(workflowSlug))
	if err != nil {
		return err
	}
	if archivedTaskGroupSelection(ctx, db, workspaceID, ref) {
		return nil
	}
	_, err = (taskgroups.TargetResolver{}).ResolveTaskGroup(ctx, workspaceRoot, ref.String())
	return err
}

// archivedTaskGroupSelection reports whether the reference resolves to a durable
// archived workflow row whose owning generation is fully archived, rather than a
// live selection or a child dropped from a recreated parent generation.
//
// A task group reference has no stable global identity; it is only meaningful
// relative to a parent generation. When an active parent initiative still
// exists, only that generation's plan is authoritative, so archived children of
// prior generations must not shadow it. In that case the caller stays on
// active-plan validation and a child absent from the recreated plan yields a
// typed task-group-not-found instead of stale archived history. Archived fallback
// is allowed only when no active parent generation exists (the whole initiative
// has been archived); a live row or unresolved lookup likewise keeps the caller
// on active-plan validation.
func archivedTaskGroupSelection(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	ref taskgroups.Ref,
) bool {
	if db == nil || strings.TrimSpace(workspaceID) == "" {
		return false
	}
	if !activeWorkflowMissing(ctx, db, workspaceID, ref.String()) {
		return false
	}
	// A task group selection is bound to its parent generation. If that parent is
	// still active, its current plan is authoritative and archived children of
	// prior generations must not shadow a task group the new plan dropped.
	if ref.IsTaskGroup() && !activeWorkflowMissing(ctx, db, workspaceID, ref.Initiative) {
		return false
	}
	_, err := db.GetLatestArchivedWorkflowBySlug(ctx, workspaceID, ref.String())
	return err == nil
}

// activeWorkflowMissing reports whether no active workflow row exists for the
// slug. Any lookup error other than ErrWorkflowNotFound is treated as "present"
// so callers conservatively stay on active-plan validation rather than serving
// durable archived history on an uncertain catalog state.
func activeWorkflowMissing(ctx context.Context, db *globaldb.GlobalDB, workspaceID, slug string) bool {
	_, err := db.GetActiveWorkflowBySlug(ctx, workspaceID, slug)
	if err == nil {
		return false
	}
	return errors.Is(err, globaldb.ErrWorkflowNotFound)
}

func (s *transportTaskService) Validate(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.ValidationSuccess, error) {
	if s == nil || s.runManager == nil {
		return apicore.ValidationSuccess{}, taskTransportUnavailable("task validation")
	}
	workspace, _, projectCfg, scope, err := s.runManager.resolveLifecycleWorkflowContext(
		ctx,
		workspaceRef,
		workflowSlug,
	)
	if err != nil {
		return apicore.ValidationSuccess{}, err
	}
	tasksDir, err := resolveTaskOperationalDirectory(workspace.RootDir, workflowSlug, scope)
	if err != nil {
		return apicore.ValidationSuccess{}, err
	}
	configuredTypes := taskscore.BuiltinTypes
	if projectCfg.Tasks.Types != nil {
		configuredTypes = *projectCfg.Tasks.Types
	}
	registry, err := taskscore.NewRegistry(configuredTypes)
	if err != nil {
		return apicore.ValidationSuccess{}, fmt.Errorf("resolve task type registry: %w", err)
	}
	report, err := taskscore.ValidateWithOptions(ctx, tasksDir, registry, taskscore.ValidateOptions{
		Recursive:        true,
		ExpectedWorkflow: strings.TrimSpace(workflowSlug),
	})
	if err != nil {
		return apicore.ValidationSuccess{}, err
	}
	if !report.OK() {
		return apicore.ValidationSuccess{}, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"task_validation_failed",
			"task validation failed",
			map[string]any{"issues": report.Issues},
			nil,
		)
	}
	return apicore.ValidationSuccess{Valid: true, CheckedAt: time.Now().UTC()}, nil
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

func (s *transportTaskService) StartRunMultiple(
	ctx context.Context,
	workspaceRef string,
	req apicore.TaskRunMultipleRequest,
) (apicore.Run, error) {
	if s == nil || s.runManager == nil {
		return apicore.Run{}, taskTransportUnavailable("multi-run task runs")
	}
	return s.runManager.StartTaskRunMultiple(ctx, workspaceRef, req)
}

func (s *transportTaskService) RunMultipleSnapshot(
	ctx context.Context,
	runID string,
) (apicore.TaskRunMultipleSnapshot, error) {
	if s == nil || s.runManager == nil {
		return apicore.TaskRunMultipleSnapshot{}, taskTransportUnavailable("multi-run task snapshots")
	}
	return s.runManager.RunMultipleSnapshot(ctx, runID)
}

func (s *transportTaskService) Archive(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.ArchiveRequest,
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
	result, err := corepkg.ArchiveWithDB(ctx, s.globalDB, workspaceRow, corepkg.ArchiveConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Name:          strings.TrimSpace(workflowSlug),
		Force:         req.Force,
	})
	if err != nil {
		var forceRequired corepkg.WorkflowArchiveForceRequiredError
		if errors.As(err, &forceRequired) {
			return apicore.ArchiveResult{}, apicore.NewProblem(
				http.StatusConflict,
				string(contract.CodeWorkflowForceRequired),
				"workflow has pending local work and requires archive confirmation",
				map[string]any{
					"workflow_slug":     strings.TrimSpace(forceRequired.Slug),
					"archive_reason":    strings.TrimSpace(forceRequired.Reason),
					"task_pending":      forceRequired.TaskNonTerminal,
					"task_non_terminal": forceRequired.TaskNonTerminal,
					"review_unresolved": forceRequired.ReviewUnresolved,
					"review_total":      forceRequired.ReviewTotal,
					"force_scope":       "local_only",
				},
				err,
			)
		}
		return apicore.ArchiveResult{}, err
	}
	return transportArchiveResult(result), nil
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
