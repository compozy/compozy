package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

func writeRequestIntent(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	intent RequestIntent,
) error {
	intent = intent.normalized()
	if err := intent.validate(); err != nil {
		return err
	}
	contextRef := contracts.OutputRefForPayload(intent.Context)
	if err := store.UpsertLoopOutputBlob(ctx, tx, contextRef, intent.Context, intent.OpenedAt); err != nil {
		return fmt.Errorf("loop: persist request context: %w", err)
	}
	var proposedRef any
	if len(intent.Proposed) > 0 {
		ref := contracts.OutputRefForPayload(intent.Proposed)
		if err := store.UpsertLoopOutputBlob(ctx, tx, ref, intent.Proposed, intent.OpenedAt); err != nil {
			return fmt.Errorf("loop: persist request proposal: %w", err)
		}
		proposedRef = ref
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO loop_requests (
		workspace_id, loop_run_id, generation, node_id, item_index, kind, state,
		prompt, context_preview_json, context_ref, answer_schema_json, edit_schema_json,
		respond_schema_json, decisions_json, proposed_ref, proposed_preview_json, opened_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(loop_run_id, generation, node_id, item_index) DO NOTHING`,
		intent.WorkspaceID, loopRunID, generation, intent.NodeID, intent.ItemIndex, intent.Kind,
		intent.Prompt, string(intent.ContextPreview), contextRef, sqlNullRawJSON(intent.AnswerSchema),
		sqlNullRawJSON(intent.EditSchema), sqlNullRawJSON(intent.RespondSchema), string(intent.Decisions),
		proposedRef, sqlNullRawJSON(intent.ProposedPreview), intent.OpenedAt, intent.ExpiresAt)
	if err != nil {
		return fmt.Errorf("loop: write request %s/%d: %w", intent.NodeID, intent.ItemIndex, err)
	}
	return nil
}

func writeNodeWaitIntent(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	intent NodeWaitIntent,
) error {
	intent = intent.normalized()
	if err := intent.validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO loop_node_waits (
		loop_run_id, generation, node_id, item_index, kind, resume_at, next_escalation_at,
		claim_state, claimed_by_kind, claimed_by_id, claimed_at, expect_json,
		ahead_payload_json, issued_epoch, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
		kind = excluded.kind, resume_at = excluded.resume_at,
		next_escalation_at = excluded.next_escalation_at, claim_state = excluded.claim_state,
		claimed_by_kind = excluded.claimed_by_kind, claimed_by_id = excluded.claimed_by_id,
		claimed_at = excluded.claimed_at,
		admission_failures = 0, expect_json = excluded.expect_json,
		ahead_payload_json = excluded.ahead_payload_json, issued_epoch = excluded.issued_epoch,
		created_at = excluded.created_at
	WHERE loop_node_waits.issued_epoch < excluded.issued_epoch`,
		loopRunID, generation, intent.NodeID, intent.ItemIndex, intent.Kind, intent.ResumeAt,
		intent.NextEscalationAt, intent.ClaimState, sqlNullString(intent.ClaimedByKind),
		sqlNullString(intent.ClaimedByID), intent.ClaimedAt, sqlNullRawJSON(intent.Expect),
		sqlNullRawJSON(intent.AheadPayload), intent.IssuedEpoch, intent.CreatedAt)
	if err != nil {
		return fmt.Errorf("loop: write node wait %s/%d: %w", intent.NodeID, intent.ItemIndex, err)
	}
	return nil
}

func sqlNullRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
