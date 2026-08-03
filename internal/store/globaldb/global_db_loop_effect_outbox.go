package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const defaultLoopEffectPageSize = 50

// ListEffectOutbox returns workspace-scoped effect delivery rows for one run.
func (g *LoopRepo) ListEffectOutbox(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.EffectOutboxEntry, error) {
	if err := g.checkReady(ctx, "list effect outbox"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopEffectOutbox(ctx, sqlcgen.ListLoopEffectOutboxParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop effect outbox: %w", err)
	}
	entries := make([]looppkg.EffectOutboxEntry, 0, len(rows))
	for _, row := range rows {
		state := looppkg.EffectOutboxState(row.State)
		if !state.Valid() {
			return nil, fmt.Errorf(
				"store: invalid loop effect outbox state %q: %w",
				row.State,
				looppkg.ErrValidation,
			)
		}
		entries = append(entries, looppkg.EffectOutboxEntry{
			LoopRunID:     looppkg.RunID(row.LoopRunID),
			WorkspaceID:   workspaceID,
			DeliveryID:    row.DeliveryID,
			SourceEventID: row.SourceEventID,
			Trigger:       row.Trigger,
			Generation:    int(row.Generation),
			NodeID:        looppkg.NodeID(row.NodeID),
			ItemIndex:     int(row.ItemIndex),
			EntryIndex:    int(row.EntryIndex),
			Entry:         json.RawMessage(row.EntryJson),
			State:         state,
			Attempts:      int(row.Attempts),
			CreatedAt:     row.CreatedAt.UTC(),
			DeliveredAt:   loopTimePointer(row.DeliveredAt),
		})
	}
	return entries, nil
}

func insertLoopEffectIntentsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	sourceEventID string,
	intents []looppkg.RenderedEffectIntent,
	at time.Time,
) error {
	queries := sqlcgen.New(exec)
	for _, intent := range intents {
		if err := intent.Validate(); err != nil {
			return err
		}
		deliveryID := looppkg.EffectDeliveryID(run.ID, sourceEventID, intent.Trigger, intent.EntryIndex)
		err := queries.InsertLoopEffectOutbox(ctx, sqlcgen.InsertLoopEffectOutboxParams{
			LoopRunID: string(run.ID), DeliveryID: deliveryID,
			SourceEventID: strings.TrimSpace(sourceEventID), Trigger: string(intent.Trigger),
			Generation: int64(intent.Generation), NodeID: string(intent.NodeID),
			ItemIndex: int64(intent.ItemIndex), EntryIndex: int64(intent.EntryIndex),
			EntryJson: string(intent.Entry), CreatedAt: at.UTC(),
		})
		if err != nil {
			return fmt.Errorf("store: insert loop effect delivery %q: %w", deliveryID, err)
		}
	}
	return nil
}

