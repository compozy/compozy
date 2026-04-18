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
	case e.TaskTotal == 0:
		return archiveReasonNoTaskFiles
	case e.PendingTasks > 0:
		return archiveReasonTasksIncomplete
	case e.UnresolvedReviewIssues > 0:
		return archiveReasonReviewsUnresolved
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
				  AND LOWER(TRIM(status)) <> 'completed'
			), 0),
			COALESCE((SELECT COUNT(1) FROM review_rounds WHERE workflow_id = ?), 0),
			COALESCE((SELECT SUM(unresolved_count) FROM review_rounds WHERE workflow_id = ?), 0),
			COALESCE((
				SELECT COUNT(1)
				FROM runs
				WHERE workflow_id = ?
				  AND LOWER(TRIM(status)) NOT IN ('completed', 'failed', 'cancelled', 'canceled', 'crashed')
			), 0)`,
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
