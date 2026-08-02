package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	for _, intent := range intents {
		if err := intent.Validate(); err != nil {
			return err
		}
		deliveryID := looppkg.EffectDeliveryID(run.ID, sourceEventID, intent.Trigger, intent.EntryIndex)
		_, err := exec.ExecContext(
			ctx,
			`INSERT INTO loop_effect_outbox (
				loop_run_id, delivery_id, source_event_id, trigger, generation,
				node_id, item_index, entry_index, entry_json, state, attempts, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', 0, ?)`,
			string(run.ID), deliveryID, strings.TrimSpace(sourceEventID), string(intent.Trigger),
			intent.Generation, string(intent.NodeID), intent.ItemIndex, intent.EntryIndex,
			string(intent.Entry), at.UTC(),
		)
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
) (entries []looppkg.EffectOutboxEntry, err error) {
	if err := g.checkReady(ctx, "list pending loop effects"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultLoopEffectPageSize
	}
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT effect.loop_run_id, run.workspace_id, effect.delivery_id,
			effect.source_event_id, effect.trigger, effect.generation, effect.node_id,
			effect.item_index, effect.entry_index, effect.entry_json, effect.state,
			effect.attempts, effect.created_at, effect.delivered_at
		 FROM loop_effect_outbox AS effect
		 JOIN loop_runs AS run ON run.id = effect.loop_run_id
		 WHERE effect.state = 'pending'
		 ORDER BY effect.created_at ASC, effect.loop_run_id ASC, effect.delivery_id ASC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list pending loop effects: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close pending loop effects rows: %w", closeErr))
		}
	}()
	entries = make([]looppkg.EffectOutboxEntry, 0, limit)
	for rows.Next() {
		entry, scanErr := scanPendingLoopEffect(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pending loop effects: %w", err)
	}
	return entries, nil
}

type loopEffectScanner interface {
	Scan(...any) error
}

func scanPendingLoopEffect(row loopEffectScanner) (looppkg.EffectOutboxEntry, error) {
	var entry looppkg.EffectOutboxEntry
	var loopRunID, workspaceID, trigger, state, entryJSON string
	var generation, itemIndex, entryIndex, attempts int64
	var deliveredAt sql.NullTime
	if err := row.Scan(
		&loopRunID, &workspaceID, &entry.DeliveryID, &entry.SourceEventID, &trigger,
		&generation, &entry.NodeID, &itemIndex, &entryIndex, &entryJSON, &state,
		&attempts, &entry.CreatedAt, &deliveredAt,
	); err != nil {
		return looppkg.EffectOutboxEntry{}, fmt.Errorf("store: scan pending loop effect: %w", err)
	}
	entry.LoopRunID = looppkg.RunID(loopRunID)
	entry.WorkspaceID = looppkg.WorkspaceID(workspaceID)
	entry.Trigger = trigger
	entry.Generation = int(generation)
	entry.ItemIndex = int(itemIndex)
	entry.EntryIndex = int(entryIndex)
	entry.Entry = json.RawMessage(entryJSON)
	entry.State = looppkg.EffectOutboxState(state)
	entry.Attempts = int(attempts)
	entry.CreatedAt = entry.CreatedAt.UTC()
	entry.DeliveredAt = loopTimePointer(deliveredAt)
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
		result, err := exec.ExecContext(
			ctx,
			`UPDATE loop_effect_outbox
			 SET state = ?, attempts = attempts + 1, delivered_at = ?
			 WHERE loop_run_id = ? AND delivery_id = ? AND state = 'pending'`,
			string(effectOutboxStateForOutcome(ack.Outcome)), ack.At.UTC(), string(ack.Entry.LoopRunID),
			strings.TrimSpace(ack.Entry.DeliveryID),
		)
		if err != nil {
			return fmt.Errorf("store: close loop effect delivery %q: %w", ack.Entry.DeliveryID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: inspect loop effect delivery %q close: %w", ack.Entry.DeliveryID, err)
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
		loopRunEventPayloadKeyNodeID:     ack.Entry.NodeID,
		loopRunEventPayloadKeyItemIndex:  ack.Entry.ItemIndex,
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
