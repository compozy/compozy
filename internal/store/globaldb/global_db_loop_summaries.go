package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const loopRunSummariesSQL = `
SELECT
	lr.id,
	lr.generation,
	lr.definition_digest,
	snapshot.definition_json,
	CASE
		WHEN lr.status = 'needs-approval' THEN 'approval'
		WHEN lr.status NOT IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') AND EXISTS (
			SELECT 1 FROM loop_node_controls AS control
			WHERE control.loop_run_id = lr.id AND control.quarantined = 1
		) THEN 'quarantine'
		WHEN lr.status NOT IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') AND EXISTS (
			SELECT 1 FROM loop_requests AS request
			WHERE request.loop_run_id = lr.id AND request.state = 'pending'
		) THEN 'request'
		ELSE ''
	END AS attention_kind,
	CASE WHEN lr.status IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') THEN 0 ELSE
	(CASE WHEN lr.status = 'needs-approval' THEN 1 ELSE 0 END) +
	(SELECT COUNT(*) FROM loop_node_controls AS control
	 WHERE control.loop_run_id = lr.id AND control.quarantined = 1) +
	(SELECT COUNT(*) FROM loop_requests AS request
	 WHERE request.loop_run_id = lr.id AND request.state = 'pending') END AS attention_count,
	CASE
		WHEN lr.status = 'needs-approval' THEN COALESCE((
			SELECT CAST(event.at AS TEXT)
			FROM loop_run_events AS event
			WHERE event.loop_run_id = lr.id AND event.kind = 'needs_approval'
			ORDER BY event.seq DESC
			LIMIT 1
		), '')
		WHEN lr.status NOT IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') AND EXISTS (
			SELECT 1 FROM loop_node_controls AS control
			WHERE control.loop_run_id = lr.id AND control.quarantined = 1
		) THEN COALESCE((
			SELECT CAST(MIN(control.quarantined_at) AS TEXT)
			FROM loop_node_controls AS control
			WHERE control.loop_run_id = lr.id AND control.quarantined = 1
		), '')
		WHEN lr.status NOT IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') AND EXISTS (
			SELECT 1 FROM loop_requests AS request
			WHERE request.loop_run_id = lr.id AND request.state = 'pending'
		) THEN COALESCE((
			SELECT CAST(MIN(request.opened_at) AS TEXT)
			FROM loop_requests AS request
			WHERE request.loop_run_id = lr.id AND request.state = 'pending'
		), '')
		ELSE ''
	END AS attention_since
FROM loop_runs AS lr
JOIN loop_definition_snapshots AS snapshot
  ON snapshot.workspace_id = lr.workspace_id
 AND snapshot.definition_digest = lr.definition_digest
WHERE lr.workspace_id = ?
  AND lr.id IN (%s)`

const loopRunSummaryForksSQL = `
SELECT forked_from_run_id, id, forked_from_generation
FROM loop_runs
WHERE workspace_id = ?
  AND forked_from_run_id IN (%s)
ORDER BY forked_from_run_id ASC, created_at ASC, id ASC`

func (g *LoopRepo) ListLoopRunSummaries(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runIDs []looppkg.RunID,
) (summaries map[looppkg.RunID]looppkg.RunListSummary, err error) {
	if err := g.checkReady(ctx, "list loop run summaries"); err != nil {
		return nil, err
	}
	if len(runIDs) == 0 {
		return map[looppkg.RunID]looppkg.RunListSummary{}, nil
	}
	placeholders, args := loopRunSummaryQueryArgs(workspaceID, runIDs)
	summaries, projections, err := g.queryLoopRunSummaryBase(ctx, placeholders, args)
	if err != nil {
		return nil, err
	}
	if err := g.projectLoopRunSummaryProgress(ctx, projections, placeholders, args); err != nil {
		return nil, err
	}
	for runID, projection := range projections {
		progress, projectionErr := looppkg.ProgressFromRosterSource(&projection.source)
		if projectionErr != nil {
			return nil, fmt.Errorf("store: project Loop run summary %q: %w", runID, projectionErr)
		}
		summary := summaries[runID]
		summary.Progress = progress
		summaries[runID] = summary
	}
	if err := g.appendLoopRunSummaryForks(ctx, summaries, placeholders, args); err != nil {
		return nil, err
	}
	return summaries, nil
}

