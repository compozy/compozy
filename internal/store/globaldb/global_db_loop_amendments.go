package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	storepkg "github.com/compozy/compozy/internal/store"
)

var _ looppkg.AmendmentStore = (*LoopRepo)(nil)
var _ looppkg.GenerationOutputOverlayReader = (*LoopRepo)(nil)

func (g *LoopRepo) AmendNodeOutput(
	ctx context.Context,
	input looppkg.AmendInput,
) (looppkg.NodeAmendment, error) {
	if err := g.checkReady(ctx, "amend Loop node output"); err != nil {
		return looppkg.NodeAmendment{}, err
	}
	if err := input.Validate(); err != nil {
		return looppkg.NodeAmendment{}, err
	}
	if err := looppkg.ValidateWaitPayload(input.Schema, input.Payload); err != nil {
		return looppkg.NodeAmendment{}, looppkg.NewRequestValidationError(err)
	}
	var amendment looppkg.NodeAmendment
	err := g.withTaskImmediateTransaction(ctx, "amend Loop node output", func(exec taskSQLExecutor) error {
		var recordedRef, status string
		var paused, runPaused, lanePaused bool
		err := exec.QueryRowContext(ctx, `SELECT COALESCE(output.output_ref, ''), output.status,
			COALESCE(control.paused, 0), run.pause_requested, lane_pause.item_index IS NOT NULL
			FROM loop_generation_outputs AS output
			JOIN loop_runs AS run ON run.id = output.loop_run_id
			LEFT JOIN loop_node_controls AS control
				ON control.loop_run_id = output.loop_run_id AND control.node_id = output.node_id
			LEFT JOIN loop_node_lane_pauses AS lane_pause
				ON lane_pause.loop_run_id = output.loop_run_id AND lane_pause.node_id = output.node_id
				AND lane_pause.item_index = output.item_index
			WHERE run.workspace_id = ? AND output.loop_run_id = ? AND output.generation = ?
				AND output.node_id = ? AND output.item_index = ?`, input.WorkspaceID, input.RunID,
			input.Generation, input.NodeID, input.ItemIndex).Scan(
			&recordedRef, &status, &paused, &runPaused, &lanePaused,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.NewRequestReasonError(looppkg.ReasonCodeAmendNoOutput, looppkg.ErrAmendNoOutput, nil)
		}
		if err != nil {
			return fmt.Errorf("store: load amendment target: %w", err)
		}
		if strings.TrimSpace(recordedRef) == "" || status != "succeeded" {
			return looppkg.NewRequestReasonError(looppkg.ReasonCodeAmendNoOutput, looppkg.ErrAmendNoOutput, nil)
		}
		if !paused && !runPaused && !lanePaused && status != "paused" && status != "waiting" {
			return looppkg.NewRequestReasonError(looppkg.ReasonCodeAmendNotParked, looppkg.ErrAmendNotParked, nil)
		}
		var sequence int
		var previousRef sql.NullString
		if err := exec.QueryRowContext(ctx, `SELECT COALESCE(MAX(amendment_seq), 0),
			(SELECT amended_ref FROM loop_node_amendments WHERE loop_run_id = ? AND generation = ?
				AND node_id = ? AND item_index = ? ORDER BY amendment_seq DESC LIMIT 1)
			FROM loop_node_amendments WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			input.RunID, input.Generation, input.NodeID, input.ItemIndex,
			input.RunID, input.Generation, input.NodeID, input.ItemIndex).Scan(&sequence, &previousRef); err != nil {
			return fmt.Errorf("store: load amendment sequence: %w", err)
		}
		originalRef := recordedRef
		if previousRef.Valid {
			originalRef = previousRef.String
		}
		amendedRef := looppkg.OutputRefForPayload(input.Payload)
		if err := storepkg.UpsertLoopOutputBlob(ctx, exec, amendedRef, input.Payload, input.RequestedAt); err != nil {
			return err
		}
		actorKind := string(input.Actor.Actor.Kind.Normalize())
		actorID := strings.TrimSpace(input.Actor.Actor.Ref)
		sequence++
		_, err = exec.ExecContext(ctx, `INSERT INTO loop_node_amendments (
			workspace_id, loop_run_id, generation, node_id, item_index, amendment_seq,
			original_ref, amended_ref, actor_kind, actor_id, reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, input.WorkspaceID, input.RunID,
			input.Generation, input.NodeID, input.ItemIndex, sequence, originalRef, amendedRef,
			actorKind, actorID, strings.TrimSpace(input.Reason), input.RequestedAt.UTC())
		if err != nil {
			return fmt.Errorf("store: insert Loop node amendment: %w", err)
		}
		if err := appendLoopRunEventWithExecutor(ctx, exec, input.RunID, input.WorkspaceID,
			loopRunEventNodeAmended, map[string]any{
				"generation": input.Generation, "node_id": input.NodeID, "item_index": input.ItemIndex,
				"amendment_seq": sequence, "actor_kind": actorKind, "actor_id": actorID,
			}, input.RequestedAt); err != nil {
			return err
		}
		originalPayload, err := getLoopOutputByRefWithExecutor(ctx, exec, originalRef)
		if err != nil {
			return fmt.Errorf("store: load original Loop node amendment value: %w", err)
		}
		amendment = looppkg.NodeAmendment{
			WorkspaceID: input.WorkspaceID, LoopRunID: input.RunID, Generation: input.Generation,
			NodeID: input.NodeID, ItemIndex: input.ItemIndex, Sequence: sequence,
			OriginalRef: originalRef, AmendedRef: amendedRef, ActorKind: actorKind, ActorID: actorID,
			Original: originalPayload, Amended: append([]byte(nil), input.Payload...),
			Reason: strings.TrimSpace(input.Reason), CreatedAt: input.RequestedAt.UTC(),
		}
		return nil
	})
	return amendment, err
}

func (g *LoopRepo) ListNodeAmendments(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.NodeAmendment, error) {
	rows, err := g.db.QueryContext(ctx, `SELECT amendment.workspace_id, amendment.loop_run_id,
		amendment.generation, amendment.node_id, amendment.item_index, amendment.amendment_seq,
		amendment.original_ref, amendment.amended_ref, amendment.actor_kind, amendment.actor_id,
		COALESCE(amendment.reason, ''), amendment.created_at
		FROM loop_node_amendments AS amendment JOIN loop_runs AS run ON run.id = amendment.loop_run_id
		WHERE run.workspace_id = ? AND amendment.loop_run_id = ?
		ORDER BY amendment.generation, amendment.node_id, amendment.item_index, amendment.amendment_seq`,
		workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop node amendments: %w", err)
	}
	amendments := make([]looppkg.NodeAmendment, 0)
	for rows.Next() {
		var amendment looppkg.NodeAmendment
		if err := rows.Scan(&amendment.WorkspaceID, &amendment.LoopRunID, &amendment.Generation,
			&amendment.NodeID, &amendment.ItemIndex, &amendment.Sequence, &amendment.OriginalRef,
			&amendment.AmendedRef, &amendment.ActorKind, &amendment.ActorID, &amendment.Reason,
			&amendment.CreatedAt); err != nil {
			return nil, errors.Join(fmt.Errorf("store: scan Loop node amendment: %w", err), rows.Close())
		}
		amendment.Original, err = getLoopOutputByRefWithExecutor(ctx, g.db, amendment.OriginalRef)
		if err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		amendment.Amended, err = getLoopOutputByRefWithExecutor(ctx, g.db, amendment.AmendedRef)
		if err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		amendment.CreatedAt = amendment.CreatedAt.UTC()
		amendments = append(amendments, amendment)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("store: iterate Loop node amendments: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: close Loop node amendments: %w", err)
	}
	return amendments, nil
}

func (g *LoopRepo) ApplyGenerationOutputOverlays(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	generation int,
	outputs []looppkg.GenerationOutput,
) ([]looppkg.GenerationOutput, error) {
	view := append([]looppkg.GenerationOutput(nil), outputs...)
	rows, err := g.db.QueryContext(ctx, `SELECT amendment.node_id, amendment.item_index, amendment.amended_ref
		FROM loop_node_amendments AS amendment JOIN loop_runs AS run ON run.id = amendment.loop_run_id
		WHERE run.workspace_id = ? AND amendment.loop_run_id = ? AND amendment.generation = ?
		AND amendment.amendment_seq = (SELECT MAX(latest.amendment_seq) FROM loop_node_amendments AS latest
			WHERE latest.loop_run_id = amendment.loop_run_id AND latest.generation = amendment.generation
			AND latest.node_id = amendment.node_id AND latest.item_index = amendment.item_index)`,
		workspaceID, runID, generation)
	if err != nil {
		return nil, fmt.Errorf("store: list effective Loop output overlays: %w", err)
	}
	overlays := make(map[string]string)
	for rows.Next() {
		var nodeID string
		var itemIndex int
		var ref string
		if err := rows.Scan(&nodeID, &itemIndex, &ref); err != nil {
			return nil, errors.Join(fmt.Errorf("store: scan effective Loop output overlay: %w", err), rows.Close())
		}
		overlays[fmt.Sprintf("%s\x00%d", nodeID, itemIndex)] = ref
	}
	for index := range view {
		if ref := overlays[fmt.Sprintf("%s\x00%d", view[index].NodeID, view[index].ItemIndex)]; ref != "" {
			view[index].OutputRef = ref
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("store: iterate effective Loop output overlays: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: close effective Loop output overlays: %w", err)
	}
	return view, nil
}
