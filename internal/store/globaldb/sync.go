package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	artifactBodyInlineKind   = "inline"
	artifactBodyBlobKind     = "body"
	artifactBodyOverflowKind = "overflow"
	artifactBodyLimitBytes   = 256 * 1024
	defaultSyncScope         = "workflow"
)

var ErrWorkflowSyncInvalid = errors.New("globaldb: workflow sync invalid")

// WorkflowSyncValidationError reports invalid authored workflow state that
// prevents sync from projecting the workflow into global.db.
type WorkflowSyncValidationError struct {
	Message string
}

func (e WorkflowSyncValidationError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "globaldb: workflow sync invalid"
	}
	return e.Message
}

func (e WorkflowSyncValidationError) Is(target error) bool {
	return target == ErrWorkflowSyncInvalid
}

func newWorkflowSyncValidationError(format string, args ...any) error {
	return WorkflowSyncValidationError{Message: fmt.Sprintf(format, args...)}
}

// ArtifactSnapshotInput describes one authored workflow artifact snapshot that
// should be mirrored into global.db.
type ArtifactSnapshotInput struct {
	ArtifactKind    string
	RelativePath    string
	Checksum        string
	FrontmatterJSON string
	BodyText        string
	SourceMTime     time.Time
}

// TaskItemInput describes one parsed task file projection row.
type TaskItemInput struct {
	TaskNumber int
	TaskID     string
	Title      string
	Status     string
	Kind       string
	DependsOn  []string
	SourcePath string
}

// ReviewIssueInput describes one parsed review issue projection row.
type ReviewIssueInput struct {
	IssueNumber int
	Severity    string
	Status      string
	SourcePath  string
}

// ReviewRoundInput describes one parsed review round plus its issue rows.
type ReviewRoundInput struct {
	RoundNumber     int
	Provider        string
	PRRef           string
	ResolvedCount   int
	UnresolvedCount int
	Issues          []ReviewIssueInput
}

// WorkflowSyncInput captures one workflow reconciliation payload.
type WorkflowSyncInput struct {
	WorkspaceID        string
	WorkflowSlug       string
	Kind               WorkflowKind
	ParentWorkflowID   string
	TaskGroupID        string
	DisplayTitle       string
	Outcome            string
	LifecycleCompleted bool
	// Missing seeds a placeholder row for a declared task group whose directory is
	// absent on disk. Materialized children leave it false so the read model can
	// tell a real, taskless task group apart from an unavailable placeholder.
	Missing bool
	// MetadataOnly restricts reconciliation to the workflow row itself, leaving the
	// existing artifact snapshots, task items, review rounds, and sync checkpoint
	// untouched. It preserves the last-known projection of a declared task group whose
	// directory has disappeared while still refreshing durable identity columns such
	// as Missing so read models reflect current source availability.
	MetadataOnly bool
	// MetadataOnlyIfExisting resolves MetadataOnly transactionally: this reconcile
	// preserves the retained projection only when a durable row already exists at the
	// point this transaction reads it, and otherwise seeds the row fresh from the
	// (empty) placeholder input. It lets a missing-directory placeholder sync decide
	// "preserve vs seed" atomically inside the reconcile write transaction instead of
	// relying on a pre-transaction existence check that a concurrent scoped sync can
	// invalidate before this transaction holds the write lock. Ignored when
	// MetadataOnly is already set.
	MetadataOnlyIfExisting bool
	Dependencies           []WorkflowDependency
	SyncedAt               time.Time
	CheckpointScope        string
	CheckpointChecksum     string
	ArtifactSnapshots      []ArtifactSnapshotInput
	TaskItems              []TaskItemInput
	ReviewRounds           []ReviewRoundInput
}

// WorkflowSyncResult reports the durable rows touched by one reconciliation.
type WorkflowSyncResult struct {
	Workflow             Workflow
	SnapshotsUpserted    int
	TaskItemsUpserted    int
	ReviewRoundsUpserted int
	ReviewIssuesUpserted int
	CheckpointsUpdated   int
}

// AggregateWorkflowSyncInput reconciles an opted-in initiative and each
// readable Task Group child in one database transaction.
type AggregateWorkflowSyncInput struct {
	Parent                  WorkflowSyncInput
	Children                []WorkflowSyncInput
	PreserveMissingChildren bool
}

// AggregateWorkflowSyncResult reports all rows touched by aggregate sync.
type AggregateWorkflowSyncResult struct {
	Parent                  WorkflowSyncResult
	Children                []WorkflowSyncResult
	PrunedChildTaskGroupIDs []string
	SkippedChildPrunes      []WorkflowPruneSkipped
}

// WorkflowPruneSkipped reports a stale active workflow row that pruning kept
// because durable state still indicates active work.
type WorkflowPruneSkipped struct {
	Slug       string
	Reason     string
	ActiveRuns int
}

// WorkflowPruneResult reports stale active workflow rows removed during a root
// sync plus rows deliberately kept for conflict reasons.
type WorkflowPruneResult struct {
	PrunedSlugs []string
	Skipped     []WorkflowPruneSkipped
}

type existingArtifactSnapshot struct {
	Checksum        string
	BodyText        string
	BodyStorageKind string
}

// ReconcileWorkflowSync upserts the authored workflow state into the daemon
// catalog and removes stale projection rows that no longer exist on disk.
func (g *GlobalDB) ReconcileWorkflowSync(
	ctx context.Context,
	input WorkflowSyncInput,
) (result WorkflowSyncResult, retErr error) {
	if err := g.requireContext(ctx, "reconcile workflow sync"); err != nil {
		return WorkflowSyncResult{}, err
	}
	input, err := normalizeWorkflowSyncInput(input)
	if err != nil {
		return WorkflowSyncResult{}, err
	}

	syncedAt := normalizeSyncTimestamp(input.SyncedAt, g.now)

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkflowSyncResult{}, fmt.Errorf("globaldb: begin workflow sync: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("globaldb: rollback workflow sync: %w", rollbackErr))
		}
	}()

	result, err = g.reconcileWorkflowSyncTx(ctx, tx, input, syncedAt)
	if err != nil {
		return WorkflowSyncResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return WorkflowSyncResult{}, fmt.Errorf("globaldb: commit workflow sync: %w", err)
	}
	committed = true

	return result, nil
}

