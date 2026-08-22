package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const defaultLoopAPIListLimit = 100
const maxLoopAPIListLimit = 500
const loopRunOperationalFetchLimit = maxLoopAPIListLimit + 1
const loopRunAPIListSelectSQL = `SELECT ` + loopRunSelectColumnsSQL + ` FROM loop_runs`
const loopRunOperationalRankSQL = `(CASE
	WHEN loop_runs.status = 'needs-approval'
		OR (loop_runs.status NOT IN ('done','no-op','blocked','failed','exhausted','stalled','canceled') AND (
		EXISTS (
			SELECT 1 FROM loop_node_controls AS control
			WHERE control.loop_run_id = loop_runs.id AND control.quarantined = 1
		)
		OR EXISTS (
			SELECT 1 FROM loop_requests AS request
			WHERE request.loop_run_id = loop_runs.id AND request.state = 'pending'
		)))
	THEN 0
	WHEN loop_runs.status IN ('queued', 'running', 'watching', 'paused') THEN 1
	ELSE 2
END)`

// ListLoopRuns loads workspace-scoped loop runs in newest-first order.
func (g *LoopRepo) ListLoopRuns(
	ctx context.Context,
	query looppkg.RunListQuery,
) (runs []looppkg.Run, err error) {
	if err := g.checkReady(ctx, "list loop runs"); err != nil {
		return nil, err
	}
	normalized, err := normalizeLoopRunListQuery(query)
	if err != nil {
		return nil, err
	}
	if normalized.After != nil {
		if err := g.validateLoopRunListPosition(ctx, normalized); err != nil {
			return nil, err
		}
	}
	clauses := []store.Clause{
		store.StringClause("workspace_id", string(normalized.WorkspaceID)),
		store.StringClause("loop_name", normalized.LoopName),
		store.StringClause("status", string(normalized.Status)),
		store.StringClause("origin_kind", normalized.OriginKind),
		store.StringClause("origin_session_id", normalized.OriginSessionID),
		store.TimeClause("created_at", ">=", normalized.CreatedAfter),
	}
	where, args := store.BuildClauses(clauses...)
	if normalized.Live != nil {
		where = append(where, loopRunLiveFilterSQL(*normalized.Live))
	}
	if normalized.OperationalOrder && normalized.After != nil {
		where = append(where, `(`+loopRunOperationalRankSQL+` > ? OR (`+
			loopRunOperationalRankSQL+` = ? AND created_at < ?) OR (`+
			loopRunOperationalRankSQL+` = ? AND created_at = ? AND id < ?))`)
		args = append(
			args,
			normalized.After.Rank,
			normalized.After.Rank,
			store.FormatTimestamp(normalized.After.CreatedAt),
			normalized.After.Rank,
			store.FormatTimestamp(normalized.After.CreatedAt),
			normalized.After.ID,
		)
	}
	// dynamic-sql: optional run filters, live-state predicate, and caller limit change the statement shape.
	orderBy := ` ORDER BY created_at DESC, id DESC LIMIT ?`
	if normalized.OperationalOrder {
		orderBy = ` ORDER BY ` + loopRunOperationalRankSQL + ` ASC, created_at DESC, id DESC LIMIT ?`
	}
	// #nosec G202 -- clauses are constants; values are parameters.
	sqlText := store.AppendWhere(loopRunAPIListSelectSQL, where) + orderBy
	args = append(args, normalized.Limit)
	rows, err := g.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list loop runs: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "loop run query")
	}()

	runs = make([]looppkg.Run, 0)
	for rows.Next() {
		run, scanErr := scanLoopRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate loop runs: %w", err)
	}
	return runs, nil
}

