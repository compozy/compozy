package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

const workflowArchiveReasonArchived = "workflow archived"

func transportWorkspace(row globaldb.Workspace) apicore.Workspace {
	return apicore.Workspace{
		ID:              row.ID,
		RootDir:         row.RootDir,
		Name:            row.Name,
		FilesystemState: row.FilesystemState,
		ReadOnly:        row.ReadOnly,
		HasCatalogData:  row.HasCatalogData,
		WorkflowCount:   row.WorkflowCount,
		RunCount:        row.RunCount,
		LastCheckedAt:   row.LastCheckedAt,
		LastSyncedAt:    row.LastSyncedAt,
		LastSyncError:   row.LastSyncError,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func transportWorkflowSummary(row globaldb.Workflow) apicore.WorkflowSummary {
	return apicore.WorkflowSummary{
		ID:           row.ID,
		WorkspaceID:  row.WorkspaceID,
		Slug:         row.Slug,
		ArchivedAt:   row.ArchivedAt,
		LastSyncedAt: row.LastSyncedAt,
	}
}

func transportWorkflowSummaryWithTaskCounts(
	row globaldb.Workflow,
	counts WorkflowTaskCounts,
) apicore.WorkflowSummary {
	summary := transportWorkflowSummary(row)
	apiCounts := transportWorkflowTaskCounts(counts)
	canStart, reason := workflowStartAction(row, counts)
	summary.TaskCounts = &apiCounts
	summary.CanStartRun = &canStart
	summary.StartBlockReason = reason
	return summary
}

func attachWorkflowArchiveEligibility(
	ctx context.Context,
	db *globaldb.GlobalDB,
	row globaldb.Workflow,
	summary *apicore.WorkflowSummary,
) error {
	if summary == nil {
		return nil
	}
	eligible, reason, err := workflowArchiveAction(ctx, db, row)
	if err != nil {
		return err
	}
	summary.ArchiveEligible = &eligible
	summary.ArchiveReason = reason
	return nil
}

func workflowArchiveAction(
	ctx context.Context,
	db *globaldb.GlobalDB,
	row globaldb.Workflow,
) (bool, string, error) {
	if row.ArchivedAt != nil {
		return false, workflowArchiveReasonArchived, nil
	}
	eligibility, err := db.GetWorkflowArchiveEligibility(ctx, row.WorkspaceID, row.Slug)
	if err != nil {
		return false, "", err
	}
	return eligibility.Archivable(), eligibility.SkipReason(), nil
}

func workflowStartAction(row globaldb.Workflow, counts WorkflowTaskCounts) (bool, string) {
	if row.ArchivedAt != nil {
		return false, workflowArchiveReasonArchived
	}
	if counts.Total > 0 && counts.Pending == 0 {
		return false, "no pending tasks"
	}
	return true, ""
}

func transportSyncResult(
	workspaceID string,
	workflowSlug string,
	syncedAt *time.Time,
	result *corepkg.SyncResult,
) apicore.SyncResult {
	out := apicore.SyncResult{
		WorkspaceID:  workspaceID,
		WorkflowSlug: workflowSlug,
		SyncedAt:     syncedAt,
	}
	if result == nil {
		return out
	}

	out.Target = result.Target
	out.WorkflowsScanned = result.WorkflowsScanned
	out.WorkflowsPruned = result.WorkflowsPruned
	out.SnapshotsUpserted = result.SnapshotsUpserted
	out.TaskItemsUpserted = result.TaskItemsUpserted
	out.ReviewRoundsUpserted = result.ReviewRoundsUpserted
	out.ReviewIssuesUpserted = result.ReviewIssuesUpserted
	out.CheckpointsUpdated = result.CheckpointsUpdated
	out.LegacyArtifactsRemoved = result.LegacyArtifactsRemoved
	out.SyncedPaths = append([]string(nil), result.SyncedPaths...)
	out.PrunedWorkflows = append([]string(nil), result.PrunedWorkflows...)
	out.Warnings = append([]string(nil), result.Warnings...)
	return out
}

func transportArchiveResult(result *corepkg.ArchiveResult) apicore.ArchiveResult {
	out := apicore.ArchiveResult{}
	if result == nil {
		return out
	}

	out.Archived = result.Archived > 0
	out.ArchivedAt = result.ArchivedAt
	out.Forced = result.Forced
	out.CompletedTasks = result.CompletedTasks
	out.ResolvedReviewIssues = result.ResolvedReviewIssues
	return out
}

func transportDashboard(payload WorkspaceDashboard) apicore.DashboardPayload {
	return apicore.DashboardPayload{
		Workspace:      payload.Workspace,
		Daemon:         payload.Daemon,
		Health:         payload.Health,
		Queue:          transportDashboardQueue(payload.Queue),
		Workflows:      transportWorkflowCards(payload.Workflows),
		ActiveRuns:     append([]apicore.Run(nil), payload.ActiveRuns...),
		PendingReviews: payload.PendingReviews,
	}
}

func transportDashboardQueue(summary DashboardQueueSummary) apicore.DashboardQueueSummary {
	return apicore.DashboardQueueSummary{
		Total:     summary.Total,
		Active:    summary.Active,
		Completed: summary.Completed,
		Failed:    summary.Failed,
		Canceled:  summary.Canceled,
	}
}

func transportWorkflowCards(cards []WorkflowCard) []apicore.WorkflowCard {
	if len(cards) == 0 {
		return nil
	}
	out := make([]apicore.WorkflowCard, 0, len(cards))
	for i := range cards {
		out = append(out, transportWorkflowCard(cards[i]))
	}
	return out
}

func transportWorkflowCard(card WorkflowCard) apicore.WorkflowCard {
	var latestReview *apicore.ReviewSummary
	if card.LatestReview != nil {
		copyValue := *card.LatestReview
		latestReview = &copyValue
	}
	return apicore.WorkflowCard{
		Workflow:         card.Workflow,
		TaskTotal:        card.TaskTotal,
		TaskCompleted:    card.TaskCompleted,
		TaskPending:      card.TaskPending,
		LatestReview:     latestReview,
		ReviewRoundCount: card.ReviewRoundCount,
		ActiveRuns:       card.ActiveRuns,
	}
}

func transportWorkflowOverview(payload WorkflowOverviewPayload) apicore.WorkflowOverviewPayload {
	var latestReview *apicore.ReviewSummary
	if payload.LatestReview != nil {
		copyValue := *payload.LatestReview
		latestReview = &copyValue
	}
	return apicore.WorkflowOverviewPayload{
		Workspace:       payload.Workspace,
		Workflow:        payload.Workflow,
		TaskCounts:      transportWorkflowTaskCounts(payload.TaskCounts),
		LatestReview:    latestReview,
		RecentRuns:      append([]apicore.Run(nil), payload.RecentRuns...),
		ArchiveEligible: payload.ArchiveEligible,
		ArchiveReason:   payload.ArchiveReason,
	}
}

func transportWorkflowTaskCounts(counts WorkflowTaskCounts) apicore.WorkflowTaskCounts {
	return apicore.WorkflowTaskCounts{
		Total:     counts.Total,
		Completed: counts.Completed,
		Pending:   counts.Pending,
	}
}

func transportTaskBoard(payload TaskBoardPayload) apicore.TaskBoardPayload {
	return apicore.TaskBoardPayload{
		Workspace:  payload.Workspace,
		Workflow:   payload.Workflow,
		TaskCounts: transportWorkflowTaskCounts(payload.TaskCounts),
		Lanes:      transportTaskLanes(payload.Lanes),
	}
}

func transportTaskLanes(lanes []TaskLane) []apicore.TaskLane {
	if len(lanes) == 0 {
		return nil
	}
	out := make([]apicore.TaskLane, 0, len(lanes))
	for i := range lanes {
		out = append(out, transportTaskLane(lanes[i]))
	}
	return out
}

func transportTaskLane(lane TaskLane) apicore.TaskLane {
	return apicore.TaskLane{
		Status: lane.Status,
		Title:  lane.Title,
		Items:  transportTaskCards(lane.Items),
	}
}

func transportTaskCards(cards []TaskCard) []apicore.TaskCard {
	if len(cards) == 0 {
		return nil
	}
	out := make([]apicore.TaskCard, 0, len(cards))
	for i := range cards {
		out = append(out, transportTaskCard(cards[i]))
	}
	return out
}

func transportTaskCard(card TaskCard) apicore.TaskCard {
	return apicore.TaskCard{
		TaskNumber: card.TaskNumber,
		TaskID:     card.TaskID,
		Title:      card.Title,
		Status:     card.Status,
		Type:       card.Type,
		DependsOn:  append([]string(nil), card.DependsOn...),
		UpdatedAt:  card.UpdatedAt,
	}
}

func transportMarkdownDocument(doc MarkdownDocument) apicore.MarkdownDocument {
	return apicore.MarkdownDocument{
		ID:        doc.ID,
		Kind:      doc.Kind,
		Title:     doc.Title,
		UpdatedAt: doc.UpdatedAt,
		Markdown:  doc.Markdown,
		Metadata:  marshalTransportMetadata(doc.Metadata),
	}
}

func transportWorkflowSpec(doc WorkflowSpecDocument) apicore.WorkflowSpecDocument {
	out := apicore.WorkflowSpecDocument{
		Workspace: doc.Workspace,
		Workflow:  doc.Workflow,
		ADRs:      make([]apicore.MarkdownDocument, 0, len(doc.ADRs)),
	}
	if doc.PRD != nil {
		prd := transportMarkdownDocument(*doc.PRD)
		out.PRD = &prd
	}
	if doc.TechSpec != nil {
		techspec := transportMarkdownDocument(*doc.TechSpec)
		out.TechSpec = &techspec
	}
	for i := range doc.ADRs {
		out.ADRs = append(out.ADRs, transportMarkdownDocument(doc.ADRs[i]))
	}
	if len(out.ADRs) == 0 {
		out.ADRs = nil
	}
	return out
}

func transportWorkflowMemoryIndex(index WorkflowMemoryIndex) apicore.WorkflowMemoryIndex {
	return apicore.WorkflowMemoryIndex{
		Workspace: index.Workspace,
		Workflow:  index.Workflow,
		Entries:   transportWorkflowMemoryEntries(index.Entries),
	}
}

func transportWorkflowMemoryEntries(entries []WorkflowMemoryEntry) []apicore.WorkflowMemoryEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]apicore.WorkflowMemoryEntry, 0, len(entries))
	for i := range entries {
		out = append(out, transportWorkflowMemoryEntry(entries[i]))
	}
	return out
}

