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
	artifactBodyOverflowKind = "overflow"
	artifactBodyLimitBytes   = 256 * 1024
	defaultSyncScope         = "workflow"
)

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
	SyncedAt           time.Time
	CheckpointScope    string
	CheckpointChecksum string
	ArtifactSnapshots  []ArtifactSnapshotInput
	TaskItems          []TaskItemInput
	ReviewRounds       []ReviewRoundInput
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
	if err := validateWorkflowSyncInput(input); err != nil {
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

	workflow, err := g.reconcileWorkflowRowTx(ctx, tx, input.WorkspaceID, input.WorkflowSlug, syncedAt)
	if err != nil {
		return WorkflowSyncResult{}, err
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
		ctx,
		tx,
		workflow.ID,
		input.ReviewRounds,
		syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}
	if result.CheckpointsUpdated, err = g.reconcileSyncCheckpointTx(
		ctx,
		tx,
		workflow.ID,
		input.CheckpointScope,
		input.CheckpointChecksum,
		syncedAt,
	); err != nil {
		return WorkflowSyncResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return WorkflowSyncResult{}, fmt.Errorf("globaldb: commit workflow sync: %w", err)
	}
	committed = true

	result.Workflow = workflow
	return result, nil
}

func validateWorkflowSyncInput(input WorkflowSyncInput) error {
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return errors.New("globaldb: workflow sync workspace id is required")
	}
	if strings.TrimSpace(input.WorkflowSlug) == "" {
		return errors.New("globaldb: workflow sync slug is required")
	}
	return nil
}

func normalizeSyncTimestamp(value time.Time, fallback func() time.Time) time.Time {
	if value.IsZero() {
		value = fallback()
	}
	return value.UTC()
}

