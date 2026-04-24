package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

const dashboardRunLimit = 500
const runStatusPending = "pending"
const runStatusRetrying = "retrying"

type daemonStatusReader interface {
	Status(context.Context) (apicore.DaemonStatus, error)
	Health(context.Context) (apicore.DaemonHealth, error)
}

// QueryServiceConfig wires the daemon read-model service.
type QueryServiceConfig struct {
	GlobalDB   *globaldb.GlobalDB
	RunManager *RunManager
	Daemon     daemonStatusReader
}

type queryService struct {
	globalDB   *globaldb.GlobalDB
	runManager *RunManager
	daemon     daemonStatusReader
	documents  *documentReader
}

type memoryDocumentRef struct {
	entry   WorkflowMemoryEntry
	absPath string
}

var _ QueryService = (*queryService)(nil)

// NewQueryService constructs the daemon-side read-model query layer.
func NewQueryService(cfg QueryServiceConfig) QueryService {
	return &queryService{
		globalDB:   cfg.GlobalDB,
		runManager: cfg.RunManager,
		daemon:     cfg.Daemon,
		documents:  newDocumentReader(),
	}
}

func (s *queryService) Dashboard(ctx context.Context, workspaceRef string) (WorkspaceDashboard, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceRef)
	if err != nil {
		return WorkspaceDashboard{}, err
	}
	if err := s.requireGlobalDB(); err != nil {
		return WorkspaceDashboard{}, err
	}

	workflows, err := s.globalDB.ListWorkflows(ctx, globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID})
	if err != nil {
		return WorkspaceDashboard{}, err
	}
	runs, err := s.listRuns(ctx, workspace.ID, "", dashboardRunLimit)
	if err != nil {
		return WorkspaceDashboard{}, err
	}

	cards := make([]WorkflowCard, 0, len(workflows))
	pendingReviews := 0
	for _, workflow := range workflows {
		card, err := s.buildWorkflowCard(ctx, workflow, runs)
		if err != nil {
			return WorkspaceDashboard{}, err
		}
		if card.LatestReview != nil {
			pendingReviews += card.LatestReview.UnresolvedCount
		}
		cards = append(cards, card)
	}

	status, health, err := s.readDaemonState(ctx)
	if err != nil {
		return WorkspaceDashboard{}, err
	}

	activeRuns := filterRuns(runs, func(run apicore.Run) bool {
		return !isTerminalRunStatus(run.Status)
	})
	return WorkspaceDashboard{
		Workspace:      transportWorkspace(workspace),
		Daemon:         status,
		Health:         health,
		Queue:          summarizeRunQueue(runs),
		Workflows:      cards,
		ActiveRuns:     activeRuns,
		PendingReviews: pendingReviews,
	}, nil
}

func (s *queryService) WorkflowOverview(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (WorkflowOverviewPayload, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowOverviewPayload{}, err
	}

	taskItems, counts, err := s.taskCardsForWorkflow(ctx, workflow.ID)
	if err != nil {
		return WorkflowOverviewPayload{}, err
	}
	recentRuns, err := s.listRelatedRuns(ctx, workspace.ID, workflow.Slug, "", dashboardRunLimit)
	if err != nil {
		return WorkflowOverviewPayload{}, err
	}

	var latestReview *apicore.ReviewSummary
	if summary, ok, err := s.latestReviewSummary(ctx, workflow); err != nil {
		return WorkflowOverviewPayload{}, err
	} else if ok {
		latestReview = &summary
	}

	eligibility, err := s.globalDB.GetWorkflowArchiveEligibility(ctx, workspace.ID, workflow.Slug)
	if err != nil {
		return WorkflowOverviewPayload{}, err
	}

	_ = taskItems
	return WorkflowOverviewPayload{
		Workspace:       transportWorkspace(workspace),
		Workflow:        transportWorkflowSummary(workflow),
		TaskCounts:      counts,
		LatestReview:    latestReview,
		RecentRuns:      recentRuns,
		ArchiveEligible: eligibility.Archivable(),
		ArchiveReason:   eligibility.SkipReason(),
	}, nil
}

