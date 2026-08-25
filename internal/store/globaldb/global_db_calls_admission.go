package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *CallRepo) AdmitCall(
	ctx context.Context,
	admission callspkg.Admission,
) (result callspkg.AdmissionResult, err error) {
	if err := g.checkReady(ctx, "admit call"); err != nil {
		return callspkg.AdmissionResult{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "admit call", func(exec taskSQLExecutor) error {
		existing, found, lookupErr := findIdempotentCall(ctx, exec, admission.Record)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			if existing.RequestDigest != admission.Record.RequestDigest {
				return &callspkg.Error{
					Code:    callspkg.CodeIdempotencyConflict,
					Message: fmt.Sprintf("idempotency key is already bound to call %q", existing.CallID),
				}
			}
			result = callspkg.AdmissionResult{Record: existing, Replayed: true}
			return nil
		}
		if err := validateCallAdmissionFence(ctx, exec, admission); err != nil {
			return err
		}
		if admission.Contract != nil {
			if err := putCallContract(ctx, exec, *admission.Contract, admission.Record.CreatedAt); err != nil {
				return err
			}
		}
		if err := putCallPayload(
			ctx, exec, admission.Record.WorkspaceID, admission.Record.PromptRef,
			admission.Prompt, admission.Record.CreatedAt,
		); err != nil {
			return err
		}
		if err := insertCall(ctx, exec, admission.Record); err != nil {
			return err
		}
		if err := insertCallPermissionAtoms(ctx, exec, admission.Record.CallID, admission.Permissions); err != nil {
			return err
		}
		if admission.Activation != nil {
			if err := insertCallActivation(ctx, exec, g.tasks, *admission.Activation, admission.Record.CreatedAt); err != nil {
				return err
			}
		}
		if admission.FollowUp != nil {
			if err := insertCallFollowUpDelivery(ctx, exec, admission.Record, *admission.FollowUp); err != nil {
				return err
			}
		}
		stored, err := getCallWithExecutor(ctx, exec, callspkg.CallScope{
			ProfileID: admission.Record.ProfileID, Scope: admission.Record.Scope,
			WorkspaceID: admission.Record.WorkspaceID,
		}, admission.Record.CallID)
		if err != nil {
			return err
		}
		result = callspkg.AdmissionResult{Record: stored}
		return nil
	})
	if err != nil {
		return callspkg.AdmissionResult{}, err
	}
	return result, nil
}

func findIdempotentCall(
	ctx context.Context,
	exec taskSQLExecutor,
	record callspkg.CallRecord,
) (callspkg.CallRecord, bool, error) {
	if strings.TrimSpace(record.IdempotencyKey) == "" {
		return callspkg.CallRecord{}, false, nil
	}
	row := exec.QueryRowContext(ctx, `SELECT `+callSelectColumnsSQL+` FROM calls
		WHERE profile_id = ? AND scope = ? AND workspace_id = ?
		AND caller_kind = ? AND caller_id = ? AND idempotency_key = ?`,
		record.ProfileID, string(record.Scope), record.WorkspaceID,
		string(record.Caller.Kind), record.Caller.ID, record.IdempotencyKey,
	)
	existing, err := scanCallRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, false, nil
	}
	if err != nil {
		return callspkg.CallRecord{}, false, fmt.Errorf("store: find idempotent call: %w", err)
	}
	return existing, true, nil
}

func validateCallAdmissionFence(
	ctx context.Context,
	exec taskSQLExecutor,
	admission callspkg.Admission,
) error {
	record := admission.Record
	var profileID, workspaceID string
	var drainingAt sql.NullString
	err := exec.QueryRowContext(ctx, `SELECT profile_id, workspace_id, draining_at
		FROM sessions WHERE id = ?`, record.GovernedRootID).Scan(&profileID, &workspaceID, &drainingAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &callspkg.Error{Code: callspkg.CodeParentTerminal, Message: "governed root is unavailable"}
	}
	if err != nil {
		return fmt.Errorf("store: inspect governed call root %q: %w", record.GovernedRootID, err)
	}
	if profileID != record.ProfileID {
		return &callspkg.Error{Code: callspkg.CodeTargetDenied, Message: "governed root belongs to another profile"}
	}
	if record.Scope == callspkg.ScopeWorkspace && workspaceID != record.WorkspaceID {
		return &callspkg.Error{Code: callspkg.CodeWorkspaceDenied, Message: "governed root belongs to another workspace"}
	}
	if drainingAt.Valid {
		return &callspkg.Error{Code: callspkg.CodeParentTerminal, Message: "governed root is draining"}
	}
	if strings.TrimSpace(record.AgentName) == "" || admission.MaxChildren <= 0 {
		return nil
	}
	var liveChildren int
	err = exec.QueryRowContext(ctx, `SELECT COUNT(1) FROM sessions
		WHERE parent_session_id = ? AND state IN ('starting', 'active', 'stopping')`,
		record.ParentSessionID,
	).Scan(&liveChildren)
	if err != nil {
		return fmt.Errorf("store: count live call children: %w", err)
	}
	if liveChildren >= admission.MaxChildren {
		return &callspkg.Error{
			Code:    callspkg.CodeChildrenCap,
			Message: fmt.Sprintf("parent has %d live children; maximum is %d", liveChildren, admission.MaxChildren),
		}
	}
	return nil
}

