package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *CallRepo) ListQueuedActivationRunIDs(ctx context.Context, limit int) (ids []string, err error) {
	if err := g.checkReady(ctx, "list queued call activations"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := g.db.QueryContext(ctx, `SELECT tr.id FROM task_runs tr
		JOIN call_activation_runs a ON a.run_id = tr.id
		JOIN calls c ON c.call_id = a.call_id
		JOIN profiles p ON p.id = c.profile_id
		WHERE tr.run_kind = 'call_activation' AND tr.status = 'queued'
		AND c.state = 'queued' AND p.state = 'active'
		ORDER BY tr.queued_at, tr.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list queued call activations: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "queued call activations") }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan queued call activation: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate queued call activations: %w", err)
	}
	return ids, nil
}

func (g *CallRepo) LoadActivation(
	ctx context.Context,
	runID string,
) (callspkg.CallRecord, callspkg.ActivationSpec, []byte, callspkg.PermissionAtoms, error) {
	if err := g.checkReady(ctx, "load call activation"); err != nil {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, err
	}
	id := strings.TrimSpace(runID)
	record, err := scanCallRecord(g.db.QueryRowContext(ctx, `SELECT `+callSelectColumnsSQL+` FROM calls
		WHERE call_id = (SELECT call_id FROM call_activation_runs WHERE run_id = ?)`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, callNotFound(id)
	}
	if err != nil {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, err
	}
	activation := callspkg.ActivationSpec{RunID: id, CallID: record.CallID}
	var idleSeconds int64
	err = g.db.QueryRowContext(ctx, `SELECT workspace_id, governed_root_id, activation_kind,
		COALESCE(parent_session_id, ''), COALESCE(target_session_id, ''), COALESCE(agent_name, ''),
		depth, idle_ttl_seconds, runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed
		FROM call_activation_runs WHERE run_id = ?`, id).Scan(
		&activation.WorkspaceID, &activation.GovernedRootID, &activation.Kind,
		&activation.ParentSessionID, &activation.TargetSessionID, &activation.AgentName,
		&activation.Depth, &idleSeconds, &activation.Runtime.Provider, &activation.Runtime.Model,
		&activation.Runtime.ReasoningEffort, &activation.Runtime.Speed,
	)
	if err != nil {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, fmt.Errorf("store: load call activation %q: %w", id, err)
	}
	activation.IdleTTL = time.Duration(idleSeconds) * time.Second
	prompt, err := g.loadVerifiedCallPayload(ctx, record.WorkspaceID, record.PromptRef)
	if err != nil {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, err
	}
	permissions, err := g.loadCallPermissionAtoms(ctx, record.CallID)
	if err != nil {
		return callspkg.CallRecord{}, callspkg.ActivationSpec{}, nil, callspkg.PermissionAtoms{}, err
	}
	return record, activation, prompt, permissions, nil
}

func (g *CallRepo) loadVerifiedCallPayload(ctx context.Context, workspaceID, ref string) ([]byte, error) {
	row, err := g.queries.GetCallPayload(ctx, sqlcgen.GetCallPayloadParams{
		WorkspaceID: workspaceID, Ref: ref,
	})
	if err != nil {
		return nil, fmt.Errorf("store: get call payload %q: %w", ref, err)
	}
	return verifyCallBlob("payload", ref, row.Bytes, &row.ByteSize)
}

func (g *CallRepo) loadCallPermissionAtoms(
	ctx context.Context,
	callID string,
) (result callspkg.PermissionAtoms, err error) {
	rows, err := g.db.QueryContext(ctx, `SELECT atom FROM call_permission_atoms WHERE call_id = ? ORDER BY atom`, callID)
	if err != nil {
		return callspkg.PermissionAtoms{}, fmt.Errorf("store: list call permissions: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "call permissions") }()
	encoded := make([]string, 0)
	for rows.Next() {
		var atom string
		if err := rows.Scan(&atom); err != nil {
			return callspkg.PermissionAtoms{}, fmt.Errorf("store: scan call permission: %w", err)
		}
		encoded = append(encoded, atom)
	}
	if err := rows.Err(); err != nil {
		return callspkg.PermissionAtoms{}, fmt.Errorf("store: iterate call permissions: %w", err)
	}
	result, err = callspkg.DecodePermissionAtoms(encoded)
	if err != nil {
		return callspkg.PermissionAtoms{}, fmt.Errorf("store: decode call permissions: %w", err)
	}
	return result, nil
}

func (g *CallRepo) ReconcileActivations(ctx context.Context, at time.Time) (roots []string, err error) {
	if err := g.checkReady(ctx, "reconcile call activations"); err != nil {
		return nil, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "reconcile call activations", func(exec taskSQLExecutor) error {
		formatted := store.FormatTimestamp(at)
		if _, err := exec.ExecContext(ctx, `UPDATE calls SET state = 'queued', started_at = NULL, updated_at = ?
			WHERE state = 'running' AND (
				child_session_id IS NULL OR EXISTS (
					SELECT 1 FROM call_activation_runs revive
					WHERE revive.run_id = calls.activation_run_id AND revive.activation_kind = 'revive'
				)
			) AND activation_run_id IN (
				SELECT id FROM task_runs WHERE run_kind = 'call_activation' AND status IN ('claimed','starting','running')
			)`, formatted); err != nil {
			return fmt.Errorf("store: reset interrupted calls: %w", err)
		}
		if _, err := exec.ExecContext(ctx, `UPDATE task_runs SET status = 'queued', claimed_by_kind = NULL,
			claimed_by_ref = NULL, claim_token = NULL, claim_token_hash = NULL, lease_until = NULL,
			heartbeat_at = NULL, claimed_at = NULL, started_at = NULL, session_id = NULL
			WHERE run_kind = 'call_activation' AND status IN ('claimed','starting','running')
			AND EXISTS (SELECT 1 FROM calls c WHERE c.activation_run_id = task_runs.id
				AND c.state = 'queued' AND (
					c.child_session_id IS NULL OR EXISTS (
						SELECT 1 FROM call_activation_runs revive
						WHERE revive.run_id = task_runs.id AND revive.activation_kind = 'revive'
					)
				))`); err != nil {
			return fmt.Errorf("store: requeue interrupted call activations: %w", err)
		}
		if _, err := exec.ExecContext(ctx, `UPDATE calls SET
			state = CASE WHEN (
				SELECT error FROM task_runs WHERE id = calls.activation_run_id
			) = 'call deadline elapsed' THEN 'timeout' ELSE 'canceled' END,
			failure_code = CASE WHEN (
				SELECT error FROM task_runs WHERE id = calls.activation_run_id
			) = 'call deadline elapsed' THEN 'call_timeout' ELSE 'call_canceled' END,
			failure_detail = CASE WHEN (
				SELECT error FROM task_runs WHERE id = calls.activation_run_id
			) = 'subtree drain' THEN 'subtree drained' ELSE COALESCE((
				SELECT error FROM task_runs WHERE id = calls.activation_run_id
			), 'activation canceled') END,
			settled_at = ?, updated_at = ?
			WHERE state IN ('queued','running') AND activation_run_id IN (
				SELECT id FROM task_runs WHERE run_kind = 'call_activation' AND status = 'canceled'
			)`, formatted, formatted); err != nil {
			return fmt.Errorf("store: settle interrupted canceled call activations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return listDrainingSessionIDs(ctx, g.db)
}

func listDrainingSessionIDs(ctx context.Context, db *sql.DB) (values []string, err error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM sessions WHERE draining_at IS NOT NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list draining session IDs: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "draining session IDs") }()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("store: scan draining session ID: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate draining session IDs: %w", err)
	}
	return values, nil
}