func (s *queryService) TaskBoard(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (TaskBoardPayload, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return TaskBoardPayload{}, err
	}

	cards, counts, err := s.taskCardsForWorkflow(ctx, workflow.ID)
	if err != nil {
		return TaskBoardPayload{}, err
	}

	return TaskBoardPayload{
		Workspace:  transportWorkspace(workspace),
		Workflow:   transportWorkflowSummary(workflow),
		TaskCounts: counts,
		Lanes:      buildTaskLanes(cards),
	}, nil
}

func (s *queryService) WorkflowSpec(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (WorkflowSpecDocument, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowSpecDocument{}, err
	}

	workflowRoot := workflowRootDir(workspace.RootDir, workflow.Slug)
	spec := WorkflowSpecDocument{
		Workspace: transportWorkspace(workspace),
		Workflow:  transportWorkflowSummary(workflow),
	}

	if prdDoc, ok, err := s.readWorkflowDocument(ctx, workflowRoot, "_prd.md", "prd", "prd"); err != nil {
		return WorkflowSpecDocument{}, err
	} else if ok {
		spec.PRD = &prdDoc
	}
	if techspecDoc, ok, err := s.readWorkflowDocument(
		ctx,
		workflowRoot,
		"_techspec.md",
		"techspec",
		"techspec",
	); err != nil {
		return WorkflowSpecDocument{}, err
	} else if ok {
		spec.TechSpec = &techspecDoc
	}

	adrsDir := filepath.Join(workflowRoot, "adrs")
	entries, err := readMarkdownDir(adrsDir)
	if err != nil {
		if !errors.Is(err, ErrDocumentMissing) {
			return WorkflowSpecDocument{}, err
		}
		return spec, nil
	}
	for _, entry := range entries {
		doc, err := s.documents.Read(
			ctx,
			entry.absPath,
			"adr",
			strings.TrimSuffix(filepath.Base(entry.displayPath), filepath.Ext(entry.displayPath)),
		)
		if err != nil {
			return WorkflowSpecDocument{}, err
		}
		spec.ADRs = append(spec.ADRs, doc)
	}
	return spec, nil
}

func (s *queryService) WorkflowMemoryIndex(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (WorkflowMemoryIndex, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowMemoryIndex{}, err
	}

	entries, err := s.listMemoryDocuments(ctx, workspace, workflow)
	if err != nil {
		return WorkflowMemoryIndex{}, err
	}

	index := WorkflowMemoryIndex{
		Workspace: transportWorkspace(workspace),
		Workflow:  transportWorkflowSummary(workflow),
		Entries:   make([]WorkflowMemoryEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		index.Entries = append(index.Entries, entry.entry)
	}
	return index, nil
}

func (s *queryService) WorkflowMemoryFile(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	fileID string,
) (MarkdownDocument, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return MarkdownDocument{}, err
	}

	entries, err := s.listMemoryDocuments(ctx, workspace, workflow)
	if err != nil {
		return MarkdownDocument{}, err
	}

	trimmedID := strings.TrimSpace(fileID)
	for _, entry := range entries {
		if entry.entry.FileID != trimmedID {
			continue
		}
		return s.documents.Read(ctx, entry.absPath, "memory", entry.entry.FileID)
	}
	return MarkdownDocument{}, StaleDocumentReferenceError{
		Kind:         "memory",
		WorkflowSlug: workflow.Slug,
		Reference:    trimmedID,
	}
}

