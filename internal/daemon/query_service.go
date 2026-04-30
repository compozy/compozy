package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	entry    WorkflowMemoryEntry
	absPath  string
	snapshot *globaldb.ArtifactSnapshotRow
}

type workflowReadTarget struct {
	workspace       globaldb.Workspace
	workflow        globaldb.Workflow
	rootDir         string
	snapshotsByPath map[string]globaldb.ArtifactSnapshotRow
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
	visibleRuns := dashboardVisibleRuns(runs)

	cards := make([]WorkflowCard, 0, len(workflows))
	pendingReviews := 0
	for _, workflow := range workflows {
		card, err := s.buildWorkflowCard(ctx, workflow, visibleRuns)
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

	activeRuns := filterRuns(visibleRuns, func(run apicore.Run) bool {
		return !isTerminalRunStatus(run.Status)
	})
	return WorkspaceDashboard{
		Workspace:      transportWorkspace(workspace),
		Daemon:         status,
		Health:         health,
		Queue:          summarizeRunQueue(visibleRuns),
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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowOverviewPayload{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

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

	archiveReason := workflowArchiveReasonArchived
	archiveEligible := false
	if workflow.ArchivedAt == nil {
		eligibility, eligibilityErr := s.globalDB.GetWorkflowArchiveEligibility(ctx, workspace.ID, workflow.Slug)
		if eligibilityErr != nil {
			return WorkflowOverviewPayload{}, eligibilityErr
		}
		archiveEligible = eligibility.Archivable()
		archiveReason = eligibility.SkipReason()
	}

	_ = taskItems
	summary := transportWorkflowSummaryWithTaskCounts(workflow, counts)
	summary.ArchiveEligible = &archiveEligible
	summary.ArchiveReason = archiveReason

	return WorkflowOverviewPayload{
		Workspace:       transportWorkspace(workspace),
		Workflow:        summary,
		TaskCounts:      counts,
		LatestReview:    latestReview,
		RecentRuns:      recentRuns,
		ArchiveEligible: archiveEligible,
		ArchiveReason:   archiveReason,
	}, nil
}

func (s *queryService) TaskBoard(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (TaskBoardPayload, error) {
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return TaskBoardPayload{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowSpecDocument{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

	spec := WorkflowSpecDocument{
		Workspace: transportWorkspace(workspace),
		Workflow:  transportWorkflowSummary(workflow),
	}

	if prdDoc, ok, err := s.readWorkflowDocument(
		ctx,
		target,
		"_prd.md",
		markdownDocumentKindPRD,
		markdownDocumentKindPRD,
	); err != nil {
		return WorkflowSpecDocument{}, err
	} else if ok {
		spec.PRD = &prdDoc
	}
	if techspecDoc, ok, err := s.readWorkflowDocument(
		ctx,
		target,
		"_techspec.md",
		markdownDocumentKindTechSpec,
		markdownDocumentKindTechSpec,
	); err != nil {
		return WorkflowSpecDocument{}, err
	} else if ok {
		spec.TechSpec = &techspecDoc
	}

	if adrs, ok, err := snapshotDocumentsByPrefix(
		target.snapshotsByPath,
		"adrs/",
		markdownDocumentKindADR,
		func(relativePath string) string {
			return strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath))
		},
	); err != nil {
		return WorkflowSpecDocument{}, err
	} else if ok {
		spec.ADRs = adrs
		return spec, nil
	}

	adrsDir := filepath.Join(target.rootDir, "adrs")
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
			markdownDocumentKindADR,
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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return WorkflowMemoryIndex{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

	entries, err := s.listMemoryDocuments(ctx, workspace, workflow, target)
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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return MarkdownDocument{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

	entries, err := s.listMemoryDocuments(ctx, workspace, workflow, target)
	if err != nil {
		return MarkdownDocument{}, err
	}

	trimmedID := strings.TrimSpace(fileID)
	for _, entry := range entries {
		if entry.entry.FileID != trimmedID {
			continue
		}
		if entry.snapshot != nil {
			return markdownDocumentFromSnapshot(*entry.snapshot, markdownDocumentKindMemory, entry.entry.FileID)
		}
		return s.documents.Read(ctx, entry.absPath, markdownDocumentKindMemory, entry.entry.FileID)
	}
	return MarkdownDocument{}, StaleDocumentReferenceError{
		Kind:         markdownDocumentKindMemory,
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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return TaskDetailPayload{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

	taskRow, err := s.globalDB.GetTaskItemByTaskID(ctx, workflow.ID, taskID)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	document, err := s.readRequiredWorkflowDocument(
		ctx,
		target,
		taskRow.SourcePath,
		markdownDocumentKindTask,
		taskRow.TaskID,
	)
	if err != nil {
		return TaskDetailPayload{}, err
	}

	memoryEntries, err := s.listMemoryDocuments(ctx, workspace, workflow, target)
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
	target, err := s.resolveWorkflowReadTarget(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return ReviewDetailPayload{}, err
	}
	workspace := target.workspace
	workflow := target.workflow

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
		target,
		issueRow.SourcePath,
		markdownDocumentKindReviewIssue,
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

func (s *queryService) resolveWorkflowReadTarget(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (workflowReadTarget, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceRef)
	if err != nil {
		return workflowReadTarget{}, err
	}
	slug := strings.TrimSpace(workflowSlug)

	workflow, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspace.ID, slug)
	if err == nil {
		rootDir, rootErr := readableWorkflowRootDir(workspace.RootDir, workflow)
		if rootErr != nil {
			return workflowReadTarget{}, rootErr
		}
		snapshots, snapshotErr := s.snapshotIndex(ctx, workflow.ID)
		if snapshotErr != nil {
			return workflowReadTarget{}, snapshotErr
		}
		return workflowReadTarget{
			workspace:       workspace,
			workflow:        workflow,
			rootDir:         rootDir,
			snapshotsByPath: snapshots,
		}, nil
	}
	if !errors.Is(err, globaldb.ErrWorkflowNotFound) {
		return workflowReadTarget{}, err
	}

	workflow, err = s.globalDB.GetLatestArchivedWorkflowBySlug(ctx, workspace.ID, slug)
	if err != nil {
		return workflowReadTarget{}, err
	}
	rootDir, err := readableWorkflowRootDir(workspace.RootDir, workflow)
	if err != nil {
		return workflowReadTarget{}, err
	}
	snapshots, err := s.snapshotIndex(ctx, workflow.ID)
	if err != nil {
		return workflowReadTarget{}, err
	}
	return workflowReadTarget{
		workspace:       workspace,
		workflow:        workflow,
		rootDir:         rootDir,
		snapshotsByPath: snapshots,
	}, nil
}

func (s *queryService) snapshotIndex(
	ctx context.Context,
	workflowID string,
) (map[string]globaldb.ArtifactSnapshotRow, error) {
	snapshots, err := s.globalDB.ListArtifactSnapshots(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	index := make(map[string]globaldb.ArtifactSnapshotRow, len(snapshots))
	for idx := range snapshots {
		snapshot := &snapshots[idx]
		index[filepath.ToSlash(strings.TrimSpace(snapshot.RelativePath))] = *snapshot
	}
	return index, nil
}

func snapshotDocument(
	snapshots map[string]globaldb.ArtifactSnapshotRow,
	relativePath string,
	kind string,
	id string,
) (MarkdownDocument, bool, error) {
	if len(snapshots) == 0 {
		return MarkdownDocument{}, false, nil
	}
	snapshot, ok := snapshots[filepath.ToSlash(strings.TrimSpace(relativePath))]
	if !ok {
		return MarkdownDocument{}, false, nil
	}
	doc, err := markdownDocumentFromSnapshot(snapshot, kind, id)
	if err != nil {
		return MarkdownDocument{}, false, err
	}
	return doc, true, nil
}

func snapshotDocumentsByPrefix(
	snapshots map[string]globaldb.ArtifactSnapshotRow,
	prefix string,
	kind string,
	idForPath func(string) string,
) ([]MarkdownDocument, bool, error) {
	if len(snapshots) == 0 {
		return nil, false, nil
	}

	prefix = filepath.ToSlash(strings.TrimSpace(prefix))
	paths := make([]string, 0)
	for relativePath := range snapshots {
		if strings.HasPrefix(relativePath, prefix) {
			paths = append(paths, relativePath)
		}
	}
	if len(paths) == 0 {
		return nil, false, nil
	}
	sort.Strings(paths)

	docs := make([]MarkdownDocument, 0, len(paths))
	for _, relativePath := range paths {
		id := relativePath
		if idForPath != nil {
			id = idForPath(relativePath)
		}
		doc, err := markdownDocumentFromSnapshot(snapshots[relativePath], kind, id)
		if err != nil {
			return nil, false, err
		}
		docs = append(docs, doc)
	}
	return docs, true, nil
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

	summary := transportWorkflowSummaryWithTaskCounts(workflow, taskCounts)
	if err := attachWorkflowArchiveEligibility(ctx, s.globalDB, workflow, &summary); err != nil {
		return WorkflowCard{}, err
	}

	return WorkflowCard{
		Workflow:         summary,
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
	target workflowReadTarget,
) ([]memoryDocumentRef, error) {
	if refs, ok, err := s.listSnapshotMemoryDocuments(target); err != nil {
		return nil, err
	} else if ok {
		return refs, nil
	}

	if target.workspace.FilesystemState == globaldb.WorkspaceFilesystemStateMissing &&
		!workflowReadTargetUsesArchivedFS(target) {
		return nil, nil
	}

	workflowRoot := target.rootDir
	memoryDir := filepath.Join(workflowRoot, "memory")
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
		doc, err := s.documents.Read(ctx, entry.absPath, markdownDocumentKindMemory, fileID)
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

func (s *queryService) listSnapshotMemoryDocuments(
	target workflowReadTarget,
) ([]memoryDocumentRef, bool, error) {
	if len(target.snapshotsByPath) == 0 {
		return nil, false, nil
	}

	paths := make([]string, 0)
	for relativePath := range target.snapshotsByPath {
		snapshot := target.snapshotsByPath[relativePath]
		if snapshot.ArtifactKind == "memory" || strings.HasPrefix(relativePath, "memory/") {
			paths = append(paths, relativePath)
		}
	}
	if len(paths) == 0 {
		return nil, false, nil
	}
	sort.Strings(paths)

	refs := make([]memoryDocumentRef, 0, len(paths))
	for _, relativePath := range paths {
		snapshot := target.snapshotsByPath[relativePath]
		fileID := memoryFileID(target.workspace.ID, target.workflow.Slug, relativePath)
		doc, err := markdownDocumentFromSnapshot(snapshot, markdownDocumentKindMemory, fileID)
		if err != nil {
			return nil, false, err
		}
		sizeBytes := int64(len(snapshot.BodyText))
		snapshotCopy := snapshot
		refs = append(refs, memoryDocumentRef{
			entry: WorkflowMemoryEntry{
				FileID:      fileID,
				DisplayPath: relativePath,
				Kind:        classifyMemoryEntry(relativePath),
				Title:       doc.Title,
				SizeBytes:   sizeBytes,
				UpdatedAt:   snapshot.SourceMTime,
			},
			snapshot: &snapshotCopy,
		})
	}
	return refs, true, nil
}

func (s *queryService) readWorkflowDocument(
	ctx context.Context,
	target workflowReadTarget,
	relativePath string,
	kind string,
	id string,
) (MarkdownDocument, bool, error) {
	if doc, ok, err := snapshotDocument(target.snapshotsByPath, relativePath, kind, id); err != nil {
		return MarkdownDocument{}, false, err
	} else if ok {
		return doc, true, nil
	}
	if target.workspace.FilesystemState == globaldb.WorkspaceFilesystemStateMissing &&
		!workflowReadTargetUsesArchivedFS(target) {
		return MarkdownDocument{}, false, nil
	}

	workflowRoot := target.rootDir
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
	target workflowReadTarget,
	relativePath string,
	kind string,
	id string,
) (MarkdownDocument, error) {
	if doc, ok, err := snapshotDocument(target.snapshotsByPath, relativePath, kind, id); err != nil {
		return MarkdownDocument{}, err
	} else if ok {
		return doc, nil
	}

	workflowRoot := target.rootDir
	absPath := filepath.Join(workflowRoot, filepath.FromSlash(relativePath))
	doc, err := s.documents.Read(ctx, absPath, kind, id)
	if err != nil {
		if errors.Is(err, ErrDocumentMissing) {
			return MarkdownDocument{}, DocumentMissingError{
				Kind:         kind,
				WorkflowSlug: target.workflow.Slug,
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

func workflowReadTargetUsesArchivedFS(target workflowReadTarget) bool {
	archiveRoot := filepath.Clean(model.ArchivedTasksDir(model.TasksBaseDirForWorkspace(target.workspace.RootDir)))
	rootDir := filepath.Clean(strings.TrimSpace(target.rootDir))
	rel, err := filepath.Rel(archiveRoot, rootDir)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func readableWorkflowRootDir(workspaceRoot string, workflow globaldb.Workflow) (string, error) {
	if workflow.ArchivedAt != nil {
		root := archivedWorkflowRootDir(workspaceRoot, workflow)
		if err := fileInfo(root); err == nil {
			return root, nil
		} else if !errors.Is(err, ErrDocumentMissing) {
			return "", err
		}
		if fallback, ok, err := latestArchivedWorkflowRootDir(workspaceRoot, workflow.Slug); err != nil {
			return "", err
		} else if ok {
			return fallback, nil
		}
		return root, nil
	}

	root := workflowRootDir(workspaceRoot, workflow.Slug)
	if err := fileInfo(root); err == nil {
		return root, nil
	} else if !errors.Is(err, ErrDocumentMissing) {
		return "", err
	}
	if fallback, ok, err := latestArchivedWorkflowRootDir(workspaceRoot, workflow.Slug); err != nil {
		return "", err
	} else if ok {
		return fallback, nil
	}
	return root, nil
}

func archivedWorkflowRootDir(workspaceRoot string, workflow globaldb.Workflow) string {
	if workflow.ArchivedAt == nil {
		return workflowRootDir(workspaceRoot, workflow.Slug)
	}
	return filepath.Join(
		model.ArchivedTasksDir(model.TasksBaseDirForWorkspace(workspaceRoot)),
		model.ArchivedWorkflowName(workflow.Slug, workflow.ID, *workflow.ArchivedAt),
	)
}

func latestArchivedWorkflowRootDir(workspaceRoot string, workflowSlug string) (string, bool, error) {
	archiveRoot := model.ArchivedTasksDir(model.TasksBaseDirForWorkspace(workspaceRoot))
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("daemon: read archived workflow directory %q: %w", archiveRoot, err)
	}

	type candidate struct {
		name      string
		path      string
		timestamp int64
	}
	candidates := make([]candidate, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		timestamp, ok := archivedWorkflowTimestampForSlug(entry.Name(), workflowSlug)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{
			name:      entry.Name(),
			path:      filepath.Join(archiveRoot, entry.Name()),
			timestamp: timestamp,
		})
	}
	if len(candidates) == 0 {
		return "", false, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].timestamp == candidates[j].timestamp {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].timestamp < candidates[j].timestamp
	})
	return candidates[len(candidates)-1].path, true, nil
}

func archivedWorkflowTimestampForSlug(name string, workflowSlug string) (int64, bool) {
	parts := strings.SplitN(strings.TrimSpace(name), "-", 3)
	if len(parts) != 3 || parts[2] != strings.TrimSpace(workflowSlug) {
		return 0, false
	}
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return timestamp, true
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

func dashboardVisibleRuns(runs []apicore.Run) []apicore.Run {
	if len(runs) == 0 {
		return nil
	}
	byID := make(map[string]apicore.Run, len(runs))
	for i := range runs {
		run := runs[i]
		runID := strings.TrimSpace(run.RunID)
		if runID == "" {
			continue
		}
		byID[runID] = run
	}

	visible := make([]apicore.Run, 0, len(runs))
	for i := range runs {
		run := runs[i]
		if dashboardShouldHideChildRun(run, byID) {
			continue
		}
		visible = append(visible, run)
	}
	return visible
}

func dashboardShouldHideChildRun(run apicore.Run, byID map[string]apicore.Run) bool {
	parentRunID := strings.TrimSpace(run.ParentRunID)
	if parentRunID == "" {
		return false
	}
	parent, ok := byID[parentRunID]
	if !ok {
		return false
	}
	if !isTerminalRunStatus(parent.Status) {
		return true
	}
	return isTerminalRunStatus(run.Status)
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
		return memoryEntryKindWorkflow
	case taskscore.ExtractTaskNumber(base) > 0:
		return memoryEntryKindTask
	default:
		return memoryEntryKindMemory
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
