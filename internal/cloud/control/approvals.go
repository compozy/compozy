package control

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const recentDecisions = 24 * time.Hour

type Approval struct {
	ID             string                 `json:"id"`
	ComputerID     remoteproto.ComputerID `json:"computer_id"`
	DeviceName     string                 `json:"device_name"`
	KeyFingerprint string                 `json:"key_fingerprint"`
	CreatedAt      time.Time              `json:"created_at"`
	State          string                 `json:"state"`
	AppliedResult  *string                `json:"applied_result"`
	CanDecide      bool                   `json:"can_decide"`
}

func approvalView(row store.CloudApproval, eligible bool, now time.Time) Approval {
	state := row.State
	if state == approvalPending && !now.Before(row.CreatedAt.Add(remoteproto.ApprovalTTL)) {
		state = "expired"
	}
	var result *string
	if row.AppliedResult != "" {
		result = new(row.AppliedResult)
	}
	return Approval{
		ID: row.ID, ComputerID: row.ComputerID, DeviceName: row.DeviceName, KeyFingerprint: row.JKT,
		CreatedAt: row.CreatedAt, State: state, AppliedResult: result,
		CanDecide: eligible && state == approvalPending && (row.DecisionJWS == "" || row.AppliedResult != ""),
	}
}

func findApproval(ctx context.Context, q *store.Queries, account, id string) (store.CloudApproval, error) {
	approval, err := q.CloudApproval(ctx, account, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.CloudApproval{}, errApprovalNotFound
	}
	return approval, err
}

func (s *Service) ListApprovals(ctx context.Context, principal DevicePrincipal) ([]Approval, error) {
	var result []Approval
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			now := s.config.Now()
			rows, err := q.ListCloudApprovals(
				ctx,
				account.ID,
				now.Add(-remoteproto.ApprovalTTL),
				now.Add(-recentDecisions),
			)
			if err != nil {
				return err
			}
			var approved []string
			if device.Admin {
				approved, err = q.ListComputersApprovingKey(ctx, account.ID, principal.JKT)
				if err != nil {
					return err
				}
			}
			result = make([]Approval, 0, len(rows))
			for _, row := range rows {
				result = append(result, approvalView(row, slices.Contains(approved, string(row.ComputerID)), now))
			}
			return nil
		},
	)
	return result, err
}

func (s *Service) GetApproval(ctx context.Context, principal DevicePrincipal, id string) (Approval, error) {
	var result Approval
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			approval, err := findApproval(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			eligible := false
			if device.Admin {
				eligible, err = q.ComputerApprovesKey(ctx, approval.ComputerID, principal.JKT)
				if err != nil {
					return err
				}
			}
			result = approvalView(approval, eligible, s.config.Now())
			return nil
		},
	)
	return result, err
}

func (s *Service) DecideApproval(ctx context.Context, principal DevicePrincipal, id, envelope string) error {
	return s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			if !device.Admin {
				return errAdminRequired
			}
			approval, err := findApproval(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			if approval.DecisionJWS == envelope && envelope != "" && approval.DecidedByDevice == device.ID {
				return nil
			}
			if _, err := ownedComputer(ctx, q, account.ID, approval.ComputerID); err != nil {
				return err
			}
			now := s.config.Now()
			if !approvalView(approval, true, now).CanDecide {
				return errApprovalNotPending
			}
			approved, err := q.ComputerApprovesKey(ctx, approval.ComputerID, principal.JKT)
			if err != nil {
				return err
			}
			if !approved {
				return errApproverNotApproved
			}
			decision, signer, err := remoteproto.VerifyApprovalDecision(envelope, now)
			if err != nil || signer != principal.JKT || !matchesApproval(decision, approval) {
				return errApprovalSignature
			}
			return s.saveDecision(ctx, q, account.ID, approval, device.ID, envelope, now)
		},
	)
}

func matchesApproval(decision remoteproto.ApprovalDecision, approval store.CloudApproval) bool {
	return decision.ApprovalID == approval.ID && decision.ComputerID == approval.ComputerID &&
		decision.DeviceID == approval.DeviceID && decision.JKT == approval.JKT
}

func (s *Service) saveDecision(
	ctx context.Context, q *store.Queries, account string, approval store.CloudApproval, actor remoteproto.DeviceID,
	envelope string, at time.Time,
) error {
	approval.DecisionJWS, approval.DecidedByDevice, approval.DecidedAt = envelope, actor, at
	if err := q.SaveCloudApprovalDecision(ctx, account, approval); err != nil {
		return err
	}
	if err := s.securityEvent(ctx, q, account, actor, "device_approval_decided",
		map[string]any{"approval_id": approval.ID, "computer_id": approval.ComputerID}); err != nil {
		return err
	}
	epoch, err := q.AdvanceSecurity(ctx, store.ControlEvent{Type: remoteproto.ControlSnapshot, AccountID: account})
	if err != nil {
		return err
	}
	return q.NotifyControlEvent(ctx, store.ControlEvent{
		Type: remoteproto.ControlApprovalDecided, AccountID: account, ComputerID: approval.ComputerID,
		SecurityEpoch: epoch, ResourceID: approval.ID,
	})
}

func (s *Service) reportApprovalPending(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, pending remoteproto.ApprovalPending,
) error {
	now := s.config.Now()
	if !validApprovalID(pending.ApprovalID) || pending.DeviceID == "" || pending.JKT == "" {
		return ErrInvalidInput
	}
	recorded, err := q.RecordPendingCloudApproval(ctx, computer, pending, now, now.Add(-remoteproto.ApprovalTTL))
	if err != nil || !recorded {
		return err
	}
	return q.NotifyControlEvent(ctx, store.ControlEvent{
		Type: remoteproto.ControlApprovalPending, AccountID: computer.AccountID, ComputerID: computer.ID,
		ResourceID: pending.ApprovalID,
	})
}

func (s *Service) reportApprovalApplied(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, applied remoteproto.ApprovalApplied,
) error {
	if !validApprovalID(applied.ApprovalID) || !validApprovalResult(applied.Result) {
		return ErrInvalidInput
	}
	now := s.config.Now()
	changed, err := q.SaveCloudApprovalResult(ctx, computer, applied, now)
	if err != nil || !changed {
		return err
	}
	if err := q.AppendSecurityEvent(ctx, store.SecurityEvent{
		AccountID: computer.AccountID,
		Actor:     computerActor(computer.ID, ""),
		Type:      "device_approval_applied",
		At:        now,
		Details: map[string]any{
			"computer_id": computer.ID,
			"approval_id": applied.ApprovalID,
			"result":      applied.Result,
		},
	}); err != nil {
		return err
	}
	return q.NotifyControlEvent(ctx, store.ControlEvent{
		Type: remoteproto.ControlApprovalApplied, AccountID: computer.AccountID, ComputerID: computer.ID,
		ResourceID: applied.ApprovalID,
	})
}

func validApprovalResult(result string) bool {
	switch result {
	case approvalApproved,
		approvalDenied,
		"approver_not_eligible",
		"approval_not_pending",
		"approval_signature_invalid":
		return true
	default:
		return false
	}
}

func validApprovalID(id string) bool {
	suffix, ok := strings.CutPrefix(id, "apr_")
	return ok && len(id) <= 128 && suffix != ""
}