func normalizeWorkflowSyncInput(input WorkflowSyncInput) (WorkflowSyncInput, error) {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.WorkflowSlug = strings.TrimSpace(input.WorkflowSlug)
	input.Kind = WorkflowKind(strings.TrimSpace(string(input.Kind)))
	input.ParentWorkflowID = strings.TrimSpace(input.ParentWorkflowID)
	input.TaskGroupID = strings.TrimSpace(input.TaskGroupID)
	input.DisplayTitle = strings.TrimSpace(input.DisplayTitle)
	input.Outcome = strings.TrimSpace(input.Outcome)
	if input.Kind == "" {
		input.Kind = WorkflowKindOrdinary
	}
	if input.WorkspaceID == "" {
		return WorkflowSyncInput{}, newWorkflowSyncValidationError("globaldb: workflow sync workspace id is required")
	}
	if input.WorkflowSlug == "" {
		return WorkflowSyncInput{}, newWorkflowSyncValidationError("globaldb: workflow sync slug is required")
	}
	if input.Kind != WorkflowKindOrdinary && input.Kind != WorkflowKindInitiative &&
		input.Kind != WorkflowKindTaskGroup {
		return WorkflowSyncInput{}, newWorkflowSyncValidationError(
			"globaldb: invalid workflow sync kind %q",
			input.Kind,
		)
	}
	if input.Kind == WorkflowKindTaskGroup {
		if input.TaskGroupID == "" {
			return WorkflowSyncInput{}, newWorkflowSyncValidationError(
				"globaldb: child workflow task group id is required",
			)
		}
		if input.ParentWorkflowID != "" {
			return WorkflowSyncInput{}, newWorkflowSyncValidationError(
				"globaldb: child workflow parent id is assigned by aggregate sync",
			)
		}
	} else if input.ParentWorkflowID != "" || input.TaskGroupID != "" {
		return WorkflowSyncInput{}, newWorkflowSyncValidationError(
			"globaldb: only child workflow sync may include task group identity",
		)
	}
	dependencies := make([]WorkflowDependency, 0, len(input.Dependencies))
	for _, dependency := range input.Dependencies {
		dependency.TaskGroupID = strings.TrimSpace(dependency.TaskGroupID)
		dependency.Rationale = strings.TrimSpace(dependency.Rationale)
		if dependency.TaskGroupID == "" || dependency.Rationale == "" {
			return WorkflowSyncInput{}, newWorkflowSyncValidationError(
				"globaldb: workflow dependency task group id and rationale are required",
			)
		}
		dependencies = append(dependencies, dependency)
	}
	input.Dependencies = dependencies
	return input, nil
}

func validateWorkflowSyncInput(input WorkflowSyncInput) error {
	_, err := normalizeWorkflowSyncInput(input)
	return err
}

// ReconcileAggregateWorkflowSync upserts one initiative and its readable
// task group children atomically. A missing child directory is represented by an
// omitted child plus PreserveMissingChildren, which intentionally suppresses
// stale-child pruning for that transaction.
func (g *GlobalDB) ReconcileAggregateWorkflowSync(
	ctx context.Context,
	input AggregateWorkflowSyncInput,
) (result AggregateWorkflowSyncResult, retErr error) {
	if err := g.requireContext(ctx, "reconcile aggregate workflow sync"); err != nil {
		return AggregateWorkflowSyncResult{}, err
	}
	parent, children, seenTaskGroupIDs, err := normalizeAggregateWorkflowSyncInput(input)
	if err != nil {
		return AggregateWorkflowSyncResult{}, err
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return AggregateWorkflowSyncResult{}, fmt.Errorf("globaldb: begin aggregate workflow sync: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("globaldb: rollback aggregate workflow sync: %w", rollbackErr))
		}
	}()

	parentSyncedAt := normalizeSyncTimestamp(parent.SyncedAt, g.now)
	result.Parent, err = g.reconcileWorkflowSyncTx(ctx, tx, parent, parentSyncedAt)
	if err != nil {
		return AggregateWorkflowSyncResult{}, err
	}
	for childIndex := range children {
		child := children[childIndex]
		child.ParentWorkflowID = result.Parent.Workflow.ID
		childSyncedAt := normalizeSyncTimestamp(child.SyncedAt, g.now)
		childResult, reconcileErr := g.reconcileWorkflowSyncTx(ctx, tx, child, childSyncedAt)
		if reconcileErr != nil {
			return AggregateWorkflowSyncResult{}, reconcileErr
		}
		result.Children = append(result.Children, childResult)
	}
	if !input.PreserveMissingChildren {
		pruned, skipped, pruneErr := g.pruneMissingChildRowsTx(ctx, tx, result.Parent.Workflow.ID, seenTaskGroupIDs)
		if pruneErr != nil {
			return AggregateWorkflowSyncResult{}, pruneErr
		}
		result.PrunedChildTaskGroupIDs = pruned
		result.SkippedChildPrunes = skipped
	}
	if err := tx.Commit(); err != nil {
		return AggregateWorkflowSyncResult{}, fmt.Errorf("globaldb: commit aggregate workflow sync: %w", err)
	}
	committed = true
	return result, nil
}

func normalizeAggregateWorkflowSyncInput(
	input AggregateWorkflowSyncInput,
) (WorkflowSyncInput, []WorkflowSyncInput, map[string]struct{}, error) {
	parent, err := normalizeWorkflowSyncInput(input.Parent)
	if err != nil {
		return WorkflowSyncInput{}, nil, nil, err
	}
	if parent.Kind != WorkflowKindInitiative {
		return WorkflowSyncInput{}, nil, nil, newWorkflowSyncValidationError(
			"globaldb: aggregate parent kind must be %q", WorkflowKindInitiative,
		)
	}
	children := make([]WorkflowSyncInput, 0, len(input.Children))
	seenTaskGroupIDs := make(map[string]struct{}, len(input.Children))
	for childIndex := range input.Children {
		child, err := normalizeWorkflowSyncInput(input.Children[childIndex])
		if err != nil {
			return WorkflowSyncInput{}, nil, nil, err
		}
		if err := validateAggregateWorkflowChild(parent, child, seenTaskGroupIDs); err != nil {
			return WorkflowSyncInput{}, nil, nil, err
		}
		seenTaskGroupIDs[child.TaskGroupID] = struct{}{}
		children = append(children, child)
	}
	return parent, children, seenTaskGroupIDs, nil
}

func validateAggregateWorkflowChild(
	parent WorkflowSyncInput,
	child WorkflowSyncInput,
	seenTaskGroupIDs map[string]struct{},
) error {
	if child.Kind != WorkflowKindTaskGroup {
		return newWorkflowSyncValidationError(
			"globaldb: aggregate child kind must be %q", WorkflowKindTaskGroup,
		)
	}
	if child.WorkspaceID != parent.WorkspaceID {
		return newWorkflowSyncValidationError(
			"globaldb: aggregate child %q has a different workspace", child.TaskGroupID,
		)
	}
	if _, exists := seenTaskGroupIDs[child.TaskGroupID]; exists {
		return newWorkflowSyncValidationError(
			"globaldb: duplicate aggregate child task group %q", child.TaskGroupID,
		)
	}
	return nil
}