func (g *LoopRepo) validateLoopRunListPosition(
	ctx context.Context,
	query looppkg.RunListQuery,
) error {
	clauses := []store.Clause{
		store.StringClause("workspace_id", string(query.WorkspaceID)),
		store.StringClause("loop_name", query.LoopName),
		store.StringClause("status", string(query.Status)),
		store.StringClause("origin_kind", query.OriginKind),
		store.StringClause("origin_session_id", query.OriginSessionID),
		store.TimeClause("created_at", ">=", query.CreatedAfter),
		store.StringClause("id", string(query.After.ID)),
	}
	where, args := store.BuildClauses(clauses...)
	if query.Live != nil {
		where = append(where, loopRunLiveFilterSQL(*query.Live))
	}
	statement := store.AppendWhere(
		"SELECT created_at, "+loopRunOperationalRankSQL+" FROM loop_runs",
		where,
	)
	var createdAtRaw string
	var rank int
	if err := g.db.QueryRowContext(ctx, statement, args...).Scan(&createdAtRaw, &rank); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(
				"%w: Loop run list cursor no longer identifies its boundary",
				looppkg.ErrInvalidRunListCursor,
			)
		}
		return fmt.Errorf("store: validate Loop run list cursor: %w", err)
	}
	createdAt, err := parseLoopRunTimestamp(createdAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse Loop run list cursor boundary time: %w", err)
	}
	if rank != query.After.Rank || !createdAt.Equal(query.After.CreatedAt.UTC()) {
		return fmt.Errorf("%w: Loop run list cursor no longer identifies its boundary", looppkg.ErrInvalidRunListCursor)
	}
	return nil
}

