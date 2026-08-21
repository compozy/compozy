package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ toolspkg.ApprovalPendingStore = (*ApprovalPendingRepo)(nil)

func (g *ApprovalPendingRepo) CreateApproval(
	ctx context.Context,
	approvalID string,
	request toolspkg.ApprovalRequest,
	requestedAt time.Time,
) (toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "create pending tool approval"); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	targetJSON := request.Target.Payload
	if len(targetJSON) == 0 {
		targetJSON = json.RawMessage(`{}`)
	}
	row, err := g.queries.CreatePendingToolApproval(ctx, sqlcgen.CreatePendingToolApprovalParams{
		ProfileID:    request.ProfileID,
		ApprovalID:   approvalID,
		WorkspaceID:  nullableApprovalString(request.WorkspaceID),
		InvocationID: request.InvocationID,
		TargetKind:   string(request.Target.Kind),
		ToolID:       nullableApprovalString(string(request.Target.ToolID)),
		TargetJson: string(
			targetJSON,
		),
		CommandID:   nullableApprovalString(request.CommandID),
		ArgsJson:    string(request.Args),
		RequestedAt: requestedAt.UnixMilli(),
		ExpiresAt:   request.ExpiresAt.UnixMilli(),
	})
	if err != nil {
		return toolspkg.ApprovalStatus{}, fmt.Errorf("store: create pending tool approval %q: %w", approvalID, err)
	}
	return toolApprovalStatusFromRow(row)
}

func (g *ApprovalPendingRepo) GetApproval(
	ctx context.Context,
	approvalID string,
) (toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "get pending tool approval"); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	if err := g.requireApprovalOwner(ctx, approvalID); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	row, err := g.queries.GetPendingToolApproval(ctx, approvalID)
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalStatus{}, toolspkg.ErrApprovalNotFound
	}
	if err != nil {
		return toolspkg.ApprovalStatus{}, fmt.Errorf("store: get pending tool approval %q: %w", approvalID, err)
	}
	return toolApprovalStatusFromRow(row)
}

func (g *ApprovalPendingRepo) ResolveApproval(
	ctx context.Context,
	approvalID string,
	outcome toolspkg.ApprovalOutcome,
	resolvedAt time.Time,
) (toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "resolve pending tool approval"); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	if err := g.requireApprovalOwner(ctx, approvalID); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	row, err := g.queries.ResolvePendingToolApproval(ctx, sqlcgen.ResolvePendingToolApprovalParams{
		ApprovalStatus: string(outcome), ResolvedAt: nullableMillis(resolvedAt), ApprovalID: approvalID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalStatus{}, g.approvalTransitionError(ctx, approvalID, toolspkg.ErrApprovalTerminal)
	}
	if err != nil {
		return toolspkg.ApprovalStatus{}, fmt.Errorf("store: resolve pending tool approval %q: %w", approvalID, err)
	}
	return toolApprovalStatusFromRow(row)
}

func (g *ApprovalPendingRepo) CompleteApprovalExecution(
	ctx context.Context,
	approvalID string,
	status toolspkg.ApprovalExecutionStatus,
	result json.RawMessage,
	errorPayload json.RawMessage,
	executedAt time.Time,
) (toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "complete tool approval execution"); err != nil {
		return toolspkg.ApprovalStatus{}, err
	}
	row, err := g.queries.CompleteToolApprovalExecution(ctx, sqlcgen.CompleteToolApprovalExecutionParams{
		ExecutionStatus: nullableApprovalString(string(status)), ResultJson: nullableJSON(result),
		ErrorJson: nullableJSON(errorPayload), ExecutedAt: nullableMillis(executedAt), ApprovalID: approvalID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ApprovalStatus{}, g.approvalTransitionError(
			ctx, approvalID, toolspkg.ErrApprovalDispatchFenced,
		)
	}
	if err != nil {
		return toolspkg.ApprovalStatus{}, fmt.Errorf("store: complete tool approval execution %q: %w", approvalID, err)
	}
	return toolApprovalStatusFromRow(row)
}

func (g *ApprovalPendingRepo) ExpireApprovals(
	ctx context.Context,
	now time.Time,
) ([]toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "expire pending tool approvals"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ExpirePendingToolApprovals(ctx, nullableMillis(now))
	if err != nil {
		return nil, fmt.Errorf("store: expire pending tool approvals: %w", err)
	}
	return toolApprovalStatusesFromRows(rows)
}

