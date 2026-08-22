package loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

func writeNodeControlMutation(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	mutation NodeControlMutation,
) error {
	mutation = mutation.normalized()
	if err := mutation.validate(); err != nil {
		return err
	}
	var result sql.Result
	var err error
	if mutation.ExpectExisting {
		result, err = updateExistingNodeControl(ctx, tx, loopRunID, mutation)
	} else {
		result, err = insertNewNodeControl(ctx, tx, loopRunID, mutation)
	}
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("loop: inspect node control mutation: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: node control %q revision changed", ErrTransitionConflict, mutation.NodeID)
	}
	if mutation.Kind == NodeControlMutationCancel && mutation.CancelState == CancelStateCanceled {
		return claimCanceledNodeWaits(ctx, tx, loopRunID, mutation)
	}
	return nil
}

func claimCanceledNodeWaits(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	mutation NodeControlMutation,
) error {
	_, err := tx.ExecContext(
		ctx,
		`UPDATE loop_requests SET state = 'canceled', resolved_at = ?,
			actor_kind = (SELECT cancel_actor_kind FROM loop_node_controls
				WHERE loop_run_id = ? AND node_id = ?),
			actor_id = (SELECT cancel_actor_id FROM loop_node_controls
				WHERE loop_run_id = ? AND node_id = ?)
		 WHERE loop_run_id = ? AND node_id = ? AND state = 'pending'`,
		mutation.At.UTC(), loopRunID, mutation.NodeID, loopRunID, mutation.NodeID,
		loopRunID, mutation.NodeID,
	)
	if err != nil {
		return fmt.Errorf("loop: cancel node %q requests: %w", mutation.NodeID, err)
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE loop_node_waits SET
			claim_state = 'claimed',
			claimed_by_kind = (
				SELECT cancel_actor_kind FROM loop_node_controls
				WHERE loop_run_id = ? AND node_id = ?
			),
			claimed_by_id = (
				SELECT cancel_actor_id FROM loop_node_controls
				WHERE loop_run_id = ? AND node_id = ?
			),
			claimed_at = ?
		WHERE loop_run_id = ? AND node_id = ?
		  AND claim_state IN ('waiting','intervention_required')`,
		loopRunID,
		mutation.NodeID,
		loopRunID,
		mutation.NodeID,
		mutation.At.UTC(),
		loopRunID,
		mutation.NodeID,
	)
	if err != nil {
		return fmt.Errorf("loop: claim canceled node %q waits: %w", mutation.NodeID, err)
	}
	return nil
}

func insertNewNodeControl(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	mutation NodeControlMutation,
) (sql.Result, error) {
	values, err := nodeControlMutationValues(mutation)
	if err != nil {
		return nil, err
	}
	paused := 0
	if mutation.Kind == NodeControlMutationPause {
		paused = 1
	}
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_node_controls (
			loop_run_id, node_id, paused, pause_actor_kind, pause_actor_id, pause_reason,
			pause_rule_id, pause_requested_at, quarantined, quarantine_entry_json, quarantined_at,
			attention_flag, attention_reason, attention_producer_node_id, gate_revisions_json,
			revision, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(loop_run_id, node_id) DO NOTHING`,
		loopRunID, mutation.NodeID, paused, sqlNullString(mutation.PauseActorKind),
		sqlNullString(mutation.PauseActorID), sqlNullString(mutation.PauseReason),
		sqlNullString(mutation.PauseRuleID), sqlNullTimeForMutation(mutation),
		values.quarantined, values.entry, values.quarantinedAt,
		values.attentionFlag, values.attentionReason, values.attentionProducer,
		values.gateRevisions, mutation.At,
	)
	if err != nil {
		return nil, fmt.Errorf("loop: insert node control %q: %w", mutation.NodeID, err)
	}
	return result, nil
}