// ListLoopRunEvents loads retained events after the requested sequence.
func (g *LoopRepo) ListLoopRunEvents(
	ctx context.Context,
	query looppkg.RunEventQuery,
) ([]looppkg.RunEvent, error) {
	if err := g.checkReady(ctx, "list loop run events"); err != nil {
		return nil, err
	}
	normalized, err := normalizeLoopRunEventQuery(query)
	if err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopRunEvents(ctx, sqlcgen.ListLoopRunEventsParams{
		WorkspaceID: string(normalized.WorkspaceID), LoopRunID: string(normalized.RunID),
		AfterSeq: normalized.AfterSeq, RowLimit: int64(normalized.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop run events: %w", err)
	}
	events := make([]looppkg.RunEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, loopRunEventFromGenerated(row))
	}
	return events, nil
}

// GetLoopRunEventHead returns the durable sequence head for one workspace-owned run.
func (g *LoopRepo) GetLoopRunEventHead(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (int64, error) {
	if err := g.checkReady(ctx, "get loop run event head"); err != nil {
		return 0, err
	}
	head, err := g.queries.GetLoopRunEventHead(ctx, sqlcgen.GetLoopRunEventHeadParams{
		WorkspaceID: string(workspaceID), LoopRunID: string(runID),
	})
	if err != nil {
		return 0, fmt.Errorf("store: get loop run event head: %w", err)
	}
	return head, nil
}

// ListLoopRunEventsBackward returns one snapshot-fenced page in descending sequence order.
func (g *LoopRepo) ListLoopRunEventsBackward(
	ctx context.Context,
	query looppkg.RunEventBackwardQuery,
) ([]looppkg.RunEvent, error) {
	if err := g.checkReady(ctx, "list loop run events backward"); err != nil {
		return nil, err
	}
	if query.Limit < 1 || query.Limit > maxLoopAPIListLimit || query.FixedHeadSeq < 0 || query.BeforeSeq < 1 {
		return nil, fmt.Errorf("%w: backward loop event query is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopRunEventsBackward(ctx, sqlcgen.ListLoopRunEventsBackwardParams{
		WorkspaceID: string(query.WorkspaceID), LoopRunID: string(query.RunID), FixedHeadSeq: query.FixedHeadSeq,
		BeforeSeq: query.BeforeSeq, RowLimit: int64(query.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop run events backward: %w", err)
	}
	events := make([]looppkg.RunEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, loopRunEventFromGenerated(row))
	}
	return events, nil
}

// ListRouteCauses loads durable route decisions for one workspace-owned generation.
func (g *LoopRepo) ListRouteCauses(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	generation int64,
) ([]looppkg.RouteCause, error) {
	if err := g.checkReady(ctx, "list loop route causes"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" || generation < 1 {
		return nil, fmt.Errorf("%w: route cause scope is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopRouteCauses(ctx, sqlcgen.ListLoopRouteCausesParams{
		WorkspaceID: string(workspaceID), LoopRunID: string(runID), Generation: strconv.FormatInt(generation, 10),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop route causes: %w", err)
	}
	causes := make([]looppkg.RouteCause, 0, len(rows))
	for _, row := range rows {
		cause, decodeErr := decodeStoredRouteCause(row.PayloadJson, row.At, generation)
		if decodeErr != nil {
			return nil, decodeErr
		}
		causes = append(causes, cause)
	}
	return causes, nil
}

// ListLoopUIAnnotations loads editor node positions for one workspace loop.
func (g *LoopRepo) ListLoopUIAnnotations(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
) ([]looppkg.UIAnnotation, error) {
	if err := g.checkReady(ctx, "list loop ui annotations"); err != nil {
		return nil, err
	}
	workspaceID, name, err := normalizeLoopAnnotationKey(ws, loopName)
	if err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopUIAnnotations(ctx, sqlcgen.ListLoopUIAnnotationsParams{
		WorkspaceID: string(workspaceID), LoopName: name,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop ui annotations: %w", err)
	}
	annotations := make([]looppkg.UIAnnotation, 0, len(rows))
	for _, row := range rows {
		annotations = append(annotations, looppkg.UIAnnotation{NodeID: looppkg.NodeID(row.NodeID), X: row.X, Y: row.Y})
	}
	return annotations, nil
}

// ReplaceLoopUIAnnotations replaces editor node positions for one workspace loop.
func (g *LoopRepo) ReplaceLoopUIAnnotations(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
	annotations []looppkg.UIAnnotation,
) error {
	if err := g.checkReady(ctx, "replace loop ui annotations"); err != nil {
		return err
	}
	workspaceID, name, err := normalizeLoopAnnotationKey(ws, loopName)
	if err != nil {
		return err
	}
	normalized, err := normalizeLoopAnnotations(annotations)
	if err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "replace loop ui annotations", func(exec taskSQLExecutor) error {
		queries := sqlcgen.New(exec)
		if err := queries.DeleteLoopUIAnnotations(ctx, sqlcgen.DeleteLoopUIAnnotationsParams{
			WorkspaceID: string(workspaceID), LoopName: name,
		}); err != nil {
			return fmt.Errorf("store: delete loop ui annotations: %w", err)
		}
		for _, annotation := range normalized {
			if err := queries.InsertLoopUIAnnotation(ctx, sqlcgen.InsertLoopUIAnnotationParams{
				WorkspaceID: string(workspaceID), LoopName: name, NodeID: string(annotation.NodeID),
				X: annotation.X, Y: annotation.Y,
			}); err != nil {
				return fmt.Errorf("store: insert loop ui annotation %q: %w", annotation.NodeID, err)
			}
		}
		return nil
	})
}

func normalizeLoopRunListQuery(query looppkg.RunListQuery) (looppkg.RunListQuery, error) {
	normalized := query
	normalized.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(query.WorkspaceID)))
	normalized.LoopName = strings.TrimSpace(query.LoopName)
	normalized.Status = looppkg.Status(strings.TrimSpace(string(query.Status)))
	normalized.OriginKind = strings.TrimSpace(query.OriginKind)
	normalized.OriginSessionID = strings.TrimSpace(query.OriginSessionID)
	normalized.CreatedAfter = query.CreatedAfter.UTC()
	if normalized.WorkspaceID == "" {
		return looppkg.RunListQuery{}, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if normalized.Status != "" && !normalized.Status.Valid() {
		return looppkg.RunListQuery{}, fmt.Errorf("%w: loop status is invalid: %q", looppkg.ErrValidation, query.Status)
	}
	if normalized.OriginKind != "" && normalized.OriginKind != string(looppkg.RunOriginCatalog) &&
		normalized.OriginKind != string(looppkg.RunOriginSession) {
		return looppkg.RunListQuery{}, fmt.Errorf(
			"%w: loop run origin is invalid: %q",
			looppkg.ErrValidation,
			query.OriginKind,
		)
	}
	if normalized.OriginSessionID != "" {
		if normalized.OriginKind == string(looppkg.RunOriginCatalog) {
			return looppkg.RunListQuery{}, fmt.Errorf(
				"%w: origin_session requires session origin",
				looppkg.ErrValidation,
			)
		}
		normalized.OriginKind = string(looppkg.RunOriginSession)
	}
	if normalized.After != nil {
		after := *normalized.After
		normalized.After = &after
		normalized.After.ID = looppkg.RunID(strings.TrimSpace(string(after.ID)))
		normalized.After.CreatedAt = after.CreatedAt.UTC()
		if !normalized.OperationalOrder || normalized.After.Rank < 0 || normalized.After.Rank > 2 ||
			normalized.After.CreatedAt.IsZero() || normalized.After.ID == "" {
			return looppkg.RunListQuery{}, fmt.Errorf("%w: loop run list position is invalid", looppkg.ErrValidation)
		}
	}
	if normalized.OperationalOrder && normalized.Limit == loopRunOperationalFetchLimit {
		normalized.Limit = loopRunOperationalFetchLimit
	} else {
		normalized.Limit = normalizeLoopAPILimit(normalized.Limit)
	}
	return normalized, nil
}

func loopRunLiveFilterSQL(live bool) string {
	if live {
		return "historical = 0 AND status IN ('queued','running','watching','needs-approval','paused')"
	}
	return "historical = 0 AND status IN ('done','no-op','blocked','failed','exhausted','stalled','canceled')"
}

func normalizeLoopRunEventQuery(query looppkg.RunEventQuery) (looppkg.RunEventQuery, error) {
	normalized := query
	normalized.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(query.WorkspaceID)))
	normalized.RunID = looppkg.RunID(strings.TrimSpace(string(query.RunID)))
	if normalized.WorkspaceID == "" {
		return looppkg.RunEventQuery{}, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if normalized.RunID == "" {
		return looppkg.RunEventQuery{}, fmt.Errorf("%w: run_id is required", looppkg.ErrValidation)
	}
	if normalized.AfterSeq < 0 {
		return looppkg.RunEventQuery{}, fmt.Errorf("%w: after_seq must be non-negative", looppkg.ErrValidation)
	}
	normalized.Limit = normalizeLoopAPILimit(normalized.Limit)
	return normalized, nil
}

func normalizeLoopAPILimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLoopAPIListLimit
	case limit > maxLoopAPIListLimit:
		return maxLoopAPIListLimit
	default:
		return limit
	}
}

