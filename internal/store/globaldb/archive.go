package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrWorkflowArchived reports an archive request against an archived workflow identity.
	ErrWorkflowArchived = errors.New("globaldb: workflow archived")
	// ErrWorkflowHasActiveRuns reports archive conflicts caused by active runs.
	ErrWorkflowHasActiveRuns = errors.New("globaldb: workflow has active runs")
	// ErrWorkflowNotArchivable reports archive conflicts caused by incomplete synced state.
	ErrWorkflowNotArchivable = errors.New("globaldb: workflow not archivable")
)

const (
	archiveReasonActiveRuns        = "workflow has active runs"
	archiveReasonNoTaskFiles       = "no task files present"
	archiveReasonTasksIncomplete   = "task workflow not fully completed"
	archiveReasonReviewsUnresolved = "review rounds not fully resolved"
)

// WorkflowArchivedError reports an archive request against an already archived workflow identity.
type WorkflowArchivedError struct {
	WorkspaceID string
	Slug        string
}

func (e WorkflowArchivedError) Error() string {
	if strings.TrimSpace(e.Slug) == "" {
		return "globaldb: workflow is already archived"
	}
	return fmt.Sprintf("globaldb: workflow %q is already archived", e.Slug)
}

func (e WorkflowArchivedError) Is(target error) bool {
	return target == ErrWorkflowArchived
}

// WorkflowActiveRunsError reports how many active runs block archiving one workflow.
type WorkflowActiveRunsError struct {
	WorkspaceID string
	WorkflowID  string
	Slug        string
	ActiveRuns  int
}

func (e WorkflowActiveRunsError) Error() string {
	name := strings.TrimSpace(e.Slug)
	if name == "" {
		name = strings.TrimSpace(e.WorkflowID)
	}
	if name == "" {
		name = "workflow"
	}
	return fmt.Sprintf("globaldb: workflow %q has %d active run(s)", name, e.ActiveRuns)
}

func (e WorkflowActiveRunsError) Is(target error) bool {
	return target == ErrWorkflowHasActiveRuns
}

// WorkflowNotArchivableError reports a synced-state reason that blocks archiving one workflow.
type WorkflowNotArchivableError struct {
	WorkspaceID string
	WorkflowID  string
	Slug        string
	Reason      string
}

func (e WorkflowNotArchivableError) Error() string {
	name := strings.TrimSpace(e.Slug)
	if name == "" {
		name = strings.TrimSpace(e.WorkflowID)
	}
	if strings.TrimSpace(e.Reason) == "" {
		if name == "" {
			return "globaldb: workflow is not archivable"
		}
		return fmt.Sprintf("globaldb: workflow %q is not archivable", name)
	}
	if name == "" {
		return fmt.Sprintf("globaldb: workflow is not archivable: %s", e.Reason)
	}
	return fmt.Sprintf("globaldb: workflow %q is not archivable: %s", name, e.Reason)
}

func (e WorkflowNotArchivableError) Is(target error) bool {
	return target == ErrWorkflowNotArchivable
}

// WorkflowArchiveEligibility captures the synced daemon state used to decide whether one workflow can be archived.
type WorkflowArchiveEligibility struct {
	Workflow               Workflow
	TaskTotal              int
	PendingTasks           int
	ReviewRoundCount       int
	ReviewIssueTotal       int
	UnresolvedReviewIssues int
	ActiveRuns             int
}

// Archivable reports whether the workflow can be archived.
func (e WorkflowArchiveEligibility) Archivable() bool {
	return strings.TrimSpace(e.SkipReason()) == ""
}

// SkipReason reports why the workflow is not archivable.
func (e WorkflowArchiveEligibility) SkipReason() string {
	switch {
	case e.ActiveRuns > 0:
		return archiveReasonActiveRuns
	case e.PendingTasks > 0:
		return archiveReasonTasksIncomplete
	case e.UnresolvedReviewIssues > 0:
		return archiveReasonReviewsUnresolved
	case e.TaskTotal == 0 && e.ReviewIssueTotal == 0:
		return archiveReasonNoTaskFiles
	default:
		return ""
	}
}

// ConflictError converts one ineligible workflow snapshot into the canonical typed conflict.
func (e WorkflowArchiveEligibility) ConflictError() error {
	if e.ActiveRuns > 0 {
		return WorkflowActiveRunsError{
			WorkspaceID: e.Workflow.WorkspaceID,
			WorkflowID:  e.Workflow.ID,
			Slug:        e.Workflow.Slug,
			ActiveRuns:  e.ActiveRuns,
		}
	}
	if reason := e.SkipReason(); reason != "" {
		return WorkflowNotArchivableError{
			WorkspaceID: e.Workflow.WorkspaceID,
			WorkflowID:  e.Workflow.ID,
			Slug:        e.Workflow.Slug,
			Reason:      reason,
		}
	}
	return nil
}