func (g *GlobalDB) reconcileWorkflowSyncTx(
	ctx context.Context,
	tx *sql.Tx,
	input WorkflowSyncInput,
	syncedAt time.Time,
) (WorkflowSyncResult, error) {
	// Resolve MetadataOnlyIfExisting against this transaction's own snapshot, before
	// the row upsert inserts it (after which the row would always appear to exist).
	// The write transaction already holds the immediate WAL lock, so a concurrent
	// scoped sync that materialized this child either committed before this snapshot
	// (row visible -> preserve its projection) or is blocked until this transaction
	// finishes (row absent -> seed fresh). Deciding existence outside the transaction
	// is a TOCTOU race that reseeds empty projections over the concurrent sync's data.
	if input.MetadataOnlyIfExisting && !input.MetadataOnly {
		exists, err := workflowSlugExistsTx(ctx, tx, input.WorkspaceID, input.WorkflowSlug)
		if err != nil {
			return WorkflowSyncResult{}, err
		}
		input.MetadataOnly = exists
	}
	workflow, err := g.reconcileWorkflowInputRowTx(ctx, tx, input, syncedAt)
	if err != nil {
		return WorkflowSyncResult{}, err
	}
	// Only initiatives own task group children, so prune on every ordinary
	// reconciliation, not just the initiative->ordinary transition. A demotion
	// that skips an active child (kept because its run is still in flight) must be
	// retried once the run ends; gating on the transition would strand that child
	// permanently, since later syncs already observe the parent as ordinary.
	if input.Kind == WorkflowKindOrdinary {
		if _, _, pruneErr := g.pruneMissingChildRowsTx(ctx, tx, workflow.ID, map[string]struct{}{}); pruneErr != nil {
			return WorkflowSyncResult{}, pruneErr
		}
	}
	result := WorkflowSyncResult{Workflow: workflow}
	if input.MetadataOnly {
		// A metadata-only reconcile refreshes durable identity columns (for example
		// flipping Missing to true) without touching the retained artifact, task,
		// review, and checkpoint projections of a task group whose directory is
		// currently absent. The projections are re-collected only when the directory
		// returns and a full reconcile runs.
		return result, nil
	}
	if result.SnapshotsUpserted, err = g.reconcileArtifactSnapshotsTx(
		ctx,
		tx,
		workflow.ID,
		input.ArtifactSnapshots,
		syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}
	if result.TaskItemsUpserted, err = g.reconcileTaskItemsTx(
		ctx,
		tx,
		workflow.ID,
		input.TaskItems,
		syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}
	if result.ReviewRoundsUpserted, result.ReviewIssuesUpserted, err = g.reconcileReviewRoundsTx(
		ctx, tx, workflow.ID, input.ReviewRounds, syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}
	if result.CheckpointsUpdated, err = g.reconcileSyncCheckpointTx(
		ctx, tx, workflow.ID, input.CheckpointScope, input.CheckpointChecksum, syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}
	return result, nil
}

func normalizeSyncTimestamp(value time.Time, fallback func() time.Time) time.Time {
	if value.IsZero() {
		value = fallback()
	}
	return value.UTC()
}

// PruneMissingActiveWorkflows removes active workflow rows whose source
// directories were absent from a successful root sync.
func (g *GlobalDB) PruneMissingActiveWorkflows(
	ctx context.Context,
	workspaceID string,
	presentSlugs []string,
) (result WorkflowPruneResult, retErr error) {
	if err := g.requireContext(ctx, "prune missing active workflows"); err != nil {
		return WorkflowPruneResult{}, err
	}

	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if trimmedWorkspaceID == "" {
		return WorkflowPruneResult{}, errors.New("globaldb: workflow prune workspace id is required")
	}

	present := make(map[string]struct{}, len(presentSlugs))
	for _, slug := range presentSlugs {
		if trimmed := strings.TrimSpace(slug); trimmed != "" {
			present[trimmed] = struct{}{}
		}
	}

	workflows, err := g.ListWorkflows(ctx, ListWorkflowsOptions{WorkspaceID: trimmedWorkspaceID})
	if err != nil {
		return WorkflowPruneResult{}, fmt.Errorf(
			"globaldb: list workflows for prune workspace %q: %w",
			trimmedWorkspaceID,
			err,
		)
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkflowPruneResult{}, fmt.Errorf("globaldb: begin workflow prune: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("globaldb: rollback workflow prune: %w", rollbackErr))
		}
	}()

	result, err = pruneMissingActiveWorkflowRowsTx(ctx, tx, workflows, present)
	if err != nil {
		return WorkflowPruneResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return WorkflowPruneResult{}, fmt.Errorf("globaldb: commit workflow prune: %w", err)
	}
	committed = true
	return result, nil
}

func pruneMissingActiveWorkflowRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	workflows []Workflow,
	present map[string]struct{},
) (WorkflowPruneResult, error) {
	result := WorkflowPruneResult{
		PrunedSlugs: make([]string, 0),
		Skipped:     make([]WorkflowPruneSkipped, 0),
	}
	for workflowIndex := range workflows {
		workflow := &workflows[workflowIndex]
		if workflow.ParentWorkflowID != "" {
			continue
		}
		if _, ok := present[workflow.Slug]; ok {
			continue
		}

		activeRuns, err := countActiveRunsForWorkflowAggregateTx(ctx, tx, workflow.ID)
		if err != nil {
			return WorkflowPruneResult{}, err
		}
		if skipped, ok := workflowPruneActiveRunSkip(workflow.Slug, activeRuns); ok {
			result.Skipped = append(result.Skipped, skipped)
			continue
		}

		deleted, err := deleteActiveWorkflowAggregateTx(ctx, tx, workflow.ID)
		if err != nil {
			return WorkflowPruneResult{}, err
		}
		if deleted {
			result.PrunedSlugs = append(result.PrunedSlugs, workflow.Slug)
			continue
		}

		activeRuns, err = countActiveRunsForWorkflowAggregateTx(ctx, tx, workflow.ID)
		if err != nil {
			return WorkflowPruneResult{}, err
		}
		if skipped, ok := workflowPruneActiveRunSkip(workflow.Slug, activeRuns); ok {
			result.Skipped = append(result.Skipped, skipped)
		}
	}
	return result, nil
}

func workflowPruneActiveRunSkip(slug string, activeRuns int) (WorkflowPruneSkipped, bool) {
	if activeRuns <= 0 {
		return WorkflowPruneSkipped{}, false
	}
	return WorkflowPruneSkipped{
		Slug:       slug,
		Reason:     archiveReasonActiveRuns,
		ActiveRuns: activeRuns,
	}, true
}

func countActiveRunsForWorkflowAggregateTx(ctx context.Context, tx *sql.Tx, workflowID string) (int, error) {
	var count int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(1)
		 FROM runs
		 WHERE (workflow_id = ? OR workflow_id IN (
			SELECT id FROM workflows WHERE parent_workflow_id = ?
		 ))
		   AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')`,
		strings.TrimSpace(workflowID),
		strings.TrimSpace(workflowID),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("globaldb: count active aggregate runs for workflow %q: %w", workflowID, err)
	}
	return count, nil
}

