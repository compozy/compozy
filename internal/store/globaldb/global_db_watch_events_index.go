package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const watchEventsPendingOutputKind = "watch_events_pending"

// ListParkedWatchEventSubscriptions rebuilds the in-memory daemon index from
// durable watch-events pending output refs.
func (g *WatchEventsRepo) ListParkedWatchEventSubscriptions(
	ctx context.Context,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	if err := g.checkReady(ctx, "list parked watch-events subscriptions"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListParkedWatchEventSubscriptions(ctx, sqlcgen.ListParkedWatchEventSubscriptionsParams{
		Status: string(looppkg.StatusWatching), OutputKind: nullableWatchEventsString(watchEventsPendingOutputKind),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list parked watch-events subscriptions: %w", err)
	}
	return parkedWatchEventSubscriptionsFromGenerated(rows)
}

// ListParkedWatchEventSubscriptionsForLoopRun returns the current parked
// subscriptions for one loop run without rebuilding the global index.
func (g *WatchEventsRepo) ListParkedWatchEventSubscriptionsForLoopRun(
	ctx context.Context,
	loopRunID string,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	if err := g.checkReady(ctx, "list parked watch-events subscriptions for loop run"); err != nil {
		return nil, err
	}
	loopRunID = strings.TrimSpace(loopRunID)
	if loopRunID == "" {
		return []looppkg.ParkedWatchEventSubscription{}, nil
	}
	rows, err := g.queries.ListParkedWatchEventSubscriptionsForLoopRun(
		ctx,
		sqlcgen.ListParkedWatchEventSubscriptionsForLoopRunParams{
			Status: string(looppkg.StatusWatching), OutputKind: nullableWatchEventsString(watchEventsPendingOutputKind),
			LoopRunID: loopRunID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("store: list parked watch-events subscriptions for loop run %q: %w", loopRunID, err)
	}
	return parkedWatchEventSubscriptionsForLoopRunFromGenerated(rows)
}

// ListParkedWatchEventSubscriptionsPage returns one bounded recovery page
// strictly after cursor in the same order as the full parked index.
func (g *WatchEventsRepo) ListParkedWatchEventSubscriptionsPage(
	ctx context.Context,
	cursor looppkg.ParkedWatchEventScanCursor,
	limit int,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	if err := g.checkReady(ctx, "list parked watch-events subscriptions page"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: parked watch-events page limit must be positive", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListParkedWatchEventSubscriptionsPage(
		ctx,
		sqlcgen.ListParkedWatchEventSubscriptionsPageParams{
			Status:     string(looppkg.StatusWatching),
			OutputKind: nullableWatchEventsString(watchEventsPendingOutputKind),
			CursorWorkspaceID: strings.TrimSpace(
				cursor.WorkspaceID,
			),
			CursorLoopRunID: strings.TrimSpace(cursor.LoopRunID),
			CursorNodeID:    strings.TrimSpace(cursor.NodeID),
			RowLimit:        int64(limit),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("store: list parked watch-events subscriptions page: %w", err)
	}
	return parkedWatchEventSubscriptionsPageFromGenerated(rows)
}

func parkedWatchEventSubscriptionsFromGenerated(
	rows []sqlcgen.ListParkedWatchEventSubscriptionsRow,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	parked := make([]looppkg.ParkedWatchEventSubscription, 0, len(rows))
	for _, row := range rows {
		subscription, err := parkedWatchEventSubscriptionFromFields(
			row.WorkspaceID, row.ID, row.LoopName, row.Generation, row.InputsJson,
			row.DefinitionDigest, row.DefinitionJson, row.NodeID, row.OutputRef,
		)
		if err != nil {
			return nil, err
		}
		if len(subscription.Subscriptions) > 0 {
			parked = append(parked, subscription)
		}
	}
	return parked, nil
}

func parkedWatchEventSubscriptionsForLoopRunFromGenerated(
	rows []sqlcgen.ListParkedWatchEventSubscriptionsForLoopRunRow,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	parked := make([]looppkg.ParkedWatchEventSubscription, 0, len(rows))
	for _, row := range rows {
		subscription, err := parkedWatchEventSubscriptionFromFields(
			row.WorkspaceID, row.ID, row.LoopName, row.Generation, row.InputsJson,
			row.DefinitionDigest, row.DefinitionJson, row.NodeID, row.OutputRef,
		)
		if err != nil {
			return nil, err
		}
		if len(subscription.Subscriptions) > 0 {
			parked = append(parked, subscription)
		}
	}
	return parked, nil
}

func parkedWatchEventSubscriptionsPageFromGenerated(
	rows []sqlcgen.ListParkedWatchEventSubscriptionsPageRow,
) ([]looppkg.ParkedWatchEventSubscription, error) {
	parked := make([]looppkg.ParkedWatchEventSubscription, 0, len(rows))
	for _, row := range rows {
		subscription, err := parkedWatchEventSubscriptionFromFields(
			row.WorkspaceID, row.ID, row.LoopName, row.Generation, row.InputsJson,
			row.DefinitionDigest, row.DefinitionJson, row.NodeID, row.OutputRef,
		)
		if err != nil {
			return nil, err
		}
		if len(subscription.Subscriptions) > 0 {
			parked = append(parked, subscription)
		}
	}
	return parked, nil
}

func parkedWatchEventSubscriptionFromFields(
	workspaceID string,
	loopRunID string,
	loopName string,
	generation int64,
	inputsRaw string,
	digest string,
	snapshotRaw string,
	nodeID string,
	outputRef string,
) (looppkg.ParkedWatchEventSubscription, error) {
	inputs, err := decodeParkedWatchEventsInputs(inputsRaw)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, err
	}
	state, ok, err := watchpkg.EventsPendingFromOutputRef(outputRef)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, err
	}
	if !ok {
		return looppkg.ParkedWatchEventSubscription{}, nil
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(json.RawMessage(snapshotRaw), digest)
	if err != nil {
		return looppkg.ParkedWatchEventSubscription{}, fmt.Errorf(
			"store: load parked watch-events definition: %w",
			err,
		)
	}
	return looppkg.ParkedWatchEventSubscription{
		WorkspaceID: strings.TrimSpace(workspaceID), LoopRunID: strings.TrimSpace(loopRunID),
		LoopName: strings.TrimSpace(loopName), Generation: int(generation), NodeID: strings.TrimSpace(nodeID),
		Inputs: inputs, Subscriptions: state.Subscriptions, Cursors: state.Cursors,
		Contracts: resolved.WatchEventsContracts,
	}, nil
}

func nullableWatchEventsString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func decodeParkedWatchEventsInputs(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewBufferString(trimmed))
	decoder.UseNumber()
	var inputs map[string]any
	if err := decoder.Decode(&inputs); err != nil {
		return nil, fmt.Errorf("store: decode parked watch-events inputs: %w", err)
	}
	if inputs == nil {
		return map[string]any{}, nil
	}
	return inputs, nil
}