// GetWorkflowArchiveEligibility reads the synced daemon catalog state used to archive one workflow.
func (g *GlobalDB) GetWorkflowArchiveEligibility(
	ctx context.Context,
	workspaceID string,
	slug string,
) (WorkflowArchiveEligibility, error) {
	if err := g.requireContext(ctx, "get workflow archive eligibility"); err != nil {
		return WorkflowArchiveEligibility{}, err
	}

	workflow, err := g.GetActiveWorkflowBySlug(ctx, workspaceID, slug)
	if err != nil {
		return WorkflowArchiveEligibility{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT
			COALESCE((SELECT COUNT(1) FROM task_items WHERE workflow_id = ?), 0),
			COALESCE((
				SELECT COUNT(1)
				FROM task_items
				WHERE workflow_id = ?
				  AND status <> 'completed'
			), 0),
			COALESCE((SELECT COUNT(1) FROM review_rounds WHERE workflow_id = ?), 0),
			COALESCE((
				SELECT COUNT(1)
				FROM review_issues issues
				JOIN review_rounds rounds ON rounds.id = issues.round_id
				WHERE rounds.workflow_id = ?
			), 0),
			COALESCE((SELECT SUM(unresolved_count) FROM review_rounds WHERE workflow_id = ?), 0),
			COALESCE((
				SELECT COUNT(1)
				FROM runs
				WHERE workflow_id = ?
				  AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')
			), 0)`,
		workflow.ID,
		workflow.ID,
		workflow.ID,
		workflow.ID,
		workflow.ID,
		workflow.ID,
	)

	eligibility := WorkflowArchiveEligibility{Workflow: workflow}
	if err := row.Scan(
		&eligibility.TaskTotal,
		&eligibility.PendingTasks,
		&eligibility.ReviewRoundCount,
		&eligibility.ReviewIssueTotal,
		&eligibility.UnresolvedReviewIssues,
		&eligibility.ActiveRuns,
	); err != nil {
		return WorkflowArchiveEligibility{}, fmt.Errorf(
			"globaldb: query workflow archive eligibility %q: %w",
			workflow.ID,
			err,
		)
	}
	return eligibility, nil
}

// WorkflowArchiveEligibilityByIDs returns archive-eligibility snapshots keyed by workflow id.
func (g *GlobalDB) WorkflowArchiveEligibilityByIDs(
	ctx context.Context,
	workflows []Workflow,
) (map[string]WorkflowArchiveEligibility, error) {
	if err := g.requireContext(ctx, "list workflow archive eligibility"); err != nil {
		return nil, err
	}

	result := make(map[string]WorkflowArchiveEligibility, len(workflows))
	workflowIDs := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		workflowID := strings.TrimSpace(workflow.ID)
		if workflowID == "" {
			continue
		}
		if _, ok := result[workflowID]; ok {
			continue
		}
		workflow.ID = workflowID
		result[workflowID] = WorkflowArchiveEligibility{Workflow: workflow}
		workflowIDs = append(workflowIDs, workflowID)
	}
	if len(workflowIDs) == 0 {
		return result, nil
	}

	rows, err := g.queryWorkflowArchiveEligibilityByIDs(ctx, workflowIDs)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	if err := scanWorkflowArchiveEligibilityRows(rows, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (g *GlobalDB) queryWorkflowArchiveEligibilityByIDs(
	ctx context.Context,
	workflowIDs []string,
) (*sql.Rows, error) {
	valuesClause, args := selectedWorkflowIDsValues(workflowIDs)
	rows, err := g.db.QueryContext(
		ctx,
		`WITH selected_workflow_ids(id) AS (VALUES `+valuesClause+`),
		 task_counts AS (
		 	SELECT task_items.workflow_id,
		 	       COUNT(1) AS total,
		 	       SUM(CASE WHEN task_items.status <> 'completed' THEN 1 ELSE 0 END) AS pending
		 	FROM task_items
		 	JOIN selected_workflow_ids selected ON selected.id = task_items.workflow_id
		 	GROUP BY task_items.workflow_id
		 ),
		 review_round_counts AS (
		 	SELECT review_rounds.workflow_id,
		 	       COUNT(1) AS total,
		 	       COALESCE(SUM(review_rounds.unresolved_count), 0) AS unresolved
		 	FROM review_rounds
		 	JOIN selected_workflow_ids selected ON selected.id = review_rounds.workflow_id
		 	GROUP BY review_rounds.workflow_id
		 ),
		 review_issue_counts AS (
		 	SELECT rounds.workflow_id,
		 	       COUNT(1) AS total
		 	FROM review_issues issues
		 	JOIN review_rounds rounds ON rounds.id = issues.round_id
		 	JOIN selected_workflow_ids selected ON selected.id = rounds.workflow_id
		 	GROUP BY rounds.workflow_id
		 ),
		 active_run_counts AS (
		 	SELECT runs.workflow_id,
		 	       COUNT(1) AS active
		 	FROM runs
		 	JOIN selected_workflow_ids selected ON selected.id = runs.workflow_id
		 	WHERE runs.status NOT IN ('completed', 'failed', 'canceled', 'crashed')
		 	GROUP BY runs.workflow_id
		 )
		 SELECT workflows.id,
		        COALESCE(task_counts.total, 0),
		        COALESCE(task_counts.pending, 0),
		        COALESCE(review_round_counts.total, 0),
		        COALESCE(review_issue_counts.total, 0),
		        COALESCE(review_round_counts.unresolved, 0),
		        COALESCE(active_run_counts.active, 0)
		 FROM workflows
		 JOIN selected_workflow_ids selected ON selected.id = workflows.id
		 LEFT JOIN task_counts ON task_counts.workflow_id = workflows.id
		 LEFT JOIN review_round_counts ON review_round_counts.workflow_id = workflows.id
		 LEFT JOIN review_issue_counts ON review_issue_counts.workflow_id = workflows.id
		 LEFT JOIN active_run_counts ON active_run_counts.workflow_id = workflows.id`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query workflow archive eligibility by ids: %w", err)
	}
	return rows, nil
}

func scanWorkflowArchiveEligibilityRows(
	rows *sql.Rows,
	result map[string]WorkflowArchiveEligibility,
) error {
	for rows.Next() {
		var (
			workflowID string
			snapshot   WorkflowArchiveEligibility
		)
		if err := rows.Scan(
			&workflowID,
			&snapshot.TaskTotal,
			&snapshot.PendingTasks,
			&snapshot.ReviewRoundCount,
			&snapshot.ReviewIssueTotal,
			&snapshot.UnresolvedReviewIssues,
			&snapshot.ActiveRuns,
		); err != nil {
			return fmt.Errorf("globaldb: scan workflow archive eligibility by ids: %w", err)
		}
		current := result[workflowID]
		current.TaskTotal = snapshot.TaskTotal
		current.PendingTasks = snapshot.PendingTasks
		current.ReviewRoundCount = snapshot.ReviewRoundCount
		current.ReviewIssueTotal = snapshot.ReviewIssueTotal
		current.UnresolvedReviewIssues = snapshot.UnresolvedReviewIssues
		current.ActiveRuns = snapshot.ActiveRuns
		result[workflowID] = current
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("globaldb: iterate workflow archive eligibility by ids: %w", err)
	}
	return nil
}

// GetLatestArchivedWorkflowBySlug returns the most recently archived row for one workflow slug.
func (g *GlobalDB) GetLatestArchivedWorkflowBySlug(
	ctx context.Context,
	workspaceID string,
	slug string,
) (Workflow, error) {
	if err := g.requireContext(ctx, "get archived workflow by slug"); err != nil {
		return Workflow{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		 FROM workflows
		 WHERE workspace_id = ? AND slug = ? AND archived_at IS NOT NULL
		 ORDER BY archived_at DESC, created_at DESC, id DESC
		 LIMIT 1`,
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(slug),
	)
	workflow, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, ErrWorkflowNotFound
		}
		return Workflow{}, err
	}
	return workflow, nil
}