func deleteActiveWorkflowAggregateTx(ctx context.Context, tx *sql.Tx, workflowID string) (bool, error) {
	trimmedWorkflowID := strings.TrimSpace(workflowID)
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM workflows AS child
		 WHERE child.parent_workflow_id = ?
		   AND EXISTS (
			SELECT 1 FROM workflows AS parent
			WHERE parent.id = ? AND parent.archived_at IS NULL
		   )
		   AND NOT EXISTS (
			SELECT 1 FROM runs
			WHERE (workflow_id = ? OR workflow_id IN (
				SELECT id FROM workflows WHERE parent_workflow_id = ?
			))
			  AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')
		   )`,
		trimmedWorkflowID,
		trimmedWorkflowID,
		trimmedWorkflowID,
		trimmedWorkflowID,
	); err != nil {
		return false, fmt.Errorf("globaldb: delete stale child workflows for %q: %w", trimmedWorkflowID, err)
	}

	result, err := tx.ExecContext(
		ctx,
		`DELETE FROM workflows
		 WHERE id = ? AND archived_at IS NULL
		   AND NOT EXISTS (
			SELECT 1 FROM runs
			WHERE (workflow_id = ? OR workflow_id IN (
				SELECT id FROM workflows WHERE parent_workflow_id = ?
			))
			  AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')
		   )`,
		trimmedWorkflowID,
		trimmedWorkflowID,
		trimmedWorkflowID,
	)
	if err != nil {
		return false, fmt.Errorf("globaldb: delete stale active workflow %q: %w", trimmedWorkflowID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globaldb: rows affected for stale workflow %q: %w", trimmedWorkflowID, err)
	}
	return affected > 0, nil
}

func (g *GlobalDB) reconcileWorkflowInputRowTx(
	ctx context.Context,
	tx *sql.Tx,
	input WorkflowSyncInput,
	syncedAt time.Time,
) (Workflow, error) {
	workflow, dependenciesJSON, err := normalizeWorkflow(Workflow{
		WorkspaceID:        input.WorkspaceID,
		Slug:               input.WorkflowSlug,
		Kind:               input.Kind,
		ParentWorkflowID:   input.ParentWorkflowID,
		TaskGroupID:        input.TaskGroupID,
		DisplayTitle:       input.DisplayTitle,
		Outcome:            input.Outcome,
		LifecycleCompleted: input.LifecycleCompleted,
		Missing:            input.Missing,
		Dependencies:       input.Dependencies,
		CreatedAt:          syncedAt,
		UpdatedAt:          syncedAt,
		LastSyncedAt:       &syncedAt,
	})
	if err != nil {
		return Workflow{}, err
	}
	existing, err := getActiveWorkflowBySlugTx(ctx, tx, workflow.WorkspaceID, workflow.Slug)
	if err == nil {
		return updateWorkflowSyncRowTx(ctx, tx, existing, workflow, dependenciesJSON, syncedAt)
	}
	if !errors.Is(err, ErrWorkflowNotFound) {
		return Workflow{}, err
	}
	return g.insertWorkflowSyncRowTx(ctx, tx, workflow, dependenciesJSON, syncedAt)
}

func updateWorkflowSyncRowTx(
	ctx context.Context,
	tx *sql.Tx,
	existing Workflow,
	workflow Workflow,
	dependenciesJSON string,
	syncedAt time.Time,
) (Workflow, error) {
	existing.Kind = workflow.Kind
	existing.ParentWorkflowID = workflow.ParentWorkflowID
	existing.TaskGroupID = workflow.TaskGroupID
	existing.DisplayTitle = workflow.DisplayTitle
	existing.Outcome = workflow.Outcome
	existing.LifecycleCompleted = workflow.LifecycleCompleted
	existing.Missing = workflow.Missing
	existing.Dependencies = workflow.Dependencies
	existing.LastSyncedAt = &syncedAt
	existing.UpdatedAt = syncedAt
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE workflows
		 SET kind = ?, parent_workflow_id = ?, task_group_id = ?, display_title = ?, outcome = ?,
		     lifecycle_completed = ?, missing = ?, dependencies_json = ?, last_synced_at = ?, updated_at = ?
		 WHERE id = ?`,
		existing.Kind,
		store.NullableString(existing.ParentWorkflowID),
		existing.TaskGroupID,
		existing.DisplayTitle,
		existing.Outcome,
		existing.LifecycleCompleted,
		existing.Missing,
		dependenciesJSON,
		store.FormatTimestamp(syncedAt),
		store.FormatTimestamp(syncedAt),
		existing.ID,
	); err != nil {
		return Workflow{}, fmt.Errorf("globaldb: update workflow sync state %q: %w", existing.ID, err)
	}
	return existing, nil
}

func (g *GlobalDB) insertWorkflowSyncRowTx(
	ctx context.Context,
	tx *sql.Tx,
	workflow Workflow,
	dependenciesJSON string,
	syncedAt time.Time,
) (Workflow, error) {
	workflow.ID = g.newID("wf")
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO workflows (
			id, workspace_id, slug, kind, parent_workflow_id, task_group_id,
			display_title, outcome, lifecycle_completed, missing, dependencies_json,
			archived_at, last_synced_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workflow.ID,
		workflow.WorkspaceID,
		workflow.Slug,
		workflow.Kind,
		store.NullableString(workflow.ParentWorkflowID),
		workflow.TaskGroupID,
		workflow.DisplayTitle,
		workflow.Outcome,
		workflow.LifecycleCompleted,
		workflow.Missing,
		dependenciesJSON,
		nil,
		store.FormatTimestamp(syncedAt),
		store.FormatTimestamp(workflow.CreatedAt),
		store.FormatTimestamp(workflow.UpdatedAt),
	); err != nil {
		if isWorkflowSlugConflict(err) {
			return getActiveWorkflowBySlugTx(ctx, tx, workflow.WorkspaceID, workflow.Slug)
		}
		return Workflow{}, fmt.Errorf("globaldb: insert workflow sync row %q: %w", workflow.ID, err)
	}
	return workflow, nil
}

func (g *GlobalDB) reconcileWorkflowRowTx(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID string,
	slug string,
	syncedAt time.Time,
) (Workflow, error) {
	input, err := normalizeWorkflowSyncInput(WorkflowSyncInput{
		WorkspaceID:  workspaceID,
		WorkflowSlug: slug,
		SyncedAt:     syncedAt,
	})
	if err != nil {
		return Workflow{}, err
	}
	return g.reconcileWorkflowInputRowTx(ctx, tx, input, syncedAt)
}

