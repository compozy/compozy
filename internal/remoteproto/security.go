package remoteproto

import (
	"encoding/json/v2"
	"slices"
	"time"
)

type LeaseDisposition uint8

const (
	LeaseIgnored LeaseDisposition = iota
	LeaseHeld
	LeaseApplied
)

// SecurityState is owned by the computer's serialized security transition and commit fence.
type SecurityState struct {
	computer      ComputerID
	account       string
	snapshot      SecuritySnapshot
	requiredEpoch uint64
	lease         EntitlementLease
	pendingLease  EntitlementLease
}

func NewSecurityState(computer ComputerID, account string) *SecurityState {
	return &SecurityState{computer: computer, account: account}
}

func (s *SecurityState) Snapshot() SecuritySnapshot {
	return cloneSnapshot(s.snapshot)
}

func (s *SecurityState) ReadyForMint(grantEpoch uint64) bool {
	applied := s.snapshot.SecurityEpoch
	return s.snapshot.Linked && grantEpoch > 0 && grantEpoch <= applied && s.requiredEpoch <= applied
}

func (s *SecurityState) CurrentLease(now time.Time) (EntitlementLease, bool) {
	lease := s.lease
	if !s.snapshot.Linked || lease.Sequence == 0 || now.Unix() >= lease.ValidUntil {
		return EntitlementLease{}, false
	}
	lease.AllowedRoutes = slices.Clone(lease.AllowedRoutes)
	return lease, true
}

func (s *SecurityState) ApplyLease(lease EntitlementLease, now time.Time) (LeaseDisposition, error) {
	if lease.ComputerID != s.computer || lease.AccountID != s.account {
		return LeaseIgnored, ErrLeaseInvalid
	}
	if lease.Sequence <= max(s.lease.Sequence, s.pendingLease.Sequence) {
		return LeaseIgnored, nil
	}
	if err := validateLease(lease, now); err != nil {
		return LeaseIgnored, err
	}
	lease.AllowedRoutes = slices.Clone(lease.AllowedRoutes)
	if lease.SecurityEpoch > s.snapshot.SecurityEpoch {
		s.pendingLease = lease
		return LeaseHeld, nil
	}
	if !s.snapshot.Linked {
		return LeaseIgnored, nil
	}
	s.lease = lease
	s.pendingLease = EntitlementLease{}
	return LeaseApplied, nil
}

// ApplySnapshot publishes a new epoch only after the owner reconciles sessions and listeners.
func (s *SecurityState) ApplySnapshot(
	snapshot SecuritySnapshot, reconcile func(SecuritySnapshot) error,
) (bool, error) {
	if snapshot.AccountID != s.account || snapshot.SecurityEpoch == 0 {
		return false, ErrSnapshotInvalid
	}
	if snapshot.SecurityEpoch <= s.snapshot.SecurityEpoch {
		return false, nil
	}
	if !validSnapshot(snapshot) {
		return false, ErrSnapshotInvalid
	}
	return s.apply(snapshot, reconcile)
}

func (s *SecurityState) ApplyIncremental(
	msg ControlMessage, reconcile func(SecuritySnapshot) error,
) (bool, *SnapshotRequest, error) {
	switch msg.Type {
	case ControlRevokeDevice, ControlScopesChanged, ControlUnlink, ControlJWKS:
	default:
		return false, nil, ErrSnapshotInvalid
	}
	applied := s.snapshot.SecurityEpoch
	if msg.SecurityEpoch <= applied {
		return false, nil, nil
	}
	if applied == 0 || msg.SecurityEpoch != applied+1 || s.requiredEpoch > applied {
		s.requiredEpoch = max(s.requiredEpoch, msg.SecurityEpoch)
		return false, &SnapshotRequest{AppliedEpoch: applied}, nil
	}
	snapshot := cloneSnapshot(s.snapshot)
	snapshot.SecurityEpoch = msg.SecurityEpoch
	if err := applyDelta(&snapshot, msg); err != nil {
		return false, nil, err
	}
	changed, err := s.apply(snapshot, reconcile)
	return changed, nil, err
}

