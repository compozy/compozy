package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	maxCallDetailBytes = 2048
	maxIssueTextBytes  = 4096
)

func (g *CallRepo) BindActivationChild(
	ctx context.Context,
	binding callspkg.ActivationBinding,
) (record callspkg.CallRecord, err error) {
	if err := g.checkReady(ctx, "bind call activation child"); err != nil {
		return callspkg.CallRecord{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "bind call activation child", func(exec taskSQLExecutor) error {
		if err := finishActivationRun(ctx, exec, binding.RunID, binding.ClaimToken, "completed", "", binding.ActivatedAt); err != nil {
			return err
		}
		result, err := exec.ExecContext(ctx, `UPDATE calls
			SET child_session_id = ?, state = 'running', started_at = COALESCE(started_at, ?), updated_at = ?
			WHERE call_id = ? AND activation_run_id = ? AND state = 'running'`,
			strings.TrimSpace(binding.ChildID), store.FormatTimestamp(binding.ActivatedAt),
			store.FormatTimestamp(binding.ActivatedAt), strings.TrimSpace(binding.CallID), strings.TrimSpace(binding.RunID),
		)
		if err != nil {
			return fmt.Errorf("store: bind child to call %q: %w", binding.CallID, err)
		}
		if err := requireOneCallRow(result, binding.CallID, "bind activation child"); err != nil {
			return err
		}
		if _, err := exec.ExecContext(ctx, `UPDATE sessions
			SET parked_at = NULL, idle_expires_at = NULL, updated_at = ? WHERE id = ?`,
			store.FormatTimestamp(binding.ActivatedAt), strings.TrimSpace(binding.ChildID)); err != nil {
			return fmt.Errorf("store: clear revived child idle state %q: %w", binding.ChildID, err)
		}
		var loadErr error
		record, loadErr = getCallByIDWithExecutor(ctx, exec, binding.CallID)
		return loadErr
	})
	return record, err
}

func (g *CallRepo) FailActivation(
	ctx context.Context,
	failure callspkg.ActivationFailure,
) (record callspkg.CallRecord, err error) {
	if err := g.checkReady(ctx, "fail call activation"); err != nil {
		return callspkg.CallRecord{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "fail call activation", func(exec taskSQLExecutor) error {
		if err := finishActivationRun(
			ctx, exec, failure.RunID, failure.ClaimToken, "failed", failure.Detail, failure.FailedAt,
		); err != nil {
			return err
		}
		result, err := exec.ExecContext(ctx, `UPDATE calls SET state = 'failed', failure_code = ?,
			failure_detail = ?, settled_at = ?, updated_at = ?
			WHERE call_id = ? AND activation_run_id = ? AND state = 'running'`,
			strings.TrimSpace(failure.Code), boundedCallDetail(failure.Detail),
			store.FormatTimestamp(failure.FailedAt), store.FormatTimestamp(failure.FailedAt),
			strings.TrimSpace(failure.CallID), strings.TrimSpace(failure.RunID),
		)
		if err != nil {
			return fmt.Errorf("store: fail call %q activation: %w", failure.CallID, err)
		}
		if err := requireOneCallRow(result, failure.CallID, "fail activation"); err != nil {
			return err
		}
		loaded, err := getCallByIDWithExecutor(ctx, exec, failure.CallID)
		if err != nil {
			return err
		}
		if err := insertCompletionDelivery(ctx, exec, loaded, failure.FailedAt); err != nil {
			return err
		}
		record = loaded
		return nil
	})
	return record, err
}

func finishActivationRun(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	claimToken string,
	status string,
	failure string,
	at time.Time,
) error {
	claimHash, err := taskpkg.ClaimTokenHash(claimToken)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `UPDATE task_runs SET status = ?, error = ?, ended_at = ?,
		claim_token = NULL, claim_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL
		WHERE id = ? AND run_kind = 'call_activation' AND claim_token_hash = ?
		AND status IN ('claimed', 'starting', 'running')`,
		status, nullableTaskString(failure), store.FormatTimestamp(at), strings.TrimSpace(runID), claimHash,
	)
	if err != nil {
		return fmt.Errorf("store: finish call activation run %q: %w", runID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect call activation run %q settlement: %w", runID, err)
	}
	if affected != 1 {
		return &callspkg.Error{
			Code:    callspkg.CodeAlreadySettled,
			Message: fmt.Sprintf("call activation run %q was already settled", runID),
		}
	}
	return nil
}

func (g *CallRepo) RecordRepair(
	ctx context.Context,
	mutation callspkg.RepairMutation,
) (record callspkg.CallRecord, err error) {
	if err := g.checkReady(ctx, "record call repair"); err != nil {
		return callspkg.CallRecord{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "record call repair", func(exec taskSQLExecutor) error {
		result, updateErr := exec.ExecContext(ctx, `UPDATE calls SET repair_attempts = 1,
			first_issue_text = ?, updated_at = ?
			WHERE call_id = ? AND state = 'running' AND repair_attempts = 0`,
			boundedIssueText(mutation.IssueText), store.FormatTimestamp(mutation.At), strings.TrimSpace(mutation.CallID),
		)
		if updateErr != nil {
			return fmt.Errorf("store: record repair for call %q: %w", mutation.CallID, updateErr)
		}
		if updateErr := requireOneCallRow(result, mutation.CallID, "record repair"); updateErr != nil {
			return updateErr
		}
		loaded, loadErr := getCallByIDWithExecutor(ctx, exec, mutation.CallID)
		if loadErr != nil {
			return loadErr
		}
		if insertErr := insertRepairDelivery(ctx, exec, loaded, mutation.At); insertErr != nil {
			return insertErr
		}
		record = loaded
		return nil
	})
	return record, err
}

func insertRepairDelivery(
	ctx context.Context,
	exec taskSQLExecutor,
	record callspkg.CallRecord,
	at time.Time,
) error {
	identity := callDeliveryIdentityFor("repair", record.CallID)
	_, err := exec.ExecContext(ctx, `INSERT INTO call_deliveries (
		delivery_id, kind, subject_id, recipient_session_id, owner_key, wake_event_id,
		state, created_at, updated_at
	) VALUES (?, 'repair', ?, ?, ?, ?, 'pending', ?, ?)`,
		identity.deliveryID, record.CallID, record.ChildSessionID,
		participation.OwnerKey(record.Caller), identity.wakeID,
		store.FormatTimestamp(at), store.FormatTimestamp(at),
	)
	if err != nil {
		return fmt.Errorf("store: insert repair delivery for call %q: %w", record.CallID, err)
	}
	return nil
}

func (g *CallRepo) SettleCall(
	ctx context.Context,
	mutation callspkg.SettlementMutation,
) (record callspkg.CallRecord, err error) {
	if err := g.checkReady(ctx, "settle call"); err != nil {
		return callspkg.CallRecord{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "settle call", func(exec taskSQLExecutor) error {
		current, err := getCallByIDWithExecutor(ctx, exec, mutation.CallID)
		if err != nil {
			return err
		}
		if len(mutation.Superseded) > 0 {
			if err := putCallPayload(
				ctx, exec, current.WorkspaceID, mutation.SupersededRef, mutation.Superseded, mutation.SettledAt,
			); err != nil {
				return err
			}
			_, err := exec.ExecContext(ctx, `UPDATE calls SET superseded_ref = ?, updated_at = ? WHERE call_id = ?`,
				mutation.SupersededRef, store.FormatTimestamp(mutation.SettledAt), mutation.CallID)
			if err != nil {
				return fmt.Errorf("store: record superseded result for call %q: %w", mutation.CallID, err)
			}
			record, err = getCallByIDWithExecutor(ctx, exec, mutation.CallID)
			return err
		}
		if current.State != mutation.ExpectedState {
			return &callspkg.Error{Code: callspkg.CodeAlreadySettled, Message: fmt.Sprintf("call is %s", current.State)}
		}
		if len(mutation.Result) > 0 {
			if err := putCallPayload(ctx, exec, current.WorkspaceID, mutation.ResultRef, mutation.Result, mutation.SettledAt); err != nil {
				return err
			}
		}
		result, err := exec.ExecContext(ctx, `UPDATE calls SET state = ?, verdict = ?, result_ref = ?,
			result_bytes = ?, failure_code = ?, failure_detail = ?, second_issue_text = ?, final_prose_preview = ?,
			settled_at = ?, updated_at = ? WHERE call_id = ? AND state = ?`,
			string(mutation.State), nullableTaskString(string(mutation.Verdict)),
			nullableTaskString(mutation.ResultRef), callNullableInt(mutation.ResultBytes, len(mutation.Result) > 0),
			nullableTaskString(mutation.FailureCode), nullableTaskString(boundedCallDetail(mutation.FailureDetail)),
			boundedIssueText(mutation.SecondIssueText),
			boundedIssueText(mutation.FinalProsePreview), store.FormatTimestamp(mutation.SettledAt),
			store.FormatTimestamp(mutation.SettledAt), mutation.CallID, string(mutation.ExpectedState),
		)
		if err != nil {
			return fmt.Errorf("store: settle call %q: %w", mutation.CallID, err)
		}
		if err := requireOneCallRow(result, mutation.CallID, "settle"); err != nil {
			return err
		}
		record, err = getCallByIDWithExecutor(ctx, exec, mutation.CallID)
		if err != nil {
			return err
		}
		return insertCompletionDelivery(ctx, exec, record, mutation.SettledAt)
	})
	return record, err
}

func insertCompletionDelivery(
	ctx context.Context,
	exec taskSQLExecutor,
	record callspkg.CallRecord,
	at time.Time,
) error {
	if strings.TrimSpace(record.ParentSessionID) == "" {
		return nil
	}
	identity := callDeliveryIdentityFor("completion", record.CallID)
	_, err := exec.ExecContext(ctx, `INSERT INTO call_deliveries (
		delivery_id, kind, subject_id, recipient_session_id, owner_key, wake_event_id,
		state, created_at, updated_at
	) VALUES (?, 'completion', ?, ?, ?, ?, 'pending', ?, ?)
	ON CONFLICT(kind, subject_id, recipient_session_id) DO NOTHING`,
		identity.deliveryID, record.CallID, record.ParentSessionID,
		participation.OwnerKey(record.Caller), identity.wakeID,
		store.FormatTimestamp(at), store.FormatTimestamp(at),
	)
	if err != nil {
		return fmt.Errorf("store: insert completion delivery for call %q: %w", record.CallID, err)
	}
	return nil
}

func getCallByIDWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	callID string,
) (callspkg.CallRecord, error) {
	record, err := scanCallRecord(exec.QueryRowContext(ctx,
		`SELECT `+callSelectColumnsSQL+` FROM calls WHERE call_id = ?`, strings.TrimSpace(callID)))
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, callNotFound(callID)
	}
	if err != nil {
		return callspkg.CallRecord{}, fmt.Errorf("store: get call %q: %w", callID, err)
	}
	return record, nil
}

func requireOneCallRow(result sql.Result, callID string, action string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect call %q %s: %w", callID, action, err)
	}
	if affected != 1 {
		return &callspkg.Error{Code: callspkg.CodeAlreadySettled, Message: fmt.Sprintf("call %q %s was fenced", callID, action)}
	}
	return nil
}

func callNullableInt(value int, valid bool) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(value), Valid: valid}
}

func boundedCallDetail(value string) string { return boundedUTF8String(value, maxCallDetailBytes) }
func boundedIssueText(value string) string  { return boundedUTF8String(value, maxIssueTextBytes) }

func boundedUTF8String(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