func transportWorkflowMemoryEntry(entry WorkflowMemoryEntry) apicore.WorkflowMemoryEntry {
	return apicore.WorkflowMemoryEntry{
		FileID:      entry.FileID,
		DisplayPath: entry.DisplayPath,
		Kind:        entry.Kind,
		Title:       entry.Title,
		SizeBytes:   entry.SizeBytes,
		UpdatedAt:   entry.UpdatedAt,
	}
}

func transportTaskDetail(payload TaskDetailPayload) apicore.TaskDetailPayload {
	return apicore.TaskDetailPayload{
		Workspace:         payload.Workspace,
		Workflow:          payload.Workflow,
		Task:              transportTaskCard(payload.Task),
		Document:          transportMarkdownDocument(payload.Document),
		MemoryEntries:     transportWorkflowMemoryEntries(payload.MemoryEntries),
		RelatedRuns:       append([]apicore.Run(nil), payload.RelatedRuns...),
		LiveTailAvailable: payload.LiveTailAvailable,
	}
}

func transportReviewDetail(payload ReviewDetailPayload) apicore.ReviewDetailPayload {
	return apicore.ReviewDetailPayload{
		Workspace:   payload.Workspace,
		Workflow:    payload.Workflow,
		Round:       payload.Round,
		Issue:       transportReviewIssueDetail(payload.Issue),
		Document:    transportMarkdownDocument(payload.Document),
		RelatedRuns: append([]apicore.Run(nil), payload.RelatedRuns...),
	}
}