func insertCall(ctx context.Context, exec taskSQLExecutor, record callspkg.CallRecord) error {
	_, err := exec.ExecContext(ctx, `INSERT INTO calls (
		call_id, profile_id, scope, workspace_id, caller_kind, caller_id, actor_kind, actor_id,
		parent_session_id, agent_name, child_session_id, governed_root_id, depth, state, expect_digest,
		prompt_ref, result_budget_bytes, result_overflow, strict, idle_ttl_seconds,
		runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
		idempotency_key, request_digest, batch_id, deadline_at, created_at, started_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.CallID, record.ProfileID, string(record.Scope), record.WorkspaceID,
		string(record.Caller.Kind), record.Caller.ID, record.Actor.Kind, record.Actor.ID,
		nullableTaskString(record.ParentSessionID), nullableTaskString(record.AgentName),
		nullableTaskString(record.ChildSessionID), record.GovernedRootID, record.Depth, string(record.State),
		nullableTaskString(record.ExpectDigest), record.PromptRef, record.ResultBudget.MaxBytes,
		string(record.ResultBudget.Overflow), boolInt64(record.Strict), durationSecondsCeil(record.IdleTTL),
		record.Runtime.Provider, record.Runtime.Model, record.Runtime.ReasoningEffort, string(record.Runtime.Speed),
		nullableTaskString(record.IdempotencyKey), record.RequestDigest, nullableTaskString(record.BatchID),
		nullableTaskTime(record.DeadlineAt), store.FormatTimestamp(record.CreatedAt),
		nullableTaskTime(record.StartedAt), store.FormatTimestamp(record.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("store: insert call %q: %w", record.CallID, err)
	}
	return nil
}

func insertCallPermissionAtoms(
	ctx context.Context,
	exec taskSQLExecutor,
	callID string,
	atoms []string,
) error {
	for _, atom := range atoms {
		if _, err := exec.ExecContext(ctx,
			`INSERT INTO call_permission_atoms (call_id, atom) VALUES (?, ?)`, callID, atom,
		); err != nil {
			return fmt.Errorf("store: insert permission atom for call %q: %w", callID, err)
		}
	}
	return nil
}

func insertCallActivation(
	ctx context.Context,
	exec taskSQLExecutor,
	tasks *TaskRepo,
	activation callspkg.ActivationSpec,
	at time.Time,
) error {
	run := taskpkg.Run{
		ID: activation.RunID, WorkspaceID: activation.WorkspaceID,
		RunKind: taskpkg.RunKindCallActivation, Status: taskpkg.TaskRunStatusQueued, Attempt: 1,
		Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "calls.admission:" + activation.CallID},
		IdempotencyKey: "call-activation:" + activation.CallID, QueuedAt: at,
	}
	run.SetNetworkState(participation.LocalSpec(), "", "", "")
	normalized, err := tasks.normalizeTaskRunForCreate(run)
	if err != nil {
		return fmt.Errorf("store: normalize call activation run: %w", err)
	}
	if err := insertTaskRunWithExecutor(ctx, exec, normalized); err != nil {
		return fmt.Errorf("store: insert call activation run: %w", err)
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO call_activation_runs (
		run_id, call_id, workspace_id, governed_root_id, activation_kind,
		parent_session_id, target_session_id, agent_name, depth, idle_ttl_seconds,
		runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		activation.RunID, activation.CallID, activation.WorkspaceID, activation.GovernedRootID,
		activation.Kind, nullableTaskString(activation.ParentSessionID),
		nullableTaskString(activation.TargetSessionID), nullableTaskString(activation.AgentName),
		activation.Depth, durationSecondsCeil(activation.IdleTTL), activation.Runtime.Provider,
		activation.Runtime.Model, activation.Runtime.ReasoningEffort, string(activation.Runtime.Speed),
		store.FormatTimestamp(at),
	)
	if err != nil {
		return fmt.Errorf("store: insert call activation details: %w", err)
	}
	result, err := exec.ExecContext(ctx,
		`UPDATE calls SET activation_run_id = ? WHERE call_id = ? AND activation_run_id IS NULL`,
		activation.RunID, activation.CallID,
	)
	if err != nil {
		return fmt.Errorf("store: bind call activation run: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect call activation binding: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("store: call %q activation binding was fenced", activation.CallID)
	}
	return nil
}

func insertCallFollowUpDelivery(
	ctx context.Context,
	exec taskSQLExecutor,
	record callspkg.CallRecord,
	delivery callspkg.Delivery,
) error {
	suffix := strings.TrimPrefix(record.CallID, "call_")
	deliveryID := "delivery_message_" + suffix
	wakeID := "wake_message_" + suffix
	_, err := exec.ExecContext(ctx, `INSERT INTO call_deliveries (
		delivery_id, kind, subject_id, recipient_session_id, owner_key, wake_event_id,
		state, created_at, updated_at
	) VALUES (?, 'message', ?, ?, ?, ?, 'pending', ?, ?)`,
		deliveryID, record.CallID, delivery.RecipientSessionID, participation.OwnerKey(record.Caller), wakeID,
		store.FormatTimestamp(record.CreatedAt), store.FormatTimestamp(record.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("store: insert follow-up delivery for call %q: %w", record.CallID, err)
	}
	return nil
}

func durationSecondsCeil(value time.Duration) int64 {
	seconds := int64(value / time.Second)
	if value%time.Second != 0 {
		seconds++
	}
	return seconds
}
