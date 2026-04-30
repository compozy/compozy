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

// ErrTaskItemNotFound reports a missing synced task-item projection row.
var ErrTaskItemNotFound = errors.New("globaldb: task item not found")

// TaskItemRow captures one durable task projection row.
type TaskItemRow struct {
	ID         string
	WorkflowID string
	TaskNumber int
	TaskID     string
	Title      string
	Status     string
	Kind       string
	DependsOn  []string
	SourcePath string
	UpdatedAt  time.Time
}

// ArtifactSnapshotRow captures one durable authored-artifact snapshot row.
type ArtifactSnapshotRow struct {
	WorkflowID      string
	ArtifactKind    string
	RelativePath    string
	Checksum        string
	FrontmatterJSON string
	BodyText        string
	BodyStorageKind string
	SourceMTime     time.Time
	SyncedAt        time.Time
}

// WorkflowTaskCountsRow captures aggregated task counts for one workflow.
type WorkflowTaskCountsRow struct {
	Total     int
	Completed int
	Pending   int
}

// ListTaskItems returns synced task-item rows for one workflow in task order.
func (g *GlobalDB) ListTaskItems(ctx context.Context, workflowID string) ([]TaskItemRow, error) {
	if err := g.requireContext(ctx, "list task items"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT id, workflow_id, task_number, task_id, title, status, kind, depends_on_json, source_path, updated_at
		 FROM task_items
		 WHERE workflow_id = ?
		 ORDER BY task_number ASC, id ASC`,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query task items for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]TaskItemRow, 0)
	for rows.Next() {
		item, scanErr := scanTaskItemRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate task items for workflow %q: %w", workflowID, err)
	}
	return items, nil
}

// TaskCountsByWorkflowIDs returns aggregated task counts keyed by workflow id.
func (g *GlobalDB) TaskCountsByWorkflowIDs(
	ctx context.Context,
	workflowIDs []string,
) (map[string]WorkflowTaskCountsRow, error) {
	if err := g.requireContext(ctx, "list workflow task counts"); err != nil {
		return nil, err
	}

	ids := distinctTrimmedStrings(workflowIDs)
	result := make(map[string]WorkflowTaskCountsRow, len(ids))
	for _, workflowID := range ids {
		result[workflowID] = WorkflowTaskCountsRow{}
	}
	if len(ids) == 0 {
		return result, nil
	}

	valuesClause, args := selectedWorkflowIDsValues(ids)
	rows, err := g.db.QueryContext(
		ctx,
		`WITH selected_workflow_ids(id) AS (VALUES `+valuesClause+`)
		 SELECT task_items.workflow_id,
		        COUNT(1) AS total,
		        SUM(CASE WHEN task_items.status = 'completed' THEN 1 ELSE 0 END) AS completed,
		        SUM(CASE WHEN task_items.status <> 'completed' THEN 1 ELSE 0 END) AS pending
		 FROM task_items
		 JOIN selected_workflow_ids selected ON selected.id = task_items.workflow_id
		 GROUP BY task_items.workflow_id`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query workflow task counts: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var (
			workflowID string
			counts     WorkflowTaskCountsRow
		)
		if err := rows.Scan(&workflowID, &counts.Total, &counts.Completed, &counts.Pending); err != nil {
			return nil, fmt.Errorf("globaldb: scan workflow task counts: %w", err)
		}
		result[workflowID] = counts
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate workflow task counts: %w", err)
	}
	return result, nil
}

// GetTaskItemByTaskID returns one synced task-item row by stable task id.
func (g *GlobalDB) GetTaskItemByTaskID(
	ctx context.Context,
	workflowID string,
	taskID string,
) (TaskItemRow, error) {
	if err := g.requireContext(ctx, "get task item"); err != nil {
		return TaskItemRow{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, task_number, task_id, title, status, kind, depends_on_json, source_path, updated_at
		 FROM task_items
		 WHERE workflow_id = ? AND task_id = ?`,
		strings.TrimSpace(workflowID),
		strings.TrimSpace(taskID),
	)
	item, err := scanTaskItemRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TaskItemRow{}, ErrTaskItemNotFound
		}
		return TaskItemRow{}, err
	}
	return item, nil
}