func transportReviewIssueDetail(detail ReviewIssueDetail) apicore.ReviewIssueDetail {
	return apicore.ReviewIssueDetail{
		ID:          detail.ID,
		IssueNumber: detail.IssueNumber,
		Severity:    detail.Severity,
		Status:      detail.Status,
		UpdatedAt:   detail.UpdatedAt,
	}
}

func transportRunDetail(payload RunDetailPayload) apicore.RunDetailPayload {
	return apicore.RunDetailPayload{
		Run:          payload.Run,
		Snapshot:     payload.Snapshot,
		JobCounts:    transportRunJobCounts(payload.JobCounts),
		Runtime:      transportRunRuntimeSummary(payload.Runtime),
		Timeline:     append([]eventspkg.Event(nil), payload.Timeline...),
		ArtifactSync: transportRunArtifactSyncEntries(payload.ArtifactSync),
	}
}

func transportRunJobCounts(counts RunJobCounts) apicore.RunJobCounts {
	return apicore.RunJobCounts{
		Queued:    counts.Queued,
		Running:   counts.Running,
		Retrying:  counts.Retrying,
		Completed: counts.Completed,
		Failed:    counts.Failed,
		Canceled:  counts.Canceled,
	}
}

func transportRunRuntimeSummary(summary RunRuntimeSummary) apicore.RunRuntimeSummary {
	return apicore.RunRuntimeSummary{
		IDEs:              append([]string(nil), summary.IDEs...),
		Models:            append([]string(nil), summary.Models...),
		ReasoningEfforts:  append([]string(nil), summary.ReasoningEfforts...),
		AccessModes:       append([]string(nil), summary.AccessModes...),
		PresentationModes: append([]string(nil), summary.PresentationModes...),
	}
}

