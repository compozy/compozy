package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type loopNodeInventoryRow struct {
	loopRunID  string
	loopName   string
	generation int64
	nodeID     string
	itemIndex  int64
	stateAt    time.Time
}

// ListNodeInventory returns one stable, workspace-scoped node-state page.
func (g *LoopRepo) ListNodeInventory(
	ctx context.Context,
	query looppkg.NodeInventoryQuery,
) (page looppkg.NodeInventoryPage, returnErr error) {
	if err := g.checkReady(ctx, "list loop node inventory"); err != nil {
		return looppkg.NodeInventoryPage{}, err
	}
	normalized, err := looppkg.NormalizeNodeInventoryQuery(query)
	if err != nil {
		return looppkg.NodeInventoryPage{}, err
	}
	cursor, err := looppkg.DecodeNodeInventoryCursor(normalized)
	if err != nil {
		return looppkg.NodeInventoryPage{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return looppkg.NodeInventoryPage{}, fmt.Errorf("store: begin loop node inventory read: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: rollback loop node inventory read: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	params := loopNodeInventoryParams(normalized, cursor)
	rows, err := g.listNodeInventoryRows(ctx, queries, normalized.State, params)
	if err != nil {
		return looppkg.NodeInventoryPage{}, fmt.Errorf("store: list %s loop node inventory: %w", normalized.State, err)
	}
	items := make([]looppkg.NodeInventoryItem, 0, min(len(rows), normalized.Limit))
	for index, row := range rows {
		if index == normalized.Limit {
			break
		}
		items = append(items, looppkg.NodeInventoryItem{
			State:      normalized.State,
			LoopRunID:  looppkg.RunID(row.loopRunID),
			LoopName:   row.loopName,
			Generation: int(row.generation),
			NodeID:     looppkg.NodeID(row.nodeID),
			ItemIndex:  int(row.itemIndex),
			StateAt:    row.stateAt.UTC(),
		})
	}
	items, err = hydrateNodeInventoryItems(ctx, queries, normalized.WorkspaceID, items)
	if err != nil {
		return looppkg.NodeInventoryPage{}, err
	}
	page = looppkg.NodeInventoryPage{Items: items}
	if len(rows) > normalized.Limit {
		page.NextCursor, err = looppkg.EncodeNodeInventoryCursor(normalized, items[len(items)-1])
		if err != nil {
			return looppkg.NodeInventoryPage{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return looppkg.NodeInventoryPage{}, fmt.Errorf("store: commit loop node inventory read: %w", err)
	}
	return page, nil
}

type nodeInventorySQLParams struct {
	workspaceID      string
	loopName         string
	runID            string
	hasCursor        int64
	cursorAt         time.Time
	cursorRunID      string
	cursorGeneration int64
	cursorNodeID     string
	cursorItemIndex  int64
	pageLimit        int64
}

func loopNodeInventoryParams(
	query looppkg.NodeInventoryQuery,
	cursor *looppkg.NodeInventoryCursor,
) nodeInventorySQLParams {
	params := nodeInventorySQLParams{
		workspaceID: string(query.WorkspaceID),
		loopName:    query.LoopName,
		runID:       string(query.RunID),
		pageLimit:   int64(query.Limit + 1),
	}
	if cursor != nil {
		params.hasCursor = 1
		params.cursorAt = cursor.StateAt.UTC()
		params.cursorRunID = string(cursor.LoopRunID)
		params.cursorGeneration = int64(cursor.Generation)
		params.cursorNodeID = string(cursor.NodeID)
		params.cursorItemIndex = int64(cursor.ItemIndex)
	}
	return params
}

func (g *LoopRepo) listNodeInventoryRows(
	ctx context.Context,
	queries *sqlcgen.Queries,
	state looppkg.NodeInventoryState,
	params nodeInventorySQLParams,
) ([]loopNodeInventoryRow, error) {
	switch state {
	case looppkg.NodeInventoryWaiting:
		rows, err := queries.ListWaitingLoopNodeInventory(ctx, sqlcgen.ListWaitingLoopNodeInventoryParams{
			FilterWorkspaceID: params.workspaceID,
			FilterLoopName:    params.loopName,
			FilterRunID:       params.runID,
			HasCursor:         params.hasCursor,
			CursorAt:          params.cursorAt,
			CursorRunID:       params.cursorRunID,
			CursorGeneration:  params.cursorGeneration,
			CursorNodeID:      params.cursorNodeID,
			CursorItemIndex:   params.cursorItemIndex,
			PageLimit:         params.pageLimit,
		})
		return waitingInventoryRows(rows), err
	case looppkg.NodeInventoryQuarantined:
		rows, err := queries.ListQuarantinedLoopNodeInventory(ctx, sqlcgen.ListQuarantinedLoopNodeInventoryParams{
			FilterWorkspaceID: params.workspaceID,
			FilterLoopName:    params.loopName,
			FilterRunID:       params.runID,
			HasCursor:         params.hasCursor,
			CursorAt:          sql.NullTime{Time: params.cursorAt, Valid: params.hasCursor == 1},
			CursorRunID:       params.cursorRunID,
			CursorGeneration:  params.cursorGeneration,
			CursorNodeID:      params.cursorNodeID,
			PageLimit:         params.pageLimit,
		})
		return quarantinedInventoryRows(rows), err
	case looppkg.NodeInventoryAttention:
		rows, err := queries.ListAttentionLoopNodeInventory(ctx, sqlcgen.ListAttentionLoopNodeInventoryParams{
			FilterWorkspaceID: params.workspaceID,
			FilterLoopName:    params.loopName,
			FilterRunID:       params.runID,
			HasCursor:         params.hasCursor,
			CursorAt:          params.cursorAt,
			CursorRunID:       params.cursorRunID,
			CursorGeneration:  params.cursorGeneration,
			CursorNodeID:      params.cursorNodeID,
			PageLimit:         params.pageLimit,
		})
		return attentionInventoryRows(rows), err
	case looppkg.NodeInventoryRetrying:
		rows, err := queries.ListRetryingLoopNodeInventory(ctx, sqlcgen.ListRetryingLoopNodeInventoryParams{
			FilterWorkspaceID: params.workspaceID,
			FilterLoopName:    params.loopName,
			FilterRunID:       params.runID,
			HasCursor:         params.hasCursor,
			CursorAt:          sql.NullTime{Time: params.cursorAt, Valid: params.hasCursor == 1},
			CursorRunID:       params.cursorRunID,
			CursorGeneration:  params.cursorGeneration,
			CursorNodeID:      params.cursorNodeID,
			CursorItemIndex:   params.cursorItemIndex,
			PageLimit:         params.pageLimit,
		})
		return retryingInventoryRows(rows), err
	default:
		return nil, fmt.Errorf("%w: node inventory state is invalid: %q", looppkg.ErrValidation, state)
	}
}

func waitingInventoryRows(rows []sqlcgen.ListWaitingLoopNodeInventoryRow) []loopNodeInventoryRow {
	items := make([]loopNodeInventoryRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, loopNodeInventoryRow{
			row.LoopRunID, row.LoopName, row.Generation, row.NodeID, row.ItemIndex, row.StateAt,
		})
	}
	return items
}

func quarantinedInventoryRows(rows []sqlcgen.ListQuarantinedLoopNodeInventoryRow) []loopNodeInventoryRow {
	items := make([]loopNodeInventoryRow, 0, len(rows))
	for _, row := range rows {
		if !row.StateAt.Valid {
			continue
		}
		items = append(items, loopNodeInventoryRow{
			row.LoopRunID, row.LoopName, row.Generation, row.NodeID, row.ItemIndex, row.StateAt.Time,
		})
	}
	return items
}

func attentionInventoryRows(rows []sqlcgen.ListAttentionLoopNodeInventoryRow) []loopNodeInventoryRow {
	items := make([]loopNodeInventoryRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, loopNodeInventoryRow{
			row.LoopRunID, row.LoopName, row.Generation, row.NodeID, row.ItemIndex, row.StateAt,
		})
	}
	return items
}

func retryingInventoryRows(rows []sqlcgen.ListRetryingLoopNodeInventoryRow) []loopNodeInventoryRow {
	items := make([]loopNodeInventoryRow, 0, len(rows))
	for _, row := range rows {
		if !row.StateAt.Valid {
			continue
		}
		items = append(items, loopNodeInventoryRow{
			row.LoopRunID, row.LoopName, row.Generation, row.NodeID, row.ItemIndex, row.StateAt.Time,
		})
	}
	return items
}