func (s *queryService) TaskDetail(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	taskID string,
) (TaskDetailPayload, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	taskRow, err := s.globalDB.GetTaskItemByTaskID(ctx, workflow.ID, taskID)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	document, err := s.readRequiredWorkflowDocument(
		ctx,
		workflowRootDir(workspace.RootDir, workflow.Slug),
		workflow.Slug,
		taskRow.SourcePath,
		runModeTask,
		taskRow.TaskID,
	)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	memoryEntries, err := s.listMemoryDocuments(ctx, workspace, workflow)
	if err != nil {
		return TaskDetailPayload{}, err
	}
	relevantMemory := make([]WorkflowMemoryEntry, 0, len(memoryEntries))
	for _, entry := range memoryEntries {
		if memoryEntryMatchesTask(entry.entry, taskRow.TaskNumber) {
			relevantMemory = append(relevantMemory, entry.entry)
		}
	}

	relatedRuns, err := s.listRelatedRuns(ctx, workspace.ID, workflow.Slug, runModeTask, dashboardRunLimit)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	return TaskDetailPayload{
		Workspace:         transportWorkspace(workspace),
		Workflow:          transportWorkflowSummary(workflow),
		Task:              taskCardFromRow(taskRow),
		Document:          document,
		MemoryEntries:     relevantMemory,
		RelatedRuns:       relatedRuns,
		LiveTailAvailable: anyLiveRuns(relatedRuns),
	}, nil
}

func (s *queryService) ReviewDetail(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
	issueRef string,
) (ReviewDetailPayload, error) {
	workspace, workflow, err := s.resolveWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return ReviewDetailPayload{}, err
	}

	roundRow, err := s.globalDB.GetReviewRound(ctx, workflow.ID, round)
	if err != nil {
		return ReviewDetailPayload{}, err
	}
	issueRow, err := s.resolveReviewIssue(ctx, roundRow, workflow.Slug, issueRef)
	if err != nil {
		return ReviewDetailPayload{}, err
	}

	document, err := s.readRequiredWorkflowDocument(
		ctx,
		workflowRootDir(workspace.RootDir, workflow.Slug),
		workflow.Slug,
		issueRow.SourcePath,
		"review_issue",
		issueRow.ID,
	)
	if err != nil {
		return ReviewDetailPayload{}, err
	}

	relatedRuns, err := s.listRelatedRuns(ctx, workspace.ID, workflow.Slug, runModeReview, dashboardRunLimit)
	if err != nil {
		return ReviewDetailPayload{}, err
	}

	return ReviewDetailPayload{
		Workspace: transportWorkspace(workspace),
		Workflow:  transportWorkflowSummary(workflow),
		Round:     transportReviewRound(workflow.Slug, roundRow),
		Issue: ReviewIssueDetail{
			ID:          issueRow.ID,
			IssueNumber: issueRow.IssueNumber,
			Severity:    issueRow.Severity,
			Status:      issueRow.Status,
			UpdatedAt:   issueRow.UpdatedAt,
		},
		Document:    document,
		RelatedRuns: relatedRuns,
	}, nil
}

func (s *queryService) RunDetail(ctx context.Context, runID string) (RunDetailPayload, error) {
	if err := s.requireRunManager(); err != nil {
		return RunDetailPayload{}, err
	}

	run, err := s.runManager.Get(ctx, runID)
	if err != nil {
		return RunDetailPayload{}, err
	}
	snapshot, err := s.runManager.Snapshot(ctx, runID)
	if err != nil {
		return RunDetailPayload{}, err
	}

	lease, err := s.runManager.acquireRunDB(ctx, run.RunID)
	if err != nil {
		return RunDetailPayload{}, err
	}
	defer func() {
		_ = lease.Close()
	}()

	timeline, err := lease.DB().ListEvents(ctx, 0, 0)
	if err != nil {
		return RunDetailPayload{}, err
	}
	artifactSync, err := lease.DB().ListArtifactSyncLog(ctx)
	if err != nil {
		return RunDetailPayload{}, err
	}

	return RunDetailPayload{
		Run:          run,
		Snapshot:     snapshot,
		JobCounts:    summarizeRunJobCounts(snapshot),
		Runtime:      summarizeRunRuntime(snapshot),
		Timeline:     append([]eventspkg.Event(nil), timeline.Events...),
		ArtifactSync: append([]rundb.ArtifactSyncRow(nil), artifactSync...),
	}, nil
}