// workflowSlugExistsTx reports whether an active workflow row for the slug is
// visible to tx, mapping ErrWorkflowNotFound to a plain false so callers can
// branch on presence without treating absence as an error.
func workflowSlugExistsTx(ctx context.Context, tx *sql.Tx, workspaceID string, slug string) (bool, error) {
	if _, err := getActiveWorkflowBySlugTx(ctx, tx, workspaceID, slug); err != nil {
		if errors.Is(err, ErrWorkflowNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getActiveWorkflowBySlugTx(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID string,
	slug string,
) (Workflow, error) {
	row := tx.QueryRowContext(
		ctx,
		`SELECT `+workflowSelectColumns+`
		 FROM workflows
		 WHERE workspace_id = ? AND slug = ? AND archived_at IS NULL`,
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(slug),
	)
	workflow, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, ErrWorkflowNotFound
		}
		return Workflow{}, fmt.Errorf("globaldb: query active workflow %q: %w", strings.TrimSpace(slug), err)
	}
	return workflow, nil
}

func (g *GlobalDB) pruneMissingChildRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	parentWorkflowID string,
	presentTaskGroupIDs map[string]struct{},
) ([]string, []WorkflowPruneSkipped, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, task_group_id, slug
		 FROM workflows
		 WHERE parent_workflow_id = ? AND archived_at IS NULL
		 ORDER BY task_group_id ASC, id ASC`,
		strings.TrimSpace(parentWorkflowID),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("globaldb: query stale child workflows: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	pruned := make([]string, 0)
	skipped := make([]WorkflowPruneSkipped, 0)
	for rows.Next() {
		var childID, taskGroupID, slug string
		if err := rows.Scan(&childID, &taskGroupID, &slug); err != nil {
			return nil, nil, fmt.Errorf("globaldb: scan stale child workflow: %w", err)
		}
		if _, present := presentTaskGroupIDs[taskGroupID]; present {
			continue
		}
		activeRuns, countErr := countActiveRunsForWorkflowTx(ctx, tx, childID)
		if countErr != nil {
			return nil, nil, countErr
		}
		if skippedRow, active := workflowPruneActiveRunSkip(slug, activeRuns); active {
			skipped = append(skipped, skippedRow)
			continue
		}
		if _, deleteErr := tx.ExecContext(
			ctx,
			`DELETE FROM workflows
			 WHERE id = ? AND archived_at IS NULL
			   AND NOT EXISTS (
				SELECT 1 FROM runs
				WHERE runs.workflow_id = workflows.id
				  AND runs.status NOT IN ('completed', 'failed', 'canceled', 'crashed')
			   )`,
			childID,
		); deleteErr != nil {
			return nil, nil, fmt.Errorf("globaldb: prune stale child workflow %q: %w", taskGroupID, deleteErr)
		}
		pruned = append(pruned, taskGroupID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("globaldb: iterate stale child workflows: %w", err)
	}
	return pruned, skipped, nil
}

func countActiveRunsForWorkflowTx(ctx context.Context, tx *sql.Tx, workflowID string) (int, error) {
	var count int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM runs
		 WHERE workflow_id = ?
		   AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')`,
		strings.TrimSpace(workflowID),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("globaldb: count active child runs for workflow %q: %w", workflowID, err)
	}
	return count, nil
}

func (g *GlobalDB) reconcileArtifactSnapshotsTx(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
	snapshots []ArtifactSnapshotInput,
	syncedAt time.Time,
) (int, error) {
	existing, err := loadExistingArtifactSnapshots(ctx, tx, workflowID)
	if err != nil {
		return 0, err
	}
	stmts, err := prepareArtifactSnapshotStatements(ctx, tx)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = stmts.close()
	}()

	seen := make(map[string]struct{}, len(snapshots))
	for _, input := range snapshots {
		prepared, key, err := prepareArtifactSnapshot(input)
		if err != nil {
			return 0, err
		}
		if _, duplicate := seen[key]; duplicate {
			return 0, fmt.Errorf("globaldb: duplicate artifact snapshot %q", key)
		}
		seen[key] = struct{}{}

		if current, ok := existing[key]; ok &&
			current.Checksum == prepared.Checksum &&
			current.BodyStorageKind != artifactBodyOverflowKind {
			prepared.BodyText = current.BodyText
			prepared.BodyStorageKind = current.BodyStorageKind
		}

		if err := upsertArtifactBody(ctx, stmts.upsertBody, prepared, syncedAt); err != nil {
			return 0, err
		}

		if _, err := stmts.upsert.ExecContext(
			ctx,
			workflowID,
			prepared.ArtifactKind,
			prepared.RelativePath,
			prepared.Checksum,
			prepared.FrontmatterJSON,
			store.NullableString(prepared.BodyText),
			prepared.BodyStorageKind,
			store.FormatTimestamp(prepared.SourceMTime),
			store.FormatTimestamp(syncedAt),
		); err != nil {
			return 0, fmt.Errorf(
				"globaldb: upsert artifact snapshot %s/%s: %w",
				prepared.ArtifactKind,
				prepared.RelativePath,
				err,
			)
		}
	}
	if err := deleteStaleArtifactSnapshots(ctx, stmts.delete, workflowID, existing, seen); err != nil {
		return 0, err
	}
	if err := cleanupUnreferencedArtifactBodies(ctx, tx); err != nil {
		return 0, err
	}

	return len(snapshots), nil
}

func upsertArtifactBody(
	ctx context.Context,
	stmt *sql.Stmt,
	prepared preparedArtifactSnapshot,
	syncedAt time.Time,
) error {
	if prepared.BodyStorageKind != artifactBodyBlobKind {
		return nil
	}
	if _, err := stmt.ExecContext(
		ctx,
		prepared.Checksum,
		prepared.BodyBlobText,
		len([]byte(prepared.BodyBlobText)),
		store.FormatTimestamp(syncedAt),
	); err != nil {
		return fmt.Errorf(
			"globaldb: upsert artifact body %s/%s: %w",
			prepared.ArtifactKind,
			prepared.RelativePath,
			err,
		)
	}
	return nil
}

type artifactSnapshotStatements struct {
	upsert     *sql.Stmt
	upsertBody *sql.Stmt
	delete     *sql.Stmt
}

func prepareArtifactSnapshotStatements(ctx context.Context, tx *sql.Tx) (artifactSnapshotStatements, error) {
	upsert, err := tx.PrepareContext(
		ctx,
		`INSERT INTO artifact_snapshots (
			workflow_id, artifact_kind, relative_path, checksum, frontmatter_json,
			body_text, body_storage_kind, source_mtime, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, artifact_kind, relative_path) DO UPDATE SET
			checksum = excluded.checksum,
			frontmatter_json = excluded.frontmatter_json,
			body_text = excluded.body_text,
			body_storage_kind = excluded.body_storage_kind,
			source_mtime = excluded.source_mtime,
			synced_at = excluded.synced_at`,
	)
	if err != nil {
		return artifactSnapshotStatements{}, fmt.Errorf("globaldb: prepare artifact snapshot upsert: %w", err)
	}

	upsertBody, err := tx.PrepareContext(
		ctx,
		`INSERT INTO artifact_bodies (checksum, body_text, size_bytes, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(checksum) DO UPDATE SET
			body_text = excluded.body_text,
			size_bytes = excluded.size_bytes`,
	)
	if err != nil {
		_ = upsert.Close()
		return artifactSnapshotStatements{}, fmt.Errorf("globaldb: prepare artifact body upsert: %w", err)
	}

	deleteStmt, err := tx.PrepareContext(
		ctx,
		`DELETE FROM artifact_snapshots
		 WHERE workflow_id = ? AND artifact_kind = ? AND relative_path = ?`,
	)
	if err != nil {
		_ = upsert.Close()
		_ = upsertBody.Close()
		return artifactSnapshotStatements{}, fmt.Errorf("globaldb: prepare artifact snapshot delete: %w", err)
	}

	return artifactSnapshotStatements{upsert: upsert, upsertBody: upsertBody, delete: deleteStmt}, nil
}

