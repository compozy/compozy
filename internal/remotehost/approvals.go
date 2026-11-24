package remotehost

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

func admitApproval(ctx context.Context, tx Tx, session Session, now time.Time) (Approval, error, error) {
	approval, err := tx.Approval(ctx, session.DeviceID, session.JKT)
	if err != nil && !errors.Is(err, ErrMissing) {
		return Approval{}, nil, err
	}
	switch approval.State {
	case approvalApproved:
		return approval, nil, nil
	case approvalDenied:
		return approval, ErrApprovalDenied, nil
	case approvalPending:
		if now.Before(approval.ExpiresAt) {
			return approval, pendingApprovalError(approval.ID), nil
		}
		approval.State = approvalExpired
		return approval, ErrApprovalExpired, tx.SetApproval(ctx, approval)
	}
	approval = Approval{
		ID: "apr_" + rand.Text(), DeviceID: session.DeviceID, JKT: session.JKT, Name: string(session.DeviceID),
		State: approvalPending, CreatedAt: now, ExpiresAt: now.Add(remoteproto.ApprovalTTL),
	}
	return approval, pendingApprovalError(approval.ID), tx.SetApproval(ctx, approval)
}

func (s *Sessions) Approvals(ctx context.Context) ([]Approval, error) {
	approvals, err := s.store.RemoteApprovals(ctx)
	now := s.now()
	for i := range approvals {
		if approvals[i].State == approvalPending && !now.Before(approvals[i].ExpiresAt) {
			approvals[i].State = approvalExpired
		}
	}
	return approvals, err
}

func (s *Sessions) DecideLocal(ctx context.Context, id remoteproto.DeviceID, approvalID string, approve bool) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	approvals, err := s.Approvals(ctx)
	if err != nil {
		return err
	}
	var target Approval
	for _, approval := range approvals {
		if approval.DeviceID != id || approval.State != approvalPending ||
			(approvalID != "" && approval.ID != approvalID) {
			continue
		}
		if target.ID != "" {
			return conflict("approval_id_required", "Choose which device key to approve.")
		}
		target = approval
	}
	if target.ID == "" {
		return ErrApprovalNotPending
	}
	target = decideApproval(target, approve, "", s.now())
	if err := s.store.WriteRemote(
		ctx,
		func(ctx context.Context, tx Tx) error { return tx.SetApproval(ctx, target) },
	); err != nil {
		return err
	}
	s.emit(remoteproto.ControlApprovalApplied, remoteproto.ApprovalApplied{ApprovalID: target.ID, Result: target.State})
	return s.reportApproved(ctx)
}

func (s *Sessions) ApplyApproval(ctx context.Context, envelope string) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	result, err := s.applyApproval(ctx, envelope)
	if err != nil || result.ApprovalID == "" {
		return err
	}
	s.emit(remoteproto.ControlApprovalApplied, result)
	if result.Result == approvalApproved || result.Result == approvalDenied {
		return s.reportApproved(ctx)
	}
	return nil
}

func (s *Sessions) applyApproval(ctx context.Context, envelope string) (remoteproto.ApprovalApplied, error) {
	now := s.now()
	decision, signer, err := remoteproto.VerifyApprovalDecision(envelope, now)
	if err != nil || decision.ComputerID != s.identity.ComputerID {
		return remoteproto.ApprovalApplied{
			ApprovalID: remoteproto.ApprovalDecisionID(envelope), Result: remoteproto.ErrApprovalSignature.Error(),
		}, nil
	}
	res := remoteproto.ApprovalApplied{ApprovalID: decision.ApprovalID, Result: "approval_not_pending"}
	snapshot := s.security.Snapshot()
	if !snapshot.Allows(decision.DeviceID, decision.JKT, nil) {
		return res, nil
	}
	index := slices.IndexFunc(snapshot.Devices, func(device remoteproto.SnapshotDevice) bool {
		return device.Admin && !s.locallyRevoked[device.DeviceID] && slices.Contains(device.JKTs, signer)
	})
	if index < 0 {
		res.Result = remoteproto.ErrApprovalSignature.Error()
		return res, nil
	}
	admin := snapshot.Devices[index]
	err = s.store.WriteRemote(ctx, func(ctx context.Context, tx Tx) error {
		key, err := tx.Approval(ctx, admin.DeviceID, signer)
		if errors.Is(err, ErrMissing) || (err == nil && key.State != approvalApproved) {
			res.Result = "approver_not_eligible"
			return nil
		}
		if err != nil {
			return err
		}
		return applyPendingDecision(ctx, tx, decision, now, &res)
	})
	return res, err
}

func applyPendingDecision(
	ctx context.Context,
	tx Tx,
	decision remoteproto.ApprovalDecision,
	now time.Time,
	result *remoteproto.ApprovalApplied,
) error {
	target, err := tx.Approval(ctx, decision.DeviceID, decision.JKT)
	if errors.Is(err, ErrMissing) || (err == nil && target.ID != decision.ApprovalID) {
		return nil
	}
	if err != nil {
		return err
	}
	if target.DecisionJTI == decision.JTI {
		result.Result = target.State
		return nil
	}
	if target.State != approvalPending || !now.Before(target.ExpiresAt) {
		return nil
	}
	fresh, err := tx.UseDecision(ctx, decision.JTI, now.Add(remoteproto.ApprovalTTL))
	if err != nil || !fresh {
		return err
	}
	target = decideApproval(target, decision.Decision == "approve", decision.JTI, now)
	result.Result = target.State
	return tx.SetApproval(ctx, target)
}

func decideApproval(approval Approval, approved bool, jti string, now time.Time) Approval {
	approval.State = approvalDenied
	if approved {
		approval.State = approvalApproved
	}
	approval.DecidedAt = new(now)
	approval.DecisionJTI = jti
	return approval
}

func (s *Sessions) checkDevice(ctx context.Context, session Session) error {
	if s.locallyRevoked[session.DeviceID] {
		return ErrDeviceRevoked
	}
	if !s.policy.requireApproval {
		return nil
	}
	approval, err := s.store.RemoteApproval(ctx, session.DeviceID, session.JKT)
	if errors.Is(err, ErrMissing) || (err == nil && approval.State != approvalApproved) {
		return ErrDeviceRevoked
	}
	return err
}

func (s *Sessions) reportPending(approval Approval) {
	s.emit(remoteproto.ControlApprovalPending, remoteproto.ApprovalPending{
		ApprovalID: approval.ID, DeviceID: approval.DeviceID, JKT: approval.JKT,
	})
}

func (s *Sessions) reportApproved(ctx context.Context) error {
	approvals, err := s.Approvals(ctx)
	if err != nil {
		return err
	}
	snapshot := s.security.Snapshot()
	keys := make([]string, 0, len(approvals))
	for _, approval := range approvals {
		if approval.State == approvalApproved && snapshot.Allows(approval.DeviceID, approval.JKT, nil) {
			keys = append(keys, approval.JKT)
		}
	}
	s.emit(remoteproto.ControlApprovedKeys, remoteproto.ApprovedKeys{JKTs: keys})
	return nil
}

func (s *Sessions) ReportApprovals(ctx context.Context) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	approvals, err := s.Approvals(ctx)
	if err != nil {
		return err
	}
	snapshot := s.security.Snapshot()
	for _, approval := range approvals {
		if approval.State == approvalPending && snapshot.Allows(approval.DeviceID, approval.JKT, nil) {
			s.reportPending(approval)
		}
	}
	return s.reportApproved(ctx)
}
