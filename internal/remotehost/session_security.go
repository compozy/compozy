package remotehost

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Sessions) ApplySnapshot(ctx context.Context, snapshot remoteproto.SecuritySnapshot) (bool, error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	changed, err := s.security.ApplySnapshot(snapshot, func(next remoteproto.SecuritySnapshot) error {
		return s.reconcileSnapshot(ctx, next)
	})
	if changed {
		err = s.applyEntitlement(ctx)
	}
	return changed, err
}

func (s *Sessions) ApplyIncremental(
	ctx context.Context,
	msg remoteproto.ControlMessage,
) (bool, *remoteproto.SnapshotRequest, error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	changed, request, err := s.security.ApplyIncremental(msg, func(next remoteproto.SecuritySnapshot) error {
		return s.reconcileSnapshot(ctx, next)
	})
	if changed {
		err = s.applyEntitlement(ctx)
	}
	return changed, request, err
}

func (s *Sessions) Snapshot() remoteproto.SecuritySnapshot {
	s.serial.Lock()
	defer s.serial.Unlock()
	return s.security.Snapshot()
}

func (s *Sessions) ApplyLease(ctx context.Context, lease remoteproto.EntitlementLease) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	now := s.now()
	result, err := s.security.ApplyLease(lease, now)
	if err != nil || result != remoteproto.LeaseApplied {
		return err
	}
	return s.applyEntitlement(ctx)
}

func (s *Sessions) reconcileSnapshot(ctx context.Context, next remoteproto.SecuritySnapshot) error {
	s.blockCommits()
	if !next.Linked {
		s.endRequests(func(Session) error { return ErrDeviceRevoked })
		return s.store.WriteRemote(ctx, func(ctx context.Context, tx Tx) error { return tx.ClearLink(ctx) })
	}
	policy := s.policy
	state := admissionState{snapshot: next, policy: policy}
	s.endRequests(func(session Session) error {
		if !state.allows(session) {
			return ErrDeviceRevoked
		}
		return nil
	})
	return s.revokeMatching(ctx, func(ctx context.Context, tx Tx, session Session) (error, error) {
		if !state.allows(session) {
			return ErrDeviceRevoked, nil
		}
		return nil, nil
	}, func(ctx context.Context, tx Tx) error { return tx.SaveSnapshot(ctx, next, s.now()) })
}

func (s *Sessions) applyEntitlement(ctx context.Context) error {
	lease, available := s.security.CurrentLease(s.now())
	s.mu.RLock()
	previous := s.admission.lease
	s.mu.RUnlock()
	if previous.Sequence > 0 &&
		(!available || previous.ValidUntil <= s.now().Unix() || lease.PlanLimited || lostRoutes(previous, lease)) {
		s.blockCommits()
		err := s.revokeMatching(ctx, func(_ context.Context, _ Tx, session Session) (error, error) {
			if !available || previous.ValidUntil <= s.now().Unix() {
				return ErrEntitlement, nil
			}
			if !lease.Allows(session.Route, s.now()) {
				return ErrPlanRequired, nil
			}
			return nil, nil
		}, nil)
		if err != nil {
			return err
		}
	}
	s.publishLocked()
	return nil
}

func lostRoutes(before, after remoteproto.EntitlementLease) bool {
	return slices.ContainsFunc(before.AllowedRoutes, func(route remoteproto.RouteKind) bool {
		return !slices.Contains(after.AllowedRoutes, route)
	})
}

func (s *Sessions) Entitlement() (remoteproto.EntitlementLease, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lease := s.admission.lease
	lease.AllowedRoutes = slices.Clone(lease.AllowedRoutes)
	return lease, lease.Sequence > 0 && s.now().Unix() < lease.ValidUntil
}

func (s *Sessions) Reconfigure(ctx context.Context, requireApproval bool, maximum []remoteproto.Scope) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	policy := sessionPolicy{requireApproval, slices.Clone(maximum)}
	if s.policy.requireApproval == policy.requireApproval && slices.Equal(s.policy.maxScopes, policy.maxScopes) {
		return nil
	}
	s.blockCommits()
	state := admissionState{snapshot: s.security.Snapshot(), policy: policy}
	err := s.revokeMatching(ctx, func(ctx context.Context, tx Tx, session Session) (error, error) {
		if !state.allows(session) {
			return ErrDeviceRevoked, nil
		}
		if policy.requireApproval {
			approved, err := tx.Approval(ctx, session.DeviceID, session.JKT)
			if errors.Is(err, ErrMissing) || (err == nil && approved.State != approvalApproved) {
				return ErrDeviceRevoked, nil
			}
			return nil, err
		}
		return nil, nil
	}, nil)
	if err == nil {
		s.policy = policy
		s.publishLocked()
	}
	return err
}