func (s artifactSnapshotStatements) close() error {
	return closeSQLStatements(s.upsert, s.upsertBody, s.delete)
}

func deleteStaleArtifactSnapshots(
	ctx context.Context,
	deleteStmt *sql.Stmt,
	workflowID string,
	existing map[string]existingArtifactSnapshot,
	seen map[string]struct{},
) error {
	for key := range existing {
		if _, ok := seen[key]; ok {
			continue
		}
		artifactKind, relativePath := splitArtifactKey(key)
		if _, err := deleteStmt.ExecContext(ctx, workflowID, artifactKind, relativePath); err != nil {
			return fmt.Errorf("globaldb: delete stale artifact snapshot %s: %w", key, err)
		}
	}
	return nil
}

func cleanupUnreferencedArtifactBodies(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM artifact_bodies
		 WHERE NOT EXISTS (
			SELECT 1
			FROM artifact_snapshots snapshots
			WHERE snapshots.checksum = artifact_bodies.checksum
			  AND snapshots.body_storage_kind = ?
		 )`,
		artifactBodyBlobKind,
	); err != nil {
		return fmt.Errorf("globaldb: delete unreferenced artifact bodies: %w", err)
	}
	return nil
}

func loadExistingArtifactSnapshots(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
) (map[string]existingArtifactSnapshot, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT artifact_kind, relative_path, checksum, body_text, body_storage_kind
		 FROM artifact_snapshots
		 WHERE workflow_id = ?`,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query artifact snapshots for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make(map[string]existingArtifactSnapshot)
	for rows.Next() {
		var (
			artifactKind    string
			relativePath    string
			checksum        string
			bodyText        sql.NullString
			bodyStorageKind string
		)
		if err := rows.Scan(&artifactKind, &relativePath, &checksum, &bodyText, &bodyStorageKind); err != nil {
			return nil, fmt.Errorf("globaldb: scan artifact snapshot: %w", err)
		}
		out[artifactKey(artifactKind, relativePath)] = existingArtifactSnapshot{
			Checksum:        checksum,
			BodyText:        bodyText.String,
			BodyStorageKind: strings.TrimSpace(bodyStorageKind),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate artifact snapshots: %w", err)
	}
	return out, nil
}

type preparedArtifactSnapshot struct {
	ArtifactKind    string
	RelativePath    string
	Checksum        string
	FrontmatterJSON string
	BodyText        string
	BodyBlobText    string
	BodyStorageKind string
	SourceMTime     time.Time
}

func prepareArtifactSnapshot(input ArtifactSnapshotInput) (preparedArtifactSnapshot, string, error) {
	artifactKind := strings.TrimSpace(input.ArtifactKind)
	relativePath := strings.TrimSpace(input.RelativePath)
	checksum := strings.TrimSpace(input.Checksum)
	if artifactKind == "" {
		return preparedArtifactSnapshot{}, "", newWorkflowSyncValidationError("globaldb: artifact kind is required")
	}
	if relativePath == "" {
		return preparedArtifactSnapshot{}, "", newWorkflowSyncValidationError(
			"globaldb: artifact relative path is required",
		)
	}
	if checksum == "" {
		return preparedArtifactSnapshot{}, "", newWorkflowSyncValidationError(
			"globaldb: artifact checksum is required for %s/%s",
			artifactKind,
			relativePath,
		)
	}

	bodyStorageKind := artifactBodyInlineKind
	bodyText := input.BodyText
	bodyBlobText := ""
	if len([]byte(bodyText)) > artifactBodyLimitBytes {
		bodyStorageKind = artifactBodyBlobKind
		bodyBlobText = bodyText
		bodyText = ""
	}

	frontmatterJSON := strings.TrimSpace(input.FrontmatterJSON)
	if frontmatterJSON == "" {
		frontmatterJSON = "{}"
	}
	if input.SourceMTime.IsZero() {
		return preparedArtifactSnapshot{}, "", newWorkflowSyncValidationError(
			"globaldb: artifact source mtime is required for %s/%s",
			artifactKind,
			relativePath,
		)
	}

	prepared := preparedArtifactSnapshot{
		ArtifactKind:    artifactKind,
		RelativePath:    relativePath,
		Checksum:        checksum,
		FrontmatterJSON: frontmatterJSON,
		BodyText:        bodyText,
		BodyBlobText:    bodyBlobText,
		BodyStorageKind: bodyStorageKind,
		SourceMTime:     input.SourceMTime.UTC(),
	}
	return prepared, artifactKey(artifactKind, relativePath), nil
}

func artifactKey(artifactKind string, relativePath string) string {
	return strings.TrimSpace(artifactKind) + "\x00" + strings.TrimSpace(relativePath)
}

func splitArtifactKey(key string) (artifactKind string, relativePath string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}

func (g *GlobalDB) reconcileTaskItemsTx(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
	items []TaskItemInput,
	syncedAt time.Time,
) (int, error) {
	existing, err := loadExistingTaskItemIDs(ctx, tx, workflowID)
	if err != nil {
		return 0, err
	}
	stmts, err := prepareTaskItemStatements(ctx, tx)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = stmts.close()
	}()

	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		prepared, err := prepareTaskItem(item)
		if err != nil {
			return 0, err
		}
		if _, duplicate := seen[prepared.TaskNumber]; duplicate {
			return 0, fmt.Errorf("globaldb: duplicate task number %d", prepared.TaskNumber)
		}
		seen[prepared.TaskNumber] = struct{}{}

		id := existing[prepared.TaskNumber]
		if id == "" {
			id = g.newID("task")
		}

		dependsOnJSON, err := marshalJSONArray(prepared.DependsOn)
		if err != nil {
			return 0, err
		}

		if _, err := stmts.upsert.ExecContext(
			ctx,
			id,
			workflowID,
			prepared.TaskNumber,
			prepared.TaskID,
			prepared.Title,
			prepared.Status,
			prepared.Kind,
			dependsOnJSON,
			prepared.SourcePath,
			store.FormatTimestamp(syncedAt),
		); err != nil {
			return 0, fmt.Errorf("globaldb: upsert task item %d: %w", prepared.TaskNumber, err)
		}
	}
	if err := deleteStaleTaskItems(ctx, stmts.delete, workflowID, existing, seen); err != nil {
		return 0, err
	}

	return len(items), nil
}