func (g *GlobalDB) reconcileWorkflowRowTx(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID string,
	slug string,
	syncedAt time.Time,
) (Workflow, error) {
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	trimmedSlug := strings.TrimSpace(slug)

	workflow, err := getActiveWorkflowBySlugTx(ctx, tx, trimmedWorkspaceID, trimmedSlug)
	if err == nil {
		workflow.LastSyncedAt = &syncedAt
		workflow.UpdatedAt = syncedAt
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE workflows
			 SET last_synced_at = ?, updated_at = ?
			 WHERE id = ?`,
			store.FormatTimestamp(syncedAt),
			store.FormatTimestamp(syncedAt),
			workflow.ID,
		); err != nil {
			return Workflow{}, fmt.Errorf("globaldb: update workflow sync state %q: %w", workflow.ID, err)
		}
		return workflow, nil
	}
	if !errors.Is(err, ErrWorkflowNotFound) {
		return Workflow{}, err
	}

	workflow = Workflow{
		ID:           g.newID("wf"),
		WorkspaceID:  trimmedWorkspaceID,
		Slug:         trimmedSlug,
		CreatedAt:    syncedAt,
		UpdatedAt:    syncedAt,
		LastSyncedAt: &syncedAt,
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO workflows (
			id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workflow.ID,
		workflow.WorkspaceID,
		workflow.Slug,
		nil,
		store.FormatTimestamp(syncedAt),
		store.FormatTimestamp(workflow.CreatedAt),
		store.FormatTimestamp(workflow.UpdatedAt),
	); err != nil {
		if isWorkflowSlugConflict(err) {
			return getActiveWorkflowBySlugTx(ctx, tx, trimmedWorkspaceID, trimmedSlug)
		}
		return Workflow{}, fmt.Errorf("globaldb: insert workflow sync row %q: %w", workflow.ID, err)
	}
	return workflow, nil
}

func getActiveWorkflowBySlugTx(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID string,
	slug string,
) (Workflow, error) {
	row := tx.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
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

		if current, ok := existing[key]; ok && current.Checksum == prepared.Checksum {
			prepared.BodyText = current.BodyText
			prepared.BodyStorageKind = current.BodyStorageKind
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

	return len(snapshots), nil
}

type artifactSnapshotStatements struct {
	upsert *sql.Stmt
	delete *sql.Stmt
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

	deleteStmt, err := tx.PrepareContext(
		ctx,
		`DELETE FROM artifact_snapshots
		 WHERE workflow_id = ? AND artifact_kind = ? AND relative_path = ?`,
	)
	if err != nil {
		_ = upsert.Close()
		return artifactSnapshotStatements{}, fmt.Errorf("globaldb: prepare artifact snapshot delete: %w", err)
	}

	return artifactSnapshotStatements{upsert: upsert, delete: deleteStmt}, nil
}

func (s artifactSnapshotStatements) close() error {
	return closeSQLStatements(s.upsert, s.delete)
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
	BodyStorageKind string
	SourceMTime     time.Time
}

func prepareArtifactSnapshot(input ArtifactSnapshotInput) (preparedArtifactSnapshot, string, error) {
	artifactKind := strings.TrimSpace(input.ArtifactKind)
	relativePath := strings.TrimSpace(input.RelativePath)
	checksum := strings.TrimSpace(input.Checksum)
	if artifactKind == "" {
		return preparedArtifactSnapshot{}, "", errors.New("globaldb: artifact kind is required")
	}
	if relativePath == "" {
		return preparedArtifactSnapshot{}, "", errors.New("globaldb: artifact relative path is required")
	}
	if checksum == "" {
		return preparedArtifactSnapshot{}, "", fmt.Errorf(
			"globaldb: artifact checksum is required for %s/%s",
			artifactKind,
			relativePath,
		)
	}

	bodyStorageKind := artifactBodyInlineKind
	bodyText := input.BodyText
	if len([]byte(bodyText)) > artifactBodyLimitBytes {
		bodyStorageKind = artifactBodyOverflowKind
		bodyText = overflowReference(relativePath, checksum)
	}

	frontmatterJSON := strings.TrimSpace(input.FrontmatterJSON)
	if frontmatterJSON == "" {
		frontmatterJSON = "{}"
	}
	if input.SourceMTime.IsZero() {
		return preparedArtifactSnapshot{}, "", fmt.Errorf(
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
		BodyStorageKind: bodyStorageKind,
		SourceMTime:     input.SourceMTime.UTC(),
	}
	return prepared, artifactKey(artifactKind, relativePath), nil
}

func overflowReference(relativePath string, checksum string) string {
	return fmt.Sprintf("overflow:%s#%s", strings.TrimSpace(relativePath), strings.TrimSpace(checksum))
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
		return TaskItemInput{}, fmt.Errorf("globaldb: task number must be positive (got %d)", input.TaskNumber)
	}
	if strings.TrimSpace(input.TaskID) == "" {
		return TaskItemInput{}, fmt.Errorf("globaldb: task id is required for task %d", input.TaskNumber)
	}
	if strings.TrimSpace(input.Title) == "" {
		return TaskItemInput{}, fmt.Errorf("globaldb: task title is required for task %d", input.TaskNumber)
	}
	if strings.TrimSpace(input.Status) == "" {
		return TaskItemInput{}, fmt.Errorf("globaldb: task status is required for task %d", input.TaskNumber)
	}
	if strings.TrimSpace(input.Kind) == "" {
		return TaskItemInput{}, fmt.Errorf("globaldb: task kind is required for task %d", input.TaskNumber)
	}
	if strings.TrimSpace(input.SourcePath) == "" {
		return TaskItemInput{}, fmt.Errorf("globaldb: task source path is required for task %d", input.TaskNumber)
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
		return ReviewRoundInput{}, fmt.Errorf("globaldb: review round must be positive (got %d)", input.RoundNumber)
	}
	if strings.TrimSpace(input.Provider) == "" {
		return ReviewRoundInput{}, fmt.Errorf(
			"globaldb: review round provider is required for round %d",
			input.RoundNumber,
		)
	}
	if input.ResolvedCount < 0 || input.UnresolvedCount < 0 {
		return ReviewRoundInput{}, fmt.Errorf(
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
		return ReviewIssueInput{}, fmt.Errorf(
			"globaldb: review issue number must be positive (got %d)",
			input.IssueNumber,
		)
	}
	if strings.TrimSpace(input.Status) == "" {
		return ReviewIssueInput{}, fmt.Errorf(
			"globaldb: review issue status is required for issue %d",
			input.IssueNumber,
		)
	}
	if strings.TrimSpace(input.SourcePath) == "" {
		return ReviewIssueInput{}, fmt.Errorf(
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
