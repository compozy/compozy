package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

const callSelectColumnsSQL = `call_id, profile_id, scope, workspace_id,
	caller_kind, caller_id, actor_kind, actor_id, activation_run_id,
	parent_session_id, agent_name, child_session_id, governed_root_id, depth, state, verdict,
	expect_digest, prompt_ref, result_ref, result_bytes, result_budget_bytes, result_overflow,
	strict, idle_ttl_seconds, runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
	failure_code, failure_detail, repair_attempts, first_issue_text, second_issue_text,
	final_prose_preview, superseded_ref, idempotency_key, request_digest, batch_id, deadline_at,
	created_at, started_at, settled_at, updated_at`

type callScanFields struct {
	activationRunID, parentSessionID, agentName, childSessionID sql.NullString
	verdict, expectDigest, resultRef                            sql.NullString
	resultBytes                                                 sql.NullInt64
	failureCode, failureDetail, supersededRef                   sql.NullString
	idempotencyKey, batchID, deadlineAt                         sql.NullString
	startedAt, settledAt                                        sql.NullString
	runtimeSpeed                                                string
	idleTTLSeconds                                              int64
}

func scanCallRecord(scanner rowScanner) (callspkg.CallRecord, error) {
	var record callspkg.CallRecord
	var fields callScanFields
	var callerKind string
	var resultOverflow string
	var strict bool
	var createdAt, updatedAt string
	if err := scanner.Scan(
		&record.CallID, &record.ProfileID, &record.Scope, &record.WorkspaceID,
		&callerKind, &record.Caller.ID, &record.Actor.Kind, &record.Actor.ID, &fields.activationRunID,
		&fields.parentSessionID, &fields.agentName, &fields.childSessionID,
		&record.GovernedRootID, &record.Depth, &record.State, &fields.verdict,
		&fields.expectDigest, &record.PromptRef, &fields.resultRef, &fields.resultBytes,
		&record.ResultBudget.MaxBytes, &resultOverflow, &strict, &fields.idleTTLSeconds,
		&record.Runtime.Provider, &record.Runtime.Model, &record.Runtime.ReasoningEffort, &fields.runtimeSpeed,
		&fields.failureCode, &fields.failureDetail, &record.RepairAttempts,
		&record.FirstIssueText, &record.SecondIssueText, &record.FinalProsePreview,
		&fields.supersededRef, &fields.idempotencyKey, &record.RequestDigest, &fields.batchID,
		&fields.deadlineAt, &createdAt, &fields.startedAt, &fields.settledAt, &updatedAt,
	); err != nil {
		return callspkg.CallRecord{}, err
	}
	record.Caller = participation.OwnerRef{
		WorkspaceID: record.WorkspaceID, Kind: participation.OwnerKind(callerKind), ID: record.Caller.ID,
	}
	record.ActivationRunID = callNullString(fields.activationRunID)
	record.ParentSessionID = callNullString(fields.parentSessionID)
	record.AgentName = callNullString(fields.agentName)
	record.ChildSessionID = callNullString(fields.childSessionID)
	record.Verdict = callspkg.Verdict(callNullString(fields.verdict))
	record.ExpectDigest = callNullString(fields.expectDigest)
	record.ResultRef = callNullString(fields.resultRef)
	if fields.resultBytes.Valid {
		record.ResultBytes = int(fields.resultBytes.Int64)
	}
	record.ResultBudget.Overflow = contracts.OverflowMode(resultOverflow)
	record.Strict = strict
	record.IdleTTL = time.Duration(fields.idleTTLSeconds) * time.Second
	record.Runtime.Speed = speed.Speed(strings.TrimSpace(fields.runtimeSpeed))
	record.FailureCode = callNullString(fields.failureCode)
	record.FailureDetail = callNullString(fields.failureDetail)
	record.SupersededRef = callNullString(fields.supersededRef)
	record.IdempotencyKey = callNullString(fields.idempotencyKey)
	record.BatchID = callNullString(fields.batchID)
	if err := assignCallTimes(&record, createdAt, updatedAt, fields); err != nil {
		return callspkg.CallRecord{}, err
	}
	return record, nil
}

func assignCallTimes(
	record *callspkg.CallRecord,
	createdAt string,
	updatedAt string,
	fields callScanFields,
) error {
	var err error
	if record.CreatedAt, err = store.ParseTimestamp(createdAt); err != nil {
		return fmt.Errorf("store: parse call created_at: %w", err)
	}
	if record.UpdatedAt, err = store.ParseTimestamp(updatedAt); err != nil {
		return fmt.Errorf("store: parse call updated_at: %w", err)
	}
	values := []struct {
		raw    sql.NullString
		target *time.Time
	}{
		{raw: fields.deadlineAt, target: &record.DeadlineAt},
		{raw: fields.startedAt, target: &record.StartedAt},
		{raw: fields.settledAt, target: &record.SettledAt},
	}
	for _, value := range values {
		raw, target := value.raw, value.target
		if !raw.Valid {
			continue
		}
		parsed, parseErr := store.ParseTimestamp(raw.String)
		if parseErr != nil {
			return fmt.Errorf("store: parse call timestamp: %w", parseErr)
		}
		*target = parsed
	}
	return nil
}

func callNullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func getCallWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	scope callspkg.CallScope,
	callID string,
) (callspkg.CallRecord, error) {
	row := exec.QueryRowContext(ctx, `SELECT `+callSelectColumnsSQL+`
		FROM calls WHERE call_id = ? AND profile_id = ? AND scope = ? AND workspace_id = ?`,
		strings.TrimSpace(callID), strings.TrimSpace(scope.ProfileID), string(scope.Scope),
		strings.TrimSpace(scope.WorkspaceID),
	)
	record, err := scanCallRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, callNotFound(callID)
	}
	if err != nil {
		return callspkg.CallRecord{}, fmt.Errorf("store: get call %q: %w", callID, err)
	}
	return record, nil
}