// ListArtifactSnapshots returns authored-artifact snapshot rows for one workflow.
func (g *GlobalDB) ListArtifactSnapshots(ctx context.Context, workflowID string) ([]ArtifactSnapshotRow, error) {
	if err := g.requireContext(ctx, "list artifact snapshots"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT snapshots.workflow_id,
		        snapshots.artifact_kind,
		        snapshots.relative_path,
		        snapshots.checksum,
		        snapshots.frontmatter_json,
		        CASE
		        	WHEN snapshots.body_storage_kind = ? THEN COALESCE(bodies.body_text, snapshots.body_text)
		        	ELSE snapshots.body_text
		        END AS body_text,
		        snapshots.body_storage_kind,
		        snapshots.source_mtime,
		        snapshots.synced_at
		 FROM artifact_snapshots snapshots
		 LEFT JOIN artifact_bodies bodies ON bodies.checksum = snapshots.checksum
		 WHERE snapshots.workflow_id = ?
		 ORDER BY snapshots.artifact_kind ASC, snapshots.relative_path ASC`,
		artifactBodyBlobKind,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query artifact snapshots for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	snapshots := make([]ArtifactSnapshotRow, 0)
	for rows.Next() {
		row, scanErr := scanArtifactSnapshotRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		snapshots = append(snapshots, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate artifact snapshots for workflow %q: %w", workflowID, err)
	}
	return snapshots, nil
}

// ListReviewRounds returns persisted review rounds for one workflow in round order.
func (g *GlobalDB) ListReviewRounds(ctx context.Context, workflowID string) ([]ReviewRound, error) {
	if err := g.requireContext(ctx, "list review rounds"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT id, workflow_id, round_number, provider, pr_ref, resolved_count, unresolved_count, updated_at
		 FROM review_rounds
		 WHERE workflow_id = ?
		 ORDER BY round_number ASC, id ASC`,
		strings.TrimSpace(workflowID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query review rounds for workflow %q: %w", workflowID, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	rounds := make([]ReviewRound, 0)
	for rows.Next() {
		round, scanErr := scanReviewRound(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		rounds = append(rounds, round)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate review rounds for workflow %q: %w", workflowID, err)
	}
	return rounds, nil
}

func scanTaskItemRow(scanner interface {
	Scan(dest ...any) error
}) (TaskItemRow, error) {
	var (
		item          TaskItemRow
		dependsOnJSON string
		updatedAt     string
	)
	if err := scanner.Scan(
		&item.ID,
		&item.WorkflowID,
		&item.TaskNumber,
		&item.TaskID,
		&item.Title,
		&item.Status,
		&item.Kind,
		&dependsOnJSON,
		&item.SourcePath,
		&updatedAt,
	); err != nil {
		return TaskItemRow{}, fmt.Errorf("globaldb: scan task item row: %w", err)
	}

	dependsOn, err := unmarshalJSONArray(dependsOnJSON)
	if err != nil {
		return TaskItemRow{}, err
	}
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return TaskItemRow{}, fmt.Errorf("globaldb: parse task item updated_at: %w", err)
	}

	item.DependsOn = dependsOn
	item.UpdatedAt = parsedUpdatedAt
	item.ID = strings.TrimSpace(item.ID)
	item.WorkflowID = strings.TrimSpace(item.WorkflowID)
	item.TaskID = strings.TrimSpace(item.TaskID)
	item.Title = strings.TrimSpace(item.Title)
	item.Status = strings.TrimSpace(item.Status)
	item.Kind = strings.TrimSpace(item.Kind)
	item.SourcePath = strings.TrimSpace(item.SourcePath)
	return item, nil
}

func scanArtifactSnapshotRow(scanner interface {
	Scan(dest ...any) error
}) (ArtifactSnapshotRow, error) {
	var (
		row         ArtifactSnapshotRow
		bodyText    sql.NullString
		sourceMTime string
		syncedAt    string
	)
	if err := scanner.Scan(
		&row.WorkflowID,
		&row.ArtifactKind,
		&row.RelativePath,
		&row.Checksum,
		&row.FrontmatterJSON,
		&bodyText,
		&row.BodyStorageKind,
		&sourceMTime,
		&syncedAt,
	); err != nil {
		return ArtifactSnapshotRow{}, fmt.Errorf("globaldb: scan artifact snapshot row: %w", err)
	}

	parsedSourceMTime, err := store.ParseTimestamp(sourceMTime)
	if err != nil {
		return ArtifactSnapshotRow{}, fmt.Errorf("globaldb: parse artifact snapshot source_mtime: %w", err)
	}
	parsedSyncedAt, err := store.ParseTimestamp(syncedAt)
	if err != nil {
		return ArtifactSnapshotRow{}, fmt.Errorf("globaldb: parse artifact snapshot synced_at: %w", err)
	}

	row.WorkflowID = strings.TrimSpace(row.WorkflowID)
	row.ArtifactKind = strings.TrimSpace(row.ArtifactKind)
	row.RelativePath = strings.TrimSpace(row.RelativePath)
	row.Checksum = strings.TrimSpace(row.Checksum)
	row.FrontmatterJSON = strings.TrimSpace(row.FrontmatterJSON)
	row.BodyText = bodyText.String
	row.BodyStorageKind = strings.TrimSpace(row.BodyStorageKind)
	row.SourceMTime = parsedSourceMTime
	row.SyncedAt = parsedSyncedAt
	return row, nil
}

func unmarshalJSONArray(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	values := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("globaldb: unmarshal json array: %w", err)
	}
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		normalized = append(normalized, trimmedValue)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func distinctTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func selectedWorkflowIDsValues(ids []string) (string, []any) {
	normalized := distinctTrimmedStrings(ids)
	args := make([]any, 0, len(normalized))

	var values strings.Builder
	for i, workflowID := range normalized {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?)")
		args = append(args, workflowID)
	}
	return values.String(), args
}