func (s *queryService) resolveWorkspace(ctx context.Context, workspaceRef string) (globaldb.Workspace, error) {
	if err := s.requireGlobalDB(); err != nil {
		return globaldb.Workspace{}, err
	}
	return resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
}

func (s *queryService) resolveWorkflow(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (globaldb.Workspace, globaldb.Workflow, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceRef)
	if err != nil {
		return globaldb.Workspace{}, globaldb.Workflow{}, err
	}
	workflow, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspace.ID, strings.TrimSpace(workflowSlug))
	if err != nil {
		return globaldb.Workspace{}, globaldb.Workflow{}, err
	}
	return workspace, workflow, nil
}

func (s *queryService) buildWorkflowCard(
	ctx context.Context,
	workflow globaldb.Workflow,
	runs []apicore.Run,
) (WorkflowCard, error) {
	taskRows, err := s.globalDB.ListTaskItems(ctx, workflow.ID)
	if err != nil {
		return WorkflowCard{}, err
	}
	taskCounts := summarizeTaskRows(taskRows)

	var latestReview *apicore.ReviewSummary
	rounds, err := s.globalDB.ListReviewRounds(ctx, workflow.ID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if len(rounds) > 0 {
		summary := transportReviewSummary(workflow.Slug, rounds[len(rounds)-1])
		latestReview = &summary
	}

	activeRuns := 0
	for i := range runs {
		run := &runs[i]
		if run.WorkflowSlug == workflow.Slug && !isTerminalRunStatus(run.Status) {
			activeRuns++
		}
	}

	return WorkflowCard{
		Workflow:         transportWorkflowSummary(workflow),
		TaskTotal:        taskCounts.Total,
		TaskCompleted:    taskCounts.Completed,
		TaskPending:      taskCounts.Pending,
		LatestReview:     latestReview,
		ReviewRoundCount: len(rounds),
		ActiveRuns:       activeRuns,
	}, nil
}

func (s *queryService) taskCardsForWorkflow(
	ctx context.Context,
	workflowID string,
) ([]TaskCard, WorkflowTaskCounts, error) {
	taskRows, err := s.globalDB.ListTaskItems(ctx, workflowID)
	if err != nil {
		return nil, WorkflowTaskCounts{}, err
	}
	cards := make([]TaskCard, 0, len(taskRows))
	for i := range taskRows {
		cards = append(cards, taskCardFromRow(taskRows[i]))
	}
	return cards, summarizeTaskRows(taskRows), nil
}

func (s *queryService) latestReviewSummary(
	ctx context.Context,
	workflow globaldb.Workflow,
) (apicore.ReviewSummary, bool, error) {
	latest, err := s.globalDB.GetLatestReviewRound(ctx, workflow.ID)
	if err == nil {
		return transportReviewSummary(workflow.Slug, latest), true, nil
	}
	if errors.Is(err, globaldb.ErrReviewRoundNotFound) {
		return apicore.ReviewSummary{}, false, nil
	}
	return apicore.ReviewSummary{}, false, err
}

func (s *queryService) resolveReviewIssue(
	ctx context.Context,
	round globaldb.ReviewRound,
	workflowSlug string,
	issueRef string,
) (globaldb.ReviewIssue, error) {
	issues, err := s.globalDB.ListReviewIssues(ctx, round.ID)
	if err != nil {
		return globaldb.ReviewIssue{}, err
	}

	trimmedRef := strings.TrimSpace(issueRef)
	issueNumber, hasNumber := parseIssueRef(trimmedRef)
	for _, issue := range issues {
		switch {
		case issue.ID == trimmedRef:
			return issue, nil
		case hasNumber && issue.IssueNumber == issueNumber:
			return issue, nil
		}
	}
	return globaldb.ReviewIssue{}, ReviewIssueNotFoundError{
		WorkflowSlug: workflowSlug,
		Round:        round.RoundNumber,
		IssueRef:     trimmedRef,
	}
}

func (s *queryService) listRuns(
	ctx context.Context,
	workspaceID string,
	mode string,
	limit int,
) ([]apicore.Run, error) {
	if err := s.requireRunManager(); err != nil {
		return nil, err
	}
	return s.runManager.List(ctx, apicore.RunListQuery{
		Workspace: workspaceID,
		Mode:      strings.TrimSpace(mode),
		Limit:     limit,
	})
}

func (s *queryService) listRelatedRuns(
	ctx context.Context,
	workspaceID string,
	workflowSlug string,
	mode string,
	limit int,
) ([]apicore.Run, error) {
	runs, err := s.listRuns(ctx, workspaceID, mode, limit)
	if err != nil {
		return nil, err
	}
	return filterRuns(runs, func(run apicore.Run) bool {
		return strings.EqualFold(strings.TrimSpace(run.WorkflowSlug), strings.TrimSpace(workflowSlug))
	}), nil
}

func (s *queryService) listMemoryDocuments(
	ctx context.Context,
	workspace globaldb.Workspace,
	workflow globaldb.Workflow,
) ([]memoryDocumentRef, error) {
	memoryDir := filepath.Join(workflowRootDir(workspace.RootDir, workflow.Slug), "memory")
	entries, err := readMarkdownDir(memoryDir)
	if err != nil {
		if errors.Is(err, ErrDocumentMissing) {
			return nil, nil
		}
		return nil, err
	}

	refs := make([]memoryDocumentRef, 0, len(entries))
	for _, entry := range entries {
		displayPath := filepath.ToSlash(filepath.Join("memory", entry.displayPath))
		fileID := memoryFileID(workspace.ID, workflow.Slug, displayPath)
		doc, err := s.documents.Read(ctx, entry.absPath, "memory", fileID)
		if err != nil {
			return nil, err
		}
		refs = append(refs, memoryDocumentRef{
			entry: WorkflowMemoryEntry{
				FileID:      fileID,
				DisplayPath: displayPath,
				Kind:        classifyMemoryEntry(displayPath),
				Title:       doc.Title,
				SizeBytes:   entry.sizeBytes,
				UpdatedAt:   entry.updatedAt,
			},
			absPath: entry.absPath,
		})
	}
	return refs, nil
}

func (s *queryService) readWorkflowDocument(
	ctx context.Context,
	workflowRoot string,
	relativePath string,
	kind string,
	id string,
) (MarkdownDocument, bool, error) {
	absPath := filepath.Join(workflowRoot, filepath.FromSlash(relativePath))
	if err := fileInfo(absPath); err != nil {
		if errors.Is(err, ErrDocumentMissing) {
			return MarkdownDocument{}, false, nil
		}
		return MarkdownDocument{}, false, err
	}
	doc, err := s.documents.Read(ctx, absPath, kind, id)
	if err != nil {
		return MarkdownDocument{}, false, err
	}
	return doc, true, nil
}

func (s *queryService) readRequiredWorkflowDocument(
	ctx context.Context,
	workflowRoot string,
	workflowSlug string,
	relativePath string,
	kind string,
	id string,
) (MarkdownDocument, error) {
	absPath := filepath.Join(workflowRoot, filepath.FromSlash(relativePath))
	doc, err := s.documents.Read(ctx, absPath, kind, id)
	if err != nil {
		if errors.Is(err, ErrDocumentMissing) {
			return MarkdownDocument{}, DocumentMissingError{
				Kind:         kind,
				WorkflowSlug: workflowSlug,
				RelativePath: filepath.ToSlash(relativePath),
			}
		}
		return MarkdownDocument{}, err
	}
	return doc, nil
}

func (s *queryService) readDaemonState(
	ctx context.Context,
) (apicore.DaemonStatus, apicore.DaemonHealth, error) {
	if s.daemon == nil {
		return apicore.DaemonStatus{}, apicore.DaemonHealth{}, nil
	}
	status, err := s.daemon.Status(ctx)
	if err != nil {
		return apicore.DaemonStatus{}, apicore.DaemonHealth{}, err
	}
	health, err := s.daemon.Health(ctx)
	if err != nil {
		return apicore.DaemonStatus{}, apicore.DaemonHealth{}, err
	}
	return status, health, nil
}

func (s *queryService) requireGlobalDB() error {
	if s == nil || s.globalDB == nil {
		return errors.New("daemon: query service global database is required")
	}
	return nil
}

func (s *queryService) requireRunManager() error {
	if s == nil || s.runManager == nil {
		return errors.New("daemon: query service run manager is required")
	}
	return nil
}

func workflowRootDir(workspaceRoot string, workflowSlug string) string {
	return model.TaskDirectoryForWorkspace(workspaceRoot, workflowSlug)
}

func taskCardFromRow(row globaldb.TaskItemRow) TaskCard {
	return TaskCard{
		TaskNumber: row.TaskNumber,
		TaskID:     row.TaskID,
		Title:      row.Title,
		Status:     row.Status,
		Type:       row.Kind,
		DependsOn:  append([]string(nil), row.DependsOn...),
		UpdatedAt:  row.UpdatedAt,
	}
}

func summarizeTaskRows(rows []globaldb.TaskItemRow) WorkflowTaskCounts {
	counts := WorkflowTaskCounts{Total: len(rows)}
	for i := range rows {
		row := &rows[i]
		if taskscore.IsTaskCompleted(model.TaskEntry{Status: row.Status}) {
			counts.Completed++
			continue
		}
		counts.Pending++
	}
	return counts
}

func buildTaskLanes(cards []TaskCard) []TaskLane {
	grouped := make(map[string][]TaskCard)
	for _, card := range cards {
		status := normalizeLaneStatus(card.Status)
		grouped[status] = append(grouped[status], card)
	}

	orderedStatuses := make([]string, 0, len(grouped))
	seen := make(map[string]struct{}, len(grouped))
	for _, status := range []string{runStatusPending, "running", "retrying", runStatusCompleted, "failed", "canceled"} {
		if _, ok := grouped[status]; ok {
			orderedStatuses = append(orderedStatuses, status)
			seen[status] = struct{}{}
		}
	}
	extra := make([]string, 0, len(grouped))
	for status := range grouped {
		if _, ok := seen[status]; ok {
			continue
		}
		extra = append(extra, status)
	}
	sort.Strings(extra)
	orderedStatuses = append(orderedStatuses, extra...)

	lanes := make([]TaskLane, 0, len(orderedStatuses))
	for _, status := range orderedStatuses {
		items := append([]TaskCard(nil), grouped[status]...)
		lanes = append(lanes, TaskLane{
			Status: status,
			Title:  laneTitle(status),
			Items:  items,
		})
	}
	return lanes
}

func normalizeLaneStatus(status string) string {
	trimmed := strings.ToLower(strings.TrimSpace(status))
	if trimmed == "" {
		return runStatusPending
	}
	if trimmed == "canceled" {
		return runStatusCancelled
	}
	return trimmed
}

func laneTitle(status string) string {
	switch normalizeLaneStatus(status) {
	case runStatusPending:
		return "Pending"
	case "running", "in_progress", "in-progress":
		return "In Progress"
	case runStatusRetrying:
		return "Retrying"
	case runStatusCompleted, "done", "finished":
		return "Completed"
	case "failed":
		return "Failed"
	case runStatusCancelled:
		return "Canceled"
	default:
		return titleCase(strings.ReplaceAll(status, "_", " "))
	}
}

func summarizeRunQueue(runs []apicore.Run) DashboardQueueSummary {
	summary := DashboardQueueSummary{Total: len(runs)}
	for i := range runs {
		run := &runs[i]
		switch normalizeRunState(run.Status) {
		case runStatusCompleted:
			summary.Completed++
		case runStatusFailed, runStatusCrashed:
			summary.Failed++
		case runStatusCancelled:
			summary.Canceled++
		default:
			summary.Active++
		}
	}
	return summary
}

func filterRuns(runs []apicore.Run, keep func(apicore.Run) bool) []apicore.Run {
	if len(runs) == 0 {
		return nil
	}
	filtered := make([]apicore.Run, 0, len(runs))
	for i := range runs {
		run := runs[i]
		if keep != nil && !keep(run) {
			continue
		}
		filtered = append(filtered, run)
	}
	return filtered
}

func anyLiveRuns(runs []apicore.Run) bool {
	for i := range runs {
		run := &runs[i]
		if !isTerminalRunStatus(run.Status) {
			return true
		}
	}
	return false
}

func memoryEntryMatchesTask(entry WorkflowMemoryEntry, taskNumber int) bool {
	if taskNumber <= 0 {
		return false
	}
	return taskscore.ExtractTaskNumber(filepath.Base(entry.DisplayPath)) == taskNumber
}

func classifyMemoryEntry(displayPath string) string {
	base := filepath.Base(filepath.ToSlash(strings.TrimSpace(displayPath)))
	switch {
	case strings.EqualFold(base, "MEMORY.md"):
		return "workflow"
	case taskscore.ExtractTaskNumber(base) > 0:
		return runModeTask
	default:
		return "memory"
	}
}

func parseIssueRef(ref string) (int, bool) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return 0, false
	}
	base := strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed))
	base = strings.TrimPrefix(strings.ToLower(base), "issue_")
	value, err := strconv.Atoi(base)
	if err != nil {
		return 0, false
	}
	return value, true
}