func transportRunArtifactSyncEntries(entries []rundb.ArtifactSyncRow) []apicore.RunArtifactSyncEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]apicore.RunArtifactSyncEntry, 0, len(entries))
	for i := range entries {
		out = append(out, apicore.RunArtifactSyncEntry{
			Sequence:     entries[i].Sequence,
			RelativePath: entries[i].RelativePath,
			ChangeKind:   entries[i].ChangeKind,
			Checksum:     entries[i].Checksum,
			SyncedAt:     entries[i].SyncedAt,
		})
	}
	return out
}

func mapQueryTransportError(err error) error {
	if err == nil {
		return nil
	}

	var missingErr DocumentMissingError
	if errors.As(err, &missingErr) {
		return apicore.NewProblem(
			http.StatusNotFound,
			"document_not_found",
			missingErr.Error(),
			map[string]any{
				"kind":          missingErr.Kind,
				"workflow_slug": missingErr.WorkflowSlug,
				"relative_path": missingErr.RelativePath,
			},
			err,
		)
	}

	var staleErr StaleDocumentReferenceError
	if errors.As(err, &staleErr) {
		return apicore.NewProblem(
			http.StatusNotFound,
			"stale_document_reference",
			staleErr.Error(),
			map[string]any{
				"kind":          staleErr.Kind,
				"workflow_slug": staleErr.WorkflowSlug,
				"reference":     staleErr.Reference,
			},
			err,
		)
	}

	var reviewIssueErr ReviewIssueNotFoundError
	if errors.As(err, &reviewIssueErr) {
		return apicore.NewProblem(
			http.StatusNotFound,
			"review_issue_not_found",
			reviewIssueErr.Error(),
			map[string]any{
				"workflow_slug": reviewIssueErr.WorkflowSlug,
				"round":         reviewIssueErr.Round,
				"issue_ref":     reviewIssueErr.IssueRef,
			},
			err,
		)
	}

	switch {
	case errors.Is(err, globaldb.ErrTaskItemNotFound):
		return apicore.NewProblem(
			http.StatusNotFound,
			"task_item_not_found",
			"task item was not found",
			nil,
			err,
		)
	case errors.Is(err, globaldb.ErrReviewRoundNotFound):
		return apicore.NewProblem(
			http.StatusNotFound,
			"review_round_not_found",
			"review round was not found",
			nil,
			err,
		)
	default:
		return err
	}
}

func cloneTransportMetadataMap(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = cloneTransportMetadataValue(value)
	}
	return cloned
}

func cloneTransportMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneTransportMetadataMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i := range typed {
			cloned[i] = cloneTransportMetadataValue(typed[i])
		}
		return cloned
	default:
		return typed
	}
}

func marshalTransportMetadata(metadata map[string]any) json.RawMessage {
	if len(metadata) == 0 {
		return nil
	}
	body, err := json.Marshal(cloneTransportMetadataMap(metadata))
	if err != nil {
		return nil
	}
	return json.RawMessage(body)
}

func resolveTransportQueryService(
	globalDB *globaldb.GlobalDB,
	runManager *RunManager,
	daemon daemonStatusReader,
	provided []QueryService,
) QueryService {
	for _, candidate := range provided {
		if candidate != nil {
			return candidate
		}
	}
	if globalDB == nil && runManager == nil && daemon == nil {
		return nil
	}
	return NewQueryService(QueryServiceConfig{
		GlobalDB:   globalDB,
		RunManager: runManager,
		Daemon:     daemon,
	})
}

func resolveWorkspaceReference(
	ctx context.Context,
	globalDB *globaldb.GlobalDB,
	ref string,
) (globaldb.Workspace, error) {
	if globalDB == nil {
		return globaldb.Workspace{}, apicore.NewProblem(
			500,
			"workspace_registry_unavailable",
			"workspace registry is unavailable",
			nil,
			nil,
		)
	}

	trimmedRef := strings.TrimSpace(ref)
	row, err := globalDB.Get(ctx, trimmedRef)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		return globaldb.Workspace{}, err
	}
	return globalDB.ResolveOrRegister(ctx, trimmedRef)
}