func loopRunSummaryQueryArgs(
	workspaceID looppkg.WorkspaceID,
	runIDs []looppkg.RunID,
) ([]string, []any) {
	placeholders := make([]string, len(runIDs))
	args := make([]any, 0, len(runIDs)+1)
	args = append(args, string(workspaceID))
	for index, runID := range runIDs {
		placeholders[index] = "?"
		args = append(args, string(runID))
	}
	return placeholders, args
}

func (g *LoopRepo) queryLoopRunSummaryBase(
	ctx context.Context,
	placeholders []string,
	args []any,
) (
	summaries map[looppkg.RunID]looppkg.RunListSummary,
	projections map[looppkg.RunID]*loopRunSummaryProjection,
	err error,
) {
	// dynamic-sql: the selected run IDs define the placeholder count; every value is parameterized.
	// #nosec G201 -- the SQL body is constant and the only interpolation is generated question marks.
	query := fmt.Sprintf(loopRunSummariesSQL, strings.Join(placeholders, ","))
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("store: list loop run summaries: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "loop run summaries query")
	}()
	summaries = make(map[looppkg.RunID]looppkg.RunListSummary, len(placeholders))
	projections = make(map[looppkg.RunID]*loopRunSummaryProjection, len(placeholders))
	for rows.Next() {
		var runID string
		var round, attentionCount int
		var definitionDigest, definitionJSON string
		var attentionKind string
		var attentionSince sql.NullString
		if scanErr := rows.Scan(
			&runID,
			&round,
			&definitionDigest,
			&definitionJSON,
			&attentionKind,
			&attentionCount,
			&attentionSince,
		); scanErr != nil {
			return nil, nil, fmt.Errorf("store: scan loop run summary: %w", scanErr)
		}
		summary := looppkg.RunListSummary{
			RunID:    looppkg.RunID(runID),
			Progress: looppkg.StepProgress{Round: round},
		}
		projection, projectionErr := newLoopRunSummaryProjection(
			looppkg.RunID(runID),
			round,
			definitionDigest,
			definitionJSON,
		)
		if projectionErr != nil {
			return nil, nil, projectionErr
		}
		projections[summary.RunID] = projection
		if attentionCount > 0 {
			since, parseErr := parseLoopRunSummaryTimestamp(attentionSince.String)
			if parseErr != nil {
				return nil, nil, fmt.Errorf("store: parse loop run attention time: %w", parseErr)
			}
			summary.Attention = &looppkg.RunListAttention{
				Kind: attentionKind, Count: attentionCount, Since: since,
			}
		}
		summaries[summary.RunID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: iterate loop run summaries: %w", err)
	}
	return summaries, projections, nil
}

func parseLoopRunSummaryTimestamp(value string) (time.Time, error) {
	parsed, err := parseLoopRunTimestamp(value)
	if err == nil {
		return parsed, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
	} {
		parsed, parseErr := time.Parse(layout, strings.TrimSpace(value))
		if parseErr == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, err
}

func (g *LoopRepo) appendLoopRunSummaryForks(
	ctx context.Context,
	summaries map[looppkg.RunID]looppkg.RunListSummary,
	placeholders []string,
	args []any,
) (err error) {
	// dynamic-sql: the selected run IDs define the placeholder count; every value is parameterized.
	// #nosec G201 -- the SQL body is constant and the only interpolation is generated question marks.
	query := fmt.Sprintf(loopRunSummaryForksSQL, strings.Join(placeholders, ","))
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: list Loop run summary forks: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "Loop run summary forks query")
	}()
	for rows.Next() {
		var parentID, runID string
		var generation sql.NullInt64
		if scanErr := rows.Scan(&parentID, &runID, &generation); scanErr != nil {
			return fmt.Errorf("store: scan Loop run summary fork: %w", scanErr)
		}
		parent := looppkg.RunID(parentID)
		summary, ok := summaries[parent]
		if !ok {
			continue
		}
		summary.Forks = append(summary.Forks, looppkg.ForkRef{
			RunID: looppkg.RunID(runID), Generation: generation.Int64,
		})
		summaries[parent] = summary
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate Loop run summary forks: %w", err)
	}
	return nil
}
