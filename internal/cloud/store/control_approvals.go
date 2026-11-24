// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type CloudApproval struct {
	ID              string
	ComputerID      remoteproto.ComputerID
	DeviceID        remoteproto.DeviceID
	DeviceName      string
	JKT             string
	State           string
	CreatedAt       time.Time
	DecidedAt       time.Time
	DecidedByDevice remoteproto.DeviceID
	DecisionJWS     string
	AppliedResult   string
}

func cloudApproval(row sqlcgen.CloudApprovalRow) CloudApproval {
	return CloudApproval{
		ID: row.ID, ComputerID: remoteproto.ComputerID(row.ComputerID), DeviceID: remoteproto.DeviceID(row.DeviceID),
		DeviceName: row.DeviceName, JKT: row.Jkt, State: row.State, CreatedAt: row.CreatedAt.Time,
		DecidedAt: row.DecidedAt.Time, DecidedByDevice: remoteproto.DeviceID(row.DecidedByDevice.String),
		DecisionJWS: row.DecisionJws.String, AppliedResult: row.AppliedResult.String,
	}
}

func (q *Queries) CloudApproval(ctx context.Context, account, id string) (CloudApproval, error) {
	row, err := q.generated.CloudApproval(ctx, sqlcgen.CloudApprovalParams{AccountID: account, ID: id})
	return cloudApproval(row), err
}

func (q *Queries) ListCloudApprovals(
	ctx context.Context, account string, pendingSince, decidedSince time.Time,
) ([]CloudApproval, error) {
	rows, err := q.generated.ListCloudApprovals(ctx, sqlcgen.ListCloudApprovalsParams{
		AccountID: account, PendingSince: timestamp(pendingSince), DecidedSince: timestamp(decidedSince),
	})
	if err != nil {
		return nil, err
	}
	approvals := make([]CloudApproval, 0, len(rows))
	for _, row := range rows {
		approvals = append(approvals, cloudApproval(sqlcgen.CloudApprovalRow(row)))
	}
	return approvals, nil
}

func (q *Queries) RecordPendingCloudApproval(
	ctx context.Context, computer ControlComputer, approval remoteproto.ApprovalPending, at, expiresBefore time.Time,
) (bool, error) {
	if err := q.generated.ExpireComputerApprovals(ctx, sqlcgen.ExpireComputerApprovalsParams{
		ComputerID: string(computer.ID), CreatedAt: timestamp(expiresBefore),
	}); err != nil {
		return false, err
	}
	count, err := q.generated.RecordPendingCloudApproval(ctx, sqlcgen.RecordPendingCloudApprovalParams{
		ApprovalID: approval.ApprovalID, ComputerID: string(computer.ID), AccountID: computer.AccountID,
		DeviceID: string(approval.DeviceID), Jkt: approval.JKT, CreatedAt: timestamp(at),
	})
	return count == 1, err
}

func (q *Queries) ComputerApprovesKey(ctx context.Context, computer remoteproto.ComputerID, jkt string) (bool, error) {
	return q.generated.ComputerApprovesKey(
		ctx,
		sqlcgen.ComputerApprovesKeyParams{ComputerID: string(computer), Jkt: jkt},
	)
}

func (q *Queries) ListComputersApprovingKey(ctx context.Context, account, jkt string) ([]string, error) {
	return q.generated.ListComputersApprovingKey(
		ctx,
		sqlcgen.ListComputersApprovingKeyParams{AccountID: account, Jkt: jkt},
	)
}

func (q *Queries) SetComputerApprovedKeys(
	ctx context.Context, computer ControlComputer, keys []string, at time.Time,
) error {
	if err := q.generated.ClearComputerApprovedKeys(ctx, string(computer.ID)); err != nil {
		return err
	}
	return q.generated.InsertComputerApprovedKeys(ctx, sqlcgen.InsertComputerApprovedKeysParams{
		ComputerID: string(computer.ID), AccountID: computer.AccountID, Jkts: keys, ReportedAt: timestamp(at),
	})
}

func (q *Queries) SaveCloudApprovalDecision(ctx context.Context, account string, approval CloudApproval) error {
	return q.generated.SaveCloudApprovalDecision(ctx, sqlcgen.SaveCloudApprovalDecisionParams{
		AccountID: account, ID: approval.ID, DecidedByDevice: dbText(string(approval.DecidedByDevice)),
		DecidedAt: timestamp(approval.DecidedAt), DecisionJws: dbText(approval.DecisionJWS),
	})
}

func (q *Queries) SaveCloudApprovalResult(
	ctx context.Context, computer ControlComputer, applied remoteproto.ApprovalApplied, at time.Time,
) (bool, error) {
	count, err := q.generated.SaveCloudApprovalResult(ctx, sqlcgen.SaveCloudApprovalResultParams{
		AccountID: computer.AccountID, ComputerID: string(computer.ID), ID: applied.ApprovalID,
		Result: applied.Result, AppliedAt: timestamp(at),
	})
	return count == 1, err
}

func (q *Queries) UndeliveredCloudApprovals(
	ctx context.Context,
	computer ControlComputer,
	since time.Time,
) ([]string, error) {
	return q.generated.UndeliveredCloudApprovals(ctx, sqlcgen.UndeliveredCloudApprovalsParams{
		AccountID: computer.AccountID, ComputerID: string(computer.ID), CreatedAt: timestamp(since),
	})
}