type taskItemStatements struct {
	upsert *sql.Stmt
	delete *sql.Stmt
}

func prepareTaskItemStatements(ctx context.Context, tx *sql.Tx) (taskItemStatements, error) {
	upsert, err := tx.PrepareContext(
		ctx,
		`INSERT INTO task_items (
			id, workflow_id, task_number, task_id, title, status, kind, depends_on_json, source_path, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, task_number) DO UPDATE SET
			task_id = excluded.task_id,
			title = excluded.title,
			status = excluded.status,
			kind = excluded.kind,
			depends_on_json = excluded.depends_on_json,
			source_path = excluded.source_path,
			updated_at = excluded.updated_at`,
	)
	if err != nil {
		return taskItemStatements{}, fmt.Errorf("globaldb: prepare task item upsert: %w", err)
	}

	deleteStmt, err := tx.PrepareContext(ctx, `DELETE FROM task_items WHERE workflow_id = ? AND task_number = ?`)
	if err != nil {
		_ = upsert.Close()
		return taskItemStatements{}, fmt.Errorf("globaldb: prepare task item delete: %w", err)
	}

	return taskItemStatements{upsert: upsert, delete: deleteStmt}, nil
}

func (s taskItemStatements) close() error {
	return closeSQLStatements(s.upsert, s.delete)
}

func deleteStaleTaskItems(
	ctx context.Context,
	deleteStmt *sql.Stmt,
	workflowID string,
	existing map[int]string,
	seen map[int]struct{},
) error {
	for taskNumber := range existing {
		if _, ok := seen[taskNumber]; ok {
			continue
		}
		if _, err := deleteStmt.ExecContext(ctx, workflowID, taskNumber); err != nil {
			return fmt.Errorf("globaldb: delete stale task item %d: %w", taskNumber, err)
		}
	}
	return nil
}