func updateExistingNodeControl(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	mutation NodeControlMutation,
) (sql.Result, error) {
	values, err := nodeControlMutationValues(mutation)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE loop_node_controls SET
			paused = CASE WHEN ? = 'pause' THEN 1 ELSE paused END,
			pause_actor_kind = CASE WHEN ? = 'pause' THEN ? ELSE pause_actor_kind END,
			pause_actor_id = CASE WHEN ? = 'pause' THEN ? ELSE pause_actor_id END,
			pause_reason = CASE WHEN ? = 'pause' THEN ? ELSE pause_reason END,
			pause_rule_id = CASE WHEN ? = 'pause' THEN ? ELSE pause_rule_id END,
			pause_requested_at = CASE WHEN ? = 'pause' THEN ? ELSE pause_requested_at END,
			quarantined = CASE WHEN ? = 'quarantine' THEN ? ELSE quarantined END,
			quarantine_entry_json = CASE WHEN ? = 'quarantine' THEN ? ELSE quarantine_entry_json END,
			quarantined_at = CASE WHEN ? = 'quarantine' THEN ? ELSE quarantined_at END,
			attention_flag = CASE WHEN ? = 'attention' THEN ? ELSE attention_flag END,
			attention_reason = CASE WHEN ? = 'attention' THEN ? ELSE attention_reason END,
			attention_producer_node_id = CASE WHEN ? = 'attention' THEN ? ELSE attention_producer_node_id END,
			cancel_state = CASE WHEN ? = 'cancel' THEN ? ELSE cancel_state END,
			gate_revisions_json = CASE WHEN ? = 'gate_revision' THEN ? ELSE gate_revisions_json END,
			revision = revision + 1,
			updated_at = ?
		WHERE loop_run_id = ? AND node_id = ? AND revision = ?`,
		mutation.Kind,
		mutation.Kind, sqlNullString(mutation.PauseActorKind),
		mutation.Kind, sqlNullString(mutation.PauseActorID),
		mutation.Kind, sqlNullString(mutation.PauseReason),
		mutation.Kind, sqlNullString(mutation.PauseRuleID),
		mutation.Kind, sqlNullTimeForMutation(mutation),
		mutation.Kind, values.quarantined,
		mutation.Kind, values.entry,
		mutation.Kind, values.quarantinedAt,
		mutation.Kind, values.attentionFlag,
		mutation.Kind, values.attentionReason,
		mutation.Kind, values.attentionProducer,
		mutation.Kind, mutation.CancelState,
		mutation.Kind, values.gateRevisions,
		mutation.At, loopRunID, mutation.NodeID, mutation.ExpectedRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("loop: update node control %q: %w", mutation.NodeID, err)
	}
	return result, nil
}

func sqlNullTimeForMutation(mutation NodeControlMutation) any {
	if mutation.Kind != NodeControlMutationPause {
		return nil
	}
	return mutation.At.UTC()
}

type nodeControlSQLValues struct {
	quarantined       int
	entry             any
	quarantinedAt     any
	attentionFlag     string
	attentionReason   string
	attentionProducer string
	gateRevisions     string
}

func nodeControlMutationValues(mutation NodeControlMutation) (nodeControlSQLValues, error) {
	values := nodeControlSQLValues{gateRevisions: "{}"}
	if mutation.Kind == NodeControlMutationGateRevision {
		encoded, err := json.Marshal(mutation.GateRevisions)
		if err != nil {
			return nodeControlSQLValues{}, fmt.Errorf("loop: encode gate revision counters: %w", err)
		}
		values.gateRevisions = string(encoded)
		return values, nil
	}
	if mutation.Kind == NodeControlMutationQuarantine {
		values.quarantined = 1
		values.entry = string(mutation.QuarantineEntry)
		values.quarantinedAt = mutation.At
		return values, nil
	}
	values.attentionFlag = mutation.AttentionFlag
	values.attentionReason = mutation.AttentionReason
	values.attentionProducer = mutation.AttentionProducerNodeID
	return values, nil
}