func (s *Sessions) RevokeDevice(ctx context.Context, device remoteproto.DeviceID) (int, error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	s.locallyRevoked[device] = true
	return s.revokeLocked(ctx, ErrDeviceRevoked, func(session Session) bool { return session.DeviceID == device })
}

func (s *Sessions) RevokeAll(ctx context.Context, reason string) error {
	err := error(ErrDeviceRevoked)
	if reason == ErrEntitlement.Code {
		err = ErrEntitlement
	}
	_, revokeErr := s.revoke(ctx, err, func(Session) bool { return true })
	return revokeErr
}

func (s *Sessions) RevokeSession(ctx context.Context, id string) error {
	_, err := s.revoke(ctx, ErrDeviceRevoked, func(session Session) bool { return session.ID == id })
	return err
}

func (s *Sessions) revoke(ctx context.Context, reason error, match func(Session) bool) (int, error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	return s.revokeLocked(ctx, reason, match)
}

func (s *Sessions) revokeLocked(ctx context.Context, reason error, match func(Session) bool) (int, error) {
	s.blockCommits()
	closed := s.endRequests(func(session Session) error {
		if match(session) {
			return reason
		}
		return nil
	})
	err := s.revokeMatching(ctx, func(_ context.Context, _ Tx, session Session) (error, error) {
		if match(session) {
			return reason, nil
		}
		return nil, nil
	}, nil)
	if err == nil {
		s.publishLocked()
	}
	return closed, err
}

func (s *Sessions) revokeMatching(
	ctx context.Context,
	match func(context.Context, Tx, Session) (error, error),
	finish func(context.Context, Tx) error,
) error {
	var revoked map[string]error
	now := s.now()
	err := s.store.WriteRemote(ctx, func(ctx context.Context, tx Tx) error {
		revoked = make(map[string]error)
		sessions, err := tx.ActiveSessions(ctx, now)
		if err != nil {
			return err
		}
		for _, session := range sessions {
			reason, err := match(ctx, tx, session)
			if err != nil {
				return err
			}
			if reason == nil {
				continue
			}
			if err := tx.RevokeSession(ctx, session.ID, now); err != nil {
				return err
			}
			revoked[session.ID] = reason
		}
		if finish != nil {
			return finish(ctx, tx)
		}
		return nil
	})
	if err == nil {
		s.endRequests(func(session Session) error { return revoked[session.ID] })
	}
	return err
}

func (s *Sessions) Expire(ctx context.Context) error {
	s.serial.Lock()
	defer s.serial.Unlock()
	now := s.now()
	s.endRequests(func(session Session) error {
		if !now.Before(session.ExpiresAt) {
			return ErrSessionExpired
		}
		if _, available := s.security.CurrentLease(now); !available {
			return ErrEntitlement
		}
		return nil
	})
	s.mu.RLock()
	lease := s.admission.lease
	s.mu.RUnlock()
	if lease.Sequence > 0 && lease.ValidUntil <= now.Unix() {
		return s.applyEntitlement(ctx)
	}
	return nil
}

func (s *Sessions) Sweep(ctx context.Context) error {
	return s.store.WriteRemote(ctx, func(ctx context.Context, tx Tx) error { return tx.Sweep(ctx, s.now()) })
}

func (s *Sessions) Stop() {
	s.serial.Lock()
	defer s.serial.Unlock()
	s.mu.Lock()
	s.admission.enabled = false
	s.mu.Unlock()
	s.endRequests(func(Session) error { return ErrEntitlement })
	s.signalSnapshotLocked()
}

func (s *Sessions) blockCommits() {
	s.mu.Lock()
	s.admission.blocked = true
	s.mu.Unlock()
}

func (s *Sessions) publishLocked() {
	lease, _ := s.security.CurrentLease(s.now())
	s.mu.Lock()
	s.admission.snapshot = s.security.Snapshot()
	s.admission.lease = lease
	s.admission.policy = s.policy
	s.admission.blocked = false
	s.mu.Unlock()
	if s.snapshotRequested <= s.security.Snapshot().SecurityEpoch {
		s.snapshotRequested = 0
	}
	s.signalSnapshotLocked()
}

func (s *Sessions) signalSnapshotLocked() {
	close(s.snapshotChanged)
	s.snapshotChanged = make(chan struct{})
}

func conflict(code, message string) error {
	return &Failure{http.StatusConflict, code, message, nil}
}