func summarizeRunJobCounts(snapshot apicore.RunSnapshot) RunJobCounts {
	var counts RunJobCounts
	for _, job := range snapshot.Jobs {
		switch normalizeRunState(job.Status) {
		case snapshotJobStatusQueued:
			counts.Queued++
		case runStatusRunning:
			counts.Running++
		case runStatusRetrying:
			counts.Retrying++
		case runStatusCompleted:
			counts.Completed++
		case runStatusFailed:
			counts.Failed++
		case runStatusCancelled:
			counts.Canceled++
		}
	}
	return counts
}

func summarizeRunRuntime(snapshot apicore.RunSnapshot) RunRuntimeSummary {
	var summary RunRuntimeSummary
	ideSet := make(map[string]struct{})
	modelSet := make(map[string]struct{})
	reasoningSet := make(map[string]struct{})
	accessSet := make(map[string]struct{})
	presentationSet := make(map[string]struct{})

	appendUnique := func(values *[]string, seen map[string]struct{}, raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		*values = append(*values, trimmed)
	}

	appendUnique(&summary.PresentationModes, presentationSet, snapshot.Run.PresentationMode)
	for _, job := range snapshot.Jobs {
		if job.Summary == nil {
			continue
		}
		appendUnique(&summary.IDEs, ideSet, job.Summary.IDE)
		appendUnique(&summary.Models, modelSet, job.Summary.Model)
		appendUnique(&summary.ReasoningEfforts, reasoningSet, job.Summary.ReasoningEffort)
		appendUnique(&summary.AccessModes, accessSet, job.Summary.AccessMode)
	}
	return summary
}

func normalizeRunState(status string) string {
	trimmed := strings.ToLower(strings.TrimSpace(status))
	if trimmed == "canceled" {
		return runStatusCancelled
	}
	return trimmed
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}