// MarkWorkflowArchived persists the archived state for one active workflow row.
func (g *GlobalDB) MarkWorkflowArchived(
	ctx context.Context,
	workflowID string,
	archivedAt time.Time,
) (Workflow, error) {
	if err := g.requireContext(ctx, "mark workflow archived"); err != nil {
		return Workflow{}, err
	}

	workflow, err := g.GetWorkflow(ctx, workflowID)
	if err != nil {
		return Workflow{}, err
	}
	if workflow.ArchivedAt != nil {
		return Workflow{}, WorkflowArchivedError{
			WorkspaceID: workflow.WorkspaceID,
			Slug:        workflow.Slug,
		}
	}

	if archivedAt.IsZero() {
		archivedAt = g.now()
	}
	archivedAt = archivedAt.UTC()

	result, err := g.db.ExecContext(
		ctx,
		`UPDATE workflows
		 SET archived_at = ?, updated_at = ?
		 WHERE id = ? AND archived_at IS NULL`,
		store.FormatTimestamp(archivedAt),
		store.FormatTimestamp(archivedAt),
		workflow.ID,
	)
	if err != nil {
		return Workflow{}, fmt.Errorf("globaldb: mark workflow archived %q: %w", workflow.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Workflow{}, fmt.Errorf("globaldb: rows affected for archived workflow %q: %w", workflow.ID, err)
	}
	if affected == 0 {
		return Workflow{}, WorkflowArchivedError{
			WorkspaceID: workflow.WorkspaceID,
			Slug:        workflow.Slug,
		}
	}

	return g.GetWorkflow(ctx, workflow.ID)
}