func loadExistingTaskItemIDs(ctx context.Context, tx *sql.Tx, workflowID string) (map[int]string, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, task_number
		 FROM task_items
		 WHERE workflow_id = ?`,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query task items for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make(map[int]string)
	for rows.Next() {
		var (
			id         string
			taskNumber int
		)
		if err := rows.Scan(&id, &taskNumber); err != nil {
			return nil, fmt.Errorf("globaldb: scan task item: %w", err)
		}
		out[taskNumber] = strings.TrimSpace(id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate task items: %w", err)
	}
	return out, nil
}

func prepareTaskItem(input TaskItemInput) (TaskItemInput, error) {
	if input.TaskNumber <= 0 {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task number must be positive (got %d)",
			input.TaskNumber,
		)
	}
	if strings.TrimSpace(input.TaskID) == "" {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task id is required for task %d",
			input.TaskNumber,
		)
	}
	if strings.TrimSpace(input.Title) == "" {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task title is required for task %d",
			input.TaskNumber,
		)
	}
	if strings.TrimSpace(input.Status) == "" {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task status is required for task %d",
			input.TaskNumber,
		)
	}
	if strings.TrimSpace(input.Kind) == "" {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task kind is required for task %d",
			input.TaskNumber,
		)
	}
	if strings.TrimSpace(input.SourcePath) == "" {
		return TaskItemInput{}, newWorkflowSyncValidationError(
			"globaldb: task source path is required for task %d",
			input.TaskNumber,
		)
	}

	input.TaskID = strings.TrimSpace(input.TaskID)
	input.Title = strings.TrimSpace(input.Title)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Kind = strings.TrimSpace(input.Kind)
	input.SourcePath = strings.TrimSpace(input.SourcePath)
	return input, nil
}

func (g *GlobalDB) reconcileReviewRoundsTx(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
	rounds []ReviewRoundInput,
	syncedAt time.Time,
) (int, int, error) {
	existingRoundIDs, err := loadExistingReviewRoundIDs(ctx, tx, workflowID)
	if err != nil {
		return 0, 0, err
	}
	stmts, err := prepareReviewRoundStatements(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = stmts.close()
	}()

	seenRounds := make(map[int]struct{}, len(rounds))
	totalIssues := 0
	for _, round := range rounds {
		prepared, err := prepareReviewRound(round)
		if err != nil {
			return 0, 0, err
		}
		if _, duplicate := seenRounds[prepared.RoundNumber]; duplicate {
			return 0, 0, fmt.Errorf("globaldb: duplicate review round %d", prepared.RoundNumber)
		}
		seenRounds[prepared.RoundNumber] = struct{}{}

		roundID := existingRoundIDs[prepared.RoundNumber]
		if roundID == "" {
			roundID = g.newID("rr")
		}

		if _, err := stmts.upsert.ExecContext(
			ctx,
			roundID,
			workflowID,
			prepared.RoundNumber,
			prepared.Provider,
			prepared.PRRef,
			prepared.ResolvedCount,
			prepared.UnresolvedCount,
			store.FormatTimestamp(syncedAt),
		); err != nil {
			return 0, 0, fmt.Errorf("globaldb: upsert review round %d: %w", prepared.RoundNumber, err)
		}

		issueCount, err := g.reconcileReviewIssuesTx(ctx, tx, roundID, prepared.Issues, syncedAt)
		if err != nil {
			return 0, 0, err
		}
		totalIssues += issueCount
	}
	if err := deleteStaleReviewRounds(ctx, stmts.delete, existingRoundIDs, seenRounds); err != nil {
		return 0, 0, err
	}

	return len(rounds), totalIssues, nil
}

type reviewRoundStatements struct {
	upsert *sql.Stmt
	delete *sql.Stmt
}

func prepareReviewRoundStatements(ctx context.Context, tx *sql.Tx) (reviewRoundStatements, error) {
	upsert, err := tx.PrepareContext(
		ctx,
		`INSERT INTO review_rounds (
			id, workflow_id, round_number, provider, pr_ref, resolved_count, unresolved_count, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, round_number) DO UPDATE SET
			provider = excluded.provider,
			pr_ref = excluded.pr_ref,
			resolved_count = excluded.resolved_count,
			unresolved_count = excluded.unresolved_count,
			updated_at = excluded.updated_at`,
	)
	if err != nil {
		return reviewRoundStatements{}, fmt.Errorf("globaldb: prepare review round upsert: %w", err)
	}

	deleteStmt, err := tx.PrepareContext(ctx, `DELETE FROM review_rounds WHERE id = ?`)
	if err != nil {
		_ = upsert.Close()
		return reviewRoundStatements{}, fmt.Errorf("globaldb: prepare review round delete: %w", err)
	}

	return reviewRoundStatements{upsert: upsert, delete: deleteStmt}, nil
}

func (s reviewRoundStatements) close() error {
	return closeSQLStatements(s.upsert, s.delete)
}

func deleteStaleReviewRounds(
	ctx context.Context,
	deleteStmt *sql.Stmt,
	existing map[int]string,
	seen map[int]struct{},
) error {
	for roundNumber, roundID := range existing {
		if _, ok := seen[roundNumber]; ok {
			continue
		}
		if _, err := deleteStmt.ExecContext(ctx, roundID); err != nil {
			return fmt.Errorf("globaldb: delete stale review round %d: %w", roundNumber, err)
		}
	}
	return nil
}

func closeSQLStatements(statements ...*sql.Stmt) error {
	var err error
	for _, stmt := range statements {
		if stmt == nil {
			continue
		}
		if closeErr := stmt.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}

func loadExistingReviewRoundIDs(ctx context.Context, tx *sql.Tx, workflowID string) (map[int]string, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, round_number
		 FROM review_rounds
		 WHERE workflow_id = ?`,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query review rounds for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make(map[int]string)
	for rows.Next() {
		var (
			id          string
			roundNumber int
		)
		if err := rows.Scan(&id, &roundNumber); err != nil {
			return nil, fmt.Errorf("globaldb: scan review round: %w", err)
		}
		out[roundNumber] = strings.TrimSpace(id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate review rounds: %w", err)
	}
	return out, nil
}

func prepareReviewRound(input ReviewRoundInput) (ReviewRoundInput, error) {
	if input.RoundNumber <= 0 {
		return ReviewRoundInput{}, newWorkflowSyncValidationError(
			"globaldb: review round must be positive (got %d)",
			input.RoundNumber,
		)
	}
	if input.ResolvedCount < 0 || input.UnresolvedCount < 0 {
		return ReviewRoundInput{}, newWorkflowSyncValidationError(
			"globaldb: review round counts must be non-negative for round %d",
			input.RoundNumber,
		)
	}
	input.Provider = strings.TrimSpace(input.Provider)
	input.PRRef = strings.TrimSpace(input.PRRef)
	return input, nil
}

func (g *GlobalDB) reconcileReviewIssuesTx(
	ctx context.Context,
	tx *sql.Tx,
	roundID string,
	issues []ReviewIssueInput,
	syncedAt time.Time,
) (int, error) {
	existingIssueIDs, err := loadExistingReviewIssueIDs(ctx, tx, roundID)
	if err != nil {
		return 0, err
	}
	upsertStmt, err := tx.PrepareContext(
		ctx,
		`INSERT INTO review_issues (
			id, round_id, issue_number, severity, status, source_path, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(round_id, issue_number) DO UPDATE SET
			severity = excluded.severity,
			status = excluded.status,
			source_path = excluded.source_path,
			updated_at = excluded.updated_at`,
	)
	if err != nil {
		return 0, fmt.Errorf("globaldb: prepare review issue upsert: %w", err)
	}
	defer func() {
		_ = upsertStmt.Close()
	}()
	deleteStmt, err := tx.PrepareContext(ctx, `DELETE FROM review_issues WHERE round_id = ? AND issue_number = ?`)
	if err != nil {
		return 0, fmt.Errorf("globaldb: prepare review issue delete: %w", err)
	}
	defer func() {
		_ = deleteStmt.Close()
	}()

	seenIssues := make(map[int]struct{}, len(issues))
	for _, issue := range issues {
		prepared, err := prepareReviewIssue(issue)
		if err != nil {
			return 0, err
		}
		if _, duplicate := seenIssues[prepared.IssueNumber]; duplicate {
			return 0, fmt.Errorf("globaldb: duplicate review issue %d", prepared.IssueNumber)
		}
		seenIssues[prepared.IssueNumber] = struct{}{}

		issueID := existingIssueIDs[prepared.IssueNumber]
		if issueID == "" {
			issueID = g.newID("ri")
		}

		if _, err := upsertStmt.ExecContext(
			ctx,
			issueID,
			roundID,
			prepared.IssueNumber,
			prepared.Severity,
			prepared.Status,
			prepared.SourcePath,
			store.FormatTimestamp(syncedAt),
		); err != nil {
			return 0, fmt.Errorf("globaldb: upsert review issue %d: %w", prepared.IssueNumber, err)
		}
	}

	for issueNumber := range existingIssueIDs {
		if _, ok := seenIssues[issueNumber]; ok {
			continue
		}
		if _, err := deleteStmt.ExecContext(
			ctx,
			roundID,
			issueNumber,
		); err != nil {
			return 0, fmt.Errorf("globaldb: delete stale review issue %d: %w", issueNumber, err)
		}
	}

	return len(issues), nil
}

func loadExistingReviewIssueIDs(ctx context.Context, tx *sql.Tx, roundID string) (map[int]string, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, issue_number
		 FROM review_issues
		 WHERE round_id = ?`,
		strings.TrimSpace(roundID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query review issues for round %q: %w", roundID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make(map[int]string)
	for rows.Next() {
		var (
			id          string
			issueNumber int
		)
		if err := rows.Scan(&id, &issueNumber); err != nil {
			return nil, fmt.Errorf("globaldb: scan review issue: %w", err)
		}
		out[issueNumber] = strings.TrimSpace(id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate review issues: %w", err)
	}
	return out, nil
}

func prepareReviewIssue(input ReviewIssueInput) (ReviewIssueInput, error) {
	if input.IssueNumber <= 0 {
		return ReviewIssueInput{}, newWorkflowSyncValidationError(
			"globaldb: review issue number must be positive (got %d)",
			input.IssueNumber,
		)
	}
	if strings.TrimSpace(input.Status) == "" {
		return ReviewIssueInput{}, newWorkflowSyncValidationError(
			"globaldb: review issue status is required for issue %d",
			input.IssueNumber,
		)
	}
	if strings.TrimSpace(input.SourcePath) == "" {
		return ReviewIssueInput{}, newWorkflowSyncValidationError(
			"globaldb: review issue source path is required for issue %d",
			input.IssueNumber,
		)
	}
	input.Severity = strings.TrimSpace(input.Severity)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.SourcePath = strings.TrimSpace(input.SourcePath)
	return input, nil
}

func (g *GlobalDB) reconcileSyncCheckpointTx(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
	scope string,
	checksum string,
	syncedAt time.Time,
) (int, error) {
	trimmedScope := strings.TrimSpace(scope)
	if trimmedScope == "" {
		trimmedScope = defaultSyncScope
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO sync_checkpoints (
			workflow_id, scope, checksum, last_scan_at, last_success_at, last_error_text
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, scope) DO UPDATE SET
			checksum = excluded.checksum,
			last_scan_at = excluded.last_scan_at,
			last_success_at = excluded.last_success_at,
			last_error_text = ''`,
		workflowID,
		trimmedScope,
		strings.TrimSpace(checksum),
		store.FormatTimestamp(syncedAt),
		store.FormatTimestamp(syncedAt),
		"",
	); err != nil {
		return 0, fmt.Errorf("globaldb: upsert sync checkpoint %q: %w", trimmedScope, err)
	}
	return 1, nil
}

func marshalJSONArray(values []string) (string, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("globaldb: marshal json array: %w", err)
	}
	return string(encoded), nil
}