func applyDelta(snapshot *SecuritySnapshot, msg ControlMessage) error {
	switch msg.Type {
	case ControlRevokeDevice:
		var revoke RevokeDevice
		if err := json.Unmarshal(msg.Data, &revoke); err != nil || revoke.DeviceID == "" {
			return ErrSnapshotInvalid
		}
		snapshot.Devices = slices.DeleteFunc(snapshot.Devices, func(device SnapshotDevice) bool {
			return device.DeviceID == revoke.DeviceID
		})
	case ControlScopesChanged:
		var change ScopesChanged
		if err := json.Unmarshal(
			msg.Data,
			&change,
		); err != nil || change.DeviceID == "" ||
			!validScopes(change.Scopes) {
			return ErrSnapshotInvalid
		}
		index := slices.IndexFunc(snapshot.Devices, func(device SnapshotDevice) bool {
			return device.DeviceID == change.DeviceID
		})
		if index < 0 {
			return ErrSnapshotInvalid
		}
		snapshot.Devices[index].Scopes = change.Scopes
	case ControlUnlink:
		snapshot.Linked = false
		snapshot.Devices = nil
	}
	// A JWKS delta reconciles keys in the same callback that closes affected sessions.
	return nil
}

func (s *SecurityState) apply(snapshot SecuritySnapshot, reconcile func(SecuritySnapshot) error) (bool, error) {
	if reconcile == nil {
		return false, ErrSnapshotInvalid
	}
	if err := reconcile(cloneSnapshot(snapshot)); err != nil {
		return false, err
	}
	s.snapshot = cloneSnapshot(snapshot)
	switch {
	case !snapshot.Linked:
		s.lease.AllowedRoutes = nil
		s.lease.ValidUntil = 0
		s.pendingLease = EntitlementLease{}
	case s.pendingLease.Sequence > 0 && s.pendingLease.SecurityEpoch <= snapshot.SecurityEpoch:
		s.lease = s.pendingLease
		s.pendingLease = EntitlementLease{}
	}
	return true, nil
}

func (s SecuritySnapshot) Allows(deviceID DeviceID, jkt string, scopes []Scope) bool {
	if !s.Linked || deviceID == "" || jkt == "" {
		return false
	}
	for _, device := range s.Devices {
		if device.DeviceID != deviceID || !slices.Contains(device.JKTs, jkt) {
			continue
		}
		return !slices.ContainsFunc(scopes, func(scope Scope) bool {
			return (scope == ScopeAdmin && !device.Admin) || !slices.Contains(device.Scopes, scope)
		})
	}
	return false
}

func validSnapshot(snapshot SecuritySnapshot) bool {
	if snapshot.AccountID == "" || snapshot.SecurityEpoch == 0 {
		return false
	}
	devices := make(map[DeviceID]bool, len(snapshot.Devices))
	keys := make(map[string]bool)
	for _, device := range snapshot.Devices {
		if device.DeviceID == "" || devices[device.DeviceID] || !validScopes(device.Scopes) {
			return false
		}
		devices[device.DeviceID] = true
		for _, jkt := range device.JKTs {
			if _, err := decodeCoordinate(jkt); err != nil || keys[jkt] {
				return false
			}
			keys[jkt] = true
		}
	}
	return true
}

func cloneSnapshot(snapshot SecuritySnapshot) SecuritySnapshot {
	snapshot.Devices = slices.Clone(snapshot.Devices)
	for i := range snapshot.Devices {
		snapshot.Devices[i].JKTs = slices.Clone(snapshot.Devices[i].JKTs)
		snapshot.Devices[i].Scopes = slices.Clone(snapshot.Devices[i].Scopes)
	}
	return snapshot
}