func (g *ApprovalPendingRepo) RecoverDispatchingApprovals(
	ctx context.Context,
	now time.Time,
) ([]toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "recover dispatching tool approvals"); err != nil {
		return nil, err
	}
	rows, err := g.queries.RecoverDispatchingToolApprovals(ctx, nullableMillis(now))
	if err != nil {
		return nil, fmt.Errorf("store: recover dispatching tool approvals: %w", err)
	}
	return toolApprovalStatusesFromRows(rows)
}

func (g *ApprovalPendingRepo) ListPendingApprovals(
	ctx context.Context,
) ([]toolspkg.ApprovalStatus, error) {
	if err := g.checkReady(ctx, "list pending tool approvals"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListPendingToolApprovals(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list pending tool approvals: %w", err)
	}
	return toolApprovalStatusesFromRows(rows)
}

func (g *ApprovalPendingRepo) approvalTransitionError(
	ctx context.Context,
	approvalID string,
	transitionErr error,
) error {
	_, err := g.queries.GetPendingToolApproval(ctx, approvalID)
	if errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ErrApprovalNotFound
	}
	if err != nil {
		return fmt.Errorf("store: inspect tool approval %q after rejected transition: %w", approvalID, err)
	}
	return transitionErr
}

func (g *ApprovalPendingRepo) requireApprovalOwner(ctx context.Context, approvalID string) error {
	profileID, err := toolspkg.ApprovalProfile(ctx)
	if err != nil {
		return err
	}
	var storedProfileID string
	if err := g.db.QueryRowContext(
		ctx,
		`SELECT profile_id FROM tool_approval_pending WHERE approval_id = ?`,
		approvalID,
	).Scan(&storedProfileID); errors.Is(err, sql.ErrNoRows) {
		return toolspkg.ErrApprovalNotFound
	} else if err != nil {
		return fmt.Errorf("store: inspect tool approval owner %q: %w", approvalID, err)
	}
	if storedProfileID != profileID {
		return toolspkg.ErrApprovalNotFound
	}
	return nil
}

func toolApprovalStatusesFromRows(rows []sqlcgen.ToolApprovalPending) ([]toolspkg.ApprovalStatus, error) {
	statuses := make([]toolspkg.ApprovalStatus, 0, len(rows))
	for _, row := range rows {
		status, err := toolApprovalStatusFromRow(row)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func toolApprovalStatusFromRow(row sqlcgen.ToolApprovalPending) (toolspkg.ApprovalStatus, error) {
	status := toolspkg.ApprovalStatus{
		ApprovalID: row.ApprovalID, ProfileID: row.ProfileID,
		WorkspaceID: row.WorkspaceID.String, InvocationID: row.InvocationID,
		CommandID: row.CommandID.String, ApprovalStatus: toolspkg.ApprovalOutcome(row.ApprovalStatus),
		ExecutionStatus: toolspkg.ApprovalExecutionStatus(row.ExecutionStatus.String),
		Target: toolspkg.ApprovalTarget{
			Kind: toolspkg.ApprovalTargetKind(row.TargetKind), ToolID: toolspkg.ToolID(row.ToolID.String),
			Payload: json.RawMessage(row.TargetJson),
		},
		Args: json.RawMessage(row.ArgsJson), RequestedAt: time.UnixMilli(row.RequestedAt).UTC(),
		ExpiresAt: time.UnixMilli(row.ExpiresAt).UTC(), ResumeFence: row.ResumeFence != 0,
	}
	status.Result = rawJSON(row.ResultJson)
	status.Error = rawJSON(row.ErrorJson)
	status.ResolvedAt = nullableApprovalTime(row.ResolvedAt)
	status.ExecutedAt = nullableApprovalTime(row.ExecutedAt)
	if !json.Valid(status.Target.Payload) || !json.Valid(status.Args) ||
		(len(status.Result) > 0 && !json.Valid(status.Result)) ||
		(len(status.Error) > 0 && !json.Valid(status.Error)) {
		return toolspkg.ApprovalStatus{}, fmt.Errorf(
			"store: decode tool approval %q: invalid persisted JSON", row.ApprovalID,
		)
	}
	return status, nil
}

func nullableMillis(value time.Time) sql.NullInt64 {
	return sql.NullInt64{Int64: value.UTC().UnixMilli(), Valid: !value.IsZero()}
}

func nullableJSON(value json.RawMessage) sql.NullString {
	return sql.NullString{String: string(value), Valid: len(value) > 0}
}

func nullableApprovalString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func rawJSON(value sql.NullString) json.RawMessage {
	if !value.Valid {
		return nil
	}
	return json.RawMessage(value.String)
}

func nullableApprovalTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := time.UnixMilli(value.Int64).UTC()
	return &parsed
}