// ListPendingLoopEffects pages committed pending rows across workspaces through loop_runs ownership.
func (g *LoopRepo) ListPendingLoopEffects(
	ctx context.Context,
	limit int,
) ([]looppkg.EffectOutboxEntry, error) {
	if err := g.checkReady(ctx, "list pending loop effects"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultLoopEffectPageSize
	}
	rows, err := g.queries.ListPendingLoopEffects(ctx, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("store: list pending loop effects: %w", err)
	}
	entries := make([]looppkg.EffectOutboxEntry, 0, len(rows))
	for _, row := range rows {
		entry, convertErr := pendingLoopEffectEntry(row)
		if convertErr != nil {
			return nil, convertErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func pendingLoopEffectEntry(row sqlcgen.ListPendingLoopEffectsRow) (looppkg.EffectOutboxEntry, error) {
	entry := looppkg.EffectOutboxEntry{
		LoopRunID: looppkg.RunID(row.LoopRunID), WorkspaceID: looppkg.WorkspaceID(row.WorkspaceID),
		DeliveryID: row.DeliveryID, SourceEventID: row.SourceEventID, Trigger: row.Trigger,
		Generation: int(row.Generation), NodeID: looppkg.NodeID(row.NodeID), ItemIndex: int(row.ItemIndex),
		EntryIndex: int(row.EntryIndex), Entry: json.RawMessage(row.EntryJson),
		State: looppkg.EffectOutboxState(row.State), Attempts: int(row.Attempts),
		CreatedAt: row.CreatedAt.UTC(), DeliveredAt: loopTimePointer(row.DeliveredAt),
	}
	if !entry.State.Valid() || !json.Valid(entry.Entry) {
		return looppkg.EffectOutboxEntry{}, fmt.Errorf("store: invalid pending loop effect: %w", looppkg.ErrValidation)
	}
	return entry, nil
}

// AcknowledgeLoopEffect atomically appends idempotent result events and closes one pending row.
func (g *LoopRepo) AcknowledgeLoopEffect(
	ctx context.Context,
	ack looppkg.EffectAcknowledgement,
) (bool, error) {
	if err := g.checkReady(ctx, "acknowledge loop effect"); err != nil {
		return false, err
	}
	if err := ack.Validate(); err != nil {
		return false, err
	}
	acknowledged := false
	err := g.withImmediateTransaction(ctx, "acknowledge loop effect", func(exec globalSQLExecutor) error {
		affected, err := sqlcgen.New(exec).ClosePendingLoopEffect(
			ctx,
			sqlcgen.ClosePendingLoopEffectParams{
				State:       string(effectOutboxStateForOutcome(ack.Outcome)),
				DeliveredAt: sql.NullTime{Time: ack.At.UTC(), Valid: true},
				LoopRunID:   string(ack.Entry.LoopRunID),
				DeliveryID:  strings.TrimSpace(ack.Entry.DeliveryID),
			},
		)
		if err != nil {
			return fmt.Errorf("store: close loop effect delivery %q: %w", ack.Entry.DeliveryID, err)
		}
		if affected == 0 {
			return nil
		}
		acknowledged = true
		if len(ack.CustomEvent) > 0 {
			if err := appendLoopDeliveryEventWithExecutor(
				ctx, exec, ack.Entry, loopRunEventCustomEvent, ack.CustomEvent, ack.At,
			); err != nil {
				return err
			}
		}
		return appendLoopDeliveryEventWithExecutor(
			ctx,
			exec,
			ack.Entry,
			loopRunEventEffectResults,
			effectResultEventPayload(ack),
			ack.At,
		)
	})
	if err != nil {
		return false, err
	}
	return acknowledged, nil
}

func effectOutboxStateForOutcome(outcome looppkg.EffectResultOutcome) looppkg.EffectOutboxState {
	if outcome == looppkg.EffectResultOK {
		return looppkg.EffectDelivered
	}
	return looppkg.EffectFailed
}

func effectResultEventPayload(ack looppkg.EffectAcknowledgement) map[string]any {
	return map[string]any{
		"delivery_id":                    ack.Entry.DeliveryID,
		"source_event_id":                ack.Entry.SourceEventID,
		"trigger":                        ack.Entry.Trigger,
		loopRunEventPayloadKeyGeneration: ack.Entry.Generation,
		"node_id":                        ack.Entry.NodeID,
		"item_index":                     ack.Entry.ItemIndex,
		"entry_index":                    ack.Entry.EntryIndex,
		globalDBOutcomeKey:               ack.Outcome,
		"code":                           strings.TrimSpace(ack.Code),
		"cause":                          strings.TrimSpace(ack.Cause),
		"duration_ms":                    ack.Duration.Milliseconds(),
	}
}

func appendLoopDeliveryEventWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	entry looppkg.EffectOutboxEntry,
	kind string,
	payload any,
	at time.Time,
) error {
	payloadJSON, err := normalizeLoopRunEventPayload(kind, payload)
	if err != nil {
		return err
	}
	seq, err := nextLoopRunEventSequence(ctx, exec, entry.LoopRunID)
	if err != nil {
		return err
	}
	deliveryKey := entry.DeliveryID + ":" + kind
	_, err = exec.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO loop_run_events (
			id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		store.NewID("loopevt"), string(entry.LoopRunID), string(entry.WorkspaceID),
		seq, kind, string(payloadJSON), at.UTC(), deliveryKey,
	)
	if err != nil {
		return fmt.Errorf("store: append loop effect event %q: %w", kind, err)
	}
	return nil
}