func normalizeLoopAnnotationKey(
	ws looppkg.WorkspaceID,
	loopName string,
) (looppkg.WorkspaceID, string, error) {
	workspaceID := looppkg.WorkspaceID(strings.TrimSpace(string(ws)))
	name := strings.TrimSpace(loopName)
	if workspaceID == "" {
		return "", "", fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if name == "" {
		return "", "", fmt.Errorf("%w: loop name is required", looppkg.ErrValidation)
	}
	return workspaceID, name, nil
}

func normalizeLoopAnnotations(annotations []looppkg.UIAnnotation) ([]looppkg.UIAnnotation, error) {
	normalized := make([]looppkg.UIAnnotation, 0, len(annotations))
	seen := map[looppkg.NodeID]struct{}{}
	for _, annotation := range annotations {
		nodeID := looppkg.NodeID(strings.TrimSpace(string(annotation.NodeID)))
		if nodeID == "" {
			return nil, fmt.Errorf("%w: annotation node_id is required", looppkg.ErrValidation)
		}
		if _, exists := seen[nodeID]; exists {
			return nil, fmt.Errorf("%w: duplicate annotation node_id %q", looppkg.ErrValidation, nodeID)
		}
		seen[nodeID] = struct{}{}
		normalized = append(normalized, looppkg.UIAnnotation{
			NodeID: nodeID,
			X:      annotation.X,
			Y:      annotation.Y,
		})
	}
	return normalized, nil
}

var _ looppkg.RunReader = (*LoopRepo)(nil)
var _ looppkg.TimelineEventReader = (*LoopRepo)(nil)
var _ looppkg.AnnotationStore = (*LoopRepo)(nil)
