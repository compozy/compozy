package loop

import (
	"context"
	"database/sql"
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
	return nil
}

func insertNewNodeControl(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	mutation NodeControlMutation,
) (sql.Result, error) {
	quarantined, entry, quarantinedAt, attentionFlag, attentionReason := nodeControlMutationValues(mutation)
	paused := 0
	if mutation.Kind == NodeControlMutationPause {
		paused = 1
	}
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_node_controls (
			loop_run_id, node_id, paused, pause_actor_kind, pause_actor_id, pause_reason,
			pause_rule_id, pause_requested_at, quarantined, quarantine_entry_json, quarantined_at,
			attention_flag, attention_reason, revision, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(loop_run_id, node_id) DO NOTHING`,
		loopRunID, mutation.NodeID, paused, sqlNullString(mutation.PauseActorKind),
		sqlNullString(mutation.PauseActorID), sqlNullString(mutation.PauseReason),
		sqlNullString(mutation.PauseRuleID), sqlNullTimeForMutation(mutation), quarantined, entry, quarantinedAt,
		attentionFlag, attentionReason, mutation.At,
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
	quarantined, entry, quarantinedAt, attentionFlag, attentionReason := nodeControlMutationValues(mutation)
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
			cancel_state = CASE WHEN ? = 'cancel' THEN ? ELSE cancel_state END,
			revision = revision + 1,
			updated_at = ?
		WHERE loop_run_id = ? AND node_id = ? AND revision = ?`,
		mutation.Kind,
		mutation.Kind, sqlNullString(mutation.PauseActorKind),
		mutation.Kind, sqlNullString(mutation.PauseActorID),
		mutation.Kind, sqlNullString(mutation.PauseReason),
		mutation.Kind, sqlNullString(mutation.PauseRuleID),
		mutation.Kind, sqlNullTimeForMutation(mutation),
		mutation.Kind, quarantined,
		mutation.Kind, entry,
		mutation.Kind, quarantinedAt,
		mutation.Kind, attentionFlag,
		mutation.Kind, attentionReason,
		mutation.Kind, mutation.CancelState,
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

func nodeControlMutationValues(mutation NodeControlMutation) (int, any, any, string, string) {
	if mutation.Kind == NodeControlMutationQuarantine {
		return 1, string(mutation.QuarantineEntry), mutation.At, "", ""
	}
	return 0, nil, nil, mutation.AttentionFlag, mutation.AttentionReason
}
